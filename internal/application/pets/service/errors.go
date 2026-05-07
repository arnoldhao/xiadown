package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	petErrorCodePackagePathRequired       = "pet_package_path_required"
	petErrorCodePackageUnsupportedType    = "pet_package_unsupported_type"
	petErrorCodePackageReadFailed         = "pet_package_read_failed"
	petErrorCodePackageTooLarge           = "pet_package_too_large"
	petErrorCodePackageOpenFailed         = "pet_package_open_failed"
	petErrorCodePackageMissingManifest    = "pet_package_missing_manifest"
	petErrorCodePackageMissingSpritesheet = "pet_package_missing_spritesheet"
	petErrorCodePackageContentsTooLarge   = "pet_package_contents_too_large"
	petErrorCodeArchiveFileOpenFailed     = "pet_archive_file_open_failed"
	petErrorCodeArchiveFileReadFailed     = "pet_archive_file_read_failed"
	petErrorCodeManifestDecodeFailed      = "pet_manifest_decode_failed"
	petErrorCodeSpritesheetDecodeFailed   = "pet_spritesheet_decode_failed"
	petErrorCodeSpritesheetSizeInvalid    = "pet_spritesheet_size_invalid"
	petErrorCodeOnlineDownloadCanceled    = "pet_online_download_canceled"
	petErrorCodeOnlineSessionRequired     = "pet_online_session_required"
	petErrorCodeOnlineSessionNotFound     = "pet_online_session_not_found"
	petErrorCodeOnlineUnsupportedSite     = "pet_online_unsupported_site"
)

type petError struct {
	Code    string
	Message string
	Cause   error
}

func newPetError(code string, message string) error {
	return petError{Code: strings.TrimSpace(code), Message: strings.TrimSpace(message)}
}

func newPetErrorf(code string, format string, args ...any) error {
	return newPetError(code, fmt.Sprintf(format, args...))
}

func wrapPetError(code string, cause error, message string) error {
	return petError{Code: strings.TrimSpace(code), Message: strings.TrimSpace(message), Cause: cause}
}

func wrapPetErrorf(code string, cause error, format string, args ...any) error {
	return wrapPetError(code, cause, fmt.Sprintf(format, args...))
}

func (err petError) Error() string {
	message := strings.TrimSpace(err.Message)
	if message == "" {
		message = strings.TrimSpace(err.Code)
	}
	payload, marshalErr := json.Marshal(struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}{
		Code:    strings.TrimSpace(err.Code),
		Message: message,
	})
	if marshalErr != nil {
		return message
	}
	return string(payload)
}

func (err petError) Unwrap() error {
	return err.Cause
}

func (err petError) PetErrorCode() string {
	return strings.TrimSpace(err.Code)
}

func (err petError) PetErrorMessage() string {
	return strings.TrimSpace(err.Message)
}

type codedPetError interface {
	error
	PetErrorCode() string
	PetErrorMessage() string
}

func petErrorDetails(err error) (string, string) {
	if err == nil {
		return "", ""
	}
	var coded codedPetError
	if errors.As(err, &coded) {
		return strings.TrimSpace(coded.PetErrorCode()), strings.TrimSpace(coded.PetErrorMessage())
	}
	return "", strings.TrimSpace(err.Error())
}
