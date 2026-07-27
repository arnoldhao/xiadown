package libraryimport

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	libraryservice "xiadown/internal/application/library/service"
	importdomain "xiadown/internal/domain/libraryimport"
)

type inspectorStub struct{}

func (inspectorStub) InspectProfessionalImport(_ context.Context, path string) (libraryservice.ProfessionalImportProbe, error) {
	if filepath.Ext(path) == ".bin" {
		return libraryservice.ProfessionalImportProbe{HasAudio: true, Format: "flac"}, nil
	}
	return libraryservice.ProfessionalImportProbe{}, nil
}

func TestScannerClassifiesHashesAndAppliesPolicies(t *testing.T) {
	root := canonicalManagedTestRoot(t)
	writeTestFile(t, filepath.Join(root, "movie.mp4"), "video")
	writeTestFile(t, filepath.Join(root, "book.epub"), "book")
	writeTestFile(t, filepath.Join(root, "cover.png"), "image")
	writeTestFile(t, filepath.Join(root, "recording.bin"), "audio")
	writeTestFile(t, filepath.Join(root, ".hidden.mp3"), "hidden")
	writeTestFile(t, filepath.Join(root, "duplicate.mp4"), "video")
	if err := os.Symlink(filepath.Join(root, "movie.mp4"), filepath.Join(root, "linked.mp4")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	scanner := NewScanner(inspectorStub{})
	items, err := scanner.Scan(context.Background(), []string{root}, scanOptions{
		BatchID: "batch", HiddenPolicy: importdomain.HiddenExclude, SymlinkPolicy: importdomain.SymlinkSkip,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 6 {
		t.Fatalf("expected five visible files plus skipped symlink, got %d: %+v", len(items), items)
	}
	byName := make(map[string]importdomain.Candidate)
	for _, item := range items {
		byName[item.DisplayName] = item
		if item.Status != importdomain.CandidateSkipped && (item.HashAlgorithm != "sha256" || len(item.ContentHash) != 64) {
			t.Fatalf("missing strong digest for %+v", item)
		}
	}
	if _, exists := byName[".hidden.mp3"]; exists {
		t.Fatal("hidden file should be excluded")
	}
	if byName["movie.mp4"].Category != importdomain.CategoryVideo ||
		byName["book.epub"].Category != importdomain.CategoryBook ||
		byName["cover.png"].Category != importdomain.CategoryImage ||
		byName["recording.bin"].Category != importdomain.CategoryAudio {
		t.Fatalf("unexpected classifications: %+v", byName)
	}
	if byName["linked.mp4"].Status != importdomain.CandidateSkipped || byName["linked.mp4"].ErrorCode != "symlink_skipped" {
		t.Fatalf("expected explicit symlink policy result: %+v", byName["linked.mp4"])
	}
	if byName["duplicate.mp4"].Status != importdomain.CandidateDuplicate && byName["movie.mp4"].Status != importdomain.CandidateDuplicate {
		t.Fatal("same size and sha256 content should be deduplicated inside the batch")
	}
}

func TestCategoryFromExtensionRecognizesLocalAudioCatalogContainers(t *testing.T) {
	for _, extension := range []string{
		".aac", ".aif", ".aiff", ".alac", ".ape", ".caf", ".flac",
		".m4a", ".m4b", ".mp3", ".mpga", ".oga", ".ogg", ".opus",
		".wav", ".wave", ".weba", ".wma",
	} {
		t.Run(extension, func(t *testing.T) {
			if got := categoryFromExtension(extension); got != importdomain.CategoryAudio {
				t.Fatalf("categoryFromExtension(%q) = %q, want audio", extension, got)
			}
		})
	}
}

func TestScannerDistinguishesTypeScriptFromMPEGTransportStreams(t *testing.T) {
	root := canonicalManagedTestRoot(t)
	sourcePath := filepath.Join(root, "test.ts")
	writeTestFile(t, sourcePath, "export const test: string = 'source';\n")

	transportPath := filepath.Join(root, "recording.ts")
	transport := make([]byte, 188*4)
	for packet := 0; packet < 4; packet++ {
		position := packet * 188
		transport[position] = 0x47
		transport[position+1] = 0x40
		transport[position+3] = 0x10
	}
	if err := os.WriteFile(transportPath, transport, 0o600); err != nil {
		t.Fatal(err)
	}

	items, err := NewScanner(inspectorStub{}).Scan(
		context.Background(),
		[]string{sourcePath, transportPath},
		scanOptions{
			BatchID:       "ambiguous-ts",
			HiddenPolicy:  importdomain.HiddenExclude,
			SymlinkPolicy: importdomain.SymlinkSkip,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	byName := make(map[string]importdomain.Candidate, len(items))
	for _, item := range items {
		byName[item.DisplayName] = item
	}
	if got := byName["test.ts"].Category; got != importdomain.CategoryOther {
		t.Fatalf("TypeScript category = %q, want other", got)
	}
	if got := byName["recording.ts"].Category; got != importdomain.CategoryVideo {
		t.Fatalf("MPEG transport category = %q, want video", got)
	}
}

func TestManagedCopyIsIdempotentAndNeverOverwrites(t *testing.T) {
	ctx := context.Background()
	source := filepath.Join(t.TempDir(), "movie.mp4")
	writeTestFile(t, source, "original bytes")
	info, _ := os.Stat(source)
	digest, err := hashFileSHA256(ctx, source)
	if err != nil {
		t.Fatal(err)
	}
	candidate := importdomain.Candidate{
		SourcePath: source, DisplayName: "movie.mp4", Category: importdomain.CategoryVideo,
		SizeBytes: info.Size(), ContentHash: digest,
	}
	root := canonicalManagedTestRoot(t)
	first, err := copyIntoManagedRoot(ctx, root, candidate)
	if err != nil {
		t.Fatal(err)
	}
	second, err := copyIntoManagedRoot(ctx, root, candidate)
	if err != nil || second != first {
		t.Fatalf("copy retry should reuse checksum-identical destination: %q, %v", second, err)
	}
	if body, _ := os.ReadFile(first); string(body) != "original bytes" {
		t.Fatalf("unexpected managed body %q", body)
	}
	if err := os.WriteFile(first, []byte("tampered data!"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = copyIntoManagedRoot(ctx, root, candidate)
	if !errors.Is(err, importdomain.ErrDestinationExists) {
		t.Fatalf("expected no-overwrite error, got %v", err)
	}
	if body, _ := os.ReadFile(first); string(body) != "tampered data!" {
		t.Fatal("existing destination was overwritten")
	}
	matches, _ := filepath.Glob(filepath.Join(filepath.Dir(first), ".xiadown-import-*.stage"))
	if len(matches) != 0 {
		t.Fatalf("staging files leaked: %v", matches)
	}
}

func TestManagedCopyResumesACompleteInterruptedStage(t *testing.T) {
	ctx := context.Background()
	source := filepath.Join(t.TempDir(), "movie.mp4")
	writeTestFile(t, source, "recoverable bytes")
	info, _ := os.Stat(source)
	digest, _ := hashFileSHA256(ctx, source)
	candidate := importdomain.Candidate{
		ID: "candidate-recovery", SourcePath: source, DisplayName: "movie.mp4", Category: importdomain.CategoryVideo,
		SizeBytes: info.Size(), ContentHash: digest,
	}
	root := canonicalManagedTestRoot(t)
	directory := filepath.Join(root, "video", digest[:2], digest[2:4])
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	stage := filepath.Join(directory, ".xiadown-import-candidate-recovery.stage")
	writeTestFile(t, stage, "recoverable bytes")
	destination, err := copyIntoManagedRoot(ctx, root, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stage); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("interrupted stage still exists: %v", err)
	}
	if body, _ := os.ReadFile(destination); string(body) != "recoverable bytes" {
		t.Fatalf("unexpected recovered body %q", body)
	}
}

func TestVerifyCandidateSourceRejectsContentChangeWithSameSize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.bin")
	writeTestFile(t, path, "aaaa")
	digest, _ := hashFileSHA256(context.Background(), path)
	candidate := importdomain.Candidate{SourcePath: path, SizeBytes: 4, ContentHash: digest}
	writeTestFile(t, path, "bbbb")
	if err := verifyCandidateSource(context.Background(), candidate); !errors.Is(err, importdomain.ErrSourceChanged) {
		t.Fatalf("expected source changed error, got %v", err)
	}
}

func TestScannerPersistsUnavailableSelectionAsPerFileResult(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.mp4")
	items, err := NewScanner(nil).Scan(context.Background(), []string{missing}, scanOptions{
		BatchID: "batch-missing", HiddenPolicy: importdomain.HiddenExclude, SymlinkPolicy: importdomain.SymlinkSkip,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Status != importdomain.CandidateSkipped || items[0].ErrorCode != "source_unavailable" || items[0].ErrorMessage == "" {
		t.Fatalf("missing source was not retained as a per-file result: %+v", items)
	}
}

func writeTestFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func canonicalManagedTestRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}
