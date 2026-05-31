package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	resourceDownloadProgressInterval = 300 * time.Millisecond
	resourceDownloadMaxRetries       = 3
	resourceDownloadRetryDelay       = 800 * time.Millisecond
	resourceDownloadMinPartSize      = int64(1024 * 1024)
	resourceDownloadDefaultParts     = 4
	resourceDefaultUserAgent         = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36"
	resourceDefaultAccept            = "*/*"
	resourceDefaultAcceptLanguage    = "zh-CN,zh;q=0.9,en;q=0.8"
)

var resourceForbiddenDownloadHeaders = map[string]struct{}{
	":authority":                  {},
	":method":                     {},
	":path":                       {},
	":scheme":                     {},
	"accept-encoding":             {},
	"connection":                  {},
	"content-length":              {},
	"host":                        {},
	"if-modified-since":           {},
	"if-none-match":               {},
	"keep-alive":                  {},
	"proxy-connection":            {},
	"range":                       {},
	"sec-ch-ua":                   {},
	"sec-ch-ua-arch":              {},
	"sec-ch-ua-bitness":           {},
	"sec-ch-ua-full-version":      {},
	"sec-ch-ua-full-version-list": {},
	"sec-ch-ua-mobile":            {},
	"sec-ch-ua-model":             {},
	"sec-ch-ua-platform":          {},
	"sec-ch-ua-platform-version":  {},
	"sec-fetch-dest":              {},
	"sec-fetch-mode":              {},
	"sec-fetch-site":              {},
	"sec-fetch-user":              {},
	"transfer-encoding":           {},
	"x-forwarded-for":             {},
	"x-real-ip":                   {},
}

type resourceDownloadOptions struct {
	URL        string
	OutputPath string
	Headers    map[string]string
	ProxyURL   string
	TotalSize  int64
	Progress   func(downloaded int64, total int64, speed string)
}

type resourceDownloadResult struct {
	Path      string
	SizeBytes int64
}

type resourceDownloadProbe struct {
	TotalSize    int64
	AcceptRanges bool
}

type resourceDownloadPart struct {
	Index int
	Start int64
	End   int64
}

func (service *LibraryService) downloadResourceFile(ctx context.Context, options resourceDownloadOptions) (resourceDownloadResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	rawURL := strings.TrimSpace(options.URL)
	if rawURL == "" {
		return resourceDownloadResult{}, fmt.Errorf("resource url is required")
	}
	outputPath := strings.TrimSpace(options.OutputPath)
	if outputPath == "" {
		return resourceDownloadResult{}, fmt.Errorf("output path is required")
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return resourceDownloadResult{}, err
	}

	client, err := newResourceHTTPClient(options.ProxyURL)
	if err != nil {
		return resourceDownloadResult{}, err
	}

	resolvedPath, err := reserveUniqueResourceOutputPath(outputPath)
	if err != nil {
		return resourceDownloadResult{}, err
	}
	file, err := os.OpenFile(resolvedPath, os.O_RDWR|os.O_TRUNC, 0o644)
	if err != nil {
		_ = os.Remove(resolvedPath)
		return resourceDownloadResult{}, err
	}
	removeOnError := true
	defer func() {
		_ = file.Close()
		if removeOnError {
			_ = os.Remove(resolvedPath)
		}
	}()

	probe, _ := probeResourceDownload(ctx, client, rawURL, options.Headers, options.TotalSize)
	totalSize := firstPositiveInt64(probe.TotalSize, options.TotalSize)
	progress := newResourceDownloadProgress(options.Progress)
	var sizeBytes int64
	if probe.AcceptRanges && totalSize > resourceDownloadMinPartSize {
		sizeBytes, err = downloadResourceMultipart(ctx, client, file, rawURL, options.Headers, totalSize, progress)
		if err != nil {
			if truncateErr := file.Truncate(0); truncateErr != nil {
				return resourceDownloadResult{}, truncateErr
			}
			if _, seekErr := file.Seek(0, 0); seekErr != nil {
				return resourceDownloadResult{}, seekErr
			}
			progress.Reset()
			sizeBytes, err = downloadResourceSingle(ctx, client, file, rawURL, options.Headers, options.TotalSize, progress)
		}
	} else {
		sizeBytes, err = downloadResourceSingle(ctx, client, file, rawURL, options.Headers, options.TotalSize, progress)
	}
	if err != nil {
		return resourceDownloadResult{}, err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(resolvedPath)
		return resourceDownloadResult{}, err
	}
	removeOnError = false
	return resourceDownloadResult{Path: resolvedPath, SizeBytes: sizeBytes}, nil
}

