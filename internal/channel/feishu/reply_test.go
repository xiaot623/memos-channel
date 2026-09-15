package feishu

import (
	"strings"
	"testing"

	"github.com/usememos/memogram/internal/channel"
)

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
