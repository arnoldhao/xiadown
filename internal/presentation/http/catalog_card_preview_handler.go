package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	libraryservice "xiadown/internal/application/library/service"
)

type CatalogCardPreviewResolver interface {
	ResolvePDF(context.Context, string) (libraryservice.CatalogPDFCardPreview, error)
	ResolveLog(context.Context, string) (libraryservice.CatalogLogCardPreview, error)
	ResolveLogText(context.Context, string) (libraryservice.CatalogLogTextPreview, error)
}

// CatalogCardPreviewHandler serves only preview-safe forms of a registered
// Catalog primary file. The route accepts an opaque item ID, never a path.
type CatalogCardPreviewHandler struct {
	resolver CatalogCardPreviewResolver
}

func NewCatalogCardPreviewHandler(resolver CatalogCardPreviewResolver) *CatalogCardPreviewHandler {
	return &CatalogCardPreviewHandler{resolver: resolver}
}

func (handler *CatalogCardPreviewHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodOptions {
		setCORSHeaders(response, request)
		response.WriteHeader(http.StatusNoContent)
		return
	}
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	setCORSHeaders(response, request)
	if handler == nil || handler.resolver == nil {
		http.Error(response, "preview unavailable", http.StatusServiceUnavailable)
		return
	}

	kind, itemID, ok := catalogCardPreviewRoute(request.URL.Path)
	if !ok || !libraryservice.ValidCatalogCardPreviewItemID(itemID) {
		http.NotFound(response, request)
		return
	}
	switch kind {
	case "pdf":
		handler.servePDF(response, request, itemID)
	case "log":
		if request.URL.Query().Get("detail") == "1" {
			handler.serveLogText(response, request, itemID)
		} else {
			handler.serveLog(response, request, itemID)
		}
	default:
		http.NotFound(response, request)
	}
}

func (handler *CatalogCardPreviewHandler) serveLogText(
	response http.ResponseWriter,
	request *http.Request,
	itemID string,
) {
	preview, err := handler.resolver.ResolveLogText(request.Context(), itemID)
	if err != nil {
		writeCatalogCardPreviewError(response, request, err)
		return
	}
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.Header().Set("X-Content-Type-Options", "nosniff")
	response.Header().Set("Cache-Control", "private, max-age=30, must-revalidate")
	if preview.ETag != "" {
		response.Header().Set("ETag", preview.ETag)
		if request.Header.Get("If-None-Match") == preview.ETag {
			response.WriteHeader(http.StatusNotModified)
			return
		}
	}
	if request.Method == http.MethodHead {
		response.WriteHeader(http.StatusOK)
		return
	}
	payload, err := json.Marshal(struct {
		Text      string `json:"text"`
		Truncated bool   `json:"truncated"`
	}{
		Text: preview.Text, Truncated: preview.Truncated,
	})
	if err != nil {
		http.Error(response, "preview unavailable", http.StatusServiceUnavailable)
		return
	}
	response.Header().Set("Content-Length", strconv.Itoa(len(payload)))
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write(payload)
}

func (handler *CatalogCardPreviewHandler) servePDF(
	response http.ResponseWriter,
	request *http.Request,
	itemID string,
) {
	preview, err := handler.resolver.ResolvePDF(request.Context(), itemID)
	if err != nil {
		writeCatalogCardPreviewError(response, request, err)
		return
	}
	if preview.File == nil {
		http.Error(response, "preview unavailable", http.StatusServiceUnavailable)
		return
	}
	defer preview.File.Close()

	response.Header().Set("Content-Type", "application/pdf")
	response.Header().Set("Content-Disposition", "inline")
	response.Header().Set("X-Content-Type-Options", "nosniff")
	response.Header().Set("Cache-Control", "private, max-age=3600")
	if preview.ETag != "" {
		response.Header().Set("ETag", preview.ETag)
		if request.Header.Get("If-None-Match") == preview.ETag {
			response.WriteHeader(http.StatusNotModified)
			return
		}
	}
	http.ServeContent(response, request, preview.Name, preview.ModTime, preview.File)
}

func (handler *CatalogCardPreviewHandler) serveLog(
	response http.ResponseWriter,
	request *http.Request,
	itemID string,
) {
	preview, err := handler.resolver.ResolveLog(request.Context(), itemID)
	if err != nil {
		writeCatalogCardPreviewError(response, request, err)
		return
	}
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.Header().Set("X-Content-Type-Options", "nosniff")
	response.Header().Set("Cache-Control", "private, max-age=30, must-revalidate")
	if preview.ETag != "" {
		response.Header().Set("ETag", preview.ETag)
		if request.Header.Get("If-None-Match") == preview.ETag {
			response.WriteHeader(http.StatusNotModified)
			return
		}
	}
	if request.Method == http.MethodHead {
		response.WriteHeader(http.StatusOK)
		return
	}
	payload, err := json.Marshal(struct {
		Lines     []string `json:"lines"`
		Truncated bool     `json:"truncated"`
	}{
		Lines: preview.Lines, Truncated: preview.Truncated,
	})
	if err != nil {
		http.Error(response, "preview unavailable", http.StatusServiceUnavailable)
		return
	}
	response.Header().Set("Content-Length", strconv.Itoa(len(payload)))
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write(payload)
}

func catalogCardPreviewRoute(path string) (string, string, bool) {
	const prefix = "/api/library/card-preview/"
	if !strings.HasPrefix(path, prefix) {
		return "", "", false
	}
	parts := strings.Split(strings.TrimPrefix(path, prefix), "/")
	if len(parts) != 2 {
		return "", "", false
	}
	kind := strings.ToLower(strings.TrimSpace(parts[0]))
	itemID := strings.TrimSpace(parts[1])
	return kind, itemID, (kind == "pdf" || kind == "log") && itemID != ""
}

func writeCatalogCardPreviewError(
	response http.ResponseWriter,
	request *http.Request,
	err error,
) {
	if errors.Is(err, libraryservice.ErrCatalogCardPreviewNotFound) {
		http.NotFound(response, request)
		return
	}
	http.Error(response, "preview unavailable", http.StatusServiceUnavailable)
}
