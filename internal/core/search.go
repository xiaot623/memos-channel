package core

import (
	"context"
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

	c.clearPendingEdit(ev)
	st := &browseState{
		View:  channel.BrowseList,
		Query: searchString,
	}
	c.rememberBrowseOrigin(ev, st)
	c.replyBrowseList(ctx, ev, st)
}

func buildMemoSearchFilter(searchString string, user *v1pb.User) string {
	return buildMemoFilter(searchString, "", user)
}
