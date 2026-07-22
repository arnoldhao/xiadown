package locallyricsreader

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/dhowden/tag"
)

func TestReadEmbeddedLyricsValidatesAdditionalContainers(t *testing.T) {
	lyrics := "[00:01.00]container lyric"
	tests := []struct {
		name        string
		extension   string
		data        []byte
		attribution string
	}{
		{name: "FLAC Vorbis", extension: ".flac", data: buildFLACComments([]string{"LYRICS=" + lyrics}), attribution: "Embedded FLAC lyrics (VORBIS)"},
		{name: "OGG OpusTags", extension: ".ogg", data: buildOGGCommentPage(append([]byte("OpusTags"), buildVorbisComments([]string{"LYRICS=" + lyrics})...)), attribution: "Embedded OGG lyrics (VORBIS)"},
		{name: "MP4 text atom", extension: ".m4a", data: buildMP4Lyrics(lyrics), attribution: "Embedded lyrics (MP4)"},
		{name: "DSF ID3 pointer", extension: ".dsf", data: buildDSFWithID3(buildID3v23USLT(lyrics)), attribution: "Embedded DSF lyrics (ID3v2.3)"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeMetadataFixture(t, test.extension, test.data)
			content, err := New().ReadEmbeddedLyrics(context.Background(), path)
			if err != nil {
				t.Fatalf("ReadEmbeddedLyrics() error = %v", err)
			}
			if string(content.Bytes) != lyrics {
				t.Fatalf("content.Bytes = %q, want %q", content.Bytes, lyrics)
			}
			if content.AttributionLabel != test.attribution {
				t.Fatalf("AttributionLabel = %q, want %q", content.AttributionLabel, test.attribution)
			}
		})
	}
}

func TestPrevalidationRejectsMaliciousDeclaredLengthsBeforeTagReader(t *testing.T) {
	t.Run("ID3 oversized frame", func(t *testing.T) {
		frame := make([]byte, 10)
		copy(frame[:4], "TIT2")
		binary.BigEndian.PutUint32(frame[4:8], 0x7fffffff)
		header := []byte{'I', 'D', '3', 3, 0, 0, 0, 0, 0, 0}
		putSyncSafe(header[6:10], len(frame))
		assertRejectedBeforeTag(t, append(header, frame...), Options{}, ErrMetadataTooLarge)
	})

	t.Run("ID3v2.4 malicious DLI", func(t *testing.T) {
		frame := make([]byte, 15)
		copy(frame[:4], "USLT")
		putSyncSafe(frame[4:8], 5)
		frame[9] = 0x01 // data length indicator
		copy(frame[10:14], []byte{0x7f, 0x7f, 0x7f, 0x7f})
		header := []byte{'I', 'D', '3', 4, 0, 0, 0, 0, 0, 0}
		putSyncSafe(header[6:10], len(frame))
		assertRejectedBeforeTag(t, append(header, frame...), Options{}, ErrMetadataTooLarge)
	})

	t.Run("FLAC Vorbis comment length", func(t *testing.T) {
		payload := make([]byte, 8)
		binary.LittleEndian.PutUint32(payload[:4], 0xffffffff)
		data := append([]byte("fLaC"), flacBlockHeader(0x84, len(payload))...)
		data = append(data, payload...)
		assertRejectedBeforeTag(t, data, Options{}, ErrMetadataTooLarge)
	})

	t.Run("FLAC picture data length", func(t *testing.T) {
		picture := make([]byte, 32)
		binary.BigEndian.PutUint32(picture[28:32], 0xffffffff)
		data := append([]byte("fLaC"), flacBlockHeader(0x86, len(picture))...)
		data = append(data, picture...)
		assertRejectedBeforeTag(t, data, Options{}, ErrMetadataTooLarge)
	})

	t.Run("Vorbis base64 picture data length", func(t *testing.T) {
		picture := make([]byte, 32)
		binary.BigEndian.PutUint32(picture[28:32], 0xffffffff)
		comment := "METADATA_BLOCK_PICTURE=" + base64.StdEncoding.EncodeToString(picture)
		assertRejectedBeforeTag(t, buildFLACComments([]string{comment}), Options{}, ErrMetadataTooLarge)
	})

	t.Run("MP4 child atom escapes parent", func(t *testing.T) {
		childHeader := make([]byte, 8)
		binary.BigEndian.PutUint32(childHeader[:4], 0xffffffff)
		copy(childHeader[4:8], "udta")
		data := append(mp4Atom("ftyp", []byte("M4A ")), mp4Atom("moov", childHeader)...)
		assertRejectedBeforeTag(t, data, Options{}, ErrUnsafeMetadata)
	})

	t.Run("OGG packet exceeds bounded budget", func(t *testing.T) {
		var header [27]byte
		copy(header[:4], "OggS")
		header[26] = 5
		data := append(header[:], []byte{255, 255, 255, 255, 255}...)
		data = append(data, bytes.Repeat([]byte{'x'}, 5*255)...)
		assertRejectedBeforeTag(t, data, Options{MaxMetadataBytes: 1024}, ErrMetadataTooLarge)
	})

	t.Run("DSF ID3 pointer escapes file", func(t *testing.T) {
		data := make([]byte, 28)
		copy(data[:4], "DSD ")
		binary.LittleEndian.PutUint64(data[4:12], 28)
		binary.LittleEndian.PutUint64(data[12:20], uint64(len(data)))
		binary.LittleEndian.PutUint64(data[20:28], ^uint64(0))
		assertRejectedBeforeTag(t, data, Options{}, ErrUnsafeMetadata)
	})
}

