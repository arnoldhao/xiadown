//go:build ios || android || server || (!darwin && !windows && !linux) || (linux && !cgo)

package wails

import "fmt"

func applyWebViewNetworkRoutePlatform(_ webViewNetworkGateway, _ bool) error {
	return fmt.Errorf("%w: no public native adapter is available for this build", ErrWebViewNetworkRouteUnsupported)
}
