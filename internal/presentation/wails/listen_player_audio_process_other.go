//go:build !windows

package wails

func (player *ListenYouTubeMusicPlayer) EqualizerAudioProcessID() uint32 {
	return 0
}
