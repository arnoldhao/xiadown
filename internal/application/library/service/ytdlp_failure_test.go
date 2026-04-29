package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"xiadown/internal/domain/dependencies"
)

type ytdlpFailureToolResolverStub struct {
	execPath string
}

func (stub *ytdlpFailureToolResolverStub) ResolveExecPath(_ context.Context, _ dependencies.DependencyName) (string, error) {
	if stub.execPath == "" {
		return "", fmt.Errorf("missing exec path")
	}
	return stub.execPath, nil
}

func (stub *ytdlpFailureToolResolverStub) ResolveDependencyDirectory(_ context.Context, _ dependencies.DependencyName) (string, error) {
	if stub.execPath == "" {
		return "", fmt.Errorf("missing exec path")
	}
	return filepath.Dir(stub.execPath), nil
}

func (stub *ytdlpFailureToolResolverStub) DependencyReadiness(_ context.Context, _ dependencies.DependencyName) (bool, string, error) {
	if stub.execPath == "" {
		return false, "missing_exec_path", nil
	}
	return true, "", nil
}

func TestCheckYTDLPVersionAcceptsVersionOutputEvenWhenProcessReturnsError(t *testing.T) {
	execPath := writeVersionScript(t, "2026.03.17", true)
	service := &LibraryService{
		tools: &ytdlpFailureToolResolverStub{execPath: execPath},
	}

	status, message := service.checkYTDLPVersion(context.Background())
	if status != ytdlpCheckStatusOK {
		t.Fatalf("expected status ok, got %q (%s)", status, message)
	}
	if message != "2026.03.17" {
		t.Fatalf("expected version 2026.03.17, got %q", message)
	}
}

func writeVersionScript(t *testing.T, version string, fail bool) string {
	t.Helper()

	tempDir := t.TempDir()
	if runtime.GOOS == "windows" {
		path := filepath.Join(tempDir, "yt-dlp.cmd")
		content := "@echo off\r\necho " + version + "\r\n"
		if fail {
			content += "exit /b 1\r\n"
		} else {
			content += "exit /b 0\r\n"
		}
		if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
			t.Fatalf("write cmd script: %v", err)
		}
		return path
	}

	path := filepath.Join(tempDir, "yt-dlp")
	content := "#!/bin/sh\n" + "echo \"" + version + "\"\n"
	if fail {
		content += "exit 1\n"
	} else {
		content += "exit 0\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write shell script: %v", err)
	}
	return path
}
