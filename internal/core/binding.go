package core

import (
	"context"
	"strings"

	"github.com/usememos/memogram/internal/channel"
)

func (c *Core) handleBind(ctx context.Context, ev channel.InboundEvent) {
	accessToken := strings.TrimSpace(ev.Command.Args)
	if accessToken == "" {
		c.reply(ctx, ev, channel.OutboundMessage{
			Kind:   channel.OutboundPromptUsage,
			Prompt: channel.CommandStart,
		})
		return
	}

	user, err := c.backend.Authenticated(accessToken).GetCurrentUser(ctx)
	if err != nil {
		c.reply(ctx, ev, channel.OutboundMessage{
			Kind:  channel.OutboundError,
			Error: "Invalid access token",
		})
		return
	}

	c.store.Set(ev.Channel, ev.PlatformUserID, accessToken)
	c.reply(ctx, ev, channel.OutboundMessage{
		Kind: channel.OutboundBound,
		User: user.DisplayName,
	})
}
