package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	appytdlp "xiadown/internal/application/ytdlp"
)

func TestPreflightYTDLPStreamReadsMediaPlaylistAndKey(t *testing.T) {
	t.Parallel()

	var sawAccept atomic.Bool
	keyText := "0123456789abcdef"
	segmentBody := encryptHLSProbeFixture(t, []byte(keyText), make([]byte, aes.BlockSize))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") == resourceDefaultAccept {
			sawAccept.Store(true)
		}
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		switch r.URL.Path {
		case "/master.m3u8":
			_, _ = w.Write([]byte(`#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=1000000
/media/stream
`))
		case "/media/stream":
			_, _ = w.Write([]byte(`#EXTM3U
#EXT-X-KEY:METHOD=AES-128,URI="/keys/main"
#EXTINF:4.0,
/segments/abcdef
#EXT-X-ENDLIST
`))
		case "/keys/main":
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write([]byte(keyText))
		case "/segments/abcdef":
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(segmentBody)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	rawURL := server.URL + "/master.m3u8"
	headers := normalizeResourceDownloadHeaders(map[string]string{"Referer": server.URL + "/watch"}, rawURL)
	preflight := (&LibraryService{}).preflightYTDLPStream(context.Background(), rawURL, headers, nil, "op-test")

	if !sawAccept.Load() {
		t.Fatalf("expected preflight requests to include default Accept header")
	}
	if !strings.HasSuffix(preflight.URL, "/media/stream") {
		t.Fatalf("expected media playlist to be analyzed, got %#v", preflight)
	}
	if preflight.EncryptionType != appytdlp.StreamEncryptionAES128 {
		t.Fatalf("expected AES-128 media playlist, got %#v", preflight)
	}
	if preflight.KeyProbe == nil || preflight.KeyProbe.LengthBytes != 16 {
		t.Fatalf("expected raw key probe, got %#v", preflight.KeyProbe)
	}
	if preflight.KeyProbe.NormalizedLengthBytes != 16 || preflight.KeyProbe.NormalizedKeySource != appytdlp.HLSKeyMaterialSourceRaw || !preflight.KeyProbe.DecryptionValidated {
		t.Fatalf("expected validated raw key probe, got %#v", preflight.KeyProbe)
	}
	if preflight.Strategy.Downloader != appytdlp.StreamDownloaderNativeM3U8 {
		t.Fatalf("expected native HLS strategy, got %#v", preflight.Strategy)
	}
	if len(preflight.Strategy.ExtractorArgs) != 0 {
		t.Fatalf("expected standard key to use manifest key URL without hls_key override, got %#v", preflight.Strategy.ExtractorArgs)
	}
}

func TestPreflightYTDLPStreamSelectsASCIIFirst16KeyBySegmentProbe(t *testing.T) {
	t.Parallel()

	keyText := "ba9bf05693b9fa202d922dd43a08f281"
	iv := make([]byte, aes.BlockSize)
	segmentBody := encryptHLSProbeFixture(t, []byte(keyText[:16]), iv)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/stream.m3u8":
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			_, _ = w.Write([]byte(`#EXTM3U
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-KEY:METHOD=AES-128,URI="/key",IV=0x00000000000000000000000000000000
#EXTINF:4.0,
/segment/0
#EXT-X-ENDLIST
`))
		case "/key":
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write([]byte(keyText))
		case "/segment/0":
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(segmentBody)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	rawURL := server.URL + "/stream.m3u8"
	headers := normalizeResourceDownloadHeaders(nil, rawURL)
	preflight := (&LibraryService{}).preflightYTDLPStream(context.Background(), rawURL, headers, nil, "op-test")

	if preflight.KeyProbe == nil {
		t.Fatalf("expected key probe")
	}
	if preflight.KeyProbe.NormalizedKeySource != appytdlp.HLSKeyMaterialSourceASCIIFirst16 {
		t.Fatalf("expected ASCII first16 key source, got %#v", preflight.KeyProbe)
	}
	if preflight.KeyProbe.NormalizedKeyRule != appytdlp.HLSKeyMaterialRuleNonStandardFirst16 ||
		!preflight.KeyProbe.NormalizedKeyNonStandard ||
		!preflight.KeyProbe.ManifestKeyOverride {
		t.Fatalf("expected nonstandard key override metadata, got %#v", preflight.KeyProbe)
	}
	if !preflight.KeyProbe.DecryptionValidated || preflight.KeyProbe.DecryptionValidationFormat != "mpegts" {
		t.Fatalf("expected first segment decryption to be validated, got %#v", preflight.KeyProbe)
	}
	wantKeyHex := hex.EncodeToString([]byte(keyText[:16]))
	if len(preflight.Strategy.ExtractorArgs) != 1 || preflight.Strategy.ExtractorArgs[0] != "generic:hls_key="+wantKeyHex {
		t.Fatalf("expected first16 hls_key extractor arg, got %#v", preflight.Strategy.ExtractorArgs)
	}
}

