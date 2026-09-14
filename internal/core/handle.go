package core

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/usememos/memogram/internal/channel"
	"github.com/usememos/memogram/internal/memos"
	v1pb "github.com/usememos/memos/proto/gen/api/v1"
)

func (c *Core) Start(ctx context.Context) error {
	if len(c.adapters) == 0 {
		return fmt.Errorf("no channel adapters enabled")
	}

	slog.Info("memos-channel started")
	profile, err := c.backend.GetInstanceProfile(ctx)
	if err != nil {
		slog.Warn("failed to get instance profile", slog.Any("err", err))
	} else {
		slog.Info("instance profile", slog.Any("profile", profile))
		if profile != nil {
			c.instanceURL = profile.InstanceUrl
		}
	}

	g, ctx := errgroup.WithContext(ctx)
	for name, adapter := range c.adapters {
		name, adapter := name, adapter
		g.Go(func() error {
			slog.Info("starting adapter", "channel", name)
			return adapter.Start(ctx, c.Handle)
		})
	}
	return g.Wait()
}

func (c *Core) Handle(ctx context.Context, ev channel.InboundEvent) error {
	if _, ok := c.adapters[ev.Channel]; !ok {
		return fmt.Errorf("unknown channel %s", ev.Channel)
	}

	switch ev.Kind {
	case channel.KindCommand:
		switch ev.Command.Name {
		case channel.CommandStart:
			c.handleBind(ctx, ev)
		case channel.CommandSearch:
			c.handleSearch(ctx, ev)
		default:
			c.reply(ctx, ev, channel.OutboundMessage{Kind: channel.OutboundError, Error: "Unknown command"})
		}
	case channel.KindAction:
		c.handleAction(ctx, ev)
	default:
		c.handleMessage(ctx, ev)
	}
	return nil
}

func (c *Core) reply(ctx context.Context, ev channel.InboundEvent, msg channel.OutboundMessage) {
	adapter, ok := c.adapters[ev.Channel]
	if !ok {
		return
	}
	if err := adapter.Reply(ctx, ev.Origin, msg); err != nil {
		slog.Error("failed to reply", slog.Any("err", err), "channel", ev.Channel)
	}
}

func (c *Core) token(ev channel.InboundEvent) (string, bool) {
	return c.store.Get(ev.Channel, ev.PlatformUserID)
}

func (c *Core) memoURL(uid string) string {
	base := strings.TrimRight(c.baseURL, "/")
	if c.instanceURL != "" {
		base = strings.TrimRight(c.instanceURL, "/")
	}
	return base + "/memos/" + uid
}

func (c *Core) memoInfo(memo *v1pb.Memo) (*channel.MemoInfo, error) {
	uid, err := memos.ExtractMemoUIDFromName(memo.Name)
	if err != nil {
		return nil, err
	}
	return &channel.MemoInfo{
		Name:       memo.Name,
		UID:        uid,
		Visibility: v1pb.Visibility_name[int32(memo.Visibility)],
		Pinned:     memo.Pinned,
		URL:        c.memoURL(uid),
	}, nil
}
