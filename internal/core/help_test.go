package core

import (
	"context"
	"strings"
	"testing"

	"github.com/usememos/memogram/internal/channel"
	v1pb "github.com/usememos/memos/proto/gen/api/v1"
)

func TestHandleHelp(t *testing.T) {
	c, adapter, _ := newTestCore(t, &fakeClient{
		users: map[string]*v1pb.User{
			"tok": {Name: "users/alice", DisplayName: "Alice"},
		},
		memos:       map[string]*v1pb.Memo{},
		attachments: map[string]int{},
	})
	ev := sampleEvent(channel.KindCommand)
	ev.Command = channel.Command{Name: channel.CommandHelp}
	if err := c.Handle(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if len(adapter.replies) != 1 {
		t.Fatalf("expected one reply, got %#v", adapter.replies)
	}
	got := adapter.replies[0]
	if got.Kind != channel.OutboundPromptUsage || got.Prompt != channel.CommandHelp {
		t.Fatalf("expected PromptUsage help, got %#v", got)
	}
	if text := channel.HelpText(); !strings.Contains(text, "/help") || !strings.Contains(text, "/list") {
		t.Fatalf("help text missing commands: %q", text)
	}
}
