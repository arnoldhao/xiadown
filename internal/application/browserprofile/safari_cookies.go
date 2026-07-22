package browserprofile

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"unicode/utf8"

	appcookies "xiadown/internal/application/cookies"
)

const (
	safariCookieCopyLimit = 64 << 20
	safariMaxPages        = 65536
	safariMaxCookies      = 100_000
	safariMaxCookieSize   = 1 << 20
	safariEpochOffset     = int64(978_307_200)
	safariMaxUnixTime     = int64(253_402_300_799)
	safariCookieHeaderLen = 56
	safariSnapshotRetries = 3
	safariDomainReadLimit = 1024
	safariScanReadLimit   = 128 << 20
)

var (
	errSafariCookieSourceChanged = errors.New("Safari cookie source changed while reading")
	safariSnapshotReadHook       = func() {}
)

func snapshotSafariCookies(profile Profile, domains []string) ([]appcookies.Record, error) {
	source := profile.snapshotFile
	if source == "" {
		return nil, fmt.Errorf("Safari profile cookie source is unavailable")
	}
	allowedDomains, err := normalizeSnapshotDomains(domains)
	if err != nil {
		return nil, err
	}
	records, err := readStableSafariCookies(source, allowedDomains)
	if err != nil {
		return nil, fmt.Errorf("read Safari cookie archive: %w", err)
	}
	return appcookies.FilterByDomains(records, allowedDomains), nil
}

// readStableSafariCookies walks Safari's page table through ReaderAt. For an
// unrelated cookie it reads only the record header and bounded domain field;
// the complete record (including value) is read only after the domain matches.
// Identity, size, and mtime are checked around the scan, and a replaced archive
// is retried a small, fixed number of times.
func readStableSafariCookies(source string, allowedDomains []string) ([]appcookies.Record, error) {
	for attempt := 0; attempt < safariSnapshotRetries; attempt++ {
		records, err := scanSafariCookieArchiveAttempt(source, allowedDomains)
		if err == nil {
			return records, nil
		}
		if !errors.Is(err, errSafariCookieSourceChanged) {
			return nil, err
		}
	}
	return nil, errSafariCookieSourceChanged
}

func scanSafariCookieArchiveAttempt(source string, allowedDomains []string) ([]appcookies.Record, error) {
	before, err := os.Lstat(source)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 || before.Size() < 0 || before.Size() > safariCookieCopyLimit {
		return nil, fmt.Errorf("Safari cookie source is not a valid regular file")
	}
	file, err := os.Open(source)
	if err != nil {
		return nil, err
	}
	opened, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		_ = file.Close()
		return nil, errSafariCookieSourceChanged
	}
	budget := int64(safariScanReadLimit)
	records, scanErr := scanSafariCookieReader(file, before.Size(), allowedDomains, &budget)
	safariSnapshotReadHook()
	openedAfter, openedAfterErr := file.Stat()
	after, err := os.Lstat(source)
	closeErr := file.Close()
	if closeErr != nil {
		return nil, closeErr
	}
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, errSafariCookieSourceChanged
		}
		return nil, err
	}
	if openedAfterErr != nil || !openedAfter.Mode().IsRegular() || !os.SameFile(before, openedAfter) ||
		!after.Mode().IsRegular() || after.Mode()&os.ModeSymlink != 0 || !os.SameFile(before, after) ||
		before.Size() != openedAfter.Size() || before.Size() != after.Size() ||
		!before.ModTime().Equal(openedAfter.ModTime()) || !before.ModTime().Equal(after.ModTime()) {
		return nil, errSafariCookieSourceChanged
	}
	if scanErr != nil {
		return nil, scanErr
	}
	return records, nil
}