func probeResourceDownload(ctx context.Context, client *http.Client, rawURL string, headers map[string]string, fallbackSize int64) (resourceDownloadProbe, error) {
	probe := resourceDownloadProbe{TotalSize: fallbackSize}
	var lastErr error
	for attempt := 0; attempt < resourceDownloadMaxRetries; attempt++ {
		headProbe, headErr, headTerminal := probeResourceDownloadHead(ctx, client, rawURL, headers, fallbackSize)
		if headErr == nil {
			probe = headProbe
			if probe.TotalSize > 0 || probe.AcceptRanges {
				return probe, nil
			}
		} else {
			lastErr = headErr
			if headTerminal {
				return probe, lastErr
			}
		}

		rangeProbe, rangeErr, rangeTerminal := probeResourceDownloadRange(ctx, client, rawURL, headers, firstPositiveInt64(headProbe.TotalSize, fallbackSize))
		if rangeErr == nil {
			probe.TotalSize = firstPositiveInt64(rangeProbe.TotalSize, probe.TotalSize)
			probe.AcceptRanges = probe.AcceptRanges || rangeProbe.AcceptRanges
			return probe, nil
		}
		if lastErr == nil {
			lastErr = rangeErr
		}
		if rangeTerminal {
			return probe, lastErr
		}
		if !sleepResourceDownloadRetry(ctx, attempt) {
			return probe, lastErr
		}
	}
	return probe, lastErr
}

func probeResourceDownloadHead(ctx context.Context, client *http.Client, rawURL string, headers map[string]string, fallbackSize int64) (resourceDownloadProbe, error, bool) {
	probe := resourceDownloadProbe{TotalSize: fallbackSize}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)
	if err != nil {
		return probe, err, true
	}
	applyResourceRequestHeaders(req, headers)
	resp, err := client.Do(req)
	if err != nil {
		return probe, err, false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return probe, fmt.Errorf("resource probe failed: %s", resp.Status), false
	}
	probe.TotalSize = resourceResponseTotalSize(resp, fallbackSize)
	probe.AcceptRanges = strings.EqualFold(strings.TrimSpace(resp.Header.Get("Accept-Ranges")), "bytes")
	return probe, nil, false
}

func probeResourceDownloadRange(ctx context.Context, client *http.Client, rawURL string, headers map[string]string, fallbackSize int64) (resourceDownloadProbe, error, bool) {
	probe := resourceDownloadProbe{TotalSize: fallbackSize}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return probe, err, true
	}
	applyResourceRequestHeaders(req, headers)
	req.Header.Set("Range", "bytes=0-0")
	resp, err := client.Do(req)
	if err != nil {
		return probe, err, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return probe, fmt.Errorf("resource range probe failed: %s", resp.Status), false
	}
	probe.TotalSize = resourceResponseTotalSize(resp, fallbackSize)
	probe.AcceptRanges = resp.StatusCode == http.StatusPartialContent || strings.EqualFold(strings.TrimSpace(resp.Header.Get("Accept-Ranges")), "bytes")
	return probe, nil, false
}

