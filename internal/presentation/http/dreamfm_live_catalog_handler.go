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
	dreamFMLiveCatalogManifestURL = "https://updates.dreamapp.cc/xiadown/manifest.json"
	dreamFMLiveCatalogTimeout     = 20 * time.Second
	dreamFMLiveCatalogMaxBodySize = 4 * 1024 * 1024
)

type dreamFMLiveCatalogHTTPClientProvider interface {
	HTTPClient() *http.Client
}

type DreamFMLiveCatalogHandler struct {
	clientProvider dreamFMLiveCatalogHTTPClientProvider
}

type dreamFMLiveCatalogManifest struct {
	DefaultChannel string                                     `json:"defaultChannel"`
	DreamFM        dreamFMLiveCatalogManifestDreamFM          `json:"dreamFm"`
	Channels       map[string]dreamFMLiveCatalogManifestEntry `json:"channels"`
}

type dreamFMLiveCatalogManifestEntry struct {
	DreamFM dreamFMLiveCatalogManifestDreamFM `json:"dreamFm"`
}

type dreamFMLiveCatalogManifestDreamFM struct {
	LiveChannel dreamFMLiveCatalogRemoteRef `json:"liveChannel"`
}

type dreamFMLiveCatalogRemoteRef struct {
	URL string `json:"url"`
}

func NewDreamFMLiveCatalogHandler(clientProvider dreamFMLiveCatalogHTTPClientProvider) *DreamFMLiveCatalogHandler {
	return &DreamFMLiveCatalogHandler{clientProvider: clientProvider}
}

func (handler *DreamFMLiveCatalogHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	ctx, cancel := context.WithTimeout(r.Context(), dreamFMLiveCatalogTimeout)
	defer cancel()

	catalog, err := handler.fetchCatalog(ctx)
	if err != nil {
		http.Error(w, "dreamfm live catalog unavailable", http.StatusBadGateway)
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

func (handler *DreamFMLiveCatalogHandler) fetchCatalog(ctx context.Context) ([]byte, error) {
	manifestData, err := handler.fetchJSON(ctx, dreamFMLiveCatalogManifestURL)
	if err != nil {
		return nil, err
	}
	var manifest dreamFMLiveCatalogManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return nil, err
	}
	channelName := strings.TrimSpace(manifest.DefaultChannel)
	if channelName == "" {
		channelName = "stable"
	}
	liveChannel := manifest.DreamFM.LiveChannel
	if channel, ok := manifest.Channels[channelName]; ok && strings.TrimSpace(channel.DreamFM.LiveChannel.URL) != "" {
		liveChannel = channel.DreamFM.LiveChannel
	}
	catalogURL := strings.TrimSpace(liveChannel.URL)
	if catalogURL == "" {
		return nil, fmt.Errorf("dreamfm live catalog url is empty")
	}
	return handler.fetchJSON(ctx, catalogURL)
}

func (handler *DreamFMLiveCatalogHandler) fetchJSON(ctx context.Context, rawURL string) ([]byte, error) {
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
		return nil, fmt.Errorf("dreamfm live catalog request failed: http %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, dreamFMLiveCatalogMaxBodySize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > dreamFMLiveCatalogMaxBodySize {
		return nil, fmt.Errorf("dreamfm live catalog response is too large")
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("dreamfm live catalog response is not json")
	}
	return data, nil
}
