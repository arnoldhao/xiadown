import type {
  YouTubePlaybackDescriptor,
  YouTubePlayerStatus,
  YouTubeWorkspaceRouteId,
  YouTubeWorkspaceVideo,
} from "@/app/youtube/types";

/**
 * The minimal playlist identity needed to restore the primary pane after a
 * Watch page is closed. Browse results remain owned by YouTubeWorkspacePage.
 */
export interface YouTubePrimaryPlaylistTarget {
  readonly playlistId: string;
  readonly title: string;
  readonly webUrl: string;
}

export type YouTubePrimaryReturnTarget =
  | { readonly kind: "browse" }
  | {
      readonly kind: "playlist";
      readonly playlist: YouTubePrimaryPlaylistTarget;
    };

/**
 * Navigation state for the YouTube workspace's primary pane.
 *
 * `routeId` identifies the selected sidebar root. A route change resets the
 * detail stack, while Watch-to-Watch navigation keeps its original return
 * target so Back remains predictable.
 */
export type YouTubePrimaryDetail =
  | {
      readonly kind: "browse";
      readonly routeId: YouTubeWorkspaceRouteId;
    }
  | {
      readonly kind: "playlist";
      readonly routeId: YouTubeWorkspaceRouteId;
      readonly playlist: YouTubePrimaryPlaylistTarget;
    }
  | {
      readonly kind: "watch";
      readonly routeId: YouTubeWorkspaceRouteId;
      readonly video: YouTubeWorkspaceVideo;
      readonly returnTarget: YouTubePrimaryReturnTarget;
    };

export interface YouTubePrimaryQueueMove {
  readonly detail: YouTubePrimaryDetail;
  readonly currentIndex: number;
}

export type YouTubePrimaryQueueDirection = "previous" | "next";

// Browse surfaces can expose a locale-specific display title while the
// player endpoint returns the creator's canonical title. Keep the caller's
// localized title first, but skip video-id placeholders when richer metadata
// is available.
export function resolveYouTubePreferredPlaybackTitle(
  videoId: string,
  ...candidates: Array<string | null | undefined>
) {
  const normalizedVideoId = videoId.trim();
  const titles = candidates
    .map((candidate) => candidate?.trim() ?? "")
    .filter(Boolean);
  return (
    titles.find((title) => title !== normalizedVideoId) ??
    titles[0] ??
    normalizedVideoId
  );
}

/**
 * Volume is provided by XiaDown's player bridge, rather than an optional
 * YouTube movie_player feature. A valid live-player session can persist the
 * requested volume while YouTube replaces or has not yet exposed its media
 * element, so a transient `controls.volume: false` must not disable the UI.
 */
export function resolveYouTubeVolumeCapability(
  playback: Pick<YouTubePlaybackDescriptor, "sessionId"> | null,
  status: Pick<YouTubePlayerStatus, "available" | "controls">,
) {
  return Boolean(
    playback?.sessionId.trim() &&
      (status.available === true ||
        (status.available === undefined && status.controls !== undefined)),
  );
}

/** Trims a query only when the user explicitly submits the Search form. */
export function submitYouTubeSearchQuery(input: string) {
  return input.trim();
}

/**
 * Keeps an existing result set while Search is being edited, but immediately
 * returns to the empty Search page once the field is cleared.
 */
export function updateYouTubeSubmittedQueryOnInput(
  submittedQuery: string,
  nextInput: string,
) {
  return nextInput.trim() ? submittedQuery : "";
}

/** Formats YouTube counters with the locale selected inside XiaDown. */
export function formatYouTubeViewCount(value: number, locale: string) {
  return new Intl.NumberFormat(locale, {
    notation: "compact",
    maximumFractionDigits: 1,
  }).format(value);
}

/** Formats canonical YouTube dates while preserving their published calendar day. */
export function formatYouTubePublishedLabel(value: string, locale: string) {
  const normalized = value.trim();
  const match = /^(\d{4})-(\d{2})-(\d{2})(?:T|$)/.exec(normalized);
  if (!match) {
    return normalized;
  }
  const year = Number(match[1]);
  const month = Number(match[2]);
  const day = Number(match[3]);
  const date = new Date(Date.UTC(year, month - 1, day, 12));
  if (
    !Number.isFinite(date.getTime()) ||
    date.getUTCFullYear() !== year ||
    date.getUTCMonth() !== month - 1 ||
    date.getUTCDate() !== day
  ) {
    return normalized;
  }
  return new Intl.DateTimeFormat(locale, {
    year: "numeric",
    month: "short",
    day: "numeric",
    timeZone: "UTC",
  }).format(date);
}

