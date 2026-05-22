package search

import "testing"

func TestMetadataJSONScanIncludesVideoPlaybackFields(t *testing.T) {
	var meta metadataJSON
	if err := meta.Scan([]byte(`{
		"width": 1920,
		"height": 1080,
		"duration": "12m34s",
		"mime": "video/x-matroska",
		"thumb": "thumbs/video.jpg",
		"playback": "playback/video.mp4",
		"playbackMime": "video/mp4",
		"hls": "hls/video/index.m3u8",
		"processing": true
	}`)); err != nil {
		t.Fatalf("Scan(): %v", err)
	}

	got := meta.toDomain()
	if got.Playback != "playback/video.mp4" {
		t.Fatalf("Playback = %q", got.Playback)
	}
	if got.PlaybackMIME != "video/mp4" {
		t.Fatalf("PlaybackMIME = %q", got.PlaybackMIME)
	}
	if got.HLS != "hls/video/index.m3u8" {
		t.Fatalf("HLS = %q", got.HLS)
	}
	if !got.Processing {
		t.Fatal("Processing = false, want true")
	}
}
