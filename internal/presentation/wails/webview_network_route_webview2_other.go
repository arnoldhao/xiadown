//go:build !windows || server

package wails

func prepareWebView2NetworkRouteEnvironment(_ []string) error {
	return nil
}
