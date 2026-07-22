import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import { YouTubeUpNextCompanion } from "@/app/youtube/YouTubeUpNextCompanion";
import type { YouTubeWorkspaceVideo } from "@/app/youtube/types";

const previousVideo: YouTubeWorkspaceVideo = {
	itemKind: "video",
	videoId: "PrevVideo01",
	title: "Previous video",
	channel: "Previous creator",
	webUrl: "https://www.youtube.com/watch?v=PrevVideo01",
};

const currentVideo: YouTubeWorkspaceVideo = {
	itemKind: "video",
	videoId: "AbCdEfGh123",
	title: "Current video",
	channel: "Current creator",
	webUrl: "https://www.youtube.com/watch?v=AbCdEfGh123",
};

const nextVideo: YouTubeWorkspaceVideo = {
	itemKind: "video",
	videoId: "ZyXwVuTs987",
	title: "Next video",
	channel: "Next creator",
	thumbnailUrl: "https://i.ytimg.com/vi/ZyXwVuTs987/hqdefault.jpg",
	durationLabel: "3:42",
	viewCount: 1_250_000,
	webUrl: "https://www.youtube.com/watch?v=ZyXwVuTs987",
};

const nonVideoItems: YouTubeWorkspaceVideo[] = [
	{
		itemKind: "playlist",
		videoId: "",
		playlistId: "PLworkspace123",
		title: "Playlist row",
		webUrl: "https://www.youtube.com/playlist?list=PLworkspace123",
	},
	{
		itemKind: "video",
		videoId: "short-id",
		title: "Invalid video row",
		webUrl: "https://www.youtube.com/watch?v=short-id",
	},
];

function renderCompanion(
	queue: YouTubeWorkspaceVideo[],
	options: { currentIndex?: number; openingVideoId?: string } = {},
) {
	return renderToStaticMarkup(
		<YouTubeUpNextCompanion
			queue={queue}
			currentIndex={options.currentIndex ?? 0}
			currentVideoId={currentVideo.videoId}
			openingVideoId={options.openingVideoId ?? ""}
			locale="en"
			title="Up Next"
			emptyLabel="Nothing queued"
			fallbackChannel="YouTube"
			onOpenVideo={() => {}}
		/>,
	);
}

describe("YouTubeUpNextCompanion", () => {
	test("shows only playable videos after the current queue item", () => {
		const markup = renderCompanion(
			[previousVideo, currentVideo, ...nonVideoItems, nextVideo],
			{ currentIndex: 99, openingVideoId: nextVideo.videoId },
		);

		expect(markup).toContain('data-youtube-companion="up-next"');
		expect(markup).toContain('data-companion-scroll-owner="youtube-up-next"');
		expect(markup).toContain('aria-label="Up Next"');
		expect(markup).not.toContain("youtube-up-next-companion-header");
		expect(markup).not.toContain("<h2");
		expect(markup).toContain("Next video");
		expect(markup).toContain("Next creator");
		expect(markup).toContain("1.3M");
		expect(markup).toContain("3:42");
		expect(markup).toContain("disabled");
		expect(markup).toContain("app-motion-spin");
		expect(markup).not.toContain("Previous video");
		expect(markup).not.toContain("Current video");
		expect(markup).not.toContain("Playlist row");
		expect(markup).not.toContain("Invalid video row");
	});

	test("renders the YouTube-specific empty state when nothing follows", () => {
		const markup = renderCompanion([currentVideo]);

		expect(markup).toContain("youtube-up-next-companion-empty");
		expect(markup).toContain("Nothing queued");
		expect(markup).not.toContain("youtube-up-next-row");
	});

	test("wires playable rows back to the workspace video opener", async () => {
		const source = await Bun.file(
			new URL("./YouTubeUpNextCompanion.tsx", import.meta.url),
		).text();

		expect(source).toContain("onClick={() => onOpenVideo(video)}");
	});

	test("uses the shared companion title and playback footer regions", async () => {
		const source = await Bun.file(
			new URL("../main/MainApp.tsx", import.meta.url),
		).text();

		expect(source).toContain(
			'<div className="app-workspace-companion__title">',
		);
		expect(source).toContain("{companionTitle}");
		expect(source).toContain(
			'companion.destination?.id === "youtube-up-next") &&',
		);
		expect(source).toContain("<PlayerCompanionFooter");
	});

	test("lets the outer companion host provide one continuous glass surface", async () => {
		const [css, appearanceCSS, workspaceAppearanceCSS, companionSource] = await Promise.all([
			Bun.file(new URL("./youtube-workspace.css", import.meta.url)).text(),
			Bun.file(new URL("../../shared/styles/dream/youtube.css", import.meta.url)).text(),
			Bun.file(new URL("../../shared/styles/dream/workspace.css", import.meta.url)).text(),
			Bun.file(new URL("../workspace/CompanionPanel.tsx", import.meta.url)).text(),
		]);
		const rule = appearanceCSS.match(
			/\.youtube-up-next-companion\s*\{([^}]*)\}/s,
		);

		expect(rule?.[1]).toContain("background: transparent");
		expect(rule?.[1]).not.toContain("linear-gradient");
		expect(workspaceAppearanceCSS).toMatch(
			/\.app-main-shell\[data-surface-style="glass"\][\s\S]*?\.app-workspace-companion\[data-glass-host="true"\],[\s\S]*?\.app-workspace-companion\[data-presentation="overlay"\]\[data-glass-host="true"\]\s*\{[^}]*background:\s*transparent/s,
		);
		expect(companionSource).toContain('data-glass-role="companion"');
		expect(`${css}\n${appearanceCSS}`).not.toMatch(
			/data-youtube-workspace-video-active="true"\][\s\S]*?\.app-workspace-companion\[data-destination="youtube-up-next"\]\s*\{/s,
		);
	});
});
