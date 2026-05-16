package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	listenLiveCatalogManifestURL = "https://updates.dreamapp.cc/xiadown/manifest.json"
	listenLiveCatalogTimeout     = 20 * time.Second
	listenLiveCatalogMaxBodySize = 4 * 1024 * 1024
)

type listenLiveCatalogHTTPClientProvider interface {
	HTTPClient() *http.Client
}

type ListenLiveCatalogHandler struct {
	clientProvider listenLiveCatalogHTTPClientProvider
}

type listenLiveCatalogManifest struct {
	DefaultChannel string                                     `json:"defaultChannel"`
	Listen        listenLiveCatalogManifestListen          `json:"listen"`
	Channels       map[string]listenLiveCatalogManifestEntry `json:"channels"`
}

type listenLiveCatalogManifestEntry struct {
	Listen listenLiveCatalogManifestListen `json:"listen"`
}

type listenLiveCatalogManifestListen struct {
	LiveChannel listenLiveCatalogRemoteRef `json:"liveChannel"`
}

type listenLiveCatalogRemoteRef struct {
	URL string `json:"url"`
}

func NewListenLiveCatalogHandler(clientProvider listenLiveCatalogHTTPClientProvider) *ListenLiveCatalogHandler {
	return &ListenLiveCatalogHandler{clientProvider: clientProvider}
}

func (handler *ListenLiveCatalogHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	ctx, cancel := context.WithTimeout(r.Context(), listenLiveCatalogTimeout)
	defer cancel()

	catalog, err := handler.fetchCatalog(ctx)
	if err != nil {
		http.Error(w, "listen live catalog unavailable", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(catalog)
}

func (handler *ListenLiveCatalogHandler) fetchCatalog(ctx context.Context) ([]byte, error) {
	manifestData, err := handler.fetchJSON(ctx, listenLiveCatalogManifestURL)
	if err != nil {
		return nil, err
	}
	var manifest listenLiveCatalogManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return nil, err
	}
	channelName := strings.TrimSpace(manifest.DefaultChannel)
	if channelName == "" {
		channelName = "stable"
	}
	liveChannel := manifest.Listen.LiveChannel
	if channel, ok := manifest.Channels[channelName]; ok && strings.TrimSpace(channel.Listen.LiveChannel.URL) != "" {
		liveChannel = channel.Listen.LiveChannel
	}
	catalogURL := strings.TrimSpace(liveChannel.URL)
	if catalogURL == "" {
		return nil, fmt.Errorf("listen live catalog url is empty")
	}
	return handler.fetchJSON(ctx, catalogURL)
}

func (handler *ListenLiveCatalogHandler) fetchJSON(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	client := http.DefaultClient
	if handler != nil && handler.clientProvider != nil {
		if provided := handler.clientProvider.HTTPClient(); provided != nil {
			client = provided
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
		return nil, fmt.Errorf("listen live catalog request failed: http %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, listenLiveCatalogMaxBodySize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > listenLiveCatalogMaxBodySize {
		return nil, fmt.Errorf("listen live catalog response is too large")
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("listen live catalog response is not json")
	}
	return data, nil
}
