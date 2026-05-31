package ytdlp

import "testing"

func TestTimestampWriterFlushesCarriageReturnProgress(t *testing.T) {
	t.Parallel()

	var lines []string
	writer := newTimestampWriter("stderr", func(entry LogEntry) {
		lines = append(lines, entry.Line)
	})

	if _, err := writer.Write([]byte("frame=1 size=1kB time=00:00:01.00\rframe=2 size=2kB time=00:00:02.00\r")); err != nil {
		t.Fatalf("write progress: %v", err)
	}
	writer.Flush()

	if len(lines) != 2 {
		t.Fatalf("expected 2 progress lines, got %#v", lines)
	}
	if lines[0] != "frame=1 size=1kB time=00:00:01.00" || lines[1] != "frame=2 size=2kB time=00:00:02.00" {
		t.Fatalf("unexpected progress lines: %#v", lines)
	}
}
