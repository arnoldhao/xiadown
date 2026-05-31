package ytdlp

import "testing"

func TestAnalyzeHLSManifestDetectsAES128ExtensionlessSegments(t *testing.T) {
	t.Parallel()

	manifest := `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:5
#EXT-X-KEY:METHOD=AES-128,URI="../key",IV=0x00000000000000000000000000000000
#EXTINF:2.68,
https://ipfs.example/ipfs/bafybeifkndiqcienj65nwpniyq7aka4qhknbijev7l3xyoihf5zifmgfb4
#EXTINF:3.0,
https://ipfs.example/ipfs/bafybeid5tm7qalxup3wslntk5loyxqj37opxwm3k76le6js
#EXT-X-ENDLIST`

	preflight := AnalyzeStreamManifest(
		"https://www.example.com/info/m3u8/8192/1.m3u8",
		[]byte(manifest),
		"application/vnd.apple.mpegurl",
		&StreamKeyProbe{
			URI:                      "../key",
			ResolvedURI:              "https://www.example.com/info/m3u8/key",
			LengthBytes:              32,
			LooksASCIIHex:            true,
			NormalizedLengthBytes:    16,
			NormalizedKeySource:      HLSKeyMaterialSourceASCIIHex,
			NormalizedKeyRule:        HLSKeyMaterialRuleCompatASCIIHex,
			NormalizedKeyNonStandard: true,
			ManifestKeyOverride:      true,
			NormalizedKeyHex:         "00112233445566778899aabbccddeeff",
			DecryptionValidated:      true,
		},
	)

	if preflight.Kind != StreamManifestKindHLS {
		t.Fatalf("expected hls preflight, got %#v", preflight)
	}
	if preflight.EncryptionType != StreamEncryptionAES128 {
		t.Fatalf("expected AES-128, got %q", preflight.EncryptionType)
	}
	if preflight.SegmentCount != 2 || !preflight.SegmentExtensionless {
		t.Fatalf("expected extensionless segments to be detected, got %#v", preflight)
	}
	if preflight.DurationMs != 5680 {
		t.Fatalf("expected duration to be accumulated, got %d", preflight.DurationMs)
	}
	if preflight.Strategy.Downloader != StreamDownloaderNativeM3U8 {
		t.Fatalf("expected native HLS strategy, got %#v", preflight.Strategy)
	}
	if preflight.Strategy.DisableConcurrentFragments {
		t.Fatalf("expected native HLS strategy to keep fragment concurrency enabled")
	}
	if len(preflight.Strategy.ExtractorArgs) != 1 || preflight.Strategy.ExtractorArgs[0] != "generic:hls_key=00112233445566778899aabbccddeeff" {
		t.Fatalf("expected normalized hls key extractor arg, got %#v", preflight.Strategy.ExtractorArgs)
	}
	if preflight.IsUnsupported() {
		t.Fatalf("expected non-DRM AES-128 HLS to remain supported, got %q", preflight.UnsupportedReason)
	}
}

func TestAnalyzeHLSManifestFailsAES128WithoutValidatedKey(t *testing.T) {
	t.Parallel()

	manifest := `#EXTM3U
#EXT-X-KEY:METHOD=AES-128,URI="../key"
#EXTINF:4.0,
seg.ts`

	preflight := AnalyzeStreamManifest(
		"https://www.example.com/stream.m3u8",
		[]byte(manifest),
		"application/vnd.apple.mpegurl",
		&StreamKeyProbe{
			URI:                   "../key",
			ResolvedURI:           "https://www.example.com/key",
			LengthBytes:           16,
			NormalizedLengthBytes: 16,
			NormalizedKeySource:   HLSKeyMaterialSourceRaw,
			NormalizedKeyHex:      "30313233343536373839616263646566",
		},
	)

	if !preflight.IsUnsupported() {
		t.Fatalf("expected AES-128 without decryption validation to be unsupported, got %#v", preflight)
	}
}

func TestNormalizeHLSKeyMaterialDecodesASCIIHexKey(t *testing.T) {
	t.Parallel()

	material := NormalizeHLSKeyMaterial([]byte("00112233445566778899aabbccddeeff"))

	if material.Source != HLSKeyMaterialSourceASCIIHex || material.LengthBytes != 16 {
		t.Fatalf("expected ASCII hex key to decode to 16 bytes, got %#v", material)
	}
	if material.KeyHex != "00112233445566778899aabbccddeeff" {
		t.Fatalf("expected key hex to be preserved, got %q", material.KeyHex)
	}
}

