package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/davidberget/cctask-go/internal/model"
)

// Server is an HTTP server for programmatic task creation via webhooks.
type Server struct {
	projectRoot string
	port        int
	authToken   string
	plugins     []PluginInfo
	mux         *http.ServeMux
	srv         *http.Server
	startTime   time.Time

	mu      sync.Mutex
	running bool
}

// New creates a new Server with the given project root and config.
func New(projectRoot string, cfg model.ServerConfig) *Server {
	port := cfg.Port
	if port == 0 {
		port = 8080
	}
	s := &Server{
		projectRoot: projectRoot,
		port:        port,
		authToken:   cfg.AuthToken,
		mux:         http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("POST /api/tasks", s.authMiddleware(s.handleCreateTask))
	s.mux.HandleFunc("GET /api/tasks", s.authMiddleware(s.handleListTasks))
}

// Start starts the server and blocks until it is stopped.
func (s *Server) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("server already running")
	}
	s.running = true
	s.startTime = time.Now()
	s.mu.Unlock()

	addr := fmt.Sprintf("127.0.0.1:%d", s.port)
	s.srv = &http.Server{
		Addr:         addr,
		Handler:      s.requestLogger(s.mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
		return fmt.Errorf("listen on %s: %w", addr, err)
	}

	log.Printf("cctask server listening on %s", addr)
	err = s.srv.Serve(ln)
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// StartBackground starts the server in a goroutine and returns once it's listening.
func (s *Server) StartBackground() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("server already running")
	}
	s.running = true
	s.startTime = time.Now()
	s.mu.Unlock()

	addr := fmt.Sprintf("127.0.0.1:%d", s.port)
	s.srv = &http.Server{
		Addr:         addr,
		Handler:      s.requestLogger(s.mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
		return fmt.Errorf("listen on %s: %w", addr, err)
	}

	go func() {
		if err := s.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("server error: %v", err)
		}
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	log.Printf("cctask server listening on %s (background)", addr)
	return nil
}

// Stop gracefully shuts down the server.
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	if s.srv == nil {
		return nil
	}
	return s.srv.Shutdown(ctx)
}

// Running returns true if the server is currently running.
func (s *Server) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// Port returns the configured port.
func (s *Server) Port() int {
	return s.port
}

// Plugins returns the list of loaded plugins.
func (s *Server) Plugins() []PluginInfo {
	return s.plugins
}

// RegisterPluginRoute adds a route that delegates to a plugin handler.
func (s *Server) RegisterPluginRoute(method, path string, handler http.HandlerFunc) {
	pattern := method + " " + path
	s.mux.HandleFunc(pattern, s.authMiddleware(handler))
}
