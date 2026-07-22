package wails

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"xiadown/internal/application/listenlyrics"
	"xiadown/internal/application/locallyrics"
	"xiadown/internal/application/youtubemusic"
)

type fakeWailsLyricsClient struct {
	requests int
	result   youtubemusic.LyricsResult
	err      error
	lastInfo youtubemusic.LyricsSearchInfo
}

type fakeWailsEmbeddedLyricsReader struct {
	content locallyrics.Content
	err     error
	reads   int
}

type fakeWailsLyricsCandidateClient struct {
	fakeWailsLyricsClient
	candidate  youtubemusic.LyricsResult
	searchInfo youtubemusic.LyricsSearchInfo
}

func (client *fakeWailsLyricsCandidateClient) SearchLyricsCandidates(_ context.Context, info youtubemusic.LyricsSearchInfo) ([]youtubemusic.LyricsCandidate, error) {
	client.searchInfo = info
	return nil, nil
}

func (client *fakeWailsLyricsCandidateClient) TrackLyricsCandidate(_ context.Context, _, _ string, _ bool) (youtubemusic.LyricsResult, error) {
	return client.candidate, nil
}

func TestListenLyricsLocalVersionIDChangesWithNormalizedContent(t *testing.T) {
	first, err := locallyrics.ParseContent(
		[]byte("[00:01.00]first"),
		locallyrics.FormatLRC,
		locallyrics.Options{},
	)
	if err != nil {
		t.Fatal(err)
	}
	second, err := locallyrics.ParseContent(
		[]byte("[00:01.00]second"),
		locallyrics.FormatLRC,
		locallyrics.Options{},
	)
	if err != nil {
		t.Fatal(err)
	}
	first.SourcePath = "chosen.lrc"
	second.SourcePath = "chosen.lrc"
	firstID := listenLyricsLocalVersionID(first, "local:one")
	secondID := listenLyricsLocalVersionID(second, "local:one")
	if firstID == secondID || firstID == "" || secondID == "" {
		t.Fatalf("expected content-addressed local lyric versions, got %q and %q", firstID, secondID)
	}
}

