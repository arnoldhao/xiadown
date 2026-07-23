package locallyricsreader

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
)

type oggPacketState struct {
	data []byte
}

func validateOGGMetadata(ctx context.Context, reader io.ReaderAt, fileSize int64, budget *metadataBudget) error {
	cursor, err := newMetadataCursor(ctx, reader, 0, fileSize)
	if err != nil {
		return err
	}
	streams := make(map[uint32]*oggPacketState)
	packetLimit := budget.limit / 4
	if packetLimit < 1024 {
		packetLimit = budget.limit
	}
	var totalBuffered int64
	var pageDataBuffer [255 * 255]byte

	for pageIndex := 0; pageIndex < maxOGGPages; pageIndex++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if cursor.remaining() < 27 {
			return unsafeMetadataf("OGG ended before a comment packet")
		}
		var header [27]byte
		if err := cursor.read(header[:]); err != nil {
			return err
		}
		if string(header[:4]) != "OggS" || header[4] != 0 {
			return unsafeMetadataf("invalid OGG page header")
		}
		if header[5]&^byte(0x07) != 0 {
			return unsafeMetadataf("OGG page uses reserved flags 0x%02x", header[5])
		}
		segmentCount := int(header[26])
		var segmentTable [255]byte
		if err := cursor.read(segmentTable[:segmentCount]); err != nil {
			return err
		}
		pageDataLength := 0
		for _, length := range segmentTable[:segmentCount] {
			pageDataLength += int(length)
		}
		if int64(pageDataLength) > cursor.remaining() {
			return unsafeMetadataf("OGG page data length %d exceeds file remainder %d", pageDataLength, cursor.remaining())
		}
		if err := budget.consume(int64(27+segmentCount+pageDataLength), "OGG page buffers"); err != nil {
			return err
		}
		pageData := pageDataBuffer[:pageDataLength]
		if err := cursor.read(pageData); err != nil {
			return err
		}

		serial := binary.LittleEndian.Uint32(header[14:18])
		state, found := streams[serial]
		if !found {
			if len(streams) >= maxOGGStreams {
				return metadataTooLargef("OGG logical stream count exceeds %d", maxOGGStreams)
			}
			state = &oggPacketState{}
			streams[serial] = state
		}
		continued := header[5]&0x01 != 0
		if continued && !found {
			return unsafeMetadataf("OGG page continues an unknown packet")
		}
		if !continued {
			totalBuffered -= int64(len(state.data))
			state.data = nil
		}

		dataOffset := 0
		for _, segmentLengthByte := range segmentTable[:segmentCount] {
			segmentLength := int(segmentLengthByte)
			if int64(len(state.data)+segmentLength) > packetLimit {
				return metadataTooLargef("OGG packet exceeds %d-byte bounded packet limit", packetLimit)
			}
			// bytes.Buffer growth in dhowden/tag (and append growth here) may hold
			// both the previous and next backing arrays during expansion.
			if err := budget.consume(int64(segmentLength)*2, "OGG packet assembly"); err != nil {
				return err
			}
			state.data = append(state.data, pageData[dataOffset:dataOffset+segmentLength]...)
			totalBuffered += int64(segmentLength)
			if totalBuffered > packetLimit {
				return metadataTooLargef("OGG buffered packets exceed %d bytes", packetLimit)
			}
			dataOffset += segmentLength
			if segmentLengthByte < 255 {
				packet := state.data
				totalBuffered -= int64(len(packet))
				state.data = nil
				prefixLength := 0
				switch {
				case bytes.HasPrefix(packet, []byte{0x03, 'v', 'o', 'r', 'b', 'i', 's'}):
					prefixLength = 7
				case bytes.HasPrefix(packet, []byte("OpusTags")):
					prefixLength = 8
				}
				if prefixLength > 0 {
					packetCursor, err := newMetadataCursor(ctx, bytes.NewReader(packet), int64(prefixLength), int64(len(packet)))
					if err != nil {
						return err
					}
					return validateVorbisComments(&packetCursor, budget, 2, false)
				}
			}
		}
	}
	return metadataTooLargef("OGG comment packet not found within %d pages", maxOGGPages)
}
