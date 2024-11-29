package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"

	"github.com/creack/pty"
	"github.com/olahol/melody"
)

// go:embed 为 index.html 和相关文件提供静态文件服务
//
//go:embed static/*
var content embed.FS

type Server struct {
	Addr       string
	Shell      string
	Melody     *melody.Melody
	FileSystem http.Handler
}

func NewServer(addr, shell string) *Server {
	// 创建 Melody 实例
	m := melody.New()

	// 设置静态文件服务
	staticFiles, _ := fs.Sub(content, "static")
	fs := http.FileServer(http.FS(staticFiles))
	return &Server{
		Addr:       addr,
		Shell:      shell,
		Melody:     m,
		FileSystem: http.StripPrefix("/", fs),
	}
}

func (s *Server) StartPTY() (*os.File, error) {
	c := exec.Command(s.Shell)
	return pty.Start(c)
}

func (s *Server) HandleWebSocket(f *os.File) {
	// goroutine 处理来自虚拟终端的消息
	go func() {
		for {
			buf := make([]byte, 1024)
			read, err := f.Read(buf)
			if err != nil {
				log.Println("Error reading from pty:", err)
				return
			}
			if err := s.Melody.Broadcast(buf[:read]); err != nil {
				log.Println("Error broadcasting message:", err)
			}
		}
	}()

	// 处理来自 WebSocket 的消息
	s.Melody.HandleMessage(func(session *melody.Session, msg []byte) {
		if _, err := f.Write(msg); err != nil {
			log.Println("Error writing to pty:", err)
		}
	})
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 路由处理
	if r.URL.Path == "/webterminal" {
		s.Melody.HandleRequest(w, r)
	} else {
		s.FileSystem.ServeHTTP(w, r)
	}
}

func main() {
	// 创建服务器实例
	server := NewServer("0.0.0.0:8089", "sh")

	// 启动 Pty
	f, err := server.StartPTY()
	if err != nil {
		log.Fatal(err)
	}

	// 启动 WebSocket 处理
	server.HandleWebSocket(f)

	// 启动服务器
	log.Println("Listening on", server.Addr)
	if err := http.ListenAndServe(server.Addr, server); err != nil {
		log.Fatal(err)
	}
}
