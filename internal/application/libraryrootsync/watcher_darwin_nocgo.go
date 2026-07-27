//go:build darwin && (!cgo || ios)

package libraryrootsync

import "context"

type unsupportedDarwinNativeWatcher struct{}

func platformNativeWatcher() nativeWatcher {
	return unsupportedDarwinNativeWatcher{}
}

func (unsupportedDarwinNativeWatcher) Available() bool      { return false }
func (unsupportedDarwinNativeWatcher) SupportsReplay() bool { return false }

func (unsupportedDarwinNativeWatcher) Watch(
	context.Context,
	string,
	uint64,
	func(watchEvent),
) error {
	return errNativeWatcherUnavailable
}