func scanSafariCookieReader(reader io.ReaderAt, fileSize int64, allowedDomains []string, budget *int64) ([]appcookies.Record, error) {
	header := make([]byte, 8)
	if err := readSafariAt(reader, fileSize, 0, header, budget); err != nil {
		return nil, err
	}
	if string(header[:4]) != "cook" {
		return nil, fmt.Errorf("invalid Safari cookie archive header")
	}
	pageCount := uint64(binary.BigEndian.Uint32(header[4:8]))
	if pageCount == 0 || pageCount > safariMaxPages {
		return nil, fmt.Errorf("invalid Safari cookie page count")
	}
	tableSize := int64(pageCount * 4)
	pageSizes := make([]byte, tableSize)
	if err := readSafariAt(reader, fileSize, 8, pageSizes, budget); err != nil {
		return nil, fmt.Errorf("read Safari cookie page table: %w", err)
	}
	pageOffset := int64(8) + tableSize
	result := make([]appcookies.Record, 0)
	totalCookies := uint64(0)
	for pageIndex := uint64(0); pageIndex < pageCount; pageIndex++ {
		sizeAt := pageIndex * 4
		pageSize := int64(binary.BigEndian.Uint32(pageSizes[sizeAt : sizeAt+4]))
		if pageSize < 8 || pageOffset < 0 || pageOffset > fileSize || pageSize > fileSize-pageOffset {
			return nil, fmt.Errorf("invalid Safari cookie page size")
		}
		pageRecords, err := scanSafariCookiePage(reader, fileSize, pageOffset, pageSize, allowedDomains, &totalCookies, budget)
		if err != nil {
			return nil, fmt.Errorf("parse Safari cookie page %d: %w", pageIndex, err)
		}
		result = append(result, pageRecords...)
		pageOffset += pageSize
	}
	if fileSize-pageOffset < 8 {
		return nil, fmt.Errorf("invalid Safari cookie archive trailer")
	}
	return result, nil
}

func scanSafariCookiePage(
	reader io.ReaderAt,
	fileSize int64,
	pageOffset int64,
	pageSize int64,
	allowedDomains []string,
	totalCookies *uint64,
	budget *int64,
) ([]appcookies.Record, error) {
	header := make([]byte, 8)
	if err := readSafariAt(reader, fileSize, pageOffset, header, budget); err != nil {
		return nil, err
	}
	if string(header[:4]) != string([]byte{0, 0, 1, 0}) {
		return nil, fmt.Errorf("invalid page header")
	}
	count := uint64(binary.LittleEndian.Uint32(header[4:8]))
	if count > safariMaxCookies || *totalCookies > safariMaxCookies-count {
		return nil, fmt.Errorf("Safari cookie count exceeds limit")
	}
	*totalCookies += count
	offsetTableSize := int64(count * 4)
	offsetTableEnd := int64(8) + offsetTableSize
	if offsetTableEnd > pageSize {
		return nil, fmt.Errorf("truncated cookie offset table")
	}
	offsets := make([]byte, offsetTableSize)
	if err := readSafariAt(reader, fileSize, pageOffset+8, offsets, budget); err != nil {
		return nil, err
	}
	seenOffsets := make(map[uint32]struct{}, count)
	result := make([]appcookies.Record, 0)
	for index := uint64(0); index < count; index++ {
		offsetAt := index * 4
		cookieOffsetValue := binary.LittleEndian.Uint32(offsets[offsetAt : offsetAt+4])
		if _, duplicate := seenOffsets[cookieOffsetValue]; duplicate {
			return nil, fmt.Errorf("duplicate cookie offset")
		}
		seenOffsets[cookieOffsetValue] = struct{}{}
		cookieOffset := int64(cookieOffsetValue)
		if cookieOffset < offsetTableEnd || cookieOffset > pageSize-safariCookieHeaderLen {
			return nil, fmt.Errorf("invalid cookie offset")
		}
		cookieHeader := make([]byte, safariCookieHeaderLen)
		if err := readSafariAt(reader, fileSize, pageOffset+cookieOffset, cookieHeader, budget); err != nil {
			return nil, err
		}
		cookieSize := int64(binary.LittleEndian.Uint32(cookieHeader[:4]))
		if cookieSize < safariCookieHeaderLen || cookieSize > safariMaxCookieSize || cookieSize > pageSize-cookieOffset {
			return nil, fmt.Errorf("invalid cookie size")
		}
		domain, err := readSafariCookieDomain(reader, fileSize, pageOffset+cookieOffset, cookieSize, cookieHeader, budget)
		if err != nil {
			return nil, fmt.Errorf("cookie %d domain: %w", index, err)
		}
		if !snapshotDomainAllowed(domain, allowedDomains) {
			continue
		}
		cookie := make([]byte, cookieSize)
		if err := readSafariAt(reader, fileSize, pageOffset+cookieOffset, cookie, budget); err != nil {
			return nil, err
		}
		record, err := parseSafariCookie(cookie)
		if err != nil {
			return nil, fmt.Errorf("cookie %d: %w", index, err)
		}
		if !snapshotDomainAllowed(record.Domain, allowedDomains) {
			return nil, errSafariCookieSourceChanged
		}
		result = append(result, record)
	}
	return result, nil
}

