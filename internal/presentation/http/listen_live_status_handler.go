package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"xiadown/internal/application/youtubemusic"
)

const (
	listenLiveStatusTimeout         = 14 * time.Second
	listenLiveStatusCacheTTL        = 5 * time.Minute
	listenLiveStatusRefreshInterval = 2 * time.Minute
	listenLiveStatusMaxVideoIDs     = 60
	listenLiveStatusMaxBodySize     = 3 * 1024 * 1024
)

type listenLiveStatusHTTPClientProvider interface {
	HTTPClient() *http.Client
}

type ListenLiveStatusHandler struct {
	clientProvider listenLiveStatusHTTPClientProvider
	now            func() time.Time

	mu            sync.Mutex
	cache         map[string]listenLiveStatusCacheEntry
	tracked       map[string]struct{}
	workerStarted bool
}

type listenLiveStatusCacheEntry struct {
	status    ListenLiveStatusItem
	expiresAt time.Time
}

type ListenLiveStatusResponse struct {
	Statuses []ListenLiveStatusItem `json:"statuses"`
}

type ListenLiveStatusItem struct {
	VideoID string `json:"videoId"`
	Status  string `json:"status"`
	Detail  string `json:"detail,omitempty"`
}

func NewListenLiveStatusHandler(clientProvider listenLiveStatusHTTPClientProvider) *ListenLiveStatusHandler {
	return &ListenLiveStatusHandler{
		clientProvider: clientProvider,
		now:            time.Now,
		cache:          make(map[string]listenLiveStatusCacheEntry),
		tracked:        make(map[string]struct{}),
	}
}

func (handler *ListenLiveStatusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setCORSHeaders(w, r)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	setCORSHeaders(w, r)

	videoIDs, err := listenLiveStatusVideoIDs(r.URL.Query())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(videoIDs) == 0 {
		writeListenLiveStatusJSON(w, r, ListenLiveStatusResponse{Statuses: []ListenLiveStatusItem{}})
		return
	}

	handler.trackVideoIDs(videoIDs)
	handler.ensureStatusWorker()
	statuses := handler.cachedStatuses(videoIDs)
	writeListenLiveStatusJSON(w, r, ListenLiveStatusResponse{Statuses: statuses})
}

func listenLiveStatusVideoIDs(query url.Values) ([]string, error) {
	rawIDs := query["id"]
	for _, value := range query["ids"] {
		rawIDs = append(rawIDs, strings.Split(value, ",")...)
	}
	seen := make(map[string]struct{})
	videoIDs := make([]string, 0, len(rawIDs))
	for _, rawID := range rawIDs {
		videoID := strings.TrimSpace(rawID)
		if videoID == "" {
			continue
		}
		if !youtubeVideoIDPattern.MatchString(videoID) {
			return nil, fmt.Errorf("invalid youtube video id: %s", videoID)
		}
		if _, ok := seen[videoID]; ok {
			continue
		}
		seen[videoID] = struct{}{}
		videoIDs = append(videoIDs, videoID)
		if len(videoIDs) >= listenLiveStatusMaxVideoIDs {
			break
		}
	}
	return videoIDs, nil
}

func (handler *ListenLiveStatusHandler) cachedStatuses(videoIDs []string) []ListenLiveStatusItem {
	statuses := make([]ListenLiveStatusItem, 0, len(videoIDs))
	for _, videoID := range videoIDs {
		if cached, ok := handler.cachedStatus(videoID); ok {
			statuses = append(statuses, cached)
		}
	}
	return statuses
}

func (handler *ListenLiveStatusHandler) fetchStatuses(ctx context.Context, videoIDs []string) []ListenLiveStatusItem {
	statuses := make([]ListenLiveStatusItem, len(videoIDs))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for index, videoID := range videoIDs {
		wg.Add(1)
		go func(index int, videoID string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				statuses[index] = listenLiveUnknownStatus(videoID, ctx.Err().Error())
				return
			}
			status := handler.fetchStatus(ctx, videoID)
			handler.storeStatus(status)
			statuses[index] = status
		}(index, videoID)
	}
	wg.Wait()
	for index, status := range statuses {
		if status.VideoID == "" {
			statuses[index] = listenLiveUnknownStatus(videoIDs[index], "status unavailable")
		}
	}
	return statuses
}

func (handler *ListenLiveStatusHandler) trackVideoIDs(videoIDs []string) {
	handler.mu.Lock()
	defer handler.mu.Unlock()
	for _, videoID := range videoIDs {
		if trimmed := strings.TrimSpace(videoID); trimmed != "" {
			handler.tracked[trimmed] = struct{}{}
		}
	}
}

func (handler *ListenLiveStatusHandler) trackedVideoIDs() []string {
	handler.mu.Lock()
	defer handler.mu.Unlock()
	videoIDs := make([]string, 0, len(handler.tracked))
	for videoID := range handler.tracked {
		videoIDs = append(videoIDs, videoID)
		if len(videoIDs) >= listenLiveStatusMaxVideoIDs {
			break
		}
	}
	return videoIDs
}

