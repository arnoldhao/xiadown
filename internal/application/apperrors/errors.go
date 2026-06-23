package apperrors

import (
	"errors"
	"fmt"
	"strings"
)

type Code string

const (
	CodeUnknown           Code = "unknown_error"
	CodeInvalidInput      Code = "invalid_input"
	CodeDependencyMissing Code = "dependency_missing"
	CodeAuthRequired      Code = "auth_required"
	CodeRateLimited       Code = "rate_limited"
	CodeNetworkError      Code = "network_error"
	CodeParsing           Code = "parsing_error"

	CodeDownloadURLRequired    Code = "download_url_required"
	CodeDownloadURLInvalid     Code = "download_url_invalid"
	CodeDownloadURLUnsupported Code = "download_url_unsupported"
	CodeDownloadURLMultiple    Code = "download_url_multiple"
	CodeDownloadBatchEmpty     Code = "download_batch_empty"
	CodeDownloadBatchTooLarge  Code = "download_batch_too_large"

	CodeResourceUnsupportedDomain    Code = "resource_unsupported_domain"
	CodeResourceBrowserUnavailable   Code = "resource_browser_unavailable"
	CodeResourceBrowserLaunchFailed  Code = "resource_browser_launch_failed"
	CodeResourceResolveFailed        Code = "resource_resolve_failed"
	CodeResourceVerificationRequired Code = "resource_verification_required"
	CodeResourceNoMediaDetected      Code = "resource_no_media_detected"
	CodeResourceDownloadFailed       Code = "resource_download_failed"
	CodeResourceOutputFailed         Code = "resource_output_failed"
)

type Error struct {
	Code    Code
	Message string
	Err     error
}

func New(code Code, message string) error {
	return &Error{Code: code, Message: strings.TrimSpace(message)}
}

func Newf(code Code, format string, args ...any) error {
	return New(code, fmt.Sprintf(format, args...))
}

func Wrap(code Code, message string, err error) error {
	return &Error{Code: code, Message: strings.TrimSpace(message), Err: err}
}

func Wrapf(code Code, err error, format string, args ...any) error {
	return Wrap(code, fmt.Sprintf(format, args...), err)
}

func (err *Error) Error() string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(err.Message)
	if message == "" && err.Err != nil {
		message = err.Err.Error()
	}
	code := strings.TrimSpace(string(err.Code))
	if code == "" {
		return message
	}
	if message == "" {
		return "[" + code + "]"
	}
	return "[" + code + "] " + message
}

func (err *Error) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Err
}

func CodeOf(err error) Code {
	if err == nil {
		return ""
	}
	var coded *Error
	if errors.As(err, &coded) && coded != nil {
		return coded.Code
	}
	return ""
}
