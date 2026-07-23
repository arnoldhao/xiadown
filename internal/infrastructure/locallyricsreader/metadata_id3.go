package locallyricsreader

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"strings"
)

type id3LogicalStream struct {
	raw           *metadataCursor
	unsynchronise bool
	previousFF    bool
	buffer        [32 << 10]byte
	bufferOffset  int
	bufferLength  int
}

func (stream *id3LogicalStream) read(destination []byte) error {
	if !stream.unsynchronise {
		return stream.raw.read(destination)
	}
	for index := range destination {
		value, err := stream.readByte()
		if err != nil {
			return err
		}
		destination[index] = value
	}
	return nil
}

func (stream *id3LogicalStream) readByte() (byte, error) {
	if !stream.unsynchronise {
		return stream.raw.readByte()
	}
	for {
		value, err := stream.readRawByte()
		if err != nil {
			return 0, err
		}
		if stream.previousFF && value == 0 {
			stream.previousFF = false
			continue
		}
		stream.previousFF = value == 0xff
		return value, nil
	}
}

func (stream *id3LogicalStream) readRawByte() (byte, error) {
	if stream.bufferOffset >= stream.bufferLength {
		length := stream.raw.remaining()
		if length <= 0 {
			return 0, unsafeMetadataf("ID3 logical stream exceeds its physical tag container")
		}
		if length > int64(len(stream.buffer)) {
			length = int64(len(stream.buffer))
		}
		if err := stream.raw.read(stream.buffer[:length]); err != nil {
			return 0, err
		}
		stream.bufferOffset = 0
		stream.bufferLength = int(length)
	}
	value := stream.buffer[stream.bufferOffset]
	stream.bufferOffset++
	return value, nil
}

func (stream *id3LogicalStream) skip(length int64) error {
	if length < 0 {
		return unsafeMetadataf("negative ID3 frame length")
	}
	if !stream.unsynchronise {
		return stream.raw.skip(length)
	}
	for index := int64(0); index < length; index++ {
		if _, err := stream.readByte(); err != nil {
			return err
		}
	}
	return nil
}

func (stream *id3LogicalStream) physicalRemaining() int64 {
	return stream.raw.remaining() + int64(stream.bufferLength-stream.bufferOffset)
}

type id3FrameStream struct {
	stream    *id3LogicalStream
	remaining int64
}

func (frame *id3FrameStream) readByte() (byte, error) {
	if frame.remaining <= 0 {
		return 0, unsafeMetadataf("ID3 frame field exceeds declared frame length")
	}
	value, err := frame.stream.readByte()
	if err != nil {
		return 0, err
	}
	frame.remaining--
	return value, nil
}

func (frame *id3FrameStream) skip(length int64) error {
	if length < 0 || length > frame.remaining {
		return unsafeMetadataf("ID3 frame field length %d exceeds remainder %d", length, frame.remaining)
	}
	if err := frame.stream.skip(length); err != nil {
		return err
	}
	frame.remaining -= length
	return nil
}

