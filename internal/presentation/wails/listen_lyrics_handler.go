package wails

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"xiadown/internal/application/listenlyrics"
	"xiadown/internal/application/locallyrics"
	"xiadown/internal/application/youtubemusic"
)

type listenLyricsClient interface {
	TrackLyrics(ctx context.Context, info youtubemusic.LyricsSearchInfo) (youtubemusic.LyricsResult, error)
}

type listenLyricsCandidateClient interface {
	SearchLyricsCandidates(ctx context.Context, info youtubemusic.LyricsSearchInfo) ([]youtubemusic.LyricsCandidate, error)
	TrackLyricsCandidate(ctx context.Context, providerID string, providerTrackID string, plainOnly bool) (youtubemusic.LyricsResult, error)
}

type listenLyricsAdapter struct {
	client         listenLyricsClient
	embeddedReader locallyrics.EmbeddedReader
}

func NewListenLyricsClient(client listenLyricsClient, embeddedReaders ...locallyrics.EmbeddedReader) listenLyricsAdapter {
	adapter := listenLyricsAdapter{client: client}
	if len(embeddedReaders) > 0 {
		adapter.embeddedReader = embeddedReaders[0]
	}
	return adapter
}

func (adapter listenLyricsAdapter) TrackLyrics(ctx context.Context, request listenlyrics.Request) (listenlyrics.Snapshot, error) {
	var localPlainFallback listenlyrics.Snapshot
	hasLocalPlainFallback := false
	if local, ok, err := adapter.trackLocalLyrics(ctx, request); err != nil {
		return listenlyrics.Snapshot{}, err
	} else if ok {
		if request.PlainOnly || listenLyricsSnapshotHasSyncedLyrics(local) {
			return local, nil
		}
		localPlainFallback = local
		hasLocalPlainFallback = true
	}
	if adapter.client == nil {
		if hasLocalPlainFallback {
			return localPlainFallback, nil
		}
		return listenlyrics.Snapshot{VideoID: firstNonEmpty(request.VideoID, request.Key), Kind: listenlyrics.KindUnavailable}, nil
	}
	if strings.TrimSpace(request.Language) != "" {
		ctx = youtubemusic.WithLocale(ctx, request.Language)
	}
	result, err := adapter.client.TrackLyrics(ctx, youtubemusic.LyricsSearchInfo{
		VideoID:         strings.TrimSpace(request.VideoID),
		Title:           strings.TrimSpace(request.Title),
		Artist:          strings.TrimSpace(request.Artist),
		Album:           strings.TrimSpace(request.Album),
		DurationSeconds: request.DurationSeconds,
		PlainOnly:       request.PlainOnly,
		SearchVariants:  listenLyricsSearchVariantsToYouTubeMusic(request.SearchVariants),
	})
	if err != nil {
		if hasLocalPlainFallback && ctx.Err() == nil {
			return localPlainFallback, nil
		}
		return listenlyrics.Snapshot{}, err
	}
	online := listenLyricsSnapshotFromYouTubeMusic(firstNonEmpty(request.VideoID, request.Key), result)
	if hasLocalPlainFallback && !listenLyricsSnapshotHasSyncedLyrics(online) {
		return localPlainFallback, nil
	}
	return online, nil
}

func listenLyricsSnapshotHasSyncedLyrics(snapshot listenlyrics.Snapshot) bool {
	return snapshot.Kind == listenlyrics.KindSynced && len(snapshot.Lines) > 0
}

