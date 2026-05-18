package media

import (
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGenerateImageThumbnail(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed")
	}

	uploadDir := t.TempDir()
	imagePath := filepath.Join(uploadDir, "sample.jpg")
	writeTestJPEG(t, imagePath, 1200, 600)

	relPath, err := generateImageThumbnail(context.Background(), imagePath)
	if err != nil {
		t.Fatalf("generateImageThumbnail(): %v", err)
	}
	if relPath != "thumbs/sample_thumb.jpg" {
		t.Fatalf("relPath = %q, want thumbs/sample_thumb.jpg", relPath)
	}

	thumbPath := filepath.Join(uploadDir, filepath.FromSlash(relPath))
	file, err := os.Open(thumbPath)
	if err != nil {
		t.Fatalf("open thumbnail: %v", err)
	}
	defer func() { _ = file.Close() }()

	cfg, err := jpeg.DecodeConfig(file)
	if err != nil {
		t.Fatalf("decode thumbnail config: %v", err)
	}
	if cfg.Width != 640 || cfg.Height != 320 {
		t.Fatalf("thumbnail size = %dx%d, want 640x320", cfg.Width, cfg.Height)
	}
}

func writeTestJPEG(t *testing.T, path string, width int, height int) {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.Set(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 128, A: 255})
		}
	}

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create test image: %v", err)
	}
	defer func() { _ = file.Close() }()

	if err := jpeg.Encode(file, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode test image: %v", err)
	}
}
