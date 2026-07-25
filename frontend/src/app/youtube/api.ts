import {
  LISTEN_LIVE_PLAYER_EVENT,
  LISTEN_LIVE_PLAYER_SERVICE,
} from "@/app/main/listen/catalog";
import type {
	YouTubeChannelSubscriptionRequest,
  YouTubeUploaderPageData,
  YouTubeUploaderRequest,
  YouTubePlaybackDescriptor,
  YouTubePlayerStatus,
  YouTubeVideoDetails,
  YouTubeVideoDetailsRequest,
  YouTubeVideoRating,
  YouTubeVideoRatingRequest,
  YouTubeWorkspaceBrowseRequest,
  YouTubeWorkspaceBrowsePage,
	YouTubeWorkspaceExternalCommand,
	YouTubeWorkspacePlayRequest,
	YouTubeWorkspacePlayVideoRequest,
  YouTubeWorkspaceRouteId,
  YouTubeWorkspaceVideo,
} from "@/app/youtube/types";

const YOUTUBE_WORKSPACE_SERVICE =
  "xiadown/internal/presentation/wails.YouTubeWorkspaceHandler";
const YOUTUBE_EMBEDDED_FULLSCREEN_CHANGE =
  "embedded-video-fullscreen-change";

async function callService<T>(service: string, method: string, payload?: unknown) {
  const { Call } = await import("@wailsio/runtime");
  const result =
    payload === undefined
      ? await Call.ByName(`${service}.${method}`)
      : await Call.ByName(`${service}.${method}`, payload);
  return result as T;
}

export async function browseYouTubeWorkspace(
  request: YouTubeWorkspaceBrowseRequest,
) {
  const raw = await callService<unknown>(
    YOUTUBE_WORKSPACE_SERVICE,
    "Browse",
    request,
  );
  return normalizeBrowsePage(raw, request.routeId);
}

export function forceRefreshYouTubeWorkspace() {
	return callService<void>(YOUTUBE_WORKSPACE_SERVICE, "ForceRefresh");
}

export async function playYouTubeWorkspaceVideo(
	video: YouTubeWorkspaceVideo,
	requestId: number,
	locale?: string,
) {
  return callService<YouTubePlaybackDescriptor>(
    YOUTUBE_WORKSPACE_SERVICE,
	"PlayVideoRequest",
	createYouTubeWorkspacePlayVideoRequest(video, requestId, locale),
  );
}

export function acceptYouTubeWorkspacePlay(requestId: number) {
	return callService<void>(
		YOUTUBE_WORKSPACE_SERVICE,
		"AcceptPlay",
		createYouTubeWorkspacePlayRequest(requestId),
	);
}

export function cancelYouTubeWorkspacePlay(requestId: number) {
	return callService<void>(
		YOUTUBE_WORKSPACE_SERVICE,
		"CancelPlay",
		createYouTubeWorkspacePlayRequest(requestId),
	);
}

export function createYouTubeWorkspacePlayVideoRequest(
	video: YouTubeWorkspaceVideo,
	requestId: number,
	locale?: string,
): YouTubeWorkspacePlayVideoRequest {
	const normalizedLocale = locale?.trim() || undefined;
	return {
		requestId,
		video,
		...(normalizedLocale ? { locale: normalizedLocale } : {}),
	};
}

export function createYouTubeWorkspacePlayRequest(
	requestId: number,
): YouTubeWorkspacePlayRequest {
	return { requestId };
}

export function createYouTubeVideoDetailsRequest(
	videoId: string,
	locale?: string,
): YouTubeVideoDetailsRequest {
	const normalizedLocale = locale?.trim() || undefined;
	return {
		videoId: videoId.trim(),
		...(normalizedLocale ? { locale: normalizedLocale } : {}),
	};
}

export async function getYouTubeWorkspaceVideoDetails(
	videoId: string,
	locale?: string,
) {
	return callService<YouTubeVideoDetails>(
		YOUTUBE_WORKSPACE_SERVICE,
		"VideoDetails",
		createYouTubeVideoDetailsRequest(videoId, locale),
	);
}

export function createYouTubeVideoRatingRequest(
	videoId: string,
	rating: YouTubeVideoRating,
): YouTubeVideoRatingRequest {
	return { videoId: videoId.trim(), rating };
}

