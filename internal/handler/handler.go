package handler

import (
	"html/template"
	"log/slog"

	"github.com/adnope/leandrop/internal/media"
	"github.com/adnope/leandrop/internal/sse"
	"github.com/adnope/leandrop/internal/store"
)

// Handler is the single dependency container passed to all route handlers.
// Constructed once in main.go and never mutated after startup.
type Handler struct {
	store    store.ItemRepository
	sessions store.SessionRepository
	users    store.UserRepository
	broker   *sse.Broker
	media    *media.Pool
	tmpl     *template.Template
	dataDir  string
	log      *slog.Logger
}

// NewHandler creates a Handler with all dependencies injected.
func NewHandler(
	itemRepo store.ItemRepository,
	sessionRepo store.SessionRepository,
	userRepo store.UserRepository,
	broker *sse.Broker,
	mediaPool *media.Pool,
	tmpl *template.Template,
	dataDir string,
	log *slog.Logger,
) *Handler {
	return &Handler{
		store:    itemRepo,
		sessions: sessionRepo,
		users:    userRepo,
		broker:   broker,
		media:    mediaPool,
		tmpl:     tmpl,
		dataDir:  dataDir,
		log:      log,
	}
}
