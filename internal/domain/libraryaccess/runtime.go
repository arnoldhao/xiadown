package libraryaccess

import (
	"context"
	"errors"
)

var ErrTailscaleRouteOwnershipConflict = errors.New("tailscale route ownership conflict")

type TailscaleInfo struct {
	Installed bool
	Connected bool
	Version   string
	Tailnet   string
	Device    string
	DNSName   string
	ServeURL  string
	LastError string

	// RouteChecked distinguishes a successfully observed missing handler from
	// an unavailable daemon/status command. RouteTarget is the exact Proxy
	// value for the requested HTTPS port/path; RouteBackendPort is non-zero
	// only for XiaDown's canonical http://127.0.0.1:<port> target form.
	RouteChecked     bool
	RouteExists      bool
	RouteTarget      string
	RouteBackendPort int
}

type TailscaleRouteOwnership struct {
	BackendPort        int
	PendingBackendPort int
}

func (ownership TailscaleRouteOwnership) AllowsBackendPort(port int) bool {
	return port > 0 && (port == ownership.BackendPort || port == ownership.PendingBackendPort)
}

type TailscaleManager interface {
	Inspect(ctx context.Context, httpsPort int, routePath string) TailscaleInfo
	Enable(ctx context.Context, localPort, httpsPort int, routePath string, ownership TailscaleRouteOwnership) error
	Disable(ctx context.Context, httpsPort int, routePath string, ownership TailscaleRouteOwnership) error
}
