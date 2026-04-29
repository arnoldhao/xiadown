package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"xiadown/internal/application/library/dto"
)

type DreamFMLocalLibraryService interface {
	ListDreamFMLocalTracks(context.Context, dto.ListDreamFMLocalTracksRequest) ([]dto.DreamFMLocalTrackDTO, error)
	RefreshDreamFMLocalIndex(context.Context, dto.RefreshDreamFMLocalIndexRequest) (dto.DreamFMLocalIndexRefreshResponse, error)
	RemoveDreamFMLocalTrack(context.Context, dto.RemoveDreamFMLocalTrackRequest) error
	ClearMissingDreamFMLocalTracks(context.Context) (dto.ClearMissingDreamFMLocalTracksResponse, error)
}

type DreamFMLocalHandler struct {
	library DreamFMLocalLibraryService
}

func NewDreamFMLocalHandler(library DreamFMLocalLibraryService) *DreamFMLocalHandler {
	return &DreamFMLocalHandler{library: library}
}

func (handler *DreamFMLocalHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setCORSHeaders(w, r)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	setCORSHeaders(w, r)
	if handler == nil || handler.library == nil {
		writeDreamFMLocalError(w, http.StatusServiceUnavailable, "local library unavailable")
		return
	}

	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/dreamfm/local"), "/")
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

func (handler *DreamFMLocalHandler) serveList(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
	offset, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("offset")))
	items, err := handler.library.ListDreamFMLocalTracks(r.Context(), dto.ListDreamFMLocalTracksRequest{
		Query:              r.URL.Query().Get("query"),
		IncludeUnavailable: strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("includeUnavailable")), "true"),
		Limit:              limit,
		Offset:             offset,
	})
	if err != nil {
		writeDreamFMLocalError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeDreamFMLocalJSON(w, r, map[string]any{"items": items})
}

func (handler *DreamFMLocalHandler) serveRefresh(w http.ResponseWriter, r *http.Request) {
	request := dto.RefreshDreamFMLocalIndexRequest{
		FileID:    strings.TrimSpace(r.URL.Query().Get("fileId")),
		LibraryID: strings.TrimSpace(r.URL.Query().Get("libraryId")),
	}
	response, err := handler.library.RefreshDreamFMLocalIndex(r.Context(), request)
	if err != nil {
		writeDreamFMLocalError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeDreamFMLocalJSON(w, r, response)
}

func (handler *DreamFMLocalHandler) serveClearMissing(w http.ResponseWriter, r *http.Request) {
	response, err := handler.library.ClearMissingDreamFMLocalTracks(r.Context())
	if err != nil {
		writeDreamFMLocalError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeDreamFMLocalJSON(w, r, response)
}

func (handler *DreamFMLocalHandler) serveRemove(w http.ResponseWriter, r *http.Request) {
	fileID := strings.TrimSpace(r.URL.Query().Get("fileId"))
	if fileID == "" {
		writeDreamFMLocalError(w, http.StatusBadRequest, "fileId is required")
		return
	}
	if err := handler.library.RemoveDreamFMLocalTrack(r.Context(), dto.RemoveDreamFMLocalTrackRequest{FileID: fileID}); err != nil {
		writeDreamFMLocalError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeDreamFMLocalJSON(w http.ResponseWriter, r *http.Request, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.Method == http.MethodHead {
		return
	}
	_ = json.NewEncoder(w).Encode(payload)
}

func writeDreamFMLocalError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"message": strings.TrimSpace(message),
		},
	})
}
