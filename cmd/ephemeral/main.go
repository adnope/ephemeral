package main

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
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
	publicLinkRepo := sqlite.NewPublicLinkRepository(db)
	sessionRepo := sqlite.NewSessionRepository(db)
	userRepo := sqlite.NewUserRepository(db)

	broker := sse.NewBroker()

	uploadStorage := filesystem.NewUploadStorage(cfg.DataDir)
	mediaClassifier := media.NewClassifier()
	mediaPool, err := media.NewPool(itemRepo, broker, uploadStorage, media.PoolOptions{
		WorkerCount:    cfg.MediaWorkerCount,
		ProcessTimeout: cfg.MediaProcessTimeout,
		HLSMinBytes:    cfg.HLSMinBytes,
		HLSMinDuration: cfg.HLSMinDuration,
	})
	if err != nil {
		logger.Error("media pool init failed", "err", err)
		os.Exit(1)
	}
	searchIndexer := search.NewIndexer(db, cfg.DataDir, cfg.BodyIndexMaxBytes, logger)

	itemUseCase := usecase.NewItemUseCase(
		itemRepo,
		publicLinkRepo,
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
		logger,
		httpdelivery.HandlerSettings{
			ChatPageSize:        cfg.ChatPageSize,
			HistoryPageSize:     cfg.HistoryPageSize,
			SearchResultLimit:   cfg.SearchResultLimit,
			MaxUploadBytes:      cfg.MaxUploadBytes,
			TextPreviewMaxBytes: cfg.TextPreviewMaxBytes,
			UploadConcurrency:   cfg.UploadConcurrency,
			CookieSecure:        cfg.CookieSecure,
		},
	)

	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	r.Use(mw.TrustedRealIP(cfg.TrustedProxies))
	r.Use(mw.SecurityHeaders)
	r.Use(mw.RequestLogger(logger))
	r.Use(mw.RateLimit(1_000_000_000, time.Minute))
	r.Use(mw.SessionAuth(sessionRepo, cfg.SessionTTL, cfg.CookieSecure))

	staticSubFS, err := fs.Sub(web.FS, "static")
	if err != nil {
		logger.Error("static fs init failed", "err", err)
		os.Exit(1)
	}

	staticFS := http.FileServer(http.FS(staticSubFS))
	r.Handle("/static/*", http.StripPrefix("/static/", staticFS))
	distSubFS, err := fs.Sub(web.FS, "dist")
	if err != nil {
		logger.Error("spa fs init failed", "err", err)
		os.Exit(1)
	}
	spaIndex, err := fs.ReadFile(distSubFS, "index.html")
	if err != nil {
		logger.Error("spa index read failed", "err", err)
		os.Exit(1)
	}
	spa := spaHandler(spaIndex)
	assetsFS := http.FileServer(http.FS(distSubFS))
	r.Handle("/assets/*", immutableAssets(assetsFS))

	r.Get("/", spa)
	r.Get("/history", spa)
	r.Get("/login", spa)

	r.Get("/api/auth/state", h.AuthState)
	r.Get("/api/config", h.Config)
	r.Get("/api/events", h.Events)
	r.Get("/api/items", h.Items)
	r.Get("/api/items/{id}", h.Item)
	r.Get("/api/items/download-zip", h.DownloadZip)
	r.Get("/api/history", h.HistoryAPI)
	r.Post("/api/upload", h.Upload)
	r.Post("/api/message", h.Message)
	r.Delete("/api/items/{id}", h.DeleteItem)
	r.Get("/api/items/{id}/public-link", h.PublicLinkStatus)
	r.Post("/api/items/{id}/public-link", h.CreatePublicLink)
	r.Delete("/api/items/{id}/public-link", h.RevokePublicLink)
	r.Get("/api/files/*", h.ServeFile)
	r.Get("/api/file-preview/{id}", h.PreviewFile)
	r.Post("/api/login", h.Login)
	r.Post("/api/logout", h.Logout)
	r.Get("/api/share/{token}", h.PublicShareAPI)
	r.Get("/share/{token}/file", h.PublicShareFile)
	r.Get("/share/{token}/download", h.PublicShareDownload)
	r.Get("/share/{token}/thumb", h.PublicShareThumb)
	r.Get("/share/{token}", spa)

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
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 30 * time.Second,
		IdleTimeout:       120 * time.Second,
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
