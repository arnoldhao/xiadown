package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"xiadown/internal/domain/settings"
)

const DefaultTestURL = "https://www.gstatic.com/generate_204"

var defaultFeatureTestURLs = []string{
	DefaultTestURL,
	"https://music.youtube.com/",
}

// Config mirrors proxy-related settings to avoid coupling infra to DTO.
type Config struct {
	Mode     settings.ProxyMode
	Scheme   settings.ProxyScheme
	Host     string
	Port     int
	Username string
	Password string
	NoProxy  []string
	Timeout  time.Duration
}

type TestResult struct {
	Success  bool
	Message  string
	TestedAt time.Time
}

type SystemProxySource string

const (
	SystemProxySourceSystem SystemProxySource = "system"
	SystemProxySourceVPN    SystemProxySource = "vpn"
)

type SystemProxyInfo struct {
	Address string
	Source  SystemProxySource
	Name    string
}

type Manager struct {
	mu         sync.RWMutex
	config     Config
	client     *http.Client
	public     *http.Client
	testURL    string
	gateway    *loopbackGateway
	generation uint64
	closed     bool
}

func NewManager(config Config) (*Manager, error) {
	config = cloneConfig(config)
	if err := validateConfig(config); err != nil {
		return nil, err
	}

	state, err := newRouteState(config, 1)
	if err != nil {
		return nil, err
	}
	gateway, err := newLoopbackGateway(state)
	if err != nil {
		state.close()
		return nil, err
	}

	client, err := newGatewayHTTPClient(gateway.URL(), config.Timeout)
	if err != nil {
		_ = gateway.Close()
		return nil, err
	}
	mgr := &Manager{
		config:     config,
		client:     client,
		testURL:    DefaultTestURL,
		gateway:    gateway,
		generation: 1,
	}
	mgr.public = newPublicHTTPClient(mgr, config.Timeout)
	return mgr, nil
}

func (m *Manager) Apply(config Config) error {
	return m.apply(config)
}

func (m *Manager) apply(config Config) error {
	config = cloneConfig(config)
	if err := validateConfig(config); err != nil {
		return err
	}
	state, err := newRouteState(config, 0)
	if err != nil {
		return err
	}
	client, err := newGatewayHTTPClient(m.GatewayURL(), config.Timeout)
	if err != nil {
		state.close()
		return err
	}
	publicClient := newPublicHTTPClient(m, config.Timeout)

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		client.CloseIdleConnections()
		state.close()
		return errors.New("proxy manager is closed")
	}
	m.generation++
	state.generation = m.generation
	oldState := m.gateway.swap(state)
	m.config = config
	oldClient := m.client
	m.client = client
	oldPublicClient := m.public
	m.public = publicClient
	m.mu.Unlock()

	// A policy generation is a hard boundary. Reusing either an idle local
	// proxy connection or an established CONNECT tunnel could otherwise keep
	// traffic on the previous route after Apply returns.
	if oldClient != nil {
		oldClient.CloseIdleConnections()
	}
	if oldPublicClient != nil {
		oldPublicClient.CloseIdleConnections()
	}
	if oldState != nil {
		oldState.close()
	}
	return nil
}

func (m *Manager) Test(ctx context.Context, config Config) (TestResult, error) {
	config = cloneConfig(config)
	client, closeClient, err := buildHTTPClient(config)
	if err != nil {
		return TestResult{}, err
	}
	defer closeClient()

	start := time.Now()
	testURLs := []string{m.testURL}
	if m.testURL == DefaultTestURL {
		testURLs = append([]string(nil), defaultFeatureTestURLs...)
	}
	lastStatus := 0
	for _, testURL := range testURLs {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, testURL, nil)
		if err != nil {
			return TestResult{}, err
		}
		resp, err := client.Do(req)
		if err != nil {
			return TestResult{
				Success:  false,
				Message:  fmt.Sprintf("%s: %s", req.URL.Hostname(), redactProxyError(err, config)),
				TestedAt: start,
			}, nil
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		lastStatus = resp.StatusCode
		if resp.StatusCode < 200 || resp.StatusCode >= 400 {
			return TestResult{
				Success:  false,
				Message:  fmt.Sprintf("%s: status %d", req.URL.Hostname(), resp.StatusCode),
				TestedAt: start,
			}, nil
		}
	}
	return TestResult{
		Success:  true,
		Message:  fmt.Sprintf("backend and music probes passed (last status %d)", lastStatus),
		TestedAt: start,
	}, nil
}

