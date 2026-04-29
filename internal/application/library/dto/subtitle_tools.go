package dto

type SubtitleCue struct {
	Index int    `json:"index"`
	Start string `json:"start"`
	End   string `json:"end"`
	Text  string `json:"text"`
}

type SubtitleDocument struct {
	Format   string         `json:"format"`
	Cues     []SubtitleCue  `json:"cues"`
	Metadata map[string]any `json:"metadata,omitempty"`
}
