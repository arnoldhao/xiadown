package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	appytdlp "xiadown/internal/application/ytdlp"
)

const (
	ytdlpStreamPreflightTotalTimeout    = 24 * time.Second
	ytdlpStreamManifestPreflightTimeout = 10 * time.Second
	ytdlpStreamMediaPlaylistTimeout     = 8 * time.Second
	ytdlpStreamKeyProbeTimeout          = 6 * time.Second
	ytdlpStreamSegmentProbeTimeout      = 12 * time.Second
	ytdlpStreamManifestMaxBytes         = 2 << 20
	ytdlpStreamKeyProbeMaxBytes         = 1024
	ytdlpStreamVariantProbeLimit        = 3
)

func (service *LibraryService) preflightYTDLPStream(ctx context.Context, rawURL string, headers map[string]string, reporter *ytdlpProgressReporter, operationID string) appytdlp.StreamManifestPreflight {
	if !appytdlp.LooksLikeStreamManifestURL(rawURL) {
		return appytdlp.StreamManifestPreflight{}
	}
	preflightCtx, cancel := context.WithTimeout(ctx, ytdlpStreamPreflightTotalTimeout)
	defer cancel()
	reporter.updateDetail("Fetching metadata", progressText("library.progressDetail.checkingStreamManifest"))
	body, contentType, err := service.fetchYTDLPPreflightURL(preflightCtx, rawURL, headers, ytdlpStreamManifestMaxBytes, ytdlpStreamManifestPreflightTimeout, "manifest")
	if err != nil {
		preflight := appytdlp.StreamManifestPreflight{
			Kind:       appytdlp.StreamManifestKindFromURL(rawURL),
			URL:        strings.TrimSpace(rawURL),
			FetchError: err.Error(),
		}
		if preflight.Kind == appytdlp.StreamManifestKindHLS {
			preflight.Strategy = appytdlp.HLSNativeStreamStrategy("hls_preflight_failed")
		}
		return preflight
	}
	preflight := appytdlp.AnalyzeStreamManifest(rawURL, body, contentType, nil)
	if childPreflight, childBody, childContentType, ok := service.preflightYTDLPHLSMediaPlaylist(preflightCtx, preflight, body, headers, reporter, operationID); ok {
		preflight = childPreflight
		body = childBody
		contentType = childContentType
	}
	if preflight.Kind == appytdlp.StreamManifestKindHLS && strings.EqualFold(preflight.EncryptionType, appytdlp.StreamEncryptionAES128) && strings.TrimSpace(preflight.KeyURI) != "" && !preflight.DRM {
		reporter.updateDetail("Fetching metadata", progressText("library.progressDetail.checkingStreamKey"))
		if keyProbe := service.probeYTDLPHLSKey(preflightCtx, preflight, body, headers); keyProbe != nil {
			variantQuery := preflight.VariantQuery
			variantQueryPassthrough := preflight.VariantQueryPassthrough
			preflight = appytdlp.AnalyzeStreamManifest(preflight.URL, body, contentType, keyProbe)
			preflight.VariantQuery = variantQuery
			preflight.VariantQueryPassthrough = variantQueryPassthrough
			preflight.Strategy = appytdlp.BuildStreamDownloadStrategy(preflight)
		}
	}
	return preflight
}

