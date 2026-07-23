package locallyrics

import (
	"errors"
	"time"
)

const (
	defaultMaxBytes        int64 = 2 << 20
	defaultMaxLines              = 20_000
	defaultMaxWordsPerLine       = 4_096
	defaultMaxXMLDepth           = 64
	defaultMaxXMLTokens          = 200_000
	maxSupportedLyricTime        = 7 * 24 * time.Hour
)

var (
	ErrInvalidPath  = errors.New("invalid lyric path")
	ErrPathEscape   = errors.New("lyric path escapes media directory")
	ErrUnsafeFile   = errors.New("unsafe lyric file")
	ErrTooLarge     = errors.New("lyric input exceeds size limit")
	ErrTooComplex   = errors.New("lyric input exceeds complexity limit")
	ErrUnsafeMarkup = errors.New("unsafe XML markup")
	ErrNoLyrics     = errors.New("no lyric content")
)

// Options bounds all file and parser work. Zero values receive conservative
// defaults. Symlinks are rejected unless explicitly enabled, and even then
// their resolved target must remain inside the media directory.
type Options struct {
	MaxBytes             int64
	MaxLines             int
	MaxWordsPerLine      int
	MaxXMLDepth          int
	MaxXMLTokens         int
	TranslationTolerance time.Duration
	DefaultLineDuration  time.Duration
	AllowSymlinks        bool
	DisablePlainFallback bool
}

func normalizeOptions(options Options) Options {
	if options.MaxBytes <= 0 {
		options.MaxBytes = defaultMaxBytes
	}
	if options.MaxLines <= 0 {
		options.MaxLines = defaultMaxLines
	}
	if options.MaxWordsPerLine <= 0 {
		options.MaxWordsPerLine = defaultMaxWordsPerLine
	}
	if options.MaxXMLDepth <= 0 {
		options.MaxXMLDepth = defaultMaxXMLDepth
	}
	if options.MaxXMLTokens <= 0 {
		options.MaxXMLTokens = defaultMaxXMLTokens
	}
	if options.TranslationTolerance <= 0 {
		options.TranslationTolerance = time.Second
	} else if options.TranslationTolerance > maxSupportedLyricTime {
		options.TranslationTolerance = maxSupportedLyricTime
	}
	if options.DefaultLineDuration <= 0 {
		options.DefaultLineDuration = 5 * time.Second
	} else if options.DefaultLineDuration > maxSupportedLyricTime {
		options.DefaultLineDuration = maxSupportedLyricTime
	}
	return options
}
