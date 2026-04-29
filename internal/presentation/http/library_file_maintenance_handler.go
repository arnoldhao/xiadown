package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"xiadown/internal/application/library/dto"
)

type LibraryFileMaintenanceService interface {
	VerifyLibraryFiles(context.Context) (dto.VerifyLibraryFilesResponse, error)
	ClearMissingLibraryFiles(context.Context) (dto.ClearMissingLibraryFilesResponse, error)
}

type LibraryFileMaintenanceHandler struct {
	library LibraryFileMaintenanceService
}

func NewLibraryFileMaintenanceHandler(library LibraryFileMaintenanceService) *LibraryFileMaintenanceHandler {
	return &LibraryFileMaintenanceHandler{library: library}
}

func (handler *LibraryFileMaintenanceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setCORSHeaders(w, r)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	setCORSHeaders(w, r)
	if handler == nil || handler.library == nil {
		writeLibraryMaintenanceError(w, http.StatusServiceUnavailable, "library unavailable")
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/library/files"), "/")
	switch path {
	case "verify":
		response, err := handler.library.VerifyLibraryFiles(r.Context())
		if err != nil {
			writeLibraryMaintenanceError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeLibraryMaintenanceJSON(w, response)
	case "clear-missing":
		response, err := handler.library.ClearMissingLibraryFiles(r.Context())
		if err != nil {
			writeLibraryMaintenanceError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeLibraryMaintenanceJSON(w, response)
	default:
		http.NotFound(w, r)
	}
}

func writeLibraryMaintenanceJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(payload)
}

func writeLibraryMaintenanceError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"message": strings.TrimSpace(message),
		},
	})
}
