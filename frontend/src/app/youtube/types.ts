export type YouTubeWorkspaceRouteId =
  | "search"
  | "home"
  | "subscriptions"
  | "explore"
  | "shorts"
  | "liked-videos"
  | "watch-later"
  | "playlists"
  | "history";

export interface YouTubeWorkspaceVideo {
  itemKind?: "video" | "playlist";
  videoId: string;
  playlistId?: string;
  title: string;
  channel?: string;
  channelId?: string;
  thumbnailUrl?: string;
  durationSeconds?: number;
  durationLabel?: string;
  viewCount?: number;
  publishedLabel?: string;
  webUrl: string;
  live?: boolean;
  short?: boolean;
}

export interface YouTubeWorkspacePlayVideoRequest {
	requestId: number;
	video: YouTubeWorkspaceVideo;
	locale?: string;
}

export interface YouTubeWorkspacePlayRequest {
	requestId: number;
}

export interface YouTubeWorkspaceBrowseRequest {
  routeId: YouTubeWorkspaceRouteId;
  query?: string;
  playlistId?: string;
  continuation?: string;
  locale?: string;
}

export interface YouTubeWorkspaceBrowsePage {
  routeId: YouTubeWorkspaceRouteId;
  title: string;
  webUrl: string;
  items: YouTubeWorkspaceVideo[];
  continuation?: string;
  requiresAuthentication?: boolean;
  emptyReason?: string;
}

export interface YouTubeVideoDetailsRequest {
  videoId: string;
  locale?: string;
}

export interface YouTubeVideoDetails {
  videoId: string;
  title: string;
  channel?: string;
  channelId?: string;
	channelAvatarUrl?: string;
  thumbnailUrl?: string;
  durationSeconds?: number;
  viewCount?: number;
  likeCount?: number;
  publishedDate?: string;
  publishedLabel?: string;
  description?: string;
	isSubscribed?: boolean;
  webUrl: string;
}

export type YouTubeVideoRating = "like" | "dislike" | "none";

export interface YouTubeVideoRatingRequest {
  videoId: string;
  rating: YouTubeVideoRating;
}

export interface YouTubeChannelSubscriptionRequest {
	channelId: string;
	subscribed: boolean;
}

export interface YouTubeUploaderRequest {
	channelId: string;
	continuation?: string;
	locale?: string;
}

export interface YouTubeUploaderPageData {
	channelId: string;
	name: string;
	handle?: string;
	description?: string;
	avatarUrl?: string;
	bannerUrl?: string;
	subscriberCount?: number;
	subscriberLabel?: string;
	videoCountLabel?: string;
	isSubscribed?: boolean;
	webUrl: string;
	videos: YouTubeWorkspaceVideo[];
	continuation?: string;
}

export interface YouTubePlaybackDescriptor {
  source: "youtube";
  mediaKind: "video";
  sessionId: string;
  videoId: string;
  title: string;
  artist?: string;
  channelId?: string;
  thumbnailUrl?: string;
  durationSeconds?: number;
  viewCount?: number;
  publishedLabel?: string;
  webUrl: string;
}

export interface YouTubePlayerStatus {
  provider?: "youtube" | "stream";
  sessionId?: string;
  available?: boolean;
  videoId?: string;
  state?: string;
  title?: string;
  artist?: string;
  thumbnailUrl?: string;
  currentTime?: number;
  duration?: number;
	volume?: number;
	muted?: boolean;
	controls?: YouTubePlayerControls;
	captionOptions?: YouTubePlayerOption[];
	audioTrackOptions?: YouTubePlayerOption[];
	qualityOptions?: YouTubePlayerOption[];
	playbackRateOptions?: YouTubePlayerOption[];
	selections?: YouTubePlayerSelections;
  errorCode?: string;
  errorMessage?: string;
}

export interface YouTubePlayerOption {
	id: string;
	label: string;
}

export interface YouTubePlayerControls {
	like: boolean;
	dislike: boolean;
	captions: boolean;
	audioTrack: boolean;
	quality: boolean;
	volume: boolean;
	playbackRate?: boolean;
}

export interface YouTubePlayerSelections {
	rating?: "like" | "dislike" | "none";
	captionId?: string;
	audioTrackId?: string;
	qualityId?: string;
	playbackRateId?: string;
}

export interface YouTubePlaybackCapabilities {
  previous: boolean;
  next: boolean;
  playPause: boolean;
  like: boolean;
  dislike: boolean;
  fullscreen: boolean;
  captions: boolean;
  audioTrack: boolean;
  quality: boolean;
  volume: boolean;
  playbackRate?: boolean;
}

export interface YouTubeWorkspacePlaybackState {
  descriptor: YouTubePlaybackDescriptor;
  status: YouTubePlayerStatus;
  currentIndex: number;
  queue: YouTubeWorkspaceVideo[];
  muted: boolean;
  volume: number;
  capabilities: YouTubePlaybackCapabilities;
}

export interface YouTubeWorkspaceExternalCommand {
	id: number;
	command: "previous" | "next" | "stop";
	revealWatch?: boolean;
}
