package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"xiadown/internal/domain/library"
)

type ListenLiveUserCatalogHandler struct {
	repo library.ListenLiveChannelRepository
}

func NewListenLiveUserCatalogHandler(repo library.ListenLiveChannelRepository) *ListenLiveUserCatalogHandler {
	return &ListenLiveUserCatalogHandler{repo: repo}
}

func (handler *ListenLiveUserCatalogHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setCORSHeaders(w, r)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	setCORSHeaders(w, r)
	if handler == nil || handler.repo == nil {
		writeListenLiveUserCatalogError(w, http.StatusServiceUnavailable, "listen live catalog storage unavailable")
		return
	}
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		handler.serveList(w, r)
	case http.MethodPost, http.MethodPut:
		handler.serveReplace(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (handler *ListenLiveUserCatalogHandler) serveList(w http.ResponseWriter, r *http.Request) {
	snapshot, err := handler.repo.List(r.Context())
	if err != nil {
		writeListenLiveUserCatalogError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeListenLiveUserCatalogJSON(w, r, snapshot)
}

func (handler *ListenLiveUserCatalogHandler) serveReplace(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var snapshot library.ListenLiveCatalogSnapshot
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&snapshot); err != nil {
		writeListenLiveUserCatalogError(w, http.StatusBadRequest, "invalid listen live catalog payload")
		return
	}
	if err := handler.repo.Replace(r.Context(), snapshot); err != nil {
		writeListenLiveUserCatalogError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeListenLiveUserCatalogJSON(w, r, map[string]bool{"ok": true})
}

func writeListenLiveUserCatalogJSON(w http.ResponseWriter, r *http.Request, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodHead {
		return
	}
	_ = json.NewEncoder(w).Encode(payload)
}

func writeListenLiveUserCatalogError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{"message": strings.TrimSpace(message)},
	})
}
