package core

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"

	"github.com/usememos/memogram/internal/channel"
	v1pb "github.com/usememos/memos/proto/gen/api/v1"
)

func (c *Core) handleList(ctx context.Context, ev channel.InboundEvent) {
	c.clearPendingEdit(ev)
	st := &browseState{View: channel.BrowseList}
	c.rememberBrowseOrigin(ev, st)
	c.replyBrowseList(ctx, ev, st)
}

func (c *Core) handleTags(ctx context.Context, ev channel.InboundEvent) {
	c.clearPendingEdit(ev)
	st := &browseState{View: channel.BrowseTags}
	c.rememberBrowseOrigin(ev, st)
	c.replyBrowseTags(ctx, ev, st)
}

func (c *Core) handleCancel(ctx context.Context, ev channel.InboundEvent) {
	if c.pendingEdit(ev) == nil {
		c.reply(ctx, ev, channel.OutboundMessage{
			Kind:  channel.OutboundError,
			Error: "No pending edit",
		})
		return
	}
	c.clearPendingEdit(ev)
	st := c.browseState(ev)
	if st != nil && st.MemoName != "" {
		st.View = channel.BrowseDetail
		c.replyBrowseDetail(ctx, ev, st)
		return
	}
	c.reply(ctx, ev, channel.OutboundMessage{
		Kind:  channel.OutboundError,
		Error: "Edit cancelled",
	})
}

func (c *Core) handleBrowse(ctx context.Context, ev channel.InboundEvent) {
	if _, ok := c.token(ev); !ok {
		c.reply(ctx, ev, channel.OutboundMessage{Kind: channel.OutboundPromptBind})
		return
	}

	st := c.browseState(ev)
	if st == nil {
		c.reply(ctx, ev, channel.OutboundMessage{
			Kind:  channel.OutboundError,
			Error: "List expired. Send /list again.",
		})
		return
	}
	c.rememberBrowseOrigin(ev, st)

	switch ev.Action.Name {
	case channel.ActionOpen:
		c.browseOpen(ctx, ev, st)
	case channel.ActionNext:
		c.browseNext(ctx, ev, st)
	case channel.ActionPrev:
		c.browsePrev(ctx, ev, st)
	case channel.ActionBack:
		c.browseBack(ctx, ev, st)
	case channel.ActionEdit:
		c.browseEdit(ctx, ev, st)
	case channel.ActionDelete:
		st.View = channel.BrowseConfirmDelete
		c.setBrowseState(ev, st)
		c.replyBrowseConfirm(ctx, ev, st)
	case channel.ActionDeleteConfirm:
		c.browseDelete(ctx, ev, st)
	case channel.ActionTags:
		c.clearPendingEdit(ev)
		st.View = channel.BrowseTags
		st.Tag = ""
		st.Query = ""
		st.PageToken = ""
		st.PrevTokens = nil
		st.TagsOffset = 0
		c.replyBrowseTags(ctx, ev, st)
	case channel.ActionTag:
		c.browseOpenTag(ctx, ev, st)
	default:
		c.reply(ctx, ev, channel.OutboundMessage{
			Kind:  channel.OutboundError,
			Error: "Unknown action",
		})
	}
}

func (c *Core) browseOpen(ctx context.Context, ev channel.InboundEvent, st *browseState) {
	idx, err := strconv.Atoi(ev.Action.Resource)
	if err != nil || idx < 0 || idx >= len(st.MemoNames) {
		c.reply(ctx, ev, channel.OutboundMessage{
			Kind:  channel.OutboundError,
			Error: "Invalid memo",
		})
		return
	}
	st.MemoName = st.MemoNames[idx]
	st.View = channel.BrowseDetail
	c.replyBrowseDetail(ctx, ev, st)
}

func (c *Core) browseOpenTag(ctx context.Context, ev channel.InboundEvent, st *browseState) {
	idx, err := strconv.Atoi(ev.Action.Resource)
	if err != nil || idx < 0 || idx >= len(st.TagNames) {
		c.reply(ctx, ev, channel.OutboundMessage{
			Kind:  channel.OutboundError,
			Error: "Invalid tag",
		})
		return
	}
	st.Tag = st.TagNames[idx]
	st.Query = ""
	st.PageToken = ""
	st.PrevTokens = nil
	st.View = channel.BrowseList
	c.replyBrowseList(ctx, ev, st)
}

func (c *Core) browseNext(ctx context.Context, ev channel.InboundEvent, st *browseState) {
	if st.View == channel.BrowseTags {
		st.TagsOffset += tagsPageSize
		c.replyBrowseTags(ctx, ev, st)
		return
	}
	if st.NextPageToken == "" {
		c.replyBrowseList(ctx, ev, st)
		return
	}
	st.PrevTokens = append(st.PrevTokens, st.PageToken)
	st.PageToken = st.NextPageToken
	c.replyBrowseList(ctx, ev, st)
}

