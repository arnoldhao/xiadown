package locallyrics

import "time"

// Format identifies the input syntax that produced a Result.
type Format string

const (
	FormatPlain       Format = "plain"
	FormatLRC         Format = "lrc"
	FormatEnhancedLRC Format = "enhanced-lrc"
	FormatVTT         Format = "vtt"
	FormatTTML        Format = "ttml"
)

// TimingQuality describes how trustworthy the most detailed timing in a
// result is. Consumers should use this instead of inferring quality from a
// filename extension or from the mere presence of Words.
type TimingQuality string

const (
	TimingQualityPlain     TimingQuality = "plain"
	TimingQualityEstimated TimingQuality = "estimated"
	TimingQualityLine      TimingQuality = "line"
	TimingQualityWord      TimingQuality = "word"
	TimingQualitySyllable  TimingQuality = "syllable"
)

// SourceKind identifies how lyric bytes entered the package.
type SourceKind string

const (
	SourceSidecar  SourceKind = "sidecar"
	SourceEmbedded SourceKind = "embedded"
)

// Attribution is deliberately provider-neutral. It can be shown to users or
// retained by a cache without coupling this package to a presentation layer.
type Attribution struct {
	Kind  SourceKind
	Label string
}

// AlternateText carries aligned text other than the main lyric. Role is
// commonly "translation" or "romanization", but is intentionally open.
type AlternateText struct {
	Role     string
	Language string
	Text     string
}

// Word always has an explicit end. EndsWithSpace preserves display spacing
// after parsers trim the timing token itself. Syllables use the same stable
// shape so renderers can choose word- or syllable-level highlighting.
type Word struct {
	Start         time.Duration
	End           time.Duration
	Text          string
	EndsWithSpace bool
	Syllables     []Word
}

// Line always has an explicit end. EndEstimated is true only when the source
// did not provide an end and the parser had to derive one.
type Line struct {
	Start          time.Duration
	End            time.Duration
	EndEstimated   bool
	Text           string
	Translation    string
	AlternateTexts []AlternateText
	Words          []Word
}

// Result is the only lyric representation returned by this package.
// PlainText is retained even for timed lyrics so callers have a readable,
// renderer-independent fallback.
type Result struct {
	Format                Format
	TimingQuality         TimingQuality
	PlainText             string
	Title                 string
	Artist                string
	Lines                 []Line
	SourcePath            string
	TranslationSourcePath string
	Attribution           Attribution
}

// Candidate is a parsed sidecar candidate. Score is based on parsed
// capabilities and is only intended for deterministic ordering within this
// package; callers should primarily inspect Result.TimingQuality.
type Candidate struct {
	Path            string
	TranslationPath string
	Score           int
	Result          Result
}

func timingQualityRank(quality TimingQuality) int {
	switch quality {
	case TimingQualitySyllable:
		return 5
	case TimingQualityWord:
		return 4
	case TimingQualityLine:
		return 3
	case TimingQualityEstimated:
		return 2
	case TimingQualityPlain:
		return 1
	default:
		return 0
	}
}
