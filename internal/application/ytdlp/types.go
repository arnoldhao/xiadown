package ytdlp

import (
	"context"
	"os/exec"
	"time"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/dependencies"
)

type ToolResolver interface {
	ResolveExecPath(ctx context.Context, name dependencies.DependencyName) (string, error)
}

type Origin struct {
	Source string `json:"source,omitempty"`
	RunID  string `json:"runId,omitempty"`
	Caller string `json:"caller,omitempty"`
}

type LogSnapshot struct {
	Path      string `json:"path,omitempty"`
	SizeBytes int64  `json:"sizeBytes,omitempty"`
	LineCount int    `json:"lineCount,omitempty"`
}

type Command struct {
	Cmd           *exec.Cmd
	Args          []string
	SanitizedArgs []string
	PrintFilePath string
	Ctx           context.Context
	Cancel        context.CancelFunc
	Cleanup       func()
}

type CommandOptions struct {
	ExecPath          string
	Tools             ToolResolver
	Request           dto.CreateYTDLPJobRequest
	OutputTemplate    string
	SubtitleTemplate  string
	ThumbnailTemplate string
	CookiesPath       string
	ProxyURL          string
	Timeout           time.Duration
}

type InfoOptions struct {
	ExecPath    string
	Tools       ToolResolver
	URL         string
	CookiesPath string
	ProxyURL    string
}
