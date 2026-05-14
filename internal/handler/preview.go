package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
)

const maxTextPreviewBytes = 10 << 20 // 10 MiB

type filePreviewResponse struct {
	ID          int64  `json:"id"`
	Filename    string `json:"filename"`
	MIME        string `json:"mime"`
	Language    string `json:"language"`
	Content     string `json:"content"`
	Filesize    int64  `json:"filesize"`
	CreatedAt   string `json:"created_at"`
	DownloadURL string `json:"download_url"`
}

func (h *Handler) PreviewFile(w http.ResponseWriter, r *http.Request) {
	rawID := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid item id", http.StatusBadRequest)
		return
	}

	item, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	if item.Type != "file" {
		http.Error(w, "preview only supports generic files", http.StatusUnsupportedMediaType)
		return
	}

	if !isPreviewableTextFile(item.Filename, item.Metadata.MIME) {
		http.Error(w, "file type is not previewable as text", http.StatusUnsupportedMediaType)
		return
	}

	if item.Filesize > maxTextPreviewBytes {
		http.Error(w, "file too large for preview", http.StatusRequestEntityTooLarge)
		return
	}

	absPath, err := h.safeUploadPath(item.Content)
	if err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	content, err := readTextPreview(absPath, maxTextPreviewBytes)
	if err != nil {
		h.log.Error("preview: read file", "item_id", id, "err", err)
		http.Error(w, "preview failed", http.StatusInternalServerError)
		return
	}

	if !utf8.Valid(content) {
		http.Error(w, "file is not valid utf-8 text", http.StatusUnsupportedMediaType)
		return
	}

	resp := filePreviewResponse{
		ID:          item.ID,
		Filename:    item.Filename,
		MIME:        item.Metadata.MIME,
		Language:    previewLanguage(item.Filename, item.Metadata.MIME),
		Content:     string(content),
		Filesize:    item.Filesize,
		CreatedAt:   item.CreatedAt.Format("Jan 2, 2006 3:04 PM"),
		DownloadURL: "/api/files/" + url.PathEscape(item.Content),
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.log.Error("preview: encode response", "item_id", id, "err", err)
	}
}

func (h *Handler) safeUploadPath(content string) (string, error) {
	cleanPath := filepath.Clean(content)
	if cleanPath == "." || filepath.IsAbs(cleanPath) || strings.Contains(cleanPath, "..") {
		return "", fmt.Errorf("unsafe upload path")
	}
	return filepath.Join(h.dataDir, "uploads", cleanPath), nil
}

func readTextPreview(path string, maxBytes int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open preview file: %w", err)
	}
	defer func() { _ = file.Close() }()

	body, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read preview file: %w", err)
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("preview exceeds max size")
	}

	return body, nil
}

func isPreviewableTextFile(filename string, mimeType string) bool {
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	if i := strings.IndexByte(mimeType, ';'); i >= 0 {
		mimeType = mimeType[:i]
	}

	if strings.HasPrefix(mimeType, "text/") {
		return true
	}

	switch mimeType {
	case "application/json",
		"application/xml",
		"application/javascript",
		"application/x-javascript",
		"application/x-sh",
		"application/sql",
		"image/svg+xml":
		return true
	}

	_, ok := previewLangByExt[strings.ToLower(filepath.Ext(filename))]
	return ok
}

func previewLanguage(filename string, mimeType string) string {
	if lang, ok := previewLangByExt[strings.ToLower(filepath.Ext(filename))]; ok {
		return lang
	}

	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	if i := strings.IndexByte(mimeType, ';'); i >= 0 {
		mimeType = mimeType[:i]
	}

	if lang, ok := previewLangByMIME[mimeType]; ok {
		return lang
	}

	return "plaintext"
}

var previewLangByExt = map[string]string{
	".txt":        "plaintext",
	".log":        "plaintext",
	".md":         "markdown",
	".markdown":   "markdown",
	".mk":         "make",
	".mak":        "make",
	".make":       "make",
	".go":         "go",
	".py":         "python",
	".js":         "javascript",
	".mjs":        "javascript",
	".cjs":        "javascript",
	".ts":         "typescript",
	".tsx":        "tsx",
	".jsx":        "jsx",
	".json":       "json",
	".yaml":       "yaml",
	".yml":        "yaml",
	".toml":       "toml",
	".xml":        "xml",
	".html":       "html",
	".css":        "css",
	".scss":       "scss",
	".csv":        "csv",
	".sql":        "sql",
	".sh":         "shellscript",
	".bash":       "shellscript",
	".zsh":        "shellscript",
	".rs":         "rust",
	".c":          "c",
	".h":          "c",
	".cpp":        "cpp",
	".hpp":        "cpp",
	".java":       "java",
	".kt":         "kotlin",
	".rb":         "ruby",
	".php":        "php",
	".lua":        "lua",
	".dockerfile": "dockerfile",
}

var previewLangByMIME = map[string]string{
	"text/plain":               "plaintext",
	"text/markdown":            "markdown",
	"text/javascript":          "javascript",
	"text/typescript":          "typescript",
	"text/yaml":                "yaml",
	"text/html":                "html",
	"text/css":                 "css",
	"text/csv":                 "csv",
	"text/xml":                 "xml",
	"application/json":         "json",
	"application/xml":          "xml",
	"application/javascript":   "javascript",
	"application/x-javascript": "javascript",
	"application/sql":          "sql",
	"application/x-sh":         "shellscript",
	"image/svg+xml":            "xml",
}

func init() {
	_ = time.RFC3339
}
