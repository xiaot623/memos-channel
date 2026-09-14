package telegram

import "testing"

func TestIsUserAllowedEmptyAllowlist(t *testing.T) {
	a := New(Options{})
	if !a.isUserAllowed("") || !a.isUserAllowed("anyone") {
		t.Fatal("empty allowlist should permit everyone")
	}
}

func TestIsUserAllowedRestricts(t *testing.T) {
	a := New(Options{AllowedUsernames: "Alice, bob"})
	if a.isUserAllowed("") {
		t.Fatal("missing username should be denied")
	}
	if !a.isUserAllowed("alice") || !a.isUserAllowed("BOB") {
		t.Fatal("listed usernames should be allowed case-insensitively")
	}
	if a.isUserAllowed("carol") {
		t.Fatal("unlisted username should be denied")
	}
}