func downloadResourceSingle(ctx context.Context, client *http.Client, file *os.File, rawURL string, headers map[string]string, fallbackTotal int64, progress *resourceDownloadProgress) (int64, error) {
	var lastErr error
	for attempt := 0; attempt < resourceDownloadMaxRetries; attempt++ {
		if attempt > 0 {
			if err := file.Truncate(0); err != nil {
				return 0, err
			}
			if _, err := file.Seek(0, 0); err != nil {
				return 0, err
			}
			progress.Reset()
		}
		sizeBytes, err := downloadResourceSingleAttempt(ctx, client, file, rawURL, headers, fallbackTotal, progress)
		if err == nil {
			return sizeBytes, nil
		}
		lastErr = err
		if !sleepResourceDownloadRetry(ctx, attempt) {
			return 0, err
		}
	}
	return 0, lastErr
}

func downloadResourceSingleAttempt(ctx context.Context, client *http.Client, file *os.File, rawURL string, headers map[string]string, fallbackTotal int64, progress *resourceDownloadProgress) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, err
	}
	applyResourceRequestHeaders(req, headers)
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		detail := readResourceErrorBody(resp.Body)
		if detail != "" {
			return 0, fmt.Errorf("resource download failed: %s: %s", resp.Status, detail)
		}
		return 0, fmt.Errorf("resource download failed: %s", resp.Status)
	}
	total := resourceResponseTotalSize(resp, fallbackTotal)
	if fallbackTotal > 0 && total <= 0 {
		total = fallbackTotal
	}
	buffer := make([]byte, 128*1024)
	downloaded := int64(0)
	for {
		n, readErr := resp.Body.Read(buffer)
		if n > 0 {
			written, writeErr := file.Write(buffer[:n])
			if writeErr != nil {
				return 0, writeErr
			}
			if written != n {
				return 0, io.ErrShortWrite
			}
			downloaded += int64(n)
			progress.Add(int64(n), total)
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return 0, readErr
		}
	}
	if total <= 0 {
		total = downloaded
	}
	progress.Publish(downloaded, total)
	return downloaded, nil
}

func downloadResourceMultipart(ctx context.Context, client *http.Client, file *os.File, rawURL string, headers map[string]string, totalSize int64, progress *resourceDownloadProgress) (int64, error) {
	if totalSize <= 0 {
		return 0, fmt.Errorf("resource multipart requires known size")
	}
	if err := file.Truncate(totalSize); err != nil {
		return 0, err
	}
	parts := buildResourceDownloadParts(totalSize, resourceDownloadDefaultParts)
	partCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	errCh := make(chan error, len(parts))
	var wg sync.WaitGroup
	for _, part := range parts {
		part := part
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := downloadResourcePartWithRetries(partCtx, client, file, rawURL, headers, part, totalSize, progress); err != nil {
				errCh <- err
				cancel()
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return 0, err
		}
	}
	progress.Publish(totalSize, totalSize)
	return totalSize, nil
}

func buildResourceDownloadParts(totalSize int64, requestedParts int) []resourceDownloadPart {
	if totalSize <= 0 {
		return nil
	}
	if requestedParts <= 0 {
		requestedParts = resourceDownloadDefaultParts
	}
	maxParts := int(totalSize / resourceDownloadMinPartSize)
	if maxParts < 1 {
		maxParts = 1
	}
	if requestedParts > maxParts {
		requestedParts = maxParts
	}
	parts := make([]resourceDownloadPart, 0, requestedParts)
	partSize := totalSize / int64(requestedParts)
	for index := 0; index < requestedParts; index++ {
		start := int64(index) * partSize
		end := start + partSize - 1
		if index == requestedParts-1 {
			end = totalSize - 1
		}
		parts = append(parts, resourceDownloadPart{Index: index, Start: start, End: end})
	}
	return parts
}

func downloadResourcePartWithRetries(ctx context.Context, client *http.Client, file *os.File, rawURL string, headers map[string]string, part resourceDownloadPart, totalSize int64, progress *resourceDownloadProgress) error {
	downloaded := int64(0)
	var lastErr error
	for attempt := 0; attempt < resourceDownloadMaxRetries; attempt++ {
		written, err := downloadResourcePartAttempt(ctx, client, file, rawURL, headers, part.Start+downloaded, part.End, totalSize, progress)
		downloaded += written
		if err == nil {
			return nil
		}
		lastErr = err
		if !sleepResourceDownloadRetry(ctx, attempt) {
			return err
		}
	}
	return lastErr
}

