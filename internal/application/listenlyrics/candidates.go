package listenlyrics

import (
	"context"
	"fmt"
	"strings"
)

type Candidate struct {
	ProviderID      string  `json:"providerId"`
	ProviderTrackID string  `json:"providerTrackId"`
	Title           string  `json:"title"`
	Artist          string  `json:"artist"`
	Album           string  `json:"album,omitempty"`
	DurationSeconds float64 `json:"durationSeconds,omitempty"`
	Instrumental    bool    `json:"instrumental,omitempty"`
	HasSynced       bool    `json:"hasSynced,omitempty"`
	HasPlain        bool    `json:"hasPlain,omitempty"`
	TimingQuality   string  `json:"timingQuality,omitempty"`
	Attribution     string  `json:"attribution,omitempty"`
	Confidence      int     `json:"confidence"`
	TitleScore      int     `json:"titleScore"`
	ArtistScore     int     `json:"artistScore"`
	AlbumScore      int     `json:"albumScore"`
	DurationScore   int     `json:"durationScore"`
	DurationDiff    float64 `json:"durationDiff,omitempty"`
	Accepted        bool    `json:"accepted"`
	Rejection       string  `json:"rejection,omitempty"`
}

type CandidateRequest struct {
	Track           Request `json:"track"`
	ProviderID      string  `json:"providerId"`
	ProviderTrackID string  `json:"providerTrackId"`
	PlainOnly       bool    `json:"plainOnly,omitempty"`
}

type CandidateClient interface {
	SearchLyricsCandidates(context.Context, Request) ([]Candidate, error)
	TrackLyricsCandidate(context.Context, CandidateRequest) (Snapshot, error)
}

func (service *Service) SearchCandidates(
	ctx context.Context,
	request Request,
) ([]Candidate, error) {
	if service == nil {
		return nil, fmt.Errorf("listen lyrics service unavailable")
	}
	client, ok := service.client.(CandidateClient)
	if !ok {
		return nil, fmt.Errorf("lyrics candidate search unavailable")
	}
	request = normalizeRequest(request)
	if request.Title == "" {
		return []Candidate{}, nil
	}
	return client.SearchLyricsCandidates(ctx, request)
}

func (service *Service) TrackCandidate(
	ctx context.Context,
	request CandidateRequest,
) (Snapshot, error) {
	if service == nil {
		return Snapshot{}, fmt.Errorf("listen lyrics service unavailable")
	}
	client, ok := service.client.(CandidateClient)
	if !ok {
		return Snapshot{}, fmt.Errorf("lyrics candidate preview unavailable")
	}
	request.Track = normalizeRequest(request.Track)
	request.ProviderID = strings.ToLower(strings.TrimSpace(request.ProviderID))
	request.ProviderTrackID = strings.TrimSpace(request.ProviderTrackID)
	request.PlainOnly = request.PlainOnly || request.Track.PlainOnly
	if request.ProviderID == "" || request.ProviderTrackID == "" {
		return Snapshot{}, fmt.Errorf("lyrics candidate identity required")
	}
	manualKey := strings.Join([]string{
		"manual",
		cacheKey(request.Track),
		request.ProviderID,
		request.ProviderTrackID,
	}, "\x00")
	service.mu.Lock()
	if cached, age, ok := service.cachedLocked(manualKey, service.currentTime()); ok && cacheEntryFresh(cached, age) {
		service.mu.Unlock()
		return cloneSnapshot(cached), nil
	}
	service.mu.Unlock()

	result, err := client.TrackLyricsCandidate(ctx, request)
	if err != nil {
		return Snapshot{}, err
	}
	result = normalizeSnapshot(result)
	if result.VideoID == "" {
		result.VideoID = requestIdentityID(request.Track)
	}
	if result.ProviderID == "" {
		result.ProviderID = request.ProviderID
	}
	if result.ProviderTrackID == "" {
		result.ProviderTrackID = request.ProviderTrackID
	}
	if result.ActiveProvider == "" {
		result.ActiveProvider = result.ProviderID
	}

	service.mu.Lock()
	defer service.mu.Unlock()
	service.storeCacheLocked(
		manualKey,
		cacheTrackIdentityKey(request.Track),
		result,
		service.currentTime(),
	)
	return cloneSnapshot(result), nil
}