export interface YouTubeWorkspaceErrorMessages {
  readonly unavailable: string;
  readonly networkUnavailable: string;
  readonly requestTimedOut: string;
  readonly authenticationRequired: string;
  readonly sessionExpired: string;
  readonly playerUnavailable: string;
  readonly controlUnavailable: string;
}

export type YouTubeWorkspaceErrorScope = "browse" | "playback" | "control";

/**
 * Maps implementation errors to provider-specific, localized prompts without
 * exposing backend details or reusing messages from another station. A canceled
 * request was superseded by newer navigation/control work and stays silent.
 */
export function resolveYouTubeWorkspaceErrorMessage(
  reason: unknown,
  messages: YouTubeWorkspaceErrorMessages,
  scope: YouTubeWorkspaceErrorScope = "browse",
) {
  if (isYouTubeWorkspaceCancellation(reason)) {
    return "";
  }

  const detail = readYouTubeWorkspaceErrorDetail(reason).toLowerCase();
  if (isYouTubeSessionExpiredError(detail)) {
    return messages.sessionExpired;
  }
  if (isYouTubeAuthenticationRequiredError(detail)) {
    return messages.authenticationRequired;
  }
  if (isYouTubeRequestTimeoutError(detail)) {
    return messages.requestTimedOut;
  }
  if (isYouTubeNetworkUnavailableError(detail)) {
    return messages.networkUnavailable;
  }
  if (scope === "playback") {
    return messages.playerUnavailable;
  }
  if (scope === "control") {
    return messages.controlUnavailable;
  }
  return messages.unavailable;
}

/** Avoids an older failed mute request undoing a newer user or player update. */
export function rollbackYouTubeOptimisticMute(
  currentMuted: boolean,
  attemptedMuted: boolean,
  previousMuted: boolean,
) {
  return currentMuted === attemptedMuted ? previousMuted : currentMuted;
}

export function createYouTubePrimaryDetail(
  routeId: YouTubeWorkspaceRouteId,
): YouTubePrimaryDetail {
  return { kind: "browse", routeId };
}

export function openYouTubePrimaryPlaylist(
  detail: YouTubePrimaryDetail,
  playlist: YouTubePrimaryPlaylistTarget,
): YouTubePrimaryDetail {
  const playlistId = playlist.playlistId.trim();
  if (!playlistId) {
    return detail;
  }
  return {
    kind: "playlist",
    routeId: detail.routeId,
    playlist: {
      playlistId,
      title: playlist.title.trim(),
      webUrl: playlist.webUrl.trim(),
    },
  };
}

export function openYouTubePrimaryWatch(
  detail: YouTubePrimaryDetail,
  video: YouTubeWorkspaceVideo,
): YouTubePrimaryDetail {
  if (!isPlayableYouTubeVideo(video)) {
    return detail;
  }
  return {
    kind: "watch",
    routeId: detail.routeId,
    video,
    returnTarget:
      detail.kind === "watch"
        ? detail.returnTarget
        : detail.kind === "playlist"
          ? { kind: "playlist", playlist: detail.playlist }
          : { kind: "browse" },
  };
}

/** Returns one primary-pane level, preserving the selected sidebar route. */
export function backFromYouTubePrimaryDetail(
  detail: YouTubePrimaryDetail,
): YouTubePrimaryDetail {
  if (detail.kind === "browse") {
    return detail;
  }
  if (detail.kind === "playlist") {
    return createYouTubePrimaryDetail(detail.routeId);
  }
  if (detail.returnTarget.kind === "playlist") {
    return {
      kind: "playlist",
      routeId: detail.routeId,
      playlist: detail.returnTarget.playlist,
    };
  }
  return createYouTubePrimaryDetail(detail.routeId);
}

/**
 * Resets drill-in content when the sidebar root changes. Passing the current
 * route is deliberately referentially stable for React state setters.
 */
