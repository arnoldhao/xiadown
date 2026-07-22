import { useCallback, useSyncExternalStore } from "react";

import type {
  ListenLyricsData,
  ListenOnlineItem,
} from "@/app/main/listen/types";
import {
  DEFAULT_LISTEN_LYRICS_FOCUS_STYLE,
  normalizeListenLyricsFocusStyle,
  type ListenLyricsFocusStyle,
} from "@/app/main/listen/lyrics-focus-style";

export type ListenLyricsRendererPreference = "scroll" | "focus";

type ListenLyricsVersionIdentity = Pick<
  ListenLyricsData,
  "providerId" | "providerTrackId" | "source" | "timingQuality"
>;

export type ListenLyricsTrackIdentity = {
  id?: string;
  lyricsId?: string;
  videoId?: string;
  title?: string;
  artist?: string;
  durationSeconds?: number;
};

export type ListenLyricsManualOverride = {
  providerId: string;
  providerTrackId: string;
  title: string;
  artist: string;
  album?: string;
  durationSeconds?: number;
  timingQuality?: ListenLyricsData["timingQuality"];
  confidence?: number;
  updatedAt: number;
};

type ListenLyricsOffsetPreference = {
  offsetMs: number;
  updatedAt: number;
};

type ListenLyricsPreferenceState = {
  version: number;
  renderer: ListenLyricsRendererPreference;
  focusStyle: ListenLyricsFocusStyle;
  overrides: Record<string, ListenLyricsManualOverride>;
  offsets: Record<string, ListenLyricsOffsetPreference>;
};

type ListenLyricsStorage = Pick<Storage, "getItem" | "setItem">;

const LISTEN_LYRICS_PREFERENCES_KEY = "xiadown.listen.lyrics.preferences.v1";
const LISTEN_LYRICS_PREFERENCES_VERSION = 3;
const LISTEN_LYRICS_MAX_OVERRIDES = 160;
const LISTEN_LYRICS_MAX_OFFSETS = 320;
const LISTEN_LYRICS_MAX_ABSOLUTE_OFFSET_MS = 5_000;
const listenLyricsPreferenceListeners = new Set<() => void>();

export function listenLyricsTrackPreferenceKey(
  track: ListenLyricsTrackIdentity,
) {
  const videoId = String(track.videoId ?? "").trim();
  if (videoId) {
    return `video:${videoId}`;
  }
  const lyricsId = String(track.lyricsId ?? "").trim();
  if (lyricsId) {
    return lyricsId.startsWith("local:") ? lyricsId : `lyrics:${lyricsId}`;
  }
  const id = String(track.id ?? "").trim();
  if (id) {
    return id.startsWith("local:") ? id : `track:${id}`;
  }
  const title = normalizeListenLyricsIdentityText(track.title);
  const artist = normalizeListenLyricsIdentityText(track.artist);
  const duration = normalizeListenLyricsDuration(track.durationSeconds);
  return `meta:${encodeURIComponent(title)}:${encodeURIComponent(artist)}:${duration}`;
}

export function listenLyricsVersionPreferenceKey(
  track: ListenLyricsTrackIdentity,
  lyrics: ListenLyricsVersionIdentity,
) {
  const trackKey = listenLyricsTrackPreferenceKey(track);
  const provider = String(lyrics.providerId || lyrics.source || "unknown")
    .trim()
    .toLowerCase();
  const providerTrackId = String(lyrics.providerTrackId ?? "auto").trim();
  const timingQuality = String(lyrics.timingQuality ?? "unknown").trim();
  return [
    trackKey,
    encodeURIComponent(provider),
    encodeURIComponent(providerTrackId),
    timingQuality,
  ].join("|");
}

export function readListenLyricsRendererPreference(
  storage = defaultListenLyricsStorage(),
): ListenLyricsRendererPreference {
  return readListenLyricsPreferenceState(storage).renderer;
}

/**
 * React snapshot for the global renderer preference. Reading localStorage
 * during render avoids painting the default before a persisted choice loads.
 */
export function getListenLyricsRendererPreferenceSnapshot() {
  return readListenLyricsRendererPreference();
}

export function readListenLyricsFocusStylePreference(
  storage = defaultListenLyricsStorage(),
): ListenLyricsFocusStyle {
  return readListenLyricsPreferenceState(storage).focusStyle;
}

export function getListenLyricsFocusStylePreferenceSnapshot() {
  return readListenLyricsFocusStylePreference();
}

