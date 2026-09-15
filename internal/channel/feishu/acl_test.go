package feishu

import "testing"

func TestIsUserAllowedEmptyAllowlist(t *testing.T) {
	a := New(Options{})
	if !a.isUserAllowed("") || !a.isUserAllowed("ou_anyone") {
		t.Fatal("empty allowlist should permit everyone")
	}
}

func TestIsUserAllowedRestricts(t *testing.T) {
	a := New(Options{AllowedOpenIDs: "ou_a, ou_b"})
	if a.isUserAllowed("") {
		t.Fatal("missing open_id should be denied")
	}
	if !a.isUserAllowed("ou_a") || !a.isUserAllowed("ou_b") {
		t.Fatal("listed open_ids should be allowed")
	}
	if a.isUserAllowed("ou_c") {
		t.Fatal("unlisted open_id should be denied")
	}
	if a.isUserAllowed("OU_A") {
		t.Fatal("open_id matching is case-sensitive")
	}
}
