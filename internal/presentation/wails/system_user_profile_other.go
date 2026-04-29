//go:build !darwin && !windows

package wails

import "context"

func loadCurrentUserProfile(_ context.Context) (CurrentUserProfile, error) {
	return finalizeCurrentUserProfile(baseCurrentUserProfile(), currentUserAvatar{})
}