func TestListenLyricsCandidatePreservesLocalTrackIdentity(t *testing.T) {
	adapter := NewListenLyricsClient(&fakeWailsLyricsCandidateClient{
		candidate: youtubemusic.LyricsResult{
			Kind:            "synced",
			ProviderID:      "lrclib",
			ProviderTrackID: "42",
			Lines:           []youtubemusic.LyricLine{{StartMs: 1000, Text: "chosen"}},
		},
	})
	result, err := adapter.TrackLyricsCandidate(context.Background(), listenlyrics.CandidateRequest{
		Track:           listenlyrics.Request{Key: "local:track-one", Title: "Track"},
		ProviderID:      "lrclib",
		ProviderTrackID: "42",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.VideoID != "local:track-one" {
		t.Fatalf("local candidate lost timeline identity: %#v", result)
	}
}

func (client *fakeWailsLyricsClient) TrackLyrics(_ context.Context, info youtubemusic.LyricsSearchInfo) (youtubemusic.LyricsResult, error) {
	client.requests++
	client.lastInfo = info
	return client.result, client.err
}

func TestListenLyricsAdapterPassesSearchVariantsToAutomaticAndCandidateSearch(t *testing.T) {
	client := &fakeWailsLyricsCandidateClient{}
	adapter := NewListenLyricsClient(client)
	request := listenlyrics.Request{
		Title:  "后来",
		Artist: "刘若英",
		SearchVariants: []listenlyrics.SearchVariant{
			{Title: "後來", Artist: "劉若英"},
		},
	}
	if _, err := adapter.TrackLyrics(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.SearchLyricsCandidates(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	for name, info := range map[string]youtubemusic.LyricsSearchInfo{
		"automatic": client.lastInfo,
		"candidate": client.searchInfo,
	} {
		if len(info.SearchVariants) != 1 || info.SearchVariants[0].Title != "後來" || info.SearchVariants[0].Artist != "劉若英" {
			t.Fatalf("%s search lost variants: %#v", name, info.SearchVariants)
		}
	}
}

func TestListenLyricsHandlerReturnsSanitizedStructuredProviderFailure(t *testing.T) {
	client := &fakeWailsLyricsClient{
		err: errors.New(`Get "https://lrclib.net/api/get-cached?artist_name=Private+Artist&track_name=Private+Track": Bad Gateway` + "\n" + "lrclib api status 503"),
	}
	handler := NewListenLyricsHandler(listenlyrics.NewService(NewListenLyricsClient(client)))

	result, err := handler.TrackLyrics(context.Background(), ListenLyricsRequest{
		Track: ListenLyricsTrackRequest{
			VideoID: "video-one",
			Title:   "Private Track",
			Artist:  "Private Artist",
		},
	})
	if err != nil {
		t.Fatalf("structured provider failure escaped the Wails DTO: %v", err)
	}
	if result.ErrorCode != listenlyrics.ErrorCodeProviderUnavailable || !result.Retryable {
		t.Fatalf("unexpected provider failure classification: %#v", result)
	}
	if result.Error != "" {
		t.Fatalf("Wails DTO exposed raw provider detail: %q", result.Error)
	}
}

func (reader *fakeWailsEmbeddedLyricsReader) ReadEmbeddedLyrics(_ context.Context, _ string) (locallyrics.Content, error) {
	reader.reads++
	return reader.content, reader.err
}

func TestListenLyricsAdapterPrefersRichLocalSidecarWithoutOnlineRequest(t *testing.T) {
	directory := t.TempDir()
	mediaPath := filepath.Join(directory, "track.mp3")
	if err := os.WriteFile(mediaPath, []byte("audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(directory, "track.lrc"),
		[]byte("[00:01.00]<00:01.00>Hello <00:01.50>world<00:02.00>"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(directory, "track.t.lrc"),
		[]byte("[00:01.00]你好世界"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	online := &fakeWailsLyricsClient{}
	adapter := NewListenLyricsClient(online)
	result, err := adapter.TrackLyrics(context.Background(), listenlyrics.Request{
		Key:       "local:track-one",
		Title:     "Track",
		LocalPath: mediaPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if online.requests != 0 {
		t.Fatalf("expected sidecar to avoid online request, got %d", online.requests)
	}
	if result.VideoID != "local:track-one" || result.ProviderID != "local_sidecar" || result.TimingQuality != listenlyrics.TimingQualityWord {
		t.Fatalf("unexpected local source metadata: %#v", result)
	}
	if len(result.Lines) != 1 || result.Lines[0].TranslationText != "你好世界" || len(result.Lines[0].Words) != 2 {
		t.Fatalf("rich local lyrics were not preserved: %#v", result.Lines)
	}
	if result.Lines[0].Words[0].EndMs != 1500 || result.Lines[0].Words[0].EndsWithSpace == nil || !*result.Lines[0].Words[0].EndsWithSpace {
		t.Fatalf("word boundary or spacing was not preserved: %#v", result.Lines[0].Words[0])
	}
}

func TestListenLyricsAdapterFallsBackOnlineWhenLocalLyricsAreMissing(t *testing.T) {
	directory := t.TempDir()
	mediaPath := filepath.Join(directory, "track.flac")
	if err := os.WriteFile(mediaPath, []byte("audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	online := &fakeWailsLyricsClient{result: youtubemusic.LyricsResult{
		Kind:   "plain",
		Source: "YouTube Music",
		Text:   "online fallback",
	}}
	adapter := NewListenLyricsClient(online)
	result, err := adapter.TrackLyrics(context.Background(), listenlyrics.Request{
		Key:       "local:track-two",
		Title:     "Track",
		LocalPath: mediaPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if online.requests != 1 || result.VideoID != "local:track-two" || result.Text != "online fallback" {
		t.Fatalf("unexpected online fallback: requests=%d result=%#v", online.requests, result)
	}
}

func TestListenLyricsAdapterUsesSyncedEmbeddedLyricsAfterPlainSidecar(t *testing.T) {
	directory := t.TempDir()
	mediaPath := filepath.Join(directory, "track.mp3")
	if err := os.WriteFile(mediaPath, []byte("audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "track.lrc"), []byte("plain sidecar"), 0o600); err != nil {
		t.Fatal(err)
	}
	embedded := &fakeWailsEmbeddedLyricsReader{content: locallyrics.Content{
		Name:  "USLT.lrc",
		Bytes: []byte("[00:01.00]synced embedded"),
	}}
	online := &fakeWailsLyricsClient{}
	adapter := NewListenLyricsClient(online, embedded)
	result, err := adapter.TrackLyrics(context.Background(), listenlyrics.Request{
		Key:       "local:embedded-synced",
		Title:     "Track",
		LocalPath: mediaPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ProviderID != "local_embedded" || result.Kind != listenlyrics.KindSynced || result.Text != "synced embedded" {
		t.Fatalf("expected synced embedded lyrics to replace plain sidecar fallback: %#v", result)
	}
	if embedded.reads != 1 || online.requests != 0 {
		t.Fatalf("unexpected source reads: embedded=%d online=%d", embedded.reads, online.requests)
	}
}

func TestListenLyricsAdapterPrefersOnlineSyncedLyricsOverAutomaticPlainSidecar(t *testing.T) {
	directory := t.TempDir()
	mediaPath := filepath.Join(directory, "track.mp3")
	if err := os.WriteFile(mediaPath, []byte("audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "track.lrc"), []byte("local plain fallback"), 0o600); err != nil {
		t.Fatal(err)
	}
	online := &fakeWailsLyricsClient{result: youtubemusic.LyricsResult{
		Kind:   listenlyrics.KindSynced,
		Source: "LRCLib",
		Text:   "online synced",
		Lines:  []youtubemusic.LyricLine{{StartMs: 1000, DurationMs: 1000, Text: "online synced"}},
	}}
	adapter := NewListenLyricsClient(online)
	result, err := adapter.TrackLyrics(context.Background(), listenlyrics.Request{
		Key:       "local:online-synced",
		Title:     "Track",
		LocalPath: mediaPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if online.requests != 1 || result.ProviderID != "lrclib" || result.Kind != listenlyrics.KindSynced {
		t.Fatalf("expected online synced lyrics, requests=%d result=%#v", online.requests, result)
	}
}

func TestListenLyricsAdapterFallsBackToAutomaticPlainWhenOnlineHasNoSyncedLyrics(t *testing.T) {
	directory := t.TempDir()
	mediaPath := filepath.Join(directory, "track.mp3")
	if err := os.WriteFile(mediaPath, []byte("audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "track.lrc"), []byte("local plain fallback"), 0o600); err != nil {
		t.Fatal(err)
	}
	online := &fakeWailsLyricsClient{result: youtubemusic.LyricsResult{
		Kind:   listenlyrics.KindPlain,
		Source: "YouTube Music",
		Text:   "online plain",
	}}
	adapter := NewListenLyricsClient(online)
	result, err := adapter.TrackLyrics(context.Background(), listenlyrics.Request{
		Key:       "local:plain-fallback",
		Title:     "Track",
		LocalPath: mediaPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if online.requests != 1 || result.ProviderID != "local_sidecar" || result.Kind != listenlyrics.KindPlain || result.Text != "local plain fallback" {
		t.Fatalf("expected local plain fallback, requests=%d result=%#v", online.requests, result)
	}
}

func TestListenLyricsAdapterFallsBackToAutomaticPlainWhenOnlineFails(t *testing.T) {
	directory := t.TempDir()
	mediaPath := filepath.Join(directory, "track.mp3")
	if err := os.WriteFile(mediaPath, []byte("audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "track.lrc"), []byte("local plain fallback"), 0o600); err != nil {
		t.Fatal(err)
	}
	online := &fakeWailsLyricsClient{err: errors.New("provider unavailable")}
	adapter := NewListenLyricsClient(online)
	result, err := adapter.TrackLyrics(context.Background(), listenlyrics.Request{
		Key:       "local:error-fallback",
		Title:     "Track",
		LocalPath: mediaPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if online.requests != 1 || result.ProviderID != "local_sidecar" || result.Text != "local plain fallback" {
		t.Fatalf("expected local fallback after online error, requests=%d result=%#v", online.requests, result)
	}
}

func TestListenLyricsAdapterPlainOnlyKeepsAutomaticLocalPriority(t *testing.T) {
	directory := t.TempDir()
	mediaPath := filepath.Join(directory, "track.mp3")
	if err := os.WriteFile(mediaPath, []byte("audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "track.lrc"), []byte("[00:01.00]local timed text"), 0o600); err != nil {
		t.Fatal(err)
	}
	online := &fakeWailsLyricsClient{result: youtubemusic.LyricsResult{
		Kind:  listenlyrics.KindSynced,
		Lines: []youtubemusic.LyricLine{{StartMs: 1000, Text: "online synced"}},
	}}
	adapter := NewListenLyricsClient(online)
	result, err := adapter.TrackLyrics(context.Background(), listenlyrics.Request{
		Key:       "local:plain-only",
		Title:     "Track",
		LocalPath: mediaPath,
		PlainOnly: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if online.requests != 0 || result.ProviderID != "local_sidecar" || result.Kind != listenlyrics.KindPlain || result.Text != "local timed text" {
		t.Fatalf("plain-only request lost local priority: requests=%d result=%#v", online.requests, result)
	}
}
