package browsercdp

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

var browserProfileNamespace = uuid.MustParse("d8f7f362-4b0f-48d8-b8ac-0dcf3a6af6e2")

func ResolveProfileStorageKey(sessionKey string, profileName string) string {
	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey == "" {
		sessionKey = "default"
	}
	profileName = strings.TrimSpace(profileName)
	if profileName == "" {
		profileName = "xiadown"
	}
	return uuid.NewSHA1(browserProfileNamespace, []byte(sessionKey+"\x00"+profileName)).String()
}

func ResolveProfileUserDataDir(sessionKey string, profileName string) string {
	return filepath.Join(
		os.TempDir(),
		"xiadown",
		"browser",
		"profiles",
		ResolveProfileStorageKey(sessionKey, profileName),
	)
}
