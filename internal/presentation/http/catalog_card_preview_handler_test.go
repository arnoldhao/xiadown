package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	libraryservice "xiadown/internal/application/library/service"
)

type catalogCardPreviewResolverStub struct {
	itemID  string
	pdf     libraryservice.CatalogPDFCardPreview
	log     libraryservice.CatalogLogCardPreview
	logText libraryservice.CatalogLogTextPreview
	err     error
}

func (resolver *catalogCardPreviewResolverStub) ResolveLogText(
	_ context.Context,
	itemID string,
) (libraryservice.CatalogLogTextPreview, error) {
	resolver.itemID = itemID
	return resolver.logText, resolver.err
}

func (resolver *catalogCardPreviewResolverStub) ResolvePDF(
	_ context.Context,
	itemID string,
) (libraryservice.CatalogPDFCardPreview, error) {
	resolver.itemID = itemID
	return resolver.pdf, resolver.err
}

func (resolver *catalogCardPreviewResolverStub) ResolveLog(
	_ context.Context,
	itemID string,
) (libraryservice.CatalogLogCardPreview, error) {
	resolver.itemID = itemID
	return resolver.log, resolver.err
}

func TestCatalogCardPreviewHandlerServesPDFRangesAndClosesSource(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "guide.pdf")
	if err := os.WriteFile(path, []byte("%PDF-1.7\nfixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	resolver := &catalogCardPreviewResolverStub{
		pdf: libraryservice.CatalogPDFCardPreview{
			File: file, Name: "guide.pdf", Size: 16,
			ModTime: time.Date(2026, 7, 26, 10, 0, 0, 0, time.UTC),
			ETag:    `"pdf-etag"`,
		},
	}
	handler := NewCatalogCardPreviewHandler(resolver)
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/library/card-preview/pdf/item-pdf?path=/etc/passwd",
		nil,
	)
	request.Header.Set("Range", "bytes=0-3")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusPartialContent || response.Body.String() != "%PDF" {
		t.Fatalf("PDF range response = %d %q", response.Code, response.Body.String())
	}
	if resolver.itemID != "item-pdf" ||
		response.Header().Get("Content-Type") != "application/pdf" ||
		response.Header().Get("ETag") != `"pdf-etag"` ||
		response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("unexpected PDF response headers: %v", response.Header())
	}
	if _, err := file.Stat(); err == nil {
		t.Fatal("PDF response did not close its source file")
	}
}

func TestCatalogCardPreviewHandlerServesBoundedLogJSONAndConditionalResponse(t *testing.T) {
	t.Parallel()
	resolver := &catalogCardPreviewResolverStub{
		log: libraryservice.CatalogLogCardPreview{
			Lines: []string{"starting", "complete"}, Truncated: true, ETag: `"log-etag"`,
		},
	}
	handler := NewCatalogCardPreviewHandler(resolver)
	response := httptest.NewRecorder()
	handler.ServeHTTP(
		response,
		httptest.NewRequest(http.MethodGet, "/api/library/card-preview/log/item-log", nil),
	)
	if response.Code != http.StatusOK ||
		response.Header().Get("Content-Type") != "application/json; charset=utf-8" ||
		!strings.Contains(response.Body.String(), `"lines":["starting","complete"]`) ||
		!strings.Contains(response.Body.String(), `"truncated":true`) {
		t.Fatalf("LOG response = %d %v %q", response.Code, response.Header(), response.Body.String())
	}

	conditional := httptest.NewRequest(
		http.MethodGet,
		"/api/library/card-preview/log/item-log",
		nil,
	)
	conditional.Header.Set("If-None-Match", `"log-etag"`)
	conditionalResponse := httptest.NewRecorder()
	handler.ServeHTTP(conditionalResponse, conditional)
	if conditionalResponse.Code != http.StatusNotModified || conditionalResponse.Body.Len() != 0 {
		t.Fatalf("conditional LOG response = %d %q", conditionalResponse.Code, conditionalResponse.Body.String())
	}
}

func TestCatalogCardPreviewHandlerServesLogTextOnlyForExplicitDetailRequest(t *testing.T) {
	t.Parallel()
	resolver := &catalogCardPreviewResolverStub{
		logText: libraryservice.CatalogLogTextPreview{
			Text: "line one\nline two", Truncated: true, ETag: `"log-text-etag"`,
		},
	}
	handler := NewCatalogCardPreviewHandler(resolver)
	response := httptest.NewRecorder()
	handler.ServeHTTP(
		response,
		httptest.NewRequest(
			http.MethodGet,
			"/api/library/card-preview/log/item-log?detail=1",
			nil,
		),
	)
	if response.Code != http.StatusOK ||
		response.Header().Get("ETag") != `"log-text-etag"` ||
		!strings.Contains(response.Body.String(), `"text":"line one\nline two"`) ||
		!strings.Contains(response.Body.String(), `"truncated":true`) {
		t.Fatalf("LOG detail response = %d %v %q", response.Code, response.Header(), response.Body.String())
	}
}

func TestCatalogCardPreviewHandlerRejectsUnknownKindsAndPathShapedIDs(t *testing.T) {
	t.Parallel()
	resolver := &catalogCardPreviewResolverStub{}
	handler := NewCatalogCardPreviewHandler(resolver)
	for _, target := range []string{
		"/api/library/card-preview/pdf/",
		"/api/library/card-preview/pdf/folder/item",
		"/api/library/card-preview/text/item",
		"/api/library/card-preview/log/" + strings.Repeat("x", 256),
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
		if response.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want 404", target, response.Code)
		}
	}
	if resolver.itemID != "" {
		t.Fatalf("invalid request reached resolver: %q", resolver.itemID)
	}
}
