//go:build !darwin || ios

package wails

import (
	"context"

	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

func sendRichOSNotification(_ context.Context, _ notifications.NotificationOptions, _ string, _ osNotificationHTTPClientProvider) (bool, error) {
	return false, nil
}
