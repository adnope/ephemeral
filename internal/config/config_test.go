package config

import (
	"testing"
	"time"
)

func TestParseByteSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want int64
	}{
		{name: "bytes", raw: "1024", want: 1024},
		{name: "binary mib", raw: "10MiB", want: 10 << 20},
		{name: "binary with space", raw: "2 GiB", want: 2 << 30},
		{name: "decimal mb", raw: "1.5MB", want: 1_500_000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseByteSize(tt.raw)
			if err != nil {
				t.Fatalf("parseByteSize(%q): %v", tt.raw, err)
			}
			if got != tt.want {
				t.Fatalf("parseByteSize(%q) = %d, want %d", tt.raw, got, tt.want)
			}
		})
	}
}

func TestLoadRuntimeTuningEnv(t *testing.T) {
	dataDir := t.TempDir()

	t.Setenv("PORT", "9000")
	t.Setenv("DATA_DIR", dataDir)
	t.Setenv("SESSION_TTL", "2h")
	t.Setenv("COOKIE_SECURE", "true")
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1, 10.0.0.0/8")
	t.Setenv("CHAT_PAGE_SIZE", "25")
	t.Setenv("HISTORY_PAGE_SIZE", "50")
	t.Setenv("SEARCH_RESULT_LIMIT", "12")
	t.Setenv("MAX_UPLOAD_SIZE", "64MiB")
	t.Setenv("TEXT_PREVIEW_MAX", "512KiB")
	t.Setenv("BODY_INDEX_MAX", "1MiB")
	t.Setenv("MEDIA_WORKER_COUNT", "3")
	t.Setenv("UPLOAD_CONCURRENCY", "42")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	if cfg.Port != 9000 || cfg.DataDir != dataDir || cfg.SessionTTL != 2*time.Hour || !cfg.CookieSecure {
		t.Fatalf("unexpected base config: %#v", cfg)
	}
	if len(cfg.TrustedProxies) != 2 {
		t.Fatalf("TrustedProxies len = %d, want 2", len(cfg.TrustedProxies))
	}
	if cfg.ChatPageSize != 25 ||
		cfg.HistoryPageSize != 50 ||
		cfg.SearchResultLimit != 12 ||
		cfg.MaxUploadBytes != 64<<20 ||
		cfg.TextPreviewMaxBytes != 512<<10 ||
		cfg.BodyIndexMaxBytes != 1<<20 ||
		cfg.MediaWorkerCount != 3 ||
		cfg.UploadConcurrency != MaxUploadConcurrency {
		t.Fatalf("unexpected tuning config: %#v", cfg)
	}
}

func TestLoadInvalidTrustedProxy(t *testing.T) {
	t.Setenv("DATA_DIR", t.TempDir())
	t.Setenv("TRUSTED_PROXIES", "not-an-ip")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid trusted proxy error")
	}
}
