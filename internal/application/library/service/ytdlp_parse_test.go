package service

import (
	"context"
	"testing"
)

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

func TestBuildYTDLPPlaylistItemsNormalizesEntries(t *testing.T) {
	t.Parallel()

	service := &LibraryService{}
	items := service.buildYTDLPPlaylistItems(context.Background(), map[string]any{
		"extractor_key": "YoutubeTab",
		"entries": []any{
			map[string]any{
				"id":     "abc123XYZ09",
				"title":  "First",
				"ie_key": "Youtube",
			},
			map[string]any{
				"webpage_url": "https://www.bilibili.com/video/BV1xx411c7mD",
				"title":       "Second",
			},
			map[string]any{
				"id":     "abc123XYZ09",
				"ie_key": "Youtube",
			},
		},
	})

	if len(items) != 2 {
		t.Fatalf("expected two deduped playlist items, got %#v", items)
	}
	if items[0].URL != "https://www.youtube.com/watch?v=abc123XYZ09" || items[0].Domain != "youtube.com" {
		t.Fatalf("unexpected first playlist item: %#v", items[0])
	}
	if items[1].URL != "https://www.bilibili.com/video/BV1xx411c7mD" || items[1].Domain != "bilibili.com" {
		t.Fatalf("unexpected second playlist item: %#v", items[1])
	}
}
