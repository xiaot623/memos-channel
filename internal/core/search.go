package core

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/usememos/memogram/internal/channel"
	v1pb "github.com/usememos/memos/proto/gen/api/v1"
)

func (c *Core) handleSearch(ctx context.Context, ev channel.InboundEvent) {
	searchString := strings.TrimSpace(ev.Command.Args)
	if searchString == "" {
		c.reply(ctx, ev, channel.OutboundMessage{
			Kind:   channel.OutboundPromptUsage,
			Prompt: channel.CommandSearch,
		})
		return
	}

	token, ok := c.token(ev)
	if !ok {
		c.reply(ctx, ev, channel.OutboundMessage{Kind: channel.OutboundPromptBind})
		return
	}

	auth := c.backend.Authenticated(token)
	user, err := auth.GetCurrentUser(ctx)
	if err != nil {
		c.reply(ctx, ev, channel.OutboundMessage{
			Kind:  channel.OutboundError,
			Error: "Invalid access token",
		})
		return
	}

	filter := buildMemoSearchFilter(searchString, user)
	memos, err := auth.ListMemos(ctx, 10, filter)
	if err != nil {
		slog.Error("failed to search memos", slog.Any("err", err))
		return
	}

	results := make([]channel.MemoSummary, 0, len(memos))
	for _, memo := range memos {
		results = append(results, channel.MemoSummary{
			Name:    memo.Name,
			Content: memo.Content,
		})
	}
	c.reply(ctx, ev, channel.OutboundMessage{
		Kind:    channel.OutboundSearchList,
		Results: results,
	})
}

func buildMemoSearchFilter(searchString string, user *v1pb.User) string {
	filter := fmt.Sprintf("content.contains(%q)", searchString)
	if user == nil {
		return filter
	}

	creator := user.Name
	if creator == "" && user.Username != "" {
		creator = "users/" + user.Username
	}
	if creator == "" {
		return filter
	}

	return fmt.Sprintf("%s && creator == %q", filter, creator)
}
