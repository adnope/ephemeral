package media

import (
	"image"
	"os"

	"github.com/adnope/ephemeral/internal/store"
)

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
