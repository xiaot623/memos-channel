package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseLineLegacyTokenWithColon(t *testing.T) {
	channel, userID, token, legacy, ok := parseLine("123:abc:def")
	if !ok {
		t.Fatal("expected legacy line to parse")
	}
	if !legacy {
		t.Fatal("expected legacy flag")
	}
	if channel != "telegram" {
		t.Fatalf("expected telegram channel, got %q", channel)
	}
	if userID != "123" {
		t.Fatalf("expected userID 123, got %q", userID)
	}
	if token != "abc:def" {
		t.Fatalf("expected token with colon, got %q", token)
	}
}

func TestParseLineJSON(t *testing.T) {
	channel, userID, token, legacy, ok := parseLine(`{"channel":"telegram","user_id":"42","token":"tok:en"}`)
	if !ok || legacy {
		t.Fatalf("ok=%v legacy=%v", ok, legacy)
	}
	if channel != "telegram" || userID != "42" || token != "tok:en" {
		t.Fatalf("unexpected record: %s %s %s", channel, userID, token)
	}
}

func TestSaveAndLoadUserAccessTokens(t *testing.T) {
	dataPath := filepath.Join(t.TempDir(), "data.txt")

	st := New(dataPath)
	if err := st.Init(); err != nil {
		t.Fatalf("init store: %v", err)
	}

	st.Set("telegram", "42", "token-one")
	st.Set("telegram", "7", "token:two")

	reloaded := New(dataPath)
	if err := reloaded.Init(); err != nil {
		t.Fatalf("init reloaded store: %v", err)
	}

	token, ok := reloaded.Get("telegram", "42")
	if !ok || token != "token-one" {
		t.Fatalf("expected token-one for user 42, got %q", token)
	}

	token, ok = reloaded.Get("telegram", "7")
	if !ok || token != "token:two" {
		t.Fatalf("expected token:two for user 7, got %q", token)
	}
}

func TestInitMigratesLegacyDataFile(t *testing.T) {
	dataPath := filepath.Join(t.TempDir(), "data.txt")
	if err := os.WriteFile(dataPath, []byte("123:abc:def\n# comment\n\n7:plain\n"), 0o644); err != nil {
		t.Fatalf("write legacy file: %v", err)
	}

	st := New(dataPath)
	if err := st.Init(); err != nil {
		t.Fatalf("init store: %v", err)
	}

	token, ok := st.Get("telegram", "123")
	if !ok || token != "abc:def" {
		t.Fatalf("expected migrated token abc:def, got %q ok=%v", token, ok)
	}
	token, ok = st.Get("telegram", "7")
	if !ok || token != "plain" {
		t.Fatalf("expected migrated token plain, got %q ok=%v", token, ok)
	}

	raw, err := os.ReadFile(dataPath)
	if err != nil {
		t.Fatalf("read rewritten file: %v", err)
	}
	body := string(raw)
	if !strings.Contains(body, `"channel":"telegram"`) || !strings.Contains(body, `"user_id":"123"`) {
		t.Fatalf("expected JSONL rewrite, got %q", body)
	}
}
