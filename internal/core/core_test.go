package core

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/usememos/memogram/internal/channel"
	"github.com/usememos/memogram/internal/store"
	v1pb "github.com/usememos/memos/proto/gen/api/v1"
)

type fakeAdapter struct {
	replies []channel.OutboundMessage
}

func (f *fakeAdapter) Name() string { return channel.Telegram }

func (f *fakeAdapter) Start(context.Context, channel.HandleFunc) error { return nil }

func (f *fakeAdapter) Reply(_ context.Context, _ channel.Origin, msg channel.OutboundMessage) error {
	f.replies = append(f.replies, msg)
	return nil
}

type fakeClient struct {
	users       map[string]*v1pb.User
	memos       map[string]*v1pb.Memo
	attachments map[string]int
	seq         int
}

func (f *fakeClient) GetInstanceProfile(context.Context) (*v1pb.InstanceProfile, error) {
	return &v1pb.InstanceProfile{InstanceUrl: "https://memos.example"}, nil
}

func (f *fakeClient) Authenticated(token string) AuthedClient {
	return &fakeAuthed{parent: f, token: token}
}

type fakeAuthed struct {
	parent *fakeClient
	token  string
}

func (a *fakeAuthed) GetCurrentUser(context.Context) (*v1pb.User, error) {
	user, ok := a.parent.users[a.token]
	if !ok {
		return nil, fmt.Errorf("unauthorized")
	}
	return user, nil
}

func (a *fakeAuthed) CreateMemo(_ context.Context, content string) (*v1pb.Memo, error) {
	if _, err := a.GetCurrentUser(context.Background()); err != nil {
		return nil, err
	}
	a.parent.seq++
	name := fmt.Sprintf("memos/memo-%d", a.parent.seq)
	memo := &v1pb.Memo{
		Name:       name,
		Content:    content,
		Visibility: v1pb.Visibility_PRIVATE,
	}
	a.parent.memos[name] = memo
	return memo, nil
}

func (a *fakeAuthed) GetMemo(_ context.Context, name string) (*v1pb.Memo, error) {
	memo, ok := a.parent.memos[name]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	clone := *memo
	return &clone, nil
}

func (a *fakeAuthed) UpdateMemo(_ context.Context, memo *v1pb.Memo, _ []string) error {
	stored, ok := a.parent.memos[memo.Name]
	if !ok {
		return fmt.Errorf("not found")
	}
	stored.Visibility = memo.Visibility
	stored.Pinned = memo.Pinned
	return nil
}

func (a *fakeAuthed) ListMemos(context.Context, int32, string) ([]*v1pb.Memo, error) {
	out := make([]*v1pb.Memo, 0, len(a.parent.memos))
	for _, memo := range a.parent.memos {
		clone := *memo
		out = append(out, &clone)
	}
	return out, nil
}

func (a *fakeAuthed) CreateAttachment(_ context.Context, _, _ string, _ []byte, memoName string) error {
	a.parent.attachments[memoName]++
	return nil
}

type staticAttachment struct {
	filename string
	data     []byte
}

func (s staticAttachment) Filename() string { return s.filename }

func (s staticAttachment) Download(context.Context) (*channel.Attachment, error) {
	return &channel.Attachment{
		Filename:    s.filename,
		ContentType: "text/plain",
		Bytes:       s.data,
	}, nil
}

func newTestCore(t *testing.T, backend Client) (*Core, *fakeAdapter, *store.Store) {
	t.Helper()
	st := store.New(filepath.Join(t.TempDir(), "data.txt"))
	if err := st.Init(); err != nil {
		t.Fatalf("init store: %v", err)
	}
	c := NewWithClient(st, backend, "http://localhost:5230")
	adapter := &fakeAdapter{}
	c.Register(adapter)
	return c, adapter, st
}

func sampleEvent(kind channel.Kind) channel.InboundEvent {
	return channel.InboundEvent{
		Channel:        channel.Telegram,
		PlatformUserID: "42",
		Origin:         channel.Origin{ChatID: "99", MessageID: "1"},
		Kind:           kind,
	}
}

func TestStartRequiresAdapter(t *testing.T) {
	st := store.New(filepath.Join(t.TempDir(), "data.txt"))
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	c := NewWithClient(st, &fakeClient{
		users:       map[string]*v1pb.User{},
		memos:       map[string]*v1pb.Memo{},
		attachments: map[string]int{},
	}, "http://localhost")
	if err := c.Start(context.Background()); err == nil {
		t.Fatal("expected error when no adapters are registered")
	}
}

