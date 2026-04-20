package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/leshchenko/pdf-extract/internal/config"
	"github.com/leshchenko/pdf-extract/internal/storage"
)

var uuidFile = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// NewRouter builds the HTTP handler.
func NewRouter(cfg *config.Config, svc *Service) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(300 * time.Second))

	r.Get("/health", healthHandler(cfg))
	r.Get("/v1/health", healthHandler(cfg))

	r.Route("/v1", func(r chi.Router) {
		r.Post("/process", processHandler(svc))
		r.Get("/files/{id}", fileDownloadHandler(cfg))
	})

	return r
}

func processHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeProblem(w, http.StatusMethodNotAllowed, "Method not allowed", "")
			return
		}
		ct := r.Header.Get("Content-Type")
		mt, _, err := mime.ParseMediaType(ct)
		if err != nil || mt == "" {
			writeProblem(w, http.StatusBadRequest, "Missing Content-Type", "Content-Type header is required")
			return
		}
		switch mt {
		case "application/json":
			r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB cap for JSON metadata
			svc.HandleProcessJSON(w, r)
		case "multipart/form-data":
			svc.HandleProcessMultipart(w, r)
		default:
			writeProblem(w, http.StatusUnsupportedMediaType, "Unsupported media type",
				`use application/json or multipart/form-data`)
		}
	}
}

func fileDownloadHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if !uuidFile.MatchString(id) {
			writeProblem(w, http.StatusBadRequest, "Invalid id", "id must be a UUID")
			return
		}
		path := filepath.Join(cfg.OutputDir, id+".png")
		st, err := os.Stat(path)
		if err != nil || st.IsDir() {
			writeProblem(w, http.StatusNotFound, "Not found", "file not found or expired")
			return
		}
		w.Header().Set("Content-Type", "image/png")
		http.ServeFile(w, r, path)
	}
}

type healthResponse struct {
	Status        string `json:"status"`
	Timestamp     string `json:"timestamp"`
	UploadsCount  int    `json:"uploads_count"`
	OutputsCount  int    `json:"outputs_count"`
	TotalCount    int    `json:"total_count"`
}

func healthHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uploads := countFiles(cfg.UploadDir)
		outputs := countFiles(cfg.OutputDir)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(healthResponse{
			Status:       "ok",
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
			UploadsCount: uploads,
			OutputsCount: outputs,
			TotalCount:   uploads + outputs,
		})
	}
}

func countFiles(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() {
			n++
		}
	}
	return n
}

// ListenAndServe starts the HTTP server.
func ListenAndServe(cfg *config.Config, log *slog.Logger) error {
	store := storage.New(cfg.FileTTL)
	svc := &Service{
		Cfg:         cfg,
		FetchClient: NewFetchClient(cfg.HTTPFetchTimeout),
		Store:       store,
	}
	handler := NewRouter(cfg, svc)
	log.Info("listening", "addr", cfg.ListenAddr)
	return http.ListenAndServe(cfg.ListenAddr, handler)
}

// Shutdown drains all pending storage operations up to the given duration.
func (s *Service) Shutdown(d time.Duration) {
	if s == nil || s.Store == nil {
		return
	}
	s.Store.Shutdown(d)
}

// Run starts the HTTP server and orchestrates graceful shutdown on SIGTERM.
func Run(cfg *config.Config, log *slog.Logger) (*Service, *http.Server, error) {
	store := storage.New(cfg.FileTTL)
	svc := &Service{
		Cfg:         cfg,
		FetchClient: NewFetchClient(cfg.HTTPFetchTimeout),
		Store:       store,
	}
	handler := NewRouter(cfg, svc)

	srv := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: handler,
	}

	ln, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		return svc, srv, err
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", cfg.ListenAddr)
		errCh <- srv.Serve(ln)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, os.Interrupt)

	select {
	case err := <-errCh:
		return svc, srv, err
	case <-sigCh:
		log.Info("shutting down")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Warn("server shutdown", "err", err)
		}
		svc.Shutdown(5 * time.Second)
		select {
		case err := <-errCh:
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				return svc, srv, err
			}
		default:
		}
		log.Info("shutdown complete")
		return svc, srv, nil
	}
}
