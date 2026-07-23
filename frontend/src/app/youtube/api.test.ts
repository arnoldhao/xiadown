import { describe, expect, test } from "bun:test";

import {
	appendYouTubeWorkspaceBrowsePage,
	appendYouTubeUploaderPage,
	createYouTubeChannelSubscriptionRequest,
	createYouTubeControlRequest,
	createYouTubeVideoDetailsRequest,
	createYouTubeVideoRatingRequest,
	createYouTubeUploaderRequest,
	createYouTubeWorkspaceBrowseRequest,
	createYouTubeWorkspacePlayRequest,
	createYouTubeWorkspacePlayVideoRequest,
  isYouTubePlayerStatusForSession,
  normalizeBrowsePage,
  normalizeRouteId,
	normalizeYouTubeUploaderPage,
  routeTitle,
} from "@/app/youtube/api";

describe("YouTube workspace API normalization", () => {
  test("keeps valid videos and drops malformed entries", () => {
    const page = normalizeBrowsePage(
      {
        routeId: "search",
        title: "Search",
        webUrl: "https://www.youtube.com/results?search_query=test",
        items: [
          {
            videoId: "AbCdEfGh123",
            title: "A video",
            webUrl: "https://www.youtube.com/watch?v=AbCdEfGh123",
          },
          {
            itemKind: "playlist",
            videoId: "",
            playlistId: "PLworkspaceCollection123",
            title: "A playlist",
            webUrl:
              "https://www.youtube.com/playlist?list=PLworkspaceCollection123",
          },
		  {
			videoId: "AbCdEfGh123",
			title: "Duplicate video",
			webUrl: "https://www.youtube.com/watch?v=AbCdEfGh123",
		  },
          { videoId: "short", title: "Invalid" },
          null,
        ],
		continuation: "  search-page-2  ",
      },
      "home",
    );

    expect(page.routeId).toBe("search");
    expect(page.items).toHaveLength(2);
    expect(page.items[0]?.videoId).toBe("AbCdEfGh123");
    expect(page.items[1]?.playlistId).toBe("PLworkspaceCollection123");
		expect(page.continuation).toBe("search-page-2");
  });

	test("normalizes empty continuation values away", () => {
		const page = normalizeBrowsePage(
			{
				routeId: "home",
				items: [],
				continuation: "   ",
			},
			"home",
		);

		expect(page.continuation).toBeUndefined();
	});

	test("builds initial and continuation requests with stable browse context", () => {
		expect(
			createYouTubeWorkspaceBrowseRequest(
				"search",
				" workspace mix ",
				undefined,
				{ locale: " zh-CN " },
			),
		).toEqual({
			routeId: "search",
			query: "workspace mix",
			playlistId: undefined,
			locale: "zh-CN",
		});
		expect(
			createYouTubeWorkspaceBrowseRequest(
				"search",
				"workspace mix",
				" PLworkspaceCollection123 ",
				{ continuation: " playlist-page-2 ", locale: "en" },
			),
		).toEqual({
			routeId: "playlists",
			query: undefined,
			playlistId: "PLworkspaceCollection123",
			continuation: "playlist-page-2",
			locale: "en",
		});
	});

	test("atomically appends a continuation and deduplicates videos and playlists", () => {
		const current = normalizeBrowsePage(
			{
				routeId: "home",
				title: "Home",
				webUrl: "https://www.youtube.com/",
				items: [
					{
						videoId: "AbCdEfGh123",
						title: "First",
						webUrl: "https://www.youtube.com/watch?v=AbCdEfGh123",
					},
					{
						itemKind: "playlist",
						videoId: "",
						playlistId: "PLworkspaceCollection123",
						title: "First playlist",
						webUrl: "https://www.youtube.com/playlist?list=PLworkspaceCollection123",
					},
				],
				continuation: "page-2",
			},
			"home",
		);
		const incoming = normalizeBrowsePage(
			{
				routeId: "home",
				items: [
					{
						videoId: "AbCdEfGh123",
						title: "Duplicate video",
						webUrl: "https://www.youtube.com/watch?v=AbCdEfGh123",
					},
					{
						videoId: "XyZaBcDe456",
						title: "Second",
						webUrl: "https://www.youtube.com/watch?v=XyZaBcDe456",
					},
					{
						itemKind: "playlist",
						videoId: "",
						playlistId: "PLworkspaceCollection123",
						title: "Duplicate playlist",
						webUrl: "https://www.youtube.com/playlist?list=PLworkspaceCollection123",
					},
				],
				continuation: "page-3",
			},
			"home",
		);

		const merged = appendYouTubeWorkspaceBrowsePage(
			current,
			incoming,
			" page-2 ",
		);

		expect(merged?.items.map((item) => item.videoId || item.playlistId)).toEqual([
			"AbCdEfGh123",
			"PLworkspaceCollection123",
			"XyZaBcDe456",
		]);
		expect(merged?.continuation).toBe("page-3");
	});

	test("ignores stale pages and terminates a repeated continuation", () => {
		const current = normalizeBrowsePage(
			{
				routeId: "home",
				items: [],
				continuation: "page-2",
			},
			"home",
		);
		const repeated = normalizeBrowsePage(
			{
				routeId: "home",
				items: [],
				continuation: "page-2",
			},
			"home",
		);
		const wrongRoute = normalizeBrowsePage(
			{
				routeId: "search",
				items: [],
				continuation: "search-page-2",
			},
			"search",
		);

		expect(
			appendYouTubeWorkspaceBrowsePage(current, repeated, "stale-page"),
		).toBe(current);
		expect(
			appendYouTubeWorkspaceBrowsePage(current, wrongRoute, "page-2"),
		).toBe(current);
		expect(
			appendYouTubeWorkspaceBrowsePage(current, repeated, "page-2")
				?.continuation,
		).toBeUndefined();
	});

  test("falls back to a known route and human title", () => {
    expect(normalizeRouteId("unknown", "home")).toBe("home");
    expect(routeTitle("liked-videos")).toBe("Liked videos");
    expect(routeTitle("subscriptions")).toBe("Subscriptions");
  });

  test("accepts events only for the active YouTube provider session", () => {
    expect(
      isYouTubePlayerStatusForSession(
        { provider: "youtube", sessionId: "video-2", state: "playing" },
        "video-2",
      ),
    ).toBe(true);
    expect(
      isYouTubePlayerStatusForSession(
        { provider: "stream", sessionId: "video-2", state: "playing" },
        "video-2",
      ),
    ).toBe(false);
    expect(
      isYouTubePlayerStatusForSession(
        { provider: "youtube", sessionId: "stale-video", state: "paused" },
        "video-2",
      ),
    ).toBe(false);
  });

	test("builds provider and session scoped player control requests", () => {
		expect(
			createYouTubeControlRequest(
				" youtube-session ",
				"select-caption",
				"en-US",
			),
		).toEqual({
			provider: "youtube",
			sessionId: "youtube-session",
			command: "select-caption",
			value: "en-US",
		});
		expect(
			createYouTubeControlRequest("video-2", "set-volume"),
		).toEqual({
			provider: "youtube",
			sessionId: "video-2",
			command: "set-volume",
			value: "",
		});
		expect(
			createYouTubeControlRequest(
				"video-2",
				"select-playback-rate",
				"1.5",
			),
		).toEqual({
			provider: "youtube",
			sessionId: "video-2",
			command: "select-playback-rate",
			value: "1.5",
		});
	});

	test("builds normalized video details requests", () => {
		expect(
			createYouTubeVideoDetailsRequest(" AbCdEfGh123 ", " zh-Hant-TW "),
		).toEqual({
			videoId: "AbCdEfGh123",
			locale: "zh-Hant-TW",
		});
		expect(createYouTubeVideoDetailsRequest("ZyXwVuTs987", "   ")).toEqual({
			videoId: "ZyXwVuTs987",
		});
	});

	test("builds normalized App Session video rating requests", () => {
		expect(
			createYouTubeVideoRatingRequest(" AbCdEfGh123 ", "like"),
		).toEqual({ videoId: "AbCdEfGh123", rating: "like" });
		expect(
			createYouTubeVideoRatingRequest("AbCdEfGh123", "none"),
		).toEqual({ videoId: "AbCdEfGh123", rating: "none" });
	});

	test("builds normalized App Session channel subscription requests", () => {
		expect(
			createYouTubeChannelSubscriptionRequest(
				" UCabcdefghijklmnopqrstuv ",
				true,
			),
		).toEqual({
			channelId: "UCabcdefghijklmnopqrstuv",
			subscribed: true,
		});
	});

	test("correlates cancellable YouTube playback requests", () => {
		const video = {
			videoId: "AbCdEfGh123",
			title: "Workspace video",
			webUrl: "https://www.youtube.com/watch?v=AbCdEfGh123",
		};
		expect(
			createYouTubeWorkspacePlayVideoRequest(
				video,
				1_700_000_000_001,
				" zh-CN ",
			),
		).toEqual({
			requestId: 1_700_000_000_001,
			video,
			locale: "zh-CN",
		});
		expect(createYouTubeWorkspacePlayRequest(1_700_000_000_001)).toEqual({
			requestId: 1_700_000_000_001,
		});
	});

	test("normalizes uploader metadata and keeps only valid videos", () => {
		const page = normalizeYouTubeUploaderPage(
			{
				channelId: " UCabcdefghijklmnopqrstuv ",
				name: " Workspace Creator ",
				handle: " @workspace ",
				description: " Channel description ",
				avatarUrl: " https://yt3.example/avatar.jpg ",
				bannerUrl: " https://yt3.example/banner.jpg ",
				subscriberCount: 34900,
				subscriberLabel: " 34.9K subscribers ",
				videoCountLabel: " 173 videos ",
				isSubscribed: true,
				webUrl: " https://www.youtube.com/@workspace ",
				videos: [
					{
						videoId: "LatestVid01",
						title: "Latest upload",
						webUrl: "https://www.youtube.com/watch?v=LatestVid01",
					},
					{
						itemKind: "playlist",
						videoId: "",
						playlistId: "PLworkspaceCollection123",
						title: "Playlist",
						webUrl: "https://www.youtube.com/playlist?list=PLworkspaceCollection123",
					},
				],
				continuation: " page-2 ",
			},
			"fallback",
		);

		expect(page).toMatchObject({
			channelId: "UCabcdefghijklmnopqrstuv",
			name: "Workspace Creator",
			handle: "@workspace",
			description: "Channel description",
			subscriberCount: 34900,
			subscriberLabel: "34.9K subscribers",
			videoCountLabel: "173 videos",
			isSubscribed: true,
			continuation: "page-2",
		});
		expect(page.videos.map((video) => video.videoId)).toEqual([
			"LatestVid01",
		]);
	});

	test("builds uploader requests and appends matching continuations", () => {
		expect(
			createYouTubeUploaderRequest(" UCabcdefghijklmnopqrstuv ", {
				continuation: " page-2 ",
				locale: " zh-Hant-TW ",
			}),
		).toEqual({
			channelId: "UCabcdefghijklmnopqrstuv",
			continuation: "page-2",
			locale: "zh-Hant-TW",
		});

		const current = normalizeYouTubeUploaderPage(
			{
				channelId: "UCabcdefghijklmnopqrstuv",
				name: "Workspace Creator",
				videos: [{
					videoId: "LatestVid01",
					title: "Latest",
					webUrl: "https://www.youtube.com/watch?v=LatestVid01",
				}],
				continuation: "page-2",
			},
			"UCabcdefghijklmnopqrstuv",
		);
		const incoming = normalizeYouTubeUploaderPage(
			{
				channelId: "UCabcdefghijklmnopqrstuv",
				videos: [
					{
						videoId: "LatestVid01",
						title: "Duplicate",
						webUrl: "https://www.youtube.com/watch?v=LatestVid01",
					},
					{
						videoId: "MoreVideo02",
						title: "More",
						webUrl: "https://www.youtube.com/watch?v=MoreVideo02",
					},
				],
				continuation: "page-3",
			},
			"UCabcdefghijklmnopqrstuv",
		);
		const merged = appendYouTubeUploaderPage(current, incoming, "page-2");
		expect(merged?.name).toBe("Workspace Creator");
		expect(merged?.videos.map((video) => video.videoId)).toEqual([
			"LatestVid01",
			"MoreVideo02",
		]);
		expect(merged?.continuation).toBe("page-3");
		expect(appendYouTubeUploaderPage(current, incoming, "stale")).toBe(current);
	});
});
