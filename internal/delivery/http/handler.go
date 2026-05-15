package httpdelivery

import (
	"html/template"
	"log/slog"
	"net/http"

	"github.com/adnope/ephemeral/internal/usecase"
)

type EventStream interface {
	ServeSSE(w http.ResponseWriter, r *http.Request)
}

type Handler struct {
	items    *usecase.ItemUseCase
	history  *usecase.HistoryUseCase
	auth     *usecase.AuthUseCase
	events   EventStream
	tmpl     *template.Template
	log      *slog.Logger
	settings HandlerSettings
}

type HandlerSettings struct {
	ChatPageSize        int
	HistoryPageSize     int
	SearchResultLimit   int
	MaxUploadBytes      int64
	TextPreviewMaxBytes int64
	UploadConcurrency   int
}

func NewHandler(
	itemUseCase *usecase.ItemUseCase,
	historyUseCase *usecase.HistoryUseCase,
	authUseCase *usecase.AuthUseCase,
	events EventStream,
	tmpl *template.Template,
	log *slog.Logger,
	settings HandlerSettings,
) *Handler {
	if log == nil {
		log = slog.Default()
	}

	return &Handler{
		items:    itemUseCase,
		history:  historyUseCase,
		auth:     authUseCase,
		events:   events,
		tmpl:     tmpl,
		log:      log,
		settings: settings,
	}
}
