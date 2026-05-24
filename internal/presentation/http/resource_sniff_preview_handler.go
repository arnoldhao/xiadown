package http

import (
	"net/http"

	libraryservice "xiadown/internal/application/library/service"
)

type ResourceSniffPreviewHandler struct {
	service *libraryservice.LibraryService
}

func NewResourceSniffPreviewHandler(service *libraryservice.LibraryService) *ResourceSniffPreviewHandler {
	return &ResourceSniffPreviewHandler{service: service}
}

func (handler *ResourceSniffPreviewHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setCORSHeaders(w, r)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	setCORSHeaders(w, r)
	if handler == nil || handler.service == nil {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	handler.service.ServeResourceSniffPreview(w, r)
}
