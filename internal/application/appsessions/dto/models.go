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
	SourceType                   string             `json:"sourceType,omitempty"`
	SourceBrowser                string             `json:"sourceBrowser,omitempty"`
	SourceProfile                string             `json:"sourceProfile,omitempty"`
	LastSyncedAt                 string             `json:"lastSyncedAt,omitempty"`
	Source                       *AppSessionSource  `json:"source,omitempty"`
}

type AppSessionSource struct {
	Mode         string `json:"mode,omitempty"`
	BrowserID    string `json:"browserId,omitempty"`
	BrowserLabel string `json:"browserLabel,omitempty"`
	ProfileID    string `json:"profileId,omitempty"`
	ProfileLabel string `json:"profileLabel,omitempty"`
	SyncedAt     string `json:"syncedAt,omitempty"`
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

// VerifyAppSessionRequest starts a read-only account verification using the
// credential snapshot already stored for the App Session. It never imports or
// replaces browser cookies.
type VerifyAppSessionRequest struct {
	ID string `json:"id"`
}

type BrowserProfileSelection struct {
	Mode      string `json:"mode,omitempty"`
	BrowserID string `json:"browserId"`
	ProfileID string `json:"profileId"`
}

type BrowserProfileDiscoveryRequest struct {
	BrowserID string `json:"browserId"`
}

type AppSessionBrowserScanItem struct {
	AppSessionID string `json:"appSessionId"`
	SiteKey      string `json:"siteKey"`
	Label        string `json:"label"`
	AccountLabel string `json:"accountLabel,omitempty"`
	Status       string `json:"status"`
	Selectable   bool   `json:"selectable"`
	Reason       string `json:"reason,omitempty"`
}

type AppSessionBrowserScanResult struct {
	BrowserID     string                      `json:"browserId"`
	ProfileID     string                      `json:"profileId"`
	SnapshotToken string                      `json:"snapshotToken"`
	Items         []AppSessionBrowserScanItem `json:"items"`
}

type AppSessionBrowserImportRequest struct {
	Mode          string   `json:"mode,omitempty"`
	BrowserID     string   `json:"browserId"`
	ProfileID     string   `json:"profileId"`
	SnapshotToken string   `json:"snapshotToken"`
	AppSessionIDs []string `json:"appSessionIds"`
}

type AppSessionBrowserImportResult struct {
	ImportedIDs []string `json:"importedIds"`
	SkippedIDs  []string `json:"skippedIds"`
}
