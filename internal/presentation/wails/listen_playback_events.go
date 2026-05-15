package wails

import (
	"xiadown/internal/application/listenplayback"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const ListenPlaybackSnapshotEvent = "listen:playback:snapshot"

func NewListenPlaybackSnapshotEmitter(app *application.App, service *listenplayback.PlayerService) func() {
	if app == nil || service == nil {
		return func() {}
	}
	return service.Subscribe(func(snapshot listenplayback.Snapshot) {
		app.Event.Emit(ListenPlaybackSnapshotEvent, snapshot)
	})
}