func TestHandleUnboundMessagePromptsBind(t *testing.T) {
	c, adapter, _ := newTestCore(t, &fakeClient{
		users:       map[string]*v1pb.User{},
		memos:       map[string]*v1pb.Memo{},
		attachments: map[string]int{},
	})
	ev := sampleEvent(channel.KindMessage)
	ev.TextMarkdown = "hello"
	if err := c.Handle(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if len(adapter.replies) != 1 || adapter.replies[0].Kind != channel.OutboundPromptBind {
		t.Fatalf("expected PromptBind, got %#v", adapter.replies)
	}
}

func TestHandleBind(t *testing.T) {
	backend := &fakeClient{
		users: map[string]*v1pb.User{
			"tok": {Name: "users/alice", DisplayName: "Alice"},
		},
		memos:       map[string]*v1pb.Memo{},
		attachments: map[string]int{},
	}
	c, adapter, st := newTestCore(t, backend)

	ev := sampleEvent(channel.KindCommand)
	ev.Command = channel.Command{Name: channel.CommandStart}
	if err := c.Handle(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if len(adapter.replies) != 1 || adapter.replies[0].Kind != channel.OutboundPromptUsage {
		t.Fatalf("expected usage prompt, got %#v", adapter.replies)
	}

	adapter.replies = nil
	ev.Command.Args = "bad"
	if err := c.Handle(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if len(adapter.replies) != 1 || adapter.replies[0].Error != "Invalid access token" {
		t.Fatalf("expected invalid token, got %#v", adapter.replies)
	}

	adapter.replies = nil
	ev.Command.Args = "tok"
	if err := c.Handle(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if len(adapter.replies) != 1 || adapter.replies[0].Kind != channel.OutboundBound || adapter.replies[0].User != "Alice" {
		t.Fatalf("expected bound hello, got %#v", adapter.replies)
	}
	if token, ok := st.Get(channel.Telegram, "42"); !ok || token != "tok" {
		t.Fatalf("expected store token tok, got %q ok=%v", token, ok)
	}
}

func TestHandleCreateMemoAndSearch(t *testing.T) {
	backend := &fakeClient{
		users: map[string]*v1pb.User{
			"tok": {Name: "users/alice", DisplayName: "Alice"},
		},
		memos:       map[string]*v1pb.Memo{},
		attachments: map[string]int{},
	}
	c, adapter, st := newTestCore(t, backend)
	st.Set(channel.Telegram, "42", "tok")

	ev := sampleEvent(channel.KindMessage)
	ev.TextMarkdown = "note body"
	ev.Attachments = []channel.AttachmentRef{staticAttachment{filename: "a.txt", data: []byte("hi")}}
	if err := c.Handle(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if len(adapter.replies) != 1 || adapter.replies[0].Kind != channel.OutboundSaved {
		t.Fatalf("expected saved, got %#v", adapter.replies)
	}
	saved := adapter.replies[0]
	if saved.Memo == nil || saved.Memo.UID != "memo-1" || saved.Memo.URL != "http://localhost:5230/memos/memo-1" {
		t.Fatalf("unexpected memo info: %#v", saved.Memo)
	}
	if backend.attachments["memos/memo-1"] != 1 {
		t.Fatalf("expected one attachment, got %d", backend.attachments["memos/memo-1"])
	}

	adapter.replies = nil
	search := sampleEvent(channel.KindCommand)
	search.Command = channel.Command{Name: channel.CommandSearch, Args: "note"}
	if err := c.Handle(context.Background(), search); err != nil {
		t.Fatal(err)
	}
	if len(adapter.replies) != 1 || adapter.replies[0].Kind != channel.OutboundSearchList || len(adapter.replies[0].Results) != 1 {
		t.Fatalf("expected search results, got %#v", adapter.replies)
	}
}

func TestHandleActionUpdatesMemo(t *testing.T) {
	backend := &fakeClient{
		users: map[string]*v1pb.User{
			"tok": {Name: "users/alice", DisplayName: "Alice"},
		},
		memos: map[string]*v1pb.Memo{
			"memos/memo-1": {Name: "memos/memo-1", Visibility: v1pb.Visibility_PRIVATE},
		},
		attachments: map[string]int{},
	}
	c, adapter, st := newTestCore(t, backend)
	st.Set(channel.Telegram, "42", "tok")

	ev := sampleEvent(channel.KindAction)
	ev.Origin.AckID = "cbq-1"
	ev.Action = channel.Action{Name: channel.ActionPublic, Resource: "memos/memo-1"}
	if err := c.Handle(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if len(adapter.replies) != 1 || adapter.replies[0].Kind != channel.OutboundSaved {
		t.Fatalf("expected saved update, got %#v", adapter.replies)
	}
	if adapter.replies[0].Memo.Visibility != "PUBLIC" {
		t.Fatalf("expected PUBLIC, got %q", adapter.replies[0].Memo.Visibility)
	}
	if backend.memos["memos/memo-1"].Visibility != v1pb.Visibility_PUBLIC {
		t.Fatal("store memo was not updated")
	}
}

func TestHandleMediaGroupReusesMemo(t *testing.T) {
	backend := &fakeClient{
		users: map[string]*v1pb.User{
			"tok": {Name: "users/alice"},
		},
		memos:       map[string]*v1pb.Memo{},
		attachments: map[string]int{},
	}
	c, _, st := newTestCore(t, backend)
	st.Set(channel.Telegram, "42", "tok")

	first := sampleEvent(channel.KindMessage)
	first.GroupID = "album-1"
	first.TextMarkdown = "album"
	second := first
	if err := c.Handle(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	if err := c.Handle(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	if backend.seq != 1 {
		t.Fatalf("expected a single memo, created %d", backend.seq)
	}
}