func validateID3Metadata(ctx context.Context, reader io.ReaderAt, start int64, fileEnd int64, budget *metadataBudget) error {
	if start < 0 || fileEnd-start < 10 {
		return unsafeMetadataf("truncated ID3 header")
	}
	headerCursor, err := newMetadataCursor(ctx, reader, start, start+10)
	if err != nil {
		return err
	}
	var header [10]byte
	if err := headerCursor.read(header[:]); err != nil {
		return err
	}
	if string(header[:3]) != "ID3" {
		return unsafeMetadataf("DSF metadata pointer does not reference ID3")
	}
	version := header[3]
	flags := header[5]
	switch version {
	case 2:
		if flags&0x3f != 0 || flags&0x40 != 0 {
			return unsafeMetadataf("unsupported ID3v2.2 flags 0x%02x", flags)
		}
	case 3:
		if flags&0x1f != 0 {
			return unsafeMetadataf("reserved ID3v2.3 flags 0x%02x", flags)
		}
	case 4:
		if flags&0x0f != 0 || flags&0x10 != 0 {
			return unsafeMetadataf("unsupported ID3v2.4 flags 0x%02x", flags)
		}
	default:
		return unsafeMetadataf("unsupported ID3 version %d", version)
	}
	tagSizeValue, err := decodeSyncSafe(header[6:10])
	if err != nil {
		return err
	}
	tagSize := int64(tagSizeValue)
	if tagSize > fileEnd-start-10 {
		return unsafeMetadataf("ID3 tag length %d exceeds file/container remainder %d", tagSize, fileEnd-start-10)
	}
	if err := budget.consume(tagSize, "ID3 tag"); err != nil {
		return err
	}
	payload, err := newMetadataCursor(ctx, reader, start+10, start+10+tagSize)
	if err != nil {
		return err
	}

	if flags&0x40 != 0 {
		if version == 2 {
			return unsafeMetadataf("ID3v2.2 compression is unsupported")
		}
		var lengthBytes [4]byte
		if err := payload.read(lengthBytes[:]); err != nil {
			return err
		}
		var extendedLength int64
		if version == 3 {
			extendedLength = int64(binary.BigEndian.Uint32(lengthBytes[:]))
		} else {
			decoded, err := decodeSyncSafe(lengthBytes[:])
			if err != nil {
				return err
			}
			if decoded < 4 {
				return unsafeMetadataf("ID3v2.4 extended header length %d is smaller than its header", decoded)
			}
			extendedLength = int64(decoded - 4)
		}
		if err := budget.require(extendedLength, "ID3 extended header"); err != nil {
			return err
		}
		if err := payload.skip(extendedLength); err != nil {
			return err
		}
	}

	stream := &id3LogicalStream{raw: &payload, unsynchronise: flags&0x80 != 0}
	frames := 0
	frameOccurrences := make(map[string]int)
	for stream.physicalRemaining() > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		frames++
		if frames > maxID3Frames {
			return metadataTooLargef("ID3 frame count exceeds %d", maxID3Frames)
		}

		nameLength := 4
		headerLength := int64(10)
		if version == 2 {
			nameLength = 3
			headerLength = 6
		}
		if stream.physicalRemaining() < headerLength {
			return unsafeMetadataf("truncated ID3 frame header")
		}
		nameBytes := make([]byte, nameLength)
		if err := stream.read(nameBytes); err != nil {
			return err
		}

		var frameSize int64
		var frameFlags [2]byte
		if version == 2 {
			var sizeBytes [3]byte
			if err := stream.read(sizeBytes[:]); err != nil {
				return err
			}
			frameSize = int64(uint32(sizeBytes[0])<<16 | uint32(sizeBytes[1])<<8 | uint32(sizeBytes[2]))
		} else {
			var sizeBytes [4]byte
			if err := stream.read(sizeBytes[:]); err != nil {
				return err
			}
			if version == 4 {
				decoded, err := decodeSyncSafe(sizeBytes[:])
				if err != nil {
					return err
				}
				frameSize = int64(decoded)
			} else {
				frameSize = int64(binary.BigEndian.Uint32(sizeBytes[:]))
			}
			if err := stream.read(frameFlags[:]); err != nil {
				return err
			}
		}

		if frameSize == 0 {
			return nil
		}
		if bytes.IndexFunc(nameBytes, func(value rune) bool {
			return !(value >= 'A' && value <= 'Z') && !(value >= '0' && value <= '9')
		}) >= 0 {
			return unsafeMetadataf("invalid ID3 frame identifier %q", nameBytes)
		}
		frameName := string(nameBytes)
		occurrences := frameOccurrences[frameName]
		if occurrences >= maxID3FramesPerID {
			return metadataTooLargef("ID3 frame %q occurs more than %d times", frameName, maxID3FramesPerID)
		}
		// dhowden/tag stores duplicate frame names by probing NAME_0, NAME_1,
		// and so on from the beginning for every occurrence. Charge the
		// incremental linear probe and map-entry overhead so the resulting
		// triangular allocation cost cannot bypass MaxMetadataBytes.
		probeCost := int64(occurrences) * id3NameProbeBytes
		if err := budget.consume(id3FrameMapBytes+probeCost, "ID3 frame index"); err != nil {
			return err
		}
		frameOccurrences[frameName] = occurrences + 1
		if err := budget.require(frameSize, "ID3 frame"); err != nil {
			return err
		}
		if frameSize > stream.physicalRemaining() {
			return unsafeMetadataf("ID3 frame %q length %d exceeds physical tag remainder %d", nameBytes, frameSize, stream.physicalRemaining())
		}

		originalFrameSize := frameSize
		compression := version > 2 && frameFlags[1]&0x80 != 0
		encryption := version > 2 && frameFlags[1]&0x40 != 0
		dataLengthIndicator := version == 4 && frameFlags[1]&0x01 != 0
		if version == 4 {
			compression = frameFlags[1]&0x08 != 0
			encryption = frameFlags[1]&0x04 != 0
		}

		if version == 3 && compression {
			if frameSize < 4 {
				return unsafeMetadataf("compressed ID3v2.3 frame %q is shorter than its length indicator", nameBytes)
			}
			var indicator [4]byte
			if err := stream.read(indicator[:]); err != nil {
				return err
			}
			declared := int64(uint32(indicator[0])<<21 | uint32(indicator[1])<<14 | uint32(indicator[2])<<7 | uint32(indicator[3]))
			if err := budget.require(declared, "ID3v2.3 data length indicator"); err != nil {
				return err
			}
			frameSize -= 4
		}
		if version == 4 {
			if compression && !dataLengthIndicator {
				return unsafeMetadataf("compressed ID3v2.4 frame %q omits its data length indicator", nameBytes)
			}
			if dataLengthIndicator {
				if originalFrameSize < 4 {
					return unsafeMetadataf("ID3v2.4 frame %q is shorter than its data length indicator", nameBytes)
				}
				var indicator [4]byte
				if err := stream.read(indicator[:]); err != nil {
					return err
				}
				decoded, err := decodeSyncSafe(indicator[:])
				if err != nil {
					return err
				}
				frameSize = int64(decoded)
				if err := budget.require(frameSize, "ID3v2.4 data length indicator"); err != nil {
					return err
				}
				// dhowden/tag uses the DLI as the number of subsequent file bytes
				// rather than as an uncompressed size. Equality is required so its
				// read cannot escape the declared frame or misalign the next header.
				if frameSize != originalFrameSize-4 {
					return unsafeMetadataf("ID3v2.4 frame %q DLI %d does not match contained bytes %d", nameBytes, frameSize, originalFrameSize-4)
				}
			}
		}
		if encryption {
			if frameSize < 1 {
				return unsafeMetadataf("encrypted ID3 frame %q omits its method byte", nameBytes)
			}
			if _, err := stream.readByte(); err != nil {
				return err
			}
			frameSize--
		}
		if frameSize > stream.physicalRemaining() {
			return unsafeMetadataf("ID3 frame %q effective length %d exceeds tag remainder %d", nameBytes, frameSize, stream.physicalRemaining())
		}
		if err := validateID3FramePayload(frameName, stream, frameSize, budget); err != nil {
			return err
		}
	}
	return nil
}

