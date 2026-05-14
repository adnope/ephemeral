package media

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/adnope/leandrop/internal/store"
)

// ffprobeOutput maps the JSON output of ffprobe.
type ffprobeOutput struct {
	Streams []ffprobeStream `json:"streams"`
	Format  ffprobeFormat   `json:"format"`
}

type ffprobeStream struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Codec  string `json:"codec_type"`
}

type ffprobeFormat struct {
	Duration string `json:"duration"`
}

func (p *ffprobeOutput) toMetadata() store.Metadata {
	meta := store.Metadata{MIME: "video/mp4"}

	for _, s := range p.Streams {
		if s.Codec == "video" {
			meta.Width = s.Width
			meta.Height = s.Height
			break
		}
	}

	if p.Format.Duration != "" {
		secs, err := strconv.ParseFloat(p.Format.Duration, 64)
		if err == nil {
			mins := int(secs) / 60
			remainSecs := int(secs) % 60
			meta.Duration = fmt.Sprintf("%02d:%02d", mins, remainSecs)
		}
	}

	return meta
}

// extractVideoMeta runs ffprobe to extract video dimensions and duration.
func extractVideoMeta(ctx context.Context, path string) (store.Metadata, error) {
	args := []string{
		"-v", "quiet", "-print_format", "json",
		"-show_streams", "-show_format", path,
	}
	cmd := exec.CommandContext(ctx, "ffprobe", args...)
	out, err := cmd.Output()
	if err != nil {
		return store.Metadata{}, fmt.Errorf("ffprobe: %w", err)
	}

	var probe ffprobeOutput
	if err := json.Unmarshal(out, &probe); err != nil {
		return store.Metadata{}, fmt.Errorf("ffprobe unmarshal: %w", err)
	}

	return probe.toMetadata(), nil
}

// generateThumbnail creates a JPEG thumbnail from the first frame of a video.
// Output: {path_without_ext}_thumb.jpg
func generateThumbnail(ctx context.Context, path string) error {
	ext := filepath.Ext(path)
	thumbPath := strings.TrimSuffix(path, ext) + "_thumb.jpg"

	args := []string{
		"-i", path,
		"-vframes", "1",
		"-q:v", "8", // lower quality for thumbnail, saves disk
		"-y",         // overwrite if exists
		thumbPath,
	}
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg thumbnail: %w", err)
	}
	return nil
}
