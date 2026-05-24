package dto

type Connector struct {
	ID              string            `json:"id"`
	Type            string            `json:"type"`
	Group           string            `json:"group"`
	Desc            string            `json:"desc"`
	Status          string            `json:"status"`
	CredentialMode  string            `json:"credentialMode"`
	CredentialState string            `json:"credentialState"`
	CookiesCount    int               `json:"cookiesCount"`
	Cookies         []ConnectorCookie `json:"cookies"`
	ProfileKey      string            `json:"profileKey,omitempty"`
	ProfilePath     string            `json:"profilePath,omitempty"`
	ProfileBrowser  string            `json:"profileBrowser,omitempty"`
	ProfileInfo     *ConnectorProfile `json:"profileInfo,omitempty"`
	Domains         []string          `json:"domains,omitempty"`
	ProfileSites    []ConnectorSite   `json:"profileSites,omitempty"`
	PolicyKey       string            `json:"policyKey,omitempty"`
	Capabilities    []string          `json:"capabilities,omitempty"`
	LastVerifiedAt  string            `json:"lastVerifiedAt"`
}

type ConnectorSite struct {
	Key   string `json:"key,omitempty"`
	Label string `json:"label,omitempty"`
	URL   string `json:"url"`
}

type ConnectorProfile struct {
	Path           string                      `json:"path,omitempty"`
	Browser        string                      `json:"browser,omitempty"`
	Exists         bool                        `json:"exists"`
	SizeBytes      int64                       `json:"sizeBytes"`
	FileCount      int                         `json:"fileCount"`
	DirectoryCount int                         `json:"directoryCount"`
	Components     []ConnectorProfileComponent `json:"components,omitempty"`
	Bindings       []ConnectorProfileBinding   `json:"bindings,omitempty"`
	Truncated      bool                        `json:"truncated,omitempty"`
	Error          string                      `json:"error,omitempty"`
}

type ConnectorProfileBinding struct {
	Browser        string `json:"browser"`
	Path           string `json:"path,omitempty"`
	Exists         bool   `json:"exists"`
	Current        bool   `json:"current,omitempty"`
	SizeBytes      int64  `json:"sizeBytes"`
	FileCount      int    `json:"fileCount"`
	DirectoryCount int    `json:"directoryCount"`
}

type ConnectorProfileComponent struct {
	Name           string `json:"name"`
	Path           string `json:"path,omitempty"`
	Kind           string `json:"kind,omitempty"`
	SizeBytes      int64  `json:"sizeBytes"`
	FileCount      int    `json:"fileCount"`
	DirectoryCount int    `json:"directoryCount"`
}

type UpsertConnectorRequest struct {
	ID             string `json:"id"`
	Type           string `json:"type"`
	Status         string `json:"status"`
	CredentialMode string `json:"credentialMode"`
	CookiesPath    string `json:"cookiesPath"`
}

type ClearConnectorRequest struct {
	ID string `json:"id"`
}

type StartConnectorConnectRequest struct {
	ID        string `json:"id"`
	TargetURL string `json:"targetUrl,omitempty"`
}

type StartConnectorConnectResult struct {
	SessionID string    `json:"sessionId"`
	Connector Connector `json:"connector"`
	TargetURL string    `json:"targetUrl,omitempty"`
}

type FinishConnectorConnectRequest struct {
	SessionID string `json:"sessionId"`
}

type FinishConnectorConnectResult struct {
	SessionID            string    `json:"sessionId"`
	Saved                bool      `json:"saved"`
	RawCookiesCount      int       `json:"rawCookiesCount"`
	FilteredCookiesCount int       `json:"filteredCookiesCount"`
	Domains              []string  `json:"domains,omitempty"`
	Reason               string    `json:"reason,omitempty"`
	Connector            Connector `json:"connector"`
}

type CancelConnectorConnectRequest struct {
	SessionID string `json:"sessionId"`
}

type ConnectorConnectSession struct {
	SessionID            string    `json:"sessionId"`
	ConnectorID          string    `json:"connectorId"`
	State                string    `json:"state"`
	BrowserStatus        string    `json:"browserStatus"`
	TargetURL            string    `json:"targetUrl,omitempty"`
	CurrentCookiesCount  int       `json:"currentCookiesCount"`
	Saved                bool      `json:"saved"`
	RawCookiesCount      int       `json:"rawCookiesCount"`
	FilteredCookiesCount int       `json:"filteredCookiesCount"`
	Domains              []string  `json:"domains,omitempty"`
	Reason               string    `json:"reason,omitempty"`
	Error                string    `json:"error,omitempty"`
	LastCookiesAt        string    `json:"lastCookiesAt,omitempty"`
	Connector            Connector `json:"connector"`
}

type GetConnectorConnectSessionRequest struct {
	SessionID string `json:"sessionId"`
}

type OpenConnectorSiteRequest struct {
	ID        string `json:"id"`
	TargetURL string `json:"targetUrl,omitempty"`
}

type ConnectorCookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain"`
	Path     string `json:"path"`
	Expires  int64  `json:"expires"`
	HttpOnly bool   `json:"httpOnly"`
	Secure   bool   `json:"secure"`
	SameSite string `json:"sameSite,omitempty"`
}