func (handler *ListenLiveStatusHandler) ensureStatusWorker() {
	handler.mu.Lock()
	if handler.workerStarted {
		handler.mu.Unlock()
		return
	}
	handler.workerStarted = true
	handler.mu.Unlock()

	go handler.runStatusWorker()
}

func (handler *ListenLiveStatusHandler) runStatusWorker() {
	handler.refreshTrackedStatuses()
	ticker := time.NewTicker(listenLiveStatusRefreshInterval)
	defer ticker.Stop()
	for range ticker.C {
		handler.refreshTrackedStatuses()
	}
}

func (handler *ListenLiveStatusHandler) refreshTrackedStatuses() {
	videoIDs := handler.trackedVideoIDs()
	if len(videoIDs) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), listenLiveStatusTimeout)
	defer cancel()
	_ = handler.fetchStatuses(ctx, videoIDs)
}

func (handler *ListenLiveStatusHandler) cachedStatus(videoID string) (ListenLiveStatusItem, bool) {
	handler.mu.Lock()
	defer handler.mu.Unlock()
	entry, ok := handler.cache[videoID]
	if !ok || handler.now().After(entry.expiresAt) {
		delete(handler.cache, videoID)
		return ListenLiveStatusItem{}, false
	}
	return entry.status, true
}

func (handler *ListenLiveStatusHandler) storeStatus(status ListenLiveStatusItem) {
	if status.VideoID == "" {
		return
	}
	handler.mu.Lock()
	defer handler.mu.Unlock()
	handler.cache[status.VideoID] = listenLiveStatusCacheEntry{
		status:    status,
		expiresAt: handler.now().Add(listenLiveStatusCacheTTL),
	}
}

func (handler *ListenLiveStatusHandler) fetchStatus(ctx context.Context, videoID string) ListenLiveStatusItem {
	watchURL := "https://www.youtube.com/watch?v=" + url.QueryEscape(videoID) + "&bpctr=9999999999&has_verified=1&hl=en"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, watchURL, nil)
	if err != nil {
		return listenLiveUnknownStatus(videoID, err.Error())
	}
	req.Header.Set("User-Agent", youtubemusic.BrowserUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", "https://www.youtube.com/")

	client := http.DefaultClient
	if handler.clientProvider != nil {
		if provided := handler.clientProvider.HTTPClient(); provided != nil {
			client = provided
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return listenLiveUnknownStatus(videoID, err.Error())
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, listenLiveStatusMaxBodySize))
	if resp.StatusCode == http.StatusNotFound {
		return ListenLiveStatusItem{VideoID: videoID, Status: "unavailable", Detail: "status 404"}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return listenLiveUnknownStatus(videoID, fmt.Sprintf("status %d", resp.StatusCode))
	}
	if readErr != nil {
		return listenLiveUnknownStatus(videoID, readErr.Error())
	}
	return resolveListenLiveStatusFromHTML(videoID, string(body))
}

func resolveListenLiveStatusFromHTML(videoID string, html string) ListenLiveStatusItem {
	value := strings.TrimSpace(html)
	if value == "" {
		return listenLiveUnknownStatus(videoID, "empty response")
	}
	compact := strings.ReplaceAll(value, " ", "")
	lower := strings.ToLower(value)

	if strings.Contains(compact, `"isLiveNow":true`) ||
		strings.Contains(compact, `"isLive":true`) ||
		strings.Contains(compact, `"liveBroadcastContent":"live"`) {
		return ListenLiveStatusItem{VideoID: videoID, Status: "live"}
	}
	if strings.Contains(compact, `"isUpcoming":true`) ||
		strings.Contains(compact, `"liveBroadcastContent":"upcoming"`) ||
		strings.Contains(compact, `"upcomingEventData"`) {
		return ListenLiveStatusItem{VideoID: videoID, Status: "upcoming"}
	}
	if strings.Contains(compact, `"status":"LIVE_STREAM_OFFLINE"`) {
		return ListenLiveStatusItem{VideoID: videoID, Status: "offline"}
	}
	if strings.Contains(compact, `"status":"ERROR"`) ||
		strings.Contains(compact, `"status":"UNPLAYABLE"`) ||
		strings.Contains(compact, `"status":"LOGIN_REQUIRED"`) ||
		strings.Contains(lower, "video unavailable") {
		return ListenLiveStatusItem{VideoID: videoID, Status: "unavailable"}
	}
	if strings.Contains(compact, `"isLiveContent":true`) ||
		strings.Contains(compact, `"liveBroadcastContent":"none"`) {
		return ListenLiveStatusItem{VideoID: videoID, Status: "offline"}
	}
	return listenLiveUnknownStatus(videoID, "live state not found")
}

func listenLiveUnknownStatus(videoID string, detail string) ListenLiveStatusItem {
	return ListenLiveStatusItem{
		VideoID: strings.TrimSpace(videoID),
		Status:  "unknown",
		Detail:  strings.TrimSpace(detail),
	}
}

func writeListenLiveStatusJSON(w http.ResponseWriter, r *http.Request, response ListenLiveStatusResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method == http.MethodHead {
		return
	}
	_ = json.NewEncoder(w).Encode(response)
}
