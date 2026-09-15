package core

import (
	"context"

	"github.com/usememos/memogram/internal/channel"
)

func (c *Core) handleHelp(ctx context.Context, ev channel.InboundEvent) {
	_ = ctx
	c.reply(ctx, ev, channel.OutboundMessage{
		Kind:   channel.OutboundPromptUsage,
		Prompt: channel.CommandHelp,
	})
}
