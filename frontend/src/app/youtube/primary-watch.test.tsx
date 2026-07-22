import { describe, expect, mock, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import type {
	YouTubePlaybackDescriptor,
	YouTubePlayerStatus,
	YouTubeVideoDetails,
	YouTubeWorkspaceVideo,
} from "@/app/youtube/types";

mock.module("@wailsio/runtime", () => ({
	Call: {
		ByName: () => Promise.resolve(undefined),
	},
	Events: {
		On: () => () => {},
	},
}));

const { YouTubePrimaryWatchPage, YouTubeVideoCard, isYouTubeNativeSurfaceRectVisible } = await import(
	"@/app/youtube/YouTubeWorkspacePage"
);

const currentVideo: YouTubeWorkspaceVideo = {
	itemKind: "video",
	videoId: "AbCdEfGh123",
	title: "Localized history title",
	channel: "Queue channel",
	thumbnailUrl: "https://i.ytimg.com/vi/AbCdEfGh123/hqdefault.jpg",
	viewCount: 1_250_000,
	publishedLabel: "2 days ago",
	webUrl: "https://www.youtube.com/watch?v=AbCdEfGh123",
};

const playback: YouTubePlaybackDescriptor = {
	source: "youtube",
	mediaKind: "video",
	sessionId: "youtube-session",
	videoId: currentVideo.videoId,
	title: "Localized history title",
	artist: "Playback creator",
	thumbnailUrl: currentVideo.thumbnailUrl,
	webUrl: currentVideo.webUrl,
};

const status: YouTubePlayerStatus = {
	state: "playing",
	title: "Player metadata title",
	artist: "Primary Watch creator",
	thumbnailUrl: currentVideo.thumbnailUrl,
};

const videoDetails: YouTubeVideoDetails = {
	videoId: currentVideo.videoId,
	title: "Canonical source title",
	channel: "Primary Watch creator",
	channelAvatarUrl: "https://yt3.example/primary-watch-avatar.jpg",
	thumbnailUrl: currentVideo.thumbnailUrl,
	viewCount: 1_250_000,
	likeCount: 48_200,
	publishedLabel: "2 days ago",
	description: "A complete video description.",
	webUrl: currentVideo.webUrl,
};

const labels = {
	back: "label.back",
	uploader: "label.uploader",
	more: "label.more",
	like: "label.like",
	dislike: "label.dislike",
	subscribe: "label.subscribe",
	unsubscribe: "label.unsubscribe",
	download: "label.download",
	openURL: "label.openURL",
	unavailable: "label.unavailable",
	videoInfo: "label.videoInfo",
	description: "label.description",
	descriptionUnavailable: "label.descriptionUnavailable",
	published: "label.published",
	views: "label.views",
	likes: "label.likes",
	close: "label.close",
};

function renderWatch(options: {
	uploaderAction?: boolean;
	subscriptionAction?: boolean;
	subscribed?: boolean;
} = {}) {
	return renderToStaticMarkup(
		<YouTubePrimaryWatchPage
			video={currentVideo}
			videoDetails={videoDetails}
			playback={playback}
			status={status}
			rating="none"
			subscribed={options.subscribed ?? false}
			subscriptionBusy={false}
			infoOpen={false}
			player={<div data-testid="injected-primary-player">player.surface</div>}
			transport={<div data-testid="injected-watch-footer">footer.surface</div>}
			locale="en"
			fallbackChannel="channel.fallback"
			labels={labels}
			onBack={() => {}}
			onOpenURL={() => {}}
			onLike={() => {}}
			onDislike={() => {}}
			onToggleSubscription={
				options.subscriptionAction === false ? undefined : () => {}
			}
			onInfoOpenChange={() => {}}
			onOpenUploader={options.uploaderAction === false ? undefined : () => {}}
		/>,
	);
}

describe("YouTubePrimaryWatchPage", () => {
	test("lets another station reuse the Home card while owning its thumbnail boundary", () => {
		const markup = renderToStaticMarkup(
			<YouTubeVideoCard
				fallbackChannel="RSS"
				locale="en"
				opening
				selected={false}
				thumbnail={<span data-testid="trusted-rss-thumbnail" />}
				video={currentVideo}
				onOpen={() => {}}
			/>,
		);
		expect(markup).toContain("youtube-workspace-video-card");
		expect(markup).toContain("youtube-workspace-thumbnail");
		expect(markup).toContain('data-testid="trusted-rss-thumbnail"');
		expect(markup).not.toContain(currentVideo.thumbnailUrl);
		expect(markup).toContain('data-surface-role="status"');
		expect(markup).toContain('data-material="regular"');
		expect(markup).toContain('data-shape="card"');
		expect(markup).toContain('role="status"');
	});

	test("renders a fixed header, adaptive video region, and footer in order", () => {
		const markup = renderWatch();

		expect(markup).toContain('data-testid="injected-primary-player"');
		expect(markup).toContain('data-testid="injected-watch-footer"');
		expect(markup).toContain("youtube-workspace-watch-page");
		expect(markup).toContain("youtube-workspace-watch-header");
		expect(markup).toContain("youtube-workspace-watch-video-region");
		expect(markup).toContain("Localized history title");
		expect(markup).not.toContain("Canonical source title");
		expect(markup).toContain("Primary Watch creator");
		expect(markup).toContain('aria-label="label.back"');
		expect(markup).not.toContain(">label.back</span>");
		expect(markup).toContain(
			'aria-label="label.uploader: Primary Watch creator"',
		);
		expect(markup).toContain("1.3M");
		expect(markup).toContain("48.2K");
		expect(markup).toContain("2 days ago");
		expect(markup).toContain('aria-label="label.more"');
		expect(markup.indexOf("youtube-workspace-watch-header")).toBeLessThan(
			markup.indexOf("youtube-workspace-watch-video-region"),
		);
		expect(markup.indexOf("youtube-workspace-watch-video-region")).toBeLessThan(
			markup.indexOf('data-testid="injected-watch-footer"'),
		);
		expect(markup).not.toContain("youtube-workspace-watch-scroll");
		expect(markup).not.toContain("youtube-workspace-watch-related");
		expect(markup).not.toContain("youtube-workspace-grid");
	});

	test("puts a distinct subscription icon first in the uploader row", () => {
		const unsubscribedMarkup = renderWatch({ subscribed: false });
		const subscribedMarkup = renderWatch({ subscribed: true });

		expect(unsubscribedMarkup).toContain('aria-label="label.subscribe"');
		expect(unsubscribedMarkup).toContain("lucide-user-plus");
		expect(subscribedMarkup).toContain('aria-label="label.unsubscribe"');
		expect(subscribedMarkup).toContain("lucide-user-check");
		for (const markup of [unsubscribedMarkup, subscribedMarkup]) {
			const subscribeClass = markup.indexOf(
				"youtube-workspace-watch-subscribe",
			);
			const subscribeButtonStart = markup.lastIndexOf(
				"<button",
				subscribeClass,
			);
			const subscribeButtonEnd = markup.indexOf(
				"</button>",
				subscribeClass,
			);
			const subscribeButton = markup.slice(
				subscribeButtonStart,
				subscribeButtonEnd,
			);
			expect(subscribeButton).toContain('data-variant="ghost"');
			expect(subscribeButton).toContain('data-size="compactIcon"');
			expect(subscribeButton).toContain('data-shape="circle"');
			expect(subscribeButton).not.toContain('data-variant="outline"');
		}
		expect(
			unsubscribedMarkup.indexOf("youtube-workspace-watch-subscribe"),
		).toBeLessThan(
			unsubscribedMarkup.indexOf("youtube-workspace-watch-uploader"),
		);
	});

	test("keeps the round channel avatar and name inside one uploader action", () => {
		const markup = renderWatch();
		const uploaderStart = markup.indexOf(
			'aria-label="label.uploader: Primary Watch creator"',
		);
		const uploaderEnd = markup.indexOf("</button>", uploaderStart);
		const uploaderMarkup = markup.slice(uploaderStart, uploaderEnd);

		expect(uploaderMarkup).toContain("youtube-workspace-watch-uploader-avatar");
		expect(uploaderMarkup).toContain(
			"https://yt3.example/primary-watch-avatar.jpg",
		);
		expect(uploaderMarkup).toContain("Primary Watch creator");
		expect(uploaderMarkup.indexOf("uploader-avatar")).toBeLessThan(
			uploaderMarkup.indexOf("<strong>Primary Watch creator"),
		);
	});

	test("keeps uploader metadata visible when no uploader action is available", () => {
		const markup = renderWatch({ uploaderAction: false });

		expect(markup).toContain("Primary Watch creator");
		expect(markup).toContain('class="youtube-workspace-watch-uploader"');
		expect(markup).not.toContain('aria-label="label.uploader:');
	});

	test("shows the native surface only while its full rect is inside the video region", () => {
		const viewport = { top: 68, right: 1200, bottom: 800, left: 280 };

		expect(
			isYouTubeNativeSurfaceRectVisible(
				{ top: 96, right: 1160, bottom: 720, left: 320 },
				viewport,
			),
		).toBe(true);
		expect(
			isYouTubeNativeSurfaceRectVisible(
				{ top: 40, right: 1160, bottom: 664, left: 320 },
				viewport,
			),
		).toBe(false);
		expect(
			isYouTubeNativeSurfaceRectVisible(
				{ top: 96, right: 1160, bottom: 824, left: 320 },
				viewport,
			),
		).toBe(false);
	});
});
