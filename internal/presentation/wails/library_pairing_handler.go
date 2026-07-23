package wails

import (
	"context"
	"net"
	"net/url"
	"strconv"
	"strings"

	"xiadown/internal/application/library/access"
)

const libraryPairingProtocolVersion = 1

type LibraryPairingHandler struct {
	service      *access.Service
	fingerprint  func() string
	lanAddress   func() string
	lanEndpoints func() []string
	tailscaleURL func(context.Context) string
}

type LibraryPairingResult struct {
	PairingVersion int      `json:"pairingVersion"`
	PairingLink    string   `json:"pairingLink"`
	Nonce          string   `json:"nonce"`
	Code           string   `json:"code"`
	ExpiresAt      string   `json:"expiresAt"`
	TLSFingerprint string   `json:"tlsFingerprint"`
	LANAddress     string   `json:"lanAddress,omitempty"`
	LANEndpoints   []string `json:"lanEndpoints,omitempty"`
	TailscaleURL   string   `json:"tailscaleURL,omitempty"`
}

type UpdateLibraryDeviceScopesRequest struct {
	GrantID          string   `json:"grantId"`
	ExpectedRevision int64    `json:"expectedRevision"`
	Scopes           []string `json:"scopes"`
}

type RevokeLibraryDeviceRequest struct {
	GrantID          string `json:"grantId"`
	ExpectedRevision int64  `json:"expectedRevision"`
}

func NewLibraryPairingHandler(
	service *access.Service,
	fingerprint func() string,
	lanAddress func() string,
	tailscaleURL func(context.Context) string,
	lanEndpointProviders ...func() []string,
) *LibraryPairingHandler {
	handler := &LibraryPairingHandler{
		service: service, fingerprint: fingerprint, lanAddress: lanAddress, tailscaleURL: tailscaleURL,
	}
	if len(lanEndpointProviders) > 0 {
		handler.lanEndpoints = lanEndpointProviders[0]
	}
	return handler
}

func (*LibraryPairingHandler) ServiceName() string { return "LibraryPairingHandler" }

func (handler *LibraryPairingHandler) StartLibraryPairing(ctx context.Context) (LibraryPairingResult, error) {
	session, err := handler.service.StartPairing()
	if err != nil {
		return LibraryPairingResult{}, err
	}
	result := LibraryPairingResult{
		PairingVersion: libraryPairingProtocolVersion,
		Nonce:          session.Nonce, Code: session.Code, ExpiresAt: session.ExpiresAt.UTC().Format("2006-01-02T15:04:05.000Z07:00"),
	}
	if handler.fingerprint != nil {
		result.TLSFingerprint = strings.TrimSpace(handler.fingerprint())
	}
	rawLANAddresses := make([]string, 0, 2)
	if handler.lanEndpoints != nil {
		rawLANAddresses = append(rawLANAddresses, handler.lanEndpoints()...)
	}
	if handler.lanAddress != nil {
		rawLANAddresses = append(rawLANAddresses, handler.lanAddress())
	}
	result.LANEndpoints, result.LANAddress = crossDeviceLANEndpoints(rawLANAddresses)
	for index, endpoint := range result.LANEndpoints {
		result.LANEndpoints[index] = libraryPairingTransportBase(endpoint)
	}
	if len(result.LANEndpoints) == 0 {
		result.LANEndpoints = nil
	}
	if handler.tailscaleURL != nil {
		result.TailscaleURL = libraryPairingTransportBase(handler.tailscaleURL(ctx))
	}
	// PairingLink is an ephemeral credential. Keep its construction local to the
	// response boundary and never log it. Repeated endpoint keys let a client try
	// every advertised transport without inventing a delimiter or losing IPv6.
	result.PairingLink = libraryPairingDeepLink(result)
	return result, nil
}

func libraryPairingDeepLink(result LibraryPairingResult) string {
	values := url.Values{}
	values.Set("v", strconv.Itoa(result.PairingVersion))
	values.Set("nonce", result.Nonce)
	values.Set("code", result.Code)
	values.Set("expires", result.ExpiresAt)
	values.Set("fingerprint", result.TLSFingerprint)
	for _, endpoint := range result.LANEndpoints {
		if endpoint = libraryPairingTransportBase(endpoint); endpoint != "" {
			values.Add("lan", endpoint)
		}
	}
	if endpoint := libraryPairingTransportBase(result.TailscaleURL); endpoint != "" {
		values.Add("remote", endpoint)
	}
	return (&url.URL{Scheme: "xiadown", Host: "pair", RawQuery: values.Encode()}).String()
}

// A pairing endpoint is a directory base: relative API hrefs must append below
// it rather than replace a Tailscale path such as /xiadown.
func libraryPairingTransportBase(value string) string {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil ||
		parsed.RawQuery != "" || parsed.Fragment != "" {
		return ""
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/"
	parsed.RawPath = ""
	return parsed.String()
}

// crossDeviceLANEndpoints deliberately omits IPv6 link-local socket zones.
// A zone name is local to the server (for example en0 or Ethernet) and is not
// a valid scope identifier on an iOS or Windows client. Such listeners remain
// discoverable through DNS-SD, whose receiving interface supplies the scope.
func crossDeviceLANEndpoints(addresses []string) ([]string, string) {
	result := make([]string, 0, len(addresses))
	seen := make(map[string]struct{}, len(addresses))
	legacy := ""
	for _, address := range addresses {
		host, port, err := net.SplitHostPort(strings.TrimSpace(address))
		if err != nil || strings.TrimSpace(host) == "" || strings.TrimSpace(port) == "" {
			continue
		}
		if zoneIndex := strings.LastIndex(host, "%"); zoneIndex >= 0 {
			ip := net.ParseIP(host[:zoneIndex])
			if ip == nil || ip.IsLinkLocalUnicast() {
				continue
			}
			host = host[:zoneIndex]
		}
		if net.ParseIP(host) == nil {
			continue
		}
		normalized := net.JoinHostPort(host, port)
		endpoint := "https://" + normalized
		if _, exists := seen[endpoint]; exists {
			continue
		}
		seen[endpoint] = struct{}{}
		result = append(result, endpoint)
		if legacy == "" {
			legacy = normalized
		}
	}
	return result, legacy
}

func (handler *LibraryPairingHandler) ListPairedLibraryDevices(ctx context.Context) ([]access.DeviceGrantMetadata, error) {
	return handler.service.ListDeviceGrants(ctx)
}

func (handler *LibraryPairingHandler) UpdatePairedLibraryDeviceScopes(
	ctx context.Context,
	request UpdateLibraryDeviceScopesRequest,
) (access.DeviceGrantMetadata, error) {
	return handler.service.UpdateDeviceGrantScopes(ctx, access.UpdateScopesRequest{
		GrantID: request.GrantID, ExpectedRevision: request.ExpectedRevision,
		Scopes: request.Scopes, ActorID: "local:desktop",
	})
}

func (handler *LibraryPairingHandler) RevokePairedLibraryDevice(
	ctx context.Context,
	request RevokeLibraryDeviceRequest,
) (access.DeviceGrantMetadata, error) {
	return handler.service.RevokeDeviceGrant(ctx, access.RevokeGrantRequest{
		GrantID: request.GrantID, ExpectedRevision: request.ExpectedRevision,
		ActorID: "local:desktop",
	})
}