func (adapter listenLyricsAdapter) trackLocalLyrics(ctx context.Context, request listenlyrics.Request) (listenlyrics.Snapshot, bool, error) {
	localPath := strings.TrimSpace(request.LocalPath)
	if localPath == "" {
		return listenlyrics.Snapshot{}, false, nil
	}
	videoID := firstNonEmpty(request.VideoID, request.Key)
	var plainFallback listenlyrics.Snapshot
	hasPlainFallback := false
	result, err := locallyrics.LoadBestSidecar(ctx, localPath, locallyrics.Options{})
	if err == nil {
		snapshot := listenLyricsSnapshotFromLocal(videoID, result, request.PlainOnly)
		if request.PlainOnly || listenLyricsSnapshotHasSyncedLyrics(snapshot) {
			return snapshot, true, nil
		}
		plainFallback = snapshot
		hasPlainFallback = true
	}
	if ctx.Err() != nil {
		return listenlyrics.Snapshot{}, false, ctx.Err()
	}
	if adapter.embeddedReader != nil {
		result, embeddedErr := locallyrics.ParseEmbedded(ctx, adapter.embeddedReader, localPath, locallyrics.Options{})
		if embeddedErr == nil {
			snapshot := listenLyricsSnapshotFromLocal(videoID, result, request.PlainOnly)
			if request.PlainOnly || listenLyricsSnapshotHasSyncedLyrics(snapshot) {
				return snapshot, true, nil
			}
			if !hasPlainFallback {
				plainFallback = snapshot
				hasPlainFallback = true
			}
		}
		if ctx.Err() != nil {
			return listenlyrics.Snapshot{}, false, ctx.Err()
		}
		if !errors.Is(embeddedErr, locallyrics.ErrNoLyrics) {
			// A malformed embedded tag should not make online fallback unusable.
			_ = embeddedErr
		}
	}
	if hasPlainFallback {
		return plainFallback, true, nil
	}
	return listenlyrics.Snapshot{}, false, nil
}

func listenLyricsSnapshotFromLocal(videoID string, result locallyrics.Result, plainOnly bool) listenlyrics.Snapshot {
	providerID := "local_" + string(result.Attribution.Kind)
	if providerID == "local_" {
		providerID = "local"
	}
	kind := listenlyrics.KindPlain
	if !plainOnly && result.TimingQuality != locallyrics.TimingQualityPlain && len(result.Lines) > 0 {
		kind = listenlyrics.KindSynced
	}
	return listenlyrics.Snapshot{
		VideoID:         strings.TrimSpace(videoID),
		Kind:            kind,
		Source:          strings.TrimSpace(result.Attribution.Label),
		ProviderID:      providerID,
		ProviderTrackID: listenLyricsLocalVersionID(result, videoID),
		Attribution:     strings.TrimSpace(result.Attribution.Label),
		TimingQuality:   string(result.TimingQuality),
		Confidence:      100,
		Text:            result.PlainText,
		Lines:           listenLyricsLinesFromLocal(result.Lines),
		ActiveProvider:  providerID,
	}
}

func listenLyricsLocalVersionID(result locallyrics.Result, fallback string) string {
	name := filepath.Base(firstNonEmpty(result.SourcePath, fallback, "local"))
	digest := sha256.New()
	writeString := func(value string) {
		var size [8]byte
		binary.LittleEndian.PutUint64(size[:], uint64(len(value)))
		_, _ = digest.Write(size[:])
		_, _ = digest.Write([]byte(value))
	}
	writeDuration := func(value time.Duration) {
		var encoded [8]byte
		binary.LittleEndian.PutUint64(encoded[:], uint64(value))
		_, _ = digest.Write(encoded[:])
	}
	var writeWords func([]locallyrics.Word)
	writeWords = func(words []locallyrics.Word) {
		writeDuration(time.Duration(len(words)))
		for _, word := range words {
			writeDuration(word.Start)
			writeDuration(word.End)
			writeString(word.Text)
			if word.EndsWithSpace {
				writeString("space")
			} else {
				writeString("joined")
			}
			writeWords(word.Syllables)
		}
	}
	writeString(string(result.Format))
	writeString(string(result.TimingQuality))
	writeString(result.PlainText)
	writeDuration(time.Duration(len(result.Lines)))
	for _, line := range result.Lines {
		writeDuration(line.Start)
		writeDuration(line.End)
		if line.EndEstimated {
			writeString("estimated")
		} else {
			writeString("exact")
		}
		writeString(line.Text)
		writeString(line.Translation)
		writeDuration(time.Duration(len(line.AlternateTexts)))
		for _, alternate := range line.AlternateTexts {
			writeString(alternate.Role)
			writeString(alternate.Language)
			writeString(alternate.Text)
		}
		writeWords(line.Words)
	}
	sum := digest.Sum(nil)
	return name + ":" + hex.EncodeToString(sum[:8])
}

