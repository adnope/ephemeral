package media

import (
	"image"
	"os"

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
