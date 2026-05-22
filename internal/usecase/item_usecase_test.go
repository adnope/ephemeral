package usecase

import "testing"

func TestHLSUploadDir(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		playlist string
		want     string
	}{
		{name: "playlist", playlist: "hls/sample/index.m3u8", want: "hls/sample"},
		{name: "empty", playlist: "", want: ""},
		{name: "not hls", playlist: "thumbs/sample.jpg", want: ""},
		{name: "hls root", playlist: "hls/index.m3u8", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := hlsUploadDir(tt.playlist); got != tt.want {
				t.Fatalf("hlsUploadDir(%q) = %q, want %q", tt.playlist, got, tt.want)
			}
		})
	}
}
