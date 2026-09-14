package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/caarlos0/env"
	"github.com/joho/godotenv"
)

type Config struct {
	ServerAddr       string `env:"SERVER_ADDR,required"`
	PublicURL        string `env:"BASE_URL"`
	Data             string `env:"DATA"`
	BotToken         string `env:"BOT_TOKEN"`
	BotProxyAddr     string `env:"BOT_PROXY_ADDR"`
	AllowedUsernames string `env:"ALLOWED_USERNAMES"`
}

func Load() (*Config, error) {
	envFileName := ".env"
	if _, err := os.Stat(envFileName); err == nil {
		if err := godotenv.Load(envFileName); err != nil {
			return nil, fmt.Errorf("load %s: %w", envFileName, err)
		}
	}

	cfg := Config{}
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	if cfg.Data == "" {
		cfg.Data = "data.txt"
	}

	if fileInfo, err := os.Stat(cfg.Data); err == nil {
		if fileInfo.IsDir() {
			return nil, fmt.Errorf("config file cannot be a directory: %s", cfg.Data)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to access data file %s: %w", cfg.Data, err)
	}

	abs, err := filepath.Abs(cfg.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for data file %s: %w", cfg.Data, err)
	}
	cfg.Data = abs

	if raw := strings.TrimSpace(cfg.PublicURL); raw != "" {
		cfg.PublicURL = HTTPSOrigin(raw)
		if cfg.PublicURL == "" {
			slog.Warn("BASE_URL ignored; must be an https origin", "base_url", raw)
		}
	} else {
		cfg.PublicURL = ""
	}

	return &cfg, nil
}

func (c *Config) ServerURL() string {
	serverURL := c.ServerAddr
	serverURL = strings.TrimPrefix(serverURL, "dns:")
	if !strings.HasPrefix(serverURL, "http://") && !strings.HasPrefix(serverURL, "https://") {
		serverURL = "http://" + serverURL
	}
	return serverURL
}

func HTTPSOrigin(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimRight(s, "/")
	if !strings.HasPrefix(s, "https://") || s == "https://" {
		return ""
	}
	return s
}

func (c *Config) TelegramEnabled() bool {
	return c.BotToken != ""
}
