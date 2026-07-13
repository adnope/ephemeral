package httpdelivery

import (
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
	log      *slog.Logger
	settings HandlerSettings
	uploads  chan struct{}
}

type HandlerSettings struct {
	ChatPageSize        int
	HistoryPageSize     int
	SearchResultLimit   int
	MaxUploadBytes      int64
	TextPreviewMaxBytes int64
	UploadConcurrency   int
	CookieSecure        bool
}

func NewHandler(
	itemUseCase *usecase.ItemUseCase,
	historyUseCase *usecase.HistoryUseCase,
	authUseCase *usecase.AuthUseCase,
	events EventStream,
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
		log:      log,
		settings: settings,
		uploads:  make(chan struct{}, boundedUploadConcurrency(settings.UploadConcurrency)),
	}
}

func boundedUploadConcurrency(value int) int {
	if value <= 0 {
		return 1
	}
	if value > 10 {
		return 10
	}
	return value
}
