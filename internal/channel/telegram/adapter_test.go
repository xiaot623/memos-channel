package telegram

import (
	"strings"
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/usememos/memogram/internal/channel"
)

func TestMessageEventHelp(t *testing.T) {
	a := New(Options{})
	ev := a.messageEvent(&models.Update{
		Message: &models.Message{
			ID:   1,
			Text: "/help",
			From: &models.User{ID: 42, Username: "alice"},
			Chat: models.Chat{ID: 99},
		},
	})
	if ev.Kind != channel.KindCommand || ev.Command.Name != channel.CommandHelp {
		t.Fatalf("expected help command, got %+v", ev.Command)
	}
}

func TestHelpUsageText(t *testing.T) {
	text := usageText(channel.CommandHelp)
	if text != channel.HelpText() {
		t.Fatalf("expected shared help text, got %q", text)
	}
	for _, cmd := range []string{"/start", "/list", "/tags", "/search", "/cancel", "/help"} {
		if !strings.Contains(text, cmd) {
			t.Fatalf("help text missing %s: %q", cmd, text)
		}
	}
}
