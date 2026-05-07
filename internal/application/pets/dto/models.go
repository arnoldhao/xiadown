package dto

type PetScope = string
type PetStatus = string

const (
	PetScopeBuiltin  PetScope = "builtin"
	PetScopeImported PetScope = "imported"

	PetStatusReady   PetStatus = "ready"
	PetStatusInvalid PetStatus = "invalid"
)

type Pet struct {
	ID                string `json:"id"`
	DisplayName       string `json:"displayName"`
	Description       string `json:"description"`
	FrameCount        int    `json:"frameCount"`
	Columns           int    `json:"columns"`
	Rows              int    `json:"rows"`
	CellWidth         int    `json:"cellWidth"`
	CellHeight        int    `json:"cellHeight"`
	SpritesheetFile   string `json:"spritesheetFile"`
	SpritesheetPath   string `json:"spritesheetPath"`
	Origin            string `json:"origin,omitempty"`
	Scope             string `json:"scope"`
	Status            string `json:"status"`
	ValidationCode    string `json:"validationCode,omitempty"`
	ValidationMessage string `json:"validationMessage,omitempty"`
	ImageWidth        int    `json:"imageWidth"`
	ImageHeight       int    `json:"imageHeight"`
	CreatedAt         string `json:"createdAt"`
	UpdatedAt         string `json:"updatedAt"`
}

type InspectPetRequest struct {
	Path string `json:"path"`
}

type PetImportDraft struct {
	Path              string `json:"path"`
	DisplayName       string `json:"displayName"`
	Description       string `json:"description"`
	FrameCount        int    `json:"frameCount"`
	Columns           int    `json:"columns"`
	Rows              int    `json:"rows"`
	CellWidth         int    `json:"cellWidth"`
	CellHeight        int    `json:"cellHeight"`
	SpritesheetFile   string `json:"spritesheetFile"`
	Status            string `json:"status"`
	ValidationCode    string `json:"validationCode,omitempty"`
	ValidationMessage string `json:"validationMessage,omitempty"`
	ImageWidth        int    `json:"imageWidth"`
	ImageHeight       int    `json:"imageHeight"`
}

type ImportPetRequest struct {
	Path   string `json:"path"`
	Origin string `json:"origin,omitempty"`
}

type StartOnlinePetImportRequest struct {
	SiteID string `json:"siteId"`
}

type GetOnlinePetImportSessionRequest struct {
	SessionID string `json:"sessionId"`
}

type FinishOnlinePetImportSessionRequest struct {
	SessionID string `json:"sessionId"`
}

type OnlinePetImportSession struct {
	SessionID     string `json:"sessionId"`
	SiteID        string `json:"siteId"`
	SiteLabel     string `json:"siteLabel"`
	URL           string `json:"url"`
	State         string `json:"state"`
	BrowserStatus string `json:"browserStatus"`
	ImportedPets  []Pet  `json:"importedPets"`
	ErrorCode     string `json:"errorCode,omitempty"`
	Error         string `json:"error,omitempty"`
	UpdatedAt     string `json:"updatedAt"`
}

type ExportPetRequest struct {
	ID         string `json:"id"`
	OutputPath string `json:"outputPath"`
}

type DeletePetRequest struct {
	ID string `json:"id"`
}

type GetPetManifestRequest struct {
	ID string `json:"id"`
}

type PetManifest struct {
	ID              string `json:"id"`
	DisplayName     string `json:"displayName"`
	Description     string `json:"description"`
	Scope           string `json:"scope"`
	SpritesheetPath string `json:"spritesheetPath"`
	ImageWidth      int    `json:"imageWidth"`
	ImageHeight     int    `json:"imageHeight"`
	SheetWidth      int    `json:"sheetWidth"`
	SheetHeight     int    `json:"sheetHeight"`
	Columns         int    `json:"columns"`
	Rows            int    `json:"rows"`
	CellWidth       int    `json:"cellWidth"`
	CellHeight      int    `json:"cellHeight"`
	CanDelete       bool   `json:"canDelete"`
	UpdatedAt       string `json:"updatedAt"`
}
