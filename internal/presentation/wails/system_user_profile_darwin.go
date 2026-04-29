//go:build darwin

package wails

import (
	"context"
	"encoding/hex"
	"os"
	"os/exec"
	"strings"
	"time"
)

func loadCurrentUserProfile(ctx context.Context) (CurrentUserProfile, error) {
	profile := baseCurrentUserProfile()
	avatar := resolveDarwinCurrentUserPicture(ctx, profile.Username)
	return finalizeCurrentUserProfile(profile, avatar)
}

func resolveDarwinCurrentUserPicture(ctx context.Context, username string) currentUserAvatar {
	trimmedUsername := strings.TrimSpace(username)
	if trimmedUsername == "" {
		return currentUserAvatar{}
	}

	commandCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()

	output, err := exec.CommandContext(commandCtx, "dscl", ".", "-read", "/Users/"+trimmedUsername, "Picture", "JPEGPhoto").Output()
	if err != nil {
		return currentUserAvatar{}
	}

	return resolveDarwinCurrentUserAvatar(string(output))
}

func resolveDarwinCurrentUserAvatar(output string) currentUserAvatar {
	path := parseDarwinDSCLValue(output, "Picture")
	if path == "" {
		if payload := parseDarwinDSCLHexValue(output, "JPEGPhoto"); len(payload) > 0 {
			avatarBase64, avatarMime, err := loadAvatarBase64FromBytes(payload, "image/tiff")
			if err == nil {
				return currentUserAvatar{
					Base64: avatarBase64,
					Mime:   avatarMime,
				}
			}
		}
		return currentUserAvatar{}
	}
	if _, err := os.Stat(path); err != nil {
		return currentUserAvatar{}
	}
	return currentUserAvatar{Path: path}
}

func parseDarwinDSCLValue(output string, key string) string {
	lines := strings.Split(output, "\n")
	prefix := key + ":"
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, prefix) {
			continue
		}
		return strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
	}
	return ""
}

func parseDarwinDSCLHexValue(output string, key string) []byte {
	lines := strings.Split(output, "\n")
	prefix := key + ":"
	var builder strings.Builder
	collecting := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !collecting {
			if !strings.HasPrefix(trimmed, prefix) {
				continue
			}
			builder.WriteString(filterDarwinDSCLHex(strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))))
			collecting = true
			continue
		}
		if trimmed == "" {
			break
		}
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			break
		}
		builder.WriteString(filterDarwinDSCLHex(trimmed))
	}

	hexValue := builder.String()
	if hexValue == "" || len(hexValue)%2 != 0 {
		return nil
	}
	payload, err := hex.DecodeString(hexValue)
	if err != nil {
		return nil
	}
	return payload
}

func filterDarwinDSCLHex(value string) string {
	var builder strings.Builder
	for _, r := range value {
		switch {
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r >= 'a' && r <= 'f':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'F':
			builder.WriteRune(r)
		}
	}
	return builder.String()
}
