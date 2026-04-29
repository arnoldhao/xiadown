package update

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"path"
	"path/filepath"
	"time"
)

const (
	defaultDownloadAttempts       = 4
	defaultDownloadAttemptTimeout = 20 * time.Minute
	defaultDownloadBackoffBase    = 750 * time.Millisecond
)

type HTTPDownloader struct {
	client         *http.Client
	clientProvider interface {
		HTTPClient() *http.Client
	}
	maxAttempts    int
	attemptTimeout time.Duration
	backoffBase    time.Duration
}

func NewHTTPDownloader(client *http.Client) *HTTPDownloader {
	return &HTTPDownloader{
		client:         cloneDownloadClient(client),
		maxAttempts:    defaultDownloadAttempts,
		attemptTimeout: defaultDownloadAttemptTimeout,
		backoffBase:    defaultDownloadBackoffBase,
	}
}

func NewHTTPDownloaderWithClientProvider(provider interface {
	HTTPClient() *http.Client
}) *HTTPDownloader {
	return &HTTPDownloader{
		clientProvider: provider,
		maxAttempts:    defaultDownloadAttempts,
		attemptTimeout: defaultDownloadAttemptTimeout,
		backoffBase:    defaultDownloadBackoffBase,
	}
}

func (downloader *HTTPDownloader) Download(ctx context.Context, url string, progress func(int)) (string, error) {
	if downloader == nil || downloader.httpClient() == nil {
		return "", fmt.Errorf("http client not configured")
	}
	if url == "" {
		return "", fmt.Errorf("download url is empty")
	}

	tempDir, err := os.MkdirTemp("", "xiadown-update-*")
	if err != nil {
		return "", err
	}
	dest := filepath.Join(tempDir, downloadFileName(url))

	var lastErr error
	for attempt := 1; attempt <= downloader.maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			_ = os.RemoveAll(tempDir)
			return "", err
		}
		lastErr = downloader.downloadAttempt(ctx, url, dest, progress)
		if lastErr == nil {
			if progress != nil {
				progress(100)
			}
			return dest, nil
		}
		if !isRetryableDownloadError(lastErr) || attempt == downloader.maxAttempts {
			break
		}
		if err := sleepWithContext(ctx, downloader.retryBackoff(attempt)); err != nil {
			_ = os.RemoveAll(tempDir)
			return "", err
		}
	}

	_ = os.RemoveAll(tempDir)
	return "", finalizeDownloadError(lastErr, downloader.maxAttempts)
}

func (downloader *HTTPDownloader) downloadAttempt(ctx context.Context, url string, dest string, progress func(int)) error {
	attemptCtx, cancel := context.WithTimeout(ctx, downloader.attemptTimeout)
	defer cancel()

	existing, err := fileSize(dest)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(attemptCtx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if existing > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existing))
	}

	client := downloader.httpClient()
	if client == nil {
		return fmt.Errorf("http client not configured")
	}
	resp, err := client.Do(req)
	if err != nil {
		if parentErr := ctx.Err(); parentErr != nil {
			return parentErr
		}
		return markRetryable(err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusRequestedRangeNotSatisfiable && existing > 0:
		if err := os.Remove(dest); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return markRetryable(fmt.Errorf("download range rejected by server"))
	case resp.StatusCode >= 500:
		return markRetryable(fmt.Errorf("download failed: http %d", resp.StatusCode))
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusRequestTimeout:
		return markRetryable(fmt.Errorf("download failed: http %d", resp.StatusCode))
	case resp.StatusCode < 200 || resp.StatusCode >= 400:
		return fmt.Errorf("download failed: http %d", resp.StatusCode)
	}

	appendMode := existing > 0 && resp.StatusCode == http.StatusPartialContent
	if appendMode {
		if start, ok := parseContentRangeStart(resp.Header.Get("Content-Range")); ok && start != existing {
			return markRetryable(fmt.Errorf("download resume offset mismatch"))
		}
	}

	file, err := openDownloadFile(dest, appendMode)
	if err != nil {
		return err
	}
	defer file.Close()

	written := existing
	if !appendMode {
		written = 0
	}
	total := totalBytes(resp, written)
	if progress != nil {
		progress(percent(written, total))
	}

	buf := make([]byte, 32*1024)
	lastReport := time.Now()

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := file.Write(buf[:n]); err != nil {
				return err
			}
			written += int64(n)
		}
		if progress != nil && (time.Since(lastReport) > 200*time.Millisecond || (total > 0 && written >= total)) {
			progress(percent(written, total))
			lastReport = time.Now()
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			if parentErr := ctx.Err(); parentErr != nil {
				return parentErr
			}
			return markRetryable(readErr)
		}
	}

	return nil
}