func TestHLSKeyMaterialCandidatesIncludesASCIIFirst16ForProbe(t *testing.T) {
	t.Parallel()

	candidates := HLSKeyMaterialCandidates([]byte("ba9bf05693b9fa202d922dd43a08f281"))

	if len(candidates) < 3 {
		t.Fatalf("expected multiple key candidates, got %#v", candidates)
	}
	if candidates[0].Source != HLSKeyMaterialSourceASCIIFirst16 ||
		candidates[0].KeyHex != "62613962663035363933623966613230" ||
		candidates[0].Rule != HLSKeyMaterialRuleNonStandardFirst16 ||
		!candidates[0].ManifestKeyOverride {
		t.Fatalf("expected first candidate to be first 16 ASCII bytes, got %#v", candidates[0])
	}
}

func TestFirstHLSSegmentProbeResolvesURLAndIV(t *testing.T) {
	t.Parallel()

	manifest := `#EXTM3U
#EXT-X-MEDIA-SEQUENCE:7
#EXT-X-KEY:METHOD=AES-128,URI="key"
#EXTINF:4.0,
seg0.ts
`
	probe, ok := FirstHLSSegmentProbe("https://media.example/path/index.m3u8", []byte(manifest))

	if !ok {
		t.Fatalf("expected first segment probe")
	}
	if probe.URL != "https://media.example/path/seg0.ts" {
		t.Fatalf("expected resolved segment URL, got %q", probe.URL)
	}
	wantIV := "00000000000000000000000000000007"
	if got := hexString(probe.IV); got != wantIV {
		t.Fatalf("expected media sequence IV %s, got %s", wantIV, got)
	}
}

func TestFirstHLSSegmentProbeSkipsClearSegments(t *testing.T) {
	t.Parallel()

	manifest := `#EXTM3U
#EXT-X-MEDIA-SEQUENCE:4
#EXTINF:4.0,
clear.ts
#EXT-X-KEY:METHOD=AES-128,URI="key"
#EXTINF:4.0,
encrypted.ts
`
	probe, ok := FirstHLSSegmentProbe("https://media.example/path/index.m3u8", []byte(manifest))

	if !ok {
		t.Fatalf("expected encrypted segment probe")
	}
	if probe.URL != "https://media.example/path/encrypted.ts" {
		t.Fatalf("expected encrypted segment URL, got %q", probe.URL)
	}
	wantIV := "00000000000000000000000000000005"
	if got := hexString(probe.IV); got != wantIV {
		t.Fatalf("expected media sequence IV %s, got %s", wantIV, got)
	}
}

func TestFirstHLSSegmentProbeParsesByteRange(t *testing.T) {
	t.Parallel()

	manifest := `#EXTM3U
#EXT-X-KEY:METHOD=AES-128,URI="key",IV=0x00000000000000000000000000000000
#EXT-X-BYTERANGE:1880@3760
#EXTINF:4.0,
media.ts
`
	probe, ok := FirstHLSSegmentProbe("https://media.example/path/index.m3u8", []byte(manifest))

	if !ok {
		t.Fatalf("expected byte-range segment probe")
	}
	if !probe.HasByteRange || probe.ByteRange.Start != 3760 || probe.ByteRange.End != 5640 {
		t.Fatalf("expected byte range 3760-5640, got %#v", probe)
	}
}

func TestBuildStreamDownloadStrategyAddsQueryExtractorArgs(t *testing.T) {
	t.Parallel()

	preflight := StreamManifestPreflight{
		Kind:                    StreamManifestKindHLS,
		EncryptionType:          StreamEncryptionAES128,
		VariantQuery:            "token=master",
		VariantQueryPassthrough: true,
		KeyProbe: &StreamKeyProbe{
			LengthBytes:              16,
			NormalizedLengthBytes:    16,
			NormalizedKeyHex:         "30313233343536373839616263646566",
			DecryptionValidated:      true,
			KeyQuery:                 "token=key",
			KeyQueryPassthrough:      true,
			FragmentQuery:            "token=segment",
			FragmentQueryPassthrough: true,
		},
	}
	strategy := BuildStreamDownloadStrategy(preflight)

	want := []string{
		"generic:variant_query=token=master",
		"generic:fragment_query=token=segment",
		"generic:key_query=token=key",
	}
	if len(strategy.ExtractorArgs) != len(want) {
		t.Fatalf("expected extractor args %#v, got %#v", want, strategy.ExtractorArgs)
	}
	for index, expected := range want {
		if strategy.ExtractorArgs[index] != expected {
			t.Fatalf("extractor arg %d = %q, want %q", index, strategy.ExtractorArgs[index], expected)
		}
	}
}

