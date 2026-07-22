//go:build (!linux && !darwin && !windows) || (linux && !cgo) || (darwin && !cgo) || android || ios || server

package wails

import "github.com/wailsapp/wails/v3/pkg/application"

func registerWebViewRemoteCapabilityPolicy(_ *application.WebviewWindow) {}

func cancelWebViewRemoteCapabilityPolicyRegistration(_ *application.WebviewWindow) {}
