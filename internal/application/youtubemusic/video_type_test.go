package youtubemusic

import "testing"

func TestMusicVideoTypeHasVideoContent(t *testing.T) {
	for _, value := range []string{
		"MUSIC_VIDEO_TYPE_OMV",
	} {
		if !MusicVideoTypeHasVideoContent(value) {
			t.Fatalf("expected %q to have video content", value)
		}
	}

	for _, value := range []string{
		"",
		"MUSIC_VIDEO_TYPE_ATV",
		"MUSIC_VIDEO_TYPE_UGC",
		"MUSIC_VIDEO_TYPE_PODCAST_EPISODE",
	} {
		if MusicVideoTypeHasVideoContent(value) {
			t.Fatalf("expected %q to not have video content", value)
		}
	}
}

func TestMusicVideoTypeVideoAvailability(t *testing.T) {
	for _, value := range []string{"MUSIC_VIDEO_TYPE_OMV", " MUSIC_VIDEO_TYPE_OMV "} {
		hasVideo, known := MusicVideoTypeVideoAvailability(value)
		if !known || !hasVideo {
			t.Fatalf("expected %q to be known video, got hasVideo=%v known=%v", value, hasVideo, known)
		}
	}

	for _, value := range []string{"MUSIC_VIDEO_TYPE_ATV"} {
		hasVideo, known := MusicVideoTypeVideoAvailability(value)
		if !known || hasVideo {
			t.Fatalf("expected %q to be known no-video, got hasVideo=%v known=%v", value, hasVideo, known)
		}
	}

	for _, value := range []string{"MUSIC_VIDEO_TYPE_UGC", "MUSIC_VIDEO_TYPE_PODCAST_EPISODE"} {
		hasVideo, known := MusicVideoTypeVideoAvailability(value)
		if known || hasVideo {
			t.Fatalf("expected %q to remain unknown, got hasVideo=%v known=%v", value, hasVideo, known)
		}
	}
}

func TestThumbnailSuggestsVideoContent(t *testing.T) {
	if !ThumbnailSuggestsVideoContent("nWb_X3ZJQjw", "https://i.ytimg.com/vi/nWb_X3ZJQjw/hq720.jpg") {
		t.Fatal("expected YouTube video thumbnail to suggest video content")
	}
	if ThumbnailSuggestsVideoContent("tm0jDoUDPFo", "https://yt3.googleusercontent.com/art=w544-h544") {
		t.Fatal("expected square YouTube Music artwork to not suggest video content")
	}
}

func TestTrackVideoAvailabilityUsesTypeAndThumbnail(t *testing.T) {
	if hasVideo, known := TrackVideoAvailability("MUSIC_VIDEO_TYPE_ATV", "TESTVID007G", "https://i.ytimg.com/vi/TESTVID007G/hq720.jpg"); !known || hasVideo {
		t.Fatalf("expected ATV to stay known no-video, got hasVideo=%v known=%v", hasVideo, known)
	}
	if hasVideo, known := TrackVideoAvailability("MUSIC_VIDEO_TYPE_UGC", "TESTVID007G", "https://i.ytimg.com/vi/TESTVID007G/hq720.jpg"); !known || !hasVideo {
		t.Fatalf("expected video thumbnail to be known video, got hasVideo=%v known=%v", hasVideo, known)
	}
	if hasVideo, known := TrackVideoAvailability("MUSIC_VIDEO_TYPE_UGC", "TESTVID007G", "https://lh3.googleusercontent.com/art=w544-h544"); !known || hasVideo {
		t.Fatalf("expected non-video thumbnail to be known no-video, got hasVideo=%v known=%v", hasVideo, known)
	}
	if hasVideo, known := TrackVideoAvailability("MUSIC_VIDEO_TYPE_UGC", "TESTVID007G", ""); known || hasVideo {
		t.Fatalf("expected missing thumbnail to remain unknown, got hasVideo=%v known=%v", hasVideo, known)
	}
}
