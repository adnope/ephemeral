package httpdelivery

import "net/http"

type runtimeConfigResponse struct {
	ChatPageSize        int   `json:"chatPageSize"`
	HistoryPageSize     int   `json:"historyPageSize"`
	MaxUploadSizeBytes  int64 `json:"maxUploadSizeBytes"`
	TextPreviewMaxBytes int64 `json:"textPreviewMaxBytes"`
	UploadConcurrency   int   `json:"uploadConcurrency"`
}

// Config handles GET /api/config.
func (h *Handler) Config(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, runtimeConfigResponse{
		ChatPageSize:        h.settings.ChatPageSize,
		HistoryPageSize:     h.settings.HistoryPageSize,
		MaxUploadSizeBytes:  h.settings.MaxUploadBytes,
		TextPreviewMaxBytes: h.settings.TextPreviewMaxBytes,
		UploadConcurrency:   h.settings.UploadConcurrency,
	})
}
