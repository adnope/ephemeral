package media

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/adnope/ephemeral/internal/store"
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

func (p *ffprobeOutput) toMetadata(mimeType string) store.Metadata {
	meta := store.Metadata{MIME: mimeType}

	for _, s := range p.Streams {
		if s.Codec == "" || s.Codec == "video" {
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

// extractVideoMeta runs ffprobe to extract only video dimensions and duration.
func extractVideoMeta(ctx context.Context, path string, mimeType string) (store.Metadata, error) {
	args := []string{
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=codec_type,width,height:format=duration",
		"-of", "json",
		path,
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

	return probe.toMetadata(mimeType), nil
}

// generateThumbnail creates a bounded JPEG thumbnail from the first video stream.
// Output: {path_without_ext}_thumb.jpg
func generateThumbnail(ctx context.Context, path string) error {
	ext := filepath.Ext(path)
	thumbPath := strings.TrimSuffix(path, ext) + "_thumb.jpg"

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
		"-vf", "scale='min(640,iw)':-2",
		"-q:v", "10",
		"-y",
		thumbPath,
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg thumbnail: %w", err)
	}
	return nil
}
