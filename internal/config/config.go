package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port          int
	DataDir       string
	SessionSecret string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:          8080,
		DataDir:       "./data",
		SessionSecret: "",
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
