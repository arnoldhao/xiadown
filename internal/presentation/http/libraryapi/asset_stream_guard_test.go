package libraryapi

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestAssetStreamGuardRejectsExcessConcurrency(t *testing.T) {
	t.Parallel()
	api := &BusinessAPI{
		config:      BusinessConfig{AssetWriteIdleTimeout: time.Second},
		streamSlots: make(chan struct{}, 1),
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	handler := api.withAssetStreamGuard(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		once.Do(func() { close(entered) })
		<-release
		w.WriteHeader(http.StatusNoContent)
	}))
	firstDone := make(chan struct{})
	go func() {
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/asset/one", nil))
		close(firstDone)
	}()
	<-entered
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/asset/two", nil))
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second asset status = %d, want %d", second.Code, http.StatusTooManyRequests)
	}
	close(release)
	<-firstDone
}

func TestAssetStreamGuardUsesSlidingWriteDeadline(t *testing.T) {
	t.Parallel()
	recorder := &deadlineResponseWriter{header: make(http.Header)}
	api := &BusinessAPI{
		config:      BusinessConfig{AssetWriteIdleTimeout: 100 * time.Millisecond},
		streamSlots: make(chan struct{}, 1),
	}
	handler := api.withAssetStreamGuard(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("first"))
		time.Sleep(5 * time.Millisecond)
		_, _ = w.Write([]byte("second"))
	}))
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/asset", nil))
	if len(recorder.deadlines) < 5 {
		t.Fatalf("write deadlines were not refreshed per chunk: %v", recorder.deadlines)
	}
	last := recorder.deadlines[len(recorder.deadlines)-1]
	if !last.IsZero() {
		t.Fatalf("final write deadline = %v, want cleared deadline", last)
	}
	var firstNonZero, lastNonZero time.Time
	for _, deadline := range recorder.deadlines {
		if deadline.IsZero() {
			continue
		}
		if firstNonZero.IsZero() {
			firstNonZero = deadline
		}
		lastNonZero = deadline
	}
	if !lastNonZero.After(firstNonZero) {
		t.Fatalf("deadline did not slide forward: %v", recorder.deadlines)
	}
}

type deadlineResponseWriter struct {
	header    http.Header
	deadlines []time.Time
}

func (writer *deadlineResponseWriter) Header() http.Header { return writer.header }
func (*deadlineResponseWriter) WriteHeader(int)            {}
func (*deadlineResponseWriter) Write(value []byte) (int, error) {
	return len(value), nil
}
func (writer *deadlineResponseWriter) SetWriteDeadline(deadline time.Time) error {
	writer.deadlines = append(writer.deadlines, deadline)
	return nil
}
