package media

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCanRemuxToBrowserMP4(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		info videoInfo
		want bool
	}{
		{
			name: "h264 aac",
			info: videoInfo{VideoCodec: "h264", VideoPixFmt: "yuv420p", AudioCodecs: []string{"aac"}},
			want: true,
		},
		{
			name: "h264 opus needs transcode",
			info: videoInfo{VideoCodec: "h264", VideoPixFmt: "yuv420p", AudioCodecs: []string{"opus"}},
			want: false,
		},
		{
			name: "av1 needs transcode",
			info: videoInfo{VideoCodec: "av1", VideoPixFmt: "yuv420p", AudioCodecs: []string{"aac"}},
			want: false,
		},
		{
			name: "h264 non yuv420p needs transcode",
			info: videoInfo{VideoCodec: "h264", VideoPixFmt: "yuv420p10le", AudioCodecs: []string{"aac"}},
			want: false,
		},
		{
			name: "silent h264",
			info: videoInfo{VideoCodec: "h264", VideoPixFmt: "yuv420p"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := canRemuxToBrowserMP4(tt.info); got != tt.want {
				t.Fatalf("canRemuxToBrowserMP4() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldGenerateHLS(t *testing.T) {
	t.Parallel()

	options := PoolOptions{
		HLSMinBytes:    100 << 20,
		HLSMinDuration: 5 * time.Minute,
	}

	tests := []struct {
		name     string
		size     int64
		duration time.Duration
		options  PoolOptions
		want     bool
	}{
		{name: "below thresholds", size: 10 << 20, duration: time.Minute, options: options, want: false},
		{name: "size threshold", size: 100 << 20, duration: time.Minute, options: options, want: true},
		{name: "duration threshold", size: 10 << 20, duration: 5 * time.Minute, options: options, want: true},
		{name: "all videos", size: 1, duration: time.Second, options: PoolOptions{}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := shouldGenerateHLS(tt.size, tt.duration, tt.options); got != tt.want {
				t.Fatalf("shouldGenerateHLS() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGeneratedMediaPaths(t *testing.T) {
	t.Parallel()

	uploadPath := filepath.Join(t.TempDir(), "1710000000000_sample video.mkv")

	playbackAbs, playbackRel, err := playbackPaths(uploadPath)
	if err != nil {
		t.Fatalf("playbackPaths(): %v", err)
	}
	if filepath.Base(playbackAbs) != "1710000000000_sample video_playback.mp4" {
		t.Fatalf("playback abs = %q", playbackAbs)
	}
	if playbackRel != "playback/1710000000000_sample video_playback.mp4" {
		t.Fatalf("playback rel = %q", playbackRel)
	}

	hlsDir, hlsRel, err := hlsPaths(uploadPath)
	if err != nil {
		t.Fatalf("hlsPaths(): %v", err)
	}
	if filepath.Base(hlsDir) != "1710000000000_sample video" {
		t.Fatalf("hls dir = %q", hlsDir)
	}
	if hlsRel != "hls/1710000000000_sample video/index.m3u8" {
		t.Fatalf("hls rel = %q", hlsRel)
	}

	baseURL := hlsSegmentBaseURL(hlsRel)
	if baseURL != "/api/files/hls%2F1710000000000_sample%20video%2F" {
		t.Fatalf("hls base URL = %q", baseURL)
	}
}

func TestFFmpegArgsLimitThreads(t *testing.T) {
	t.Parallel()

	tests := [][]string{
		remuxMP4Args("input.mkv", "output.mp4"),
		transcodeMP4Args("input.mkv", "output.mp4"),
		hlsArgs("input.mp4", "/api/files/hls%2Fsample%2F"),
	}

	for _, args := range tests {
		if !containsThreadLimit(args) {
			t.Fatalf("args missing thread limit: %v", args)
		}
	}
}

func TestGenerateBrowserPlaybackAndHLS(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe is not installed")
	}
	if !ffmpegEncoderAvailable(t, "libx264") {
		t.Skip("ffmpeg libx264 encoder is not available")
	}

	uploadDir := t.TempDir()
	inputPath := filepath.Join(uploadDir, "sample.avi")
	makeTestVideo(t, inputPath)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	info, err := extractVideoInfo(ctx, inputPath, "video/x-msvideo")
	if err != nil {
		t.Fatalf("extractVideoInfo(): %v", err)
	}
	if canRemuxToBrowserMP4(info) {
		t.Fatal("test input unexpectedly remuxable")
	}

	playback, err := generateBrowserPlayback(ctx, inputPath, info)
	if err != nil {
		t.Fatalf("generateBrowserPlayback(): %v", err)
	}
	if playback.MIME != playbackMIME {
		t.Fatalf("playback MIME = %q", playback.MIME)
	}
	if _, err := os.Stat(playback.AbsPath); err != nil {
		t.Fatalf("stat playback: %v", err)
	}

	hls, err := generateHLS(ctx, inputPath, playback.AbsPath)
	if err != nil {
		t.Fatalf("generateHLS(): %v", err)
	}
	if hls.MIME != hlsMIME {
		t.Fatalf("hls MIME = %q", hls.MIME)
	}

	playlist, err := os.ReadFile(hls.AbsPath)
	if err != nil {
		t.Fatalf("read hls playlist: %v", err)
	}
	if !strings.Contains(string(playlist), "/api/files/hls%2Fsample%2Fsegment_") {
		t.Fatalf("playlist does not contain escaped segment URLs:\n%s", playlist)
	}
}

func containsThreadLimit(args []string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "-threads" && args[i+1] == ffmpegThreads {
			return true
		}
	}
	return false
}

func makeTestVideo(t *testing.T, path string) {
	t.Helper()

	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-f", "lavfi",
		"-i", "testsrc=size=64x64:duration=1:rate=10",
		"-c:v", "mpeg4",
		"-y",
		path,
	}
	cmd := exec.Command("ffmpeg", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create test video: %v: %s", err, strings.TrimSpace(string(output)))
	}
}

func ffmpegEncoderAvailable(t *testing.T, encoder string) bool {
	t.Helper()

	cmd := exec.Command("ffmpeg", "-hide_banner", "-encoders")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("list ffmpeg encoders: %v", err)
	}
	return strings.Contains(string(output), encoder)
}
