import {
getXiaText
} from "@/features/xiadown/shared";

import { fetchListenTrackLyrics,getListenErrorCode,getListenErrorMessage,getListenErrorRetryable } from "@/app/main/listen/api";
import type { ListenQueuePopupAnchor } from "@/app/main/listen/queue-popups";
import type { ListenLyricsData,ListenNativePlayerEvent,ListenOnlineItem,ListenRemotePlaybackState } from "@/app/main/listen/types";
import { doesListenThumbnailSuggestVideoContent,hasListenMusicVideoContent,isListenMusicVideoKnownNoVideo } from "@/app/main/listen/video-types";

export { getListenErrorCode,getListenErrorMessage };

export type ListenVideoAvailability = "checking" | "available" | "unavailable";
export type ListenLyricsTrackRequest = Parameters<typeof fetchListenTrackLyrics>[1];
export type ListenArtistLabelPart =
  | { kind: "artist"; text: string }
  | { kind: "separator"; text: string };
export type ListenLyricsCurrentState = {
  loading: boolean;
  data: ListenLyricsData | null;
  error: string;
  errorCode?: string;
  errorRetryable?: boolean;
};

export function resolveListenLyricsCurrentState(
  state: ListenLyricsCurrentState,
  stateIdentity: string,
  activeIdentity: string,
): ListenLyricsCurrentState {
  const expected = activeIdentity.trim();
  if (stateIdentity.trim() === expected) {
    return state;
  }
  return {
    loading: expected !== "",
    data: null,
    error: "",
  };
}

const LISTEN_LYRICS_AUTO_RETRY_DELAYS_MS = [700, 1600] as const;
const LISTEN_LYRICS_SYNCED_CACHE_TTL_MS = 24 * 60 * 60 * 1000;
const LISTEN_LYRICS_PLAIN_CACHE_TTL_MS = 5 * 60 * 1000;
const LISTEN_LYRICS_UNAVAILABLE_CACHE_TTL_MS = 2 * 60 * 1000;
const LISTEN_LOCAL_LYRICS_CACHE_TTL_MS = 60 * 1000;
const LISTEN_LYRICS_CACHE_MAX_ENTRIES = 120;
export const LISTEN_INLINE_VIDEO_FALLBACK_ASPECT_RATIO = 16 / 9;
const LISTEN_INLINE_VIDEO_MIN_ASPECT_RATIO = 9 / 16;
const LISTEN_INLINE_VIDEO_MAX_ASPECT_RATIO = 21 / 9;
const LISTEN_RELEASE_YEAR_ARTIST_PATTERN = /^(?:19|20)\d{2}\s*年?$/;
const LISTEN_ARTIST_SEPARATOR_PATTERN =
  /(\s*(?:,|，|、|;|；|\|)\s*|\s+\/\s+|\s+(?:&|and|feat\.?|ft\.?|featuring|with|x|×)\s+)/gi;

type ListenLyricsCacheEntry = {
  data: ListenLyricsData;
  syncedRequested: boolean;
  updatedAt: number;
  lastAccess: number;
};

const listenLyricsCache = new Map<string, ListenLyricsCacheEntry>();
const listenLyricsRequests = new Map<string, Promise<ListenLyricsData>>();
const listenLyricsCacheGenerations = new Map<string, number>();
const listenLyricsIdentityGenerations = new Map<string, number>();
const listenLyricsActiveCacheRequests = new Map<string, number>();
const listenLyricsActiveIdentityRequests = new Map<string, number>();

export const LISTEN_EMPTY_PROGRESS = {
  currentTime: 0,
  duration: 0,
  bufferedTime: 0,
};

/** Keeps transport intent, visual time and loading feedback as separate facts. */
export function resolveListenPlaybackActivity(
  state: ListenRemotePlaybackState,
) {
  return {
    transportActive:
      state === "loading" || state === "playing" || state === "buffering",
    timelineRunning: state === "playing",
    loading: state === "loading" || state === "buffering",
  };
}

export function isListenLiveEventForSession(
  event: Pick<ListenNativePlayerEvent, "provider" | "sessionId"> | null | undefined,
  provider: "stream" | "youtube",
  sessionId = "",
) {
  if (!event || event.provider !== provider) {
    return false;
  }
  const expectedSessionID = sessionId.trim();
  if (!expectedSessionID) {
    return true;
  }
  return event.sessionId?.trim() === expectedSessionID;
}

