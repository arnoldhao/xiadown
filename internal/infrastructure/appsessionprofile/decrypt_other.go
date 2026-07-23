//go:build (!darwin && !windows) || ios

package appsessionprofile

import (
	"context"

	"xiadown/internal/domain/appsessions"
)

func newPlatformCookieDecryptor(context.Context, browserDefinition, string) (cookieDecryptor, error) {
	return nil, appsessions.ErrUnsupported
}
