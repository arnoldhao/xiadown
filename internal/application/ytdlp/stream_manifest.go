package ytdlp

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
)

const (
	StreamManifestKindHLS  = "hls"
	StreamManifestKindDASH = "dash"

	StreamEncryptionNone         = "none"
	StreamEncryptionAES128       = "aes-128"
	StreamEncryptionSampleAES    = "sample-aes"
	StreamEncryptionSampleAESCTR = "sample-aes-ctr"
	StreamEncryptionAES256GCM    = "aes-256-gcm"
	StreamEncryptionDRM          = "drm"
	StreamEncryptionUnknown      = "unknown"
	StreamDownloaderNativeM3U8   = "m3u8:native"

	HLSKeyMaterialSourceRaw          = "raw"
	HLSKeyMaterialSourceASCIIHex     = "ascii_hex"
	HLSKeyMaterialSourceASCIIFirst16 = "ascii_first16"
	HLSKeyMaterialSourceASCIIRaw32   = "ascii_raw_32"

	HLSKeyMaterialRuleStandardRaw16        = "standard.raw_16"
	HLSKeyMaterialRuleCompatASCIIHex       = "compat.ascii_hex"
	HLSKeyMaterialRuleNonStandardFirst16   = "nonstandard.ascii_first16"
	HLSKeyMaterialRuleNonStandardRaw24Or32 = "nonstandard.raw_24_32"
)

type StreamDownloadStrategy struct {
	Downloader                 string   `json:"downloader,omitempty"`
	DownloaderArgs             []string `json:"downloaderArgs,omitempty"`
	ExtractorArgs              []string `json:"-"`
	DisableConcurrentFragments bool     `json:"disableConcurrentFragments,omitempty"`
	Reason                     string   `json:"reason,omitempty"`
}

type StreamKeyProbe struct {
	URI                         string `json:"uri,omitempty"`
	ResolvedURI                 string `json:"resolvedUri,omitempty"`
	Method                      string `json:"method,omitempty"`
	KeyFormat                   string `json:"keyFormat,omitempty"`
	LengthBytes                 int    `json:"lengthBytes,omitempty"`
	LooksASCIIHex               bool   `json:"looksAsciiHex,omitempty"`
	NormalizedLengthBytes       int    `json:"normalizedLengthBytes,omitempty"`
	NormalizedKeySource         string `json:"normalizedKeySource,omitempty"`
	NormalizedKeyRule           string `json:"normalizedKeyRule,omitempty"`
	NormalizedKeyNonStandard    bool   `json:"normalizedKeyNonStandard,omitempty"`
	ManifestKeyOverride         bool   `json:"manifestKeyOverride,omitempty"`
	NormalizedKeyHex            string `json:"-"`
	KeyQuery                    string `json:"-"`
	KeyQueryPassthrough         bool   `json:"keyQueryPassthrough,omitempty"`
	FragmentQuery               string `json:"-"`
	FragmentQueryPassthrough    bool   `json:"fragmentQueryPassthrough,omitempty"`
	DecryptionValidated         bool   `json:"decryptionValidated,omitempty"`
	DecryptionValidationFormat  string `json:"decryptionValidationFormat,omitempty"`
	DecryptionValidationSegment string `json:"decryptionValidationSegment,omitempty"`
	DecryptionValidationError   string `json:"decryptionValidationError,omitempty"`
	Error                       string `json:"error,omitempty"`
}

type StreamManifestPreflight struct {
	Kind                    string                 `json:"kind,omitempty"`
	URL                     string                 `json:"url,omitempty"`
	ContentType             string                 `json:"contentType,omitempty"`
	EncryptionType          string                 `json:"encryptionType,omitempty"`
	DRM                     bool                   `json:"drm,omitempty"`
	DRMSystems              []string               `json:"drmSystems,omitempty"`
	UnsupportedReason       string                 `json:"unsupportedReason,omitempty"`
	HLSMethods              []string               `json:"hlsMethods,omitempty"`
	KeyURI                  string                 `json:"keyUri,omitempty"`
	KeyURICount             int                    `json:"keyUriCount,omitempty"`
	KeyFormat               string                 `json:"keyFormat,omitempty"`
	HasInitMap              bool                   `json:"hasInitMap,omitempty"`
	HasVariants             bool                   `json:"hasVariants,omitempty"`
	SegmentCount            int                    `json:"segmentCount,omitempty"`
	DurationMs              int64                  `json:"durationMs,omitempty"`
	SegmentExtensionless    bool                   `json:"segmentExtensionless,omitempty"`
	SegmentExtensions       []string               `json:"segmentExtensions,omitempty"`
	DASHProtectionScheme    string                 `json:"dashProtectionScheme,omitempty"`
	KeyProbe                *StreamKeyProbe        `json:"keyProbe,omitempty"`
	Strategy                StreamDownloadStrategy `json:"strategy,omitempty"`
	Warnings                []string               `json:"warnings,omitempty"`
	FetchError              string                 `json:"fetchError,omitempty"`
	VariantQuery            string                 `json:"-"`
	VariantQueryPassthrough bool                   `json:"variantQueryPassthrough,omitempty"`
}

