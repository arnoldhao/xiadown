package service

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"xiadown/internal/application/apperrors"
)

const libraryDisplayNameMaxRunes = 160

var libraryReservedDisplayNames = map[string]struct{}{
	"CON":  {},
	"PRN":  {},
	"AUX":  {},
	"NUL":  {},
	"COM1": {},
	"COM2": {},
	"COM3": {},
	"COM4": {},
	"COM5": {},
	"COM6": {},
	"COM7": {},
	"COM8": {},
	"COM9": {},
	"LPT1": {},
	"LPT2": {},
	"LPT3": {},
	"LPT4": {},
	"LPT5": {},
	"LPT6": {},
	"LPT7": {},
	"LPT8": {},
	"LPT9": {},
}

func normalizeLibraryDisplayName(value string) (string, error) {
	name := strings.TrimSpace(value)
	if name == "" {
		return "", apperrors.New(apperrors.CodeInvalidInput, "display name is required")
	}
	if utf8.RuneCountInString(name) > libraryDisplayNameMaxRunes {
		return "", apperrors.Newf(apperrors.CodeInvalidInput, "display name must be %d characters or fewer", libraryDisplayNameMaxRunes)
	}
	if strings.Trim(name, ".") == "" {
		return "", apperrors.New(apperrors.CodeInvalidInput, "display name cannot be . or ..")
	}
	if strings.HasSuffix(name, ".") {
		return "", apperrors.New(apperrors.CodeInvalidInput, "display name cannot end with a dot")
	}
	for _, char := range name {
		if unicode.IsControl(char) || strings.ContainsRune(`/\<>:"|?*`, char) {
			return "", apperrors.New(apperrors.CodeInvalidInput, `display name cannot contain / \ < > : " | ? * or control characters`)
		}
	}

	reservedCandidate := strings.ToUpper(name)
	if dotIndex := strings.IndexRune(reservedCandidate, '.'); dotIndex >= 0 {
		reservedCandidate = reservedCandidate[:dotIndex]
	}
	if _, reserved := libraryReservedDisplayNames[reservedCandidate]; reserved {
		return "", apperrors.New(apperrors.CodeInvalidInput, "display name is reserved by the operating system")
	}
	return name, nil
}