func (service *LibraryService) preflightYTDLPHLSMediaPlaylist(ctx context.Context, preflight appytdlp.StreamManifestPreflight, body []byte, headers map[string]string, reporter *ytdlpProgressReporter, operationID string) (appytdlp.StreamManifestPreflight, []byte, string, bool) {
	if preflight.Kind != appytdlp.StreamManifestKindHLS || !preflight.HasVariants || preflight.SegmentCount > 0 || preflight.IsUnsupported() {
		return appytdlp.StreamManifestPreflight{}, nil, "", false
	}
	refs := appytdlp.ExtractHLSPlaylistReferences(preflight.URL, body)
	if len(refs) == 0 {
		return appytdlp.StreamManifestPreflight{}, nil, "", false
	}
	for index, ref := range refs {
		if index >= ytdlpStreamVariantProbeLimit {
			break
		}
		reporter.updateDetail("Fetching metadata", progressText("library.progressDetail.checkingMediaPlaylist"))
		childURL := ref
		childBody, childContentType, err := service.fetchYTDLPPreflightURL(ctx, childURL, headers, ytdlpStreamManifestMaxBytes, ytdlpStreamMediaPlaylistTimeout, "media playlist")
		if err != nil {
			if retryURL, ok := manifestQueryRetryURL(ref, preflight.URL); ok {
				if retryBody, retryContentType, retryErr := service.fetchYTDLPPreflightURL(ctx, retryURL, headers, ytdlpStreamManifestMaxBytes, ytdlpStreamMediaPlaylistTimeout, "media playlist"); retryErr == nil {
					childURL = retryURL
					childBody = retryBody
					childContentType = retryContentType
					err = nil
				}
			}
			if err != nil {
				continue
			}
		}
		childPreflight := appytdlp.AnalyzeStreamManifest(childURL, childBody, childContentType, nil)
		if childPreflight.Kind != appytdlp.StreamManifestKindHLS {
			continue
		}
		if childURL != ref {
			childPreflight.VariantQuery = appytdlp.ManifestRawQuery(preflight.URL)
			childPreflight.VariantQueryPassthrough = true
			childPreflight.Strategy = appytdlp.BuildStreamDownloadStrategy(childPreflight)
		}
		return childPreflight, childBody, childContentType, true
	}
	return appytdlp.StreamManifestPreflight{}, nil, "", false
}

func (service *LibraryService) probeYTDLPHLSKey(ctx context.Context, preflight appytdlp.StreamManifestPreflight, manifestBody []byte, headers map[string]string) *appytdlp.StreamKeyProbe {
	resolvedURI := appytdlp.ResolveManifestReference(preflight.URL, preflight.KeyURI)
	if strings.TrimSpace(resolvedURI) == "" {
		return nil
	}
	probe := &appytdlp.StreamKeyProbe{
		URI:         strings.TrimSpace(preflight.KeyURI),
		ResolvedURI: resolvedURI,
		Method:      preflight.EncryptionType,
		KeyFormat:   preflight.KeyFormat,
	}
	body, _, err := service.fetchYTDLPPreflightURL(ctx, resolvedURI, headers, ytdlpStreamKeyProbeMaxBytes, ytdlpStreamKeyProbeTimeout, "key")
	if err != nil {
		if retryURL, ok := manifestQueryRetryURL(resolvedURI, preflight.URL); ok {
			if retryBody, _, retryErr := service.fetchYTDLPPreflightURL(ctx, retryURL, headers, ytdlpStreamKeyProbeMaxBytes, ytdlpStreamKeyProbeTimeout, "key"); retryErr == nil {
				body = retryBody
				err = nil
				probe.KeyQuery = appytdlp.ManifestRawQuery(preflight.URL)
				probe.KeyQueryPassthrough = true
			}
		}
		if err != nil {
			probe.Error = err.Error()
			return probe
		}
	}
	trimmed := []byte(strings.TrimSpace(string(body)))
	if len(trimmed) > 0 && appytdlp.LooksLikeASCIIHex(trimmed) {
		body = trimmed
	}
	probe.LengthBytes = len(body)
	probe.LooksASCIIHex = appytdlp.LooksLikeASCIIHex(body)
	selection := service.selectYTDLPHLSKeyMaterial(ctx, preflight, manifestBody, headers, body)
	if selection.material.KeyHex != "" {
		probe.NormalizedLengthBytes = selection.material.LengthBytes
		probe.NormalizedKeySource = selection.material.Source
		probe.NormalizedKeyRule = selection.material.Rule
		probe.NormalizedKeyNonStandard = selection.material.NonStandard
		probe.ManifestKeyOverride = selection.material.ManifestKeyOverride
		probe.NormalizedKeyHex = selection.material.KeyHex
	}
	probe.DecryptionValidated = selection.validated
	probe.DecryptionValidationFormat = selection.format
	probe.DecryptionValidationSegment = selection.segmentURL
	probe.DecryptionValidationError = selection.err
	if strings.TrimSpace(selection.fragmentQuery) != "" {
		probe.FragmentQuery = selection.fragmentQuery
		probe.FragmentQueryPassthrough = true
	}
	return probe
}

