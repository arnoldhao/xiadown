package locallyricsreader

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/dhowden/tag"

	"xiadown/internal/application/locallyrics"
)

func TestReadEmbeddedLyricsID3USLT(t *testing.T) {
	lyrics := "[00:01.00]First line\n[00:03.25]Second line"
	path := writeID3v23USLT(t, lyrics)
	reader := New()

	content, err := reader.ReadEmbeddedLyrics(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadEmbeddedLyrics() error = %v", err)
	}
	if !bytes.Equal(content.Bytes, []byte(lyrics)) {
		t.Fatalf("content.Bytes = %q, want %q", content.Bytes, lyrics)
	}
	if content.Name != "embedded-lyrics.txt" {
		t.Fatalf("content.Name = %q", content.Name)
	}
	if content.AttributionLabel != "Embedded MP3 lyrics (ID3v2.3)" {
		t.Fatalf("content.AttributionLabel = %q", content.AttributionLabel)
	}

	result, err := locallyrics.ParseEmbedded(context.Background(), reader, path, locallyrics.Options{})
	if err != nil {
		t.Fatalf("ParseEmbedded() error = %v", err)
	}
	if result.Format != locallyrics.FormatLRC || len(result.Lines) != 2 {
		t.Fatalf("parsed result = format %q, %d lines", result.Format, len(result.Lines))
	}
	if result.Attribution.Kind != locallyrics.SourceEmbedded {
		t.Fatalf("attribution kind = %q", result.Attribution.Kind)
	}
}

func TestReadEmbeddedLyricsRejectsBlankAndOversize(t *testing.T) {
	t.Run("blank", func(t *testing.T) {
		path := writeID3v23USLT(t, " \r\n\t")
		_, err := New().ReadEmbeddedLyrics(context.Background(), path)
		if !errors.Is(err, ErrNoEmbeddedLyrics) {
			t.Fatalf("error = %v, want ErrNoEmbeddedLyrics", err)
		}
	})

	t.Run("oversize", func(t *testing.T) {
		path := writeID3v23USLT(t, strings.Repeat("x", 9))
		reader := NewWithOptions(Options{MaxLyricsBytes: 8})
		_, err := reader.ReadEmbeddedLyrics(context.Background(), path)
		if !errors.Is(err, ErrLyricsTooLarge) {
			t.Fatalf("error = %v, want ErrLyricsTooLarge", err)
		}
	})
}

func TestReadEmbeddedLyricsHonorsCanceledContext(t *testing.T) {
	path := writeID3v23USLT(t, "lyrics")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := New().ReadEmbeddedLyrics(ctx, path)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestReadEmbeddedLyricsRejectsNilContext(t *testing.T) {
	path := writeID3v23USLT(t, "lyrics")
	_, err := New().ReadEmbeddedLyrics(nil, path)
	if !errors.Is(err, ErrInvalidContext) {
		t.Fatalf("error = %v, want ErrInvalidContext", err)
	}
}

func TestReaderZeroValueUsesDefaultLimit(t *testing.T) {
	path := writeID3v23USLT(t, "lyrics")
	var reader Reader
	content, err := reader.ReadEmbeddedLyrics(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadEmbeddedLyrics() error = %v", err)
	}
	if string(content.Bytes) != "lyrics" {
		t.Fatalf("content.Bytes = %q", content.Bytes)
	}
}

func TestReadEmbeddedLyricsRejectsNonRegularAndSymlink(t *testing.T) {
	t.Run("directory", func(t *testing.T) {
		_, err := New().ReadEmbeddedLyrics(context.Background(), t.TempDir())
		if !errors.Is(err, ErrUnsafeMediaFile) {
			t.Fatalf("error = %v, want ErrUnsafeMediaFile", err)
		}
	})

	t.Run("symlink", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("symlink creation commonly requires additional privileges on Windows")
		}
		target := writeID3v23USLT(t, "lyrics")
		link := filepath.Join(t.TempDir(), "linked.mp3")
		if err := os.Symlink(target, link); err != nil {
			t.Fatalf("Symlink() error = %v", err)
		}
		_, err := New().ReadEmbeddedLyrics(context.Background(), link)
		if !errors.Is(err, ErrUnsafeMediaFile) {
			t.Fatalf("error = %v, want ErrUnsafeMediaFile", err)
		}
	})
}

func TestReadEmbeddedLyricsRejectsFileChangedDuringMetadataRead(t *testing.T) {
	path := writeID3v23USLT(t, "original")
	reader := New()
	readMetadata := reader.readMetadata
	reader.readMetadata = func(stream io.ReadSeeker) (tag.Metadata, error) {
		if err := os.WriteFile(path, buildID3v23USLT("replacement with a different size"), 0o600); err != nil {
			t.Fatalf("mutate media fixture: %v", err)
		}
		return readMetadata(stream)
	}

	_, err := reader.ReadEmbeddedLyrics(context.Background(), path)
	if !errors.Is(err, ErrUnsafeMediaFile) {
		t.Fatalf("error = %v, want ErrUnsafeMediaFile", err)
	}
}

func TestLyricsFromRawUsesOnlyKnownTextShapes(t *testing.T) {
	tests := []struct {
		name string
		raw  map[string]interface{}
		want string
	}{
		{name: "vorbis unsynced", raw: map[string]interface{}{"unsyncedlyrics": "line"}, want: "line"},
		{name: "byte lyrics", raw: map[string]interface{}{"LYRICS": []byte("bytes")}, want: "bytes"},
		{name: "ID3 v2.2 ULT", raw: map[string]interface{}{"ULT": &tag.Comm{Text: "old id3"}}, want: "old id3"},
		{name: "described TXXX", raw: map[string]interface{}{"TXXX": &tag.Comm{Description: "UNSYNCED_LYRICS", Text: "user text"}}, want: "user text"},
		{name: "unknown object", raw: map[string]interface{}{"LYRICS": struct{ Text string }{Text: "unsafe"}}, want: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := lyricsFromRaw(test.raw); got != test.want {
				t.Fatalf("lyricsFromRaw() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestReadEmbeddedLyricsWithoutTags(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plain.bin")
	if err := os.WriteFile(path, bytes.Repeat([]byte{'x'}, 256), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	_, err := New().ReadEmbeddedLyrics(context.Background(), path)
	if !errors.Is(err, ErrNoEmbeddedLyrics) {
		t.Fatalf("error = %v, want ErrNoEmbeddedLyrics", err)
	}
}

func writeID3v23USLT(t *testing.T, lyrics string) string {
	t.Helper()
	data := buildID3v23USLT(lyrics)

	path := filepath.Join(t.TempDir(), "track.mp3")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func buildID3v23USLT(lyrics string) []byte {
	body := append([]byte{0, 'e', 'n', 'g', 0}, []byte(lyrics)...)
	frame := make([]byte, 10+len(body))
	copy(frame[:4], "USLT")
	binary.BigEndian.PutUint32(frame[4:8], uint32(len(body)))
	copy(frame[10:], body)

	header := []byte{'I', 'D', '3', 3, 0, 0, 0, 0, 0, 0}
	putSyncSafe(header[6:10], len(frame))
	data := append(header, frame...)
	data = append(data, 0xff, 0xfb, 0x90, 0x64) // harmless fake audio bytes
	return data
}

func putSyncSafe(destination []byte, value int) {
	destination[0] = byte((value >> 21) & 0x7f)
	destination[1] = byte((value >> 14) & 0x7f)
	destination[2] = byte((value >> 7) & 0x7f)
	destination[3] = byte(value & 0x7f)
}
