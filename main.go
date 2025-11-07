package main

import (
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/url"
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
	Addr           string
	Shell          string
	Melody         *melody.Melody
	FileSystem     http.Handler
	EnableAuth     bool
	AuthUsername   string
	AuthPassword   string
	Sessions       map[string]time.Time
	Lock           sync.Mutex
	AllowedOrigins []string
	FailedAuth     map[string]int
	BlockedUntil   map[string]time.Time
}

func NewServer(addr, shell string, enableAuth bool, authUsername, authPassword string, allowedOrigins []string) *Server {
	m := melody.New()

	// Serve static files from the embedded filesystem
	staticFiles, _ := fs.Sub(content, "static")
	fs := http.FileServer(http.FS(staticFiles))

	return &Server{
		Addr:           addr,
		Shell:          shell,
		Melody:         m,
		FileSystem:     http.StripPrefix("/xterm/", fs),
		EnableAuth:     enableAuth,
		AuthUsername:   authUsername,
		AuthPassword:   authPassword,
		Sessions:       make(map[string]time.Time),
		Lock:           sync.Mutex{},
		AllowedOrigins: allowedOrigins,
		FailedAuth:     make(map[string]int),
		BlockedUntil:   make(map[string]time.Time),
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

	// 将该连接对应的 PTY 句柄存入 session 上下文
	conn.Set("pty", f)

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
}

// RegisterMessageHandlers registers global handlers for message and close events once.
func (s *Server) RegisterMessageHandlers() {
	// 消息处理：根据连接上下文找到对应 PTY
	s.Melody.HandleMessage(func(conn *melody.Session, msg []byte) {
		obj, ok := conn.Get("pty")
		if !ok || obj == nil {
			return
		}
		f, ok := obj.(*os.File)
		if !ok || f == nil {
			return
		}

		// 支持 resize 指令
		var m struct {
			Type string `json:"type"`
			Cols int    `json:"cols"`
			Rows int    `json:"rows"`
		}
		if err := json.Unmarshal(msg, &m); err == nil && m.Type == "resize" && m.Cols > 0 && m.Rows > 0 {
			if err := pty.Setsize(f, &pty.Winsize{Cols: uint16(m.Cols), Rows: uint16(m.Rows)}); err != nil {
				log.Printf("Error resizing PTY: %v", err)
			}
			return
		}

		if _, err := f.Write(msg); err != nil {
			log.Printf("Error writing to PTY: %v", err)
		}
	})

	// 关闭处理：关闭该连接的 PTY
	s.Melody.HandleClose(func(conn *melody.Session, i int, s2 string) error {
		obj, ok := conn.Get("pty")
		if ok && obj != nil {
			if f, ok2 := obj.(*os.File); ok2 && f != nil {
				f.Close()
			}
		}
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
	// Normalize access without trailing slash to /xterm/
	if r.URL.Path == "/xterm" {
		http.Redirect(w, r, "/xterm/", http.StatusMovedPermanently)
		return
	}

	// Redirect root to /xterm/
	if r.URL.Path == "/" {
		http.Redirect(w, r, "/xterm/", http.StatusMovedPermanently)
		return
	}

	if r.URL.Path == "/xterm/webterminal" {
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

	// Rate limit: block if IP is currently blocked
	ip := clientIP(r)
	if until, ok := s.BlockedUntil[ip]; ok {
		if time.Now().Before(until) {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", int(time.Until(until).Seconds())))
			return false
		}
	}

	// Perform basic auth; count only when Authorization header is present but invalid
	authHeader := r.Header.Get("Authorization")
	if !s.basicAuth(w, r) {
		if authHeader != "" {
			s.FailedAuth[ip]++
			if s.FailedAuth[ip] >= 5 {
				s.BlockedUntil[ip] = time.Now().Add(5 * time.Minute)
			}
		}
		return false
	}

	// Reset counters on success
	delete(s.FailedAuth, ip)
	delete(s.BlockedUntil, ip)

	// Create a new session
	sessionID := uuid.New().String()
	expiry := time.Now().Add(time.Minute * 30)
	s.Sessions[sessionID] = expiry
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Expires:  expiry,
		Path:     "/xterm",
		HttpOnly: true,
		Secure:   isSecureRequest(r),
		SameSite: http.SameSiteStrictMode,
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
	addr           string
	enableAuth     bool
	authUsername   string
	authPassword   string
	shell          string
	allowedOrigins []string
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
	rootCmd.Flags().StringSliceVar(&allowedOrigins, "allowed-origins", []string{}, "Comma-separated list of allowed Origin values for WebSocket; empty = same-origin only")
}

func main() {

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func RunServer(cmd *cobra.Command, args []string) {
	// Create server instance
	server := NewServer(addr, shell, enableAuth, authUsername, authPassword, allowedOrigins)

	// Handle new WebSocket connections
	server.Melody.HandleConnect(func(s *melody.Session) {
		server.HandleWebSocket(s)
	})
	// Register global handlers (message/close) once
	server.RegisterMessageHandlers()

	// Strict Origin check for WebSocket upgrades
	server.Melody.Upgrader.CheckOrigin = func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return false
		}
		if len(server.AllowedOrigins) == 0 {
			u, err := url.Parse(origin)
			if err != nil {
				return false
			}
			return strings.EqualFold(u.Host, r.Host)
		}
		for _, o := range server.AllowedOrigins {
			if strings.EqualFold(o, origin) {
				return true
			}
		}
		return false
	}

	// Start server
	log.Println("Listening on", server.Addr)
	if err := http.ListenAndServe(server.Addr, server); err != nil {
		log.Fatal(err)
	}
}

// isSecureRequest best-effort detection for TLS or proxy-forwarded TLS
func isSecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); strings.EqualFold(proto, "https") {
		return true
	}
	return false
}

func clientIP(r *http.Request) string {
	if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
		// take first IP
		parts := strings.Split(xf, ",")
		return strings.TrimSpace(parts[0])
	}
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i != -1 {
		return host[:i]
	}
	return host
}
