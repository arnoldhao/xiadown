package locallyricsreader

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
)

func validateFLACMetadata(ctx context.Context, reader io.ReaderAt, fileSize int64, budget *metadataBudget) error {
	if fileSize < 8 {
		return unsafeMetadataf("truncated FLAC metadata")
	}
	cursor, err := newMetadataCursor(ctx, reader, 0, fileSize)
	if err != nil {
		return err
	}
	var magic [4]byte
	if err := cursor.read(magic[:]); err != nil {
		return err
	}
	if string(magic[:]) != "fLaC" {
		return unsafeMetadataf("invalid FLAC signature")
	}
	for blockIndex := 0; blockIndex < maxFLACBlocks; blockIndex++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		blockType, err := cursor.readByte()
		if err != nil {
			return err
		}
		blockLengthValue, err := cursor.uint24BigEndian()
		if err != nil {
			return err
		}
		blockLength := int64(blockLengthValue)
		if err := budget.consume(blockLength+4, "FLAC metadata block"); err != nil {
			return err
		}
		block, err := cursor.take(blockLength)
		if err != nil {
			return err
		}
		switch blockType & 0x7f {
		case 4:
			if err := validateVorbisComments(&block, budget, 1, true); err != nil {
				return fmt.Errorf("validate FLAC Vorbis comment: %w", err)
			}
		case 6:
			if err := validateFLACPicture(&block, budget); err != nil {
				return fmt.Errorf("validate FLAC picture: %w", err)
			}
		}
		if blockType&0x80 != 0 {
			return nil
		}
	}
	return metadataTooLargef("FLAC metadata block count exceeds %d", maxFLACBlocks)
}

// validateVorbisComments validates declarations without materialising comment
// values. extraStringCopies accounts for allocations performed after the
// enclosing container has already been charged to the budget.
func validateVorbisComments(cursor *metadataCursor, budget *metadataBudget, extraStringCopies int64, requireExact bool) error {
	vendorLengthValue, err := cursor.uint32LittleEndian()
	if err != nil {
		return err
	}
	vendorLength := int64(vendorLengthValue)
	if err := budget.require(vendorLength, "Vorbis vendor string"); err != nil {
		return err
	}
	if vendorLength > cursor.remaining() {
		return unsafeMetadataf("Vorbis vendor length %d exceeds packet/block remainder %d", vendorLength, cursor.remaining())
	}
	if err := budget.consume(vendorLength*extraStringCopies, "Vorbis vendor allocations"); err != nil {
		return err
	}
	if err := cursor.skip(vendorLength); err != nil {
		return err
	}
	commentCountValue, err := cursor.uint32LittleEndian()
	if err != nil {
		return err
	}
	if commentCountValue > maxVorbisComments {
		return metadataTooLargef("Vorbis comment count %d exceeds %d", commentCountValue, maxVorbisComments)
	}
	if int64(commentCountValue)*4 > cursor.remaining() {
		return unsafeMetadataf("Vorbis comment count cannot fit in container remainder")
	}
	if err := budget.consume(int64(commentCountValue)*96, "Vorbis comment map entries"); err != nil {
		return err
	}

	for index := uint32(0); index < commentCountValue; index++ {
		commentLengthValue, err := cursor.uint32LittleEndian()
		if err != nil {
			return err
		}
		commentLength := int64(commentLengthValue)
		if err := budget.require(commentLength, "Vorbis comment"); err != nil {
			return err
		}
		comment, err := cursor.take(commentLength)
		if err != nil {
			return err
		}
		if err := budget.consume(commentLength*extraStringCopies, "Vorbis comment string allocations"); err != nil {
			return err
		}
		key, keyLength, value, err := splitVorbisComment(&comment)
		if err != nil {
			return err
		}
		if keyLength == 0 || keyLength > 128 {
			return unsafeMetadataf("Vorbis comment key length %d is outside 1..128", keyLength)
		}
		if err := budget.consume(keyLength, "Vorbis lowercase map key"); err != nil {
			return err
		}
		if strings.EqualFold(key, "metadata_block_picture") {
			if err := validateBase64Picture(value, budget); err != nil {
				return fmt.Errorf("validate METADATA_BLOCK_PICTURE: %w", err)
			}
		}
	}
	if requireExact && cursor.remaining() != 0 {
		return unsafeMetadataf("Vorbis comment leaves %d undeclared bytes in its FLAC block", cursor.remaining())
	}
	return nil
}

func splitVorbisComment(comment *metadataCursor) (string, int64, metadataCursor, error) {
	const retainedKeyLimit = 128
	keyBytes := make([]byte, 0, retainedKeyLimit)
	var keyLength int64
	for comment.remaining() > 0 {
		value, err := comment.readByte()
		if err != nil {
			return "", 0, metadataCursor{}, err
		}
		if value == '=' {
			valueCursor, err := comment.take(comment.remaining())
			return string(keyBytes), keyLength, valueCursor, err
		}
		keyLength++
		if len(keyBytes) < retainedKeyLimit {
			keyBytes = append(keyBytes, value)
		}
	}
	return "", 0, metadataCursor{}, unsafeMetadataf("Vorbis comment lacks '=' separator")
}

