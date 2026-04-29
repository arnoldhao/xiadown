//go:build !windows

package wails

func prepareOSNotificationServiceStartup() (func(), error) {
	return func() {}, nil
}
