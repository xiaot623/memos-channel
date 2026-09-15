package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/usememos/memogram/internal/channel/feishu"
	"github.com/usememos/memogram/internal/channel/telegram"
	"github.com/usememos/memogram/internal/config"
	"github.com/usememos/memogram/internal/core"
	"github.com/usememos/memogram/internal/memos"
	"github.com/usememos/memogram/internal/store"
)

func main() {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	st := store.New(cfg.Data)
	if err := st.Init(); err != nil {
		panic(err)
	}

	c := core.New(st, memos.NewClient(cfg.ServerURL()), cfg.PublicURL)
	if cfg.TelegramEnabled() {
		c.Register(telegram.New(telegram.Options{
			Token:            cfg.BotToken,
			ProxyAddr:        cfg.BotProxyAddr,
			AllowedUsernames: cfg.AllowedUsernames,
		}))
	}
	if cfg.FeishuEnabled() {
		c.Register(feishu.New(feishu.Options{
			AppID:          cfg.FeishuAppID,
			AppSecret:      cfg.FeishuAppSecret,
			BaseURL:        cfg.FeishuBaseURL,
			AllowedOpenIDs: cfg.FeishuAllowedOpenIDs,
		}))
	}

	if err := c.Start(ctx); err != nil {
		slog.Error("service stopped", "err", err)
		os.Exit(1)
	}
}
