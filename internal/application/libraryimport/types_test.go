package libraryimport

import (
	"testing"

	"xiadown/internal/domain/library"
	importdomain "xiadown/internal/domain/libraryimport"
)

func TestFileKindForTreatsLogAsDocument(t *testing.T) {
	t.Parallel()
	if kind := fileKindFor(importdomain.Candidate{Extension: ".log"}); kind != string(library.FileKindDocument) {
		t.Fatalf("LOG kind = %q, want document", kind)
	}
}
