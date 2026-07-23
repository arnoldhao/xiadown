package locallyrics

import (
	"context"
	"errors"
	"testing"
)

type embeddedReaderStub struct {
	content Content
	err     error
	calls   int
}

func (reader *embeddedReaderStub) ReadEmbeddedLyrics(_ context.Context, _ string) (Content, error) {
	reader.calls++
	return reader.content, reader.err
}

func TestEmbeddedEntryPointPreservesAttribution(t *testing.T) {
	reader := &embeddedReaderStub{content: Content{
		Name:  "USLT",
		Bytes: []byte("[00:01.00]Embedded"),
	}}
	embedded, err := ParseEmbedded(context.Background(), reader, "/music/track.mp3", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if reader.calls != 1 || embedded.Attribution.Kind != SourceEmbedded || embedded.Attribution.Label != "Embedded lyric" {
		t.Fatalf("unexpected embedded result: calls=%d result=%#v", reader.calls, embedded)
	}
}

func TestParseEmbeddedHonorsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	reader := &embeddedReaderStub{}
	if _, err := ParseEmbedded(ctx, reader, "/music/track.mp3", Options{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
	if reader.calls != 0 {
		t.Fatalf("reader should not run after cancellation")
	}
}
