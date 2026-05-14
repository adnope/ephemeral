package handler

import (
	"html/template"
	"log/slog"

	"github.com/adnope/ephemeral/internal/bodyindex"
	"github.com/adnope/ephemeral/internal/media"
	"github.com/adnope/ephemeral/internal/sse"
	"github.com/adnope/ephemeral/internal/store"
)

type Handler struct {
	store     store.ItemRepository
	sessions  store.SessionRepository
	users     store.UserRepository
	broker    *sse.Broker
	media     *media.Pool
	tmpl      *template.Template
	dataDir   string
	log       *slog.Logger
	bodyIndex *bodyindex.Indexer
}

func NewHandler(
	itemRepo store.ItemRepository,
	sessionRepo store.SessionRepository,
	userRepo store.UserRepository,
	broker *sse.Broker,
	mediaPool *media.Pool,
	bodyIndexer *bodyindex.Indexer,
	tmpl *template.Template,
	dataDir string,
	log *slog.Logger,
) *Handler {
	return &Handler{
		store:     itemRepo,
		sessions:  sessionRepo,
		users:     userRepo,
		broker:    broker,
		media:     mediaPool,
		bodyIndex: bodyIndexer,
		tmpl:      tmpl,
		dataDir:   dataDir,
		log:       log,
	}
}