func readSafariCookieDomain(
	reader io.ReaderAt,
	fileSize int64,
	cookieOffset int64,
	cookieSize int64,
	header []byte,
	budget *int64,
) (string, error) {
	domainOffset := int64(binary.LittleEndian.Uint32(header[16:20]))
	if domainOffset < safariCookieHeaderLen || domainOffset >= cookieSize {
		return "", fmt.Errorf("invalid string offset")
	}
	fieldEnd := cookieSize
	for _, at := range []int{20, 24, 28} {
		offset := int64(binary.LittleEndian.Uint32(header[at : at+4]))
		if offset > domainOffset && offset < fieldEnd {
			fieldEnd = offset
		}
	}
	length := fieldEnd - domainOffset
	if length <= 0 || length > safariDomainReadLimit {
		return "", fmt.Errorf("domain exceeds limit")
	}
	payload := make([]byte, length)
	if err := readSafariAt(reader, fileSize, cookieOffset+domainOffset, payload, budget); err != nil {
		return "", err
	}
	end := 0
	for end < len(payload) && payload[end] != 0 {
		end++
	}
	if end == len(payload) || !utf8.Valid(payload[:end]) {
		return "", fmt.Errorf("invalid domain string")
	}
	return string(payload[:end]), nil
}

func readSafariAt(reader io.ReaderAt, fileSize int64, offset int64, payload []byte, budget *int64) error {
	length := int64(len(payload))
	if offset < 0 || length < 0 || offset > fileSize || length > fileSize-offset {
		return io.ErrUnexpectedEOF
	}
	if budget == nil || length > *budget {
		return fmt.Errorf("Safari cookie scan exceeds read limit")
	}
	*budget -= length
	read, err := reader.ReadAt(payload, offset)
	if read != len(payload) {
		return io.ErrUnexpectedEOF
	}
	if err != nil {
		return err
	}
	return nil
}

func parseSafariCookies(payload []byte) ([]appcookies.Record, error) {
	if len(payload) < 16 || string(payload[:4]) != "cook" {
		return nil, fmt.Errorf("invalid Safari cookie archive header")
	}
	pageCount := uint64(binary.BigEndian.Uint32(payload[4:8]))
	if pageCount == 0 || pageCount > safariMaxPages {
		return nil, fmt.Errorf("invalid Safari cookie page count")
	}
	tableEnd := uint64(8) + pageCount*4
	if tableEnd > uint64(len(payload)) {
		return nil, fmt.Errorf("truncated Safari cookie page table")
	}
	pageOffset := tableEnd
	result := make([]appcookies.Record, 0)
	totalCookies := uint64(0)
	for pageIndex := uint64(0); pageIndex < pageCount; pageIndex++ {
		sizeOffset := 8 + pageIndex*4
		pageSize := uint64(binary.BigEndian.Uint32(payload[sizeOffset : sizeOffset+4]))
		if pageSize < 8 || pageOffset > uint64(len(payload)) || pageSize > uint64(len(payload))-pageOffset {
			return nil, fmt.Errorf("invalid Safari cookie page size")
		}
		page := payload[pageOffset : pageOffset+pageSize]
		pageOffset += pageSize
		cookies, err := parseSafariCookiePage(page, &totalCookies)
		if err != nil {
			return nil, fmt.Errorf("parse Safari cookie page %d: %w", pageIndex, err)
		}
		result = append(result, cookies...)
	}
	// Safari stores an eight-byte checksum after the cookie pages and some
	// versions append a bounded binary preferences payload. The page table is
	// authoritative; trailing bytes are never interpreted as cookies.
	if uint64(len(payload))-pageOffset < 8 {
		return nil, fmt.Errorf("invalid Safari cookie archive trailer")
	}
	return result, nil
}

