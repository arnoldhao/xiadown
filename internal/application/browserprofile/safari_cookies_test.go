package browserprofile

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type safariReadRange struct {
	start int64
	end   int64
}

type trackingSafariReaderAt struct {
	reader *bytes.Reader
	reads  []safariReadRange
}

func (reader *trackingSafariReaderAt) ReadAt(payload []byte, offset int64) (int, error) {
	reader.reads = append(reader.reads, safariReadRange{start: offset, end: offset + int64(len(payload))})
	return reader.reader.ReadAt(payload, offset)
}

func safariCookieFixture(t *testing.T) []byte {
	t.Helper()
	strings := [][]byte{
		[]byte(".example.com\x00"),
		[]byte("session\x00"),
		[]byte("/account\x00"),
		[]byte("secret-value\x00"),
	}
	cookieSize := safariCookieHeaderLen
	for _, value := range strings {
		cookieSize += len(value)
	}
	cookie := make([]byte, cookieSize)
	binary.LittleEndian.PutUint32(cookie[0:4], uint32(cookieSize))
	binary.LittleEndian.PutUint32(cookie[8:12], 1|4)
	offset := safariCookieHeaderLen
	for index, fieldOffset := range []int{16, 20, 24, 28} {
		binary.LittleEndian.PutUint32(cookie[fieldOffset:fieldOffset+4], uint32(offset))
		copy(cookie[offset:], strings[index])
		offset += len(strings[index])
	}
	binary.LittleEndian.PutUint64(cookie[40:48], math.Float64bits(1_000_000))
	binary.LittleEndian.PutUint64(cookie[48:56], math.Float64bits(900_000))

	page := make([]byte, 16+len(cookie))
	copy(page[:4], []byte{0, 0, 1, 0})
	binary.LittleEndian.PutUint32(page[4:8], 1)
	binary.LittleEndian.PutUint32(page[8:12], 16)
	copy(page[16:], cookie)

	archive := make([]byte, 12+len(page)+8)
	copy(archive[:4], "cook")
	binary.BigEndian.PutUint32(archive[4:8], 1)
	binary.BigEndian.PutUint32(archive[8:12], uint32(len(page)))
	copy(archive[12:], page)
	return archive
}

func safariCookieRecordFixture(domain string, name string, path string, value string) []byte {
	fields := [][]byte{
		[]byte(domain + "\x00"),
		[]byte(name + "\x00"),
		[]byte(path + "\x00"),
		[]byte(value + "\x00"),
	}
	size := safariCookieHeaderLen
	for _, field := range fields {
		size += len(field)
	}
	record := make([]byte, size)
	binary.LittleEndian.PutUint32(record[:4], uint32(size))
	offset := safariCookieHeaderLen
	for index, at := range []int{16, 20, 24, 28} {
		binary.LittleEndian.PutUint32(record[at:at+4], uint32(offset))
		copy(record[offset:], fields[index])
		offset += len(fields[index])
	}
	return record
}

func safariCookieArchiveFixture(records ...[]byte) []byte {
	offsetTableEnd := 8 + len(records)*4
	cursor := offsetTableEnd + 4
	pageSize := cursor
	for _, record := range records {
		pageSize += len(record)
	}
	page := make([]byte, pageSize)
	copy(page[:4], []byte{0, 0, 1, 0})
	binary.LittleEndian.PutUint32(page[4:8], uint32(len(records)))
	for index, record := range records {
		binary.LittleEndian.PutUint32(page[8+index*4:12+index*4], uint32(cursor))
		copy(page[cursor:], record)
		cursor += len(record)
	}
	archive := make([]byte, 12+len(page)+8)
	copy(archive[:4], "cook")
	binary.BigEndian.PutUint32(archive[4:8], 1)
	binary.BigEndian.PutUint32(archive[8:12], uint32(len(page)))
	copy(archive[12:], page)
	return archive
}

func TestParseSafariCookiesReadsBoundedArchive(t *testing.T) {
	records, err := parseSafariCookies(safariCookieFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one Safari cookie, got %#v", records)
	}
	record := records[0]
	if record.Domain != ".example.com" || record.Name != "session" || record.Path != "/account" || record.Value != "secret-value" {
		t.Fatalf("unexpected Safari cookie: %#v", record)
	}
	if !record.Secure || !record.HttpOnly {
		t.Fatalf("expected Safari security flags: %#v", record)
	}
	if record.Expires != safariEpochOffset+1_000_000 {
		t.Fatalf("unexpected Safari expiration: %d", record.Expires)
	}
}

func TestParseSafariCookiesRejectsMalformedBounds(t *testing.T) {
	tests := map[string]func([]byte){
		"page size": func(payload []byte) {
			binary.BigEndian.PutUint32(payload[8:12], uint32(len(payload)))
		},
		"cookie count": func(payload []byte) {
			binary.LittleEndian.PutUint32(payload[16:20], safariMaxCookies+1)
		},
		"cookie offset": func(payload []byte) {
			binary.LittleEndian.PutUint32(payload[20:24], uint32(len(payload)))
		},
		"string offset": func(payload []byte) {
			cookieStart := 12 + 16
			binary.LittleEndian.PutUint32(payload[cookieStart+16:cookieStart+20], uint32(len(payload)))
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			payload := safariCookieFixture(t)
			mutate(payload)
			if _, err := parseSafariCookies(payload); err == nil {
				t.Fatal("expected malformed Safari archive to be rejected")
			}
		})
	}
}