func (preflight StreamManifestPreflight) IsEmpty() bool {
	return strings.TrimSpace(preflight.Kind) == "" && strings.TrimSpace(preflight.FetchError) == ""
}

func (preflight StreamManifestPreflight) IsUnsupported() bool {
	return strings.TrimSpace(preflight.UnsupportedReason) != ""
}

func (preflight StreamManifestPreflight) MetadataMap() map[string]any {
	if preflight.IsEmpty() {
		return nil
	}
	data, err := json.Marshal(preflight)
	if err != nil {
		return nil
	}
	payload := map[string]any{}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil
	}
	return payload
}

func StreamManifestKindFromURL(rawURL string) string {
	extension := streamURLPathExtension(rawURL)
	switch extension {
	case "m3u8":
		return StreamManifestKindHLS
	case "mpd":
		return StreamManifestKindDASH
	default:
		return ""
	}
}

func LooksLikeStreamManifestURL(rawURL string) bool {
	return StreamManifestKindFromURL(rawURL) != ""
}

func ExtractHLSPlaylistReferences(manifestURL string, body []byte) []string {
	refs := map[string]struct{}{}
	expectVariant := false
	for _, rawLine := range strings.Split(strings.TrimSpace(string(body)), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		upperLine := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(upperLine, "#EXT-X-STREAM-INF"):
			expectVariant = true
			continue
		case strings.HasPrefix(upperLine, "#EXT-X-I-FRAME-STREAM-INF:"):
			attrs := parseHLSAttributeList(line[len("#EXT-X-I-FRAME-STREAM-INF:"):])
			if ref := ResolveManifestReference(manifestURL, attrs["URI"]); strings.TrimSpace(ref) != "" {
				refs[ref] = struct{}{}
			}
			continue
		case strings.HasPrefix(upperLine, "#EXT-X-MEDIA:"):
			attrs := parseHLSAttributeList(line[len("#EXT-X-MEDIA:"):])
			if ref := ResolveManifestReference(manifestURL, attrs["URI"]); strings.TrimSpace(ref) != "" {
				refs[ref] = struct{}{}
			}
			continue
		case strings.HasPrefix(line, "#"):
			continue
		}
		if !expectVariant {
			continue
		}
		expectVariant = false
		if ref := ResolveManifestReference(manifestURL, line); strings.TrimSpace(ref) != "" {
			refs[ref] = struct{}{}
		}
	}
	return sortedKeys(refs)
}

func HLSNativeStreamStrategy(reason string, extractorArgs ...string) StreamDownloadStrategy {
	trimmedReason := strings.TrimSpace(reason)
	if trimmedReason == "" {
		trimmedReason = "hls_native"
	}
	strategy := StreamDownloadStrategy{
		Downloader: StreamDownloaderNativeM3U8,
		Reason:     trimmedReason,
	}
	for _, arg := range extractorArgs {
		if trimmed := strings.TrimSpace(arg); trimmed != "" {
			strategy.ExtractorArgs = append(strategy.ExtractorArgs, trimmed)
		}
	}
	return strategy
}

func AnalyzeStreamManifest(rawURL string, body []byte, contentType string, keyProbe *StreamKeyProbe) StreamManifestPreflight {
	kind := StreamManifestKindFromURL(rawURL)
	trimmedBody := strings.TrimSpace(string(body))
	if kind == "" {
		lowerType := strings.ToLower(strings.TrimSpace(contentType))
		switch {
		case strings.Contains(lowerType, "mpegurl"), strings.Contains(trimmedBody, "#EXTM3U"):
			kind = StreamManifestKindHLS
		case strings.Contains(lowerType, "dash+xml"), strings.Contains(trimmedBody, "<MPD"):
			kind = StreamManifestKindDASH
		}
	}
	switch kind {
	case StreamManifestKindHLS:
		return analyzeHLSManifest(rawURL, trimmedBody, contentType, keyProbe)
	case StreamManifestKindDASH:
		return analyzeDASHManifest(rawURL, body, contentType)
	default:
		return StreamManifestPreflight{URL: strings.TrimSpace(rawURL), ContentType: strings.TrimSpace(contentType)}
	}
}

