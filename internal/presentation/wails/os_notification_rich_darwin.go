//go:build darwin && !ios

package wails

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework Cocoa -framework UserNotifications

#import "os_notification_rich_darwin.h"
#include <stdlib.h>
*/
import "C"
import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

const maxNotificationImageBytes = 8 * 1024 * 1024

func sendRichOSNotification(ctx context.Context, options notifications.NotificationOptions, imageURL string, clientProvider osNotificationHTTPClientProvider) (bool, error) {
	imagePath, err := cacheOSNotificationImage(ctx, imageURL, clientProvider)
	if err != nil {
		return false, err
	}
	if imagePath == "" {
		return false, nil
	}
	if err := sendDarwinNotificationWithAttachment(options, imagePath); err != nil {
		return false, err
	}
	return true, nil
}

func sendDarwinNotificationWithAttachment(options notifications.NotificationOptions, imagePath string) error {
	dataJSON := ""
	if options.Data != nil {
		payload, err := json.Marshal(options.Data)
		if err != nil {
			return fmt.Errorf("marshal notification data: %w", err)
		}
		dataJSON = string(payload)
	}

	cIdentifier := C.CString(options.ID)
	cTitle := C.CString(options.Title)
	cSubtitle := C.CString(options.Subtitle)
	cBody := C.CString(options.Body)
	cDataJSON := C.CString(dataJSON)
	cImagePath := C.CString(imagePath)
	defer C.free(unsafe.Pointer(cIdentifier))
	defer C.free(unsafe.Pointer(cTitle))
	defer C.free(unsafe.Pointer(cSubtitle))
	defer C.free(unsafe.Pointer(cBody))
	defer C.free(unsafe.Pointer(cDataJSON))
	defer C.free(unsafe.Pointer(cImagePath))

	errText := C.xiadownSendNotificationWithAttachment(
		cIdentifier,
		cTitle,
		cSubtitle,
		cBody,
		cDataJSON,
		cImagePath,
	)
	if errText == nil {
		return nil
	}
	defer C.free(unsafe.Pointer(errText))
	return fmt.Errorf("%s", C.GoString(errText))
}

func cacheOSNotificationImage(ctx context.Context, rawURL string, clientProvider osNotificationHTTPClientProvider) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", nil
	}
	if path, ok := localNotificationImagePath(rawURL); ok {
		return path, nil
	}

	parsed, err := neturl.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse notification image url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", nil
	}

	cacheDir, err := notificationImageCacheDir()
	if err != nil {
		return "", err
	}
	hash := sha1.Sum([]byte(rawURL))
	ext := imageExtensionFromPath(parsed.Path)
	if ext == "" {
		ext = ".jpg"
	}
	target := filepath.Join(cacheDir, hex.EncodeToString(hash[:])+ext)
	if _, err := os.Stat(target); err == nil {
		return target, nil
	}

	requestCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "XiaDown")
	client := http.DefaultClient
	if clientProvider != nil {
		if provided := clientProvider.HTTPClient(); provided != nil {
			client = provided
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download notification image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("download notification image: status %d", resp.StatusCode)
	}
	if contentExt := imageExtensionFromContentType(resp.Header.Get("Content-Type")); contentExt != "" {
		ext = contentExt
		target = filepath.Join(cacheDir, hex.EncodeToString(hash[:])+ext)
		if _, err := os.Stat(target); err == nil {
			return target, nil
		}
	}

	temp, err := os.CreateTemp(cacheDir, "notification-*"+ext)
	if err != nil {
		return "", err
	}
	tempPath := temp.Name()
	_, copyErr := io.Copy(temp, io.LimitReader(resp.Body, maxNotificationImageBytes+1))
	closeErr := temp.Close()
	if copyErr != nil {
		_ = os.Remove(tempPath)
		return "", copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tempPath)
		return "", closeErr
	}
	if info, err := os.Stat(tempPath); err != nil {
		_ = os.Remove(tempPath)
		return "", err
	} else if info.Size() > maxNotificationImageBytes {
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("notification image is too large")
	}
	if err := os.Rename(tempPath, target); err != nil {
		_ = os.Remove(tempPath)
		return "", err
	}
	return target, nil
}

func localNotificationImagePath(rawURL string) (string, bool) {
	if strings.HasPrefix(rawURL, "file://") {
		parsed, err := neturl.Parse(rawURL)
		if err != nil {
			return "", false
		}
		path := parsed.Path
		if path == "" {
			return "", false
		}
		return path, true
	}
	if filepath.IsAbs(rawURL) {
		return rawURL, true
	}
	return "", false
}

func notificationImageCacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "xiadown", "notifications")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func imageExtensionFromPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".heic", ".heif", ".tiff":
		return ext
	default:
		return ""
	}
}

func imageExtensionFromContentType(contentType string) string {
	if contentType == "" {
		return ""
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return ""
	}
	switch strings.ToLower(mediaType) {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/heic":
		return ".heic"
	case "image/heif":
		return ".heif"
	case "image/tiff":
		return ".tiff"
	default:
		return ""
	}
}
