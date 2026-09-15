package feishu

import (
	"strings"
	"testing"
	"time"

	"github.com/usememos/memogram/internal/channel"
)

func TestTruncateButtonFlattensAndEllipsizes(t *testing.T) {
	got := truncateRunes("hello\n\nworld   more", buttonMaxRunes)
	if got != "hello world more" {
		t.Fatalf("flatten: got %q", got)
	}
	long := "abcdefghijklmnopqrstuvwxyz0123456789"
	got = truncateRunes(long, buttonMaxRunes)
	if got != "abcdefghijklmnopqrstuvwxyz01..." {
		t.Fatalf("ellipsize: got %q", got)
	}
	if got := truncateRunes("   ", buttonMaxRunes); got != "Untitled" {
		t.Fatalf("empty: got %q", got)
	}
}

func TestListButtonTextPrefixesMMDD(t *testing.T) {
	updated := time.Date(2026, 9, 13, 15, 0, 0, 0, time.Local)
	got := listButtonText(channel.MemoSummary{
		Snippet:   "Daily News Report",
		UpdatedAt: updated,
	})
	if got != "(0913) Daily News Report" {
		t.Fatalf("got %q", got)
	}
}

func TestClipCardMarkdown(t *testing.T) {
	if got := clipCardMarkdown("  "); got != "(empty memo)" {
		t.Fatalf("empty: %q", got)
	}
	long := strings.Repeat("a", cardMaxRunes+10)
	got := clipCardMarkdown(long)
	if !strings.HasSuffix(got, "...") || len([]rune(got)) != cardMaxRunes {
		t.Fatalf("clip len=%d suffix=%q", len([]rune(got)), got[len(got)-3:])
	}
}

func TestBrowseCardCallbackValues(t *testing.T) {
	card := listCard(&channel.BrowsePayload{
		Start:   1,
		End:     1,
		HasNext: true,
		Items: []channel.MemoSummary{
			{Snippet: "hello"},
		},
	})
	body := card["body"].(map[string]any)
	elements := body["elements"].([]any)
	name, resource, ok := callbackValue(elements[0])
	if !ok || name != channel.ActionOpen || resource != "0" {
		t.Fatalf("open button: name=%q resource=%q ok=%v", name, resource, ok)
	}
	nav := elements[1].(map[string]any)
	cols := nav["columns"].([]any)
	col := cols[0].(map[string]any)
	btn := col["elements"].([]any)[0]
	name, resource, ok = callbackValue(btn)
	if !ok || name != channel.ActionNext || resource != channel.BrowsePlaceholder {
		t.Fatalf("next button: name=%q resource=%q ok=%v", name, resource, ok)
	}
}

func TestSavedCardActions(t *testing.T) {
	card := savedCard("Content saved", &channel.MemoInfo{Name: "memos/abc", Visibility: "PRIVATE"}, channel.DefaultMemoActions())
	body := card["body"].(map[string]any)
	elements := body["elements"].([]any)
	if len(elements) != 2 {
		t.Fatalf("elements=%d", len(elements))
	}
	row := elements[1].(map[string]any)
	cols := row["columns"].([]any)
	if len(cols) != 3 {
		t.Fatalf("actions=%d", len(cols))
	}
	col := cols[0].(map[string]any)
	name, resource, ok := callbackValue(col["elements"].([]any)[0])
	if !ok || name != channel.ActionPublic || resource != "memos/abc" {
		t.Fatalf("public: name=%q resource=%q ok=%v", name, resource, ok)
	}
}

func TestDetailCardOpenURL(t *testing.T) {
	card := detailCard(&channel.BrowsePayload{
		Content: "**hi**",
		Memo:    &channel.MemoInfo{URL: "https://keep.example/memos/1"},
	})
	body := card["body"].(map[string]any)
	elements := body["elements"].([]any)
	open := elements[len(elements)-1].(map[string]any)
	behaviors := open["behaviors"].([]any)
	behavior := behaviors[0].(map[string]any)
	if behavior["type"] != "open_url" || behavior["default_url"] != "https://keep.example/memos/1" {
		t.Fatalf("open url button: %+v", behavior)
	}
}

func callbackValue(el any) (name, resource string, ok bool) {
	btn, ok := el.(map[string]any)
	if !ok {
		return "", "", false
	}
	behaviors, _ := btn["behaviors"].([]any)
	if len(behaviors) == 0 {
		return "", "", false
	}
	behavior, _ := behaviors[0].(map[string]any)
	value, _ := behavior["value"].(map[string]string)
	if value["name"] == "" || value["resource"] == "" {
		return "", "", false
	}
	return value["name"], value["resource"], true
}
