package core

import (
	"github.com/usememos/memogram/internal/channel"
)

const (
	listPageSize = 10
	tagsPageSize = 20
	listOrderBy  = "update_time desc"
)

type browseState struct {
	View          channel.BrowseView
	Tag           string
	Query         string
	PageToken     string
	PrevTokens    []string
	NextPageToken string
	MemoNames     []string
	TagNames      []string
	MemoName      string
	TagsOffset    int
	ChatID        string
	MessageID     string
}

type pendingEdit struct {
	MemoName  string
	ChatID    string
	MessageID string
}

func sessionKey(ev channel.InboundEvent) string {
	return ev.Channel + "\x1f" + ev.PlatformUserID
}

func (c *Core) browseState(ev channel.InboundEvent) *browseState {
	c.mu.Lock()
	defer c.mu.Unlock()
	st, ok := c.browse[sessionKey(ev)]
	if !ok {
		return nil
	}
	clone := *st
	clone.PrevTokens = append([]string(nil), st.PrevTokens...)
	clone.MemoNames = append([]string(nil), st.MemoNames...)
	clone.TagNames = append([]string(nil), st.TagNames...)
	return &clone
}

func (c *Core) setBrowseState(ev channel.InboundEvent, st *browseState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if st == nil {
		delete(c.browse, sessionKey(ev))
		return
	}
	clone := *st
	clone.PrevTokens = append([]string(nil), st.PrevTokens...)
	clone.MemoNames = append([]string(nil), st.MemoNames...)
	clone.TagNames = append([]string(nil), st.TagNames...)
	c.browse[sessionKey(ev)] = &clone
}

func (c *Core) rememberBrowseOrigin(ev channel.InboundEvent, st *browseState) {
	if ev.Origin.MessageID == "" {
		return
	}
	st.ChatID = ev.Origin.ChatID
	st.MessageID = ev.Origin.MessageID
}

func (c *Core) pendingEdit(ev channel.InboundEvent) *pendingEdit {
	c.mu.Lock()
	defer c.mu.Unlock()
	ed, ok := c.edits[sessionKey(ev)]
	if !ok {
		return nil
	}
	clone := *ed
	return &clone
}

func (c *Core) setPendingEdit(ev channel.InboundEvent, ed *pendingEdit) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ed == nil {
		delete(c.edits, sessionKey(ev))
		return
	}
	clone := *ed
	c.edits[sessionKey(ev)] = &clone
}

func (c *Core) clearPendingEdit(ev channel.InboundEvent) {
	c.setPendingEdit(ev, nil)
}
