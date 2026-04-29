package ytdlp

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
	"unicode"
)

type timestampWriter struct {
	pipe    string
	handler func(LogEntry)

	buf            bytes.Buffer
	lastWriteStart time.Time
}

func newTimestampWriter(pipe string, handler func(LogEntry)) *timestampWriter {
	return &timestampWriter{pipe: pipe, handler: handler}
}

func (w *timestampWriter) Write(p []byte) (n int, err error) {
	if w.lastWriteStart.IsZero() {
		w.lastWriteStart = time.Now()
	}

	if i := bytes.IndexByte(p, '\n'); i >= 0 {
		w.buf.Write(p[:i+1])
		w.flush()
		_, err = w.Write(p[i+1:])
		return len(p), err
	}

	return w.buf.Write(p)
}

func (w *timestampWriter) flush() {
	if w.buf.Len() == 0 {
		return
	}
	line := bytes.TrimRightFunc(w.buf.Bytes(), unicode.IsSpace)
	if len(line) > 0 && w.handler != nil {
		w.handler(LogEntry{
			Timestamp: w.lastWriteStart,
			Pipe:      w.pipe,
			Line:      string(line),
		})
	}
	w.lastWriteStart = time.Time{}
	w.buf.Reset()
}

func (w *timestampWriter) Flush() {
	w.flush()
}

func SortLogs(entries []LogEntry) []LogEntry {
	if len(entries) == 0 {
		return entries
	}
	sorted := append([]LogEntry{}, entries...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.Before(sorted[j].Timestamp)
	})
	return sorted
}

func WriteLogFile(path string, entries []LogEntry) (int64, error) {
	if strings.TrimSpace(path) == "" {
		return 0, fmt.Errorf("log path is empty")
	}
	if len(entries) == 0 {
		return 0, nil
	}
	file, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	var total int64
	for i, entry := range entries {
		line := fmt.Sprintf("[%s::%s] %s", entry.Timestamp.Format(time.DateTime), entry.Pipe, entry.Line)
		if i < len(entries)-1 {
			line += "\n"
		}
		n, err := writer.WriteString(line)
		total += int64(n)
		if err != nil {
			_ = writer.Flush()
			return total, err
		}
	}
	if err := writer.Flush(); err != nil {
		return total, err
	}
	return total, nil
}
