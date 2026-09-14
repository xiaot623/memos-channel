package core

import (
	"context"
	"log/slog"

	"github.com/usememos/memogram/internal/channel"
	v1pb "github.com/usememos/memos/proto/gen/api/v1"
)

func (c *Core) handleMessage(ctx context.Context, ev channel.InboundEvent) {
	token, ok := c.token(ev)
	if !ok {
		c.reply(ctx, ev, channel.OutboundMessage{Kind: channel.OutboundPromptBind})
		return
	}

	if ev.TextMarkdown == "" && len(ev.Attachments) == 0 {
		c.reply(ctx, ev, channel.OutboundMessage{
			Kind:  channel.OutboundError,
			Error: "Please input memo content",
		})
		return
	}

	auth := c.backend.Authenticated(token)
	memo, err := c.createOrReuseMemo(ctx, auth, ev)
	if err != nil {
		c.reply(ctx, ev, channel.OutboundMessage{
			Kind:  channel.OutboundError,
			Error: "Failed to create memo",
		})
		return
	}

	for _, ref := range ev.Attachments {
		att, err := ref.Download(ctx)
		if err != nil {
			slog.Error("failed to get file", slog.Any("err", err))
			c.reply(ctx, ev, channel.OutboundMessage{
				Kind:  channel.OutboundError,
				Error: "Error: failed to get file: " + err.Error(),
			})
			continue
		}
		if err := auth.CreateAttachment(ctx, att.Filename, att.ContentType, att.Bytes, memo.Name); err != nil {
			slog.Error("failed to save attachment", slog.Any("err", err))
			c.reply(ctx, ev, channel.OutboundMessage{
				Kind:  channel.OutboundError,
				Error: "Error: failed to save attachment: " + err.Error(),
			})
		}
	}

	info, err := c.memoInfo(memo)
	if err != nil {
		slog.Error("failed to extract memo UID", slog.Any("err", err))
		c.reply(ctx, ev, channel.OutboundMessage{
			Kind:  channel.OutboundError,
			Error: "Failed to save memo",
		})
		return
	}
	c.reply(ctx, ev, channel.OutboundMessage{
		Kind:    channel.OutboundSaved,
		Memo:    info,
		Actions: channel.DefaultMemoActions(),
	})
}

func (c *Core) createOrReuseMemo(ctx context.Context, auth AuthedClient, ev channel.InboundEvent) (*v1pb.Memo, error) {
	create := func() (*v1pb.Memo, error) {
		return auth.CreateMemo(ctx, ev.TextMarkdown)
	}
	if ev.GroupID == "" {
		return create()
	}
	return c.groups.getOrCreate(ev.Channel+"\x1f"+ev.GroupID, create)
}
