package locallyricsreader

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
)

const (
	maxID3Frames      = 4096
	maxID3FramesPerID = 64
	id3FrameMapBytes  = 128
	id3NameProbeBytes = 64
	maxFLACBlocks     = 1024
	maxVorbisComments = 4096
	maxOGGPages       = 256
	maxOGGStreams     = 16
	maxMP4Atoms       = 16384
	maxMP4Depth       = 16
)

type metadataBudget struct {
	limit int64
	used  int64
}

func (budget *metadataBudget) require(length int64, label string) error {
	if length < 0 {
		return unsafeMetadataf("%s has a negative length", label)
	}
	if length > budget.limit {
		return metadataTooLargef("%s declares %d bytes, limit %d", label, length, budget.limit)
	}
	return nil
}

func (budget *metadataBudget) consume(length int64, label string) error {
	if err := budget.require(length, label); err != nil {
		return err
	}
	if length > budget.limit-budget.used {
		return metadataTooLargef("%s would use %d of %d bytes", label, budget.used+length, budget.limit)
	}
	budget.used += length
	return nil
}

func unsafeMetadataf(format string, arguments ...interface{}) error {
	return fmt.Errorf("%w: %s", ErrUnsafeMetadata, fmt.Sprintf(format, arguments...))
}

func metadataTooLargef(format string, arguments ...interface{}) error {
	return fmt.Errorf("%w: %s", ErrMetadataTooLarge, fmt.Sprintf(format, arguments...))
}

type metadataCursor struct {
	ctx    context.Context
	reader io.ReaderAt
	pos    int64
	end    int64
}

func newMetadataCursor(ctx context.Context, reader io.ReaderAt, start int64, end int64) (metadataCursor, error) {
	if start < 0 || end < start {
		return metadataCursor{}, unsafeMetadataf("invalid container range %d..%d", start, end)
	}
	return metadataCursor{ctx: ctx, reader: reader, pos: start, end: end}, nil
}

func (cursor *metadataCursor) remaining() int64 {
	return cursor.end - cursor.pos
}

func (cursor *metadataCursor) read(destination []byte) error {
	if err := cursor.ctx.Err(); err != nil {
		return err
	}
	length := int64(len(destination))
	if length < 0 || length > cursor.remaining() {
		return unsafeMetadataf("read of %d bytes exceeds container remainder %d", length, cursor.remaining())
	}
	if len(destination) == 0 {
		return nil
	}
	read, err := cursor.reader.ReadAt(destination, cursor.pos)
	if read != len(destination) {
		return unsafeMetadataf("declared metadata exceeds readable file bytes")
	}
	if err != nil && err != io.EOF {
		return fmt.Errorf("read metadata header: %w", err)
	}
	cursor.pos += length
	return nil
}

func (cursor *metadataCursor) readByte() (byte, error) {
	var value [1]byte
	if err := cursor.read(value[:]); err != nil {
		return 0, err
	}
	return value[0], nil
}

func (cursor *metadataCursor) skip(length int64) error {
	if err := cursor.ctx.Err(); err != nil {
		return err
	}
	if length < 0 || length > cursor.remaining() {
		return unsafeMetadataf("declared length %d exceeds container remainder %d", length, cursor.remaining())
	}
	cursor.pos += length
	return nil
}

func (cursor *metadataCursor) take(length int64) (metadataCursor, error) {
	if length < 0 || length > cursor.remaining() {
		return metadataCursor{}, unsafeMetadataf("declared length %d exceeds container remainder %d", length, cursor.remaining())
	}
	child, err := newMetadataCursor(cursor.ctx, cursor.reader, cursor.pos, cursor.pos+length)
	if err != nil {
		return metadataCursor{}, err
	}
	cursor.pos += length
	return child, nil
}

func (cursor *metadataCursor) uint24BigEndian() (uint32, error) {
	var value [3]byte
	if err := cursor.read(value[:]); err != nil {
		return 0, err
	}
	return uint32(value[0])<<16 | uint32(value[1])<<8 | uint32(value[2]), nil
}

func (cursor *metadataCursor) uint32BigEndian() (uint32, error) {
	var value [4]byte
	if err := cursor.read(value[:]); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(value[:]), nil
}

func (cursor *metadataCursor) uint32LittleEndian() (uint32, error) {
	var value [4]byte
	if err := cursor.read(value[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(value[:]), nil
}

func (cursor *metadataCursor) uint64LittleEndian() (uint64, error) {
	var value [8]byte
	if err := cursor.read(value[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(value[:]), nil
}

func validateMetadataBeforeTag(ctx context.Context, reader io.ReaderAt, fileSize int64, maxMetadataBytes int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if fileSize < 0 {
		return unsafeMetadataf("negative media file size")
	}
	if maxMetadataBytes <= 0 {
		maxMetadataBytes = defaultMaxMetadataBytes
	}
	if maxMetadataBytes > hardMaxMetadataBytes {
		maxMetadataBytes = hardMaxMetadataBytes
	}
	if fileSize < 4 {
		return nil
	}

	probeLength := int64(11)
	if fileSize < probeLength {
		probeLength = fileSize
	}
	probe := make([]byte, probeLength)
	if _, err := reader.ReadAt(probe, 0); err != nil && err != io.EOF {
		return fmt.Errorf("read media metadata probe: %w", err)
	}
	budget := &metadataBudget{limit: maxMetadataBytes}
	switch {
	case len(probe) >= 4 && string(probe[:4]) == "fLaC":
		return validateFLACMetadata(ctx, reader, fileSize, budget)
	case len(probe) >= 4 && string(probe[:4]) == "OggS":
		return validateOGGMetadata(ctx, reader, fileSize, budget)
	case len(probe) >= 8 && string(probe[4:8]) == "ftyp":
		return validateMP4Metadata(ctx, reader, fileSize, budget)
	case len(probe) >= 3 && string(probe[:3]) == "ID3":
		return validateID3Metadata(ctx, reader, 0, fileSize, budget)
	case len(probe) >= 4 && string(probe[:4]) == "DSD ":
		return validateDSFMetadata(ctx, reader, fileSize, budget)
	default:
		// The fallback parser is ID3v1, whose reads and allocations are fixed.
		return nil
	}
}

func decodeSyncSafe(value []byte) (uint32, error) {
	if len(value) != 4 {
		return 0, unsafeMetadataf("syncsafe integer has %d bytes", len(value))
	}
	var decoded uint32
	for _, current := range value {
		if current&0x80 != 0 {
			return 0, unsafeMetadataf("syncsafe integer uses a reserved high bit")
		}
		decoded = decoded<<7 | uint32(current)
	}
	return decoded, nil
}
