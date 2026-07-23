package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"

	mediatag "github.com/dhowden/tag"
)

type listenLocalRawTagManifest struct {
	tags     map[string]any
	hasID3v1 bool
}

func probeListenLocalRawTagManifest(ctx context.Context, path string) (listenLocalRawTagManifest, error) {
	if err := ctx.Err(); err != nil {
		return listenLocalRawTagManifest{}, err
	}
	file, err := os.Open(path)
	if err != nil {
		return listenLocalRawTagManifest{}, err
	}
	defer file.Close()
	hasID3v1 := listenLocalFileHasID3v1(file)
	if _, err := file.Seek(0, 0); err != nil {
		return listenLocalRawTagManifest{}, err
	}
	metadata, err := mediatag.ReadFrom(file)
	if errors.Is(err, mediatag.ErrNoTagsFound) {
		return listenLocalRawTagManifest{tags: map[string]any{}, hasID3v1: hasID3v1}, nil
	}
	if err != nil {
		return listenLocalRawTagManifest{}, listenLocalMetadataPreservationError(fmt.Sprintf("raw tags that could not be inspected: %v", err))
	}
	result := make(map[string]any, len(metadata.Raw()))
	for key, value := range metadata.Raw() {
		result[normalizeListenLocalRawTagKey(key)] = value
	}
	return listenLocalRawTagManifest{tags: result, hasID3v1: hasID3v1}, nil
}

func verifyListenLocalRawTagsPreserved(before listenLocalRawTagManifest, after listenLocalRawTagManifest) error {
	if before.hasID3v1 != after.hasID3v1 {
		return listenLocalMetadataPreservationError("the ID3v1 tag presence changed")
	}
	for key, value := range before.tags {
		if listenLocalRawEditedTag(key) || listenLocalRawVolatileTag(key) {
			continue
		}
		afterValue, exists := after.tags[key]
		if !exists || !reflect.DeepEqual(value, afterValue) {
			return listenLocalMetadataPreservationError(fmt.Sprintf("raw tag %q changed or was removed", key))
		}
	}
	return nil
}

func listenLocalFileHasID3v1(file *os.File) bool {
	if file == nil {
		return false
	}
	info, err := file.Stat()
	if err != nil || info.Size() < 128 {
		return false
	}
	header := make([]byte, 3)
	if _, err := file.ReadAt(header, info.Size()-128); err != nil {
		return false
	}
	return bytes.Equal(header, []byte("TAG"))
}

func normalizeListenLocalRawTagKey(key string) string {
	return strings.ToLower(strings.TrimSpace(key))
}

func listenLocalRawEditedTag(key string) bool {
	key = normalizeListenLocalRawTagKey(key)
	itunesKey := strings.TrimLeft(key, "©\ufffd")
	switch itunesKey {
	case "nam", "art", "alb", "gen", "day":
		return true
	}
	switch key {
	case "title", "tit2", "tt2",
		"artist", "tpe1", "tp1",
		"album", "talb", "tal",
		"albumartist", "album_artist", "tpe2", "tp2", "aart",
		"genre", "tcon", "tco", "gnre",
		"track", "tracknumber", "trck", "trkn", "trk",
		"disc", "discnumber", "tpos", "tpa", "disk",
		"date", "year", "tdrc", "tyer", "tye",
		// Chapter integrity, including titles and timing, is compared by the
		// ffprobe preservation manifest. ID3 chapter frames are rewritten when
		// converting between ID3v2.4 and the broadly compatible ID3v2.3 output.
		"chap", "ctoc":
		return true
	default:
		return false
	}
}

func listenLocalRawVolatileTag(key string) bool {
	key = normalizeListenLocalRawTagKey(key)
	if strings.TrimLeft(key, "©\ufffd") == "too" {
		return true
	}
	switch key {
	case "encoder", "encodedby", "encoded_by", "tenc":
		return true
	default:
		return false
	}
}
