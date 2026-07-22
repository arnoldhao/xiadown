import { describe, expect, test } from "bun:test";

import type { YouTubeWorkspaceVideo } from "@/app/youtube/types";
import { createYouTubeBrowseUploaderTarget } from "@/app/youtube/uploader-navigation";

const video: YouTubeWorkspaceVideo = {
	itemKind: "video",
	videoId: "BrowseVideo01",
	title: "Browse video",
	channel: "Workspace Creator",
	channelId: "UCabcdefghijklmnopqrstuv",
	viewCount: 1234,
	webUrl: "https://www.youtube.com/watch?v=BrowseVideo01",
};

describe("YouTube browse uploader navigation", () => {
	test("builds an internal uploader target only for a real YouTube channel", () => {
		expect(
			createYouTubeBrowseUploaderTarget(video, "YouTube", true),
		).toEqual({
			channelId: "UCabcdefghijklmnopqrstuv",
			name: "Workspace Creator",
			subscribed: true,
			videoId: "BrowseVideo01",
		});
		expect(
			createYouTubeBrowseUploaderTarget(
				{ ...video, channelId: "@workspace" },
				"YouTube",
			),
		).toBeNull();
	});

	test("wires separate video and uploader actions into the shared browse card", async () => {
		const source = await Bun.file(
			new URL("./YouTubeWorkspacePage.tsx", import.meta.url),
		).text();

		expect(source).toContain("onOpenUploader={youtubeChannelURL(video.channelId)");
		expect(source).toContain("? () => openBrowseUploader(video)");
		expect(source).toContain('data-youtube-video-action="open"');
		expect(source).toContain('data-youtube-uploader-action="open"');
		expect(source).toContain("event.stopPropagation();");
		expect(source).toContain("setUploaderTarget(target);");
	});
});
