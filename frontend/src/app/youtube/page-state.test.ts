import { describe, expect, test } from "bun:test";

import {
  consumeYouTubeExternalCommand,
  createYouTubeWorkspaceBrowseRequest,
  resolveYouTubeExternalQueueIndex,
  shouldCommitYouTubeVideoOpen,
} from "@/app/youtube/api";
import {
  backFromYouTubePrimaryDetail,
  createYouTubePrimaryDetail,
  formatYouTubePublishedLabel,
  formatYouTubeViewCount,
  moveYouTubePrimaryWatchInQueue,
  openYouTubePrimaryPlaylist,
  openYouTubePrimaryWatch,
  resetYouTubePrimaryDetailForRoute,
  resolveYouTubePreferredPlaybackTitle,
  resolveYouTubeVolumeCapability,
  resolveYouTubeWorkspaceErrorMessage,
  rollbackYouTubeOptimisticMute,
  submitYouTubeSearchQuery,
  updateYouTubeSubmittedQueryOnInput,
} from "@/app/youtube/page-state";
import type { YouTubeWorkspaceVideo } from "@/app/youtube/types";

function video(
  videoId: string,
  title = `Video ${videoId}`,
): YouTubeWorkspaceVideo {
  return {
    itemKind: "video",
    videoId,
    title,
    webUrl: `https://www.youtube.com/watch?v=${videoId}`,
  };
}

const playlist = {
  playlistId: "PLworkspaceCollection123",
  title: "Workspace collection",
  webUrl: "https://www.youtube.com/playlist?list=PLworkspaceCollection123",
} as const;

const youtubeErrors = {
  unavailable: "youtube.unavailable",
  networkUnavailable: "youtube.network_unavailable",
  requestTimedOut: "youtube.request_timed_out",
  authenticationRequired: "youtube.authentication_required",
  sessionExpired: "youtube.session_expired",
  playerUnavailable: "youtube.player_unavailable",
  controlUnavailable: "youtube.control_unavailable",
} as const;

