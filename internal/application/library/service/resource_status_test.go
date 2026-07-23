package service

import (
	"context"
	"testing"
	"time"

	"xiadown/internal/application/library/dto"
)

type resourceStatusIconResolver struct {
	icon string
}

func (resolver resourceStatusIconResolver) ResolveDomainIcon(context.Context, string) (string, error) {
	return resolver.icon, nil
}

func TestProjectManagedResourceSniffStatusUsesVisibleResourcesAndCaptureTime(t *testing.T) {
	firstSeenAt := time.Date(2026, time.July, 10, 8, 30, 0, 0, time.UTC)
	lastVisibleSeenAt := firstSeenAt.Add(2 * time.Minute)
	hiddenSegmentSeenAt := lastVisibleSeenAt.Add(2 * time.Minute)
	capture := newResourceCaptureState()
	capture.recordObserved(resourceObservedResource{
		url:          "https://media.example/video.mp4",
		pageURL:      "https://www.example.test/watch/1",
		contentType:  "video/mp4",
		resourceType: "Media",
		status:       200,
		sizeBytes:    8 * 1024 * 1024,
		seenAt:       firstSeenAt,
	})
	capture.recordObserved(resourceObservedResource{
		url:          "https://media.example/live/index.m3u8",
		pageURL:      "https://www.example.test/watch/1",
		contentType:  "application/vnd.apple.mpegurl",
		resourceType: "Fetch",
		status:       200,
		seenAt:       lastVisibleSeenAt,
	})
	capture.recordObserved(resourceObservedResource{
		url:          "https://media.example/live/chunk-42.m4s",
		pageURL:      "https://www.example.test/watch/1",
		contentType:  "video/iso.segment",
		resourceType: "Media",
		status:       200,
		sizeBytes:    512 * 1024,
		seenAt:       hiddenSegmentSeenAt,
	})

	service := &LibraryService{
		iconResolver: resourceStatusIconResolver{icon: "data:image/png;base64,icon"},
		resourceSniffs: map[string]*resourceSniffSession{
			"session-1": {
				ID: "session-1",
				Tabs: map[string]*resourceSniffTab{
					"tab-1": {
						TargetID: "tab-1",
						Capture:  capture,
					},
				},
			},
		},
	}

	got := service.projectManagedResourceSniffStatus(context.Background(), dto.ResourceSniffSession{
		SessionID:     "session-1",
		State:         resourceSniffStateRunning,
		BrowserStatus: resourceSniffBrowserStatusOpen,
		URL:           "https://www.example.test/watch/1",
		CurrentURL:    "https://www.example.test/watch/1",
		Title:         "Example Stream",
	})

	if got.Runtime != resourceSniffStatusRuntimeManaged || got.State != resourceSniffStatusStateActive {
		t.Fatalf("unexpected runtime/state: %#v", got)
	}
	if got.ResourceCount != 2 {
		t.Fatalf("expected two visible resources, got %d", got.ResourceCount)
	}
	if got.DownloadableCount != 1 {
		t.Fatalf("expected one downloadable resource, got %d", got.DownloadableCount)
	}
	if got.LastCaptureAt != formatResourceSniffRawTime(lastVisibleSeenAt) {
		t.Fatalf("expected last visible capture %q, got %q", formatResourceSniffRawTime(lastVisibleSeenAt), got.LastCaptureAt)
	}
	if got.Favicon != "data:image/png;base64,icon" {
		t.Fatalf("expected resolved favicon, got %q", got.Favicon)
	}
	if !got.CanClear || !got.CanStop {
		t.Fatalf("expected clear and stop capabilities, got %#v", got)
	}
}

func TestProjectResourceSniffStatusHandlesIdleAndOrphanRuntime(t *testing.T) {
	service := &LibraryService{}

	idle := service.projectResourceSniffStatus(context.Background(), dto.CDPBrowserStatus{})
	if idle.Runtime != resourceSniffStatusRuntimeNone || idle.State != resourceSniffStatusStateIdle || idle.CanStop {
		t.Fatalf("unexpected idle projection: %#v", idle)
	}

	orphan := service.projectResourceSniffStatus(context.Background(), dto.CDPBrowserStatus{
		Active:    true,
		Mode:      "orphan",
		RuntimeID: "runtime-1",
	})
	if orphan.Runtime != resourceSniffStatusRuntimeOrphan || orphan.State != resourceSniffStatusStateActive {
		t.Fatalf("unexpected orphan projection: %#v", orphan)
	}
	if orphan.RuntimeID != "runtime-1" || !orphan.CanStop || orphan.CanClear {
		t.Fatalf("unexpected orphan capabilities: %#v", orphan)
	}
}

func TestResourceSniffStatusListOmitsPreviewPayload(t *testing.T) {
	seenAt := time.Date(2026, time.July, 10, 9, 15, 0, 0, time.UTC)
	capture := newResourceCaptureState()
	capture.recordObserved(resourceObservedResource{
		url:          "https://cdn.example.test/cover.png",
		pageURL:      "https://www.example.test/watch/1",
		mimeType:     "image/png",
		contentType:  "image/png",
		resourceType: "Image",
		status:       200,
		sizeBytes:    32 * 1024,
		seenAt:       seenAt,
	})
	capture.recordPreviewSnapshot(resourcePreviewSnapshot{
		URL:         "https://cdn.example.test/cover.png",
		PageURL:     "https://www.example.test/watch/1",
		Kind:        "image",
		MimeType:    "image/png",
		ContentType: "image/png",
		SizeBytes:   8,
		Body:        []byte("preview!"),
		SeenAt:      seenAt,
	})
	session := &resourceSniffSession{
		ID: "session-1",
		Tabs: map[string]*resourceSniffTab{
			"tab-1": {
				TargetID: "tab-1",
				Capture:  capture,
			},
		},
	}
	service := &LibraryService{}

	statusItems := service.listResourceSniffRawResourcesForStatus(session)
	if len(statusItems) != 1 {
		t.Fatalf("expected one status item, got %d", len(statusItems))
	}
	if statusItems[0].PreviewDataBase64 != "" || statusItems[0].PreviewAvailable {
		t.Fatalf("status projection must omit preview payloads: %#v", statusItems[0].ResourceSniffRawResource)
	}

	fullItems := service.listResourceSniffRawResources(session)
	if len(fullItems) != 1 || fullItems[0].PreviewDataBase64 == "" || !fullItems[0].PreviewAvailable {
		t.Fatalf("full resource list should retain preview payload: %#v", fullItems)
	}
}

func TestNormalizeResourceSniffActivityState(t *testing.T) {
	tests := []struct {
		name          string
		state         string
		browserStatus string
		want          string
	}{
		{name: "running", state: resourceSniffStateRunning, browserStatus: resourceSniffBrowserStatusOpen, want: resourceSniffStatusStateActive},
		{name: "closing", state: resourceSniffStateClosing, browserStatus: resourceSniffBrowserStatusOpen, want: resourceSniffStatusStateClosing},
		{name: "closed tab", state: resourceSniffStateRunning, browserStatus: resourceSniffBrowserStatusTabClosed, want: resourceSniffStatusStateError},
		{name: "closed runtime", state: resourceSniffStateClosed, browserStatus: resourceSniffBrowserStatusClosed, want: resourceSniffStatusStateIdle},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeResourceSniffActivityState(tt.state, tt.browserStatus); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}
