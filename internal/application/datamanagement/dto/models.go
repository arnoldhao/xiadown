package dto

type DataManagementSnapshot struct {
	TotalBytes           int64                    `json:"totalBytes"`
	SafeReclaimableBytes int64                    `json:"safeReclaimableBytes"`
	ScannedAt            string                   `json:"scannedAt"`
	Categories           []DataManagementCategory `json:"categories"`
}

type DataManagementCategory struct {
	ID         string               `json:"id"`
	LabelKey   string               `json:"labelKey"`
	TotalBytes int64                `json:"totalBytes"`
	Items      []DataManagementItem `json:"items"`
}

type DataManagementItem struct {
	ID                string `json:"id"`
	LabelKey          string `json:"labelKey"`
	DescriptionKey    string `json:"descriptionKey"`
	SizeBytes         int64  `json:"sizeBytes"`
	ItemCount         int    `json:"itemCount,omitempty"`
	State             string `json:"state"`
	Risk              string `json:"risk"`
	Clearable         bool   `json:"clearable"`
	SelectedByDefault bool   `json:"selectedByDefault,omitempty"`
}

type CleanDataManagementRequest struct {
	ResourceIDs []string `json:"resourceIds"`
}

type CleanDataManagementResult struct {
	ResourceID string `json:"resourceId"`
	Status     string `json:"status"`
	BytesFreed int64  `json:"bytesFreed,omitempty"`
	Message    string `json:"message,omitempty"`
}

type CleanDataManagementResponse struct {
	Results  []CleanDataManagementResult `json:"results"`
	Snapshot DataManagementSnapshot      `json:"snapshot"`
}

type ResetApplicationResponse struct {
	Scheduled bool `json:"scheduled"`
}
