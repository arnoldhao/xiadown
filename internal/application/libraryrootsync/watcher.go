package libraryrootsync

import (
	"context"
	"errors"
)

var errNativeWatcherUnavailable = errors.New("native filesystem watcher unavailable")

type nativeWatcher interface {
	Available() bool
	SupportsReplay() bool
	Watch(
		ctx context.Context,
		rootPath string,
		since uint64,
		emit func(watchEvent),
	) error
}

func newNativeWatcher() nativeWatcher {
	return platformNativeWatcher()
}
