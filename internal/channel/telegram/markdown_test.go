package telegram

import (
	"strings"
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/usememos/memogram/internal/channel"
)

func TestFormatMemoContentRendersBold(t *testing.T) {
	got := formatMemoContent("hello **world**")
	if strings.Contains(got.Text, "**") {
		t.Fatalf("still markdown: %q", got.Text)
	}
	if got.Text != "hello world" {
		t.Fatalf("text: got %q", got.Text)
	}
	if len(got.Entities) != 1 {
		t.Fatalf("entities: got %+v", got.Entities)
	}
	e := got.Entities[0]
	if e.Type != models.MessageEntityTypeBold || e.Offset != 6 || e.Length != 5 {
		t.Fatalf("bold entity: %+v", e)
	}
}

func TestFormatMemoContentEmpty(t *testing.T) {
	got := formatMemoContent("   ")
	if got.Text != "(empty memo)" {
		t.Fatalf("got %q", got.Text)
	}
	if got.Entities != nil {
		t.Fatalf("entities: %+v", got.Entities)
	}
}

func TestFormatBrowseListHasNoEntities(t *testing.T) {
	got := formatBrowse(&channel.BrowsePayload{
		View:  channel.BrowseList,
		Start: 1,
		End:   1,
		Items: []channel.MemoSummary{{Content: "**bold**"}},
	})
	if got.Entities != nil {
		t.Fatalf("list entities: %+v", got.Entities)
	}
	if got.Text != "Memos · updated · 1–1" {
		t.Fatalf("list text: got %q", got.Text)
	}
}

func TestBrowseDetailOpenButton(t *testing.T) {
	kb := browseKeyboard(&channel.BrowsePayload{
		View: channel.BrowseDetail,
		Memo: &channel.MemoInfo{URL: "https://keep.example/memos/abc"},
	})
	if kb == nil || len(kb.InlineKeyboard) != 2 {
		t.Fatalf("rows: %+v", kb)
	}
	open := kb.InlineKeyboard[1]
	if len(open) != 1 || open[0].Text != "Open" || open[0].URL != "https://keep.example/memos/abc" {
		t.Fatalf("open: %+v", open)
	}
	if open[0].CallbackData != "" {
		t.Fatalf("url button should not have callback: %q", open[0].CallbackData)
	}
}

func TestBrowseDetailOmitsOpenWithoutURL(t *testing.T) {
	kb := browseKeyboard(&channel.BrowsePayload{View: channel.BrowseDetail})
	if kb == nil || len(kb.InlineKeyboard) != 1 {
		t.Fatalf("rows: %+v", kb)
	}
}