type hlsKeyMaterialSelection struct {
	material      appytdlp.HLSKeyMaterial
	validated     bool
	format        string
	segmentURL    string
	fragmentQuery string
	err           string
}

func (service *LibraryService) selectYTDLPHLSKeyMaterial(ctx context.Context, preflight appytdlp.StreamManifestPreflight, manifestBody []byte, headers map[string]string, keyBody []byte) hlsKeyMaterialSelection {
	candidates := appytdlp.HLSKeyMaterialCandidatesWithContext(appytdlp.HLSKeyMaterialCandidateContext{
		ManifestURL:  preflight.URL,
		ManifestBody: manifestBody,
		KeyURI:       preflight.KeyURI,
		KeyBody:      keyBody,
		KeyFormat:    preflight.KeyFormat,
	})
	if len(candidates) == 0 {
		return hlsKeyMaterialSelection{
			err: "no valid AES key candidates",
		}
	}
	segment, ok := appytdlp.FirstHLSSegmentProbe(preflight.URL, manifestBody)
	if !ok || strings.TrimSpace(segment.URL) == "" || len(segment.IV) != aes.BlockSize {
		return hlsKeyMaterialSelection{
			err: "first media segment was not found",
		}
	}
	encrypted, _, err := service.fetchYTDLPPreflightRange(ctx, segment.URL, headers, segmentRangeStart(segment), 512, ytdlpStreamSegmentProbeTimeout, "segment probe")
	fragmentQuery := ""
	if err != nil {
		if retryURL, ok := manifestQueryRetryURL(segment.URL, preflight.URL); ok {
			if retryEncrypted, _, retryErr := service.fetchYTDLPPreflightRange(ctx, retryURL, headers, segmentRangeStart(segment), 512, ytdlpStreamSegmentProbeTimeout, "segment probe"); retryErr == nil {
				encrypted = retryEncrypted
				err = nil
				fragmentQuery = appytdlp.ManifestRawQuery(preflight.URL)
			}
		}
	}
	if err != nil || len(encrypted) < aes.BlockSize {
		detail := "first media segment request failed"
		if err != nil {
			detail = err.Error()
		}
		return hlsKeyMaterialSelection{
			segmentURL: segment.URL,
			err:        detail,
		}
	}
	for _, candidate := range candidates {
		plain, ok := decryptHLSProbeBytes(encrypted, candidate, segment.IV)
		if !ok {
			continue
		}
		if format, ok := playableHLSProbeFormat(plain); ok {
			return hlsKeyMaterialSelection{
				material:      candidate,
				validated:     true,
				format:        format,
				segmentURL:    segment.URL,
				fragmentQuery: fragmentQuery,
			}
		}
	}
	return hlsKeyMaterialSelection{
		segmentURL: segment.URL,
		err:        "candidate keys did not produce a playable first segment",
	}
}