func listenLyricsLinesFromLocal(lines []locallyrics.Line) []listenlyrics.Line {
	if len(lines) == 0 {
		return nil
	}
	result := make([]listenlyrics.Line, 0, len(lines))
	for _, line := range lines {
		result = append(result, listenlyrics.Line{
			StartMs:         listenLyricsMilliseconds(line.Start),
			DurationMs:      listenLyricsMilliseconds(max(line.End-line.Start, 0)),
			EndEstimated:    line.EndEstimated,
			Text:            line.Text,
			TranslationText: line.Translation,
			AlternateTexts:  listenLyricsAlternateTextsFromLocal(line.AlternateTexts),
			Words:           listenLyricsWordsFromLocal(line.Words),
		})
	}
	return result
}

func listenLyricsAlternateTextsFromLocal(values []locallyrics.AlternateText) []listenlyrics.AlternateText {
	if len(values) == 0 {
		return nil
	}
	result := make([]listenlyrics.AlternateText, 0, len(values))
	for _, value := range values {
		result = append(result, listenlyrics.AlternateText{
			Role:     value.Role,
			Language: value.Language,
			Text:     value.Text,
		})
	}
	return result
}

func listenLyricsWordsFromLocal(words []locallyrics.Word) []listenlyrics.Word {
	if len(words) == 0 {
		return nil
	}
	result := make([]listenlyrics.Word, 0, len(words))
	for _, word := range words {
		endsWithSpace := word.EndsWithSpace
		result = append(result, listenlyrics.Word{
			StartMs:       listenLyricsMilliseconds(word.Start),
			EndMs:         listenLyricsMilliseconds(word.End),
			Text:          word.Text,
			EndsWithSpace: &endsWithSpace,
			Syllables:     listenLyricsWordsFromLocal(word.Syllables),
		})
	}
	return result
}

func listenLyricsMilliseconds(value time.Duration) int {
	if value <= 0 {
		return 0
	}
	return int(value / time.Millisecond)
}

func (adapter listenLyricsAdapter) SearchLyricsCandidates(ctx context.Context, request listenlyrics.Request) ([]listenlyrics.Candidate, error) {
	client, ok := adapter.client.(listenLyricsCandidateClient)
	if !ok {
		return nil, fmt.Errorf("lyrics candidate search unavailable")
	}
	if strings.TrimSpace(request.Language) != "" {
		ctx = youtubemusic.WithLocale(ctx, request.Language)
	}
	candidates, err := client.SearchLyricsCandidates(ctx, youtubemusic.LyricsSearchInfo{
		VideoID:         strings.TrimSpace(request.VideoID),
		Title:           strings.TrimSpace(request.Title),
		Artist:          strings.TrimSpace(request.Artist),
		Album:           strings.TrimSpace(request.Album),
		DurationSeconds: request.DurationSeconds,
		PlainOnly:       request.PlainOnly,
		SearchVariants:  listenLyricsSearchVariantsToYouTubeMusic(request.SearchVariants),
	})
	if err != nil {
		return nil, err
	}
	result := make([]listenlyrics.Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		result = append(result, listenLyricsCandidateFromYouTubeMusic(candidate))
	}
	return result, nil
}

