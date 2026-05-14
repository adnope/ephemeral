package handler

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	mediapkg "github.com/adnope/leandrop/internal/media"
	"github.com/adnope/leandrop/internal/sse"
	"github.com/adnope/leandrop/internal/store"
)

// Upload handles multipart file uploads.
// POST /upload
func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	// maxMemory=32MB: threshold for spilling part headers to temp file.
	// File body remains a streaming io.Reader; never loaded into heap.
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file field", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Create temp file in uploads dir; atomic rename after write completes
	uploadDir := filepath.Join(h.dataDir, "uploads")
	tmpFile, err := os.CreateTemp(uploadDir, "upload-*")
	if err != nil {
		h.log.Error("upload: create temp", "err", err)
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}

	// Cleanup on failure; removed after successful rename
	tmpPath := tmpFile.Name()
	success := false
	defer func() {
		tmpFile.Close()
		if !success {
			os.Remove(tmpPath)
		}
	}()

	// Zero-copy streaming: io.Copy uses 32KB goroutine-stack buffer
	written, err := io.Copy(tmpFile, file)
	if err != nil {
		h.log.Error("upload: copy", "err", err)
		http.Error(w, "upload failed", http.StatusInternalServerError)
		return
	}
	tmpFile.Close()

	// Generate unique filename: timestamp_originalname
	ts := time.Now().UnixMilli()
	safeFilename := filepath.Base(header.Filename)
	finalName := fmt.Sprintf("%d_%s", ts, safeFilename)
	finalPath := filepath.Join(uploadDir, finalName)

	if err := os.Rename(tmpPath, finalPath); err != nil {
		h.log.Error("upload: rename", "err", err)
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	success = true

	// Sniff MIME type
	mime, err := mediapkg.SniffMIME(finalPath)
	if err != nil {
		mime = "application/octet-stream"
	}

	itemType := mediapkg.ItemTypeFromMIME(mime)

	item := &store.Item{
		Type:     itemType,
		Content:  finalName, // relative path within uploads/
		Filename: safeFilename,
		Filesize: written,
		Metadata: store.Metadata{MIME: mime},
	}

	id, err := h.store.Create(r.Context(), item)
	if err != nil {
		h.log.Error("upload: db create", "err", err)
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}

	// Instant SSE broadcast: client sees the item immediately
	h.broker.Broadcast(sse.Event{Type: "item:new", ID: id})

	// Async metadata extraction: enriches the record within seconds
	h.media.Enqueue(mediapkg.Job{
		ItemID:   id,
		FilePath: finalPath,
		MIMEType: mime,
	})

	// Return the new item partial for HTMX swap
	item.ID = id
	item.CreatedAt = time.Now().UTC()
	if err := h.tmpl.ExecuteTemplate(w, "item_partial", item); err != nil {
		h.log.Error("upload: render", "err", err)
	}
}