export function rateYouTubeWorkspaceVideo(
	videoId: string,
	rating: YouTubeVideoRating,
) {
	return callService<void>(
		YOUTUBE_WORKSPACE_SERVICE,
		"RateVideo",
		createYouTubeVideoRatingRequest(videoId, rating),
	);
}

export function createYouTubeChannelSubscriptionRequest(
	channelId: string,
	subscribed: boolean,
): YouTubeChannelSubscriptionRequest {
	return { channelId: channelId.trim(), subscribed };
}

export function setYouTubeWorkspaceChannelSubscription(
	channelId: string,
	subscribed: boolean,
) {
	return callService<void>(
		YOUTUBE_WORKSPACE_SERVICE,
		"SetChannelSubscription",
		createYouTubeChannelSubscriptionRequest(channelId, subscribed),
	);
}

export function createYouTubeUploaderRequest(
	channelId: string,
	options: { continuation?: string; locale?: string } = {},
): YouTubeUploaderRequest {
	const continuation = options.continuation?.trim() || undefined;
	const locale = options.locale?.trim() || undefined;
	return {
		channelId: channelId.trim(),
		...(continuation ? { continuation } : {}),
		...(locale ? { locale } : {}),
	};
}

export async function getYouTubeWorkspaceUploader(
	channelId: string,
	options: { continuation?: string; locale?: string } = {},
) {
	const raw = await callService<unknown>(
		YOUTUBE_WORKSPACE_SERVICE,
		"Uploader",
		createYouTubeUploaderRequest(channelId, options),
	);
	return normalizeYouTubeUploaderPage(raw, channelId);
}

export function showYouTubeEmbeddedVideo(rect: {
  x: number;
  y: number;
  width: number;
  height: number;
  centerX: number;
  centerY: number;
  viewportWidth: number;
  viewportHeight: number;
  radius: number;
  interactive: boolean;
  sequence: number;
}) {
  return callService<boolean>(
    LISTEN_LIVE_PLAYER_SERVICE,
    "ShowEmbeddedVideo",
    rect,
  );
}

export function hideYouTubeEmbeddedVideo(sequence: number) {
  return callService<boolean>(
    LISTEN_LIVE_PLAYER_SERVICE,
    "HideEmbeddedVideoForSequence",
    { sequence },
  );
}

export function requestYouTubeEmbeddedVideoFullscreen(sessionId: string) {
  return callService<void>(
    LISTEN_LIVE_PLAYER_SERVICE,
    "RequestEmbeddedVideoFullscreen",
    youtubeSessionRequest(sessionId),
  );
}

export function exitYouTubeEmbeddedVideoFullscreen(sessionId: string) {
  return callService<void>(
    LISTEN_LIVE_PLAYER_SERVICE,
    "ExitEmbeddedVideoFullscreen",
    youtubeSessionRequest(sessionId),
  );
}

export function subscribeYouTubeEmbeddedVideoFullscreen(
  handler: (active: boolean, sessionId: string) => void,
) {
  let active = true;
  let unsubscribe = () => {};
  void import("@wailsio/runtime")
    .then(({ Events }) => {
      if (!active) {
        return;
      }
      unsubscribe = Events.On(LISTEN_LIVE_PLAYER_EVENT, (event: unknown) => {
        const payload = ((event as { data?: unknown })?.data ?? event) as
          | {
              type?: string;
              provider?: string;
              sessionId?: string;
              active?: boolean;
            }
          | undefined;
        if (
          payload?.type === YOUTUBE_EMBEDDED_FULLSCREEN_CHANGE &&
          payload.provider === "youtube"
        ) {
          handler(payload.active === true, payload.sessionId?.trim() ?? "");
        }
      });
    })
    .catch(() => {});
  return () => {
    active = false;
    unsubscribe();
  };
}

function youtubeSessionRequest(sessionId: string) {
  return { provider: "youtube" as const, sessionId: sessionId.trim() };
}

export function createYouTubeControlRequest(
	sessionId: string,
	command:
		| "like"
		| "dislike"
		| "toggle-captions"
		| "select-caption"
		| "select-audio-track"
		| "select-quality"
		| "select-playback-rate"
		| "set-volume",
	value = "",
) {
	return { ...youtubeSessionRequest(sessionId), command, value };
}

