package main

import (
	"context"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/adnope/ephemeral/internal/migrations"
	"github.com/adnope/ephemeral/web"

	"github.com/adnope/ephemeral/internal/config"
	httpdelivery "github.com/adnope/ephemeral/internal/delivery/http"
	"github.com/adnope/ephemeral/internal/infrastructure/filesystem"
	"github.com/adnope/ephemeral/internal/infrastructure/media"
	"github.com/adnope/ephemeral/internal/infrastructure/search"
	"github.com/adnope/ephemeral/internal/infrastructure/sqlite"
	"github.com/adnope/ephemeral/internal/infrastructure/sse"
	mw "github.com/adnope/ephemeral/internal/middleware"
	"github.com/adnope/ephemeral/internal/usecase"
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

	migrationSQL, err := loadMigrations(migrations.FS)
	if err != nil {
		logger.Error("migration read failed", "err", err)
		os.Exit(1)
	}

	db, err := sqlite.OpenDB(cfg.DBPath(), string(migrationSQL))
	if err != nil {
		logger.Error("database init failed", "err", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	itemRepo := sqlite.NewItemRepository(db)
	sessionRepo := sqlite.NewSessionRepository(db)
	userRepo := sqlite.NewUserRepository(db)

	broker := sse.NewBroker()

	uploadStorage := filesystem.NewUploadStorage(cfg.DataDir)
	mediaClassifier := media.NewClassifier()
	mediaPool, err := media.NewPool(itemRepo, broker, cfg.MediaWorkerCount)
	if err != nil {
		logger.Error("media pool init failed", "err", err)
		os.Exit(1)
	}
	searchIndexer := search.NewIndexer(db, cfg.DataDir, cfg.BodyIndexMaxBytes, logger)

	tmpl, err := parseTemplates()
	if err != nil {
		logger.Error("template parse failed", "err", err)
		os.Exit(1)
	}

	itemUseCase := usecase.NewItemUseCase(
		itemRepo,
		broker,
		mediaPool,
		searchIndexer,
		uploadStorage,
		mediaClassifier,
		logger,
	)
	historyUseCase := usecase.NewHistoryUseCase(searchIndexer)
	authUseCase := usecase.NewAuthUseCase(userRepo, sessionRepo, cfg.SessionTTL)

	h := httpdelivery.NewHandler(
		itemUseCase,
		historyUseCase,
		authUseCase,
		broker,
		tmpl,
		logger,
		httpdelivery.HandlerSettings{
			ChatPageSize:        cfg.ChatPageSize,
			HistoryPageSize:     cfg.HistoryPageSize,
			SearchResultLimit:   cfg.SearchResultLimit,
			MaxUploadBytes:      cfg.MaxUploadBytes,
			TextPreviewMaxBytes: cfg.TextPreviewMaxBytes,
			UploadConcurrency:   cfg.UploadConcurrency,
		},
	)

	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)

	r.Use(mw.RequestLogger(logger))
	r.Use(mw.RateLimit(100, time.Minute))
	r.Use(mw.SessionAuth(sessionRepo, cfg.SessionTTL))

	staticSubFS, err := fs.Sub(web.FS, "static")
	if err != nil {
		logger.Error("static fs init failed", "err", err)
		os.Exit(1)
	}

	staticFS := http.FileServer(http.FS(staticSubFS))
	r.Handle("/static/*", http.StripPrefix("/static/", staticFS))

	r.Get("/", h.Index)
	r.Get("/history", h.History)
	r.Get("/search", h.SearchItems)
	r.Get("/login", h.LoginPage)

	r.Get("/api/auth/state", h.AuthState)
	r.Get("/api/config", h.Config)
	r.Get("/api/events", h.Events)
	r.Get("/api/items", h.Items)
	r.Get("/api/history", h.HistoryAPI)
	r.Post("/api/upload", h.Upload)
	r.Post("/api/message", h.Message)
	r.Delete("/api/items/{id}", h.DeleteItem)
	r.Get("/api/files/*", h.ServeFile)
	r.Get("/api/file-preview/{id}", h.PreviewFile)
	r.Post("/api/login", h.Login)
	r.Post("/api/logout", h.Logout)

	go func() {
		if err := sessionRepo.PurgeExpired(context.Background()); err != nil {
			logger.Error("session purge failed", "err", err)
		}

		ticker := time.NewTicker(15 * time.Minute)
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

func loadMigrations(fsys fs.FS) (string, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return "", fmt.Errorf("read embedded migrations: %w", err)
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
		sqlBytes, err := fs.ReadFile(fsys, name)
		if err != nil {
			return "", fmt.Errorf("read embedded migration %s: %w", name, err)
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
		"linkifyText": linkifyText,
		"queryEscape": queryEscape,
	}

	tmpl, err := template.New("").Funcs(funcMap).ParseFS(
		web.FS,
		"template/*.html",
		"template/partials/*.html",
	)
	if err != nil {
		return nil, fmt.Errorf("parse embedded templates: %w", err)
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
