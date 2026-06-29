package service

import "testing"

func TestBuildYTDLPFormatOptionsKeepsAudioTrackMetadata(t *testing.T) {
	t.Parallel()

	formats := buildYTDLPFormatOptions(map[string]any{
		"formats": []any{
			map[string]any{
				"format_id":      "30280",
				"vcodec":         "none",
				"acodec":         "mp4a.40.2",
				"ext":            "m4a",
				"format_note":    "Chinese",
				"language":       "zh-Hans",
				"abr":            132.5,
				"tbr":            140.0,
				"audio_channels": 2,
				"filesize":       1234567,
			},
		},
	})

	if len(formats) != 1 {
		t.Fatalf("expected one format, got %d", len(formats))
	}
	format := formats[0]
	if format.ID != "30280" || !format.HasAudio || format.HasVideo {
		t.Fatalf("unexpected audio format flags: %#v", format)
	}
	if format.FormatNote != "Chinese" || format.Language != "zh-Hans" {
		t.Fatalf("expected language metadata, got %#v", format)
	}
	if format.ABR != 132.5 || format.TBR != 140.0 || format.AudioChannels != 2 {
		t.Fatalf("expected bitrate and channel metadata, got %#v", format)
	}
}
