package browsercdp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestResolveProfileStorageKeyStableAndDistinct(t *testing.T) {
	t.Parallel()

	sessionKey := "v2::-::aui::-::788e3ff2-b4ad-426d-be0f-1424156032fd::-::788e3ff2-b4ad-426d-be0f-1424156032fd"
	profileName := "xiadown"

	first := ResolveProfileStorageKey(sessionKey, profileName)
	second := ResolveProfileStorageKey(sessionKey, profileName)
	otherProfile := ResolveProfileStorageKey(sessionKey, "work")

	if first != second {
		t.Fatalf("expected stable storage key, got %q and %q", first, second)
	}
	if first == otherProfile {
		t.Fatalf("expected different profiles to use different storage keys")
	}
	if _, err := uuid.Parse(first); err != nil {
		t.Fatalf("expected uuid storage key, got %q: %v", first, err)
	}
}

func TestResolveProfileUserDataDirUsesShallowSafePath(t *testing.T) {
	t.Parallel()

	sessionKey := "v2::-::aui::-::788e3ff2-b4ad-426d-be0f-1424156032fd::-::788e3ff2-b4ad-426d-be0f-1424156032fd"
	profileName := "dream:creator"
	dir := ResolveProfileUserDataDir(sessionKey, profileName)

	if strings.Contains(dir, sessionKey) {
		t.Fatalf("expected session key to stay out of path, got %q", dir)
	}
	if strings.Contains(dir, profileName) {
		t.Fatalf("expected profile name to stay out of path, got %q", dir)
	}
	if strings.ContainsAny(filepath.Base(dir), `<>:"/\|?*`) {
		t.Fatalf("expected final directory name to be path safe, got %q", filepath.Base(dir))
	}

	rel, err := filepath.Rel(os.TempDir(), dir)
	if err != nil {
		t.Fatalf("resolve relative path: %v", err)
	}
	parts := strings.Split(rel, string(os.PathSeparator))
	if len(parts) != 4 {
		t.Fatalf("expected shallow temp path, got %q (%d parts)", rel, len(parts))
	}
	if parts[0] != "xiadown" || parts[1] != "browser" || parts[2] != "profiles" {
		t.Fatalf("unexpected profile path layout: %q", rel)
	}
}

func TestNormalizeSessionOptionsUsesEncodedProfileDir(t *testing.T) {
	t.Parallel()

	options := normalizeSessionOptions(SessionOptions{
		SessionKey:  "v2::-::aui::-::thread:1::-::thread:1",
		ProfileName: "dream:creator",
	})

	want := ResolveProfileUserDataDir("v2::-::aui::-::thread:1::-::thread:1", "dream:creator")
	if options.UserDataDir != want {
		t.Fatalf("unexpected encoded user data dir: got %q want %q", options.UserDataDir, want)
	}
}