export function resetYouTubePrimaryDetailForRoute(
  detail: YouTubePrimaryDetail,
  routeId: YouTubeWorkspaceRouteId,
): YouTubePrimaryDetail {
  return detail.routeId === routeId
    ? detail
    : createYouTubePrimaryDetail(routeId);
}

/**
 * Moves an open Watch page through its queue. Non-video rows are skipped, a
 * stale index is repaired from the current video id, and queue boundaries are
 * no-ops. The Watch page's original return target is retained.
 */
export function moveYouTubePrimaryWatchInQueue(
  detail: YouTubePrimaryDetail,
  queue: readonly YouTubeWorkspaceVideo[],
  currentIndex: number,
  direction: YouTubePrimaryQueueDirection,
): YouTubePrimaryQueueMove {
  if (detail.kind !== "watch") {
    return { detail, currentIndex };
  }

  const resolvedIndex = resolveCurrentQueueIndex(detail, queue, currentIndex);
  if (resolvedIndex < 0) {
    return { detail, currentIndex };
  }

  const step = direction === "previous" ? -1 : 1;
  for (
    let candidateIndex = resolvedIndex + step;
    candidateIndex >= 0 && candidateIndex < queue.length;
    candidateIndex += step
  ) {
    const candidate = queue[candidateIndex];
    if (!candidate || !isPlayableYouTubeVideo(candidate)) {
      continue;
    }
    return {
      detail: openYouTubePrimaryWatch(detail, candidate),
      currentIndex: candidateIndex,
    };
  }

  return { detail, currentIndex: resolvedIndex };
}

function resolveCurrentQueueIndex(
  detail: Extract<YouTubePrimaryDetail, { kind: "watch" }>,
  queue: readonly YouTubeWorkspaceVideo[],
  currentIndex: number,
) {
  const currentVideoId = detail.video.videoId.trim();
  if (
    currentIndex >= 0 &&
    currentIndex < queue.length &&
    queue[currentIndex]?.videoId.trim() === currentVideoId
  ) {
    return currentIndex;
  }
  return queue.findIndex((item) => item.videoId.trim() === currentVideoId);
}

function isPlayableYouTubeVideo(video: YouTubeWorkspaceVideo) {
  return (
    video.itemKind !== "playlist" &&
    /^[A-Za-z0-9_-]{11}$/.test(video.videoId.trim())
  );
}

function isYouTubeWorkspaceCancellation(reason: unknown) {
  const name = reason instanceof Error ? reason.name.trim().toLowerCase() : "";
  if (name === "aborterror") {
    return true;
  }
  const message = readYouTubeWorkspaceErrorDetail(reason);
  return /(?:^|\b)(?:context |operation |request )?cancel(?:led|ed)(?:\b|$)/i.test(
    message.trim(),
  );
}

function readYouTubeWorkspaceErrorDetail(reason: unknown) {
  if (reason instanceof Error || typeof reason === "string") {
    return reason instanceof Error ? reason.message : reason;
  }
  if (!reason || typeof reason !== "object") {
    return "";
  }
  const record = reason as Record<string, unknown>;
  for (const key of ["message", "errorMessage", "error", "detail"]) {
    const value = record[key];
    if (typeof value === "string" && value.trim()) {
      return value;
    }
  }
  return "";
}

function isYouTubeSessionExpiredError(detail: string) {
  return (
    /(?:authentication|app session|session) (?:has )?expired/.test(detail) ||
    /(?:status|http status) (?:401|403)\b/.test(detail)
  );
}

function isYouTubeAuthenticationRequiredError(detail: string) {
  return /(?:not authenticated|authentication required|sign[ -]?in required|log[ -]?in required|sign in to|log in to|session not found|session is missing|missing app session|invalid session|no cookies)/.test(
    detail,
  );
}

function isYouTubeRequestTimeoutError(detail: string) {
  return /(?:request timed out|timed out|timeout|deadline exceeded)/.test(detail);
}

function isYouTubeNetworkUnavailableError(detail: string) {
  return /(?:network unavailable|network is unreachable|no such host|name resolution|connection refused|connection reset|broken pipe|unexpected eof|offline)/.test(
    detail,
  );
}