export function isMissingListenArtistLabel(value: string) {
  const artist = value.trim();
  const normalized = artist.toLowerCase();
  return (
    !artist ||
    normalized === "unknown" ||
    normalized === "unknown artist" ||
    normalized === "youtube" ||
    normalized === "youtube music" ||
    LISTEN_RELEASE_YEAR_ARTIST_PATTERN.test(artist)
  );
}

export function hasTrustedListenOnlineArtist(
  item: Pick<ListenOnlineItem, "channel" | "artistBrowseId" | "artistSource">,
) {
  if (isMissingListenArtistLabel(item.channel)) {
    return false;
  }
  return (
    Boolean(item.artistBrowseId?.trim()) ||
    item.artistSource === "api-linked" ||
    item.artistSource === "api-linked-multiple" ||
    item.artistSource === "api-metadata"
  );
}

export function resolveTrustedListenOnlineArtistLabel(
  item: Pick<ListenOnlineItem, "channel" | "artistBrowseId" | "artistSource">,
  fallback = "",
) {
  return hasTrustedListenOnlineArtist(item) ? item.channel.trim() : fallback.trim();
}

export function splitListenArtistLabel(value: string): ListenArtistLabelPart[] {
  const label = value.trim();
  if (!label) {
    return [];
  }
  const parts: ListenArtistLabelPart[] = [];
  const pattern = new RegExp(LISTEN_ARTIST_SEPARATOR_PATTERN);
  let previousIndex = 0;
  let match: RegExpExecArray | null;
  while ((match = pattern.exec(label)) !== null) {
    const artist = label.slice(previousIndex, match.index).trim();
    if (artist) {
      parts.push({ kind: "artist", text: artist });
    }
    if (match[0]) {
      parts.push({ kind: "separator", text: match[0] });
    }
    previousIndex = match.index + match[0].length;
  }
  const artist = label.slice(previousIndex).trim();
  if (artist) {
    parts.push({ kind: "artist", text: artist });
  }
  return parts.length > 0 ? parts : [{ kind: "artist", text: label }];
}

export function listenArtistCountFromLabelParts(parts: ListenArtistLabelPart[]) {
  return parts.filter((part) => part.kind === "artist").length;
}

export function readListenNativeEventURLVideoId(value: string) {
  const trimmed = value.trim();
  if (!trimmed) {
    return "";
  }
  try {
    const parsed = new URL(trimmed);
    const queryVideoId = parsed.searchParams.get("v")?.trim() ?? "";
    if (queryVideoId) {
      return queryVideoId;
    }
    const embedMatch = parsed.pathname.match(/\/embed\/([A-Za-z0-9_-]{11})/);
    return embedMatch?.[1] ?? "";
  } catch {
    return "";
  }
}

export function normalizeListenInlineVideoAspectRatio(value: number) {
  if (!Number.isFinite(value) || value <= 0) {
    return LISTEN_INLINE_VIDEO_FALLBACK_ASPECT_RATIO;
  }
  return Math.min(
    LISTEN_INLINE_VIDEO_MAX_ASPECT_RATIO,
    Math.max(LISTEN_INLINE_VIDEO_MIN_ASPECT_RATIO, value),
  );
}

export function resolveListenNativeEventVideoAspectRatio(
  event: ListenNativePlayerEvent,
) {
  const width = Number(event.videoWidth ?? 0);
  const height = Number(event.videoHeight ?? 0);
  if (!Number.isFinite(width) || !Number.isFinite(height) || width <= 0 || height <= 0) {
    return null;
  }
  return normalizeListenInlineVideoAspectRatio(width / height);
}

export async function copyListenTextToClipboard(value: string) {
  const text = value.trim();
  if (!text) {
    return;
  }
  if (navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text);
      return;
    } catch {
      // Fall through to the textarea path for embedded WebViews.
    }
  }
  const textarea = document.createElement("textarea");
  textarea.value = text;
  textarea.setAttribute("readonly", "true");
  textarea.className = "app-clipboard-fallback-textarea";
  document.body.appendChild(textarea);
  textarea.select();
  try {
    if (!document.execCommand("text")) {
      throw new Error("text command failed");
    }
  } finally {
    document.body.removeChild(textarea);
  }
}

