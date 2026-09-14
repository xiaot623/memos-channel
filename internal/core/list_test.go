package core

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/usememos/memogram/internal/channel"
	v1pb "github.com/usememos/memos/proto/gen/api/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func boundBrowseCore(t *testing.T, memos map[string]*v1pb.Memo) (*Core, *fakeAdapter, *fakeClient) {
	t.Helper()
	backend := &fakeClient{
		users: map[string]*v1pb.User{
			"tok": {Name: "users/alice", DisplayName: "Alice"},
		},
		memos:       memos,
		attachments: map[string]int{},
	}
	c, adapter, st := newTestCore(t, backend)
	st.Set(channel.Telegram, "42", "tok")
	return c, adapter, backend
}

func numberedMemos(n int, tag string) map[string]*v1pb.Memo {
	out := make(map[string]*v1pb.Memo, n)
	for i := 1; i <= n; i++ {
		name := fmt.Sprintf("memos/memo-%02d", i)
		out[name] = &v1pb.Memo{
			Name:       name,
			Content:    fmt.Sprintf("note %d body", i),
			Snippet:    fmt.Sprintf("note %d body", i),
			Tags:       []string{tag},
			UpdateTime: timestamppb.New(time.Unix(int64(1000+i), 0)),
		}
	}
	return out
}