func downloadResourcePartAttempt(ctx context.Context, client *http.Client, file *os.File, rawURL string, headers map[string]string, start int64, end int64, totalSize int64, progress *resourceDownloadProgress) (int64, error) {
	if start > end {
		return 0, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, err
	}
	applyResourceRequestHeaders(req, headers)
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent {
		detail := readResourceErrorBody(resp.Body)
		if detail != "" {
			return 0, fmt.Errorf("resource range download failed: %s: %s", resp.Status, detail)
		}
		return 0, fmt.Errorf("resource range download failed: %s", resp.Status)
	}
	buffer := make([]byte, 128*1024)
	downloaded := int64(0)
	for {
		n, readErr := resp.Body.Read(buffer)
		if n > 0 {
			offset := start + downloaded
			written, writeErr := file.WriteAt(buffer[:n], offset)
			if writeErr != nil {
				return downloaded, writeErr
			}
			if written != n {
				return downloaded, io.ErrShortWrite
			}
			writtenBytes := int64(n)
			downloaded += writtenBytes
			progress.Add(writtenBytes, totalSize)
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return downloaded, readErr
		}
	}
	expected := end - start + 1
	if downloaded != expected {
		return downloaded, fmt.Errorf("resource range download short read: got %d want %d", downloaded, expected)
	}
	return downloaded, nil
}

func sleepResourceDownloadRetry(ctx context.Context, attempt int) bool {
	if attempt >= resourceDownloadMaxRetries-1 {
		return false
	}
	select {
	case <-ctx.Done():
		return false
	case <-time.After(resourceDownloadRetryDelay):
		return true
	}
}

func firstPositiveInt64(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

type resourceDownloadProgress struct {
	mu          sync.Mutex
	callback    func(downloaded int64, total int64, speed string)
	started     time.Time
	lastPublish time.Time
	lastBytes   int64
	downloaded  int64
}

func newResourceDownloadProgress(callback func(downloaded int64, total int64, speed string)) *resourceDownloadProgress {
	now := time.Now()
	return &resourceDownloadProgress{
		callback:    callback,
		started:     now,
		lastPublish: now,
	}
}

func (progress *resourceDownloadProgress) Reset() {
	if progress == nil {
		return
	}
	progress.mu.Lock()
	defer progress.mu.Unlock()
	now := time.Now()
	progress.started = now
	progress.lastPublish = now
	progress.lastBytes = 0
	progress.downloaded = 0
}

func (progress *resourceDownloadProgress) Add(bytes int64, total int64) {
	if progress == nil || bytes <= 0 {
		return
	}
	progress.mu.Lock()
	defer progress.mu.Unlock()
	progress.downloaded += bytes
	now := time.Now()
	if now.Sub(progress.lastPublish) < resourceDownloadProgressInterval {
		return
	}
	speed := formatResourceSpeed(float64(progress.downloaded-progress.lastBytes) / now.Sub(progress.lastPublish).Seconds())
	publishResourceDownloadProgress(progress.callback, progress.downloaded, total, speed)
	progress.lastPublish = now
	progress.lastBytes = progress.downloaded
}

func (progress *resourceDownloadProgress) Publish(downloaded int64, total int64) {
	if progress == nil {
		return
	}
	progress.mu.Lock()
	defer progress.mu.Unlock()
	elapsed := time.Since(progress.started).Seconds()
	speed := ""
	if elapsed > 0 {
		speed = formatResourceSpeed(float64(downloaded) / elapsed)
	}
	publishResourceDownloadProgress(progress.callback, downloaded, total, speed)
}

func newResourceHTTPClient(proxyURL string) (*http.Client, error) {
	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}
	if trimmed := strings.TrimSpace(proxyURL); trimmed != "" {
		parsed, err := url.Parse(trimmed)
		if err != nil {
			return nil, err
		}
		transport.Proxy = http.ProxyURL(parsed)
	}
	return &http.Client{Transport: transport}, nil
}