func TestPrevalidationEnforcesCountAndDepthLimits(t *testing.T) {
	t.Run("ID3 frame count", func(t *testing.T) {
		frames := make([]byte, 0, (maxID3Frames+1)*11)
		for index := 0; index <= maxID3Frames; index++ {
			frame := make([]byte, 11)
			copy(frame[:4], "TIT2")
			binary.BigEndian.PutUint32(frame[4:8], 1)
			frames = append(frames, frame...)
		}
		header := []byte{'I', 'D', '3', 3, 0, 0, 0, 0, 0, 0}
		putSyncSafe(header[6:10], len(frames))
		assertRejectedBeforeTag(t, append(header, frames...), Options{}, ErrMetadataTooLarge)
	})

	t.Run("ID3 duplicate frame probe cost", func(t *testing.T) {
		frames := make([]byte, 0, (maxID3FramesPerID+1)*11)
		for index := 0; index <= maxID3FramesPerID; index++ {
			frame := make([]byte, 11)
			copy(frame[:4], "PRIV")
			binary.BigEndian.PutUint32(frame[4:8], 1)
			frames = append(frames, frame...)
		}
		header := append([]byte("ID3\x03\x00\x00"), make([]byte, 4)...)
		putSyncSafe(header[6:10], len(frames))
		assertRejectedBeforeTag(t, append(header, frames...), Options{}, ErrMetadataTooLarge)
	})

	t.Run("FLAC block count", func(t *testing.T) {
		data := []byte("fLaC")
		for index := 0; index <= maxFLACBlocks; index++ {
			blockType := byte(1)
			if index == maxFLACBlocks {
				blockType |= 0x80
			}
			data = append(data, flacBlockHeader(blockType, 0)...)
		}
		assertRejectedBeforeTag(t, data, Options{}, ErrMetadataTooLarge)
	})

	t.Run("Vorbis comment count", func(t *testing.T) {
		payload := make([]byte, 8)
		binary.LittleEndian.PutUint32(payload[4:8], maxVorbisComments+1)
		data := append([]byte("fLaC"), flacBlockHeader(0x84, len(payload))...)
		data = append(data, payload...)
		assertRejectedBeforeTag(t, data, Options{}, ErrMetadataTooLarge)
	})

	t.Run("OGG page count", func(t *testing.T) {
		data := make([]byte, 0, (maxOGGPages+1)*27)
		for index := 0; index <= maxOGGPages; index++ {
			page := make([]byte, 27)
			copy(page[:4], "OggS")
			binary.LittleEndian.PutUint32(page[14:18], 1)
			data = append(data, page...)
		}
		assertRejectedBeforeTag(t, data, Options{}, ErrMetadataTooLarge)
	})

	t.Run("MP4 atom depth", func(t *testing.T) {
		deep := []byte{}
		for index := 0; index < maxMP4Depth+2; index++ {
			deep = mp4Atom("moov", deep)
		}
		data := append(mp4Atom("ftyp", []byte("M4A ")), deep...)
		assertRejectedBeforeTag(t, data, Options{}, ErrMetadataTooLarge)
	})
}

func TestMetadataBudgetHardCeilingAndNearLyricsLimit(t *testing.T) {
	reader := NewWithOptions(Options{MaxMetadataBytes: hardMaxMetadataBytes * 4})
	if reader.metadataLimit() != hardMaxMetadataBytes {
		t.Fatalf("metadataLimit() = %d, want hard cap %d", reader.metadataLimit(), hardMaxMetadataBytes)
	}

	lyrics := bytes.Repeat([]byte{'x'}, int(defaultMaxLyricsBytes-64))
	path := writeMetadataFixture(t, ".mp3", buildID3v23USLT(string(lyrics)))
	content, err := New().ReadEmbeddedLyrics(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadEmbeddedLyrics() near lyric limit error = %v", err)
	}
	if len(content.Bytes) != len(lyrics) {
		t.Fatalf("content length = %d, want %d", len(content.Bytes), len(lyrics))
	}
}

