package appsessions

import "errors"

var (
	ErrSessionNotFound             = errors.New("app session not found")
	ErrInvalidSession              = errors.New("invalid app session")
	ErrNoCookies                   = errors.New("no app session cookies stored")
	ErrBrowserCookieAccessRequired = errors.New("browser cookie access required")
	ErrBrowserCookieProtected      = errors.New("protected browser cookies are unsupported")
	ErrUnsupported                 = errors.New("app session unsupported")
	ErrSessionDead                 = errors.New("app session browser ended")
	ErrSessionGone                 = errors.New("app session not found")
)