func redactProxyError(err error, config Config) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	for _, secret := range []string{config.Password, config.Username} {
		if secret != "" {
			message = strings.ReplaceAll(message, secret, "[redacted]")
			message = strings.ReplaceAll(message, url.QueryEscape(secret), "[redacted]")
		}
	}
	return message
}

func (m *Manager) HTTPClient() *http.Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.client
}

// PublicHTTPClient shares the active proxy generation but rejects local,
// private, link-local, metadata, and other special destinations before any
// direct or proxied connection is established. Use it for untrusted public
// URL fetches; HTTPClient intentionally retains LAN/local product behavior.
func (m *Manager) PublicHTTPClient() *http.Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.public
}

// PublicDialContext is the route-class boundary used by PublicHTTPClient. DNS
// is resolved and pinned locally, and every result is checked to prevent DNS
// rebinding from reaching a non-public address.
func (m *Manager) PublicDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return m.PublicDialURLContext(ctx, network, address, &url.URL{Scheme: "https", Host: address})
}

// PublicDialURLContext preserves the complete logical URL for validation,
// redirects, Host/SNI, and App NoProxy matching while address identifies the
// hostname whose validated public IPs are pinned for the socket. System PAC is
// deliberately normalized to this URL's origin so opaque HTTPS CONNECT users
// make the same decision. Callers which know the URL must use this method.
func (m *Manager) PublicDialURLContext(ctx context.Context, network, address string, targetURL *url.URL) (net.Conn, error) {
	if network != "tcp" && network != "tcp4" && network != "tcp6" {
		return nil, fmt.Errorf("public route does not support network %q", network)
	}
	if targetURL == nil {
		return nil, errors.New("public route requires a logical URL")
	}
	m.mu.RLock()
	if m.closed || m.gateway == nil {
		m.mu.RUnlock()
		return nil, errors.New("proxy manager is closed")
	}
	state := m.gateway.active.Load()
	m.mu.RUnlock()
	if state == nil {
		return nil, errors.New("proxy manager is closed")
	}
	return state.dialPublicURL(ctx, network, address, targetURL)
}

// GatewayURL is the stable, loopback-only HTTP forward/CONNECT proxy owned by
// this manager. It is available as soon as NewManager returns and does not
// change across Apply calls.
func (m *Manager) GatewayURL() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.gateway == nil {
		return ""
	}
	return m.gateway.URL()
}

// ConsumerProxyURL is the proxy URL that app-managed non-Go consumers such as
// yt-dlp, managed browsers, media engines, and child processes use to share the
// active policy. Native WebViews intentionally use their platform network.
func (m *Manager) ConsumerProxyURL() string {
	return m.GatewayURL()
}

// ConsumerProxyAttestation returns a request/response challenge that managed
// browsers use before loading public content. It detects command-line proxy
// flags being ignored or overridden by browser policy and fails the launch
// closed instead of silently using a different network route.
func (m *Manager) ConsumerProxyAttestation() (string, string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed || m.gateway == nil {
		return "", ""
	}
	return m.gateway.attestation()
}

// RegisterInternalLoopbackURL permits one exact, app-owned loopback listener
// through the shared gateway. It is intentionally authority-scoped: user
// NoProxy rules must not turn a managed consumer into a general localhost
// proxy.
func (m *Manager) RegisterInternalLoopbackURL(rawURL string) error {
	m.mu.RLock()
	if m.closed || m.gateway == nil {
		m.mu.RUnlock()
		return errors.New("proxy manager is closed")
	}
	gateway := m.gateway
	m.mu.RUnlock()
	return gateway.internal.register(rawURL)
}

// ChildProxyEnvironment returns an environment for one child process without
// mutating process-global proxy variables. Inherited proxy variables are
// removed case-insensitively and replaced with the stable loopback gateway.
func (m *Manager) ChildProxyEnvironment(base []string) []string {
	gatewayURL := m.ConsumerProxyURL()
	result := make([]string, 0, len(base)+8)
	for _, item := range base {
		name, _, ok := strings.Cut(item, "=")
		if ok && isProxyEnvironmentVariable(name) {
			continue
		}
		result = append(result, item)
	}
	if gatewayURL == "" {
		// A closed/unavailable manager must not silently turn a child into a
		// direct-network process. Bind inherited consumers to a guaranteed-dead
		// loopback endpoint so the failure is explicit and local.
		gatewayURL = "http://127.0.0.1:1"
	}
	for _, name := range []string{"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy", "ALL_PROXY", "all_proxy"} {
		result = append(result, name+"="+gatewayURL)
	}
	// Never inherit or manufacture a destination bypass. Proxy clients connect
	// to the loopback gateway directly; NO_PROXY applies to the destination,
	// so listing loopback here would let a child skip the gateway's exact
	// app-owned endpoint registry.
	result = append(result, "NO_PROXY=", "no_proxy=")
	return result
}