export function normalizeListenLiveNativeState(
  state: ListenRemotePlaybackState,
  event: ListenNativePlayerEvent,
): ListenRemotePlaybackState {
  if (state !== "buffering" && state !== "loading") {
    return state;
  }
  const readyState = Number(event.readyState ?? 0);
  const currentTime = Number(event.currentTime ?? 0);
  const bufferedTime = Number(event.bufferedTime ?? 0);
  if (readyState >= 2 || currentTime > 0.15 || bufferedTime > 0.15) {
    return "playing";
  }
  return state;
}

export function resolveListenPlaybackStatusLabel(
  state: ListenRemotePlaybackState,
  text: ReturnType<typeof getXiaText>,
) {
  switch (state) {
    case "playing":
      return text.listen.playingStatus;
    case "paused":
      return text.listen.pausedStatus;
    case "loading":
    case "buffering":
      return text.listen.loadingStatus;
    case "error":
      return text.listen.errorStatus;
    case "ended":
      return text.listen.idleStatus;
    case "idle":
    default:
      return text.listen.idleStatus;
  }
}

export function resolveListenTrackVideoAvailability(
  track: ListenOnlineItem,
  live: boolean,
): ListenVideoAvailability {
  const videoId = track.videoId.trim();
  if (!videoId) {
    return "unavailable";
  }
  if (live) {
    return "available";
  }
  const musicVideoType = track.musicVideoType?.trim();
  if (isListenMusicVideoKnownNoVideo(musicVideoType)) {
    return "unavailable";
  }
  if (hasListenMusicVideoContent(musicVideoType)) {
    return "available";
  }
  if (doesListenThumbnailSuggestVideoContent(videoId, track.thumbnailUrl)) {
    return "available";
  }
  if (track.thumbnailUrl?.trim()) {
    return "unavailable";
  }
  return "checking";
}

export type ListenRadioFullscreenVideoDecision = "open" | "fallback" | "keep";

export function resolveListenRadioFullscreenVideoDecision(options: {
  presentation: "page" | "companion" | "fullscreen";
  workspaceFullscreen: boolean;
  active: boolean;
  enabled: boolean;
  live: boolean;
  trackKey: string;
  attemptedTrackKey: string;
  hasVideo: boolean;
  nativeVideoAvailable: boolean;
  queueOpen: boolean;
  mediaMode: "cover" | "lyrics" | "video";
}): ListenRadioFullscreenVideoDecision {
  const radioFullscreen =
    options.presentation === "fullscreen" &&
    options.workspaceFullscreen &&
    options.live;
  if (
    radioFullscreen &&
    options.mediaMode === "video" &&
    (!options.hasVideo || !options.nativeVideoAvailable)
  ) {
    return "fallback";
  }
  if (
    !radioFullscreen ||
    !options.active ||
    !options.enabled ||
    !options.trackKey ||
    options.attemptedTrackKey === options.trackKey ||
    !options.hasVideo ||
    !options.nativeVideoAvailable ||
    options.queueOpen ||
    options.mediaMode !== "cover"
  ) {
    return "keep";
  }
  return "open";
}

export function resolveListenQueuePopupAnchor(
  element: HTMLElement,
): ListenQueuePopupAnchor {
  const rect = element.getBoundingClientRect();
  const root = element.closest("[data-listen-player-root='true']");
  const rootRect =
    root instanceof HTMLElement
      ? root.getBoundingClientRect()
      : { left: 0, top: 0, width: window.innerWidth };
  return {
    x: rect.left - rootRect.left,
    y: rect.top - rootRect.top,
    width: rect.width,
    height: rect.height,
    rootWidth: rootRect.width,
  };
}

export function logListenLyrics(_message: string, _details?: Record<string, unknown>) {
}

export function listenLyricsSummary(data: ListenLyricsData | null | undefined) {
  if (!data) {
    return { kind: "none" };
  }
  return {
    videoId: data.videoId,
    kind: data.kind,
    source: data.source,
    lines: data.lines.length,
    textChars: data.text.trim().length,
  };
}

