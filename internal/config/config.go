package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	Port          int
	DataDir       string
	SessionSecret string
}

// Load reads configuration from environment variables with sensible defaults.
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
		cfg.SessionSecret = "leandrop-default-secret-change-me"
	}

	// Ensure data directories exist
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

// DBPath returns the full path to the SQLite database file.
func (c *Config) DBPath() string {
	return c.DataDir + "/leandrop.db"
}

// UploadDir returns the path to the uploads directory.
func (c *Config) UploadDir() string {
	return c.DataDir + "/uploads"
}