export function pauseYouTubeWorkspaceVideo(sessionId: string) {
  return callService<void>(
    LISTEN_LIVE_PLAYER_SERVICE,
    "PauseSession",
    youtubeSessionRequest(sessionId),
  );
}

export function resumeYouTubeWorkspaceVideo(sessionId: string) {
  return callService<void>(
    LISTEN_LIVE_PLAYER_SERVICE,
    "ResumeSession",
    youtubeSessionRequest(sessionId),
  );
}

export function seekYouTubeWorkspaceVideo(sessionId: string, seconds: number) {
  return callService<void>(LISTEN_LIVE_PLAYER_SERVICE, "SeekSession", {
    ...youtubeSessionRequest(sessionId),
    seconds: Math.max(0, seconds),
  });
}

export function setYouTubeWorkspaceVolume(sessionId: string, volume: number, muted: boolean) {
	return callService<void>(LISTEN_LIVE_PLAYER_SERVICE, "ControlSession", {
		...createYouTubeControlRequest(sessionId, "set-volume"),
		volume: Math.max(0, Math.min(1, volume)),
		muted,
	});
}

export function likeYouTubeWorkspaceVideo(sessionId: string) {
	return callService<void>(
		LISTEN_LIVE_PLAYER_SERVICE,
		"ControlSession",
		createYouTubeControlRequest(sessionId, "like"),
	);
}

export function dislikeYouTubeWorkspaceVideo(sessionId: string) {
	return callService<void>(
		LISTEN_LIVE_PLAYER_SERVICE,
		"ControlSession",
		createYouTubeControlRequest(sessionId, "dislike"),
	);
}

export function toggleYouTubeWorkspaceCaptions(sessionId: string) {
	return callService<void>(
		LISTEN_LIVE_PLAYER_SERVICE,
		"ControlSession",
		createYouTubeControlRequest(sessionId, "toggle-captions"),
	);
}

export function selectYouTubeWorkspaceCaption(sessionId: string, captionId: string) {
	return callService<void>(
		LISTEN_LIVE_PLAYER_SERVICE,
		"ControlSession",
		createYouTubeControlRequest(sessionId, "select-caption", captionId),
	);
}

export function selectYouTubeWorkspaceAudioTrack(sessionId: string, audioTrackId: string) {
	return callService<void>(
		LISTEN_LIVE_PLAYER_SERVICE,
		"ControlSession",
		createYouTubeControlRequest(sessionId, "select-audio-track", audioTrackId),
	);
}

export function selectYouTubeWorkspaceQuality(sessionId: string, qualityId: string) {
	return callService<void>(
		LISTEN_LIVE_PLAYER_SERVICE,
		"ControlSession",
		createYouTubeControlRequest(sessionId, "select-quality", qualityId),
	);
}

export function selectYouTubeWorkspacePlaybackRate(
	sessionId: string,
	playbackRateId: string,
) {
	return callService<void>(
		LISTEN_LIVE_PLAYER_SERVICE,
		"ControlSession",
		createYouTubeControlRequest(
			sessionId,
			"select-playback-rate",
			playbackRateId,
		),
	);
}

export function getYouTubeWorkspacePlayerStatus() {
  return callService<YouTubePlayerStatus>(LISTEN_LIVE_PLAYER_SERVICE, "Status");
}

export function subscribeYouTubePlayerStatus(
  sessionId: string,
  handler: (status: YouTubePlayerStatus) => void,
) {
  let active = true;
  let unsubscribe = () => {};
  void import("@wailsio/runtime")
    .then(({ Events }) => {
      if (!active) {
        return;
      }
      unsubscribe = Events.On(LISTEN_LIVE_PLAYER_EVENT, (event: unknown) => {
        const payload = ((event as { data?: unknown })?.data ?? event) as
          | YouTubePlayerStatus
          | undefined;
        if (isYouTubePlayerStatusForSession(payload, sessionId)) {
          handler(payload);
        }
      });
    })
    .catch(() => {});
  return () => {
    active = false;
    unsubscribe();
  };
}

export function isYouTubePlayerStatusForSession(
  status: YouTubePlayerStatus | null | undefined,
  sessionId: string,
): status is YouTubePlayerStatus {
  const expectedSessionID = sessionId.trim();
  return Boolean(status) &&
    status?.provider === "youtube" &&
    Boolean(expectedSessionID) &&
    status.sessionId?.trim() === expectedSessionID;
}

