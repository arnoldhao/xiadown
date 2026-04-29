package proxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"xiadown/internal/domain/settings"
)

const DefaultTestURL = "https://www.gstatic.com/generate_204"

var darwinServiceIDRegexp = regexp.MustCompile(`[A-Fa-f0-9-]{36}`)

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
	mu      sync.RWMutex
	config  Config
	client  *http.Client
	testURL string
}

func NewManager(config Config) (*Manager, error) {
	mgr := &Manager{
		testURL: DefaultTestURL,
	}
	if err := mgr.apply(config); err != nil {
		return nil, err
	}
	return mgr, nil
}

func (m *Manager) Apply(config Config) error {
	return m.apply(config)
}

func (m *Manager) apply(config Config) error {
	client, err := buildHTTPClient(config)
	if err != nil {
		return err
	}

	setupEnv(config)

	m.mu.Lock()
	oldClient := m.client
	m.config = config
	m.client = client
	m.mu.Unlock()
	if oldClient != nil {
		oldClient.CloseIdleConnections()
	}
	return nil
}

func (m *Manager) Test(ctx context.Context, config Config) (TestResult, error) {
	client, err := buildHTTPClient(config)
	if err != nil {
		return TestResult{}, err
	}
	defer client.CloseIdleConnections()

	testURL := m.testURL
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, testURL, nil)
	if err != nil {
		return TestResult{}, err
	}

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return TestResult{
			Success:  false,
			Message:  err.Error(),
			TestedAt: start,
		}, nil
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return TestResult{
			Success:  true,
			Message:  fmt.Sprintf("status %d", resp.StatusCode),
			TestedAt: start,
		}, nil
	}

	return TestResult{
		Success:  false,
		Message:  fmt.Sprintf("status %d", resp.StatusCode),
		TestedAt: start,
	}, nil
}

func (m *Manager) HTTPClient() *http.Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.client
}

func (m *Manager) ResolveProxy(rawURL string) (string, error) {
	m.mu.RLock()
	cfg := m.config
	m.mu.RUnlock()

	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	proxyFunc, dialer := proxyResolution(cfg)
	req := &http.Request{URL: u}
	if dialer != nil && cfg.Mode == settings.ProxyModeSystem && proxyFunc == nil {
		if addr := systemSocksAddress(); addr != "" {
			return "socks5://" + addr, nil
		}
		return "", nil
	}
	if proxyFunc == nil {
		return "", nil
	}
	p, err := proxyFunc(req)
	if err != nil || p == nil {
		return "", err
	}
	return p.String(), nil
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

	if runtime.GOOS == "darwin" {
		selection, err := selectDarwinProxyInfo()
		if err == nil && selection != nil {
			if selection.info != nil {
				if proxyURL, err := selection.info.proxyURLForScheme(u.Scheme); err != nil || proxyURL != nil {
					if proxyURL == nil {
						return SystemProxyInfo{Source: selection.source, Name: selection.name}, err
					}
					return SystemProxyInfo{Address: proxyURL.String(), Source: selection.source, Name: selection.name}, err
				}
				if !selection.info.hasAnyHTTPProxy() {
					if addr := selection.info.socksAddress(); addr != "" {
						return SystemProxyInfo{Address: "socks5://" + addr, Source: selection.source, Name: selection.name}, nil
					}
				}
			}
			return SystemProxyInfo{Source: selection.source, Name: selection.name}, nil
		}

		return SystemProxyInfo{Source: SystemProxySourceSystem}, nil
	}

	m.mu.RLock()
	cfg := m.config
	m.mu.RUnlock()
	cfg.Mode = settings.ProxyModeSystem

	proxyFunc, dialer := proxyResolution(cfg)
	req := &http.Request{URL: u}
	if dialer != nil && proxyFunc == nil {
		if addr := systemSocksAddress(); addr != "" {
			return SystemProxyInfo{Address: "socks5://" + addr, Source: SystemProxySourceSystem}, nil
		}
		return SystemProxyInfo{Source: SystemProxySourceSystem}, nil
	}
	if proxyFunc == nil {
		return SystemProxyInfo{Source: SystemProxySourceSystem}, nil
	}
	p, err := proxyFunc(req)
	if err != nil || p == nil {
		return SystemProxyInfo{Source: SystemProxySourceSystem}, err
	}
	return SystemProxyInfo{Address: p.String(), Source: SystemProxySourceSystem}, nil
}