func analyzeHLSManifest(rawURL string, body string, contentType string, keyProbe *StreamKeyProbe) StreamManifestPreflight {
	preflight := StreamManifestPreflight{
		Kind:           StreamManifestKindHLS,
		URL:            strings.TrimSpace(rawURL),
		ContentType:    strings.TrimSpace(contentType),
		EncryptionType: StreamEncryptionNone,
	}
	methods := map[string]struct{}{}
	keyURIs := map[string]struct{}{}
	segmentExts := map[string]struct{}{}
	warnings := map[string]struct{}{}
	drmSystems := map[string]struct{}{}
	segmentExtensionless := false
	segmentCount := 0
	totalDurationMs := int64(0)
	pendingSegmentDurationMs := int64(0)
	expectMediaSegment := false
	expectVariant := false

	for _, rawLine := range strings.Split(body, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		upperLine := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(upperLine, "#EXT-X-STREAM-INF"):
			preflight.HasVariants = true
			expectVariant = true
			continue
		case strings.HasPrefix(upperLine, "#EXT-X-MAP"):
			preflight.HasInitMap = true
			continue
		case strings.HasPrefix(upperLine, "#EXTINF"):
			expectMediaSegment = true
			pendingSegmentDurationMs = parseHLSEXTINFDurationMs(line)
			continue
		case strings.HasPrefix(upperLine, "#EXT-X-KEY:"):
			attrs := parseHLSAttributeList(line[len("#EXT-X-KEY:"):])
			method := strings.ToUpper(strings.TrimSpace(attrs["METHOD"]))
			if method == "" {
				method = "UNKNOWN"
			}
			methods[method] = struct{}{}
			if preflight.KeyURI == "" {
				preflight.KeyURI = strings.TrimSpace(attrs["URI"])
			}
			if uri := strings.TrimSpace(attrs["URI"]); uri != "" {
				keyURIs[uri] = struct{}{}
			}
			if preflight.KeyFormat == "" {
				preflight.KeyFormat = strings.TrimSpace(attrs["KEYFORMAT"])
			}
			registerHLSDRM(attrs, drmSystems)
			continue
		case strings.HasPrefix(upperLine, "#EXT-X-SESSION-KEY:"):
			attrs := parseHLSAttributeList(line[len("#EXT-X-SESSION-KEY:"):])
			method := strings.ToUpper(strings.TrimSpace(attrs["METHOD"]))
			if method == "" {
				method = "UNKNOWN"
			}
			methods[method] = struct{}{}
			if preflight.KeyURI == "" {
				preflight.KeyURI = strings.TrimSpace(attrs["URI"])
			}
			if uri := strings.TrimSpace(attrs["URI"]); uri != "" {
				keyURIs[uri] = struct{}{}
			}
			if preflight.KeyFormat == "" {
				preflight.KeyFormat = strings.TrimSpace(attrs["KEYFORMAT"])
			}
			registerHLSDRM(attrs, drmSystems)
			continue
		case strings.HasPrefix(upperLine, "#EXT-X-FAXS-CM"):
			drmSystems["adobe-access"] = struct{}{}
			continue
		case strings.HasPrefix(line, "#"):
			continue
		}

		if expectVariant {
			expectVariant = false
			continue
		}
		if expectMediaSegment || !preflight.HasVariants {
			expectMediaSegment = false
			segmentCount++
			if pendingSegmentDurationMs > 0 {
				totalDurationMs += pendingSegmentDurationMs
			}
			pendingSegmentDurationMs = 0
			ext := streamURLPathExtension(line)
			if ext == "" {
				segmentExtensionless = true
				warnings["hls_segment_extensionless"] = struct{}{}
				continue
			}
			segmentExts[ext] = struct{}{}
			if !isKnownHLSMediaSegmentExtension(ext) {
				warnings["hls_segment_unknown_extension"] = struct{}{}
			}
		}
	}

	preflight.SegmentCount = segmentCount
	preflight.DurationMs = totalDurationMs
	preflight.SegmentExtensionless = segmentExtensionless
	preflight.SegmentExtensions = sortedKeys(segmentExts)
	preflight.HLSMethods = sortedKeys(methods)
	preflight.KeyURICount = len(keyURIs)
	preflight.DRMSystems = sortedKeys(drmSystems)
	preflight.DRM = len(preflight.DRMSystems) > 0
	if keyProbe != nil {
		keyProbe.KeyFormat = strings.TrimSpace(firstNonEmpty(keyProbe.KeyFormat, preflight.KeyFormat))
		preflight.KeyProbe = keyProbe
		if keyProbe.Error != "" {
			warnings["hls_key_probe_failed"] = struct{}{}
		}
		if keyProbe.LengthBytes > 0 && keyProbe.LengthBytes != 16 {
			warnings["hls_key_nonstandard_length"] = struct{}{}
		}
		if keyProbe.LengthBytes == 32 && keyProbe.LooksASCIIHex {
			warnings["hls_key_ascii_hex_32"] = struct{}{}
		}
		if keyProbe.NormalizedKeyNonStandard {
			warnings["hls_key_nonstandard_material"] = struct{}{}
		}
	}
	preflight.EncryptionType = resolveHLSEncryptionType(preflight.HLSMethods, preflight.DRM)
	if preflight.KeyProbe != nil {
		preflight.KeyProbe.Method = strings.ToLower(strings.TrimSpace(firstNonEmpty(preflight.KeyProbe.Method, preflight.EncryptionType)))
	}
	if preflight.DRM {
		preflight.EncryptionType = StreamEncryptionDRM
		preflight.UnsupportedReason = "DRM-protected HLS streams are not supported"
	} else if unsupported := unsupportedHLSEncryptionMethod(preflight.HLSMethods); unsupported != "" {
		preflight.UnsupportedReason = "unsupported HLS encryption method: " + unsupported
	} else if preflight.EncryptionType == StreamEncryptionAES128 {
		if keyProbe == nil {
			preflight.UnsupportedReason = "HLS AES-128 key was not verified before download"
		} else if strings.TrimSpace(keyProbe.Error) != "" {
			preflight.UnsupportedReason = "HLS AES-128 key request failed: " + strings.TrimSpace(keyProbe.Error)
		} else if !keyProbe.DecryptionValidated {
			if strings.TrimSpace(keyProbe.DecryptionValidationError) != "" {
				preflight.UnsupportedReason = "HLS AES-128 first segment could not be decrypted: " + strings.TrimSpace(keyProbe.DecryptionValidationError)
			} else {
				preflight.UnsupportedReason = "HLS AES-128 first segment could not be decrypted"
			}
		} else if strings.TrimSpace(keyProbe.NormalizedKeyHex) == "" {
			preflight.UnsupportedReason = "HLS AES-128 key material could not be interpreted"
		} else if keyProbe.NormalizedLengthBytes != 16 {
			preflight.UnsupportedReason = "HLS AES-128 key normalized to unsupported length: " + strconv.Itoa(keyProbe.NormalizedLengthBytes) + " bytes"
		} else if keyProbe.ManifestKeyOverride && preflight.KeyURICount > 1 {
			preflight.UnsupportedReason = "HLS AES-128 streams with multiple nonstandard keys are not supported"
		}
	}
	preflight.Warnings = sortedKeys(warnings)
	preflight.Strategy = BuildStreamDownloadStrategy(preflight)
	return preflight
}

