package dto

type SpriteAuthor struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

type SpriteSliceGrid struct {
	X []int `json:"x"`
	Y []int `json:"y"`
}

type Sprite struct {
	ID                string       `json:"id"`
	Name              string       `json:"name"`
	Description       string       `json:"description"`
	FrameCount        int          `json:"frameCount"`
	Columns           int          `json:"columns"`
	Rows              int          `json:"rows"`
	SpriteFile        string       `json:"spriteFile"`
	SpritePath        string       `json:"spritePath"`
	SourceType        string       `json:"sourceType,omitempty"`
	Origin            string       `json:"origin,omitempty"`
	Scope             string       `json:"scope"`
	Status            string       `json:"status"`
	ValidationMessage string       `json:"validationMessage,omitempty"`
	ImageWidth        int          `json:"imageWidth"`
	ImageHeight       int          `json:"imageHeight"`
	Author            SpriteAuthor `json:"author"`
	CreatedAt         string       `json:"createdAt"`
	UpdatedAt         string       `json:"updatedAt"`
	Version           string       `json:"version"`
	CoverImageDataURL string       `json:"coverImageDataUrl,omitempty"`
}

type InspectSpriteRequest struct {
	Path string `json:"path"`
}

type SpriteImportDraft struct {
	Path              string `json:"path"`
	PreviewPath       string `json:"previewPath"`
	SourceType        string `json:"sourceType"`
	Name              string `json:"name"`
	Description       string `json:"description"`
	AuthorDisplayName string `json:"authorDisplayName"`
	Version           string `json:"version"`
	FrameCount        int    `json:"frameCount"`
	Columns           int    `json:"columns"`
	Rows              int    `json:"rows"`
	SpriteFile        string `json:"spriteFile"`
	Status            string `json:"status"`
	ValidationMessage string `json:"validationMessage,omitempty"`
	ImageWidth        int    `json:"imageWidth"`
	ImageHeight       int    `json:"imageHeight"`
}

type ImportSpriteRequest struct {
	Path              string `json:"path"`
	Name              string `json:"name"`
	Description       string `json:"description"`
	AuthorDisplayName string `json:"authorDisplayName"`
	Version           string `json:"version"`
	Origin            string `json:"origin,omitempty"`
}

type InstallSpriteFromURLRequest struct {
	URL               string `json:"url"`
	SHA256            string `json:"sha256,omitempty"`
	Size              int64  `json:"size,omitempty"`
	Name              string `json:"name"`
	Description       string `json:"description"`
	AuthorDisplayName string `json:"authorDisplayName"`
	Version           string `json:"version"`
}

type UpdateSpriteRequest struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Description       string `json:"description"`
	AuthorDisplayName string `json:"authorDisplayName"`
	Version           string `json:"version"`
}

type ExportSpriteRequest struct {
	ID         string `json:"id"`
	OutputPath string `json:"outputPath"`
}

type DeleteSpriteRequest struct {
	ID string `json:"id"`
}

type GetSpriteManifestRequest struct {
	ID string `json:"id"`
}

type SpriteManifest struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Scope       string          `json:"scope"`
	SpritePath  string          `json:"spritePath"`
	SourceType  string          `json:"sourceType,omitempty"`
	ImageWidth  int             `json:"imageWidth"`
	ImageHeight int             `json:"imageHeight"`
	SheetWidth  int             `json:"sheetWidth"`
	SheetHeight int             `json:"sheetHeight"`
	Columns     int             `json:"columns"`
	Rows        int             `json:"rows"`
	SliceGrid   SpriteSliceGrid `json:"sliceGrid"`
	CanEdit     bool            `json:"canEdit"`
	UpdatedAt   string          `json:"updatedAt"`
}

type UpdateSpriteSlicesRequest struct {
	ID        string          `json:"id"`
	SliceGrid SpriteSliceGrid `json:"sliceGrid"`
}
