package dto

type Connector struct {
	ID             string            `json:"id"`
	Type           string            `json:"type"`
	Group          string            `json:"group"`
	Desc           string            `json:"desc"`
	Status         string            `json:"status"`
	CookiesCount   int               `json:"cookiesCount"`
	Cookies        []ConnectorCookie `json:"cookies"`
	Domains        []string          `json:"domains,omitempty"`
	PolicyKey      string            `json:"policyKey,omitempty"`
	Capabilities   []string          `json:"capabilities,omitempty"`
	LastVerifiedAt string            `json:"lastVerifiedAt"`
}

type UpsertConnectorRequest struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	CookiesPath string `json:"cookiesPath"`
}

type ClearConnectorRequest struct {
	ID string `json:"id"`
}

type StartConnectorConnectRequest struct {
	ID string `json:"id"`
}

type StartConnectorConnectResult struct {
	SessionID string    `json:"sessionId"`
	Connector Connector `json:"connector"`
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
	ID string `json:"id"`
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