func (service *LibraryService) fetchYTDLPPreflightURL(ctx context.Context, rawURL string, headers map[string]string, limit int64, timeout time.Duration, label string) ([]byte, string, error) {
	if timeout <= 0 {
		timeout = ytdlpStreamManifestPreflightTimeout
	}
	label = strings.TrimSpace(label)
	if label == "" {
		label = "manifest"
	}
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, strings.TrimSpace(rawURL), nil)
	if err != nil {
		return nil, "", err
	}
	for key, value := range headers {
		trimmedKey := strings.TrimSpace(key)
		trimmedValue := strings.TrimSpace(value)
		if trimmedKey == "" || trimmedValue == "" {
			continue
		}
		req.Header.Set(trimmedKey, trimmedValue)
	}
	client := service.ytdlpAuxiliaryHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.Header.Get("Content-Type"), fmt.Errorf("%s request returned status %d", label, resp.StatusCode)
	}
	if limit <= 0 {
		limit = ytdlpStreamManifestMaxBytes
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, resp.Header.Get("Content-Type"), err
	}
	if int64(len(data)) > limit {
		return nil, resp.Header.Get("Content-Type"), fmt.Errorf("%s is larger than %d bytes", label, limit)
	}
	return data, resp.Header.Get("Content-Type"), nil
}

func (service *LibraryService) fetchYTDLPPreflightRange(ctx context.Context, rawURL string, headers map[string]string, start int64, limit int64, timeout time.Duration, label string) ([]byte, string, error) {
	if start < 0 {
		start = 0
	}
	if limit <= 0 {
		limit = 512
	}
	if timeout <= 0 {
		timeout = ytdlpStreamKeyProbeTimeout
	}
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, strings.TrimSpace(rawURL), nil)
	if err != nil {
		return nil, "", err
	}
	applyResourceRequestHeaders(req, headers)
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, start+limit-1))
	client := service.ytdlpAuxiliaryHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return nil, resp.Header.Get("Content-Type"), fmt.Errorf("%s request returned status %d", strings.TrimSpace(label), resp.StatusCode)
	}
	if start > 0 && resp.StatusCode != http.StatusPartialContent {
		return nil, resp.Header.Get("Content-Type"), fmt.Errorf("%s request ignored byte range", strings.TrimSpace(label))
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return nil, resp.Header.Get("Content-Type"), err
	}
	return data, resp.Header.Get("Content-Type"), nil
}

func manifestQueryRetryURL(rawURL string, manifestURL string) (string, bool) {
	query := appytdlp.ManifestRawQuery(manifestURL)
	if strings.TrimSpace(query) == "" {
		return "", false
	}
	retryURL := appytdlp.AppendRawQuery(rawURL, query)
	if strings.TrimSpace(retryURL) == "" || retryURL == strings.TrimSpace(rawURL) {
		return "", false
	}
	return retryURL, true
}

func segmentRangeStart(segment appytdlp.HLSSegmentProbe) int64 {
	if segment.HasByteRange && segment.ByteRange.Start > 0 {
		return segment.ByteRange.Start
	}
	return 0
}

func decryptHLSProbeBytes(encrypted []byte, material appytdlp.HLSKeyMaterial, iv []byte) ([]byte, bool) {
	key, err := hex.DecodeString(strings.TrimSpace(material.KeyHex))
	if err != nil {
		return nil, false
	}
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, false
	}
	if len(iv) != aes.BlockSize {
		return nil, false
	}
	size := len(encrypted) - (len(encrypted) % aes.BlockSize)
	if size <= 0 {
		return nil, false
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, false
	}
	plain := make([]byte, size)
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plain, encrypted[:size])
	return plain, true
}

func playableHLSProbeFormat(data []byte) (string, bool) {
	if len(data) >= 188 && data[0] == 0x47 {
		if len(data) < 376 || data[188] == 0x47 {
			return "mpegts", true
		}
	}
	if len(data) >= 8 {
		box := string(data[4:8])
		switch box {
		case "ftyp", "styp", "moof", "mdat":
			return "fmp4", true
		}
	}
	return "", false
}

func appendYTDLPStreamPreflightMetadata(payload map[string]any, preflight appytdlp.StreamManifestPreflight) map[string]any {
	metadata := preflight.MetadataMap()
	if len(metadata) == 0 {
		return payload
	}
	if payload == nil {
		payload = map[string]any{}
	}
	payload["streamPreflight"] = metadata
	return payload
}
