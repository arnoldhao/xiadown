import type { YouTubeWorkspaceVideo } from "@/app/youtube/types";

export interface YouTubeUploaderTarget {
	channelId: string;
	name: string;
	avatarUrl?: string;
	subscribed: boolean;
	videoId: string;
}

export function youtubeChannelURL(channelId?: string) {
	const normalized = channelId?.trim() || "";
	return /^UC[A-Za-z0-9_-]{20,}$/.test(normalized)
		? `https://www.youtube.com/channel/${encodeURIComponent(normalized)}`
		: "";
}

export function createYouTubeBrowseUploaderTarget(
	video: YouTubeWorkspaceVideo,
	fallbackName: string,
	subscribed = false,
): YouTubeUploaderTarget | null {
	const channelId = video.channelId?.trim() || "";
	if (!youtubeChannelURL(channelId)) {
		return null;
	}
	return {
		channelId,
		name: video.channel?.trim() || fallbackName,
		subscribed,
		videoId: video.videoId?.trim() || "",
	};
}