func (m *Manager) CurrentConfig() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
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

func buildHTTPClient(config Config) (*http.Client, error) {
	proxyFunc, dialer := proxyResolution(config)

	transport := &http.Transport{
		Proxy: proxyFunc,
		DialContext: (&net.Dialer{
			Timeout:   config.Timeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	if dialer != nil {
		transport.Proxy = nil
		transport.DialContext = dialer
	}

	return &http.Client{
		Transport: transport,
		Timeout:   config.Timeout,
	}, nil
}

func proxyResolution(config Config) (func(*http.Request) (*url.URL, error), func(ctx context.Context, network, address string) (net.Conn, error)) {
	switch config.Mode {
	case settings.ProxyModeNone:
		return nil, nil
	case settings.ProxyModeSystem:
		return systemProxyFunc(config), systemSocksDialer(config)
	case settings.ProxyModeManual:
		return manualProxyFunc(config), manualSocksDialer(config)
	default:
		return nil, nil
	}
}

func manualProxyFunc(config Config) func(*http.Request) (*url.URL, error) {
	if strings.EqualFold(config.Scheme.String(), settings.ProxySchemeSocks5.String()) {
		return nil
	}

	manualURL := buildProxyURL(config)
	if manualURL == nil {
		return nil
	}

	return func(req *http.Request) (*url.URL, error) {
		if shouldBypass(req.URL.Hostname(), config.NoProxy) {
			return nil, nil
		}
		return manualURL, nil
	}
}

func manualSocksDialer(config Config) func(ctx context.Context, network, address string) (net.Conn, error) {
	if !strings.EqualFold(config.Scheme.String(), settings.ProxySchemeSocks5.String()) {
		return nil
	}
	if config.Host == "" || config.Port == 0 {
		return nil
	}
	return socks5DialContext(net.JoinHostPort(config.Host, strconv.Itoa(config.Port)), config.Timeout)
}

func systemProxyFunc(config Config) func(*http.Request) (*url.URL, error) {
	return func(req *http.Request) (*url.URL, error) {
		switch runtime.GOOS {
		case "windows":
			if proxy, err := getSystemProxyFromRegistry(); err == nil && proxy != "" {
				return parseWindowsProxy(proxy)
			}
			return http.ProxyFromEnvironment(req)
		case "darwin":
			return getDarwinProxy(req)
		default:
			return http.ProxyFromEnvironment(req)
		}
	}
}

func systemSocksDialer(config Config) func(ctx context.Context, network, address string) (net.Conn, error) {
	if runtime.GOOS != "darwin" {
		return nil
	}

	selection, err := selectDarwinProxyInfo()
	if err != nil || selection == nil || selection.info == nil {
		return nil
	}
	if selection.info.hasAnyHTTPProxy() {
		return nil
	}
	if addr := selection.info.socksAddress(); addr != "" {
		return socks5DialContext(addr, config.Timeout)
	}
	return nil
}

func systemSocksAddress() string {
	if runtime.GOOS != "darwin" {
		return ""
	}
	selection, err := selectDarwinProxyInfo()
	if err != nil || selection == nil || selection.info == nil {
		return ""
	}
	if addr := selection.info.socksAddress(); addr != "" {
		return addr
	}
	return ""
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

func shouldBypass(host string, noProxy []string) bool {
	if host == "" {
		return false
	}
	if host == "localhost" || strings.HasPrefix(host, "127.") {
		return true
	}
	for _, entry := range noProxy {
		if entry == "" {
			continue
		}
		if strings.Contains(host, entry) {
			return true
		}
	}
	return false
}

func setupEnv(config Config) {
	envVars := []string{"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy", "ALL_PROXY", "all_proxy", "NO_PROXY", "no_proxy"}
	for _, env := range envVars {
		os.Unsetenv(env)
	}

	if config.Mode == settings.ProxyModeManual {
		proxyURL := buildProxyURL(config)
		if proxyURL == nil {
			return
		}
		proxyStr := proxyURL.String()

		if strings.EqualFold(config.Scheme.String(), settings.ProxySchemeSocks5.String()) {
			os.Setenv("ALL_PROXY", proxyStr)
			os.Setenv("all_proxy", proxyStr)
		} else {
			os.Setenv("HTTP_PROXY", proxyStr)
			os.Setenv("http_proxy", proxyStr)
			os.Setenv("HTTPS_PROXY", proxyStr)
			os.Setenv("https_proxy", proxyStr)
		}

		if len(config.NoProxy) > 0 {
			noProxy := strings.Join(config.NoProxy, ",")
			os.Setenv("NO_PROXY", noProxy)
			os.Setenv("no_proxy", noProxy)
		}
		return
	}

	if config.Mode == settings.ProxyModeSystem {
		if runtime.GOOS == "darwin" {
			if selection, err := selectDarwinProxyInfo(); err == nil && selection != nil && selection.info != nil {
				dpi := selection.info
				if dpi.hasHTTPProxy() {
					proxyStr := fmt.Sprintf("http://%s:%s", dpi.HTTPProxy, dpi.HTTPPort)
					os.Setenv("HTTP_PROXY", proxyStr)
					os.Setenv("http_proxy", proxyStr)
				}
				if dpi.hasHTTPSProxy() {
					proxyStr := fmt.Sprintf("http://%s:%s", dpi.HTTPSProxy, dpi.HTTPSPort)
					os.Setenv("HTTPS_PROXY", proxyStr)
					os.Setenv("https_proxy", proxyStr)
				}
				if addr := dpi.socksAddress(); addr != "" {
					socksStr := fmt.Sprintf("socks5://%s", addr)
					os.Setenv("ALL_PROXY", socksStr)
					os.Setenv("all_proxy", socksStr)
				}
			}
		} else if runtime.GOOS == "windows" {
			if proxyStr, err := getSystemProxyFromRegistry(); err == nil && proxyStr != "" {
				if parsed, err := parseWindowsProxy(proxyStr); err == nil && parsed != nil {
					populateHTTPEnv(parsed)
				}
			}
		}
	}
}

func parseWindowsProxy(proxyStr string) (*url.URL, error) {
	if strings.Contains(proxyStr, "=") {
		parts := strings.Split(proxyStr, ";")
		for _, part := range parts {
			if strings.HasPrefix(part, "http=") {
				proxyStr = strings.TrimPrefix(part, "http=")
				break
			}
		}
	}
	if !strings.HasPrefix(proxyStr, "http://") && !strings.HasPrefix(proxyStr, "https://") {
		proxyStr = "http://" + proxyStr
	}
	return url.Parse(proxyStr)
}

func getDarwinProxy(req *http.Request) (*url.URL, error) {
	selection, err := selectDarwinProxyInfo()
	if err == nil && selection != nil {
		if selection.info != nil {
			if proxyURL, err := selection.info.proxyURLForScheme(req.URL.Scheme); err != nil || proxyURL != nil {
				return proxyURL, err
			}
		}
		if selection.source == SystemProxySourceVPN {
			return nil, nil
		}
	}
	if proxy, err := http.ProxyFromEnvironment(req); err == nil && proxy != nil {
		return proxy, nil
	}
	return nil, nil
}

type darwinProxyInfo struct {
	HTTPEnabled  bool
	HTTPProxy    string
	HTTPPort     string
	HTTPSEnabled bool
	HTTPSProxy   string
	HTTPSPort    string
	SOCKSEnabled bool
	SOCKSProxy   string
	SOCKSPort    string
	PACEnabled   bool
	PACURL       string
}

type darwinProxySnapshot struct {
	Global darwinProxyInfo
	Scoped map[string]darwinProxyInfo
}

type darwinProxySelection struct {
	info   *darwinProxyInfo
	source SystemProxySource
	name   string
}

func (dpi *darwinProxyInfo) hasHTTPProxy() bool {
	return dpi != nil && dpi.HTTPEnabled && dpi.HTTPProxy != "" && dpi.HTTPPort != ""
}

func (dpi *darwinProxyInfo) hasHTTPSProxy() bool {
	return dpi != nil && dpi.HTTPSEnabled && dpi.HTTPSProxy != "" && dpi.HTTPSPort != ""
}

func (dpi *darwinProxyInfo) hasAnyHTTPProxy() bool {
	return dpi.hasHTTPProxy() || dpi.hasHTTPSProxy()
}

func (dpi *darwinProxyInfo) hasSOCKSProxy() bool {
	return dpi != nil && dpi.SOCKSEnabled && dpi.SOCKSProxy != "" && dpi.SOCKSPort != ""
}

func (dpi *darwinProxyInfo) socksAddress() string {
	if !dpi.hasSOCKSProxy() {
		return ""
	}
	return fmt.Sprintf("%s:%s", dpi.SOCKSProxy, dpi.SOCKSPort)
}

func (dpi *darwinProxyInfo) proxyURLForScheme(scheme string) (*url.URL, error) {
	if dpi == nil {
		return nil, nil
	}
	normalizedScheme := scheme
	if normalizedScheme == "ws" {
		normalizedScheme = "http"
	} else if normalizedScheme == "wss" {
		normalizedScheme = "https"
	}

	if normalizedScheme == "https" {
		if dpi.hasHTTPSProxy() {
			return url.Parse(fmt.Sprintf("http://%s:%s", dpi.HTTPSProxy, dpi.HTTPSPort))
		}
		if dpi.hasHTTPProxy() {
			return url.Parse(fmt.Sprintf("http://%s:%s", dpi.HTTPProxy, dpi.HTTPPort))
		}
		return nil, nil
	}

	if dpi.hasHTTPProxy() {
		return url.Parse(fmt.Sprintf("http://%s:%s", dpi.HTTPProxy, dpi.HTTPPort))
	}
	if dpi.hasHTTPSProxy() {
		return url.Parse(fmt.Sprintf("http://%s:%s", dpi.HTTPSProxy, dpi.HTTPSPort))
	}
	return nil, nil
}

func selectDarwinProxyInfo() (*darwinProxySelection, error) {
	snapshot, err := getDarwinProxySnapshot()
	if err != nil {
		return nil, err
	}
	if snapshot == nil {
		return nil, nil
	}
	if connectedVPNs, err := getDarwinConnectedVPNs(); err == nil && len(connectedVPNs) > 0 {
		primary := connectedVPNs[0]
		selection := darwinProxySelection{source: SystemProxySourceVPN, name: primary.Name}
		if scoped, ok := snapshot.Scoped[primary.ID]; ok {
			selected := scoped
			selection.info = &selected
		}
		return &selection, nil
	}
	return &darwinProxySelection{info: &snapshot.Global, source: SystemProxySourceSystem}, nil
}

func getDarwinProxySnapshot() (*darwinProxySnapshot, error) {
	out, err := exec.Command("scutil", "--proxy").Output()
	if err != nil {
		return nil, err
	}
	return parseDarwinProxySnapshot(string(out)), nil
}

func parseDarwinProxySnapshot(output string) *darwinProxySnapshot {
	snapshot := &darwinProxySnapshot{
		Scoped: map[string]darwinProxyInfo{},
	}
	stack := []string{"root"}
	lines := strings.Split(output, "\n")
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		if ln == "}" {
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
			continue
		}
		if strings.HasSuffix(ln, "{") {
			open := strings.TrimSpace(strings.TrimSuffix(ln, "{"))
			if !strings.Contains(open, ":") {
				continue
			}
			parts := strings.SplitN(open, ":", 2)
			key := strings.TrimSpace(parts[0])
			stack = append(stack, key)
			continue
		}

		parts := strings.SplitN(ln, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if len(stack) == 1 {
			applyDarwinProxyValue(&snapshot.Global, key, val)
			continue
		}
		if len(stack) >= 2 && stack[len(stack)-2] == "Scoped" {
			serviceID := stack[len(stack)-1]
			info := snapshot.Scoped[serviceID]
			applyDarwinProxyValue(&info, key, val)
			snapshot.Scoped[serviceID] = info
		}
	}
	return snapshot
}

func applyDarwinProxyValue(info *darwinProxyInfo, key, val string) {
	if info == nil {
		return
	}
	switch key {
	case "HTTPEnable":
		info.HTTPEnabled = val == "1"
	case "HTTPProxy":
		info.HTTPProxy = val
	case "HTTPPort":
		info.HTTPPort = val
	case "HTTPSEnable":
		info.HTTPSEnabled = val == "1"
	case "HTTPSProxy":
		info.HTTPSProxy = val
	case "HTTPSPort":
		info.HTTPSPort = val
	case "SOCKSEnable":
		info.SOCKSEnabled = val == "1"
	case "SOCKSProxy":
		info.SOCKSProxy = val
	case "SOCKSPort":
		info.SOCKSPort = val
	case "ProxyAutoConfigEnable":
		info.PACEnabled = val == "1"
	case "ProxyAutoConfigURLString":
		info.PACURL = val
	}
}

type darwinVPNService struct {
	ID   string
	Name string
}

func getDarwinConnectedVPNs() ([]darwinVPNService, error) {
	out, err := exec.Command("scutil", "--nc", "list").Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(out), "\n")
	services := make([]darwinVPNService, 0)
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" || !strings.Contains(ln, "(Connected)") {
			continue
		}
		serviceID := parseDarwinVPNServiceID(ln)
		if serviceID == "" {
			continue
		}
		services = append(services, darwinVPNService{
			ID:   serviceID,
			Name: parseDarwinVPNDisplayName(ln),
		})
	}
	if len(services) == 0 {
		return nil, nil
	}
	return services, nil
}

func parseDarwinVPNServiceID(line string) string {
	return darwinServiceIDRegexp.FindString(line)
}

func parseDarwinVPNDisplayName(line string) string {
	start := strings.Index(line, "\"")
	if start == -1 {
		return ""
	}
	end := strings.Index(line[start+1:], "\"")
	if end == -1 {
		return ""
	}
	return line[start+1 : start+1+end]
}

func socks5DialContext(socksAddr string, baseTimeout time.Duration) func(ctx context.Context, network, address string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		var d net.Dialer
		if deadline, ok := ctx.Deadline(); ok {
			d.Timeout = time.Until(deadline)
		} else {
			d.Timeout = baseTimeout
		}
		conn, err := d.DialContext(ctx, "tcp", socksAddr)
		if err != nil {
			return nil, err
		}

		defer func() {
			if err != nil {
				conn.Close()
			}
		}()

		if _, err = conn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
			return nil, err
		}
		buf := make([]byte, 2)
		if _, err = ioReadFull(ctx, conn, buf); err != nil {
			return nil, err
		}
		if buf[0] != 0x05 || buf[1] != 0x00 {
			return nil, fmt.Errorf("socks5: unsupported method %d", buf[1])
		}

		host, portStr, err2 := net.SplitHostPort(address)
		if err2 != nil {
			return nil, err2
		}
		portNum, err2 := parsePort(portStr)
		if err2 != nil {
			return nil, err2
		}

		req := []byte{0x05, 0x01, 0x00}
		ip := net.ParseIP(host)
		if ip4 := ip.To4(); ip4 != nil {
			req = append(req, 0x01)
			req = append(req, ip4...)
		} else if ip6 := ip.To16(); ip6 != nil {
			req = append(req, 0x04)
			req = append(req, ip6...)
		} else {
			if len(host) > 255 {
				return nil, fmt.Errorf("socks5: host name too long")
			}
			req = append(req, 0x03, byte(len(host)))
			req = append(req, []byte(host)...)
		}
		req = append(req, byte(portNum>>8), byte(portNum))

		if _, err = conn.Write(req); err != nil {
			return nil, err
		}

		hdr := make([]byte, 4)
		if _, err = ioReadFull(ctx, conn, hdr); err != nil {
			return nil, err
		}
		if hdr[0] != 0x05 || hdr[1] != 0x00 {
			return nil, fmt.Errorf("socks5: connect failed, rep=%d", hdr[1])
		}
		var toRead int
		switch hdr[3] {
		case 0x01:
			toRead = 4
		case 0x04:
			toRead = 16
		case 0x03:
			lb := make([]byte, 1)
			if _, err = ioReadFull(ctx, conn, lb); err != nil {
				return nil, err
			}
			toRead = int(lb[0])
		default:
			return nil, fmt.Errorf("socks5: invalid atyp %d", hdr[3])
		}
		if toRead > 0 {
			dummy := make([]byte, toRead)
			if _, err = ioReadFull(ctx, conn, dummy); err != nil {
				return nil, err
			}
		}
		dummy := make([]byte, 2)
		if _, err = ioReadFull(ctx, conn, dummy); err != nil {
			return nil, err
		}

		return conn, nil
	}
}

func ioReadFull(ctx context.Context, conn net.Conn, buf []byte) (int, error) {
	type res struct {
		n   int
		err error
	}
	ch := make(chan res, 1)
	go func() {
		n, err := io.ReadFull(conn, buf)
		ch <- res{n, err}
	}()
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case r := <-ch:
		return r.n, r.err
	}
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

func populateHTTPEnv(proxyURL *url.URL) {
	if proxyURL == nil {
		return
	}
	proxyStr := proxyURL.String()
	os.Setenv("HTTP_PROXY", proxyStr)
	os.Setenv("http_proxy", proxyStr)
	os.Setenv("HTTPS_PROXY", proxyStr)
	os.Setenv("https_proxy", proxyStr)
}
