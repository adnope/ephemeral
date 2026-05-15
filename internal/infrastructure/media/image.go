package media

import (
	"context"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"os"
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

	source, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = source.Close() }()

	src, _, err := image.Decode(source)
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	dst := resizeToMaxWidth(src, 640)
	thumbPath, thumbRelPath, err := thumbnailPaths(path)
	if err != nil {
		return "", err
	}

	out, err := os.Create(thumbPath)
	if err != nil {
		return "", err
	}
	success := false
	defer func() {
		_ = out.Close()
		if !success {
			_ = os.Remove(thumbPath)
		}
	}()
	if err := ctx.Err(); err != nil {
		return "", err
	}

	if err := jpeg.Encode(out, dst, &jpeg.Options{Quality: 82}); err != nil {
		return "", err
	}
	if err := out.Sync(); err != nil {
		return "", err
	}

	success = true
	return thumbRelPath, nil
}

func resizeToMaxWidth(src image.Image, maxWidth int) image.Image {
	bounds := src.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 || width <= maxWidth {
		return src
	}

	dstWidth := maxWidth
	dstHeight := (height * dstWidth) / width
	if dstHeight <= 0 {
		dstHeight = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, dstWidth, dstHeight))
	for y := 0; y < dstHeight; y++ {
		srcY := bounds.Min.Y + (y*height)/dstHeight
		for x := 0; x < dstWidth; x++ {
			srcX := bounds.Min.X + (x*width)/dstWidth
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}
	return dst
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