export function readListenLyricsCache(
  videoId: string,
  language?: string,
  options: { synced?: boolean } = {},
): ListenLyricsData | null {
  const key = listenLyricsCacheKey(videoId, language, options);
  if (!key) {
    return null;
  }
  const entry = listenLyricsCache.get(key);
  if (!entry) {
    return null;
  }
  const now = Date.now();
  if (now - entry.updatedAt > listenLyricsCacheTTL(entry)) {
    listenLyricsCache.delete(key);
    return null;
  }
  entry.lastAccess = now;
  return entry.data;
}

export function forgetListenLyricsCache(
  videoId: string,
  language?: string,
  options: { synced?: boolean } = {},
) {
  const key = listenLyricsCacheKey(videoId, language, options);
  if (key) {
    listenLyricsCache.delete(key);
    if ((listenLyricsActiveCacheRequests.get(key) ?? 0) > 0) {
      listenLyricsCacheGenerations.set(
        key,
        (listenLyricsCacheGenerations.get(key) ?? 0) + 1,
      );
    }
  }
}

export function forgetListenLyricsCacheVariants(videoId: string) {
  const normalizedID = videoId.trim();
  if (!normalizedID) {
    return;
  }
  if ((listenLyricsActiveIdentityRequests.get(normalizedID) ?? 0) > 0) {
    listenLyricsIdentityGenerations.set(
      normalizedID,
      (listenLyricsIdentityGenerations.get(normalizedID) ?? 0) + 1,
    );
  }
  for (const key of listenLyricsCache.keys()) {
    if (listenLyricsCacheIDFromKey(key) === normalizedID) {
      listenLyricsCache.delete(key);
    }
  }
}

function listenLyricsCacheKey(
  videoId: string,
  language?: string,
  options: { synced?: boolean } = {},
) {
  const normalizedID = videoId.trim();
  if (!normalizedID) {
    return "";
  }
  return [
    (language ?? "").trim().toLowerCase(),
    options.synced === false ? "plain" : "synced",
    normalizedID,
  ].join("\u0000");
}

function listenLyricsCacheIDFromKey(key: string) {
  const languageEnd = key.indexOf("\u0000");
  if (languageEnd < 0) {
    return "";
  }
  const modeEnd = key.indexOf("\u0000", languageEnd + 1);
  return modeEnd >= 0 ? key.slice(modeEnd + 1) : "";
}

function retainListenLyricsRequest(cacheKey: string, cacheID: string) {
  if (cacheKey) {
    listenLyricsActiveCacheRequests.set(
      cacheKey,
      (listenLyricsActiveCacheRequests.get(cacheKey) ?? 0) + 1,
    );
  }
  if (cacheID) {
    listenLyricsActiveIdentityRequests.set(
      cacheID,
      (listenLyricsActiveIdentityRequests.get(cacheID) ?? 0) + 1,
    );
  }
}

function releaseListenLyricsRequest(cacheKey: string, cacheID: string) {
  if (cacheKey) {
    const remaining = (listenLyricsActiveCacheRequests.get(cacheKey) ?? 1) - 1;
    if (remaining > 0) {
      listenLyricsActiveCacheRequests.set(cacheKey, remaining);
    } else {
      listenLyricsActiveCacheRequests.delete(cacheKey);
      listenLyricsCacheGenerations.delete(cacheKey);
    }
  }
  if (cacheID) {
    const remaining =
      (listenLyricsActiveIdentityRequests.get(cacheID) ?? 1) - 1;
    if (remaining > 0) {
      listenLyricsActiveIdentityRequests.set(cacheID, remaining);
    } else {
      listenLyricsActiveIdentityRequests.delete(cacheID);
      listenLyricsIdentityGenerations.delete(cacheID);
    }
  }
}

function listenLyricsCacheTTL(entry: ListenLyricsCacheEntry) {
  if (
    entry.data.providerId?.startsWith("local_") ||
    entry.data.videoId.startsWith("local:")
  ) {
    return LISTEN_LOCAL_LYRICS_CACHE_TTL_MS;
  }
  if (entry.data.kind === "synced") {
    return LISTEN_LYRICS_SYNCED_CACHE_TTL_MS;
  }
  if (entry.data.kind === "plain") {
    return entry.syncedRequested
      ? LISTEN_LYRICS_PLAIN_CACHE_TTL_MS
      : LISTEN_LYRICS_SYNCED_CACHE_TTL_MS;
  }
  return LISTEN_LYRICS_UNAVAILABLE_CACHE_TTL_MS;
}

