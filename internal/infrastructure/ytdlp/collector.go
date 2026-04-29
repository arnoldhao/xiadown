package ytdlp

import (
	"strings"
	"sync"
)

type outputCollector struct {
	maxBytes int
	buffer   strings.Builder
	mu       sync.Mutex
}

func newOutputCollector(maxBytes int) *outputCollector {
	return &outputCollector{maxBytes: maxBytes}
}

func (collector *outputCollector) Append(line string) {
	if collector == nil || collector.maxBytes <= 0 {
		return
	}
	collector.mu.Lock()
	defer collector.mu.Unlock()
	if collector.buffer.Len() >= collector.maxBytes {
		return
	}
	remaining := collector.maxBytes - collector.buffer.Len()
	if remaining <= 0 {
		return
	}
	if len(line) > remaining {
		line = line[:remaining]
	}
	collector.buffer.WriteString(line)
	if collector.buffer.Len() < collector.maxBytes {
		collector.buffer.WriteByte('\n')
	}
}

func (collector *outputCollector) String() string {
	if collector == nil {
		return ""
	}
	collector.mu.Lock()
	defer collector.mu.Unlock()
	return strings.TrimSpace(collector.buffer.String())
}
