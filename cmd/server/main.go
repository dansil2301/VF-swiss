package main

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/vfswiss/vfswiss/internal/config"
	"github.com/vfswiss/vfswiss/internal/database"
	"github.com/vfswiss/vfswiss/internal/handlers"
	"github.com/vfswiss/vfswiss/internal/logger"
	"github.com/vfswiss/vfswiss/internal/mail"
	"github.com/vfswiss/vfswiss/internal/middleware"
	"github.com/vfswiss/vfswiss/internal/repository"
	"github.com/vfswiss/vfswiss/internal/router"
	"github.com/vfswiss/vfswiss/internal/service"
)

func main() {
	cfg, err := config.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(1)
	}

	log := logger.New()
	log.Info("starting server", "port", cfg.Port)

	ctx := context.Background()
	db, err := database.Open(ctx, cfg.DBPath)
	if err != nil {
		log.Error("database open", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	repo := repository.New(db)
	mailClient := mail.New(cfg)
	svc := service.New(cfg, repo, mailClient, log)
	limiter := middleware.NewRateLimiter(cfg.RateLimitCount, cfg.RateLimitWindow)

	templates, err := parseTemplates()
	if err != nil {
		log.Error("parse templates", "error", err)
		os.Exit(1)
	}

	h := handlers.New(templates, cfg, svc, log, limiter)
	routerHandler := router.New(cfg, h, log)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           routerHandler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-done
	log.Info("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown error", "error", err)
	}

	if err := db.Close(); err != nil {
		log.Error("database close", "error", err)
	}
	log.Info("server stopped")
}

func parseTemplates() (map[string]*template.Template, error) {
	layoutPath := filepath.Join("web", "templates", "layout.html")
	layout, err := template.ParseFiles(layoutPath)
	if err != nil {
		return nil, err
	}

	pages := map[string]string{
		"home":     filepath.Join("web", "templates", "home.html"),
		"examples": filepath.Join("web", "templates", "examples.html"),
		"success":  filepath.Join("web", "templates", "success.html"),
		"error":    filepath.Join("web", "templates", "error.html"),
	}
	templates := make(map[string]*template.Template, len(pages))
	for name, pagePath := range pages {
		clone, err := layout.Clone()
		if err != nil {
			return nil, err
		}
		clone, err = clone.ParseFiles(pagePath)
		if err != nil {
			return nil, err
		}
		templates[name] = clone
	}
	return templates, nil
}