func applyResourceRequestHeaders(req *http.Request, headers map[string]string) {
	if req == nil {
		return
	}
	for key, value := range headers {
		trimmedKey := strings.TrimSpace(key)
		trimmedValue := strings.TrimSpace(value)
		if trimmedKey == "" || trimmedValue == "" || strings.HasPrefix(trimmedKey, ":") {
			continue
		}
		if _, forbidden := resourceForbiddenDownloadHeaders[strings.ToLower(trimmedKey)]; forbidden {
			continue
		}
		req.Header.Set(trimmedKey, trimmedValue)
	}
	if _, ok := findHeader(httpHeaderToStringMap(req.Header), "User-Agent"); !ok {
		req.Header.Set("User-Agent", resourceDefaultUserAgent)
	}
	if _, ok := findHeader(httpHeaderToStringMap(req.Header), "Accept"); !ok {
		req.Header.Set("Accept", resourceDefaultAccept)
	}
	if _, ok := findHeader(httpHeaderToStringMap(req.Header), "Accept-Language"); !ok {
		req.Header.Set("Accept-Language", resourceDefaultAcceptLanguage)
	}
}

func httpHeaderToStringMap(headers http.Header) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	result := make(map[string]string, len(headers))
	for key, values := range headers {
		if len(values) == 0 {
			continue
		}
		result[key] = values[0]
	}
	return result
}

func resourceResponseTotalSize(resp *http.Response, fallback int64) int64 {
	if resp == nil {
		return fallback
	}
	if resp.StatusCode == http.StatusPartialContent {
		if total := parseContentRangeTotal(resp.Header.Get("Content-Range")); total > 0 {
			return total
		}
	}
	if resp.ContentLength > 0 {
		return resp.ContentLength
	}
	return fallback
}

func parseContentRangeTotal(value string) int64 {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0
	}
	index := strings.LastIndex(trimmed, "/")
	if index < 0 || index+1 >= len(trimmed) {
		return 0
	}
	rawTotal := strings.TrimSpace(trimmed[index+1:])
	if rawTotal == "*" {
		return 0
	}
	total, err := strconv.ParseInt(rawTotal, 10, 64)
	if err != nil || total < 0 {
		return 0
	}
	return total
}

func readResourceErrorBody(reader io.Reader) string {
	if reader == nil {
		return ""
	}
	data, err := io.ReadAll(io.LimitReader(reader, 2048))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func publishResourceDownloadProgress(progress func(downloaded int64, total int64, speed string), downloaded int64, total int64, speed string) {
	if progress == nil {
		return
	}
	progress(downloaded, total, strings.TrimSpace(speed))
}

func formatResourceSpeed(bytesPerSecond float64) string {
	if bytesPerSecond <= 0 {
		return ""
	}
	units := []string{"B/s", "KiB/s", "MiB/s", "GiB/s"}
	value := bytesPerSecond
	unit := 0
	for value >= 1024 && unit < len(units)-1 {
		value /= 1024
		unit++
	}
	precision := 0
	if unit > 0 {
		precision = 1
	}
	return fmt.Sprintf("%.*f %s", precision, value, units[unit])
}

func reserveUniqueResourceOutputPath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", fmt.Errorf("output path is required")
	}
	dir := filepath.Dir(trimmed)
	base := strings.TrimSuffix(filepath.Base(trimmed), filepath.Ext(trimmed))
	ext := filepath.Ext(trimmed)
	if base == "" {
		base = "download"
	}
	for index := 0; index < 1000; index++ {
		candidate := trimmed
		if index > 0 {
			candidate = filepath.Join(dir, fmt.Sprintf("%s-%d%s", base, index+1, ext))
		}
		file, err := os.OpenFile(candidate, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			if closeErr := file.Close(); closeErr != nil {
				_ = os.Remove(candidate)
				return "", closeErr
			}
			return candidate, nil
		}
		if !os.IsExist(err) {
			return "", err
		}
	}
	return "", fmt.Errorf("could not reserve unique output path")
}
