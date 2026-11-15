package router

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/vfswiss/vfswiss/internal/config"
	"github.com/vfswiss/vfswiss/internal/handlers"
	"github.com/vfswiss/vfswiss/internal/middleware"
)

func New(cfg *config.Config, h *handlers.Handlers, logger *slog.Logger) http.Handler {
	fileServer := http.FileServer(noDirFS{fs: http.Dir(cfg.StaticDir)})
	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))
	mux.HandleFunc("/robots.txt", h.Robots)
	mux.HandleFunc("/sitemap.xml", h.Sitemap)
	mux.HandleFunc("/health", h.Health)
	mux.HandleFunc("/success", h.Success)
	mux.HandleFunc("/examples", h.Examples)
	mux.HandleFunc("/request", h.SubmitRequest)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			h.NotFound(w, r)
			return
		}
		h.Home(w, r)
	})

	wrapped := middleware.RequestID(middleware.SecurityHeaders(middleware.Logger(logger)(middleware.Recovery(logger)(mux))))
	return wrapped
}

type noDirFS struct {
	fs http.FileSystem
}

func (n noDirFS) Open(name string) (http.File, error) {
	f, err := n.fs.Open(name)
	if err != nil {
		return nil, err
	}
	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if stat.IsDir() {
		index := name + "/index.html"
		if _, err := n.fs.Open(index); err != nil {
			return nil, os.ErrNotExist
		}
	}
	return f, nil
}
