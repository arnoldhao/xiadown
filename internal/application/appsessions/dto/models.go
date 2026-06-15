package dto

type AppSession struct {
	ID                           string             `json:"id"`
	SiteKey                      string             `json:"siteKey"`
	Group                        string             `json:"group"`
	Label                        string             `json:"label"`
	Desc                         string             `json:"desc"`
	Status                       string             `json:"status"`
	CredentialState              string             `json:"credentialState"`
	CookiesCount                 int                `json:"cookiesCount"`
	Cookies                      []AppSessionCookie `json:"cookies"`
	Domains                      []string           `json:"domains,omitempty"`
	Account                      *AppSessionAccount `json:"account,omitempty"`
	PolicyKey                    string             `json:"policyKey,omitempty"`
	Capabilities                 []string           `json:"capabilities,omitempty"`
	ProviderSupported            bool               `json:"providerSupported"`
	AccountVerificationStatus    string             `json:"accountVerificationStatus"`
	AccountVerificationError     string             `json:"accountVerificationError,omitempty"`
	AccountVerificationStartedAt string             `json:"accountVerificationStartedAt,omitempty"`
	LastVerifiedAt               string             `json:"lastVerifiedAt"`
}

type AppSessionAccount struct {
	DisplayName string            `json:"displayName,omitempty"`
	Handle      string            `json:"handle,omitempty"`
	AvatarURL   string            `json:"avatarURL,omitempty"`
	TierKey     string            `json:"tierKey,omitempty"`
	TierLabel   string            `json:"tierLabel,omitempty"`
	Badges      []AppSessionBadge `json:"badges,omitempty"`
	Metadata    map[string]any    `json:"metadata,omitempty"`
	ExpiresAt   string            `json:"expiresAt,omitempty"`
}

type AppSessionBadge struct {
	Key   string `json:"key,omitempty"`
	Label string `json:"label,omitempty"`
}

type AppSessionCookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain"`
	Path     string `json:"path"`
	Expires  int64  `json:"expires"`
	HttpOnly bool   `json:"httpOnly"`
	Secure   bool   `json:"secure"`
	SameSite string `json:"sameSite,omitempty"`
}

type ClearAppSessionRequest struct {
	ID string `json:"id"`
}

type StartAppSessionConnectRequest struct {
	ID        string `json:"id"`
	TargetURL string `json:"targetUrl,omitempty"`
}

type StartAppSessionConnectResult struct {
	SessionID  string     `json:"sessionId"`
	AppSession AppSession `json:"appSession"`
	TargetURL  string     `json:"targetUrl,omitempty"`
}

type FinishAppSessionConnectRequest struct {
	SessionID string `json:"sessionId"`
}

type FinishAppSessionConnectResult struct {
	SessionID            string     `json:"sessionId"`
	Saved                bool       `json:"saved"`
	RawCookiesCount      int        `json:"rawCookiesCount"`
	FilteredCookiesCount int        `json:"filteredCookiesCount"`
	Domains              []string   `json:"domains,omitempty"`
	Reason               string     `json:"reason,omitempty"`
	AppSession           AppSession `json:"appSession"`
}

type CancelAppSessionConnectRequest struct {
	SessionID string `json:"sessionId"`
}

type GetAppSessionConnectSessionRequest struct {
	SessionID string `json:"sessionId"`
}

type AppSessionConnectSession struct {
	SessionID            string     `json:"sessionId"`
	AppSessionID         string     `json:"appSessionId"`
	State                string     `json:"state"`
	BrowserStatus        string     `json:"browserStatus"`
	TargetURL            string     `json:"targetUrl,omitempty"`
	CurrentCookiesCount  int        `json:"currentCookiesCount"`
	Saved                bool       `json:"saved"`
	RawCookiesCount      int        `json:"rawCookiesCount"`
	FilteredCookiesCount int        `json:"filteredCookiesCount"`
	Domains              []string   `json:"domains,omitempty"`
	Reason               string     `json:"reason,omitempty"`
	Error                string     `json:"error,omitempty"`
	LastCookiesAt        string     `json:"lastCookiesAt,omitempty"`
	AppSession           AppSession `json:"appSession"`
}

type OpenAppSessionSiteRequest struct {
	ID        string `json:"id"`
	TargetURL string `json:"targetUrl,omitempty"`
}