func analyzeDASHManifest(rawURL string, body []byte, contentType string) StreamManifestPreflight {
	preflight := StreamManifestPreflight{
		Kind:           StreamManifestKindDASH,
		URL:            strings.TrimSpace(rawURL),
		ContentType:    strings.TrimSpace(contentType),
		EncryptionType: StreamEncryptionNone,
	}
	decoder := xml.NewDecoder(strings.NewReader(string(body)))
	drmSystems := map[string]struct{}{}
	encrypted := false
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		start, ok := token.(xml.StartElement)
		if !ok || !strings.EqualFold(start.Name.Local, "ContentProtection") {
			continue
		}
		schemeID := ""
		value := ""
		for _, attr := range start.Attr {
			switch strings.ToLower(attr.Name.Local) {
			case "schemeiduri":
				schemeID = strings.ToLower(strings.TrimSpace(attr.Value))
			case "value":
				value = strings.ToLower(strings.TrimSpace(attr.Value))
			}
		}
		switch {
		case schemeID == "urn:mpeg:dash:mp4protection:2011":
			encrypted = true
			if value != "" && preflight.DASHProtectionScheme == "" {
				preflight.DASHProtectionScheme = value
			}
		case strings.HasPrefix(schemeID, "urn:uuid:"):
			encrypted = true
			drmSystems[dashDRMSystemName(strings.TrimPrefix(schemeID, "urn:uuid:"))] = struct{}{}
		}
	}
	preflight.DRMSystems = sortedKeys(drmSystems)
	preflight.DRM = len(preflight.DRMSystems) > 0
	if encrypted {
		if preflight.DASHProtectionScheme != "" {
			preflight.EncryptionType = preflight.DASHProtectionScheme
		} else {
			preflight.EncryptionType = StreamEncryptionUnknown
		}
	}
	if preflight.DRM {
		preflight.EncryptionType = StreamEncryptionDRM
		preflight.UnsupportedReason = "DRM-protected DASH streams are not supported"
	} else if encrypted {
		preflight.UnsupportedReason = "encrypted DASH streams without a supported clear-key workflow are not supported"
	}
	preflight.Strategy = BuildStreamDownloadStrategy(preflight)
	return preflight
}

