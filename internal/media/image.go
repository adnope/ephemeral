package media

import (
	"image"
	// Register decoders for common image formats
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"

	"github.com/adnope/ephemeral/internal/store"
)

// extractImageMeta reads only the image header to extract dimensions.
// image.DecodeConfig never allocates the full pixel buffer.
func extractImageMeta(path string) (store.Metadata, error) {
	f, err := os.Open(path)
	if err != nil {
		return store.Metadata{}, err
	}
	defer func() { _ = f.Close() }()

	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		return store.Metadata{}, err
	}

	return store.Metadata{
		Width:  cfg.Width,
		Height: cfg.Height,
		MIME:   "image/" + format,
	}, nil
}
