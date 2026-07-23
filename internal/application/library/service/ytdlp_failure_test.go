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
	t.Setenv("YTDLP_NO_PLUGINS", "0")
	t.Setenv("PYTHONNOUSERSITE", "0")
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

func TestResolveYTDLPErrorCodeClassifiesTooManyOpenFiles(t *testing.T) {
	t.Parallel()

	detail := "ERROR: unable to open for writing: [Errno 24] Too many open files: '/tmp/video.mp4.part-Frag594.part'"
	if got := resolveYTDLPErrorCode(detail, nil); got != ytdlpErrorCodeResourceLimit {
		t.Fatalf("expected resource limit error code, got %q", got)
	}
}

func writeVersionScript(t *testing.T, version string, fail bool) string {
	t.Helper()

	tempDir := t.TempDir()
	if runtime.GOOS == "windows" {
		path := filepath.Join(tempDir, "yt-dlp.cmd")
		content := "@echo off\r\n" +
			"if not \"%~1\"==\"--ignore-config\" exit /b 2\r\n" +
			"if not \"%~2\"==\"--no-config-locations\" exit /b 2\r\n" +
			"if not \"%~3\"==\"--no-plugin-dirs\" exit /b 2\r\n" +
			"if not \"%~4\"==\"--no-exec\" exit /b 2\r\n" +
			"if not \"%~5\"==\"--version\" exit /b 2\r\n" +
			"if not \"%YTDLP_NO_PLUGINS%\"==\"1\" exit /b 3\r\n" +
			"if not \"%PYTHONNOUSERSITE%\"==\"1\" exit /b 3\r\n" +
			"echo " + version + "\r\n"
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
	content := "#!/bin/sh\n" +
		"[ \"$1\" = \"--ignore-config\" ] || exit 2\n" +
		"[ \"$2\" = \"--no-config-locations\" ] || exit 2\n" +
		"[ \"$3\" = \"--no-plugin-dirs\" ] || exit 2\n" +
		"[ \"$4\" = \"--no-exec\" ] || exit 2\n" +
		"[ \"$5\" = \"--version\" ] || exit 2\n" +
		"[ \"$YTDLP_NO_PLUGINS\" = \"1\" ] || exit 3\n" +
		"[ \"$PYTHONNOUSERSITE\" = \"1\" ] || exit 3\n" +
		"echo \"" + version + "\"\n"
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