func TestPreflightYTDLPStreamFailsBeforeDownloadWhenKeyCannotDecryptFirstSegment(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/stream.m3u8":
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			_, _ = w.Write([]byte(`#EXTM3U
#EXT-X-KEY:METHOD=AES-128,URI="/key",IV=0x00000000000000000000000000000000
#EXTINF:4.0,
/segment/0
#EXT-X-ENDLIST
`))
		case "/key":
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write([]byte("0123456789abcdef"))
		case "/segment/0":
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write([]byte("not-encrypted-media"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	rawURL := server.URL + "/stream.m3u8"
	preflight := (&LibraryService{}).preflightYTDLPStream(context.Background(), rawURL, normalizeResourceDownloadHeaders(nil, rawURL), nil, "op-test")

	if !preflight.IsUnsupported() {
		t.Fatalf("expected invalid first segment decryption to fail before download, got %#v", preflight)
	}
	if preflight.KeyProbe == nil || preflight.KeyProbe.DecryptionValidated {
		t.Fatalf("expected failed decryption probe, got %#v", preflight.KeyProbe)
	}
}

func TestPreflightYTDLPStreamRetriesKeyAndSegmentWithManifestQuery(t *testing.T) {
	t.Parallel()

	keyText := "0123456789abcdef"
	segmentBody := encryptHLSProbeFixture(t, []byte(keyText), make([]byte, aes.BlockSize))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/stream.m3u8":
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			_, _ = w.Write([]byte(`#EXTM3U
#EXT-X-KEY:METHOD=AES-128,URI="/key",IV=0x00000000000000000000000000000000
#EXTINF:4.0,
/segment/0
#EXT-X-ENDLIST
`))
		case "/key":
			if r.URL.Query().Get("sig") != "1" {
				http.Error(w, "missing query", http.StatusForbidden)
				return
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write([]byte(keyText))
		case "/segment/0":
			if r.URL.Query().Get("sig") != "1" {
				http.Error(w, "missing query", http.StatusForbidden)
				return
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(segmentBody)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	rawURL := server.URL + "/stream.m3u8?sig=1"
	preflight := (&LibraryService{}).preflightYTDLPStream(context.Background(), rawURL, normalizeResourceDownloadHeaders(nil, rawURL), nil, "op-test")

	if preflight.IsUnsupported() {
		t.Fatalf("expected query retry preflight to be supported, got %#v", preflight)
	}
	if preflight.KeyProbe == nil || !preflight.KeyProbe.KeyQueryPassthrough || !preflight.KeyProbe.FragmentQueryPassthrough {
		t.Fatalf("expected key and fragment query passthrough metadata, got %#v", preflight.KeyProbe)
	}
	args := strings.Join(preflight.Strategy.ExtractorArgs, "\n")
	for _, expected := range []string{"generic:key_query=sig=1", "generic:fragment_query=sig=1"} {
		if !strings.Contains(args, expected) {
			t.Fatalf("expected extractor args to include %q, got %#v", expected, preflight.Strategy.ExtractorArgs)
		}
	}
}

func TestPreflightYTDLPStreamProbesByteRangeSegment(t *testing.T) {
	t.Parallel()

	keyText := "0123456789abcdef"
	segmentBody := encryptHLSProbeFixture(t, []byte(keyText), make([]byte, aes.BlockSize))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/stream.m3u8":
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			_, _ = w.Write([]byte(`#EXTM3U
#EXT-X-KEY:METHOD=AES-128,URI="/key",IV=0x00000000000000000000000000000000
#EXT-X-BYTERANGE:384@188
#EXTINF:4.0,
/media.ts
#EXT-X-ENDLIST
`))
		case "/key":
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write([]byte(keyText))
		case "/media.ts":
			if r.Header.Get("Range") != "bytes=188-699" {
				http.Error(w, "unexpected range", http.StatusRequestedRangeNotSatisfiable)
				return
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Range", "bytes 188-571/572")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(segmentBody)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	rawURL := server.URL + "/stream.m3u8"
	preflight := (&LibraryService{}).preflightYTDLPStream(context.Background(), rawURL, normalizeResourceDownloadHeaders(nil, rawURL), nil, "op-test")

	if preflight.IsUnsupported() {
		t.Fatalf("expected byte-range preflight to be supported, got %#v", preflight)
	}
	if preflight.KeyProbe == nil || !preflight.KeyProbe.DecryptionValidated {
		t.Fatalf("expected byte-range segment decryption to be validated, got %#v", preflight.KeyProbe)
	}
}

func encryptHLSProbeFixture(t *testing.T, key []byte, iv []byte) []byte {
	t.Helper()
	plain := make([]byte, 384)
	plain[0] = 0x47
	plain[188] = 0x47
	plain[376] = 0x47
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	encrypted := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(encrypted, plain)
	return encrypted
}