function storeListenLyricsCache(
  videoId: string,
  language: string | undefined,
  options: { synced?: boolean },
  data: ListenLyricsData,
) {
  const key = listenLyricsCacheKey(videoId, language, options);
  if (!key) {
    return;
  }
  const now = Date.now();
  listenLyricsCache.set(key, {
    data,
    syncedRequested: options.synced !== false,
    updatedAt: now,
    lastAccess: now,
  });
  for (const [candidateKey, entry] of listenLyricsCache) {
    if (now - entry.updatedAt > listenLyricsCacheTTL(entry)) {
      listenLyricsCache.delete(candidateKey);
    }
  }
  while (listenLyricsCache.size > LISTEN_LYRICS_CACHE_MAX_ENTRIES) {
    let oldestKey = "";
    let oldestAccess = Number.POSITIVE_INFINITY;
    for (const [candidateKey, entry] of listenLyricsCache) {
      if (entry.lastAccess < oldestAccess) {
        oldestKey = candidateKey;
        oldestAccess = entry.lastAccess;
      }
    }
    if (!oldestKey) {
      break;
    }
    listenLyricsCache.delete(oldestKey);
  }
}

export function loadListenLyricsCached(options: {
  cacheID: string;
  language?: string;
  synced?: boolean;
  force?: boolean;
  refreshPlain?: boolean;
  requestKey: string;
  loader: () => Promise<ListenLyricsData>;
}) {
  const cacheOptions = { synced: options.synced };
  const cacheKey = listenLyricsCacheKey(
    options.cacheID,
    options.language,
    cacheOptions,
  );
  const cacheGeneration = listenLyricsCacheGenerations.get(cacheKey) ?? 0;
  const normalizedCacheID = options.cacheID.trim();
  const identityGeneration =
    listenLyricsIdentityGenerations.get(normalizedCacheID) ?? 0;
  if (!options.force) {
    const cached = readListenLyricsCache(
      options.cacheID,
      options.language,
      cacheOptions,
    );
    if (cached && !(options.refreshPlain && cached.kind === "plain")) {
      return Promise.resolve(cached);
    }
  }

  const requestKey = [
    cacheKey,
    String(cacheGeneration),
    String(identityGeneration),
    options.requestKey,
  ].join("\u0000");
  const pending = listenLyricsRequests.get(requestKey);
  if (pending) {
    return pending;
  }

  const request = Promise.resolve().then(options.loader).then((data) => {
    if (
      (listenLyricsCacheGenerations.get(cacheKey) ?? 0) !==
        cacheGeneration ||
      (listenLyricsIdentityGenerations.get(normalizedCacheID) ?? 0) !==
        identityGeneration
    ) {
      return data;
    }
    storeListenLyricsCache(
      options.cacheID,
      options.language,
      cacheOptions,
      data,
    );
    if (data.videoId.trim() && data.videoId.trim() !== options.cacheID.trim()) {
      storeListenLyricsCache(
        data.videoId,
        options.language,
        cacheOptions,
        data,
      );
    }
    return data;
  });
  listenLyricsRequests.set(requestKey, request);
  retainListenLyricsRequest(cacheKey, normalizedCacheID);
  const clearRequest = () => {
    if (listenLyricsRequests.get(requestKey) === request) {
      listenLyricsRequests.delete(requestKey);
    }
    releaseListenLyricsRequest(cacheKey, normalizedCacheID);
  };
  void request.then(clearRequest, clearRequest);
  return request;
}

export function isListenLyricsDataAvailable(data: ListenLyricsData | null | undefined) {
  if (!data || data.kind === "unavailable") {
    return false;
  }
  if (data.kind === "plain") {
    return data.text.trim().length > 0;
  }
  return data.lines.some((line) => {
    if (line.text.trim()) {
      return true;
    }
    return (line.words ?? []).some((word) => word.text.trim());
  });
}

function isListenLyricsAbortError(error: unknown) {
  return error instanceof DOMException && error.name === "AbortError";
}

