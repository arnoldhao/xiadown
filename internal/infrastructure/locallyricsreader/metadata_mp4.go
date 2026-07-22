package locallyricsreader

import (
	"context"
	"io"
)

type mp4ValidationState struct {
	atoms int
}

func validateMP4Metadata(ctx context.Context, reader io.ReaderAt, fileSize int64, budget *metadataBudget) error {
	if fileSize < 8 {
		return unsafeMetadataf("truncated MP4 atom header")
	}
	cursor, err := newMetadataCursor(ctx, reader, 0, fileSize)
	if err != nil {
		return err
	}
	state := &mp4ValidationState{}
	return validateMP4AtomRange(&cursor, budget, state, 0, true)
}

func validateMP4AtomRange(cursor *metadataCursor, budget *metadataBudget, state *mp4ValidationState, depth int, topLevel bool) error {
	if depth > maxMP4Depth {
		return metadataTooLargef("MP4 atom nesting exceeds depth %d", maxMP4Depth)
	}
	first := true
	for cursor.remaining() > 0 {
		if err := cursor.ctx.Err(); err != nil {
			return err
		}
		if cursor.remaining() < 8 {
			return unsafeMetadataf("MP4 container has %d trailing bytes, fewer than an atom header", cursor.remaining())
		}
		state.atoms++
		if state.atoms > maxMP4Atoms {
			return metadataTooLargef("MP4 atom count exceeds %d", maxMP4Atoms)
		}
		if err := budget.consume(32, "MP4 atom bookkeeping"); err != nil {
			return err
		}

		sizeValue, err := cursor.uint32BigEndian()
		if err != nil {
			return err
		}
		var nameBytes [4]byte
		if err := cursor.read(nameBytes[:]); err != nil {
			return err
		}
		name := string(nameBytes[:])
		if topLevel && first && name != "ftyp" {
			return unsafeMetadataf("MP4 first atom is %q, want ftyp", name)
		}
		first = false
		if sizeValue < 8 {
			return unsafeMetadataf("MP4 atom %q has unsupported size %d", name, sizeValue)
		}
		payloadLength := int64(sizeValue) - 8
		if payloadLength > cursor.remaining() {
			return unsafeMetadataf("MP4 atom %q length %d exceeds container remainder %d", name, sizeValue, cursor.remaining()+8)
		}
		if !isMP4MediaPayload(name) {
			if err := budget.require(payloadLength, "MP4 atom "+name); err != nil {
				return err
			}
		}
		payload, err := cursor.take(payloadLength)
		if err != nil {
			return err
		}

		switch name {
		case "moov", "udta", "ilst":
			if err := validateMP4AtomRange(&payload, budget, state, depth+1, false); err != nil {
				return err
			}
		case "meta":
			if payload.remaining() < 4 {
				return unsafeMetadataf("MP4 meta atom omits version/flags")
			}
			if err := payload.skip(4); err != nil {
				return err
			}
			if err := validateMP4AtomRange(&payload, budget, state, depth+1, false); err != nil {
				return err
			}
		case "----":
			if err := validateMP4CustomAtom(&payload, budget, state); err != nil {
				return err
			}
		default:
			if isMP4MetadataAtom(name) {
				if err := validateMP4MetadataAtom(name, &payload, budget); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func isMP4MediaPayload(name string) bool {
	switch name {
	case "mdat", "free", "skip", "wide":
		return true
	default:
		return false
	}
}

func isMP4MetadataAtom(name string) bool {
	switch name {
	case "\xa9alb", "\xa9art", "\xa9ART", "aART", "\xa9day", "\xa9nam", "\xa9gen", "trkn",
		"\xa9wrt", "\xa9too", "cprt", "covr", "\xa9grp", "keyw", "\xa9lyr", "\xa9cmt",
		"tmpo", "cpil", "disk":
		return true
	default:
		return false
	}
}

func validateMP4MetadataAtom(name string, payload *metadataCursor, budget *metadataBudget) error {
	payloadLength := payload.remaining()
	if err := budget.consume(payloadLength, "MP4 metadata atom buffer"); err != nil {
		return err
	}
	if payloadLength < 16 {
		return unsafeMetadataf("MP4 metadata atom %q is shorter than a data atom", name)
	}
	childSize, err := payload.uint32BigEndian()
	if err != nil {
		return err
	}
	var childName [4]byte
	if err := payload.read(childName[:]); err != nil {
		return err
	}
	if string(childName[:]) != "data" || childSize < 16 || int64(childSize) != payloadLength {
		return unsafeMetadataf("MP4 metadata atom %q has an invalid contained data atom", name)
	}
	var versionFlags [4]byte
	if err := payload.read(versionFlags[:]); err != nil {
		return err
	}
	contentClass := uint32(versionFlags[1])<<16 | uint32(versionFlags[2])<<8 | uint32(versionFlags[3])
	if err := payload.skip(4); err != nil { // locale
		return err
	}
	contentLength := payload.remaining()

	switch name {
	case "covr":
		if contentClass != 0 && contentClass != 13 && contentClass != 14 {
			return unsafeMetadataf("MP4 cover atom has unsupported class %d", contentClass)
		}
	case "trkn", "disk":
		if contentLength < 6 {
			return unsafeMetadataf("MP4 %s atom is shorter than six content bytes", name)
		}
	case "tmpo", "cpil":
		if contentClass != 21 || contentLength < 1 {
			return unsafeMetadataf("MP4 integer atom %q has invalid class or length", name)
		}
	default:
		if contentClass != 1 {
			return unsafeMetadataf("MP4 text atom %q has class %d, want 1", name, contentClass)
		}
		if err := budget.consume(contentLength, "MP4 metadata text copy"); err != nil {
			return err
		}
	}
	return payload.skip(contentLength)
}

func validateMP4CustomAtom(payload *metadataCursor, budget *metadataBudget, state *mp4ValidationState) error {
	var dataBytes int64
	for payload.remaining() > 0 {
		if payload.remaining() < 8 {
			return unsafeMetadataf("MP4 custom atom has a truncated sub-atom")
		}
		state.atoms++
		if state.atoms > maxMP4Atoms {
			return metadataTooLargef("MP4 atom count exceeds %d", maxMP4Atoms)
		}
		subSize, err := payload.uint32BigEndian()
		if err != nil {
			return err
		}
		var subNameBytes [4]byte
		if err := payload.read(subNameBytes[:]); err != nil {
			return err
		}
		subName := string(subNameBytes[:])
		if subSize < 8 {
			return unsafeMetadataf("MP4 custom sub-atom %q has size %d", subName, subSize)
		}
		subPayloadLength := int64(subSize) - 8
		if subPayloadLength > payload.remaining() {
			return unsafeMetadataf("MP4 custom sub-atom %q escapes its parent", subName)
		}
		if err := budget.consume(subPayloadLength, "MP4 custom sub-atom buffer"); err != nil {
			return err
		}
		subPayload, err := payload.take(subPayloadLength)
		if err != nil {
			return err
		}
		if subPayloadLength < 4 {
			return unsafeMetadataf("MP4 custom sub-atom %q omits version/flags", subName)
		}
		if err := subPayload.skip(4); err != nil {
			return err
		}
		textLength := subPayload.remaining()
		if subName == "mean" || subName == "name" || subName == "data" {
			if err := budget.consume(textLength, "MP4 custom string copy"); err != nil {
				return err
			}
			if subName == "data" {
				dataBytes += textLength
			}
		}
		if err := subPayload.skip(textLength); err != nil {
			return err
		}
	}
	if dataBytes > 0 {
		if err := budget.consume(dataBytes*2, "MP4 custom joined text"); err != nil {
			return err
		}
	}
	return nil
}