export function normalizeBrowsePage(
  raw: unknown,
  fallbackRoute: YouTubeWorkspaceRouteId,
): YouTubeWorkspaceBrowsePage {
  const value = (raw ?? {}) as Partial<YouTubeWorkspaceBrowsePage>;
  const normalizedItems = Array.isArray(value.items)
    ? value.items.filter(
        (item): item is YouTubeWorkspaceVideo =>
          Boolean(
            item &&
              typeof item === "object" &&
              typeof item.videoId === "string" &&
              (item.videoId.length === 11 ||
                (item.itemKind === "playlist" &&
                  typeof item.playlistId === "string" &&
                  item.playlistId.length >= 10)),
          ),
      )
    : [];
  const seenItems = new Set<string>();
  const items = normalizedItems.filter((item) => {
    const identity = youtubeBrowseItemIdentity(item);
    if (!identity || seenItems.has(identity)) {
      return false;
    }
    seenItems.add(identity);
    return true;
  });
  return {
    routeId: normalizeRouteId(value.routeId, fallbackRoute),
    title: String(value.title || routeTitle(fallbackRoute)),
    webUrl: String(value.webUrl || "https://www.youtube.com/"),
    items,
    continuation:
      typeof value.continuation === "string" && value.continuation.trim()
        ? value.continuation.trim()
        : undefined,
    requiresAuthentication: value.requiresAuthentication === true,
    emptyReason: value.emptyReason ? String(value.emptyReason) : undefined,
  };
}

export function normalizeYouTubeUploaderPage(
	raw: unknown,
	fallbackChannelId: string,
): YouTubeUploaderPageData {
	const value = (raw ?? {}) as Partial<YouTubeUploaderPageData>;
	const channelId =
		typeof value.channelId === "string" && value.channelId.trim()
			? value.channelId.trim()
			: fallbackChannelId.trim();
	const normalizedVideos = normalizeBrowsePage(
		{
			routeId: "home",
			items: Array.isArray(value.videos) ? value.videos : [],
		},
		"home",
	).items.filter((video) => video.itemKind !== "playlist");
	const subscriberCount = Number(value.subscriberCount);
	return {
		channelId,
		name:
			typeof value.name === "string" && value.name.trim()
				? value.name.trim()
				: channelId,
		handle: normalizedOptionalString(value.handle),
		description: normalizedOptionalString(value.description),
		avatarUrl: normalizedOptionalString(value.avatarUrl),
		bannerUrl: normalizedOptionalString(value.bannerUrl),
		subscriberCount:
			Number.isFinite(subscriberCount) && subscriberCount > 0
				? subscriberCount
				: undefined,
		subscriberLabel: normalizedOptionalString(value.subscriberLabel),
		videoCountLabel: normalizedOptionalString(value.videoCountLabel),
		isSubscribed:
			typeof value.isSubscribed === "boolean"
				? value.isSubscribed
				: undefined,
		webUrl:
			typeof value.webUrl === "string" && value.webUrl.trim()
				? value.webUrl.trim()
				: `https://www.youtube.com/channel/${encodeURIComponent(channelId)}`,
		videos: normalizedVideos,
		continuation: normalizedOptionalString(value.continuation),
	};
}

export function appendYouTubeUploaderPage(
	current: YouTubeUploaderPageData | null,
	incoming: YouTubeUploaderPageData,
	requestedContinuation: string,
) {
	const expectedContinuation = requestedContinuation.trim();
	if (
		!current ||
		!expectedContinuation ||
		current.channelId !== incoming.channelId ||
		current.continuation?.trim() !== expectedContinuation
	) {
		return current;
	}
	const seen = new Set(current.videos.map((video) => video.videoId));
	const videos = [...current.videos];
	for (const video of incoming.videos) {
		if (!video.videoId || seen.has(video.videoId)) {
			continue;
		}
		seen.add(video.videoId);
		videos.push(video);
	}
	const nextContinuation = incoming.continuation?.trim() || "";
	return {
		...current,
		videos,
		continuation:
			nextContinuation && nextContinuation !== expectedContinuation
				? nextContinuation
				: undefined,
	};
}

function normalizedOptionalString(value: unknown) {
	return typeof value === "string" && value.trim() ? value.trim() : undefined;
}

