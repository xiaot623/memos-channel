package core

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
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
	instanceURL string
}

func (f *fakeClient) GetInstanceProfile(context.Context) (*v1pb.InstanceProfile, error) {
	url := f.instanceURL
	if url == "" {
		url = "https://memos.example"
	}
	return &v1pb.InstanceProfile{InstanceUrl: url}, nil
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
	if memo.Content != "" {
		stored.Content = memo.Content
		stored.Snippet = memo.Content
	}
	return nil
}

func (a *fakeAuthed) ListMemos(_ context.Context, pageSize int32, pageToken, _, filter string) (ListMemosPage, error) {
	if _, err := a.GetCurrentUser(context.Background()); err != nil {
		return ListMemosPage{}, err
	}
	out := make([]*v1pb.Memo, 0, len(a.parent.memos))
	for _, memo := range a.parent.memos {
		if !fakeMemoMatches(memo, filter) {
			continue
		}
		clone := *memo
		out = append(out, &clone)
	}
	sort.Slice(out, func(i, j int) bool {
		ti, tj := out[i].GetUpdateTime(), out[j].GetUpdateTime()
		if ti != nil && tj != nil && !ti.AsTime().Equal(tj.AsTime()) {
			return ti.AsTime().After(tj.AsTime())
		}
		return out[i].Name > out[j].Name
	})
	offset := 0
	if pageToken != "" {
		offset, _ = strconv.Atoi(pageToken)
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if offset > len(out) {
		offset = len(out)
	}
	end := offset + int(pageSize)
	next := ""
	if end < len(out) {
		next = strconv.Itoa(end)
	} else {
		end = len(out)
	}
	return ListMemosPage{Memos: out[offset:end], NextPageToken: next}, nil
}

func (a *fakeAuthed) GetUserStats(context.Context, string) (*v1pb.UserStats, error) {
	counts := map[string]int32{}
	for _, memo := range a.parent.memos {
		for _, tag := range memo.Tags {
			counts[tag]++
		}
	}
	return &v1pb.UserStats{TagCount: counts}, nil
}

func (a *fakeAuthed) DeleteMemo(_ context.Context, name string) error {
	if _, ok := a.parent.memos[name]; !ok {
		return fmt.Errorf("not found")
	}
	delete(a.parent.memos, name)
	return nil
}

func fakeMemoMatches(memo *v1pb.Memo, filter string) bool {
	if strings.TrimSpace(filter) == "" {
		return true
	}
	for _, part := range strings.Split(filter, "&&") {
		part = strings.TrimSpace(part)
		switch {
		case strings.HasPrefix(part, "content.contains(") && strings.HasSuffix(part, ")"):
			q, err := strconv.Unquote(part[len("content.contains(") : len(part)-1])
			if err != nil || !strings.Contains(memo.Content, q) {
				return false
			}
		case strings.HasPrefix(part, "creator =="):
			want, err := strconv.Unquote(strings.TrimSpace(strings.TrimPrefix(part, "creator ==")))
			if err != nil {
				return false
			}
			if memo.Creator != "" && memo.Creator != want {
				return false
			}
		case strings.HasSuffix(part, " in tags"):
			want, err := strconv.Unquote(strings.TrimSpace(strings.TrimSuffix(part, " in tags")))
			if err != nil {
				return false
			}
			found := false
			for _, tag := range memo.Tags {
				if tag == want {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}
	return true
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
	return newTestCoreWithPublicURL(t, backend, "")
}

func newTestCoreWithPublicURL(t *testing.T, backend Client, publicURL string) (*Core, *fakeAdapter, *store.Store) {
	t.Helper()
	st := store.New(filepath.Join(t.TempDir(), "data.txt"))
	if err := st.Init(); err != nil {
		t.Fatalf("init store: %v", err)
	}
	c := NewWithClient(st, backend, publicURL)
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
	}, "")
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
	if saved.Memo == nil || saved.Memo.UID != "memo-1" || saved.Memo.URL != "" {
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
	if len(adapter.replies) != 1 || adapter.replies[0].Kind != channel.OutboundBrowse {
		t.Fatalf("expected search browse list, got %#v", adapter.replies)
	}
	if adapter.replies[0].Browse == nil || len(adapter.replies[0].Browse.Items) != 1 {
		t.Fatalf("expected one search result, got %#v", adapter.replies[0].Browse)
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

func boundSaveBackend() *fakeClient {
	return &fakeClient{
		users: map[string]*v1pb.User{
			"tok": {Name: "users/alice", DisplayName: "Alice"},
		},
		memos:       map[string]*v1pb.Memo{},
		attachments: map[string]int{},
	}
}

func saveBoundNote(t *testing.T, c *Core, adapter *fakeAdapter) *channel.MemoInfo {
	t.Helper()
	adapter.replies = nil
	ev := sampleEvent(channel.KindMessage)
	ev.TextMarkdown = "note body"
	if err := c.Handle(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if len(adapter.replies) != 1 || adapter.replies[0].Memo == nil {
		t.Fatalf("expected saved memo, got %#v", adapter.replies)
	}
	return adapter.replies[0].Memo
}

func TestMemoURLPrefersPublicHTTPS(t *testing.T) {
	backend := boundSaveBackend()
	c, adapter, st := newTestCoreWithPublicURL(t, backend, "https://keep.example/")
	st.Set(channel.Telegram, "42", "tok")
	if err := c.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := saveBoundNote(t, c, adapter)
	if got.URL != "https://keep.example/memos/memo-1" {
		t.Fatalf("url: got %q", got.URL)
	}
}

func TestMemoURLFallsBackToInstanceHTTPS(t *testing.T) {
	backend := boundSaveBackend()
	c, adapter, st := newTestCoreWithPublicURL(t, backend, "http://keep.example")
	st.Set(channel.Telegram, "42", "tok")
	if err := c.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := saveBoundNote(t, c, adapter)
	if got.URL != "https://memos.example/memos/memo-1" {
		t.Fatalf("url: got %q", got.URL)
	}
}

func TestMemoURLIgnoresHTTPOrigins(t *testing.T) {
	backend := boundSaveBackend()
	backend.instanceURL = "http://memos:5230"
	c, adapter, st := newTestCoreWithPublicURL(t, backend, "http://localhost:5230")
	st.Set(channel.Telegram, "42", "tok")
	if err := c.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := saveBoundNote(t, c, adapter)
	if got.URL != "" {
		t.Fatalf("url: got %q, want empty", got.URL)
	}
}
