package media

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	pathpkg "path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/adnope/ephemeral/internal/domain"
)

const (
	ffmpegThreads = "2"
	playbackMIME  = "video/mp4"
	hlsMIME       = "application/vnd.apple.mpegurl"
)

type ffprobeOutput struct {
	Streams []ffprobeStream `json:"streams"`
	Format  ffprobeFormat   `json:"format"`
}

type ffprobeStream struct {
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	CodecType string `json:"codec_type"`
	CodecName string `json:"codec_name"`
	PixFmt    string `json:"pix_fmt"`
}

type ffprobeFormat struct {
	Duration string `json:"duration"`
}

type videoInfo struct {
	Metadata    domain.Metadata
	Duration    time.Duration
	VideoCodec  string
	VideoPixFmt string
	AudioCodecs []string
}

type generatedMedia struct {
	AbsPath string
	RelPath string
	MIME    string
}

func (p *ffprobeOutput) toVideoInfo(mimeType string) videoInfo {
	meta := domain.Metadata{MIME: mimeType}
	info := videoInfo{Metadata: meta}

	for _, stream := range p.Streams {
		switch stream.CodecType {
		case "", "video":
			if info.VideoCodec == "" {
				meta.Width = stream.Width
				meta.Height = stream.Height
				info.VideoCodec = strings.ToLower(stream.CodecName)
				info.VideoPixFmt = strings.ToLower(stream.PixFmt)
			}
		case "audio":
			if stream.CodecName != "" {
				info.AudioCodecs = append(info.AudioCodecs, strings.ToLower(stream.CodecName))
			}
		}
	}

	if p.Format.Duration != "" {
		seconds, err := strconv.ParseFloat(p.Format.Duration, 64)
		if err == nil {
			info.Duration = time.Duration(seconds * float64(time.Second))
			minutes := int(seconds) / 60
			remainingSeconds := int(seconds) % 60
			meta.Duration = fmt.Sprintf("%02d:%02d", minutes, remainingSeconds)
		}
	}

	info.Metadata = meta
	return info
}

func extractVideoInfo(ctx context.Context, path string, mimeType string) (videoInfo, error) {
	args := []string{
		"-v", "error",
		"-show_entries", "stream=codec_type,codec_name,width,height,pix_fmt:format=duration",
		"-of", "json",
		path,
	}

	cmd := exec.CommandContext(ctx, "ffprobe", args...)
	output, err := cmd.Output()
	if err != nil {
		return videoInfo{}, fmt.Errorf("ffprobe: %w", err)
	}

	var probe ffprobeOutput
	if err := json.Unmarshal(output, &probe); err != nil {
		return videoInfo{}, fmt.Errorf("ffprobe unmarshal: %w", err)
	}

	return probe.toVideoInfo(mimeType), nil
}

func generateThumbnail(ctx context.Context, path string) (string, error) {
	thumbPath, thumbRelPath, err := thumbnailPaths(path)
	if err != nil {
		return "", fmt.Errorf("mkdir thumbnail dir: %w", err)
	}

	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-nostdin",
		"-ss", "00:00:01.000",
		"-threads", ffmpegThreads,
		"-i", path,
		"-map", "0:v:0",
		"-an",
		"-sn",
		"-dn",
		"-frames:v", "1",
		"-vf", "scale='min(640,iw)':-2:flags=lanczos",
		"-threads", ffmpegThreads,
		"-q:v", "3",
		"-y",
		thumbPath,
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ffmpeg thumbnail: %w", err)
	}

	return thumbRelPath, nil
}

func generateBrowserPlayback(ctx context.Context, inputPath string, info videoInfo) (generatedMedia, error) {
	outputPath, relPath, err := playbackPaths(inputPath)
	if err != nil {
		return generatedMedia{}, err
	}

	tmpPath := outputPath + ".tmp"
	success := false
	defer func() {
		if !success {
			_ = os.Remove(tmpPath)
		}
	}()

	if canRemuxToBrowserMP4(info) {
		if err := runFFmpeg(ctx, "", remuxMP4Args(inputPath, tmpPath)); err == nil {
			if err := os.Rename(tmpPath, outputPath); err != nil {
				return generatedMedia{}, fmt.Errorf("commit remuxed playback: %w", err)
			}
			success = true
			return generatedMedia{AbsPath: outputPath, RelPath: relPath, MIME: playbackMIME}, nil
		}
		_ = os.Remove(tmpPath)
	}

	if err := runFFmpeg(ctx, "", transcodeMP4Args(inputPath, tmpPath)); err != nil {
		return generatedMedia{}, fmt.Errorf("transcode browser playback: %w", err)
	}
	if err := os.Rename(tmpPath, outputPath); err != nil {
		return generatedMedia{}, fmt.Errorf("commit transcoded playback: %w", err)
	}
	success = true
	return generatedMedia{AbsPath: outputPath, RelPath: relPath, MIME: playbackMIME}, nil
}

