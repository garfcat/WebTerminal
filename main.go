package main

import (
	"embed"
	"encoding/base64"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/google/uuid"
	"github.com/olahol/melody"
	"github.com/spf13/cobra"
)

// Embedding static files
//
//go:embed static/*
var content embed.FS

type Server struct {
	Addr         string
	Shell        string
	Melody       *melody.Melody
	FileSystem   http.Handler
	EnableAuth   bool
	AuthUsername string
	AuthPassword string
	Sessions     map[string]time.Time
	Lock         sync.Mutex
}

func NewServer(addr, shell string, enableAuth bool, authUsername, authPassword string) *Server {
	m := melody.New()

	// Serve static files from the embedded filesystem
	staticFiles, _ := fs.Sub(content, "static")
	fs := http.FileServer(http.FS(staticFiles))

	return &Server{
		Addr:         addr,
		Shell:        shell,
		Melody:       m,
		FileSystem:   http.StripPrefix("/", fs),
		EnableAuth:   enableAuth,
		AuthUsername: authUsername,
		AuthPassword: authPassword,
		Sessions:     make(map[string]time.Time),
		Lock:         sync.Mutex{},
	}
}

func (s *Server) StartPTY() (*os.File, error) {
	c := exec.Command(s.Shell)
	return pty.Start(c)
}

func (s *Server) HandleWebSocket(conn *melody.Session) {
	f, err := s.StartPTY()
	if err != nil {
		log.Printf("Error starting PTY: %v", err)
		conn.CloseWithMsg([]byte("Failed to start terminal"))
		return
	}

	// Goroutine to handle messages from PTY
	go func() {
		defer f.Close()
		for {
			buf := make([]byte, 1024)
			read, err := f.Read(buf)
			if err != nil {
				log.Printf("Error reading from PTY: %v", err)
				return
			}
			if err := conn.Write(buf[:read]); err != nil {
				log.Printf("Error writing to WebSocket: %v", err)
				return
			}
		}
	}()

	// Handle messages from WebSocket
	s.Melody.HandleMessage(func(conn *melody.Session, msg []byte) {
		if _, err := f.Write(msg); err != nil {
			log.Printf("Error writing to PTY: %v", err)
		}
	})

	// On Close cleanup
	s.Melody.HandleClose(func(conn *melody.Session, i int, s2 string) error {
		f.Close()
		return nil
	})
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.EnableAuth {
		if !s.checkSession(w, r) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	// Route handling
	if r.URL.Path == "/webterminal" {
		s.Melody.HandleRequest(w, r)
	} else {
		s.FileSystem.ServeHTTP(w, r)
	}
}

func (s *Server) checkSession(w http.ResponseWriter, r *http.Request) bool {
	s.Lock.Lock()
	defer s.Lock.Unlock()
	cookie, err := r.Cookie("session_id")
	if err == nil {
		sessionID := cookie.Value
		if expiry, ok := s.Sessions[sessionID]; ok {
			if time.Now().Before(expiry) {
				// Update session expiry
				s.Sessions[sessionID] = time.Now().Add(time.Minute * 30)
				return true
			}
		}
	}

	// Perform basic auth
	if !s.basicAuth(w, r) {
		return false
	}

	// Create a new session
	sessionID := uuid.New().String()
	expiry := time.Now().Add(time.Minute * 30)
	s.Sessions[sessionID] = expiry
	http.SetCookie(w, &http.Cookie{
		Name:    "session_id",
		Value:   sessionID,
		Expires: expiry,
	})

	return true
}

func (s *Server) basicAuth(w http.ResponseWriter, r *http.Request) bool {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
		return false
	}

	token := strings.SplitN(auth, " ", 2)
	if len(token) != 2 || token[0] != "Basic" {
		return false
	}

	payload, _ := base64.StdEncoding.DecodeString(token[1])
	pair := strings.SplitN(string(payload), ":", 2)
	expectedUsername := s.AuthUsername
	expectedPassword := s.AuthPassword

	return len(pair) == 2 && pair[0] == expectedUsername && pair[1] == expectedPassword
}

var (
	// Define command line flags using Cobra
	addr         string
	enableAuth   bool
	authUsername string
	authPassword string
	shell        string
)

var rootCmd = &cobra.Command{
	Use:   "webterminal",
	Short: "WebTerminal is a web-based terminal application.",
	Run:   RunServer,
}

func init() {

	// Define flags
	rootCmd.Flags().StringVar(&addr, "addr", "0.0.0.0:8089", "Server address")
	rootCmd.Flags().BoolVar(&enableAuth, "auth", false, "Enable authentication (default: false)")
	rootCmd.Flags().StringVar(&authUsername, "username", "admin", "Authentication username")
	rootCmd.Flags().StringVar(&authPassword, "password", "password", "Authentication password")
	rootCmd.Flags().StringVar(&shell, "shell", "sh", "Shell to use (default: sh)")
}

func main() {

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func RunServer(cmd *cobra.Command, args []string) {
	// Create server instance
	server := NewServer(addr, shell, enableAuth, authUsername, authPassword)

	// Handle new WebSocket connections
	server.Melody.HandleConnect(func(s *melody.Session) {
		server.HandleWebSocket(s)
	})

	// Start server
	log.Println("Listening on", server.Addr)
	if err := http.ListenAndServe(server.Addr, server); err != nil {
		log.Fatal(err)
	}
}
