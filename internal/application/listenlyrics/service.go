package listenlyrics

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
)

const (
	KindSynced      = "synced"
	KindPlain       = "plain"
	KindUnavailable = "unavailable"
)

type Client interface {
	TrackLyrics(ctx context.Context, request Request) (Snapshot, error)
}

type Request struct {
	VideoID         string
	Title           string
	Artist          string
	DurationSeconds float64
	PlainOnly       bool
	Language        string
}

type Snapshot struct {
	VideoID        string `json:"videoId,omitempty"`
	Kind           string `json:"kind,omitempty"`
	Source         string `json:"source,omitempty"`
	Text           string `json:"text,omitempty"`
	Lines          []Line `json:"lines,omitempty"`
	Loading        bool   `json:"loading,omitempty"`
	Error          string `json:"error,omitempty"`
	ActiveProvider string `json:"activeProvider,omitempty"`
}

type Line struct {
	StartMs       int    `json:"startMs"`
	DurationMs    int    `json:"durationMs"`
	Text          string `json:"text"`
	RomanizedText string `json:"romanizedText,omitempty"`
	RomanizedKind string `json:"romanizedKind,omitempty"`
	Words         []Word `json:"words,omitempty"`
}

type Word struct {
	StartMs int    `json:"startMs"`
	Text    string `json:"text"`
}

type Service struct {
	mu sync.Mutex

	client Client

	current    Snapshot
	generation uint64
	cache      map[string]Snapshot
}

func NewService(client Client) *Service {
	return &Service{
		client: client,
		cache:  make(map[string]Snapshot),
	}
}

func (service *Service) Current() Snapshot {
	if service == nil {
		return Snapshot{Kind: KindUnavailable}
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	return normalizeSnapshot(cloneSnapshot(service.current))
}

func (service *Service) TrackLyrics(ctx context.Context, request Request) (Snapshot, error) {
	if service == nil {
		return Snapshot{}, fmt.Errorf("listen lyrics service unavailable")
	}
	request = normalizeRequest(request)
	if request.VideoID == "" && request.Title == "" {
		return Snapshot{Kind: KindUnavailable}, nil
	}

	key := cacheKey(request)
	service.mu.Lock()
	service.generation++
	generation := service.generation
	cached, hasCached := service.cache[key]
	if hasCached && cached.Kind == KindSynced {
		service.current = cloneSnapshot(cached)
		service.mu.Unlock()
		return cloneSnapshot(cached), nil
	}
	if hasCached {
		service.current = cloneSnapshot(cached)
		service.current.Loading = true
	} else {
		service.current = Snapshot{
			VideoID: request.VideoID,
			Loading: true,
		}
	}
	service.mu.Unlock()

	var result Snapshot
	var err error
	if service.client == nil {
		result = Snapshot{VideoID: request.VideoID, Kind: KindUnavailable}
	} else {
		result, err = service.client.TrackLyrics(ctx, request)
	}
	result = normalizeSnapshot(result)
	if result.VideoID == "" {
		result.VideoID = request.VideoID
	}
	if err != nil {
		if hasCached && cached.Kind == KindPlain {
			result = cloneSnapshot(cached)
			err = nil
		} else {
			result = Snapshot{
				VideoID: request.VideoID,
				Kind:    KindUnavailable,
				Error:   strings.TrimSpace(err.Error()),
			}
		}
	} else if result.Kind == KindPlain && hasCached && cached.Kind == KindPlain {
		result = cloneSnapshot(cached)
	} else if !snapshotAvailable(result) && hasCached && cached.Kind == KindPlain {
		result = cloneSnapshot(cached)
	} else if rank(cached.Kind) > rank(result.Kind) {
		result = cloneSnapshot(cached)
	}

	service.mu.Lock()
	defer service.mu.Unlock()
	if generation != service.generation {
		return cloneSnapshot(result), err
	}
	result = normalizeSnapshot(result)
	result.Loading = false
	if result.ActiveProvider == "" {
		result.ActiveProvider = result.Source
	}
	service.current = cloneSnapshot(result)
	if result.Kind == KindSynced || result.Kind == KindPlain || result.Kind == KindUnavailable {
		service.cache[key] = cloneSnapshot(result)
	}
	return cloneSnapshot(result), err
}

func normalizeRequest(request Request) Request {
	request.VideoID = strings.TrimSpace(request.VideoID)
	request.Title = strings.TrimSpace(request.Title)
	request.Artist = strings.TrimSpace(request.Artist)
	request.Language = strings.TrimSpace(request.Language)
	if math.IsNaN(request.DurationSeconds) || math.IsInf(request.DurationSeconds, 0) || request.DurationSeconds < 0 {
		request.DurationSeconds = 0
	}
	return request
}

func normalizeSnapshot(snapshot Snapshot) Snapshot {
	snapshot.VideoID = strings.TrimSpace(snapshot.VideoID)
	snapshot.Kind = strings.TrimSpace(snapshot.Kind)
	snapshot.Source = strings.TrimSpace(snapshot.Source)
	snapshot.Error = strings.TrimSpace(snapshot.Error)
	snapshot.ActiveProvider = strings.TrimSpace(snapshot.ActiveProvider)
	if snapshot.Kind != KindSynced && snapshot.Kind != KindPlain && snapshot.Kind != KindUnavailable {
		snapshot.Kind = KindUnavailable
	}
	snapshot.Lines = cloneLines(snapshot.Lines)
	return snapshot
}

func cacheKey(request Request) string {
	mode := "synced"
	if request.PlainOnly {
		mode = "plain"
	}
	if request.VideoID != "" {
		return strings.Join([]string{request.Language, mode, "video", request.VideoID}, "\x00")
	}
	parts := []string{request.Language, mode, "title", strings.ToLower(request.Title), strings.ToLower(request.Artist)}
	if request.DurationSeconds > 0 {
		parts = append(parts, strconv.Itoa(int(math.Round(request.DurationSeconds))))
	}
	return strings.Join(parts, "\x00")
}

func snapshotAvailable(snapshot Snapshot) bool {
	switch snapshot.Kind {
	case KindSynced:
		return len(snapshot.Lines) > 0
	case KindPlain:
		return strings.TrimSpace(snapshot.Text) != ""
	default:
		return false
	}
}

func rank(kind string) int {
	switch kind {
	case KindSynced:
		return 2
	case KindPlain:
		return 1
	default:
		return 0
	}
}

func cloneSnapshot(snapshot Snapshot) Snapshot {
	snapshot.Lines = cloneLines(snapshot.Lines)
	return snapshot
}

func cloneLines(lines []Line) []Line {
	if len(lines) == 0 {
		return nil
	}
	clone := make([]Line, len(lines))
	for index, line := range lines {
		clone[index] = line
		if len(line.Words) > 0 {
			clone[index].Words = append([]Word(nil), line.Words...)
		}
	}
	return clone
}
