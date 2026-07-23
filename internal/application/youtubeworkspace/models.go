package youtubeworkspace

type BrowseRequest struct {
	RouteID      string `json:"routeId"`
	Query        string `json:"query,omitempty"`
	PlaylistID   string `json:"playlistId,omitempty"`
	Continuation string `json:"continuation,omitempty"`
	Locale       string `json:"locale,omitempty"`
}

type BrowsePage struct {
	RouteID                string  `json:"routeId"`
	Title                  string  `json:"title"`
	WebURL                 string  `json:"webUrl"`
	Items                  []Video `json:"items"`
	Continuation           string  `json:"continuation,omitempty"`
	RequiresAuthentication bool    `json:"requiresAuthentication,omitempty"`
	EmptyReason            string  `json:"emptyReason,omitempty"`
}

// VideoDetailsRequest identifies a regular YouTube video whose watch metadata
// should be loaded through the WEB InnerTube player endpoint.
type VideoDetailsRequest struct {
	VideoID string `json:"videoId"`
	Locale  string `json:"locale,omitempty"`
}

// VideoDetails contains the richer, canonical metadata exposed by the player
// endpoint. Browse results intentionally remain lightweight; callers can load
// this shape only when opening the video's information surface.
type VideoDetails struct {
	VideoID          string  `json:"videoId"`
	Title            string  `json:"title"`
	Channel          string  `json:"channel,omitempty"`
	ChannelID        string  `json:"channelId,omitempty"`
	ChannelAvatarURL string  `json:"channelAvatarUrl,omitempty"`
	ThumbnailURL     string  `json:"thumbnailUrl,omitempty"`
	DurationSeconds  float64 `json:"durationSeconds,omitempty"`
	ViewCount        int64   `json:"viewCount,omitempty"`
	LikeCount        int64   `json:"likeCount,omitempty"`
	PublishedDate    string  `json:"publishedDate,omitempty"`
	PublishedLabel   string  `json:"publishedLabel,omitempty"`
	Description      string  `json:"description,omitempty"`
	IsSubscribed     bool    `json:"isSubscribed,omitempty"`
	WebURL           string  `json:"webUrl"`
}

type VideoRating string

const (
	VideoRatingNone    VideoRating = "none"
	VideoRatingLike    VideoRating = "like"
	VideoRatingDislike VideoRating = "dislike"
)

type VideoRatingRequest struct {
	VideoID string      `json:"videoId"`
	Rating  VideoRating `json:"rating"`
}

type ChannelSubscriptionRequest struct {
	ChannelID  string `json:"channelId"`
	Subscribed bool   `json:"subscribed"`
}

// UploaderRequest identifies a regular YouTube channel browse surface. The
// initial request returns the channel header and first video page; continuation
// requests return only the next video page while preserving the same shape.
type UploaderRequest struct {
	ChannelID    string `json:"channelId"`
	Continuation string `json:"continuation,omitempty"`
	Locale       string `json:"locale,omitempty"`
}

// UploaderPage is the app-native representation of a YouTube channel. Labels
// such as subscriber and video counts intentionally remain localized strings
// from InnerTube, while SubscriberCount is also exposed for accessible compact
// formatting when the label is unavailable.
type UploaderPage struct {
	ChannelID       string  `json:"channelId"`
	Name            string  `json:"name"`
	Handle          string  `json:"handle,omitempty"`
	Description     string  `json:"description,omitempty"`
	AvatarURL       string  `json:"avatarUrl,omitempty"`
	BannerURL       string  `json:"bannerUrl,omitempty"`
	SubscriberCount int64   `json:"subscriberCount,omitempty"`
	SubscriberLabel string  `json:"subscriberLabel,omitempty"`
	VideoCountLabel string  `json:"videoCountLabel,omitempty"`
	IsSubscribed    bool    `json:"isSubscribed,omitempty"`
	WebURL          string  `json:"webUrl"`
	Videos          []Video `json:"videos"`
	Continuation    string  `json:"continuation,omitempty"`
}

type Video struct {
	ItemKind        string  `json:"itemKind,omitempty"`
	VideoID         string  `json:"videoId"`
	PlaylistID      string  `json:"playlistId,omitempty"`
	Title           string  `json:"title"`
	Channel         string  `json:"channel,omitempty"`
	ChannelID       string  `json:"channelId,omitempty"`
	ThumbnailURL    string  `json:"thumbnailUrl,omitempty"`
	DurationSeconds float64 `json:"durationSeconds,omitempty"`
	DurationLabel   string  `json:"durationLabel,omitempty"`
	ViewCount       int64   `json:"viewCount,omitempty"`
	PublishedLabel  string  `json:"publishedLabel,omitempty"`
	WebURL          string  `json:"webUrl"`
	Live            bool    `json:"live,omitempty"`
	Short           bool    `json:"short,omitempty"`
}

// PlaybackDescriptor is deliberately player-neutral. It is returned when a
// workspace result is opened so the global playback coordinator can adopt the
// same metadata without coupling itself to the browse response.
type PlaybackDescriptor struct {
	Source          string  `json:"source"`
	MediaKind       string  `json:"mediaKind"`
	SessionID       string  `json:"sessionId,omitempty"`
	VideoID         string  `json:"videoId"`
	Title           string  `json:"title"`
	Artist          string  `json:"artist,omitempty"`
	ChannelID       string  `json:"channelId,omitempty"`
	ThumbnailURL    string  `json:"thumbnailUrl,omitempty"`
	DurationSeconds float64 `json:"durationSeconds,omitempty"`
	ViewCount       int64   `json:"viewCount,omitempty"`
	PublishedLabel  string  `json:"publishedLabel,omitempty"`
	WebURL          string  `json:"webUrl"`
}
