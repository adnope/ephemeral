package httpdelivery

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/adnope/ephemeral/internal/domain"
	"github.com/adnope/ephemeral/internal/usecase"
	"github.com/go-chi/chi/v5"
)

// ServeFile handles GET /api/files/{path}.
func (h *Handler) ServeFile(w http.ResponseWriter, r *http.Request) {
	relPath := chi.URLParam(r, "*")

	decodedPath, err := url.PathUnescape(relPath)
	if err != nil {
		http.Error(w, "bad file path", http.StatusBadRequest)
		return
	}

	absPath, err := h.items.ResolveUploadPath(decodedPath)
	if err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	setUploadFileHeaders(w, decodedPath)
	http.ServeFile(w, r, absPath)
}

func setUploadFileHeaders(w http.ResponseWriter, relPath string) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if strings.HasPrefix(filepath.ToSlash(relPath), "hls/") {
		switch strings.ToLower(filepath.Ext(relPath)) {
		case ".m3u8":
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		case ".ts":
			w.Header().Set("Content-Type", "video/mp2t")
		}
	}
	if isActiveUploadDocument(relPath) {
		w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'; img-src 'self' data: blob:; media-src 'self' blob:; style-src 'unsafe-inline'")
	}
}

func isActiveUploadDocument(relPath string) bool {
	switch strings.ToLower(filepath.Ext(relPath)) {
	case ".htm", ".html", ".svg", ".xml", ".xhtml":
		return true
	default:
		return false
	}
}

// DownloadZip handles GET /api/items/download-zip.
func (h *Handler) DownloadZip(w http.ResponseWriter, r *http.Request) {
	idsParam := r.URL.Query().Get("ids")
	if idsParam == "" {
		http.Error(w, "missing ids parameter", http.StatusBadRequest)
		return
	}

	var parsedIDs []int64
	for _, part := range strings.Split(idsParam, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil || id <= 0 {
			http.Error(w, "invalid item id", http.StatusBadRequest)
			return
		}
		parsedIDs = append(parsedIDs, id)
	}

	if len(parsedIDs) == 0 {
		http.Error(w, "no item ids provided", http.StatusBadRequest)
		return
	}

	var items []*domain.Item
	for _, id := range parsedIDs {
		item, err := h.items.GetItem(r.Context(), id)
		if err != nil {
			if errors.Is(err, usecase.ErrNotFound) {
				continue
			}
			h.log.Error("download zip: get item", "id", id, "err", err)
			http.Error(w, "failed to retrieve items", http.StatusInternalServerError)
			return
		}
		items = append(items, item)
	}

	if len(items) == 0 {
		http.Error(w, "no valid items found to download", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="ephemeral_download.zip"`)

	zw := zip.NewWriter(w)
	defer func() {
		if err := zw.Close(); err != nil {
			h.log.Warn("download zip: close zip writer failed", "err", err)
		}
	}()

	type fileKey struct {
		name string
		ext  string
	}
	seenNames := make(map[fileKey]int)

	getUniqueName := func(origName string) string {
		origName = strings.TrimSpace(origName)
		if origName == "" {
			origName = "download"
		}

		ext := filepath.Ext(origName)
		base := strings.TrimSuffix(origName, ext)

		key := fileKey{name: base, ext: ext}
		count, exists := seenNames[key]
		if !exists {
			seenNames[key] = 1
			return origName
		}

		seenNames[key] = count + 1
		return fmt.Sprintf("%s (%d)%s", base, count, ext)
	}

	sanitizeName := func(name string) string {
		name = filepath.Base(name)
		name = strings.ReplaceAll(name, "/", "")
		name = strings.ReplaceAll(name, "\\", "")
		return name
	}

	for _, item := range items {
		if item.Type == domain.ItemTypeText {
			zipName := getUniqueName(fmt.Sprintf("message_%d.txt", item.ID))

			header := &zip.FileHeader{
				Name:     zipName,
				Method:   zip.Deflate,
				Modified: item.CreatedAt,
			}

			writer, err := zw.CreateHeader(header)
			if err != nil {
				h.log.Error("download zip: create text header", "id", item.ID, "err", err)
				return
			}

			_, err = io.WriteString(writer, item.Content)
			if err != nil {
				h.log.Error("download zip: write text content", "id", item.ID, "err", err)
				return
			}
		} else {
			absPath, err := h.items.ResolveUploadPath(item.Content)
			if err != nil {
				h.log.Warn("download zip: resolve path forbidden", "id", item.ID, "content", item.Content)
				continue
			}

			file, err := os.Open(absPath)
			if err != nil {
				h.log.Warn("download zip: open file failed", "id", item.ID, "path", absPath, "err", err)
				continue
			}

			origName := sanitizeName(item.Filename)
			zipName := getUniqueName(origName)

			header := &zip.FileHeader{
				Name:     zipName,
				Method:   zip.Deflate,
				Modified: item.CreatedAt,
			}

			writer, err := zw.CreateHeader(header)
			if err != nil {
				_ = file.Close()
				h.log.Error("download zip: create file header", "id", item.ID, "err", err)
				return
			}

			_, err = io.Copy(writer, file)
			_ = file.Close()
			if err != nil {
				h.log.Error("download zip: copy file content", "id", item.ID, "err", err)
				return
			}
		}
	}
}