export function subscribeListenLyricsPreferences(listener: () => void) {
  listenLyricsPreferenceListeners.add(listener);
  const eventTarget = defaultListenLyricsWindow();
  const handleStorage = (event: StorageEvent) => {
    if (
      event.key !== null &&
      event.key !== LISTEN_LYRICS_PREFERENCES_KEY
    ) {
      return;
    }
    const storage = defaultListenLyricsStorage();
    if (event.storageArea && storage && event.storageArea !== storage) {
      return;
    }
    listener();
  };
  eventTarget?.addEventListener("storage", handleStorage);
  return () => {
    listenLyricsPreferenceListeners.delete(listener);
    eventTarget?.removeEventListener("storage", handleStorage);
  };
}

/**
 * Renderer selection shared by every lyrics surface and synchronized across
 * both same-window writes and other windows through the storage event.
 */
export function useListenLyricsRendererPreference() {
  return useSyncExternalStore(
    subscribeListenLyricsPreferences,
    getListenLyricsRendererPreferenceSnapshot,
    getListenLyricsRendererPreferenceServerSnapshot,
  );
}

/** Focus effect remains remembered while Dynamic or Plain is visible. */
export function useListenLyricsFocusStylePreference() {
  return useSyncExternalStore(
    subscribeListenLyricsPreferences,
    getListenLyricsFocusStylePreferenceSnapshot,
    getListenLyricsFocusStylePreferenceServerSnapshot,
  );
}

/**
 * Offset selection shared by every lyrics surface. The version key is derived
 * during render so a track or provider change can never paint with the prior
 * version's offset while waiting for an effect.
 */
export function useListenLyricsOffsetPreference(
  track: ListenLyricsTrackIdentity,
  lyrics: ListenLyricsVersionIdentity | null | undefined,
) {
  const versionKey = lyrics
    ? listenLyricsVersionPreferenceKey(track, lyrics)
    : "";
  const getSnapshot = useCallback(
    () => readListenLyricsOffsetPreferenceByKey(versionKey),
    [versionKey],
  );
  return useSyncExternalStore(
    subscribeListenLyricsPreferences,
    getSnapshot,
    getListenLyricsOffsetPreferenceServerSnapshot,
  );
}

export function saveListenLyricsRendererPreference(
  renderer: ListenLyricsRendererPreference,
  storage = defaultListenLyricsStorage(),
) {
  const state = readListenLyricsPreferenceState(storage);
  state.renderer = renderer === "focus" ? "focus" : "scroll";
  writeListenLyricsPreferenceState(state, storage);
}

export function saveListenLyricsFocusStylePreference(
  focusStyle: ListenLyricsFocusStyle,
  storage = defaultListenLyricsStorage(),
) {
  const state = readListenLyricsPreferenceState(storage);
  state.focusStyle = normalizeListenLyricsFocusStyle(focusStyle);
  return writeListenLyricsPreferenceState(state, storage);
}

export function readListenLyricsManualOverride(
  track: ListenLyricsTrackIdentity,
  storage = defaultListenLyricsStorage(),
) {
  const key = listenLyricsTrackPreferenceKey(track);
  return readListenLyricsPreferenceState(storage).overrides[key] ?? null;
}

export function saveListenLyricsManualOverride(
  track: ListenLyricsTrackIdentity,
  override: Omit<ListenLyricsManualOverride, "updatedAt">,
  storage = defaultListenLyricsStorage(),
  now = Date.now(),
) {
  const providerId = override.providerId.trim().toLowerCase();
  const providerTrackId = override.providerTrackId.trim();
  if (!providerId || !providerTrackId) {
    return false;
  }
  const state = readListenLyricsPreferenceState(storage);
  state.overrides[listenLyricsTrackPreferenceKey(track)] = {
    providerId,
    providerTrackId,
    title: override.title.trim(),
    artist: override.artist.trim(),
    album: override.album?.trim() || undefined,
    durationSeconds: normalizeOptionalListenLyricsDuration(
      override.durationSeconds,
    ),
    timingQuality: normalizeListenLyricsTimingQuality(override.timingQuality),
    confidence: normalizeListenLyricsConfidence(override.confidence),
    updatedAt: normalizeListenLyricsTimestamp(now),
  };
  state.overrides = pruneListenLyricsEntries(
    state.overrides,
    LISTEN_LYRICS_MAX_OVERRIDES,
  );
  return writeListenLyricsPreferenceState(state, storage);
}

export function clearListenLyricsManualOverride(
  track: ListenLyricsTrackIdentity,
  storage = defaultListenLyricsStorage(),
) {
  const state = readListenLyricsPreferenceState(storage);
  const key = listenLyricsTrackPreferenceKey(track);
  if (!(key in state.overrides)) {
    return true;
  }
  delete state.overrides[key];
  return writeListenLyricsPreferenceState(state, storage);
}

export function readListenLyricsOffset(
  track: ListenLyricsTrackIdentity,
  lyrics: ListenLyricsVersionIdentity,
  storage = defaultListenLyricsStorage(),
) {
  return getListenLyricsOffsetPreferenceSnapshot(track, lyrics, storage);
}