func parseSafariCookiePage(page []byte, totalCookies *uint64) ([]appcookies.Record, error) {
	if len(page) < 8 || string(page[:4]) != string([]byte{0, 0, 1, 0}) {
		return nil, fmt.Errorf("invalid page header")
	}
	count := uint64(binary.LittleEndian.Uint32(page[4:8]))
	if count > safariMaxCookies || *totalCookies > safariMaxCookies-count {
		return nil, fmt.Errorf("Safari cookie count exceeds limit")
	}
	*totalCookies += count
	offsetTableEnd := uint64(8) + count*4
	if offsetTableEnd > uint64(len(page)) {
		return nil, fmt.Errorf("truncated cookie offset table")
	}
	result := make([]appcookies.Record, 0, int(count))
	for index := uint64(0); index < count; index++ {
		offsetAt := 8 + index*4
		cookieOffset := uint64(binary.LittleEndian.Uint32(page[offsetAt : offsetAt+4]))
		if cookieOffset < offsetTableEnd || cookieOffset > uint64(len(page))-4 {
			return nil, fmt.Errorf("invalid cookie offset")
		}
		record, err := parseSafariCookie(page[cookieOffset:])
		if err != nil {
			return nil, fmt.Errorf("cookie %d: %w", index, err)
		}
		result = append(result, record)
	}
	return result, nil
}

func parseSafariCookie(payload []byte) (appcookies.Record, error) {
	if len(payload) < safariCookieHeaderLen {
		return appcookies.Record{}, fmt.Errorf("truncated cookie header")
	}
	size := uint64(binary.LittleEndian.Uint32(payload[:4]))
	if size < safariCookieHeaderLen || size > safariMaxCookieSize || size > uint64(len(payload)) {
		return appcookies.Record{}, fmt.Errorf("invalid cookie size")
	}
	cookie := payload[:size]
	flags := binary.LittleEndian.Uint32(cookie[8:12])
	domain, err := safariCookieString(cookie, binary.LittleEndian.Uint32(cookie[16:20]))
	if err != nil {
		return appcookies.Record{}, fmt.Errorf("domain: %w", err)
	}
	name, err := safariCookieString(cookie, binary.LittleEndian.Uint32(cookie[20:24]))
	if err != nil {
		return appcookies.Record{}, fmt.Errorf("name: %w", err)
	}
	path, err := safariCookieString(cookie, binary.LittleEndian.Uint32(cookie[24:28]))
	if err != nil {
		return appcookies.Record{}, fmt.Errorf("path: %w", err)
	}
	value, err := safariCookieString(cookie, binary.LittleEndian.Uint32(cookie[28:32]))
	if err != nil {
		return appcookies.Record{}, fmt.Errorf("value: %w", err)
	}
	if name == "" || domain == "" {
		return appcookies.Record{}, fmt.Errorf("cookie identity is empty")
	}
	if path == "" {
		path = "/"
	}
	expires := safariCookieTime(binary.LittleEndian.Uint64(cookie[40:48]))
	return appcookies.Record{
		Name:     name,
		Value:    value,
		Domain:   domain,
		Path:     path,
		Expires:  expires,
		Secure:   flags&1 != 0,
		HttpOnly: flags&4 != 0,
	}, nil
}

func safariCookieString(cookie []byte, offset uint32) (string, error) {
	start := uint64(offset)
	if start < safariCookieHeaderLen || start >= uint64(len(cookie)) {
		return "", fmt.Errorf("invalid string offset")
	}
	end := start
	for end < uint64(len(cookie)) && cookie[end] != 0 {
		end++
	}
	if end == uint64(len(cookie)) {
		return "", fmt.Errorf("unterminated string")
	}
	value := cookie[start:end]
	if !utf8.Valid(value) {
		return "", fmt.Errorf("invalid UTF-8")
	}
	return string(value), nil
}

func safariCookieTime(bits uint64) int64 {
	seconds := math.Float64frombits(bits)
	if math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds <= 0 || seconds > float64(safariMaxUnixTime-safariEpochOffset) {
		return 0
	}
	return int64(seconds) + safariEpochOffset
}