func BuildStreamDownloadStrategy(preflight StreamManifestPreflight) StreamDownloadStrategy {
	if preflight.Kind != StreamManifestKindHLS || preflight.IsUnsupported() || preflight.DRM {
		return StreamDownloadStrategy{}
	}
	reasons := []string{"hls_native"}
	if preflight.SegmentExtensionless {
		reasons = append(reasons, "hls_extensionless_segments")
	}
	extractorArgs := []string{}
	if preflight.KeyProbe != nil && preflight.KeyProbe.LengthBytes == 32 && preflight.KeyProbe.LooksASCIIHex {
		reasons = append(reasons, "hls_ascii_hex_key")
	}
	if preflight.KeyProbe != nil && preflight.KeyProbe.ManifestKeyOverride {
		reasons = append(reasons, "hls_key_override")
		if strings.TrimSpace(preflight.KeyProbe.NormalizedKeyHex) != "" && preflight.KeyURICount <= 1 {
			extractorArgs = append(extractorArgs, "generic:hls_key="+strings.TrimSpace(preflight.KeyProbe.NormalizedKeyHex))
		}
	}
	if strings.TrimSpace(preflight.VariantQuery) != "" {
		reasons = append(reasons, "hls_variant_query")
		extractorArgs = append(extractorArgs, "generic:variant_query="+strings.TrimSpace(preflight.VariantQuery))
	}
	if preflight.KeyProbe != nil {
		if strings.TrimSpace(preflight.KeyProbe.FragmentQuery) != "" {
			reasons = append(reasons, "hls_fragment_query")
			extractorArgs = append(extractorArgs, "generic:fragment_query="+strings.TrimSpace(preflight.KeyProbe.FragmentQuery))
		}
		if !preflight.KeyProbe.ManifestKeyOverride && strings.TrimSpace(preflight.KeyProbe.KeyQuery) != "" {
			reasons = append(reasons, "hls_key_query")
			extractorArgs = append(extractorArgs, "generic:key_query="+strings.TrimSpace(preflight.KeyProbe.KeyQuery))
		}
	}
	return HLSNativeStreamStrategy(strings.Join(reasons, ","), extractorArgs...)
}

type HLSKeyMaterial struct {
	KeyHex              string
	LengthBytes         int
	Source              string
	Rule                string
	NonStandard         bool
	ManifestKeyOverride bool
}

type HLSSegmentProbe struct {
	URL          string
	IV           []byte
	ByteRange    HLSByteRange
	HasByteRange bool
}

type HLSByteRange struct {
	Start int64
	End   int64
}

func NormalizeHLSKeyMaterial(data []byte) HLSKeyMaterial {
	if len(data) == 0 {
		return HLSKeyMaterial{}
	}
	trimmed := []byte(strings.TrimSpace(string(data)))
	if len(trimmed) > 0 && len(trimmed)%2 == 0 && LooksLikeASCIIHex(trimmed) {
		if decoded, err := hex.DecodeString(string(trimmed)); err == nil && len(decoded) > 0 {
			switch len(decoded) {
			case 16, 24, 32:
				return HLSKeyMaterial{
					KeyHex:              strings.ToLower(string(trimmed)),
					LengthBytes:         len(decoded),
					Source:              HLSKeyMaterialSourceASCIIHex,
					Rule:                HLSKeyMaterialRuleCompatASCIIHex,
					NonStandard:         true,
					ManifestKeyOverride: true,
				}
			}
		}
	}
	switch len(data) {
	case 16:
		return HLSKeyMaterial{
			KeyHex:      hex.EncodeToString(data),
			LengthBytes: len(data),
			Source:      HLSKeyMaterialSourceRaw,
			Rule:        HLSKeyMaterialRuleStandardRaw16,
		}
	case 24, 32:
		return HLSKeyMaterial{
			KeyHex:      hex.EncodeToString(data),
			LengthBytes: len(data),
			Source:      HLSKeyMaterialSourceRaw,
			Rule:        HLSKeyMaterialRuleNonStandardRaw24Or32,
			NonStandard: true,
		}
	default:
		return HLSKeyMaterial{}
	}
}

type HLSKeyMaterialCandidateContext struct {
	ManifestURL  string
	ManifestBody []byte
	KeyURI       string
	KeyBody      []byte
	KeyFormat    string
}

type hlsKeyMaterialCandidateRule struct {
	ID      string
	Builder func(HLSKeyMaterialCandidateContext) []HLSKeyMaterial
}

var hlsKeyMaterialCandidateRules = []hlsKeyMaterialCandidateRule{
	{ID: HLSKeyMaterialRuleStandardRaw16, Builder: buildHLSRaw16KeyCandidate},
	{ID: HLSKeyMaterialRuleNonStandardFirst16, Builder: buildHLSASCIIFirst16KeyCandidate},
	{ID: HLSKeyMaterialRuleCompatASCIIHex, Builder: buildHLSASCIIHexKeyCandidate},
	{ID: HLSKeyMaterialRuleNonStandardRaw24Or32, Builder: buildHLSRaw24Or32KeyCandidate},
}

