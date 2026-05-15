package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port          int
	DataDir       string
	SessionSecret string
	SessionTTL    time.Duration
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:          8080,
		DataDir:       "./data",
		SessionSecret: "",
		SessionTTL:    30 * 24 * time.Hour,
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

	if v := os.Getenv("SESSION_SECRET"); v != "" {
		cfg.SessionSecret = v
	} else {
		cfg.SessionSecret = "default-secret"
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

	dirs := []string{
		cfg.DataDir,
		cfg.DataDir + "/uploads",
		cfg.DataDir + "/uploads/thumbs",
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
