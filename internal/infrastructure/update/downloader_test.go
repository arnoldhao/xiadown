package update

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestNewHTTPDownloaderUsesDedicatedClientTimeout(t *testing.T) {
	t.Parallel()

	base := &http.Client{Timeout: 30 * time.Second}
	downloader := NewHTTPDownloader(base)
	if downloader.client == nil {
		t.Fatal("expected downloader client")
	}
	if downloader.client.Timeout != 0 {
		t.Fatalf("expected dedicated download client timeout to be cleared, got %s", downloader.client.Timeout)
	}
	if base.Timeout != 30*time.Second {
		t.Fatalf("expected source client timeout to remain unchanged, got %s", base.Timeout)
	}
}

func TestHTTPDownloaderRetriesWithRangeResume(t *testing.T) {
	t.Parallel()

	payload := bytes.Repeat([]byte("xiadown-update-"), 2048)
	half := len(payload) / 2

	var (
		mu           sync.Mutex
		requestCount int
		rangeHeaders []string
	)

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		mu.Lock()
		requestCount++
		attempt := requestCount
		rangeHeaders = append(rangeHeaders, request.Header.Get("Range"))
		mu.Unlock()

		if attempt == 1 {
			writer.Header().Set("Content-Length", strconv.Itoa(len(payload)))
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write(payload[:half])
			if flusher, ok := writer.(http.Flusher); ok {
				flusher.Flush()
			}
			forceCloseConnection(writer)
			return
		}

		writer.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", half, len(payload)-1, len(payload)))
		writer.Header().Set("Content-Length", strconv.Itoa(len(payload)-half))
		writer.WriteHeader(http.StatusPartialContent)
		_, _ = writer.Write(payload[half:])
	}))
	defer server.Close()

	downloader := NewHTTPDownloader(server.Client())
	downloader.maxAttempts = 2
	downloader.backoffBase = time.Millisecond

	path, err := downloader.Download(context.Background(), server.URL+"/xiadown.zip", nil)
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}

	downloaded, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read download failed: %v", err)
	}
	if !bytes.Equal(downloaded, payload) {
		t.Fatal("downloaded payload mismatch")
	}

	mu.Lock()
	defer mu.Unlock()
	if requestCount != 2 {
		t.Fatalf("expected 2 requests, got %d", requestCount)
	}
	expectedRange := fmt.Sprintf("bytes=%d-", half)
	if len(rangeHeaders) < 2 || rangeHeaders[1] != expectedRange {
		t.Fatalf("expected resume request %q, got %#v", expectedRange, rangeHeaders)
	}
}

func TestHTTPDownloaderRestartsWhenServerIgnoresRangeRequest(t *testing.T) {
	t.Parallel()

	payload := bytes.Repeat([]byte("xiadown-portable-"), 1536)
	half := len(payload) / 2

	var (
		mu           sync.Mutex
		requestCount int
		rangeHeaders []string
	)

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		mu.Lock()
		requestCount++
		attempt := requestCount
		rangeHeaders = append(rangeHeaders, request.Header.Get("Range"))
		mu.Unlock()

		if attempt == 1 {
			writer.Header().Set("Content-Length", strconv.Itoa(len(payload)))
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write(payload[:half])
			if flusher, ok := writer.(http.Flusher); ok {
				flusher.Flush()
			}
			forceCloseConnection(writer)
			return
		}

		writer.Header().Set("Content-Length", strconv.Itoa(len(payload)))
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(payload)
	}))
	defer server.Close()

	downloader := NewHTTPDownloader(server.Client())
	downloader.maxAttempts = 2
	downloader.backoffBase = time.Millisecond

	path, err := downloader.Download(context.Background(), server.URL+"/xiadown.zip", nil)
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}

	downloaded, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read download failed: %v", err)
	}
	if !bytes.Equal(downloaded, payload) {
		t.Fatal("expected downloader to restart from the beginning when range is ignored")
	}

	mu.Lock()
	defer mu.Unlock()
	if requestCount != 2 {
		t.Fatalf("expected 2 requests, got %d", requestCount)
	}
	expectedRange := fmt.Sprintf("bytes=%d-", half)
	if len(rangeHeaders) < 2 || rangeHeaders[1] != expectedRange {
		t.Fatalf("expected second request to attempt ranged resume %q, got %#v", expectedRange, rangeHeaders)
	}
}

func forceCloseConnection(writer http.ResponseWriter) {
	hijacker, ok := writer.(http.Hijacker)
	if !ok {
		return
	}
	conn, _, err := hijacker.Hijack()
	if err != nil {
		return
	}
	_ = conn.Close()
}
