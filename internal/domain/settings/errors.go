package settings

import "errors"

var (
	ErrSettingsNotFound = errors.New("settings not found")
	ErrInvalidSettings  = errors.New("invalid settings")
)
