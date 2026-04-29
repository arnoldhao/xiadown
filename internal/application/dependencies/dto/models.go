package dto

type Dependency struct {
	Name        string `json:"name"`
	Kind        string `json:"kind,omitempty"`
	ExecPath    string `json:"execPath"`
	Version     string `json:"version"`
	Status      string `json:"status"`
	SourceKind  string `json:"sourceKind,omitempty"`
	SourceRef   string `json:"sourceRef,omitempty"`
	Manager     string `json:"manager,omitempty"`
	InstalledAt string `json:"installedAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type DependencyUpdateInfo struct {
	Name               string `json:"name"`
	LatestVersion      string `json:"latestVersion"`
	RecommendedVersion string `json:"recommendedVersion,omitempty"`
	UpstreamVersion    string `json:"upstreamVersion,omitempty"`
	ReleaseNotes       string `json:"releaseNotes"`
	ReleaseNotesURL    string `json:"releaseNotesUrl"`
	AutoUpdate         bool   `json:"autoUpdate,omitempty"`
	Required           bool   `json:"required,omitempty"`
}

type DependencyInstallState struct {
	Name      string `json:"name"`
	Stage     string `json:"stage"`
	Progress  int    `json:"progress"`
	Message   string `json:"message"`
	UpdatedAt string `json:"updatedAt"`
}

type InstallDependencyRequest struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Manager string `json:"manager,omitempty"`
}

type SetDependencyPathRequest struct {
	Name     string `json:"name"`
	ExecPath string `json:"execPath"`
}

type VerifyDependencyRequest struct {
	Name string `json:"name"`
}

type RemoveDependencyRequest struct {
	Name string `json:"name"`
}

type OpenDependencyDirectoryRequest struct {
	Name string `json:"name"`
}

type GetDependencyInstallStateRequest struct {
	Name string `json:"name"`
}