func hexString(data []byte) string {
	const digits = "0123456789abcdef"
	result := make([]byte, len(data)*2)
	for i, b := range data {
		result[i*2] = digits[b>>4]
		result[i*2+1] = digits[b&0x0f]
	}
	return string(result)
}

func TestAnalyzeHLSManifestRejectsFairPlay(t *testing.T) {
	t.Parallel()

	manifest := `#EXTM3U
#EXT-X-KEY:METHOD=SAMPLE-AES,URI="skd://asset",KEYFORMAT="com.apple.streamingkeydelivery"
#EXTINF:4.0,
segment1.m4s`

	preflight := AnalyzeStreamManifest("https://media.example/stream.m3u8", []byte(manifest), "", nil)

	if !preflight.DRM {
		t.Fatalf("expected DRM to be detected, got %#v", preflight)
	}
	if preflight.EncryptionType != StreamEncryptionDRM {
		t.Fatalf("expected DRM encryption type, got %q", preflight.EncryptionType)
	}
	if !preflight.IsUnsupported() {
		t.Fatalf("expected FairPlay stream to be unsupported")
	}
}

func TestAnalyzeDASHManifestDetectsWidevineProtection(t *testing.T) {
	t.Parallel()

	manifest := `<?xml version="1.0" encoding="UTF-8"?>
<MPD xmlns:cenc="urn:mpeg:cenc:2013">
  <Period>
    <AdaptationSet>
      <ContentProtection schemeIdUri="urn:mpeg:dash:mp4protection:2011" value="cenc" cenc:default_KID="00010203-0405-0607-0809-0a0b0c0d0e0f"/>
      <ContentProtection schemeIdUri="urn:uuid:edef8ba9-79d6-4ace-a3c8-27dcd51d21ed">
        <cenc:pssh>AAAA</cenc:pssh>
      </ContentProtection>
    </AdaptationSet>
  </Period>
</MPD>`

	preflight := AnalyzeStreamManifest("https://media.example/manifest.mpd", []byte(manifest), "application/dash+xml", nil)

	if preflight.Kind != StreamManifestKindDASH {
		t.Fatalf("expected dash preflight, got %#v", preflight)
	}
	if !preflight.DRM || preflight.EncryptionType != StreamEncryptionDRM {
		t.Fatalf("expected DRM DASH to be detected, got %#v", preflight)
	}
	if len(preflight.DRMSystems) != 1 || preflight.DRMSystems[0] != "widevine" {
		t.Fatalf("expected widevine system, got %#v", preflight.DRMSystems)
	}
	if !preflight.IsUnsupported() {
		t.Fatalf("expected DRM DASH to be unsupported")
	}
}

func TestResolveManifestReference(t *testing.T) {
	t.Parallel()

	got := ResolveManifestReference("https://www.example.com/info/m3u8/8192/1.m3u8", "../key")
	want := "https://www.example.com/info/m3u8/key"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestExtractHLSPlaylistReferences(t *testing.T) {
	t.Parallel()

	manifest := `#EXTM3U
#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="audio",URI="audio/eng.m3u8"
#EXT-X-STREAM-INF:BANDWIDTH=1000000
low/index.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=2000000
https://media.example/high/index.m3u8
`
	got := ExtractHLSPlaylistReferences("https://cdn.example/master/main.m3u8", []byte(manifest))
	want := []string{
		"https://cdn.example/master/audio/eng.m3u8",
		"https://cdn.example/master/low/index.m3u8",
		"https://media.example/high/index.m3u8",
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d references, got %#v", len(want), got)
	}
	for index, expected := range want {
		if got[index] != expected {
			t.Fatalf("reference %d = %q, want %q", index, got[index], expected)
		}
	}
}
