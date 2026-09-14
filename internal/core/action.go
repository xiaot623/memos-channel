package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/usememos/memogram/internal/channel"
	v1pb "github.com/usememos/memos/proto/gen/api/v1"
)

func (c *Core) handleAction(ctx context.Context, ev channel.InboundEvent) {
	token, ok := c.token(ev)
	if !ok {
		c.reply(ctx, ev, channel.OutboundMessage{Kind: channel.OutboundPromptBind})
		return
	}

	if ev.Action.Name == "" || ev.Action.Resource == "" {
		c.reply(ctx, ev, channel.OutboundMessage{
			Kind:  channel.OutboundError,
			Error: "Invalid command",
		})
		return
	}

	auth := c.backend.Authenticated(token)
	memo, err := auth.GetMemo(ctx, ev.Action.Resource)
	if err != nil {
		c.reply(ctx, ev, channel.OutboundMessage{
			Kind:  channel.OutboundError,
			Error: fmt.Sprintf("Memo %s not found", ev.Action.Resource),
		})
		return
	}

	switch ev.Action.Name {
	case channel.ActionPublic:
		memo.Visibility = v1pb.Visibility_PUBLIC
	case channel.ActionProtected:
		memo.Visibility = v1pb.Visibility_PROTECTED
	case channel.ActionPrivate:
		memo.Visibility = v1pb.Visibility_PRIVATE
	case channel.ActionPin:
		memo.Pinned = !memo.Pinned
	default:
		c.reply(ctx, ev, channel.OutboundMessage{
			Kind:  channel.OutboundError,
			Error: "Unknown action",
		})
		return
	}

	if err := auth.UpdateMemo(ctx, memo, []string{"visibility", "pinned"}); err != nil {
		slog.Error("failed to update memo", slog.Any("err", err))
		c.reply(ctx, ev, channel.OutboundMessage{
			Kind:  channel.OutboundError,
			Error: "Failed to update memo",
		})
		return
	}

	info, err := c.memoInfo(memo)
	if err != nil {
		slog.Error("failed to extract memo UID", slog.Any("err", err))
		c.reply(ctx, ev, channel.OutboundMessage{
			Kind:  channel.OutboundError,
			Error: "Failed to update memo",
		})
		return
	}
	c.reply(ctx, ev, channel.OutboundMessage{
		Kind:    channel.OutboundSaved,
		Memo:    info,
		Actions: channel.DefaultMemoActions(),
	})
}