func TestHandleListPagesTenAtATime(t *testing.T) {
	c, adapter, _ := boundBrowseCore(t, numberedMemos(12, "work"))

	ev := sampleEvent(channel.KindCommand)
	ev.Command = channel.Command{Name: channel.CommandList}
	if err := c.Handle(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if len(adapter.replies) != 1 || adapter.replies[0].Kind != channel.OutboundBrowse {
		t.Fatalf("expected browse list, got %#v", adapter.replies)
	}
	page := adapter.replies[0].Browse
	if page.View != channel.BrowseList || len(page.Items) != 10 || !page.HasNext || page.HasPrev {
		t.Fatalf("unexpected first page: %#v", page)
	}

	adapter.replies = nil
	next := sampleEvent(channel.KindAction)
	next.Origin.AckID = "cbq-next"
	next.Action = channel.Action{Name: channel.ActionNext, Resource: channel.BrowsePlaceholder}
	if err := c.Handle(context.Background(), next); err != nil {
		t.Fatal(err)
	}
	page = adapter.replies[0].Browse
	if len(page.Items) != 2 || !page.HasPrev || page.HasNext {
		t.Fatalf("unexpected second page: %#v", page)
	}

	adapter.replies = nil
	prev := next
	prev.Action.Name = channel.ActionPrev
	if err := c.Handle(context.Background(), prev); err != nil {
		t.Fatal(err)
	}
	page = adapter.replies[0].Browse
	if len(page.Items) != 10 || page.HasPrev || !page.HasNext {
		t.Fatalf("unexpected back to first page: %#v", page)
	}
}

func TestHandleListOpenAndBack(t *testing.T) {
	c, adapter, _ := boundBrowseCore(t, numberedMemos(3, "work"))

	ev := sampleEvent(channel.KindCommand)
	ev.Command = channel.Command{Name: channel.CommandList}
	if err := c.Handle(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	first := adapter.replies[0].Browse.Items[0].Name

	adapter.replies = nil
	open := sampleEvent(channel.KindAction)
	open.Origin.AckID = "cbq-open"
	open.Action = channel.Action{Name: channel.ActionOpen, Resource: "0"}
	if err := c.Handle(context.Background(), open); err != nil {
		t.Fatal(err)
	}
	detail := adapter.replies[0].Browse
	if detail.View != channel.BrowseDetail || detail.Memo == nil || detail.Memo.Name != first {
		t.Fatalf("unexpected detail: %#v", detail)
	}

	adapter.replies = nil
	back := open
	back.Action = channel.Action{Name: channel.ActionBack, Resource: channel.BrowsePlaceholder}
	if err := c.Handle(context.Background(), back); err != nil {
		t.Fatal(err)
	}
	if adapter.replies[0].Browse.View != channel.BrowseList || len(adapter.replies[0].Browse.Items) != 3 {
		t.Fatalf("expected list after back, got %#v", adapter.replies[0].Browse)
	}
}

func TestHandleTagsFiltersList(t *testing.T) {
	memos := numberedMemos(3, "work")
	memos["memos/inbox-1"] = &v1pb.Memo{
		Name:       "memos/inbox-1",
		Content:    "inbox note",
		Snippet:    "inbox note",
		Tags:       []string{"inbox"},
		UpdateTime: timestamppb.New(time.Unix(5000, 0)),
	}
	c, adapter, _ := boundBrowseCore(t, memos)

	ev := sampleEvent(channel.KindCommand)
	ev.Command = channel.Command{Name: channel.CommandTags}
	if err := c.Handle(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	tags := adapter.replies[0].Browse
	if tags.View != channel.BrowseTags || len(tags.Tags) != 2 {
		t.Fatalf("expected two tags, got %#v", tags)
	}
	inboxIdx := -1
	for i, tag := range tags.Tags {
		if tag.Name == "inbox" {
			inboxIdx = i
		}
	}
	if inboxIdx < 0 {
		t.Fatalf("inbox tag missing: %#v", tags.Tags)
	}

	adapter.replies = nil
	pick := sampleEvent(channel.KindAction)
	pick.Origin.AckID = "cbq-tag"
	pick.Action = channel.Action{Name: channel.ActionTag, Resource: fmt.Sprintf("%d", inboxIdx)}
	if err := c.Handle(context.Background(), pick); err != nil {
		t.Fatal(err)
	}
	page := adapter.replies[0].Browse
	if page.View != channel.BrowseList || page.Tag != "inbox" || len(page.Items) != 1 || page.Items[0].Name != "memos/inbox-1" {
		t.Fatalf("expected inbox list, got %#v", page)
	}
}

func TestHandleSearchUsesBrowseList(t *testing.T) {
	c, adapter, _ := boundBrowseCore(t, numberedMemos(3, "work"))

	ev := sampleEvent(channel.KindCommand)
	ev.Command = channel.Command{Name: channel.CommandSearch, Args: "note 2"}
	if err := c.Handle(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	page := adapter.replies[0].Browse
	if page == nil || page.View != channel.BrowseList || page.Query != "note 2" || len(page.Items) != 1 {
		t.Fatalf("unexpected search browse: %#v", page)
	}
}

func TestHandlePendingEditUpdatesContent(t *testing.T) {
	c, adapter, backend := boundBrowseCore(t, numberedMemos(1, "work"))

	ev := sampleEvent(channel.KindCommand)
	ev.Command = channel.Command{Name: channel.CommandList}
	if err := c.Handle(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	open := sampleEvent(channel.KindAction)
	open.Origin.AckID = "cbq-open"
	open.Origin.MessageID = "77"
	open.Action = channel.Action{Name: channel.ActionOpen, Resource: "0"}
	if err := c.Handle(context.Background(), open); err != nil {
		t.Fatal(err)
	}
	edit := open
	edit.Action = channel.Action{Name: channel.ActionEdit, Resource: channel.BrowsePlaceholder}
	if err := c.Handle(context.Background(), edit); err != nil {
		t.Fatal(err)
	}
	if adapter.replies[len(adapter.replies)-1].Browse.View != channel.BrowseEditPrompt {
		t.Fatalf("expected edit prompt, got %#v", adapter.replies[len(adapter.replies)-1])
	}

	msg := sampleEvent(channel.KindMessage)
	msg.TextMarkdown = "replaced body"
	if err := c.Handle(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
	if backend.memos["memos/memo-01"].Content != "replaced body" {
		t.Fatalf("content not updated: %#v", backend.memos["memos/memo-01"])
	}
	got := adapter.replies[len(adapter.replies)-1]
	if got.Kind != channel.OutboundBrowse || got.Browse.View != channel.BrowseDetail || got.Browse.Content != "replaced body" {
		t.Fatalf("expected refreshed detail, got %#v", got)
	}
}

func TestHandleDeleteReturnsToList(t *testing.T) {
	c, adapter, backend := boundBrowseCore(t, numberedMemos(2, "work"))

	ev := sampleEvent(channel.KindCommand)
	ev.Command = channel.Command{Name: channel.CommandList}
	if err := c.Handle(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	open := sampleEvent(channel.KindAction)
	open.Origin.AckID = "cbq-open"
	open.Action = channel.Action{Name: channel.ActionOpen, Resource: "0"}
	if err := c.Handle(context.Background(), open); err != nil {
		t.Fatal(err)
	}
	name := adapter.replies[len(adapter.replies)-1].Browse.Memo.Name
	del := open
	del.Action = channel.Action{Name: channel.ActionDeleteConfirm, Resource: channel.BrowsePlaceholder}
	if err := c.Handle(context.Background(), del); err != nil {
		t.Fatal(err)
	}
	if _, ok := backend.memos[name]; ok {
		t.Fatalf("memo %s still present", name)
	}
	page := adapter.replies[len(adapter.replies)-1].Browse
	if page.View != channel.BrowseList || len(page.Items) != 1 {
		t.Fatalf("expected remaining list, got %#v", page)
	}
}
