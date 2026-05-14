package handler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	mediapkg "github.com/adnope/ephemeral/internal/media"
	"github.com/adnope/ephemeral/internal/sse"
	"github.com/adnope/ephemeral/internal/store"
)

const maxUploadBytes = 2 << 30 // 2 GiB

// POST /api/upload
func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)

	reader, err := r.MultipartReader()
	if err != nil {
		http.Error(w, "invalid multipart form", http.StatusBadRequest)
		return
	}

	uploadDir := filepath.Join(h.dataDir, "uploads")

	var (
		originalName string
		finalName    string
		finalPath    string
		written      int64
		foundFile    bool
	)

	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			http.Error(w, "invalid multipart data", http.StatusBadRequest)
			return
		}

		if part.FormName() != "file" {
			_ = part.Close()
			continue
		}

		foundFile = true
		originalName = filepath.Base(part.FileName())
		if originalName == "." || originalName == "" {
			_ = part.Close()
			http.Error(w, "missing filename", http.StatusBadRequest)
			return
		}

		tmpFile, err := os.CreateTemp(uploadDir, "upload-*")
		if err != nil {
			_ = part.Close()
			h.log.Error("upload: create temp", "err", err)
			http.Error(w, "storage error", http.StatusInternalServerError)
			return
		}

		tmpPath := tmpFile.Name()
		success := false

		func() {
			defer func() {
				_ = part.Close()
				_ = tmpFile.Close()
				if !success {
					_ = os.Remove(tmpPath)
				}
			}()

			written, err = io.Copy(tmpFile, part)
			if err != nil {
				h.log.Error("upload: copy", "err", err)
				return
			}

			if err = tmpFile.Sync(); err != nil {
				h.log.Error("upload: sync temp", "err", err)
				return
			}

			finalName = fmt.Sprintf("%d_%s", time.Now().UnixMilli(), originalName)
			finalPath = filepath.Join(uploadDir, finalName)

			if err = os.Rename(tmpPath, finalPath); err != nil {
				h.log.Error("upload: rename", "err", err)
				return
			}

			success = true
		}()

		if err != nil {
			http.Error(w, "upload failed", http.StatusInternalServerError)
			return
		}

		break
	}

	if !foundFile {
		http.Error(w, "missing file field", http.StatusBadRequest)
		return
	}

	mime, err := mediapkg.SniffMIME(finalPath)
	if err != nil {
		mime = "application/octet-stream"
	}

	itemType := mediapkg.ItemTypeFromMIME(mime)

	item := &store.Item{
		Type:     itemType,
		Content:  finalName,
		Filename: originalName,
		Filesize: written,
		Metadata: store.Metadata{MIME: mime},
	}

	id, err := h.store.Create(r.Context(), item)
	if err != nil {
		h.log.Error("upload: db create", "err", err)
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := h.bodyIndex.IndexUploadedFile(
			ctx,
			id,
			finalName,
			originalName,
			written,
			store.Metadata{MIME: mime},
		); err != nil {
			h.log.Warn("upload: body index failed", "item_id", id, "err", err)
		}
	}()

	h.broker.Broadcast(sse.Event{Type: "item:new", ID: id})

	h.media.Enqueue(mediapkg.Job{
		ItemID:   id,
		FilePath: finalPath,
		MIMEType: mime,
	})

	item.ID = id
	item.CreatedAt = time.Now().UTC()
	if err := h.tmpl.ExecuteTemplate(w, "item_partial", item); err != nil {
		h.log.Error("upload: render", "err", err)
	}
}
