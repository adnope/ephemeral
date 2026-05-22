package config

import (
	"fmt"
	"math"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"
)

const MaxUploadConcurrency = 10

type Config struct {
	Port                int
	DataDir             string
	SessionTTL          time.Duration
	CookieSecure        bool
	TrustedProxies      []netip.Prefix
	ChatPageSize        int
	HistoryPageSize     int
	SearchResultLimit   int
	MaxUploadBytes      int64
	TextPreviewMaxBytes int64
	BodyIndexMaxBytes   int64
	MediaWorkerCount    int
	MediaProcessTimeout time.Duration
	HLSMinBytes         int64
	HLSMinDuration      time.Duration
	UploadConcurrency   int
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:                8080,
		DataDir:             "./data",
		SessionTTL:          30 * 24 * time.Hour,
		ChatPageSize:        100,
		HistoryPageSize:     100,
		SearchResultLimit:   30,
		MaxUploadBytes:      2 << 30,
		TextPreviewMaxBytes: 10 << 20,
		BodyIndexMaxBytes:   20 << 20,
		MediaWorkerCount:    1,
		MediaProcessTimeout: 30 * time.Minute,
		HLSMinBytes:         100 << 20,
		HLSMinDuration:      5 * time.Minute,
		UploadConcurrency:   1,
	}

	if v := os.Getenv("PORT"); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("config: invalid PORT %q: %w", v, err)
		}
		cfg.Port = p
	}

	if v := os.Getenv("DATA_DIR"); v != "" {
		cfg.DataDir = v
	}

	if v := os.Getenv("SESSION_TTL"); v != "" {
		ttl, err := parseDurationWithDays(v)
		if err != nil {
			return nil, fmt.Errorf("config: invalid SESSION_TTL %q: %w", v, err)
		}
		if ttl < time.Minute {
			return nil, fmt.Errorf("config: SESSION_TTL must be at least 1 minute")
		}
		cfg.SessionTTL = ttl
	}

	if err := loadBool("COOKIE_SECURE", &cfg.CookieSecure); err != nil {
		return nil, err
	}

	if v := os.Getenv("TRUSTED_PROXIES"); v != "" {
		proxies, err := parseTrustedProxies(v)
		if err != nil {
			return nil, fmt.Errorf("config: invalid TRUSTED_PROXIES %q: %w", v, err)
		}
		cfg.TrustedProxies = proxies
	}

	if err := loadPositiveInt("CHAT_PAGE_SIZE", &cfg.ChatPageSize); err != nil {
		return nil, err
	}
	if err := loadPositiveInt("HISTORY_PAGE_SIZE", &cfg.HistoryPageSize); err != nil {
		return nil, err
	}
	if err := loadPositiveInt("SEARCH_RESULT_LIMIT", &cfg.SearchResultLimit); err != nil {
		return nil, err
	}
	if err := loadByteSize("MAX_UPLOAD_SIZE", &cfg.MaxUploadBytes); err != nil {
		return nil, err
	}
	if err := loadByteSize("TEXT_PREVIEW_MAX", &cfg.TextPreviewMaxBytes); err != nil {
		return nil, err
	}
	if err := loadByteSize("BODY_INDEX_MAX", &cfg.BodyIndexMaxBytes); err != nil {
		return nil, err
	}
	if err := loadPositiveInt("MEDIA_WORKER_COUNT", &cfg.MediaWorkerCount); err != nil {
		return nil, err
	}
	if err := loadPositiveDuration("MEDIA_PROCESS_TIMEOUT", &cfg.MediaProcessTimeout); err != nil {
		return nil, err
	}
	if err := loadNonNegativeByteSize("HLS_MIN_SIZE", &cfg.HLSMinBytes); err != nil {
		return nil, err
	}
	if err := loadNonNegativeDuration("HLS_MIN_DURATION", &cfg.HLSMinDuration); err != nil {
		return nil, err
	}
	if err := loadBoundedPositiveInt("UPLOAD_CONCURRENCY", &cfg.UploadConcurrency, MaxUploadConcurrency); err != nil {
		return nil, err
	}

	dirs := []string{
		cfg.DataDir,
		cfg.DataDir + "/uploads",
		cfg.DataDir + "/uploads/thumbs",
		cfg.DataDir + "/uploads/playback",
		cfg.DataDir + "/uploads/hls",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, fmt.Errorf("config: mkdir %s: %w", d, err)
		}
	}

	return cfg, nil
}

func (c *Config) DBPath() string {
	return c.DataDir + "/ephemeral.db"
}

func (c *Config) UploadDir() string {
	return c.DataDir + "/uploads"
}