func isProxyEnvironmentVariable(name string) bool {
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY":
		return true
	default:
		return false
	}
}

// Generation identifies the active network-policy generation.
func (m *Manager) Generation() uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.generation
}

// Close stops the loopback gateway and closes all idle and tunneled
// connections. It is safe to call more than once.
func (m *Manager) Close() error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true
	client := m.client
	publicClient := m.public
	gateway := m.gateway
	m.mu.Unlock()

	if client != nil {
		client.CloseIdleConnections()
	}
	if publicClient != nil {
		publicClient.CloseIdleConnections()
	}
	if gateway != nil {
		return gateway.Close()
	}
	return nil
}

func (m *Manager) ResolveProxy(rawURL string) (string, error) {
	if strings.TrimSpace(rawURL) != "" {
		if _, err := url.ParseRequestURI(rawURL); err != nil {
			return "", err
		}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed || m.gateway == nil {
		return "", errors.New("proxy manager is closed")
	}
	// Even "none" mode enters through the gateway: explicit direct routing,
	// NoProxy matching, generation changes, and tunnel revocation all live at
	// this single policy boundary.
	return m.gateway.URL(), nil
}

func (m *Manager) ResolveSystemProxy(rawURL string) (string, error) {
	info, err := m.ResolveSystemProxyInfo(rawURL)
	if err != nil {
		return "", err
	}
	return info.Address, nil
}

func (m *Manager) ResolveSystemProxyInfo(rawURL string) (SystemProxyInfo, error) {
	if rawURL == "" {
		rawURL = DefaultTestURL
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return SystemProxyInfo{}, err
	}

	info := SystemProxyInfo{Source: SystemProxySourceSystem}
	resolveContext, cancel := systemProxyDiagnosticContext(context.Background())
	defer cancel()
	proxyURL, err := platformSystemProxyURLContext(resolveContext, u)
	if err != nil || proxyURL == nil {
		return info, err
	}

	// This value is exposed to the settings UI. Keep native credentials on
	// the internal route only and never serialize them to the frontend.
	displayURL := *proxyURL
	displayURL.User = nil
	info.Address = displayURL.String()
	return info, nil
}

func (m *Manager) CurrentConfig() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneConfig(m.config)
}

func ConfigFromSettings(proxy settings.ProxySettings) Config {
	return Config{
		Mode:     proxy.Mode(),
		Scheme:   proxy.Scheme(),
		Host:     proxy.Host(),
		Port:     proxy.Port(),
		Username: proxy.Username(),
		Password: proxy.Password(),
		NoProxy:  proxy.NoProxy(),
		Timeout:  proxy.Timeout(),
	}
}

func buildHTTPClient(config Config) (*http.Client, func(), error) {
	config = cloneConfig(config)
	if err := validateConfig(config); err != nil {
		return nil, nil, err
	}
	state, err := newRouteState(config, 0)
	if err != nil {
		return nil, nil, err
	}
	client := &http.Client{
		Transport: &routeStateRoundTripper{state: state},
		Timeout:   config.Timeout,
	}
	return client, state.close, nil
}

func buildProxyURL(config Config) *url.URL {
	if config.Host == "" || config.Port == 0 {
		return nil
	}
	scheme := config.Scheme.String()
	if scheme == "" {
		scheme = settings.ProxySchemeHTTP.String()
	}
	hostPort := net.JoinHostPort(config.Host, strconv.Itoa(config.Port))
	u := &url.URL{
		Scheme: scheme,
		Host:   hostPort,
	}
	if config.Username != "" || config.Password != "" {
		u.User = url.UserPassword(config.Username, config.Password)
	}
	return u
}

func parsePort(s string) (uint16, error) {
	if s == "" {
		return 0, fmt.Errorf("invalid port")
	}
	p, err := strconv.Atoi(s)
	if err != nil || p <= 0 || p > 65535 {
		return 0, fmt.Errorf("invalid port")
	}
	return uint16(p), nil
}