func HLSKeyMaterialCandidates(data []byte) []HLSKeyMaterial {
	return HLSKeyMaterialCandidatesWithContext(HLSKeyMaterialCandidateContext{KeyBody: data})
}

func HLSKeyMaterialCandidatesWithContext(context HLSKeyMaterialCandidateContext) []HLSKeyMaterial {
	if len(context.KeyBody) == 0 {
		return nil
	}
	result := make([]HLSKeyMaterial, 0, 4)
	seen := map[string]struct{}{}
	add := func(material HLSKeyMaterial) {
		if strings.TrimSpace(material.KeyHex) == "" || material.LengthBytes <= 0 || strings.TrimSpace(material.Source) == "" {
			return
		}
		key := material.KeyHex + ":" + strconv.Itoa(material.LengthBytes)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		result = append(result, material)
	}
	for _, rule := range hlsKeyMaterialCandidateRules {
		for _, material := range rule.Builder(context) {
			add(material)
		}
	}
	return result
}

func buildHLSRaw16KeyCandidate(context HLSKeyMaterialCandidateContext) []HLSKeyMaterial {
	if len(context.KeyBody) != 16 {
		return nil
	}
	return []HLSKeyMaterial{{
		KeyHex:      hex.EncodeToString(context.KeyBody),
		LengthBytes: 16,
		Source:      HLSKeyMaterialSourceRaw,
		Rule:        HLSKeyMaterialRuleStandardRaw16,
	}}
}

func buildHLSASCIIFirst16KeyCandidate(context HLSKeyMaterialCandidateContext) []HLSKeyMaterial {
	trimmed := []byte(strings.TrimSpace(string(context.KeyBody)))
	if len(trimmed) != 32 || !LooksLikeASCIIHex(trimmed) {
		return nil
	}
	return []HLSKeyMaterial{{
		KeyHex:              hex.EncodeToString(trimmed[:16]),
		LengthBytes:         16,
		Source:              HLSKeyMaterialSourceASCIIFirst16,
		Rule:                HLSKeyMaterialRuleNonStandardFirst16,
		NonStandard:         true,
		ManifestKeyOverride: true,
	}}
}

func buildHLSASCIIHexKeyCandidate(context HLSKeyMaterialCandidateContext) []HLSKeyMaterial {
	trimmed := []byte(strings.TrimSpace(string(context.KeyBody)))
	if len(trimmed) == 0 || len(trimmed)%2 != 0 || !LooksLikeASCIIHex(trimmed) {
		return nil
	}
	decoded, err := hex.DecodeString(string(trimmed))
	if err != nil {
		return nil
	}
	switch len(decoded) {
	case 16, 24, 32:
		return []HLSKeyMaterial{{
			KeyHex:              strings.ToLower(string(trimmed)),
			LengthBytes:         len(decoded),
			Source:              HLSKeyMaterialSourceASCIIHex,
			Rule:                HLSKeyMaterialRuleCompatASCIIHex,
			NonStandard:         true,
			ManifestKeyOverride: true,
		}}
	default:
		return nil
	}
}

func buildHLSRaw24Or32KeyCandidate(context HLSKeyMaterialCandidateContext) []HLSKeyMaterial {
	switch len(context.KeyBody) {
	case 24, 32:
		source := HLSKeyMaterialSourceRaw
		if len(context.KeyBody) == 32 && LooksLikeASCIIHex(context.KeyBody) {
			source = HLSKeyMaterialSourceASCIIRaw32
		}
		return []HLSKeyMaterial{{
			KeyHex:      hex.EncodeToString(context.KeyBody),
			LengthBytes: len(context.KeyBody),
			Source:      source,
			Rule:        HLSKeyMaterialRuleNonStandardRaw24Or32,
			NonStandard: true,
		}}
	default:
		return nil
	}
}

