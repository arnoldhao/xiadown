package libraryaccess

type Config struct {
	RemoteEnabled      bool   `json:"remoteEnabled"`
	LANEnabled         bool   `json:"lanEnabled"`
	LANPort            int    `json:"lanPort"`
	TailscaleEnabled   bool   `json:"tailscaleEnabled"`
	TailscaleHTTPSPort int    `json:"tailscaleHTTPSPort"`
	TailscalePath      string `json:"tailscalePath"`
	DeviceName         string `json:"deviceName"`
}

type UpdateConfigRequest struct {
	RemoteEnabled      *bool   `json:"remoteEnabled"`
	LANEnabled         *bool   `json:"lanEnabled"`
	LANPort            *int    `json:"lanPort"`
	TailscaleEnabled   *bool   `json:"tailscaleEnabled"`
	TailscaleHTTPSPort *int    `json:"tailscaleHTTPSPort"`
	TailscalePath      *string `json:"tailscalePath"`
	DeviceName         *string `json:"deviceName"`
}

type Status struct {
	DesiredEnabled bool            `json:"desiredEnabled"`
	LAN            LANStatus       `json:"lan"`
	Tailscale      TailscaleStatus `json:"tailscale"`
	// observed states bypass cached Apply errors for the background reconciler;
	// they are intentionally not part of the Wails/public contract.
	observedLANState       string
	observedTailscaleState string
}

type LANStatus struct {
	DesiredEnabled bool   `json:"desiredEnabled"`
	State          string `json:"state"`
	Port           int    `json:"port"`
	LastError      string `json:"lastError,omitempty"`
}

type TailscaleStatus struct {
	DesiredEnabled bool   `json:"desiredEnabled"`
	State          string `json:"state"`
	Installed      bool   `json:"installed"`
	Version        string `json:"version,omitempty"`
	Tailnet        string `json:"tailnet,omitempty"`
	Device         string `json:"device,omitempty"`
	ServeURL       string `json:"serveURL,omitempty"`
	LastError      string `json:"lastError,omitempty"`
}
