package feishu

import (
	"context"
	"testing"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/usememos/memogram/internal/channel"
)

func TestParseCommand(t *testing.T) {
	tests := []struct {
		in     string
		want   channel.Command
		wantOK bool
	}{
		{in: "/start abc", want: channel.Command{Name: channel.CommandStart, Args: "abc"}, wantOK: true},
		{in: "/start@bot tok", want: channel.Command{Name: channel.CommandStart, Args: "tok"}, wantOK: true},
		{in: "  /list  ", want: channel.Command{Name: channel.CommandList}, wantOK: true},
		{in: "/search hello world", want: channel.Command{Name: channel.CommandSearch, Args: "hello world"}, wantOK: true},
		{in: "/tags", want: channel.Command{Name: channel.CommandTags}, wantOK: true},
		{in: "/cancel extra", want: channel.Command{Name: channel.CommandCancel, Args: "extra"}, wantOK: true},
		{in: "/help", want: channel.Command{Name: channel.CommandHelp}, wantOK: true},
		{in: "/HELP", want: channel.Command{Name: channel.CommandHelp}, wantOK: true},
		{in: "/unknown", wantOK: false},
		{in: "not a command", wantOK: false},
	}
	for _, tt := range tests {
		got, ok := parseCommand(tt.in)
		if ok != tt.wantOK {
			t.Fatalf("%q: ok=%v want %v", tt.in, ok, tt.wantOK)
		}
		if !ok {
			continue
		}
		if got != tt.want {
			t.Fatalf("%q: got %+v want %+v", tt.in, got, tt.want)
		}
	}
}

func TestActionFromValue(t *testing.T) {
	got, ok := actionFromValue(map[string]interface{}{"name": "open", "resource": "0"})
	if !ok || got.Name != channel.ActionOpen || got.Resource != "0" {
		t.Fatalf("got %+v ok=%v", got, ok)
	}
	if _, ok := actionFromValue(map[string]interface{}{"name": "open"}); ok {
		t.Fatal("missing resource should fail")
	}
}

func ptr(s string) *string { return &s }

func TestMessageEventTextCommand(t *testing.T) {
	a := New(Options{})
	ev, err := a.messageEvent(&larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Sender: &larkim.EventSender{SenderId: &larkim.UserId{OpenId: ptr("ou_1")}},
			Message: &larkim.EventMessage{
				ChatId:      ptr("oc_1"),
				MessageId:   ptr("om_1"),
				MessageType: ptr("text"),
				Content:     ptr(`{"text":"/start token"}`),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ev.Channel != channel.Feishu || ev.PlatformUserID != "ou_1" {
		t.Fatalf("identity %+v", ev)
	}
	if ev.Kind != channel.KindCommand || ev.Command.Name != channel.CommandStart || ev.Command.Args != "token" {
		t.Fatalf("command %+v", ev)
	}
}

func TestMessageEventImage(t *testing.T) {
	a := New(Options{})
	ev, err := a.messageEvent(&larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Sender: &larkim.EventSender{SenderId: &larkim.UserId{OpenId: ptr("ou_1")}},
			Message: &larkim.EventMessage{
				ChatId:      ptr("oc_1"),
				MessageId:   ptr("om_1"),
				MessageType: ptr("image"),
				Content:     ptr(`{"image_key":"img_abc"}`),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ev.Kind != channel.KindMessage || len(ev.Attachments) != 1 {
		t.Fatalf("attachments %+v", ev)
	}
	if ev.Attachments[0].Filename() != "img_abc.png" {
		t.Fatalf("filename %q", ev.Attachments[0].Filename())
	}
}

func TestReplyCallbackStoresToast(t *testing.T) {
	a := New(Options{})
	slot := &pendingCard{}
	a.pending.Store("tok", slot)
	err := a.Reply(context.Background(), channel.Origin{AckID: "tok"}, channel.OutboundMessage{
		Kind: channel.OutboundPromptBind,
	})
	if err != nil {
		t.Fatal(err)
	}
	if slot.resp == nil || slot.resp.Toast == nil || slot.resp.Toast.Content != promptBindText() {
		t.Fatalf("resp %+v", slot.resp)
	}
}
