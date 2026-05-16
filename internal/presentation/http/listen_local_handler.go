package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"xiadown/internal/application/library/dto"
)

type ListenLocalLibraryService interface {
	ListListenLocalTracks(context.Context, dto.ListListenLocalTracksRequest) ([]dto.ListenLocalTrackDTO, error)
	RefreshListenLocalIndex(context.Context, dto.RefreshListenLocalIndexRequest) (dto.ListenLocalIndexRefreshResponse, error)
	RemoveListenLocalTrack(context.Context, dto.RemoveListenLocalTrackRequest) error
	ClearMissingListenLocalTracks(context.Context) (dto.ClearMissingListenLocalTracksResponse, error)
}

type ListenLocalHandler struct {
	library ListenLocalLibraryService
}

func NewListenLocalHandler(library ListenLocalLibraryService) *ListenLocalHandler {
	return &ListenLocalHandler{library: library}
}

func (handler *ListenLocalHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setCORSHeaders(w, r)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	setCORSHeaders(w, r)
	if handler == nil || handler.library == nil {
		writeListenLocalError(w, http.StatusServiceUnavailable, "local library unavailable")
		return
	}

	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/listen/local"), "/")
	switch {
	case path == "" && (r.Method == http.MethodGet || r.Method == http.MethodHead):
		handler.serveList(w, r)
	case path == "refresh" && r.Method == http.MethodPost:
		handler.serveRefresh(w, r)
	case path == "clear-missing" && r.Method == http.MethodPost:
		handler.serveClearMissing(w, r)
	case path == "" && r.Method == http.MethodDelete:
		handler.serveRemove(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (handler *ListenLocalHandler) serveList(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
	offset, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("offset")))
	items, err := handler.library.ListListenLocalTracks(r.Context(), dto.ListListenLocalTracksRequest{
		Query:              r.URL.Query().Get("query"),
		IncludeUnavailable: strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("includeUnavailable")), "true"),
		Limit:              limit,
		Offset:             offset,
	})
	if err != nil {
		writeListenLocalError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeListenLocalJSON(w, r, map[string]any{"items": items})
}

func (handler *ListenLocalHandler) serveRefresh(w http.ResponseWriter, r *http.Request) {
	request := dto.RefreshListenLocalIndexRequest{
		FileID:    strings.TrimSpace(r.URL.Query().Get("fileId")),
		LibraryID: strings.TrimSpace(r.URL.Query().Get("libraryId")),
	}
	response, err := handler.library.RefreshListenLocalIndex(r.Context(), request)
	if err != nil {
		writeListenLocalError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeListenLocalJSON(w, r, response)
}

func (handler *ListenLocalHandler) serveClearMissing(w http.ResponseWriter, r *http.Request) {
	response, err := handler.library.ClearMissingListenLocalTracks(r.Context())
	if err != nil {
		writeListenLocalError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeListenLocalJSON(w, r, response)
}

func (handler *ListenLocalHandler) serveRemove(w http.ResponseWriter, r *http.Request) {
	fileID := strings.TrimSpace(r.URL.Query().Get("fileId"))
	if fileID == "" {
		writeListenLocalError(w, http.StatusBadRequest, "fileId is required")
		return
	}
	if err := handler.library.RemoveListenLocalTrack(r.Context(), dto.RemoveListenLocalTrackRequest{FileID: fileID}); err != nil {
		writeListenLocalError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeListenLocalJSON(w http.ResponseWriter, r *http.Request, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.Method == http.MethodHead {
		return
	}
	_ = json.NewEncoder(w).Encode(payload)
}

func writeListenLocalError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"message": strings.TrimSpace(message),
		},
	})
}
