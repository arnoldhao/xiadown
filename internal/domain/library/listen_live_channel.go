package library

import (
	"strings"
	"time"
)

type ListenLiveColumn struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	SortOrder int       `json:"sortOrder"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type ListenLiveChannel struct {
	ID           string    `json:"id"`
	ColumnID     string    `json:"columnId"`
	Title        string    `json:"title"`
	Channel      string    `json:"channel"`
	Description  string    `json:"description"`
	Source       string    `json:"source"`
	VideoID      string    `json:"videoId"`
	ThumbnailURL string    `json:"thumbnailUrl"`
	Enabled      bool      `json:"enabled"`
	SortOrder    int       `json:"sortOrder"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type ListenLiveCatalogSnapshot struct {
	Columns  []ListenLiveColumn  `json:"columns"`
	Channels []ListenLiveChannel `json:"channels"`
}

func NormalizeListenLiveColumn(item ListenLiveColumn, now time.Time) ListenLiveColumn {
	item.ID = strings.TrimSpace(item.ID)
	item.Title = strings.TrimSpace(item.Title)
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = now
	}
	return item
}

func NormalizeListenLiveChannel(item ListenLiveChannel, now time.Time) ListenLiveChannel {
	item.ID = strings.TrimSpace(item.ID)
	item.ColumnID = strings.TrimSpace(item.ColumnID)
	item.Title = strings.TrimSpace(item.Title)
	item.Channel = strings.TrimSpace(item.Channel)
	item.Description = strings.TrimSpace(item.Description)
	item.Source = strings.TrimSpace(item.Source)
	item.VideoID = strings.TrimSpace(item.VideoID)
	item.ThumbnailURL = strings.TrimSpace(item.ThumbnailURL)
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = now
	}
	return item
}