func loadPositiveInt(name string, target *int) error {
	value := os.Getenv(name)
	if value == "" {
		return nil
	}

	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fmt.Errorf("config: invalid %s %q: %w", name, value, err)
	}
	if parsed <= 0 {
		return fmt.Errorf("config: %s must be positive", name)
	}

	*target = parsed
	return nil
}

func loadBoundedPositiveInt(name string, target *int, maxValue int) error {
	if err := loadPositiveInt(name, target); err != nil {
		return err
	}
	if *target > maxValue {
		*target = maxValue
	}
	return nil
}

func loadBool(name string, target *bool) error {
	value := os.Getenv(name)
	if value == "" {
		return nil
	}

	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return fmt.Errorf("config: invalid %s %q: %w", name, value, err)
	}

	*target = parsed
	return nil
}

func loadByteSize(name string, target *int64) error {
	value := os.Getenv(name)
	if value == "" {
		return nil
	}

	parsed, err := parseByteSize(value)
	if err != nil {
		return fmt.Errorf("config: invalid %s %q: %w", name, value, err)
	}
	if parsed <= 0 {
		return fmt.Errorf("config: %s must be positive", name)
	}

	*target = parsed
	return nil
}

func loadNonNegativeByteSize(name string, target *int64) error {
	value := os.Getenv(name)
	if value == "" {
		return nil
	}

	parsed, err := parseByteSize(value)
	if err != nil {
		return fmt.Errorf("config: invalid %s %q: %w", name, value, err)
	}
	if parsed < 0 {
		return fmt.Errorf("config: %s must be non-negative", name)
	}

	*target = parsed
	return nil
}

func loadPositiveDuration(name string, target *time.Duration) error {
	value := os.Getenv(name)
	if value == "" {
		return nil
	}

	parsed, err := parseDurationWithDays(value)
	if err != nil {
		return fmt.Errorf("config: invalid %s %q: %w", name, value, err)
	}
	if parsed <= 0 {
		return fmt.Errorf("config: %s must be positive", name)
	}

	*target = parsed
	return nil
}

func loadNonNegativeDuration(name string, target *time.Duration) error {
	value := os.Getenv(name)
	if value == "" {
		return nil
	}

	parsed, err := parseDurationWithDays(value)
	if err != nil {
		return fmt.Errorf("config: invalid %s %q: %w", name, value, err)
	}
	if parsed < 0 {
		return fmt.Errorf("config: %s must be non-negative", name)
	}

	*target = parsed
	return nil
}

func parseTrustedProxies(value string) ([]netip.Prefix, error) {
	parts := strings.Split(value, ",")
	proxies := make([]netip.Prefix, 0, len(parts))
	for _, part := range parts {
		raw := strings.TrimSpace(part)
		if raw == "" {
			continue
		}

		if strings.Contains(raw, "/") {
			prefix, err := netip.ParsePrefix(raw)
			if err != nil {
				return nil, err
			}
			proxies = append(proxies, prefix.Masked())
			continue
		}

		addr, err := netip.ParseAddr(raw)
		if err != nil {
			return nil, err
		}
		addr = addr.Unmap()
		bits := 32
		if addr.Is6() {
			bits = 128
		}
		proxies = append(proxies, netip.PrefixFrom(addr, bits))
	}

	return proxies, nil
}

func parseDurationWithDays(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if before, ok := strings.CutSuffix(value, "d"); ok {
		daysRaw := before
		days, err := strconv.Atoi(daysRaw)
		if err != nil {
			return 0, err
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}

	return time.ParseDuration(value)
}

func parseByteSize(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("empty size")
	}

	numberPart := value
	unitPart := ""
	for i, r := range value {
		if (r < '0' || r > '9') && r != '.' {
			numberPart = strings.TrimSpace(value[:i])
			unitPart = strings.TrimSpace(value[i:])
			break
		}
	}

	if numberPart == "" {
		return 0, fmt.Errorf("missing size value")
	}

	number, err := strconv.ParseFloat(numberPart, 64)
	if err != nil {
		return 0, err
	}
	if number < 0 {
		return 0, fmt.Errorf("size must be non-negative")
	}

	multiplier, ok := byteSizeMultipliers[strings.ToLower(unitPart)]
	if !ok {
		return 0, fmt.Errorf("unsupported size unit %q", unitPart)
	}

	size := number * float64(multiplier)
	if size > math.MaxInt64 {
		return 0, fmt.Errorf("size overflows int64")
	}
	return int64(size), nil
}

var byteSizeMultipliers = map[string]int64{
	"":    1,
	"b":   1,
	"kb":  1000,
	"mb":  1000 * 1000,
	"gb":  1000 * 1000 * 1000,
	"tb":  1000 * 1000 * 1000 * 1000,
	"kib": 1 << 10,
	"mib": 1 << 20,
	"gib": 1 << 30,
	"tib": 1 << 40,
}
