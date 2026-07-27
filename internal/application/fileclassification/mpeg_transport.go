package fileclassification

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const transportProbeBytes = 64 * 1024

// IsAmbiguousMPEGTransportPath reports extensions that are used both by MPEG
// transport streams and TypeScript source files.
func IsAmbiguousMPEGTransportPath(path string) bool {
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(path))) {
	case ".ts", ".mts":
		return true
	default:
		return false
	}
}

// LooksLikeMPEGTransportStream validates repeated packet sync bytes instead of
// trusting the ambiguous .ts/.mts suffix. MPEG-TS commonly uses 188-byte
// packets, while M2TS and protected variants use 192- and 204-byte packets.
func LooksLikeMPEGTransportStream(path string) bool {
	file, err := os.Open(filepath.Clean(strings.TrimSpace(path)))
	if err != nil {
		return false
	}
	defer file.Close()

	buffer := make([]byte, transportProbeBytes)
	read, readErr := io.ReadFull(file, buffer)
	if readErr != nil && !errors.Is(readErr, io.EOF) &&
		!errors.Is(readErr, io.ErrUnexpectedEOF) {
		return false
	}
	buffer = buffer[:read]
	for _, packetSize := range []int{188, 192, 204} {
		if hasTransportPacketRun(buffer, packetSize) {
			return true
		}
	}
	return false
}

func hasTransportPacketRun(buffer []byte, packetSize int) bool {
	const requiredPackets = 4
	if len(buffer) < packetSize*(requiredPackets-1)+4 {
		return false
	}
	maxOffset := min(packetSize, len(buffer)-packetSize*(requiredPackets-1)-3)
	for offset := 0; offset < maxOffset; offset++ {
		valid := true
		for packet := 0; packet < requiredPackets; packet++ {
			position := offset + packet*packetSize
			// 0x47 is the MPEG-TS sync byte. Adaptation-field-control may not
			// be zero in a valid packet header.
			if buffer[position] != 0x47 ||
				buffer[position+3]&0x30 == 0 {
				valid = false
				break
			}
		}
		if valid {
			return true
		}
	}
	return false
}
