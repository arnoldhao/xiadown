package locallyrics

import (
	"context"
	"fmt"
	"strings"
)

// Content is an explicit, in-memory lyric payload. Name fields are display
// names and format hints only; they are never opened as paths.
type Content struct {
	Name              string
	Bytes             []byte
	Format            Format
	TranslationName   string
	TranslationBytes  []byte
	TranslationFormat Format
	AttributionLabel  string
}

// EmbeddedReader is implemented by a media metadata layer. The package never
// invokes ffmpeg, tag readers, shells, or any other external command itself.
type EmbeddedReader interface {
	ReadEmbeddedLyrics(ctx context.Context, mediaPath string) (Content, error)
}

// ParseEmbedded asks the injected reader for bytes and parses them without
// granting it any implicit filesystem or process behavior.
func ParseEmbedded(ctx context.Context, reader EmbeddedReader, mediaPath string, options Options) (Result, error) {
	if reader == nil {
		return Result{}, fmt.Errorf("%w: embedded lyric reader unavailable", ErrNoLyrics)
	}
	if strings.TrimSpace(mediaPath) == "" || strings.ContainsRune(mediaPath, '\x00') {
		return Result{}, ErrInvalidPath
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	content, err := reader.ReadEmbeddedLyrics(ctx, mediaPath)
	if err != nil {
		return Result{}, err
	}
	return parseExplicitContent(content, SourceEmbedded, "Embedded lyric", options)
}

func parseExplicitContent(content Content, kind SourceKind, defaultLabel string, options Options) (Result, error) {
	mainHint := content.Format
	if mainHint == "" {
		mainHint = formatFromName(content.Name)
	}
	translationHint := content.TranslationFormat
	if translationHint == "" {
		translationHint = formatFromName(content.TranslationName)
	}
	result, err := ParseWithTranslation(content.Bytes, mainHint, content.TranslationBytes, translationHint, options)
	if err != nil {
		return Result{}, err
	}
	label := strings.TrimSpace(content.AttributionLabel)
	if label == "" {
		label = defaultLabel
	}
	result.Attribution = Attribution{Kind: kind, Label: label}
	result.SourcePath = safeDisplayName(content.Name)
	result.TranslationSourcePath = safeDisplayName(content.TranslationName)
	return result, nil
}

func safeDisplayName(value string) string {
	if strings.ContainsRune(value, '\x00') {
		return ""
	}
	return portableBase(strings.TrimSpace(value))
}