func (c *Core) browsePrev(ctx context.Context, ev channel.InboundEvent, st *browseState) {
	if st.View == channel.BrowseTags {
		st.TagsOffset -= tagsPageSize
		if st.TagsOffset < 0 {
			st.TagsOffset = 0
		}
		c.replyBrowseTags(ctx, ev, st)
		return
	}
	if len(st.PrevTokens) == 0 {
		c.replyBrowseList(ctx, ev, st)
		return
	}
	st.PageToken = st.PrevTokens[len(st.PrevTokens)-1]
	st.PrevTokens = st.PrevTokens[:len(st.PrevTokens)-1]
	c.replyBrowseList(ctx, ev, st)
}

func (c *Core) browseBack(ctx context.Context, ev channel.InboundEvent, st *browseState) {
	c.clearPendingEdit(ev)
	switch st.View {
	case channel.BrowseConfirmDelete, channel.BrowseEditPrompt:
		st.View = channel.BrowseDetail
		c.replyBrowseDetail(ctx, ev, st)
	default:
		st.View = channel.BrowseList
		c.replyBrowseList(ctx, ev, st)
	}
}

func (c *Core) browseEdit(ctx context.Context, ev channel.InboundEvent, st *browseState) {
	if st.MemoName == "" {
		c.reply(ctx, ev, channel.OutboundMessage{
			Kind:  channel.OutboundError,
			Error: "Invalid memo",
		})
		return
	}
	st.View = channel.BrowseEditPrompt
	c.setBrowseState(ev, st)
	c.setPendingEdit(ev, &pendingEdit{
		MemoName:  st.MemoName,
		ChatID:    ev.Origin.ChatID,
		MessageID: ev.Origin.MessageID,
	})
	c.reply(ctx, ev, channel.OutboundMessage{
		Kind: channel.OutboundBrowse,
		Browse: &channel.BrowsePayload{
			View:    channel.BrowseEditPrompt,
			Tag:     st.Tag,
			Query:   st.Query,
			Content: "Send the replacement text. The next message will overwrite this memo.\n/cancel to abort.",
			Memo:    &channel.MemoInfo{Name: st.MemoName},
		},
	})
}

func (c *Core) browseDelete(ctx context.Context, ev channel.InboundEvent, st *browseState) {
	token, ok := c.token(ev)
	if !ok {
		c.reply(ctx, ev, channel.OutboundMessage{Kind: channel.OutboundPromptBind})
		return
	}
	if st.MemoName == "" {
		c.reply(ctx, ev, channel.OutboundMessage{
			Kind:  channel.OutboundError,
			Error: "Invalid memo",
		})
		return
	}
	if err := c.backend.Authenticated(token).DeleteMemo(ctx, st.MemoName); err != nil {
		slog.Error("failed to delete memo", slog.Any("err", err))
		c.reply(ctx, ev, channel.OutboundMessage{
			Kind:  channel.OutboundError,
			Error: "Failed to delete memo",
		})
		return
	}
	st.MemoName = ""
	st.View = channel.BrowseList
	c.replyBrowseList(ctx, ev, st)
}

func (c *Core) replyBrowseList(ctx context.Context, ev channel.InboundEvent, st *browseState) {
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
	page, err := auth.ListMemos(ctx, listPageSize, st.PageToken, listOrderBy, buildMemoFilter(st.Query, st.Tag, user))
	if err != nil {
		slog.Error("failed to list memos", slog.Any("err", err))
		c.reply(ctx, ev, channel.OutboundMessage{
			Kind:  channel.OutboundError,
			Error: "Failed to list memos",
		})
		return
	}
	if len(page.Memos) == 0 && st.PageToken != "" && len(st.PrevTokens) > 0 {
		st.PageToken = st.PrevTokens[len(st.PrevTokens)-1]
		st.PrevTokens = st.PrevTokens[:len(st.PrevTokens)-1]
		c.replyBrowseList(ctx, ev, st)
		return
	}

	st.View = channel.BrowseList
	st.NextPageToken = page.NextPageToken
	st.MemoNames = make([]string, 0, len(page.Memos))
	items := make([]channel.MemoSummary, 0, len(page.Memos))
	for _, memo := range page.Memos {
		st.MemoNames = append(st.MemoNames, memo.Name)
		items = append(items, memoSummary(memo))
	}
	c.setBrowseState(ev, st)
	start := len(st.PrevTokens)*listPageSize + 1
	end := start + len(items) - 1
	if len(items) == 0 {
		start, end = 0, 0
	}
	c.reply(ctx, ev, channel.OutboundMessage{
		Kind: channel.OutboundBrowse,
		Browse: &channel.BrowsePayload{
			View:    channel.BrowseList,
			Tag:     st.Tag,
			Query:   st.Query,
			Start:   start,
			End:     end,
			HasPrev: len(st.PrevTokens) > 0,
			HasNext: page.NextPageToken != "",
			Items:   items,
		},
	})
}

