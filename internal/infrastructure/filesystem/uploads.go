package filesystem

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/adnope/ephemeral/internal/domain"
)

type UploadStorage struct {
	uploadDir string
}

func NewUploadStorage(dataDir string) *UploadStorage {
	return &UploadStorage{
		uploadDir: filepath.Join(dataDir, "uploads"),
	}
}

func (s *UploadStorage) Save(ctx context.Context, file domain.UploadFile) (domain.StoredUpload, error) {
	if file.Reader == nil {
		return domain.StoredUpload{}, fmt.Errorf("upload reader is nil")
	}

	originalName := filepath.Base(file.Name)
	if originalName == "." || originalName == "" {
		return domain.StoredUpload{}, fmt.Errorf("missing filename")
	}

	tmpFile, err := os.CreateTemp(s.uploadDir, "upload-*")
	if err != nil {
		return domain.StoredUpload{}, fmt.Errorf("create upload temp file: %w", err)
	}

	tmpPath := tmpFile.Name()
	success := false
	var written int64
	defer func() {
		_ = tmpFile.Close()
		if !success {
			_ = os.Remove(tmpPath)
		}
	}()

	written, err = copyWithContext(ctx, tmpFile, file.Reader)
	if err != nil {
		return domain.StoredUpload{}, fmt.Errorf("write upload temp file: %w", err)
	}

	if err := tmpFile.Sync(); err != nil {
		return domain.StoredUpload{}, fmt.Errorf("sync upload temp file: %w", err)
	}

	storedName := fmt.Sprintf("%d_%s", time.Now().UnixMilli(), originalName)
	finalPath := filepath.Join(s.uploadDir, storedName)
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return domain.StoredUpload{}, fmt.Errorf("commit upload file: %w", err)
	}

	success = true
	return domain.StoredUpload{
		OriginalName: originalName,
		StoredName:   storedName,
		AbsolutePath: finalPath,
		Size:         written,
	}, nil
}

func (s *UploadStorage) Path(content string) (string, error) {
	cleanPath := filepath.Clean(content)
	if cleanPath == "." || filepath.IsAbs(cleanPath) || strings.Contains(cleanPath, "..") {
		return "", fmt.Errorf("unsafe upload path")
	}
	return filepath.Join(s.uploadDir, cleanPath), nil
}

func (s *UploadStorage) Remove(content string) error {
	path, err := s.Path(content)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove upload file: %w", err)
	}
	return nil
}

func (s *UploadStorage) ReadLimited(content string, maxBytes int64) ([]byte, error) {
	path, err := s.Path(content)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open upload file: %w", err)
	}
	defer func() { _ = file.Close() }()

	body, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read upload file: %w", err)
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("upload file exceeds max read size")
	}

	return body, nil
}

func copyWithContext(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
	buffer := make([]byte, 256*1024)
	var written int64

	for {
		if err := ctx.Err(); err != nil {
			return written, err
		}

		nr, readErr := src.Read(buffer)
		if nr > 0 {
			nw, writeErr := dst.Write(buffer[:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if writeErr != nil {
				return written, writeErr
			}
			if nw != nr {
				return written, io.ErrShortWrite
			}
		}

		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return written, nil
			}
			return written, readErr
		}
	}
}