func generateHLS(ctx context.Context, originalPath string, inputPath string) (generatedMedia, error) {
	hlsDir, relPath, err := hlsPaths(originalPath)
	if err != nil {
		return generatedMedia{}, err
	}

	parentDir := filepath.Dir(hlsDir)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return generatedMedia{}, fmt.Errorf("mkdir hls parent: %w", err)
	}

	tmpDir, err := os.MkdirTemp(parentDir, filepath.Base(hlsDir)+"-*")
	if err != nil {
		return generatedMedia{}, fmt.Errorf("create hls temp dir: %w", err)
	}
	success := false
	defer func() {
		if !success {
			_ = os.RemoveAll(tmpDir)
		}
	}()

	if err := runFFmpeg(ctx, tmpDir, hlsArgs(inputPath, hlsSegmentBaseURL(relPath))); err != nil {
		return generatedMedia{}, fmt.Errorf("generate hls: %w", err)
	}

	if err := os.RemoveAll(hlsDir); err != nil {
		return generatedMedia{}, fmt.Errorf("replace hls dir: %w", err)
	}
	if err := os.Rename(tmpDir, hlsDir); err != nil {
		return generatedMedia{}, fmt.Errorf("commit hls dir: %w", err)
	}
	success = true
	return generatedMedia{
		AbsPath: filepath.Join(hlsDir, "index.m3u8"),
		RelPath: relPath,
		MIME:    hlsMIME,
	}, nil
}

func canRemuxToBrowserMP4(info videoInfo) bool {
	if info.VideoCodec != "h264" {
		return false
	}
	if info.VideoPixFmt != "yuv420p" {
		return false
	}

	for _, codec := range info.AudioCodecs {
		switch codec {
		case "", "aac", "mp3":
		default:
			return false
		}
	}
	return true
}

func shouldGenerateHLS(sizeBytes int64, duration time.Duration, options PoolOptions) bool {
	if options.HLSMinBytes == 0 && options.HLSMinDuration == 0 {
		return true
	}
	if options.HLSMinBytes > 0 && sizeBytes >= options.HLSMinBytes {
		return true
	}
	if options.HLSMinDuration > 0 && duration >= options.HLSMinDuration {
		return true
	}
	return false
}

func playbackPaths(path string) (string, string, error) {
	ext := filepath.Ext(path)
	baseName := filepath.Base(strings.TrimSuffix(path, ext))
	outputName := baseName + "_playback.mp4"
	outputDir := filepath.Join(filepath.Dir(path), "playback")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", "", fmt.Errorf("mkdir playback dir: %w", err)
	}
	return filepath.Join(outputDir, outputName), filepath.ToSlash(filepath.Join("playback", outputName)), nil
}

func hlsPaths(path string) (string, string, error) {
	ext := filepath.Ext(path)
	baseName := filepath.Base(strings.TrimSuffix(path, ext))
	outputDir := filepath.Join(filepath.Dir(path), "hls", baseName)
	if baseName == "" || baseName == "." {
		return "", "", fmt.Errorf("invalid hls base name")
	}
	return outputDir, filepath.ToSlash(filepath.Join("hls", baseName, "index.m3u8")), nil
}

func remuxMP4Args(inputPath string, outputPath string) []string {
	return []string{
		"-hide_banner",
		"-loglevel", "error",
		"-nostdin",
		"-threads", ffmpegThreads,
		"-i", inputPath,
		"-map", "0:v:0",
		"-map", "0:a?",
		"-c", "copy",
		"-threads", ffmpegThreads,
		"-movflags", "+faststart",
		"-f", "mp4",
		"-y",
		outputPath,
	}
}

func transcodeMP4Args(inputPath string, outputPath string) []string {
	return []string{
		"-hide_banner",
		"-loglevel", "error",
		"-nostdin",
		"-threads", ffmpegThreads,
		"-i", inputPath,
		"-map", "0:v:0",
		"-map", "0:a?",
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-crf", "23",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-b:a", "128k",
		"-threads", ffmpegThreads,
		"-movflags", "+faststart",
		"-f", "mp4",
		"-y",
		outputPath,
	}
}

func hlsArgs(inputPath string, segmentBaseURL string) []string {
	return []string{
		"-hide_banner",
		"-loglevel", "error",
		"-nostdin",
		"-threads", ffmpegThreads,
		"-i", inputPath,
		"-map", "0:v:0",
		"-map", "0:a?",
		"-c", "copy",
		"-threads", ffmpegThreads,
		"-hls_time", "4",
		"-hls_playlist_type", "vod",
		"-hls_base_url", segmentBaseURL,
		"-hls_segment_filename", "segment_%05d.ts",
		"-f", "hls",
		"-y",
		"index.m3u8",
	}
}

func hlsSegmentBaseURL(playlistRelPath string) string {
	dir := pathpkg.Dir(playlistRelPath)
	if dir == "." {
		return ""
	}
	return "/api/files/" + url.PathEscape(dir+"/")
}

func runFFmpeg(ctx context.Context, dir string, args []string) error {
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	if msg := strings.TrimSpace(string(output)); msg != "" {
		return fmt.Errorf("%w: %s", err, truncateCommandOutput(msg))
	}
	return err
}

func truncateCommandOutput(output string) string {
	const maxLen = 2048
	if len(output) <= maxLen {
		return output
	}
	return output[:maxLen] + "...[truncated]"
}