func (c *Core) replyBrowseTags(ctx context.Context, ev channel.InboundEvent, st *browseState) {
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
	stats, err := auth.GetUserStats(ctx, user.Name)
	if err != nil {
		slog.Error("failed to get user stats", slog.Any("err", err))
		c.reply(ctx, ev, channel.OutboundMessage{
			Kind:  channel.OutboundError,
			Error: "Failed to list tags",
		})
		return
	}
	names := make([]string, 0, len(stats.GetTagCount()))
	for name := range stats.GetTagCount() {
		names = append(names, name)
	}
	sort.Strings(names)
	if st.TagsOffset < 0 {
		st.TagsOffset = 0
	}
	if st.TagsOffset > 0 && st.TagsOffset >= len(names) {
		st.TagsOffset -= tagsPageSize
		if st.TagsOffset < 0 {
			st.TagsOffset = 0
		}
	}
	end := st.TagsOffset + tagsPageSize
	if end > len(names) {
		end = len(names)
	}
	page := names[min(st.TagsOffset, len(names)):end]
	if len(page) == 0 {
		st.TagsOffset = 0
		end = min(tagsPageSize, len(names))
		page = names[:end]
	}
	st.View = channel.BrowseTags
	st.TagNames = page
	c.setBrowseState(ev, st)

	tags := make([]channel.TagCount, 0, len(page))
	for _, name := range page {
		tags = append(tags, channel.TagCount{Name: name, Count: stats.GetTagCount()[name]})
	}
	start := st.TagsOffset + 1
	endNum := st.TagsOffset + len(page)
	if len(page) == 0 {
		start, endNum = 0, 0
	}
	c.reply(ctx, ev, channel.OutboundMessage{
		Kind: channel.OutboundBrowse,
		Browse: &channel.BrowsePayload{
			View:    channel.BrowseTags,
			Start:   start,
			End:     endNum,
			HasPrev: st.TagsOffset > 0,
			HasNext: end < len(names),
			Tags:    tags,
		},
	})
}

func (c *Core) replyBrowseDetail(ctx context.Context, ev channel.InboundEvent, st *browseState) {
	token, ok := c.token(ev)
	if !ok {
		c.reply(ctx, ev, channel.OutboundMessage{Kind: channel.OutboundPromptBind})
		return
	}
	memo, err := c.backend.Authenticated(token).GetMemo(ctx, st.MemoName)
	if err != nil {
		c.reply(ctx, ev, channel.OutboundMessage{
			Kind:  channel.OutboundError,
			Error: fmt.Sprintf("Memo %s not found", st.MemoName),
		})
		return
	}
	info, err := c.memoInfo(memo)
	if err != nil {
		slog.Error("failed to extract memo UID", slog.Any("err", err))
		c.reply(ctx, ev, channel.OutboundMessage{
			Kind:  channel.OutboundError,
			Error: "Failed to load memo",
		})
		return
	}
	st.View = channel.BrowseDetail
	c.setBrowseState(ev, st)
	c.reply(ctx, ev, channel.OutboundMessage{
		Kind: channel.OutboundBrowse,
		Browse: &channel.BrowsePayload{
			View:    channel.BrowseDetail,
			Tag:     st.Tag,
			Query:   st.Query,
			Memo:    info,
			Content: memo.Content,
		},
	})
}

func (c *Core) replyBrowseConfirm(ctx context.Context, ev channel.InboundEvent, st *browseState) {
	c.reply(ctx, ev, channel.OutboundMessage{
		Kind: channel.OutboundBrowse,
		Browse: &channel.BrowsePayload{
			View:    channel.BrowseConfirmDelete,
			Tag:     st.Tag,
			Query:   st.Query,
			Memo:    &channel.MemoInfo{Name: st.MemoName},
			Content: "Delete this memo?",
		},
	})
}

func memoSummary(memo *v1pb.Memo) channel.MemoSummary {
	summary := channel.MemoSummary{
		Name:    memo.Name,
		Snippet: memo.Snippet,
		Content: memo.Content,
	}
	if memo.UpdateTime != nil {
		summary.UpdatedAt = memo.UpdateTime.AsTime()
	} else if memo.CreateTime != nil {
		summary.UpdatedAt = memo.CreateTime.AsTime()
	}
	return summary
}

func buildMemoFilter(query, tag string, user *v1pb.User) string {
	var parts []string
	if query != "" {
		parts = append(parts, fmt.Sprintf("content.contains(%q)", query))
	}
	if creator := creatorName(user); creator != "" {
		parts = append(parts, fmt.Sprintf("creator == %q", creator))
	}
	if tag != "" {
		parts = append(parts, fmt.Sprintf("%q in tags", tag))
	}
	return strings.Join(parts, " && ")
}

func creatorName(user *v1pb.User) string {
	if user == nil {
		return ""
	}
	if user.Name != "" {
		return user.Name
	}
	if user.Username != "" {
		return "users/" + user.Username
	}
	return ""
}
