package telegram

import (
	"testing"
	"time"

	"github.com/usememos/memogram/internal/channel"
)

func TestTruncateButtonFlattensAndEllipsizes(t *testing.T) {
	got := truncateButton("hello\n\nworld   more")
	if got != "hello world more" {
		t.Fatalf("flatten: got %q", got)
	}
	long := "abcdefghijklmnopqrstuvwxyz0123456789"
	got = truncateButton(long)
	if got != "abcdefghijklmnopqrstuvwxyz01..." {
		t.Fatalf("ellipsize: got %q", got)
	}
	if got := truncateButton("   "); got != "Untitled" {
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

	long := listButtonText(channel.MemoSummary{
		Snippet:   "abcdefghijklmnopqrstuvwxyz0123456789",
		UpdatedAt: updated,
	})
	if got := long; got != "(0913) abcdefghijklmnopqrstu..." {
		t.Fatalf("truncated with date: got %q", got)
	}
}
