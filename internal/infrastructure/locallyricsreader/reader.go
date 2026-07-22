package locallyricsreader

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dhowden/tag"

	"xiadown/internal/application/locallyrics"
)

const (
	defaultMaxLyricsBytes   int64 = 2 << 20
	defaultMaxMetadataBytes int64 = 16 << 20
	hardMaxMetadataBytes    int64 = 64 << 20
)

var (
	// ErrInvalidMediaPath reports an empty path or a path containing NUL.
	ErrInvalidMediaPath = errors.New("invalid media path")
	// ErrInvalidContext reports a nil context.
	ErrInvalidContext = errors.New("invalid context")
	// ErrUnsafeMediaFile reports a symlink or a non-regular media file.
	ErrUnsafeMediaFile = errors.New("unsafe media file")
	// ErrNoEmbeddedLyrics reports valid metadata with no non-blank lyric value.
	ErrNoEmbeddedLyrics = errors.New("no embedded lyrics")
	// ErrLyricsTooLarge reports lyrics larger than Options.MaxLyricsBytes.
	ErrLyricsTooLarge = errors.New("embedded lyrics exceed size limit")
	// ErrUnsafeMetadata reports malformed metadata whose declared structure
	// cannot be proven to stay within its enclosing container and file.
	ErrUnsafeMetadata = errors.New("unsafe embedded metadata")
	// ErrMetadataTooLarge reports metadata whose declared or estimated parser
	// allocations exceed Options.MaxMetadataBytes.
	ErrMetadataTooLarge = errors.New("embedded metadata exceeds allocation budget")
)

// Options controls bounded work before and after metadata extraction. Zero and
// negative values use conservative defaults. MaxMetadataBytes defaults to
// 16 MiB and is always clamped to a 64 MiB hard ceiling.
type Options struct {
	MaxLyricsBytes   int64
	MaxMetadataBytes int64
}

// Reader is an immutable embedded metadata reader and is safe for concurrent
// use. Its zero value uses the default limit. Callers must provide a completed,
// stable media file rather than a path that another process is actively
// rewriting. Size/identity/time checks and a post-read revalidation detect
// ordinary concurrent changes, but cannot turn a hostile writer sharing the
// same local account into an immutable snapshot. Metadata decoding is provided
// by the pure-Go github.com/dhowden/tag package; no shell or external media
// process is started.
type Reader struct {
	maxLyricsBytes   int64
	maxMetadataBytes int64
	readMetadata     func(io.ReadSeeker) (tag.Metadata, error)
}

var _ locallyrics.EmbeddedReader = (*Reader)(nil)

// New returns a Reader using conservative defaults.
func New() *Reader {
	return NewWithOptions(Options{})
}

// NewWithOptions returns a Reader with explicit resource limits.
func NewWithOptions(options Options) *Reader {
	maxLyricsBytes := options.MaxLyricsBytes
	if maxLyricsBytes <= 0 {
		maxLyricsBytes = defaultMaxLyricsBytes
	}
	maxMetadataBytes := options.MaxMetadataBytes
	if maxMetadataBytes <= 0 {
		maxMetadataBytes = defaultMaxMetadataBytes
	} else if maxMetadataBytes > hardMaxMetadataBytes {
		maxMetadataBytes = hardMaxMetadataBytes
	}
	return &Reader{
		maxLyricsBytes:   maxLyricsBytes,
		maxMetadataBytes: maxMetadataBytes,
		readMetadata:     tag.ReadFrom,
	}
}