describe("YouTube workspace page state", () => {
  test("keeps a localized browse title over canonical player metadata", () => {
    expect(
      resolveYouTubePreferredPlaybackTitle(
        "AbCdEfGh123",
        "Localized history title",
        "Canonical source title",
      ),
    ).toBe("Localized history title");
    expect(
      resolveYouTubePreferredPlaybackTitle(
        "AbCdEfGh123",
        "AbCdEfGh123",
        "Canonical source title",
      ),
    ).toBe("Canonical source title");
  });

  test("keeps bridge volume available across transient YouTube capability snapshots", () => {
    const descriptor = { sessionId: "youtube-session" };

    expect(
      resolveYouTubeVolumeCapability(descriptor, { available: true }),
    ).toBe(true);
    expect(
      resolveYouTubeVolumeCapability(descriptor, {
        available: undefined,
        controls: {
          like: false,
          dislike: false,
          captions: false,
          audioTrack: false,
          quality: false,
          volume: false,
        },
      }),
    ).toBe(true);
    expect(resolveYouTubeVolumeCapability(descriptor, {})).toBe(false);
    expect(
      resolveYouTubeVolumeCapability(descriptor, { available: false }),
    ).toBe(false);
    expect(
      resolveYouTubeVolumeCapability({ sessionId: "  " }, { available: true }),
    ).toBe(false);
  });

  test("returns Search to its empty state as soon as a submitted query is cleared", () => {
    const submitted = submitYouTubeSearchQuery("  workspace mix  ");
    expect(submitted).toBe("workspace mix");
    expect(updateYouTubeSubmittedQueryOnInput(submitted, "workspace mi")).toBe(
      submitted,
    );
    expect(updateYouTubeSubmittedQueryOnInput(submitted, "   ")).toBe("");
  });

  test("formats view counts with the app locale rather than the host locale", () => {
    expect(formatYouTubeViewCount(1_250_000, "en")).toBe("1.3M");
    expect(formatYouTubeViewCount(1_250_000, "zh-CN")).toBe(
      `125${String.fromCodePoint(0x4e07)}`,
    );
  });

  test("formats canonical published timestamps without shifting their calendar day", () => {
    expect(
      formatYouTubePublishedLabel("2026-01-07T22:01:12-08:00", "en"),
    ).toBe("Jan 7, 2026");
    expect(formatYouTubePublishedLabel("2026-01-07", "zh-CN")).toBe(
      `2026${String.fromCodePoint(0x5e74)}1${String.fromCodePoint(0x6708)}7${String.fromCodePoint(0x65e5)}`,
    );
    expect(formatYouTubePublishedLabel("2 days ago", "en")).toBe(
      "2 days ago",
    );
  });

  test("localizes YouTube failures and silently classifies cancellations", () => {
    expect(
      resolveYouTubeWorkspaceErrorMessage(
        new Error("youtube workspace: execute javascript: Go backend failed"),
        youtubeErrors,
      ),
    ).toBe(youtubeErrors.unavailable);
    expect(
      resolveYouTubeWorkspaceErrorMessage(
        new Error("youtube network unavailable: no such host"),
        youtubeErrors,
      ),
    ).toBe(youtubeErrors.networkUnavailable);
    expect(
      resolveYouTubeWorkspaceErrorMessage(
        new Error("youtube request timed out"),
        youtubeErrors,
      ),
    ).toBe(youtubeErrors.requestTimedOut);
    expect(
      resolveYouTubeWorkspaceErrorMessage(
        new Error("youtube is not authenticated: session not found"),
        youtubeErrors,
      ),
    ).toBe(youtubeErrors.authenticationRequired);
    expect(
      resolveYouTubeWorkspaceErrorMessage(
        { errorMessage: "youtube authentication expired" },
        youtubeErrors,
      ),
    ).toBe(youtubeErrors.sessionExpired);
    expect(
      resolveYouTubeWorkspaceErrorMessage(
        "opaque native player failure",
        youtubeErrors,
        "playback",
      ),
    ).toBe(youtubeErrors.playerUnavailable);
    expect(
      resolveYouTubeWorkspaceErrorMessage(
        "opaque native control failure",
        youtubeErrors,
        "control",
      ),
    ).toBe(youtubeErrors.controlUnavailable);
    expect(
      resolveYouTubeWorkspaceErrorMessage(
        new Error("context canceled"),
        youtubeErrors,
      ),
    ).toBe("");
    expect(
      resolveYouTubeWorkspaceErrorMessage(
        new DOMException("request aborted", "AbortError"),
        youtubeErrors,
      ),
    ).toBe("");
  });

  test("rolls back only the mute value owned by the rejected request", () => {
    expect(rollbackYouTubeOptimisticMute(true, true, false)).toBe(false);
    expect(rollbackYouTubeOptimisticMute(false, true, false)).toBe(false);
  });

  test("consumes each external previous or next command only once", () => {
    const command = { id: 7, command: "next" as const };
    const first = consumeYouTubeExternalCommand(command, null, 1, 4);
    expect(first).toEqual({
	  clearPlayback: false,
	  handledID: 7,
	  targetIndex: 2,
	});

    const duplicate = consumeYouTubeExternalCommand(
      command,
      first.handledID,
      2,
      4,
    );
    expect(duplicate).toEqual({
	  clearPlayback: false,
	  handledID: 7,
	  targetIndex: -1,
	});
    expect(resolveYouTubeExternalQueueIndex("previous", 0, 4)).toBe(-1);
    expect(resolveYouTubeExternalQueueIndex("next", 3, 4)).toBe(-1);
  });

	test("consumes Stop once without navigating the queue", () => {
		const command = { id: 8, command: "stop" as const };
		const first = consumeYouTubeExternalCommand(command, 7, 1, 4);
		expect(first).toEqual({
			clearPlayback: true,
			handledID: 8,
			targetIndex: -1,
		});
		expect(consumeYouTubeExternalCommand(command, 8, 1, 4)).toEqual({
			clearPlayback: false,
			handledID: 8,
			targetIndex: -1,
		});
		expect(resolveYouTubeExternalQueueIndex("stop", 1, 4)).toBe(-1);
	});

	test("commits global queue navigation while the YouTube workspace is hidden", () => {
		expect(shouldCommitYouTubeVideoOpen(true, false, true, true)).toBe(true);
		expect(shouldCommitYouTubeVideoOpen(true, false, false, true)).toBe(true);
		expect(shouldCommitYouTubeVideoOpen(true, false, true, false)).toBe(false);
		expect(shouldCommitYouTubeVideoOpen(false, true, true, true)).toBe(false);
	});

  test("drills into a playlist internally and restores the route request on back", () => {
    expect(
      createYouTubeWorkspaceBrowseRequest(
        "search",
        "workspace mix",
        " PLworkspaceCollection123 ",
      ),
    ).toEqual({
      routeId: "playlists",
      query: undefined,
      playlistId: "PLworkspaceCollection123",
    });
    expect(
      createYouTubeWorkspaceBrowseRequest("search", " workspace mix "),
    ).toEqual({
      routeId: "search",
      query: "workspace mix",
      playlistId: undefined,
    });
  });

  test("opens a Watch page in primary content and returns to its browse root", () => {
    const browse = createYouTubePrimaryDetail("home");
    const selected = video("AbCdEfGh123", "Primary Watch video");
    const watch = openYouTubePrimaryWatch(browse, selected);

    expect(watch).toEqual({
      kind: "watch",
      routeId: "home",
      video: selected,
      returnTarget: { kind: "browse" },
    });
    expect(backFromYouTubePrimaryDetail(watch)).toEqual(browse);
    expect(backFromYouTubePrimaryDetail(browse)).toBe(browse);
  });

  test("returns from Watch to the playlist it was opened from", () => {
    const browse = createYouTubePrimaryDetail("playlists");
    const playlistDetail = openYouTubePrimaryPlaylist(browse, playlist);
    const selected = video("XyZaBcDe456");
    const watch = openYouTubePrimaryWatch(playlistDetail, selected);

    expect(watch).toEqual({
      kind: "watch",
      routeId: "playlists",
      video: selected,
      returnTarget: { kind: "playlist", playlist },
    });
    expect(backFromYouTubePrimaryDetail(watch)).toEqual(playlistDetail);
    expect(backFromYouTubePrimaryDetail(playlistDetail)).toEqual(browse);
  });

  test("keeps the original return target across Watch-to-Watch navigation", () => {
    const playlistDetail = openYouTubePrimaryPlaylist(
      createYouTubePrimaryDetail("playlists"),
      playlist,
    );
    const firstWatch = openYouTubePrimaryWatch(
      playlistDetail,
      video("AbCdEfGh123"),
    );
    const relatedWatch = openYouTubePrimaryWatch(
      firstWatch,
      video("XyZaBcDe456"),
    );

    expect(relatedWatch.kind).toBe("watch");
    if (relatedWatch.kind !== "watch") {
      throw new Error("expected a Watch detail");
    }
    expect(relatedWatch.returnTarget).toEqual({
      kind: "playlist",
      playlist,
    });
    expect(backFromYouTubePrimaryDetail(relatedWatch)).toEqual(playlistDetail);
  });

  test("resets drill-in state only when the sidebar route changes", () => {
    const watch = openYouTubePrimaryWatch(
      createYouTubePrimaryDetail("home"),
      video("AbCdEfGh123"),
    );

    expect(resetYouTubePrimaryDetailForRoute(watch, "home")).toBe(watch);
    expect(resetYouTubePrimaryDetailForRoute(watch, "history")).toEqual({
      kind: "browse",
      routeId: "history",
    });
  });

  test("moves previous and next while preserving the Watch return target", () => {
    const first = video("AbCdEfGh123");
    const second = video("XyZaBcDe456");
    const third = video("LmNoPqRs789");
    const queue: YouTubeWorkspaceVideo[] = [
      first,
      {
        itemKind: "playlist",
        videoId: "",
        playlistId: playlist.playlistId,
        title: playlist.title,
        webUrl: playlist.webUrl,
      },
      second,
      video("too-short"),
      third,
    ];
    const playlistDetail = openYouTubePrimaryPlaylist(
      createYouTubePrimaryDetail("playlists"),
      playlist,
    );
    const firstWatch = openYouTubePrimaryWatch(playlistDetail, first);

    const next = moveYouTubePrimaryWatchInQueue(
      firstWatch,
      queue,
      0,
      "next",
    );
    expect(next.currentIndex).toBe(2);
    expect(next.detail).toEqual({
      kind: "watch",
      routeId: "playlists",
      video: second,
      returnTarget: { kind: "playlist", playlist },
    });

    const previous = moveYouTubePrimaryWatchInQueue(
      next.detail,
      queue,
      next.currentIndex,
      "previous",
    );
    expect(previous.currentIndex).toBe(0);
    expect(previous.detail).toEqual(firstWatch);
  });

  test("repairs a stale queue index and leaves boundaries or browse state alone", () => {
    const first = video("AbCdEfGh123");
    const second = video("XyZaBcDe456");
    const third = video("LmNoPqRs789");
    const queue = [first, second, third];
    const secondWatch = openYouTubePrimaryWatch(
      createYouTubePrimaryDetail("home"),
      second,
    );

    const repairedNext = moveYouTubePrimaryWatchInQueue(
      secondWatch,
      queue,
      0,
      "next",
    );
    expect(repairedNext.currentIndex).toBe(2);
    expect(
      repairedNext.detail.kind === "watch"
        ? repairedNext.detail.video.videoId
        : "",
    ).toBe(third.videoId);

    const atBoundary = moveYouTubePrimaryWatchInQueue(
      repairedNext.detail,
      queue,
      repairedNext.currentIndex,
      "next",
    );
    expect(atBoundary.detail).toBe(repairedNext.detail);
    expect(atBoundary.currentIndex).toBe(2);

    const browse = createYouTubePrimaryDetail("home");
    expect(
      moveYouTubePrimaryWatchInQueue(browse, queue, 1, "previous"),
    ).toEqual({ detail: browse, currentIndex: 1 });
  });

  test("rejects malformed Watch and playlist targets without corrupting state", () => {
    const browse = createYouTubePrimaryDetail("home");

    expect(openYouTubePrimaryWatch(browse, video("invalid"))).toBe(browse);
    expect(
      openYouTubePrimaryWatch(browse, {
        ...video("AbCdEfGh123"),
        itemKind: "playlist",
      }),
    ).toBe(browse);
    expect(
      openYouTubePrimaryPlaylist(browse, {
        ...playlist,
        playlistId: "   ",
      }),
    ).toBe(browse);
  });
});
