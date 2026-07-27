//go:build !darwin && !windows

package libraryrootsync

import "context"

type unsupportedNativeWatcher struct{}

func platformNativeWatcher() nativeWatcher {
	return unsupportedNativeWatcher{}
}

func (unsupportedNativeWatcher) Available() bool      { return false }
func (unsupportedNativeWatcher) SupportsReplay() bool { return false }

func (unsupportedNativeWatcher) Watch(
	context.Context,
	string,
	uint64,
	func(watchEvent),
) error {
	return errNativeWatcherUnavailable
}