func TestSnapshotSafariCookiesDoesNotMutateSource(t *testing.T) {
	source := filepath.Join(t.TempDir(), "Cookies.binarycookies")
	original := safariCookieFixture(t)
	if err := os.WriteFile(source, original, 0o600); err != nil {
		t.Fatal(err)
	}
	profile := Profile{BrowserID: "safari", Available: true, snapshotFile: source}
	if _, err := snapshotSafariCookies(profile, []string{"example.com"}); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(original) {
		t.Fatal("Safari source cookie archive was mutated")
	}
}

func TestSnapshotSafariCookiesReadsOnlyMemoryAndRetriesAChangedSource(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "Cookies.binarycookies")
	original := safariCookieFixture(t)
	if err := os.WriteFile(source, original, 0o600); err != nil {
		t.Fatal(err)
	}
	controlledRoot := filepath.Join(root, "snapshots-must-not-be-created")
	previousRoot := snapshotRoot
	previousHook := safariSnapshotReadHook
	snapshotRoot = func() string { return controlledRoot }
	attempts := 0
	safariSnapshotReadHook = func() {
		attempts++
		if attempts == 1 {
			changed := time.Now().Add(2 * time.Second)
			if err := os.Chtimes(source, changed, changed); err != nil {
				t.Errorf("touch Safari fixture: %v", err)
			}
		}
	}
	t.Cleanup(func() {
		snapshotRoot = previousRoot
		safariSnapshotReadHook = previousHook
	})
	profile := Profile{BrowserID: "safari", Available: true, snapshotFile: source}
	records, err := snapshotSafariCookies(profile, []string{"example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || attempts != 2 {
		t.Fatalf("expected one retry and one parsed cookie, attempts=%d records=%#v", attempts, records)
	}
	if _, err := os.Lstat(controlledRoot); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Safari plaintext snapshot was written to disk: %v", err)
	}
}

func TestSnapshotSafariCookiesRejectsOversizedAndSymlinkSources(t *testing.T) {
	root := t.TempDir()
	oversized := filepath.Join(root, "oversized.binarycookies")
	file, err := os.OpenFile(oversized, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(safariCookieCopyLimit + 1); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := snapshotSafariCookies(Profile{BrowserID: "safari", Available: true, snapshotFile: oversized}, []string{"example.com"}); err == nil {
		t.Fatal("expected oversized Safari archive to be rejected")
	}
	target := filepath.Join(root, "target.binarycookies")
	if err := os.WriteFile(target, safariCookieFixture(t), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link.binarycookies")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	if _, err := snapshotSafariCookies(Profile{BrowserID: "safari", Available: true, snapshotFile: link}, []string{"example.com"}); err == nil {
		t.Fatal("expected Safari archive symlink to be rejected")
	}
}

func TestSafariReaderAtSkipsDisallowedCookieValues(t *testing.T) {
	denied := safariCookieRecordFixture(".evil.test", "denied", "/", "DENIED-VALUE-MUST-NOT-BE-READ")
	actualDeniedValueOffset := int64(binary.LittleEndian.Uint32(denied[28:32]))
	// A malformed value offset proves the disallowed record is not fully
	// parsed. Its header and domain remain valid for the allowlist decision.
	binary.LittleEndian.PutUint32(denied[28:32], uint32(len(denied)+32))
	allowed := safariCookieRecordFixture(".login.example.com", "session", "/", "allowed-secret")
	archive := safariCookieArchiveFixture(denied, allowed)
	if _, err := parseSafariCookies(archive); err == nil {
		t.Fatal("full archive parser unexpectedly accepted malformed denied value")
	}

	reader := &trackingSafariReaderAt{reader: bytes.NewReader(archive)}
	budget := int64(safariScanReadLimit)
	records, err := scanSafariCookieReader(reader, int64(len(archive)), []string{"example.com"}, &budget)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Domain != ".login.example.com" || records[0].Value != "allowed-secret" {
		t.Fatalf("Safari ReaderAt did not retain only the allowed cookie: %#v", records)
	}
	pageStart := int64(12)
	deniedRecordOffset := int64(binary.LittleEndian.Uint32(archive[pageStart+8 : pageStart+12]))
	deniedValueStart := pageStart + deniedRecordOffset + actualDeniedValueOffset
	for _, read := range reader.reads {
		if read.end > deniedValueStart {
			// Reads for later records may lie after this point. Only reject a read
			// beginning inside the denied record beyond its declared value start.
			deniedRecordEnd := pageStart + deniedRecordOffset + int64(len(denied))
			if read.start < deniedRecordEnd && read.end > deniedValueStart {
				t.Fatalf("ReaderAt touched a denied cookie value range: %+v", read)
			}
		}
	}

	source := filepath.Join(t.TempDir(), "Cookies.binarycookies")
	if err := os.WriteFile(source, archive, 0o600); err != nil {
		t.Fatal(err)
	}
	profile := Profile{BrowserID: "safari", Available: true, snapshotFile: source}
	records, err = snapshotSafariCookies(profile, []string{"example.com"})
	if err != nil || len(records) != 1 || records[0].Value != "allowed-secret" {
		t.Fatalf("stable Safari snapshot filter failed: records=%#v err=%v", records, err)
	}
}

func TestParseSafariCookiesExternalFixture(t *testing.T) {
	path := os.Getenv("XIADOWN_SAFARI_COOKIE_FIXTURE")
	if path == "" {
		t.Skip("set XIADOWN_SAFARI_COOKIE_FIXTURE to validate a captured Safari archive")
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	records, err := parseSafariCookies(payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) == 0 {
		t.Fatal("expected Safari fixture cookies")
	}
}

func TestProfileStateForPermissionIsNotNoData(t *testing.T) {
	if got := profileStateForError(os.ErrPermission); got != ProfileStatePermissionRequired {
		t.Fatalf("permission denial must be explicit, got %q", got)
	}
}
