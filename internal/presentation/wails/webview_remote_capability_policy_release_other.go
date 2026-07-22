//go:build !windows || server

package wails

import "github.com/wailsapp/wails/v3/pkg/application"

func releaseWebViewRemoteCapabilityPolicy(_ *application.WebviewWindow) {}
