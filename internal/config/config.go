package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/caarlos0/env"
	"github.com/joho/godotenv"
)

type Config struct {
	ServerAddr       string `env:"SERVER_ADDR,required"`
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

	return &cfg, nil
}

func (c *Config) BaseURL() string {
	baseURL := c.ServerAddr
	baseURL = strings.TrimPrefix(baseURL, "dns:")
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "http://" + baseURL
	}
	return baseURL
}

func (c *Config) TelegramEnabled() bool {
	return c.BotToken != ""
}