export function getListenLyricsOffsetPreferenceSnapshot(
  track: ListenLyricsTrackIdentity,
  lyrics: ListenLyricsVersionIdentity | null | undefined,
  storage = defaultListenLyricsStorage(),
) {
  if (!lyrics) {
    return 0;
  }
  return readListenLyricsOffsetPreferenceByKey(
    listenLyricsVersionPreferenceKey(track, lyrics),
    storage,
  );
}

export function saveListenLyricsOffset(
  track: ListenLyricsTrackIdentity,
  lyrics: ListenLyricsVersionIdentity,
  offsetMs: number,
  storage = defaultListenLyricsStorage(),
  now = Date.now(),
) {
  const state = readListenLyricsPreferenceState(storage);
  const key = listenLyricsVersionPreferenceKey(track, lyrics);
  const normalizedOffset = normalizeListenLyricsOffset(offsetMs);
  if (normalizedOffset === 0) {
    delete state.offsets[key];
  } else {
    state.offsets[key] = {
      offsetMs: normalizedOffset,
      updatedAt: normalizeListenLyricsTimestamp(now),
    };
  }
  state.offsets = pruneListenLyricsEntries(
    state.offsets,
    LISTEN_LYRICS_MAX_OFFSETS,
  );
  return writeListenLyricsPreferenceState(state, storage);
}

export function listenLyricsTrackIdentityFromOnlineItem(
  track: Pick<
    ListenOnlineItem,
    "videoId" | "title" | "channel" | "artists" | "durationSeconds"
  >,
): ListenLyricsTrackIdentity {
  const artist = (track.artists ?? [])
    .map((item) => item.name.trim())
    .filter(Boolean)
    .join(", ");
  return {
    videoId: track.videoId,
    title: track.title,
    artist: artist || track.channel,
    durationSeconds: track.durationSeconds,
  };
}

function defaultListenLyricsStorage(): ListenLyricsStorage | undefined {
  try {
    return typeof window === "undefined" ? undefined : window.localStorage;
  } catch {
    return undefined;
  }
}

function defaultListenLyricsWindow(): Window | undefined {
  return typeof window === "undefined" ? undefined : window;
}

function getListenLyricsRendererPreferenceServerSnapshot(): ListenLyricsRendererPreference {
  return getListenLyricsRendererPreferenceSnapshot();
}

function getListenLyricsFocusStylePreferenceServerSnapshot(): ListenLyricsFocusStyle {
  return getListenLyricsFocusStylePreferenceSnapshot();
}

function getListenLyricsOffsetPreferenceServerSnapshot() {
  return 0;
}

function readListenLyricsOffsetPreferenceByKey(
  versionKey: string,
  storage = defaultListenLyricsStorage(),
) {
  if (!versionKey) {
    return 0;
  }
  return (
    readListenLyricsPreferenceState(storage).offsets[versionKey]?.offsetMs ?? 0
  );
}

function emitListenLyricsPreferenceChange() {
  for (const listener of Array.from(listenLyricsPreferenceListeners)) {
    listener();
  }
}

function emptyListenLyricsPreferenceState(): ListenLyricsPreferenceState {
  return {
    version: LISTEN_LYRICS_PREFERENCES_VERSION,
    renderer: "scroll",
    focusStyle: DEFAULT_LISTEN_LYRICS_FOCUS_STYLE,
    overrides: {},
    offsets: {},
  };
}

function readListenLyricsPreferenceState(
  storage: ListenLyricsStorage | undefined,
): ListenLyricsPreferenceState {
  if (!storage) {
    return emptyListenLyricsPreferenceState();
  }
  try {
    const raw = storage.getItem(LISTEN_LYRICS_PREFERENCES_KEY);
    if (!raw) {
      return emptyListenLyricsPreferenceState();
    }
    return normalizeListenLyricsPreferenceState(JSON.parse(raw));
  } catch {
    return emptyListenLyricsPreferenceState();
  }
}

function writeListenLyricsPreferenceState(
  state: ListenLyricsPreferenceState,
  storage: ListenLyricsStorage | undefined,
) {
  if (!storage) {
    return false;
  }
  try {
    storage.setItem(LISTEN_LYRICS_PREFERENCES_KEY, JSON.stringify(state));
    if (storage === defaultListenLyricsStorage()) {
      emitListenLyricsPreferenceChange();
    }
    return true;
  } catch {
    return false;
  }
}

