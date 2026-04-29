package wails

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"mime"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"unicode/utf8"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
)

type currentUserAvatar struct {
	Path   string
	Base64 string
	Mime   string
}

func baseCurrentUserProfile() CurrentUserProfile {
	var profile CurrentUserProfile
	current, err := user.Current()
	if err == nil {
		profile.Username = normalizeCurrentUsername(current.Username)
		profile.DisplayName = strings.TrimSpace(current.Name)
	}

	if profile.Username == "" {
		profile.Username = normalizeCurrentUsername(os.Getenv("USERNAME"))
	}
	if profile.Username == "" {
		profile.Username = normalizeCurrentUsername(os.Getenv("USER"))
	}
	if profile.DisplayName == "" {
		profile.DisplayName = strings.TrimSpace(os.Getenv("USER"))
	}
	if profile.DisplayName == "" {
		profile.DisplayName = profile.Username
	}
	profile.Initials = computeCurrentUserInitials(profile.DisplayName, profile.Username)
	return profile
}

func finalizeCurrentUserProfile(profile CurrentUserProfile, avatar currentUserAvatar) (CurrentUserProfile, error) {
	profile.AvatarPath = strings.TrimSpace(avatar.Path)
	profile.AvatarBase64 = strings.TrimSpace(avatar.Base64)
	profile.AvatarMime = strings.TrimSpace(avatar.Mime)
	if profile.Username == "" {
		profile.Username = "user"
	}
	if profile.DisplayName == "" {
		profile.DisplayName = profile.Username
	}
	if profile.Initials == "" {
		profile.Initials = computeCurrentUserInitials(profile.DisplayName, profile.Username)
	}
	if profile.AvatarBase64 != "" {
		if profile.AvatarMime == "" {
			profile.AvatarMime = "image/png"
		}
		return profile, nil
	}
	if profile.AvatarPath == "" {
		return profile, nil
	}
	avatarBase64, avatarMime, err := loadAvatarBase64(profile.AvatarPath)
	if err != nil {
		profile.AvatarPath = ""
		return profile, nil
	}
	profile.AvatarBase64 = avatarBase64
	profile.AvatarMime = avatarMime
	return profile, nil
}

func normalizeCurrentUsername(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if cut := strings.LastIndex(trimmed, `\`); cut >= 0 && cut < len(trimmed)-1 {
		trimmed = trimmed[cut+1:]
	}
	if cut := strings.LastIndex(trimmed, `/`); cut >= 0 && cut < len(trimmed)-1 {
		trimmed = trimmed[cut+1:]
	}
	return strings.TrimSpace(trimmed)
}

func computeCurrentUserInitials(displayName string, username string) string {
	parts := strings.Fields(strings.TrimSpace(displayName))
	initials := strings.Builder{}
	for _, part := range parts {
		r, _ := utf8.DecodeRuneInString(part)
		if r == utf8.RuneError {
			continue
		}
		initials.WriteRune(r)
		if initials.Len() >= 2 {
			break
		}
	}
	if initials.Len() == 0 {
		normalized := strings.TrimSpace(username)
		for _, r := range normalized {
			initials.WriteRune(r)
			if initials.Len() >= 2 {
				break
			}
		}
	}
	return strings.ToUpper(strings.TrimSpace(initials.String()))
}

func loadAvatarBase64(path string) (string, string, error) {
	cleaned := filepath.Clean(strings.TrimSpace(path))
	if cleaned == "" {
		return "", "", os.ErrNotExist
	}
	payload, err := os.ReadFile(cleaned)
	if err != nil {
		return "", "", err
	}
	return loadAvatarBase64FromBytes(payload, mime.TypeByExtension(strings.ToLower(filepath.Ext(cleaned))))
}

func loadAvatarBase64FromBytes(payload []byte, fallbackMime string) (string, string, error) {
	if len(payload) == 0 {
		return "", "", os.ErrNotExist
	}

	img, _, decodeErr := image.Decode(bytes.NewReader(payload))
	if decodeErr == nil {
		var buffer bytes.Buffer
		if err := png.Encode(&buffer, img); err == nil {
			return base64.StdEncoding.EncodeToString(buffer.Bytes()), "image/png", nil
		}
	}

	avatarMime := strings.TrimSpace(fallbackMime)
	if avatarMime == "" {
		avatarMime = http.DetectContentType(payload)
	}
	if avatarMime == "" {
		avatarMime = "application/octet-stream"
	}
	return base64.StdEncoding.EncodeToString(payload), avatarMime, nil
}