func FirstHLSSegmentProbe(manifestURL string, body []byte) (HLSSegmentProbe, bool) {
	mediaSequence := int64(0)
	var currentIV []byte
	currentMethod := "NONE"
	expectSegment := false
	expectVariant := false
	var byteRange HLSByteRange
	hasByteRange := false
	byteRangeOffset := int64(0)
	for _, rawLine := range strings.Split(strings.TrimSpace(string(body)), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		upperLine := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(upperLine, "#EXT-X-MEDIA-SEQUENCE:"):
			value := strings.TrimSpace(line[len("#EXT-X-MEDIA-SEQUENCE:"):])
			if parsed, err := strconv.ParseInt(value, 10, 64); err == nil && parsed >= 0 {
				mediaSequence = parsed
			}
			continue
		case strings.HasPrefix(upperLine, "#EXT-X-STREAM-INF"):
			expectVariant = true
			continue
		case strings.HasPrefix(upperLine, "#EXT-X-KEY:"):
			attrs := parseHLSAttributeList(line[len("#EXT-X-KEY:"):])
			currentMethod = strings.ToUpper(strings.TrimSpace(attrs["METHOD"]))
			if currentMethod == "" {
				currentMethod = "UNKNOWN"
			}
			if currentMethod == "AES-128" {
				if iv, ok := ParseHLSIV(attrs["IV"]); ok {
					currentIV = iv
				} else {
					currentIV = nil
				}
			} else {
				currentIV = nil
			}
			continue
		case strings.HasPrefix(upperLine, "#EXT-X-BYTERANGE:"):
			if parsedRange, ok := ParseHLSByteRange(line[len("#EXT-X-BYTERANGE:"):], byteRangeOffset); ok {
				byteRange = parsedRange
				hasByteRange = true
			}
			continue
		case strings.HasPrefix(upperLine, "#EXTINF"):
			expectSegment = true
			continue
		case strings.HasPrefix(line, "#"):
			continue
		}
		if expectVariant {
			expectVariant = false
			continue
		}
		if !expectSegment {
			continue
		}
		expectSegment = false
		segmentRange := byteRange
		segmentHasByteRange := hasByteRange
		if hasByteRange {
			byteRangeOffset = byteRange.End
			hasByteRange = false
			byteRange = HLSByteRange{}
		}
		if currentMethod != "AES-128" {
			mediaSequence++
			continue
		}
		iv := currentIV
		if len(iv) != 16 {
			iv = HLSMediaSequenceIV(mediaSequence)
		}
		probe := HLSSegmentProbe{
			URL: ResolveManifestReference(manifestURL, line),
			IV:  append([]byte{}, iv...),
		}
		if segmentHasByteRange {
			probe.ByteRange = segmentRange
			probe.HasByteRange = true
		}
		return probe, true
	}
	return HLSSegmentProbe{}, false
}

func ParseHLSByteRange(value string, previousEnd int64) (HLSByteRange, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return HLSByteRange{}, false
	}
	lengthText, offsetText, hasOffset := strings.Cut(trimmed, "@")
	length, err := strconv.ParseInt(strings.TrimSpace(lengthText), 10, 64)
	if err != nil || length <= 0 {
		return HLSByteRange{}, false
	}
	start := previousEnd
	if hasOffset {
		parsedStart, err := strconv.ParseInt(strings.TrimSpace(offsetText), 10, 64)
		if err != nil || parsedStart < 0 {
			return HLSByteRange{}, false
		}
		start = parsedStart
	}
	return HLSByteRange{Start: start, End: start + length}, true
}

func ParseHLSIV(value string) ([]byte, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, false
	}
	trimmed = strings.TrimPrefix(strings.TrimPrefix(trimmed, "0x"), "0X")
	if trimmed == "" {
		return nil, false
	}
	if len(trimmed)%2 == 1 {
		trimmed = "0" + trimmed
	}
	decoded, err := hex.DecodeString(trimmed)
	if err != nil || len(decoded) == 0 || len(decoded) > 16 {
		return nil, false
	}
	if len(decoded) == 16 {
		return decoded, true
	}
	iv := make([]byte, 16)
	copy(iv[16-len(decoded):], decoded)
	return iv, true
}

func HLSMediaSequenceIV(sequence int64) []byte {
	iv := make([]byte, 16)
	if sequence < 0 {
		return iv
	}
	binary.BigEndian.PutUint64(iv[8:], uint64(sequence))
	return iv
}

func parseHLSAttributeList(value string) map[string]string {
	result := map[string]string{}
	var key strings.Builder
	var val strings.Builder
	inKey := true
	inQuote := false
	flush := func() {
		trimmedKey := strings.ToUpper(strings.TrimSpace(key.String()))
		trimmedValue := strings.TrimSpace(val.String())
		trimmedValue = strings.Trim(trimmedValue, `"`)
		if trimmedKey != "" {
			result[trimmedKey] = trimmedValue
		}
		key.Reset()
		val.Reset()
		inKey = true
	}
	for _, r := range value {
		switch {
		case inKey && r == '=':
			inKey = false
		case !inKey && r == '"':
			inQuote = !inQuote
			val.WriteRune(r)
		case !inKey && r == ',' && !inQuote:
			flush()
		default:
			if inKey {
				key.WriteRune(r)
			} else {
				val.WriteRune(r)
			}
		}
	}
	flush()
	return result
}

func parseHLSEXTINFDurationMs(line string) int64 {
	_, rawValue, ok := strings.Cut(strings.TrimSpace(line), ":")
	if !ok {
		return 0
	}
	durationValue, _, _ := strings.Cut(rawValue, ",")
	seconds, err := strconv.ParseFloat(strings.TrimSpace(durationValue), 64)
	if err != nil || seconds <= 0 {
		return 0
	}
	return int64(seconds * 1000)
}

