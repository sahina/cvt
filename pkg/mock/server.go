package mock

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/sahina/cvt/pkg/cvt"
)

// ServerConfig holds configuration for the mock server.
type ServerConfig struct {
	Host             string
	Port             int
	SchemaFiles      []string // file paths or URLs
	Watch            bool
	ValidateRequests bool
	UseExamples      bool
	LatencyMs        int
	Quiet            bool
}

// Server is a mock HTTP server that generates responses from OpenAPI schemas.
type Server struct {
	config  ServerConfig
	handler atomic.Pointer[http.Handler]
	httpSrv *http.Server
}

// NewServer creates a new mock server with the given configuration.
func NewServer(cfg ServerConfig) *Server {
	return &Server{config: cfg}
}

// Start loads schemas, starts the HTTP server, and blocks until shutdown.
func (s *Server) Start() error {
	v, schemaIDs, err := s.loadSchemas(s.config.SchemaFiles)
	if err != nil {
		return err
	}

	s.printBanner(v, schemaIDs)

	handler := s.buildHandler(v, schemaIDs)
	s.handler.Store(&handler)

	// Delegating handler reads from atomic pointer (supports hot-reload)
	delegator := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := *s.handler.Load()
		h.ServeHTTP(w, r)
	})

	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	s.httpSrv = &http.Server{
		Addr:         addr,
		Handler:      delegator,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start watcher if enabled
	if s.config.Watch {
		filePaths := filterFilePaths(s.config.SchemaFiles)
		if len(filePaths) > 0 {
			w, err := NewWatcher(filePaths, func() {
				newV, newIDs, loadErr := s.loadSchemas(s.config.SchemaFiles)
				if loadErr != nil {
					fmt.Fprintf(os.Stderr, "[mock] WARNING: reload failed: %v (keeping current schemas)\n", loadErr)
					return
				}
				newHandler := s.buildHandler(newV, newIDs)
				s.handler.Store(&newHandler)
				if !s.config.Quiet {
					fmt.Println("[mock] Schemas reloaded successfully")
				}
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "[mock] WARNING: file watch unavailable: %v\n", err)
			} else {
				defer w.Close()
				if !s.config.Quiet {
					fmt.Println("[mock] Watching for file changes")
				}
			}
		}
	}

	// Listen on port
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("port %d already in use", s.config.Port)
	}

	if !s.config.Quiet {
		fmt.Printf("[mock] Mock server listening on http://%s\n", addr)
		fmt.Println("[mock] Press Ctrl+C to stop")
	}

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.httpSrv.Serve(ln)
	}()

	select {
	case sig := <-sigCh:
		if !s.config.Quiet {
			fmt.Printf("\n[mock] Received %s, shutting down...\n", sig)
		}
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("server failed: %w", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if shutdownErr := s.httpSrv.Shutdown(ctx); shutdownErr != nil {
		return fmt.Errorf("shutdown failed: %w", shutdownErr)
	}

	if !s.config.Quiet {
		fmt.Println("[mock] Server stopped")
	}
	return nil
}

func (s *Server) loadSchemas(paths []string) (*cvt.Validator, []string, error) {
	v := cvt.NewValidator()
	var schemaIDs []string

	for _, path := range paths {
		id := schemaIDFromPath(path)
		if err := v.RegisterSchemaFromPath(id, path); err != nil {
			return nil, nil, fmt.Errorf("failed to load schema %s: %w", path, err)
		}
		schemaIDs = append(schemaIDs, id)
	}

	s.detectOverlaps(v, schemaIDs)
	return v, schemaIDs, nil
}

func (s *Server) buildHandler(v *cvt.Validator, schemaIDs []string) http.Handler {
	mockHandler := NewMockHandler(v, schemaIDs, HandlerConfig{
		UseExamples:      s.config.UseExamples,
		ValidateRequests: s.config.ValidateRequests,
		LatencyMs:        s.config.LatencyMs,
		Quiet:            s.config.Quiet,
	})

	indexH := IndexHandler(v, schemaIDs)

	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" && r.Method == "GET" {
			indexH.ServeHTTP(w, r)
			return
		}
		mockHandler.ServeHTTP(w, r)
	}))

	return CORSMiddleware(mockHandler.RecoverMiddleware(mux))
}

func (s *Server) printBanner(v *cvt.Validator, schemaIDs []string) {
	if s.config.Quiet {
		return
	}
	for _, id := range schemaIDs {
		endpoints, _ := v.ListEndpoints(id)
		fmt.Printf("[mock] Loaded schema: %s (%d endpoints)\n", id, len(endpoints))
	}
}

func (s *Server) detectOverlaps(v *cvt.Validator, schemaIDs []string) {
	if s.config.Quiet || len(schemaIDs) < 2 {
		return
	}
	seen := make(map[string]string)
	for _, id := range schemaIDs {
		endpoints, err := v.ListEndpoints(id)
		if err != nil {
			continue
		}
		for _, ep := range endpoints {
			if prev, exists := seen[ep]; exists {
				fmt.Printf("[mock] WARNING: %s defined in both %s and %s (using %s)\n", ep, prev, id, prev)
			} else {
				seen[ep] = id
			}
		}
	}
}

func schemaIDFromPath(path string) string {
	name := filepath.Base(path)
	ext := filepath.Ext(name)
	if ext != "" {
		name = strings.TrimSuffix(name, ext)
	}
	if name == "" || name == "." {
		return "schema"
	}
	return name
}

func filterFilePaths(paths []string) []string {
	var files []string
	for _, p := range paths {
		if !isURL(p) {
			files = append(files, p)
		}
	}
	return files
}

func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}
