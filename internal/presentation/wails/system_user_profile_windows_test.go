//go:build windows

package wails

import "testing"

func TestNormalizeWindowsUserTileName(t *testing.T) {
	if got := normalizeWindowsUserTileName(`DOMAIN\user`); got != "DOMAIN+user" {
		t.Fatalf("expected DOMAIN+user, got %q", got)
	}
	if got := normalizeWindowsUserTileName(`azuread/user`); got != "azuread+user" {
		t.Fatalf("expected azuread+user, got %q", got)
	}
}

func TestParseWindowsImageValueScore(t *testing.T) {
	if got := parseWindowsImageValueScore("Image240"); got != 240 {
		t.Fatalf("expected 240, got %d", got)
	}
	if got := parseWindowsImageValueScore("Image"); got != 0 {
		t.Fatalf("expected 0 for missing suffix, got %d", got)
	}
}