function youtubeBrowseItemIdentity(item: YouTubeWorkspaceVideo) {
  const playlistID = item.playlistId?.trim() || "";
  if (item.itemKind === "playlist" && playlistID) {
    return `playlist:${playlistID}`;
  }
  const videoID = item.videoId.trim();
  return videoID ? `video:${videoID}` : "";
}

/**
 * Appends one continuation only while it is still the page's active token.
 * This makes late or duplicate responses a no-op and keeps item order stable.
 */
export function appendYouTubeWorkspaceBrowsePage(
  current: YouTubeWorkspaceBrowsePage | null,
  incoming: YouTubeWorkspaceBrowsePage,
  requestedContinuation: string,
) {
  const expectedContinuation = requestedContinuation.trim();
  if (
    !current ||
    !expectedContinuation ||
    current.routeId !== incoming.routeId ||
    current.continuation?.trim() !== expectedContinuation
  ) {
    return current;
  }

  const seen = new Set(
    current.items.map(youtubeBrowseItemIdentity).filter(Boolean),
  );
  const items = [...current.items];
  for (const item of incoming.items) {
    const identity = youtubeBrowseItemIdentity(item);
    if (!identity || seen.has(identity)) {
      continue;
    }
    seen.add(identity);
    items.push(item);
  }

  const nextContinuation = incoming.continuation?.trim() || "";
  return {
    ...current,
    items,
    continuation:
      nextContinuation && nextContinuation !== expectedContinuation
        ? nextContinuation
        : undefined,
    requiresAuthentication:
      incoming.requiresAuthentication || current.requiresAuthentication,
    emptyReason: incoming.emptyReason || current.emptyReason,
  };
}

export function normalizeRouteId(
  routeId: unknown,
  fallback: YouTubeWorkspaceRouteId = "home",
): YouTubeWorkspaceRouteId {
  switch (routeId) {
    case "search":
    case "home":
    case "subscriptions":
    case "explore":
    case "shorts":
    case "liked-videos":
    case "watch-later":
    case "playlists":
    case "history":
      return routeId;
    default:
      return fallback;
  }
}

export function routeTitle(routeId: YouTubeWorkspaceRouteId) {
  switch (routeId) {
    case "liked-videos":
      return "Liked videos";
    case "watch-later":
      return "Watch later";
    default:
      return routeId.charAt(0).toUpperCase() + routeId.slice(1);
  }
}

export function resolveYouTubeExternalQueueIndex(
	command: YouTubeWorkspaceExternalCommand["command"],
	currentIndex: number,
	queueLength: number,
) {
	if (command === "stop") {
		return -1;
	}
	const candidate = command === "previous" ? currentIndex - 1 : currentIndex + 1;
	return candidate >= 0 && candidate < queueLength ? candidate : -1;
}

export function consumeYouTubeExternalCommand(
	command: YouTubeWorkspaceExternalCommand | null,
	lastHandledID: number | null,
	currentIndex: number,
	queueLength: number,
) {
	if (!command || command.id === lastHandledID) {
		return { clearPlayback: false, handledID: lastHandledID, targetIndex: -1 };
	}
	return {
		clearPlayback: command.command === "stop",
		handledID: command.id,
		targetIndex: resolveYouTubeExternalQueueIndex(
			command.command,
			currentIndex,
			queueLength,
		),
	};
}

export function shouldCommitYouTubeVideoOpen(
	isCurrentRequest: boolean,
	isActive: boolean,
	isSameRoute: boolean,
	allowInBackground: boolean,
) {
	return isCurrentRequest && (allowInBackground || (isActive && isSameRoute));
}

export function createYouTubeWorkspaceBrowseRequest(
	routeId: YouTubeWorkspaceRouteId,
	query: string,
	playlistId?: string,
	options: {
		continuation?: string;
		locale?: string;
	} = {},
): YouTubeWorkspaceBrowseRequest {
	const normalizedPlaylistID = playlistId?.trim() || undefined;
	const continuation = options.continuation?.trim() || undefined;
	const locale = options.locale?.trim() || undefined;
	return {
		routeId: normalizedPlaylistID ? ("playlists" as const) : routeId,
		query:
			routeId === "search" && !normalizedPlaylistID
				? query.trim() || undefined
				: undefined,
		playlistId: normalizedPlaylistID,
		...(continuation ? { continuation } : {}),
		...(locale ? { locale } : {}),
	};
}