func validateID3FramePayload(name string, stream *id3LogicalStream, length int64, budget *metadataBudget) error {
	frame := &id3FrameStream{stream: stream, remaining: length}
	if isID3TextFrame(name) {
		if err := budget.consume(length*id3TextExtraMultiplier(name), "ID3 decoded text"); err != nil {
			return err
		}
		return frame.skip(frame.remaining)
	}
	if name == "UFID" || name == "UFI" {
		if err := budget.consume(length, "ID3 UFID string"); err != nil {
			return err
		}
		return frame.skip(frame.remaining)
	}
	if name == "APIC" {
		return validateAPICFrame(frame, budget)
	}
	if name == "PIC" {
		return validatePICFrame(frame, budget)
	}
	return frame.skip(frame.remaining)
}

func id3TextExtraMultiplier(name string) int64 {
	// Ordinary T/W frames run strings.Split over every NUL. A frame made of
	// alternating bytes and NULs can therefore allocate a large []string even
	// though its declared byte length is modest. Described-text frames use only
	// SplitN(2) and have a substantially smaller bound.
	switch name {
	case "TXXX", "TXX", "WXXX", "WXX", "COMM", "COM", "USLT", "ULT":
		return 6
	}
	if strings.HasPrefix(name, "W") {
		return 20
	}
	return 19
}