func (downloader *HTTPDownloader) httpClient() *http.Client {
	if downloader == nil {
		return nil
	}
	if downloader.clientProvider != nil {
		return cloneDownloadClient(downloader.clientProvider.HTTPClient())
	}
	return downloader.client
}

func cloneDownloadClient(base *http.Client) *http.Client {
	if base == nil {
		return nil
	}

	cloned := *base
	if transport, ok := base.Transport.(*http.Transport); ok && transport != nil {
		cloned.Transport = transport.Clone()
	} else if base.Transport == nil {
		if defaultTransport, ok := http.DefaultTransport.(*http.Transport); ok {
			cloned.Transport = defaultTransport.Clone()
		}
	}
	// Large installers should not inherit the short global request timeout used by
	// generic API calls. Per-attempt timeouts are enforced via context instead.
	cloned.Timeout = 0
	return &cloned
}

func downloadFileName(rawURL string) string {
	parsed, err := neturl.Parse(rawURL)
	if err == nil {
		if name := path.Base(parsed.Path); name != "" && name != "." && name != "/" {
			return name
		}
	}
	return "xiadown-update.bin"
}

func fileSize(dest string) (int64, error) {
	info, err := os.Stat(dest)
	if err == nil {
		return info.Size(), nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	return 0, err
}

func openDownloadFile(dest string, appendMode bool) (*os.File, error) {
	if appendMode {
		return os.OpenFile(dest, os.O_WRONLY|os.O_APPEND, 0o644)
	}
	return os.Create(dest)
}

func totalBytes(resp *http.Response, startOffset int64) int64 {
	if resp == nil {
		return 0
	}
	if total, ok := parseContentRangeTotal(resp.Header.Get("Content-Range")); ok {
		return total
	}
	if resp.ContentLength <= 0 {
		return 0
	}
	if resp.StatusCode == http.StatusPartialContent {
		return startOffset + resp.ContentLength
	}
	return resp.ContentLength
}

type retryableDownloadError struct {
	err error
}

func (err retryableDownloadError) Error() string {
	return err.err.Error()
}

func (err retryableDownloadError) Unwrap() error {
	return err.err
}

func markRetryable(err error) error {
	if err == nil {
		return nil
	}
	var retryable retryableDownloadError
	if errors.As(err, &retryable) {
		return err
	}
	return retryableDownloadError{err: err}
}

func isRetryableDownloadError(err error) bool {
	var retryable retryableDownloadError
	return errors.As(err, &retryable)
}

func finalizeDownloadError(err error, attempts int) error {
	if err == nil {
		return nil
	}
	if !isRetryableDownloadError(err) || attempts <= 1 {
		return err
	}
	return fmt.Errorf("download failed after %d attempts: %w", attempts, errors.Unwrap(err))
}

func parseContentRangeStart(header string) (int64, bool) {
	var start int64
	var end int64
	var total int64
	if _, err := fmt.Sscanf(header, "bytes %d-%d/%d", &start, &end, &total); err != nil {
		return 0, false
	}
	return start, true
}

func parseContentRangeTotal(header string) (int64, bool) {
	var start int64
	var end int64
	var total int64
	if _, err := fmt.Sscanf(header, "bytes %d-%d/%d", &start, &end, &total); err != nil {
		return 0, false
	}
	return total, true
}

func (downloader *HTTPDownloader) retryBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		return downloader.backoffBase
	}
	backoff := downloader.backoffBase * time.Duration(1<<(attempt-1))
	if backoff > 6*time.Second {
		return 6 * time.Second
	}
	return backoff
}

func sleepWithContext(ctx context.Context, wait time.Duration) error {
	timer := time.NewTimer(wait)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func percent(written int64, total int64) int {
	if total <= 0 {
		return 0
	}
	p := int(float64(written) / float64(total) * 100)
	if p > 100 {
		return 100
	}
	if p < 0 {
		return 0
	}
	return p
}
