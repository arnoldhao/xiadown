package browsercdp

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	targetpkg "github.com/chromedp/cdproto/target"
)

const managedNetworkProbePageURLPrefix = "data:text/html,xiadown-managed-network-route-probe-"

var managedNetworkProbeFallbackID atomic.Uint64

type managedNetworkProbeTargetRequest struct {
	URL        string
	Background bool
	Hidden     bool
}

func newManagedNetworkProbePageURL() string {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err == nil {
		return managedNetworkProbePageURLPrefix + hex.EncodeToString(nonce[:])
	}
	return fmt.Sprintf(
		"%s%x-%x",
		managedNetworkProbePageURLPrefix,
		time.Now().UnixNano(),
		managedNetworkProbeFallbackID.Add(1),
	)
}

// newManagedNetworkProbeTargetRequest deliberately owns the target URL. The
// caller cannot substitute a user or public URL before the managed network
// route has been attested.
func newManagedNetworkProbeTargetRequest(existingTargets []*targetpkg.Info) managedNetworkProbeTargetRequest {
	return managedNetworkProbeTargetRequest{
		URL:        newManagedNetworkProbePageURL(),
		Background: true,
		Hidden:     hasManagedNetworkProbePageTarget(existingTargets),
	}
}

func (request managedNetworkProbeTargetRequest) protocolParams() map[string]any {
	params := map[string]any{
		"url":        request.URL,
		"background": request.Background,
	}
	if request.Hidden {
		params["hidden"] = true
	}
	return params
}

func hasManagedNetworkProbePageTarget(targets []*targetpkg.Info) bool {
	for _, info := range targets {
		if info != nil && strings.EqualFold(strings.TrimSpace(info.Type), "page") {
			return true
		}
	}
	return false
}

func isManagedNetworkProbeURL(rawURL string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(rawURL)), managedNetworkProbePageURLPrefix)
}

func isManagedNetworkProbeTargetInfo(info *targetpkg.Info) bool {
	return info != nil && isManagedNetworkProbeURL(info.URL)
}