function shouldRetryListenLyricsError(
  error: unknown,
  track: ListenLyricsTrackRequest,
) {
  if (isListenLyricsAbortError(error)) {
    return false;
  }
  if (!track.videoId?.trim()) {
    return false;
  }
  if (getListenErrorRetryable(error)) {
    return true;
  }
  const code = getListenErrorCode(error);
  return code === "youtube_network_unavailable" ||
    code === "youtube_timeout" ||
    error instanceof TypeError;
}

function waitListenLyricsRetryDelay(delayMs: number, signal: AbortSignal) {
  if (delayMs <= 0) {
    return Promise.resolve();
  }
  return new Promise<void>((resolve, reject) => {
    let timer = 0;
    const cleanup = () => {
      window.clearTimeout(timer);
      signal.removeEventListener("abort", handleAbort);
    };
    const handleResolve = () => {
      cleanup();
      resolve();
    };
    const handleAbort = () => {
      cleanup();
      reject(new DOMException("Lyrics request aborted", "AbortError"));
    };
    if (signal.aborted) {
      handleAbort();
      return;
    }
    timer = window.setTimeout(handleResolve, delayMs);
    signal.addEventListener("abort", handleAbort, { once: true });
  });
}

async function fetchListenLyricsWithRetry(
  httpBaseURL: string,
  track: ListenLyricsTrackRequest,
  durationSeconds: number,
  signal: AbortSignal,
  language?: string,
  options: { synced?: boolean } = {},
) {
  for (let attempt = 0; ; attempt += 1) {
    try {
      return await fetchListenTrackLyrics(
        httpBaseURL,
        track,
        signal,
        durationSeconds,
        language,
        { synced: options.synced },
      );
    } catch (error) {
      const delay = LISTEN_LYRICS_AUTO_RETRY_DELAYS_MS[attempt];
      if (
        delay === undefined ||
        !shouldRetryListenLyricsError(error, track)
      ) {
        logListenLyrics("retry stop", {
          attempt,
          videoId: track.videoId || "",
          lyricsId: track.lyricsId || "",
          error: getListenErrorMessage(error),
          code: getListenErrorCode(error),
        });
        throw error;
      }
      logListenLyrics("retry scheduled", {
        attempt,
        delayMs: delay,
        videoId: track.videoId || "",
        lyricsId: track.lyricsId || "",
        error: getListenErrorMessage(error),
        code: getListenErrorCode(error),
      });
      await waitListenLyricsRetryDelay(delay, signal);
    }
  }
}

export function fetchListenLyricsCached(
  httpBaseURL: string,
  track: ListenLyricsTrackRequest,
  durationSeconds: number,
  language?: string,
  options: { force?: boolean; refreshPlain?: boolean; synced?: boolean } = {},
) {
  logListenLyrics("fetch service cache entry", {
    videoId: track.videoId || "",
    lyricsId: track.lyricsId || "",
    title: track.title,
    artist: track.artist || track.channel || "",
    durationSeconds,
    language: language || "",
    force: options.force === true,
    refreshPlain: options.refreshPlain === true,
    synced: options.synced !== false,
  });
  const lyricsID =
    track.lyricsId?.trim() ||
    track.videoId?.trim() ||
    `${track.title.trim()}\u0000${(track.artist ?? track.channel ?? "").trim()}`;
  const requestKey = [
    httpBaseURL.trim(),
    track.title.trim().toLowerCase(),
    (track.artist ?? track.channel ?? "").trim().toLowerCase(),
    String(Math.max(0, Math.round(durationSeconds))),
  ].join("\u0000");
  return loadListenLyricsCached({
    cacheID: lyricsID,
    language,
    synced: options.synced,
    force: options.force,
    requestKey,
    loader: () => {
      const controller = new AbortController();
      return fetchListenLyricsWithRetry(
        httpBaseURL,
        track,
        durationSeconds,
        controller.signal,
        language,
        { synced: options.synced },
      )
        .then((data) => {
          logListenLyrics("fetch resolved", { result: listenLyricsSummary(data) });
          return data;
        })
        .catch((error: unknown) => {
          logListenLyrics("fetch failed", {
            videoId: track.videoId || "",
            lyricsId: track.lyricsId || "",
            error: getListenErrorMessage(error),
            code: getListenErrorCode(error),
          });
          throw error;
        });
    },
  });
}
