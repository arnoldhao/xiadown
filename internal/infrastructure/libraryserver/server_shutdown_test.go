package libraryserver

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestServerShutdownForcesStalledStreamAfterBound(t *testing.T) {
	t.Parallel()
	requestStarted := make(chan struct{})
	release := make(chan struct{})
	server, err := New(Config{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			close(requestStarted)
			w.WriteHeader(http.StatusOK)
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			<-release
		}),
		ShutdownTimeout: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := server.Start(ctx); err != nil {
		t.Fatal(err)
	}
	clientDone := make(chan struct{})
	go func() {
		response, requestErr := http.Get("http://" + server.BackendAddress())
		if requestErr == nil {
			_ = response.Body.Close()
		}
		close(clientDone)
	}()
	<-requestStarted
	started := time.Now()
	_ = server.Shutdown(context.Background())
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("Shutdown remained blocked for %v", elapsed)
	}
	close(release)
	select {
	case <-clientDone:
	case <-time.After(time.Second):
		t.Fatal("client remained blocked after forced shutdown")
	}
}