// ReadEmbeddedLyrics extracts lyrics from ID3, MP4/M4A, OGG, FLAC, or DSF
// metadata. Context cancellation is checked around file and decode operations;
// the underlying synchronous parser itself has no cancellation hook.
func (r *Reader) ReadEmbeddedLyrics(ctx context.Context, mediaPath string) (locallyrics.Content, error) {
	if ctx == nil {
		return locallyrics.Content{}, ErrInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return locallyrics.Content{}, err
	}
	if strings.TrimSpace(mediaPath) == "" || strings.ContainsRune(mediaPath, '\x00') {
		return locallyrics.Content{}, ErrInvalidMediaPath
	}

	file, err := openRegularFile(mediaPath)
	if err != nil {
		return locallyrics.Content{}, err
	}
	defer file.Close()

	if err := ctx.Err(); err != nil {
		return locallyrics.Content{}, err
	}
	fileInfo, err := file.Stat()
	if err != nil {
		return locallyrics.Content{}, fmt.Errorf("inspect media before metadata validation: %w", err)
	}
	if err := validateMetadataBeforeTag(ctx, file, fileInfo.Size(), r.metadataLimit()); err != nil {
		return locallyrics.Content{}, err
	}
	validatedInfo, err := file.Stat()
	if err != nil {
		return locallyrics.Content{}, fmt.Errorf("inspect media after metadata validation: %w", err)
	}
	if err := ensureMediaFileUnchanged(fileInfo, validatedInfo, "during metadata validation"); err != nil {
		return locallyrics.Content{}, err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return locallyrics.Content{}, fmt.Errorf("rewind media after metadata validation: %w", err)
	}
	metadata, metadataErr := r.metadataReader()(file)
	parsedInfo, err := file.Stat()
	if err != nil {
		return locallyrics.Content{}, fmt.Errorf("inspect media after metadata read: %w", err)
	}
	if err := ensureMediaFileUnchanged(validatedInfo, parsedInfo, "during metadata read"); err != nil {
		return locallyrics.Content{}, err
	}
	// Revalidate after the third-party parser so accidental concurrent writes
	// are never accepted even when a filesystem preserves size or timestamp
	// granularity. This is detection, not a substitute for the static-file
	// precondition documented on Reader.
	if err := validateMetadataBeforeTag(ctx, file, parsedInfo.Size(), r.metadataLimit()); err != nil {
		return locallyrics.Content{}, err
	}
	if metadataErr != nil {
		if errors.Is(metadataErr, tag.ErrNoTagsFound) {
			return locallyrics.Content{}, fmt.Errorf("%w: metadata tags unavailable", ErrNoEmbeddedLyrics)
		}
		return locallyrics.Content{}, fmt.Errorf("read media metadata: %w", metadataErr)
	}
	if err := ctx.Err(); err != nil {
		return locallyrics.Content{}, err
	}

	limit := r.lyricsLimit()
	lyrics := metadata.Lyrics()
	if int64(len(lyrics)) > limit {
		return locallyrics.Content{}, fmt.Errorf("%w: got %d bytes, limit %d", ErrLyricsTooLarge, len(lyrics), limit)
	}
	if strings.TrimSpace(lyrics) == "" {
		lyrics = lyricsFromRaw(metadata.Raw())
	}
	if int64(len(lyrics)) > limit {
		return locallyrics.Content{}, fmt.Errorf("%w: got %d bytes, limit %d", ErrLyricsTooLarge, len(lyrics), limit)
	}
	if strings.TrimSpace(lyrics) == "" {
		return locallyrics.Content{}, ErrNoEmbeddedLyrics
	}
	if err := ctx.Err(); err != nil {
		return locallyrics.Content{}, err
	}

	return locallyrics.Content{
		Name:             "embedded-lyrics.txt",
		Bytes:            []byte(lyrics),
		AttributionLabel: attributionLabel(metadata),
	}, nil
}

func ensureMediaFileUnchanged(before os.FileInfo, after os.FileInfo, phase string) error {
	if before == nil || after == nil ||
		!after.Mode().IsRegular() ||
		!os.SameFile(before, after) ||
		before.Size() != after.Size() ||
		!before.ModTime().Equal(after.ModTime()) {
		return fmt.Errorf("%w: media file changed %s", ErrUnsafeMediaFile, phase)
	}
	return nil
}

func (r *Reader) lyricsLimit() int64 {
	if r == nil || r.maxLyricsBytes <= 0 {
		return defaultMaxLyricsBytes
	}
	return r.maxLyricsBytes
}

func (r *Reader) metadataLimit() int64 {
	if r == nil || r.maxMetadataBytes <= 0 {
		return defaultMaxMetadataBytes
	}
	if r.maxMetadataBytes > hardMaxMetadataBytes {
		return hardMaxMetadataBytes
	}
	return r.maxMetadataBytes
}

func (r *Reader) metadataReader() func(io.ReadSeeker) (tag.Metadata, error) {
	if r == nil || r.readMetadata == nil {
		return tag.ReadFrom
	}
	return r.readMetadata
}

func openRegularFile(mediaPath string) (*os.File, error) {
	before, err := os.Lstat(mediaPath)
	if err != nil {
		return nil, fmt.Errorf("inspect media file: %w", err)
	}
	if before.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("%w: symlinks are not allowed", ErrUnsafeMediaFile)
	}
	if !before.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: expected a regular file", ErrUnsafeMediaFile)
	}

	file, err := os.Open(mediaPath)
	if err != nil {
		return nil, fmt.Errorf("open media file: %w", err)
	}
	opened, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("inspect opened media file: %w", err)
	}
	if !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		file.Close()
		return nil, fmt.Errorf("%w: media file changed while opening", ErrUnsafeMediaFile)
	}

	// Recheck the path after opening so a final-component symlink swap is not
	// silently accepted. The descriptor identity check above remains the source
	// of bytes passed to the parser.
	after, err := os.Lstat(mediaPath)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("reinspect media file: %w", err)
	}
	if after.Mode()&os.ModeSymlink != 0 || !after.Mode().IsRegular() || !os.SameFile(opened, after) {
		file.Close()
		return nil, fmt.Errorf("%w: media path changed while opening", ErrUnsafeMediaFile)
	}
	return file, nil
}

func attributionLabel(metadata tag.Metadata) string {
	fileType := strings.TrimSpace(string(metadata.FileType()))
	format := strings.TrimSpace(string(metadata.Format()))
	switch {
	case fileType != "" && format != "":
		return fmt.Sprintf("Embedded %s lyrics (%s)", fileType, format)
	case fileType != "":
		return fmt.Sprintf("Embedded %s lyrics", fileType)
	case format != "":
		return fmt.Sprintf("Embedded lyrics (%s)", format)
	default:
		return "Embedded lyrics"
	}
}