func registerHLSDRM(attrs map[string]string, systems map[string]struct{}) {
	keyFormat := strings.ToLower(strings.TrimSpace(attrs["KEYFORMAT"]))
	uri := strings.ToLower(strings.TrimSpace(attrs["URI"]))
	switch {
	case strings.Contains(keyFormat, "com.apple.streamingkeydelivery"), strings.HasPrefix(uri, "skd://"):
		systems["fairplay"] = struct{}{}
	case strings.Contains(keyFormat, "com.microsoft.playready"):
		systems["playready"] = struct{}{}
	case strings.Contains(keyFormat, "widevine"):
		systems["widevine"] = struct{}{}
	case keyFormat != "" && keyFormat != "identity":
		systems[keyFormat] = struct{}{}
	}
}

func resolveHLSEncryptionType(methods []string, drm bool) string {
	if drm {
		return StreamEncryptionDRM
	}
	if len(methods) == 0 {
		return StreamEncryptionNone
	}
	for _, method := range methods {
		switch strings.ToUpper(strings.TrimSpace(method)) {
		case "", "NONE":
			continue
		case "AES-128":
			return StreamEncryptionAES128
		case "SAMPLE-AES":
			return StreamEncryptionSampleAES
		case "SAMPLE-AES-CTR":
			return StreamEncryptionSampleAESCTR
		case "AES-256-GCM":
			return StreamEncryptionAES256GCM
		default:
			return StreamEncryptionUnknown
		}
	}
	return StreamEncryptionNone
}

func unsupportedHLSEncryptionMethod(methods []string) string {
	for _, method := range methods {
		method = strings.ToUpper(strings.TrimSpace(method))
		switch method {
		case "", "NONE", "AES-128":
			continue
		default:
			return method
		}
	}
	return ""
}

func dashDRMSystemName(systemID string) string {
	switch strings.ToLower(strings.TrimSpace(systemID)) {
	case "edef8ba9-79d6-4ace-a3c8-27dcd51d21ed":
		return "widevine"
	case "9a04f079-9840-4286-ab92-e65be0885f95":
		return "playready"
	case "94ce86fb-07ff-4f43-adb8-93d2fa968ca2":
		return "fairplay"
	case "e2719d58-a985-b3c9-781a-b030af78d30e":
		return "clearkey"
	default:
		return strings.ToLower(strings.TrimSpace(systemID))
	}
}

func streamURLPathExtension(rawURL string) string {
	trimmed := strings.TrimSpace(rawURL)
	if parsed, err := url.Parse(trimmed); err == nil {
		trimmed = parsed.Path
	}
	return strings.ToLower(strings.TrimPrefix(path.Ext(trimmed), "."))
}

func ResolveManifestReference(baseURL string, ref string) string {
	trimmedRef := strings.TrimSpace(ref)
	if trimmedRef == "" {
		return ""
	}
	parsedRef, err := url.Parse(trimmedRef)
	if err == nil && parsedRef.IsAbs() {
		return trimmedRef
	}
	parsedBase, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return trimmedRef
	}
	resolved, err := url.Parse(trimmedRef)
	if err != nil {
		return trimmedRef
	}
	return parsedBase.ResolveReference(resolved).String()
}

func ManifestRawQuery(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(parsed.RawQuery)
}

func AppendRawQuery(rawURL string, rawQuery string) string {
	trimmedQuery := strings.TrimSpace(rawQuery)
	if trimmedQuery == "" {
		return strings.TrimSpace(rawURL)
	}
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return strings.TrimSpace(rawURL)
	}
	existing, _ := url.ParseQuery(parsed.RawQuery)
	extra, err := url.ParseQuery(trimmedQuery)
	if err != nil {
		return strings.TrimSpace(rawURL)
	}
	for key, values := range extra {
		if strings.TrimSpace(key) == "" {
			continue
		}
		existing[key] = values
	}
	parsed.RawQuery = existing.Encode()
	return parsed.String()
}

func isKnownHLSMediaSegmentExtension(ext string) bool {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case "ts", "m2ts", "mpegts", "m4s", "mp4", "m4a", "aac", "ac3", "eac3", "mp3", "vtt", "webvtt", "cmfv", "cmfa", "fmp4":
		return true
	default:
		return false
	}
}

func sortedKeys(values map[string]struct{}) []string {
	if len(values) == 0 {
		return nil
	}
	items := make([]string, 0, len(values))
	for item := range values {
		if strings.TrimSpace(item) != "" {
			items = append(items, strings.TrimSpace(item))
		}
	}
	sort.Strings(items)
	return items
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func LooksLikeASCIIHex(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	for _, b := range data {
		if !((b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')) {
			return false
		}
	}
	return true
}