func isID3TextFrame(name string) bool {
	return strings.HasPrefix(name, "T") || strings.HasPrefix(name, "W") ||
		name == "COMM" || name == "COM" || name == "USLT" || name == "ULT"
}

func validateAPICFrame(frame *id3FrameStream, budget *metadataBudget) error {
	encoding, err := frame.readByte()
	if err != nil {
		return err
	}
	mimeLength, err := scanID3Terminator(frame, false)
	if err != nil {
		return err
	}
	if mimeLength > 1024 {
		return unsafeMetadataf("ID3 APIC MIME type is unreasonably long")
	}
	if _, err := frame.readByte(); err != nil { // picture type
		return err
	}
	descriptionLength, err := scanID3Terminator(frame, encoding == 1 || encoding == 2)
	if err != nil {
		return err
	}
	if err := budget.consume(mimeLength+descriptionLength*6, "ID3 picture strings"); err != nil {
		return err
	}
	return frame.skip(frame.remaining)
}

func validatePICFrame(frame *id3FrameStream, budget *metadataBudget) error {
	encoding, err := frame.readByte()
	if err != nil {
		return err
	}
	if err := frame.skip(4); err != nil { // three-byte format and picture type
		return err
	}
	descriptionLength, err := scanID3Terminator(frame, encoding == 1 || encoding == 2)
	if err != nil {
		return err
	}
	if err := budget.consume(descriptionLength*6, "ID3 picture description"); err != nil {
		return err
	}
	return frame.skip(frame.remaining)
}

func scanID3Terminator(frame *id3FrameStream, doubleZero bool) (int64, error) {
	var length int64
	previousZero := false
	for frame.remaining > 0 {
		value, err := frame.readByte()
		if err != nil {
			return 0, err
		}
		if !doubleZero {
			if value == 0 {
				return length, nil
			}
			length++
			continue
		}
		if previousZero && value == 0 {
			if length > 0 {
				length--
			}
			return length, nil
		}
		previousZero = value == 0
		length++
	}
	return 0, unsafeMetadataf("ID3 text field lacks its terminator")
}

func validateDSFMetadata(ctx context.Context, reader io.ReaderAt, fileSize int64, budget *metadataBudget) error {
	if fileSize < 28 {
		return unsafeMetadataf("truncated DSF header")
	}
	cursor, err := newMetadataCursor(ctx, reader, 0, 28)
	if err != nil {
		return err
	}
	var magic [4]byte
	if err := cursor.read(magic[:]); err != nil {
		return err
	}
	if string(magic[:]) != "DSD " {
		return unsafeMetadataf("invalid DSF signature")
	}
	chunkSize, err := cursor.uint64LittleEndian()
	if err != nil {
		return err
	}
	declaredFileSize, err := cursor.uint64LittleEndian()
	if err != nil {
		return err
	}
	id3Pointer, err := cursor.uint64LittleEndian()
	if err != nil {
		return err
	}
	if chunkSize != 28 {
		return unsafeMetadataf("DSF header chunk length is %d, want 28", chunkSize)
	}
	if declaredFileSize != uint64(fileSize) {
		return unsafeMetadataf("DSF declared file length %d does not match %d", declaredFileSize, fileSize)
	}
	if id3Pointer == 0 {
		return nil
	}
	if id3Pointer < chunkSize || id3Pointer > uint64(fileSize-10) {
		return unsafeMetadataf("DSF ID3 pointer %d escapes file length %d", id3Pointer, fileSize)
	}
	return validateID3Metadata(ctx, reader, int64(id3Pointer), fileSize, budget)
}