func (adapter listenLyricsAdapter) TrackLyricsCandidate(ctx context.Context, request listenlyrics.CandidateRequest) (listenlyrics.Snapshot, error) {
	client, ok := adapter.client.(listenLyricsCandidateClient)
	if !ok {
		return listenlyrics.Snapshot{}, fmt.Errorf("lyrics candidate preview unavailable")
	}
	if strings.TrimSpace(request.Track.Language) != "" {
		ctx = youtubemusic.WithLocale(ctx, request.Track.Language)
	}
	result, err := client.TrackLyricsCandidate(
		ctx,
		request.ProviderID,
		request.ProviderTrackID,
		request.PlainOnly,
	)
	if err != nil {
		return listenlyrics.Snapshot{}, err
	}
	snapshot := listenLyricsSnapshotFromYouTubeMusic(
		firstNonEmpty(request.Track.VideoID, request.Track.Key),
		result,
	)
	snapshot.ProviderID = strings.ToLower(strings.TrimSpace(request.ProviderID))
	snapshot.ProviderTrackID = strings.TrimSpace(request.ProviderTrackID)
	snapshot.ActiveProvider = snapshot.ProviderID
	return snapshot, nil
}

func listenLyricsSnapshotFromYouTubeMusic(videoID string, result youtubemusic.LyricsResult) listenlyrics.Snapshot {
	providerID, attribution := listenLyricsProviderMetadata(result.Source)
	if strings.TrimSpace(result.ProviderID) != "" {
		providerID = strings.ToLower(strings.TrimSpace(result.ProviderID))
	}
	if strings.TrimSpace(result.Attribution) != "" {
		attribution = strings.TrimSpace(result.Attribution)
	}
	return listenlyrics.Snapshot{
		VideoID:         videoID,
		Kind:            strings.TrimSpace(result.Kind),
		Source:          strings.TrimSpace(result.Source),
		ProviderID:      providerID,
		ProviderTrackID: strings.TrimSpace(result.ProviderTrackID),
		Attribution:     attribution,
		TimingQuality:   strings.TrimSpace(result.TimingQuality),
		Confidence:      result.Confidence,
		Text:            result.Text,
		Lines:           listenLyricsLinesFromYouTubeMusic(result.Lines),
		ActiveProvider:  providerID,
	}
}

func listenLyricsProviderMetadata(source string) (string, string) {
	trimmed := strings.TrimSpace(source)
	if strings.EqualFold(trimmed, "LRCLib") {
		return "lrclib", "LRCLIB contributors"
	}
	return "youtube_music", trimmed
}

func listenLyricsCandidateFromYouTubeMusic(candidate youtubemusic.LyricsCandidate) listenlyrics.Candidate {
	return listenlyrics.Candidate{
		ProviderID:      candidate.ProviderID,
		ProviderTrackID: candidate.ProviderTrackID,
		Title:           candidate.Title,
		Artist:          candidate.Artist,
		Album:           candidate.Album,
		DurationSeconds: candidate.DurationSeconds,
		Instrumental:    candidate.Instrumental,
		HasSynced:       candidate.HasSynced,
		HasPlain:        candidate.HasPlain,
		TimingQuality:   candidate.TimingQuality,
		Attribution:     candidate.Attribution,
		Confidence:      candidate.Confidence,
		TitleScore:      candidate.TitleScore,
		ArtistScore:     candidate.ArtistScore,
		AlbumScore:      candidate.AlbumScore,
		DurationScore:   candidate.DurationScore,
		DurationDiff:    candidate.DurationDiff,
		Accepted:        candidate.Accepted,
		Rejection:       candidate.Rejection,
	}
}

func listenLyricsLinesFromYouTubeMusic(lines []youtubemusic.LyricLine) []listenlyrics.Line {
	if len(lines) == 0 {
		return nil
	}
	result := make([]listenlyrics.Line, 0, len(lines))
	for _, line := range lines {
		result = append(result, listenlyrics.Line{
			StartMs:         line.StartMs,
			DurationMs:      line.DurationMs,
			Text:            line.Text,
			TranslationText: line.TranslationText,
			RomanizedText:   line.RomanizedText,
			RomanizedKind:   line.RomanizedKind,
			Words:           listenLyricsWordsFromYouTubeMusic(line.Words),
		})
	}
	return result
}