func TestID3PrevalidationHandlesGlobalUnsynchronisationAndAPIC(t *testing.T) {
	pictureBody := append([]byte{0}, []byte("image/png")...)
	pictureBody = append(pictureBody, 0, 3, 0, 0x89, 'P', 'N', 'G')
	pictureFrame := id3v23Frame("APIC", pictureBody)
	lyricsBody := append([]byte{0, 'e', 'n', 'g', 0}, []byte("lyrics")...)
	lyricsFrame := id3v23Frame("USLT", lyricsBody)
	frames := append(pictureFrame, lyricsFrame...)
	header := []byte{'I', 'D', '3', 3, 0, 0x80, 0, 0, 0, 0}
	putSyncSafe(header[6:10], len(frames))
	path := writeMetadataFixture(t, ".mp3", append(header, frames...))
	content, err := New().ReadEmbeddedLyrics(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadEmbeddedLyrics() error = %v", err)
	}
	if string(content.Bytes) != "lyrics" {
		t.Fatalf("content.Bytes = %q", content.Bytes)
	}
}

func assertRejectedBeforeTag(t *testing.T, data []byte, options Options, want error) {
	t.Helper()
	path := writeMetadataFixture(t, ".bin", data)
	reader := NewWithOptions(options)
	called := false
	reader.readMetadata = func(io.ReadSeeker) (tag.Metadata, error) {
		called = true
		return nil, fmt.Errorf("tag reader should not be called")
	}
	_, err := reader.ReadEmbeddedLyrics(context.Background(), path)
	if called {
		t.Fatal("tag.ReadFrom replacement was called before malicious metadata was rejected")
	}
	if !errors.Is(err, want) {
		t.Fatalf("ReadEmbeddedLyrics() error = %v, want %v", err, want)
	}
}

func writeMetadataFixture(t *testing.T, extension string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "track"+extension)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func buildVorbisComments(comments []string) []byte {
	buffer := &bytes.Buffer{}
	_ = binary.Write(buffer, binary.LittleEndian, uint32(len("xiadown")))
	buffer.WriteString("xiadown")
	_ = binary.Write(buffer, binary.LittleEndian, uint32(len(comments)))
	for _, comment := range comments {
		_ = binary.Write(buffer, binary.LittleEndian, uint32(len(comment)))
		buffer.WriteString(comment)
	}
	return buffer.Bytes()
}

func buildFLACComments(comments []string) []byte {
	payload := buildVorbisComments(comments)
	data := append([]byte("fLaC"), flacBlockHeader(0x84, len(payload))...)
	return append(data, payload...)
}

func flacBlockHeader(blockType byte, length int) []byte {
	return []byte{blockType, byte(length >> 16), byte(length >> 8), byte(length)}
}

func buildOGGCommentPage(packet []byte) []byte {
	if len(packet) >= 255 {
		panic("test packet must fit one OGG segment")
	}
	header := make([]byte, 27)
	copy(header[:4], "OggS")
	header[5] = 0x02
	binary.LittleEndian.PutUint32(header[14:18], 1)
	header[26] = 1
	page := append(header, byte(len(packet)))
	page = append(page, packet...)
	crc := oggFixtureCRC(page)
	binary.LittleEndian.PutUint32(page[22:26], crc)
	return page
}

func oggFixtureCRC(data []byte) uint32 {
	var crc uint32
	for _, value := range data {
		crc ^= uint32(value) << 24
		for bit := 0; bit < 8; bit++ {
			if crc&0x80000000 != 0 {
				crc = crc<<1 ^ 0x04c11db7
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

func buildMP4Lyrics(lyrics string) []byte {
	dataPayload := append([]byte{0, 0, 0, 1, 0, 0, 0, 0}, []byte(lyrics)...)
	lyricsAtom := mp4Atom("\xa9lyr", mp4Atom("data", dataPayload))
	ilst := mp4Atom("ilst", lyricsAtom)
	meta := mp4Atom("meta", append([]byte{0, 0, 0, 0}, ilst...))
	udta := mp4Atom("udta", meta)
	moov := mp4Atom("moov", udta)
	return append(mp4Atom("ftyp", []byte("M4A ")), moov...)
}

func mp4Atom(name string, payload []byte) []byte {
	if len(name) != 4 {
		panic("MP4 atom name must be four bytes")
	}
	atom := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint32(atom[:4], uint32(len(atom)))
	copy(atom[4:8], name)
	copy(atom[8:], payload)
	return atom
}

func id3v23Frame(name string, payload []byte) []byte {
	if len(name) != 4 {
		panic("ID3v2.3 frame name must be four bytes")
	}
	frame := make([]byte, 10+len(payload))
	copy(frame[:4], name)
	binary.BigEndian.PutUint32(frame[4:8], uint32(len(payload)))
	copy(frame[10:], payload)
	return frame
}

func buildDSFWithID3(id3 []byte) []byte {
	data := make([]byte, 28, 28+len(id3))
	copy(data[:4], "DSD ")
	binary.LittleEndian.PutUint64(data[4:12], 28)
	binary.LittleEndian.PutUint64(data[12:20], uint64(28+len(id3)))
	binary.LittleEndian.PutUint64(data[20:28], 28)
	return append(data, id3...)
}
