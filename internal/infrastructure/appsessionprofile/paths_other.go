//go:build (!darwin && !windows) || ios

package appsessionprofile

import "xiadown/internal/domain/appsessions"

func platformBrowserDefinition(string) (browserDefinition, error) {
	return browserDefinition{}, appsessions.ErrUnsupported
}
