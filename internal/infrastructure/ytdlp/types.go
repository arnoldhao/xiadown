package ytdlp

import (
	"os/exec"
	"time"
)

type LogEntry struct {
	Timestamp time.Time
	Pipe      string
	Line      string
}

type ProgressHandler interface {
	HandleLine(string)
}

type RunOptions struct {
	Command        *exec.Cmd
	Progress       ProgressHandler
	LogLine        func(pipe string, line string)
	OutputPath     func(path string)
	PrintFilePath  string
	ProgressPrefix string
	OnStarted      func(cmd *exec.Cmd) func()
}

type RunResult struct {
	Logs             []LogEntry
	Metadata         []map[string]any
	OutputPaths      []string
	AfterMovePaths   []string
	SubtitleLogPaths []string
	Output           string
	Warnings         string
	Stderr           string
}