function normalizeListenLyricsPreferenceState(
  value: unknown,
): ListenLyricsPreferenceState {
  if (!value || typeof value !== "object") {
    return emptyListenLyricsPreferenceState();
  }
  const raw = value as Partial<ListenLyricsPreferenceState>;
  const state = emptyListenLyricsPreferenceState();
  state.renderer = raw.renderer === "focus" ? "focus" : "scroll";
  state.focusStyle = normalizeListenLyricsFocusStyle(raw.focusStyle);
  state.overrides = normalizeListenLyricsOverrides(raw.overrides);
  state.offsets = normalizeListenLyricsOffsets(
    raw.offsets,
    raw.version === 1 ? -1 : 1,
  );
  return state;
}

function normalizeListenLyricsOverrides(value: unknown) {
  const overrides: Record<string, ListenLyricsManualOverride> = {};
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return overrides;
  }
  for (const [key, rawValue] of Object.entries(value)) {
    if (!rawValue || typeof rawValue !== "object") {
      continue;
    }
    const raw = rawValue as Partial<ListenLyricsManualOverride>;
    const providerId = String(raw.providerId ?? "").trim().toLowerCase();
    const providerTrackId = String(raw.providerTrackId ?? "").trim();
    if (!key || !providerId || !providerTrackId) {
      continue;
    }
    overrides[key] = {
      providerId,
      providerTrackId,
      title: String(raw.title ?? "").trim(),
      artist: String(raw.artist ?? "").trim(),
      album: String(raw.album ?? "").trim() || undefined,
      durationSeconds: normalizeOptionalListenLyricsDuration(
        raw.durationSeconds,
      ),
      timingQuality: normalizeListenLyricsTimingQuality(raw.timingQuality),
      confidence: normalizeListenLyricsConfidence(raw.confidence),
      updatedAt: normalizeListenLyricsTimestamp(raw.updatedAt),
    };
  }
  return pruneListenLyricsEntries(overrides, LISTEN_LYRICS_MAX_OVERRIDES);
}

function normalizeListenLyricsOffsets(value: unknown, direction = 1) {
  const offsets: Record<string, ListenLyricsOffsetPreference> = {};
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return offsets;
  }
  for (const [key, rawValue] of Object.entries(value)) {
    if (!key || !rawValue || typeof rawValue !== "object") {
      continue;
    }
    const raw = rawValue as Partial<ListenLyricsOffsetPreference>;
    const offsetMs = normalizeListenLyricsOffset(
      Number(raw.offsetMs) * direction,
    );
    if (offsetMs === 0) {
      continue;
    }
    offsets[key] = {
      offsetMs,
      updatedAt: normalizeListenLyricsTimestamp(raw.updatedAt),
    };
  }
  return pruneListenLyricsEntries(offsets, LISTEN_LYRICS_MAX_OFFSETS);
}

function pruneListenLyricsEntries<T extends { updatedAt: number }>(
  entries: Record<string, T>,
  maximum: number,
) {
  return Object.fromEntries(
    Object.entries(entries)
      .sort(
        ([leftKey, left], [rightKey, right]) =>
          right.updatedAt - left.updatedAt || leftKey.localeCompare(rightKey),
      )
      .slice(0, maximum),
  ) as Record<string, T>;
}

function normalizeListenLyricsIdentityText(value: unknown) {
  return String(value ?? "")
    .normalize("NFKC")
    .trim()
    .toLocaleLowerCase()
    .replace(/\s+/g, " ");
}

function normalizeListenLyricsDuration(value: unknown) {
  const duration = Number(value);
  return Number.isFinite(duration) && duration > 0
    ? Math.round(duration)
    : 0;
}

function normalizeOptionalListenLyricsDuration(value: unknown) {
  const duration = normalizeListenLyricsDuration(value);
  return duration > 0 ? duration : undefined;
}

function normalizeListenLyricsTimestamp(value: unknown) {
  const timestamp = Number(value);
  return Number.isFinite(timestamp) && timestamp > 0
    ? Math.round(timestamp)
    : Date.now();
}

function normalizeListenLyricsOffset(value: unknown) {
  const offset = Number(value);
  return Number.isFinite(offset)
    ? Math.max(
        -LISTEN_LYRICS_MAX_ABSOLUTE_OFFSET_MS,
        Math.min(LISTEN_LYRICS_MAX_ABSOLUTE_OFFSET_MS, Math.round(offset)),
      )
    : 0;
}

function normalizeListenLyricsTimingQuality(
  value: unknown,
): ListenLyricsData["timingQuality"] {
  return value === "plain" ||
    value === "line" ||
    value === "word" ||
    value === "syllable" ||
    value === "estimated"
    ? value
    : undefined;
}

function normalizeListenLyricsConfidence(value: unknown) {
  const confidence = Number(value);
  return Number.isFinite(confidence)
    ? Math.min(100, Math.max(0, Math.round(confidence)))
    : undefined;
}
