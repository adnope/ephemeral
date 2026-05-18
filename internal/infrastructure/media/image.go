package media

import (
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/adnope/ephemeral/internal/domain"
)

func extractImageMeta(path string) (domain.Metadata, error) {
	file, err := os.Open(path)
	if err != nil {
		return domain.Metadata{}, err
	}
	defer func() { _ = file.Close() }()

	cfg, format, err := image.DecodeConfig(file)
	if err != nil {
		return domain.Metadata{}, err
	}

	return domain.Metadata{
		Width:  cfg.Width,
		Height: cfg.Height,
		MIME:   "image/" + format,
	}, nil
}

func generateImageThumbnail(ctx context.Context, path string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	thumbPath, thumbRelPath, err := thumbnailPaths(path)
	if err != nil {
		return "", err
	}

	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-nostdin",
		"-i", path,
		"-map", "0:v:0",
		"-an",
		"-sn",
		"-dn",
		"-frames:v", "1",
		"-vf", "scale='min(640,iw)':-2:flags=lanczos",
		"-q:v", "3",
		"-y",
		thumbPath,
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	if err := cmd.Run(); err != nil {
		_ = os.Remove(thumbPath)
		return "", fmt.Errorf("ffmpeg image thumbnail: %w", err)
	}

	return thumbRelPath, nil
}

func thumbnailPaths(path string) (string, string, error) {
	ext := filepath.Ext(path)
	baseName := filepath.Base(strings.TrimSuffix(path, ext))

	thumbName := baseName + "_thumb.jpg"
	thumbDir := filepath.Join(filepath.Dir(path), "thumbs")
	if err := os.MkdirAll(thumbDir, 0o755); err != nil {
		return "", "", err
	}

	return filepath.Join(thumbDir, thumbName), filepath.ToSlash(filepath.Join("thumbs", thumbName)), nil
}
