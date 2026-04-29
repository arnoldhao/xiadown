package connectors

import "errors"

var (
	ErrConnectorNotFound    = errors.New("connector not found")
	ErrInvalidConnector     = errors.New("invalid connector")
	ErrNoCookies            = errors.New("no cookies stored")
	ErrConnectorSessionDead = errors.New("connector browser session ended")
	ErrConnectorSessionGone = errors.New("connector session not found")
)
