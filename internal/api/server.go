package api

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"vyapari-voice-copilot/internal/config"
	"vyapari-voice-copilot/internal/domain"
	"vyapari-voice-copilot/internal/gnani"
	"vyapari-voice-copilot/internal/store"
)

const Version = "1.0.0"

type Server struct {
	cfg       config.Config
	store     *store.Store
	gnani     *gnani.Client
	mux       *http.ServeMux
	startedAt time.Time
	webDir    string
}

func New(cfg config.Config, dataStore *store.Store, gnaniClient *gnani.Client) *Server {
	s := &Server{
		cfg:       cfg,
		store:     dataStore,
		gnani:     gnaniClient,
		mux:       http.NewServeMux(),
		startedAt: time.Now(),
		webDir:    findWebDir(),
	}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.recoverPanic(s.securityHeaders(s.logRequests(s.mux)))
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/dashboard", s.handleDashboard)
	s.mux.HandleFunc("POST /api/command", s.handleCommand)
	s.mux.HandleFunc("POST /api/stt", s.handleSTT)
	s.mux.HandleFunc("POST /api/tts", s.handleTTS)
	s.mux.HandleFunc("POST /api/products", s.handleAddProduct)
	s.mux.HandleFunc("POST /api/stock", s.handleStockAction)
	s.mux.HandleFunc("POST /api/undo", s.handleUndo)
	s.mux.HandleFunc("POST /api/reset", s.handleReset)
	s.mux.HandleFunc("GET /api/export.csv", s.handleExport)
	s.mux.HandleFunc("GET /api/reorder.csv", s.handleReorderExport)
	s.mux.HandleFunc("GET /", s.handleStatic)
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "API endpoint not found", nil)
		return
	}

	clean := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/"))
	if clean == "." || clean == "" {
		clean = "index.html"
	}
	full := filepath.Join(s.webDir, clean)
	if !pathWithin(full, s.webDir) {
		http.NotFound(w, r)
		return
	}

	info, err := os.Stat(full)
	if err != nil || info.IsDir() {
		// Single-page application fallback.
		full = filepath.Join(s.webDir, "index.html")
	}
	if filepath.Base(full) == "service-worker.js" || filepath.Base(full) == "index.html" {
		w.Header().Set("Cache-Control", "no-cache")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=3600")
	}
	http.ServeFile(w, r, full)
}

func findWebDir() string {
	candidates := []string{"web", "./web", "../../web", "../../../web"}
	if executable, err := os.Executable(); err == nil {
		executableDir := filepath.Dir(executable)
		candidates = append(candidates,
			filepath.Join(executableDir, "web"),
			filepath.Join(executableDir, "..", "web"),
		)
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(filepath.Join(candidate, "index.html")); err == nil && !info.IsDir() {
			absolute, absErr := filepath.Abs(candidate)
			if absErr == nil {
				return absolute
			}
			return candidate
		}
	}
	return "web"
}

func pathWithin(path, root string) bool {
	absolutePath, err1 := filepath.Abs(path)
	absoluteRoot, err2 := filepath.Abs(root)
	if err1 != nil || err2 != nil {
		return false
	}
	relative, err := filepath.Rel(absoluteRoot, absolutePath)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Permissions-Policy", "microphone=(self), camera=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'; media-src 'self' blob:; worker-src 'self'; manifest-src 'self'; base-uri 'self'; form-action 'self'")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			log.Printf("%s %s %d %s", r.Method, r.URL.Path, recorder.status, time.Since(started).Round(time.Millisecond))
		}
	})
}

func (s *Server) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("panic serving %s: %v\n%s", r.URL.Path, recovered, debug.Stack())
				writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong. Please try again.", nil)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func formatMoney(value float64) string {
	return fmt.Sprintf("₹%.0f", value)
}

func productByID(products []domain.Product, id string) (domain.Product, bool) {
	for _, product := range products {
		if product.ID == id {
			return product, true
		}
	}
	return domain.Product{}, false
}