func validateFLACPicture(cursor *metadataCursor, budget *metadataBudget) error {
	pictureType, err := cursor.uint32BigEndian()
	if err != nil {
		return err
	}
	if pictureType > 20 {
		return unsafeMetadataf("FLAC picture type %d is invalid", pictureType)
	}
	mimeLength, err := cursor.uint32BigEndian()
	if err != nil {
		return err
	}
	if err := validateAndSkipPictureField(cursor, budget, int64(mimeLength), "FLAC picture MIME"); err != nil {
		return err
	}
	descriptionLength, err := cursor.uint32BigEndian()
	if err != nil {
		return err
	}
	if err := validateAndSkipPictureField(cursor, budget, int64(descriptionLength), "FLAC picture description"); err != nil {
		return err
	}
	if err := cursor.skip(16); err != nil { // width, height, depth, palette count
		return err
	}
	dataLength, err := cursor.uint32BigEndian()
	if err != nil {
		return err
	}
	if err := budget.require(int64(dataLength), "FLAC picture data"); err != nil {
		return err
	}
	if err := cursor.skip(int64(dataLength)); err != nil {
		return err
	}
	if cursor.remaining() != 0 {
		return unsafeMetadataf("FLAC picture leaves %d bytes outside declared fields", cursor.remaining())
	}
	return nil
}

func validateAndSkipPictureField(cursor *metadataCursor, budget *metadataBudget, length int64, label string) error {
	if err := budget.require(length, label); err != nil {
		return err
	}
	if err := budget.consume(length, label+" string allocation"); err != nil {
		return err
	}
	return cursor.skip(length)
}

type decodedPictureStream struct {
	ctx      context.Context
	reader   io.Reader
	consumed int64
	upper    int64
}

func (stream *decodedPictureStream) read(destination []byte) error {
	if err := stream.ctx.Err(); err != nil {
		return err
	}
	if int64(len(destination)) > stream.upper-stream.consumed {
		return unsafeMetadataf("decoded picture declaration exceeds base64 container")
	}
	if _, err := io.ReadFull(stream.reader, destination); err != nil {
		return unsafeMetadataf("truncated or invalid base64 picture: %v", err)
	}
	stream.consumed += int64(len(destination))
	return nil
}

func (stream *decodedPictureStream) uint32BigEndian() (uint32, error) {
	var value [4]byte
	if err := stream.read(value[:]); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(value[:]), nil
}

func (stream *decodedPictureStream) skip(length int64) error {
	if length < 0 || length > stream.upper-stream.consumed {
		return unsafeMetadataf("decoded picture length %d exceeds remaining upper bound %d", length, stream.upper-stream.consumed)
	}
	buffer := make([]byte, 32<<10)
	for length > 0 {
		chunk := int64(len(buffer))
		if chunk > length {
			chunk = length
		}
		if err := stream.read(buffer[:chunk]); err != nil {
			return err
		}
		length -= chunk
	}
	return nil
}

func validateBase64Picture(encoded metadataCursor, budget *metadataBudget) error {
	encodedLength := encoded.remaining()
	if err := budget.require(encodedLength, "base64 picture text"); err != nil {
		return err
	}
	decodedUpper := int64(base64.StdEncoding.DecodedLen(int(encodedLength)))
	if err := budget.consume(decodedUpper, "base64 decoded picture buffer"); err != nil {
		return err
	}
	section := io.NewSectionReader(encoded.reader, encoded.pos, encodedLength)
	decoder := base64.NewDecoder(base64.StdEncoding, section)
	stream := &decodedPictureStream{ctx: encoded.ctx, reader: decoder, upper: decodedUpper}
	pictureType, err := stream.uint32BigEndian()
	if err != nil {
		return err
	}
	if pictureType > 20 {
		return unsafeMetadataf("base64 picture type %d is invalid", pictureType)
	}
	mimeLength, err := stream.uint32BigEndian()
	if err != nil {
		return err
	}
	if err := budget.require(int64(mimeLength), "base64 picture MIME"); err != nil {
		return err
	}
	if err := budget.consume(int64(mimeLength)*2, "base64 picture MIME allocations"); err != nil {
		return err
	}
	if err := stream.skip(int64(mimeLength)); err != nil {
		return err
	}
	descriptionLength, err := stream.uint32BigEndian()
	if err != nil {
		return err
	}
	if err := budget.require(int64(descriptionLength), "base64 picture description"); err != nil {
		return err
	}
	if err := budget.consume(int64(descriptionLength)*2, "base64 picture description allocations"); err != nil {
		return err
	}
	if err := stream.skip(int64(descriptionLength)); err != nil {
		return err
	}
	if err := stream.skip(16); err != nil {
		return err
	}
	dataLength, err := stream.uint32BigEndian()
	if err != nil {
		return err
	}
	if err := budget.require(int64(dataLength), "base64 picture data"); err != nil {
		return err
	}
	if err := budget.consume(int64(dataLength), "base64 picture data copy"); err != nil {
		return err
	}
	if err := stream.skip(int64(dataLength)); err != nil {
		return err
	}
	var extra [1]byte
	read, readErr := decoder.Read(extra[:])
	if read > 0 {
		return unsafeMetadataf("base64 picture contains trailing decoded bytes")
	}
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return unsafeMetadataf("invalid base64 picture tail: %v", readErr)
	}
	return nil
}