func listenLyricsWordsFromYouTubeMusic(words []youtubemusic.TimedWord) []listenlyrics.Word {
	if len(words) == 0 {
		return nil
	}
	result := make([]listenlyrics.Word, 0, len(words))
	for _, word := range words {
		result = append(result, listenlyrics.Word{
			StartMs:       word.StartMs,
			EndMs:         word.EndMs,
			Text:          word.Text,
			EndsWithSpace: cloneListenLyricsBool(word.EndsWithSpace),
			Syllables:     listenLyricsWordsFromYouTubeMusic(word.Syllables),
		})
	}
	return result
}

func cloneListenLyricsBool(value *bool) *bool {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

type ListenLyricsHandler struct {
	service *listenlyrics.Service
}

type ListenLyricsTrackRequest struct {
	LyricsID        string  `json:"lyricsId,omitempty"`
	VideoID         string  `json:"videoId,omitempty"`
	Title           string  `json:"title,omitempty"`
	Artist          string  `json:"artist,omitempty"`
	Channel         string  `json:"channel,omitempty"`
	Album           string  `json:"album,omitempty"`
	LocalPath       string  `json:"localPath,omitempty"`
	DurationSeconds float64 `json:"durationSeconds,omitempty"`
}

type ListenLyricsRequest struct {
	Track           ListenLyricsTrackRequest    `json:"track"`
	LyricsID        string                      `json:"lyricsId,omitempty"`
	VideoID         string                      `json:"videoId,omitempty"`
	Title           string                      `json:"title,omitempty"`
	Artist          string                      `json:"artist,omitempty"`
	Album           string                      `json:"album,omitempty"`
	LocalPath       string                      `json:"localPath,omitempty"`
	DurationSeconds float64                     `json:"durationSeconds,omitempty"`
	PlainOnly       bool                        `json:"plainOnly,omitempty"`
	Language        string                      `json:"language,omitempty"`
	SearchVariants  []ListenLyricsSearchVariant `json:"searchVariants,omitempty"`
}

type ListenLyricsSearchVariant struct {
	Title  string `json:"title,omitempty"`
	Artist string `json:"artist,omitempty"`
}

type ListenLyricsCandidateRequest struct {
	Track           ListenLyricsTrackRequest `json:"track"`
	LyricsID        string                   `json:"lyricsId,omitempty"`
	VideoID         string                   `json:"videoId,omitempty"`
	Title           string                   `json:"title,omitempty"`
	Artist          string                   `json:"artist,omitempty"`
	Album           string                   `json:"album,omitempty"`
	LocalPath       string                   `json:"localPath,omitempty"`
	DurationSeconds float64                  `json:"durationSeconds,omitempty"`
	ProviderID      string                   `json:"providerId"`
	ProviderTrackID string                   `json:"providerTrackId"`
	PlainOnly       bool                     `json:"plainOnly,omitempty"`
	Language        string                   `json:"language,omitempty"`
}

func NewListenLyricsHandler(service *listenlyrics.Service) *ListenLyricsHandler {
	return &ListenLyricsHandler{service: service}
}

func (handler *ListenLyricsHandler) TrackLyrics(ctx context.Context, request ListenLyricsRequest) (listenlyrics.Snapshot, error) {
	if handler == nil || handler.service == nil {
		return listenlyrics.Snapshot{}, fmt.Errorf("listen lyrics service unavailable")
	}
	snapshot, err := handler.service.TrackLyrics(ctx, listenlyrics.Request{
		Key:             firstNonEmpty(request.LyricsID, request.Track.LyricsID),
		VideoID:         firstNonEmpty(request.VideoID, request.Track.VideoID),
		Title:           firstNonEmpty(request.Title, request.Track.Title),
		Artist:          firstNonEmpty(request.Artist, request.Track.Artist, request.Track.Channel),
		Album:           firstNonEmpty(request.Album, request.Track.Album),
		LocalPath:       firstNonEmpty(request.LocalPath, request.Track.LocalPath),
		DurationSeconds: firstPositive(request.DurationSeconds, request.Track.DurationSeconds),
		PlainOnly:       request.PlainOnly,
		Language:        request.Language,
		SearchVariants:  listenLyricsSearchVariantsFromWails(request.SearchVariants),
	})
	if err != nil && snapshot.ErrorCode != "" {
		return snapshot, nil
	}
	return snapshot, err
}

func (handler *ListenLyricsHandler) SearchCandidates(ctx context.Context, request ListenLyricsRequest) ([]listenlyrics.Candidate, error) {
	if handler == nil || handler.service == nil {
		return nil, fmt.Errorf("listen lyrics service unavailable")
	}
	return handler.service.SearchCandidates(ctx, listenLyricsRequestFromWails(request))
}

func (handler *ListenLyricsHandler) TrackCandidate(ctx context.Context, request ListenLyricsCandidateRequest) (listenlyrics.Snapshot, error) {
	if handler == nil || handler.service == nil {
		return listenlyrics.Snapshot{}, fmt.Errorf("listen lyrics service unavailable")
	}
	return handler.service.TrackCandidate(ctx, listenlyrics.CandidateRequest{
		Track: listenlyrics.Request{
			Key:             firstNonEmpty(request.LyricsID, request.Track.LyricsID),
			VideoID:         firstNonEmpty(request.VideoID, request.Track.VideoID),
			Title:           firstNonEmpty(request.Title, request.Track.Title),
			Artist:          firstNonEmpty(request.Artist, request.Track.Artist, request.Track.Channel),
			Album:           firstNonEmpty(request.Album, request.Track.Album),
			LocalPath:       firstNonEmpty(request.LocalPath, request.Track.LocalPath),
			DurationSeconds: firstPositive(request.DurationSeconds, request.Track.DurationSeconds),
			PlainOnly:       request.PlainOnly,
			Language:        request.Language,
		},
		ProviderID:      request.ProviderID,
		ProviderTrackID: request.ProviderTrackID,
		PlainOnly:       request.PlainOnly,
	})
}

func listenLyricsRequestFromWails(request ListenLyricsRequest) listenlyrics.Request {
	return listenlyrics.Request{
		Key:             firstNonEmpty(request.LyricsID, request.Track.LyricsID),
		VideoID:         firstNonEmpty(request.VideoID, request.Track.VideoID),
		Title:           firstNonEmpty(request.Title, request.Track.Title),
		Artist:          firstNonEmpty(request.Artist, request.Track.Artist, request.Track.Channel),
		Album:           firstNonEmpty(request.Album, request.Track.Album),
		LocalPath:       firstNonEmpty(request.LocalPath, request.Track.LocalPath),
		DurationSeconds: firstPositive(request.DurationSeconds, request.Track.DurationSeconds),
		PlainOnly:       request.PlainOnly,
		Language:        request.Language,
		SearchVariants:  listenLyricsSearchVariantsFromWails(request.SearchVariants),
	}
}

func listenLyricsSearchVariantsFromWails(variants []ListenLyricsSearchVariant) []listenlyrics.SearchVariant {
	if len(variants) == 0 {
		return nil
	}
	result := make([]listenlyrics.SearchVariant, 0, len(variants))
	for _, variant := range variants {
		result = append(result, listenlyrics.SearchVariant{Title: variant.Title, Artist: variant.Artist})
	}
	return result
}

func listenLyricsSearchVariantsToYouTubeMusic(variants []listenlyrics.SearchVariant) []youtubemusic.LyricsSearchVariant {
	if len(variants) == 0 {
		return nil
	}
	result := make([]youtubemusic.LyricsSearchVariant, 0, len(variants))
	for _, variant := range variants {
		result = append(result, youtubemusic.LyricsSearchVariant{Title: variant.Title, Artist: variant.Artist})
	}
	return result
}

func (handler *ListenLyricsHandler) CurrentLyrics(_ context.Context) (listenlyrics.Snapshot, error) {
	if handler == nil || handler.service == nil {
		return listenlyrics.Snapshot{}, fmt.Errorf("listen lyrics service unavailable")
	}
	return handler.service.Current(), nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func firstPositive(values ...float64) float64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
