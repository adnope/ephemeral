package main

import (
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/adnope/ephemeral/internal/bodyindex"
	"github.com/adnope/ephemeral/internal/config"
	"github.com/adnope/ephemeral/internal/handler"
	"github.com/adnope/ephemeral/internal/media"
	mw "github.com/adnope/ephemeral/internal/middleware"
	"github.com/adnope/ephemeral/internal/sse"
	"github.com/adnope/ephemeral/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("config load failed", "err", err)
		os.Exit(1)
	}

	migrationSQL, err := loadMigrations("migrations")
	if err != nil {
		logger.Error("migration read failed", "err", err)
		os.Exit(1)
	}

	db, err := store.OpenDB(cfg.DBPath(), string(migrationSQL))
	if err != nil {
		logger.Error("database init failed", "err", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	itemRepo := store.NewItemRepo(db)
	sessionRepo := store.NewSessionRepo(db)
	userRepo := store.NewUserRepo(db)

	broker := sse.NewBroker()

	mediaPool := media.NewPool(itemRepo, broker)

	bodyIndexer := bodyindex.New(db, cfg.DataDir, logger)

	tmpl, err := parseTemplates()
	if err != nil {
		logger.Error("template parse failed", "err", err)
		os.Exit(1)
	}

	h := handler.NewHandler(
		itemRepo,
		sessionRepo,
		userRepo,
		broker,
		mediaPool,
		bodyIndexer,
		tmpl,
		cfg.DataDir,
		logger,
	)

	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)

	r.Use(mw.RequestLogger(logger))
	r.Use(mw.RateLimit(100, time.Minute))
	r.Use(mw.SessionAuth(sessionRepo))

	staticFS := http.FileServer(http.Dir("web/static"))
	r.Handle("/static/*", http.StripPrefix("/static/", staticFS))

	r.Get("/", h.Index)
	r.Get("/history", h.History)
	r.Get("/search", h.SearchItems)
	r.Get("/login", h.LoginPage)

	r.Get("/api/events", h.Events)
	r.Post("/api/upload", h.Upload)
	r.Post("/api/message", h.Message)
	r.Get("/api/files/*", h.ServeFile)
	r.Get("/api/file-preview/{id}", h.PreviewFile)
	r.Post("/api/login", h.Login)
	r.Post("/api/logout", h.Logout)

	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			if err := sessionRepo.PurgeExpired(context.Background()); err != nil {
				logger.Error("session purge failed", "err", err)
			}
		}
	}()

	addr := fmt.Sprintf(":%d", cfg.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  120 * time.Second,
	}

	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("server starting", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	<-shutdownCh
	logger.Info("shutting down...")

	broker.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server shutdown error", "err", err)
	}
	mediaPool.Shutdown(ctx)

	logger.Info("shutdown complete")
}

func loadMigrations(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("read migrations dir: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".sql") {
			names = append(names, entry.Name())
		}
	}

	sort.Strings(names)

	var b strings.Builder
	for _, name := range names {
		path := filepath.Join(dir, name)

		sqlBytes, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read migration %s: %w", name, err)
		}

		b.WriteString("\n-- migration: ")
		b.WriteString(name)
		b.WriteString("\n")
		b.Write(sqlBytes)
		b.WriteString("\n")
	}

	return b.String(), nil
}

func parseTemplates() (*template.Template, error) {
	funcMap := template.FuncMap{
		"formatSize":  formatSize,
		"fileURL":     fileURL,
		"queryEscape": queryEscape,
	}

	tmpl, err := template.New("").Funcs(funcMap).ParseGlob("web/template/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	tmpl, err = tmpl.ParseGlob("web/template/partials/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse partials: %w", err)
	}

	return tmpl, nil
}

func formatSize(bytes int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)

	switch {
	case bytes >= gb:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(gb))
	case bytes >= mb:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func fileURL(name string) string {
	return "/api/files/" + url.PathEscape(name)
}

func queryEscape(value string) string {
	return url.QueryEscape(value)
}
