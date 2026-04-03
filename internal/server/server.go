package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/studyforge/study-agent/internal/config"
	"github.com/studyforge/study-agent/internal/orchestrator"
)

// Server holds the dependencies and exposes the HTTP API powering the web UI.
type Server struct {
	mu       sync.RWMutex
	cfg      *config.Config
	orch     *orchestrator.Orchestrator
	port     int
	staticFS http.FileSystem
}

// New creates a Server bound to the given port.
// When staticDir is non-empty the server serves the built frontend from that directory.
func New(cfg *config.Config, orch *orchestrator.Orchestrator, port int, staticDir string) *Server {
	if port <= 0 {
		port = 8080
	}
	s := &Server{
		cfg:  cfg,
		orch: orch,
		port: port,
	}
	if staticDir != "" {
		s.staticFS = http.Dir(staticDir)
	}
	return s
}

// ListenAndServe starts the HTTP server. It blocks until the server shuts down.
func (s *Server) ListenAndServe() error {
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	var handler http.Handler = mux
	if s.staticFS != nil {
		handler = s.spaFallback(mux)
	}
	handler = s.corsMiddleware(handler)

	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("study-forge-ai server listening on http://localhost%s", addr)
	return http.ListenAndServe(addr, handler)
}

// spaFallback serves static files for non-API routes, falling back to
// index.html for client-side routing.
func (s *Server) spaFallback(apiMux *http.ServeMux) http.Handler {
	fileServer := http.FileServer(s.staticFS)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			apiMux.ServeHTTP(w, r)
			return
		}
		// Try serving the file directly; fall back to index.html for SPA routing
		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}
		if _, err := s.staticFS.Open(path); err != nil {
			r.URL.Path = "/index.html"
		}
		fileServer.ServeHTTP(w, r)
	})
}

// Addr returns the listen address string.
func (s *Server) Addr() string {
	return fmt.Sprintf("http://localhost:%d", s.port)
}

// Config returns a read-locked copy of the current config.
func (s *Server) Config() *config.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c := *s.cfg
	return &c
}

// Orchestrator returns a read-locked reference to the current orchestrator.
func (s *Server) Orchestrator() *orchestrator.Orchestrator {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.orch
}

// SetConfigAndOrchestrator atomically replaces the config and orchestrator.
func (s *Server) SetConfigAndOrchestrator(cfg *config.Config, orch *orchestrator.Orchestrator) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg = cfg
	s.orch = orch
}

// --- Response helpers --------------------------------------------------------

func jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("json encode error: %v", err)
	}
}

func jsonError(w http.ResponseWriter, status int, message string) {
	jsonResponse(w, status, map[string]string{"error": message})
}

// sseSetup configures response headers for Server-Sent Events and returns a
// flush function. The caller should call flush() after each write.
func sseSetup(w http.ResponseWriter) func() {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		return func() {}
	}
	return flusher.Flush
}

// sseEvent writes a single SSE data line and flushes.
func sseEvent(w http.ResponseWriter, flush func(), data any) {
	raw, err := json.Marshal(data)
	if err != nil {
		log.Printf("sse marshal error: %v", err)
		return
	}
	fmt.Fprintf(w, "data: %s\n\n", raw)
	flush()
}

// --- Middleware ---------------------------------------------------------------

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// --- Path helpers ------------------------------------------------------------

// pathParam extracts the remaining path segment after prefix.
// For example, pathParam("/api/classes/math101", "/api/classes/") returns "math101".
func pathParam(r *http.Request, prefix string) string {
	return strings.TrimPrefix(r.URL.Path, prefix)
}

// decodeJSON reads and unmarshals the request body into dst.
func decodeJSON(r *http.Request, dst any) error {
	if r.Body == nil {
		return fmt.Errorf("request body is empty")
	}
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(dst)
}
