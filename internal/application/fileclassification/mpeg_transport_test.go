package fileclassification

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestLooksLikeMPEGTransportStreamRejectsTypeScript(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.ts")
	if err := os.WriteFile(
		path,
		[]byte("export const test: string = 'not a video';\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if LooksLikeMPEGTransportStream(path) {
		t.Fatal("TypeScript source was identified as an MPEG transport stream")
	}
}

func TestLooksLikeMPEGTransportStreamRecognizesPacketLayouts(t *testing.T) {
	for _, packetSize := range []int{188, 192, 204} {
		t.Run(strconv.Itoa(packetSize), func(t *testing.T) {
			offset := 0
			if packetSize == 192 {
				offset = 4
			}
			body := make([]byte, offset+packetSize*4)
			for packet := 0; packet < 4; packet++ {
				position := offset + packet*packetSize
				body[position] = 0x47
				body[position+1] = 0x40
				body[position+3] = 0x10
			}
			path := filepath.Join(t.TempDir(), "video.ts")
			if err := os.WriteFile(path, body, 0o600); err != nil {
				t.Fatal(err)
			}
			if !LooksLikeMPEGTransportStream(path) {
				t.Fatalf("%d-byte MPEG transport packets were not recognized", packetSize)
			}
		})
	}
}
