package browserprofile

import (
	"errors"
	"strings"
)

const (
	CookieAccessStateAccessRequired       = "access_required"
	CookieAccessStateProtectedUnsupported = "protected_unsupported"
)

// CookieAccessError reports that a browser cookie store exists but its
// protected values cannot be read through the selected access path. It stays
// distinct from an empty profile: callers must not turn this error into a
// misleading "no authentication cookies" result.
type CookieAccessError struct {
	State string
	Err   error
}

func (err *CookieAccessError) Error() string {
	if err == nil {
		return ""
	}
	if err.Err != nil {
		return err.Err.Error()
	}
	return strings.TrimSpace(err.State)
}

func (err *CookieAccessError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Err
}

func CookieAccessErrorState(err error) string {
	var accessErr *CookieAccessError
	if errors.As(err, &accessErr) && accessErr != nil {
		return strings.TrimSpace(accessErr.State)
	}
	return ""
}

func protectedCookieAccessError(browserID string) error {
	if strings.EqualFold(strings.TrimSpace(browserID), "chrome") {
		return &CookieAccessError{
			State: CookieAccessStateAccessRequired,
			Err:   errors.New("Chrome requires approved current-browser access to read protected cookies"),
		}
	}
	return &CookieAccessError{
		State: CookieAccessStateProtectedUnsupported,
		Err:   errors.New("this browser's protected cookies are unsupported"),
	}
}
