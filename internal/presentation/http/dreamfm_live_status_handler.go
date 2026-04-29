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
	dreamFMLiveStatusTimeout     = 14 * time.Second
	dreamFMLiveStatusCacheTTL    = 2 * time.Minute
	dreamFMLiveStatusMaxVideoIDs = 60
	dreamFMLiveStatusMaxBodySize = 3 * 1024 * 1024
)

type dreamFMLiveStatusHTTPClientProvider interface {
	HTTPClient() *http.Client
}

type DreamFMLiveStatusHandler struct {
	clientProvider dreamFMLiveStatusHTTPClientProvider
	now            func() time.Time

	mu    sync.Mutex
	cache map[string]dreamFMLiveStatusCacheEntry
}

type dreamFMLiveStatusCacheEntry struct {
	status    DreamFMLiveStatusItem
	expiresAt time.Time
}

type DreamFMLiveStatusResponse struct {
	Statuses []DreamFMLiveStatusItem `json:"statuses"`
}

type DreamFMLiveStatusItem struct {
	VideoID string `json:"videoId"`
	Status  string `json:"status"`
	Detail  string `json:"detail,omitempty"`
}

func NewDreamFMLiveStatusHandler(clientProvider dreamFMLiveStatusHTTPClientProvider) *DreamFMLiveStatusHandler {
	return &DreamFMLiveStatusHandler{
		clientProvider: clientProvider,
		now:            time.Now,
		cache:          make(map[string]dreamFMLiveStatusCacheEntry),
	}
}

func (handler *DreamFMLiveStatusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	videoIDs, err := dreamFMLiveStatusVideoIDs(r.URL.Query())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(videoIDs) == 0 {
		writeDreamFMLiveStatusJSON(w, r, DreamFMLiveStatusResponse{Statuses: []DreamFMLiveStatusItem{}})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), dreamFMLiveStatusTimeout)
	defer cancel()

	statuses := handler.statuses(ctx, videoIDs)
	writeDreamFMLiveStatusJSON(w, r, DreamFMLiveStatusResponse{Statuses: statuses})
}

func dreamFMLiveStatusVideoIDs(query url.Values) ([]string, error) {
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
		if len(videoIDs) >= dreamFMLiveStatusMaxVideoIDs {
			break
		}
	}
	return videoIDs, nil
}

func (handler *DreamFMLiveStatusHandler) statuses(ctx context.Context, videoIDs []string) []DreamFMLiveStatusItem {
	statuses := make([]DreamFMLiveStatusItem, len(videoIDs))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for index, videoID := range videoIDs {
		if cached, ok := handler.cachedStatus(videoID); ok {
			statuses[index] = cached
			continue
		}
		wg.Add(1)
		go func(index int, videoID string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				statuses[index] = dreamFMLiveUnknownStatus(videoID, ctx.Err().Error())
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
			statuses[index] = dreamFMLiveUnknownStatus(videoIDs[index], "status unavailable")
		}
	}
	return statuses
}

func (handler *DreamFMLiveStatusHandler) cachedStatus(videoID string) (DreamFMLiveStatusItem, bool) {
	handler.mu.Lock()
	defer handler.mu.Unlock()
	entry, ok := handler.cache[videoID]
	if !ok || handler.now().After(entry.expiresAt) {
		delete(handler.cache, videoID)
		return DreamFMLiveStatusItem{}, false
	}
	return entry.status, true
}

func (handler *DreamFMLiveStatusHandler) storeStatus(status DreamFMLiveStatusItem) {
	if status.VideoID == "" {
		return
	}
	handler.mu.Lock()
	defer handler.mu.Unlock()
	handler.cache[status.VideoID] = dreamFMLiveStatusCacheEntry{
		status:    status,
		expiresAt: handler.now().Add(dreamFMLiveStatusCacheTTL),
	}
}

func (handler *DreamFMLiveStatusHandler) fetchStatus(ctx context.Context, videoID string) DreamFMLiveStatusItem {
	watchURL := "https://www.youtube.com/watch?v=" + url.QueryEscape(videoID) + "&bpctr=9999999999&has_verified=1&hl=en"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, watchURL, nil)
	if err != nil {
		return dreamFMLiveUnknownStatus(videoID, err.Error())
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
		return dreamFMLiveUnknownStatus(videoID, err.Error())
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, dreamFMLiveStatusMaxBodySize))
	if resp.StatusCode == http.StatusNotFound {
		return DreamFMLiveStatusItem{VideoID: videoID, Status: "unavailable", Detail: "status 404"}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return dreamFMLiveUnknownStatus(videoID, fmt.Sprintf("status %d", resp.StatusCode))
	}
	if readErr != nil {
		return dreamFMLiveUnknownStatus(videoID, readErr.Error())
	}
	return resolveDreamFMLiveStatusFromHTML(videoID, string(body))
}

func resolveDreamFMLiveStatusFromHTML(videoID string, html string) DreamFMLiveStatusItem {
	value := strings.TrimSpace(html)
	if value == "" {
		return dreamFMLiveUnknownStatus(videoID, "empty response")
	}
	compact := strings.ReplaceAll(value, " ", "")
	lower := strings.ToLower(value)

	if strings.Contains(compact, `"isLiveNow":true`) ||
		strings.Contains(compact, `"isLive":true`) ||
		strings.Contains(compact, `"liveBroadcastContent":"live"`) {
		return DreamFMLiveStatusItem{VideoID: videoID, Status: "live"}
	}
	if strings.Contains(compact, `"isUpcoming":true`) ||
		strings.Contains(compact, `"liveBroadcastContent":"upcoming"`) ||
		strings.Contains(compact, `"upcomingEventData"`) {
		return DreamFMLiveStatusItem{VideoID: videoID, Status: "upcoming"}
	}
	if strings.Contains(compact, `"status":"LIVE_STREAM_OFFLINE"`) {
		return DreamFMLiveStatusItem{VideoID: videoID, Status: "offline"}
	}
	if strings.Contains(compact, `"status":"ERROR"`) ||
		strings.Contains(compact, `"status":"UNPLAYABLE"`) ||
		strings.Contains(compact, `"status":"LOGIN_REQUIRED"`) ||
		strings.Contains(lower, "video unavailable") {
		return DreamFMLiveStatusItem{VideoID: videoID, Status: "unavailable"}
	}
	if strings.Contains(compact, `"isLiveContent":true`) ||
		strings.Contains(compact, `"liveBroadcastContent":"none"`) {
		return DreamFMLiveStatusItem{VideoID: videoID, Status: "offline"}
	}
	return dreamFMLiveUnknownStatus(videoID, "live state not found")
}

func dreamFMLiveUnknownStatus(videoID string, detail string) DreamFMLiveStatusItem {
	return DreamFMLiveStatusItem{
		VideoID: strings.TrimSpace(videoID),
		Status:  "unknown",
		Detail:  strings.TrimSpace(detail),
	}
}

func writeDreamFMLiveStatusJSON(w http.ResponseWriter, r *http.Request, response DreamFMLiveStatusResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method == http.MethodHead {
		return
	}
	_ = json.NewEncoder(w).Encode(response)
}
