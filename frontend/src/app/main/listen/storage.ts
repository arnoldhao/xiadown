import { LISTEN_DEFAULT_COVER_IMAGE_URL } from "@/shared/assets/default-cover";

import { LISTEN_STORAGE_KEY } from "@/app/main/listen/catalog";
import { clampVolume } from "@/app/main/listen/local-library";
import type { ListenLibraryShelf,ListenLibraryShelfKind,ListenLivePlaybackKind,ListenMode,ListenOnlineGroup,ListenOnlineItem,ListenOnlineQueueKind,ListenOnlineQueueState,ListenPlayMode,ListenPlaylistItem,ListenStorageState,ListenTrackArtist } from "@/app/main/listen/types";
import { doesListenThumbnailSuggestVideoContent,hasListenMusicVideoContent,isListenMusicVideoKnownNoVideo } from "@/app/main/listen/video-types";

const LISTEN_IMAGE_CACHE_AVATAR_SIZE = 96;
const LISTEN_IMAGE_CACHE_ARTWORK_SIZE = 320;
const LISTEN_IMAGE_CACHE_POSTER_SIZE = 640;
const LISTEN_STORED_QUEUE_ITEM_LIMIT = 250;

export type ListenImageCacheOptions = {
  size?: number;
};

export function buildYouTubeWatchURL(videoId: string) {
  return `https://www.youtube.com/watch?v=${encodeURIComponent(videoId)}`;
}

export function buildYouTubePosterURL(videoId: string) {
  const trimmedVideoID = videoId.trim();
  if (!trimmedVideoID) {
    return "";
  }
  return `https://i.ytimg.com/vi/${encodeURIComponent(trimmedVideoID)}/hqdefault.jpg`;
}

export function buildListenHighQualityThumbnailURL(thumbnailUrl: string) {
  const trimmedURL = thumbnailUrl.trim();
  if (!trimmedURL) {
    return "";
  }
  const normalizedURL = trimmedURL.startsWith("//")
    ? `https:${trimmedURL}`
    : trimmedURL;
  try {
    const parsedURL = new URL(normalizedURL);
    if (
      parsedURL.hostname.includes("ytimg.com") ||
      parsedURL.hostname.includes("googleusercontent.com") ||
      parsedURL.hostname.includes("ggpht.com")
    ) {
      return normalizedURL
        .replace(/w60-h60/g, "w226-h226")
        .replace(/w120-h120/g, "w226-h226");
    }
  } catch {
    return normalizedURL;
  }
  return normalizedURL;
}

export function buildListenImageCacheURL(
  httpBaseURL: string,
  imageUrl: string,
  options: ListenImageCacheOptions = {},
) {
  void httpBaseURL;
  void options;
  return normalizeListenImageSourceURL(imageUrl);
}

export function normalizeListenImageSourceURL(imageUrl: string) {
  const trimmedURL = imageUrl.trim();
  if (!trimmedURL) {
    return "";
  }
  const normalizedURL = trimmedURL.startsWith("//")
    ? `https:${trimmedURL}`
    : trimmedURL;
  const sourceURL = listenImageProxySourceURL(normalizedURL);
  return sourceURL || normalizedURL;
}

function listenImageProxySourceURL(imageUrl: string) {
  try {
    const parsedURL = new URL(imageUrl, "http://127.0.0.1");
    if (parsedURL.pathname !== "/api/listen/image") {
      return "";
    }
    const sourceURL = parsedURL.searchParams.get("url")?.trim() ?? "";
    return sourceURL.startsWith("//") ? `https:${sourceURL}` : sourceURL;
  } catch {
    return "";
  }
}

function dedupeListenImageCandidates(values: string[]) {
  const seen = new Set<string>();
  const result: string[] = [];
  values.forEach((value) => {
    const trimmedValue = String(value || "").trim();
    if (!trimmedValue || seen.has(trimmedValue)) {
      return;
    }
    seen.add(trimmedValue);
    result.push(trimmedValue);
  });
  return result;
}

export function buildListenImageCandidates(
  httpBaseURL: string,
  imageUrl: string,
  options: ListenImageCacheOptions = { size: LISTEN_IMAGE_CACHE_ARTWORK_SIZE },
) {
  const sourceArtworkURL = buildListenHighQualityThumbnailURL(imageUrl);
  return dedupeListenImageCandidates([
    buildListenImageCacheURL(httpBaseURL, sourceArtworkURL, options),
    sourceArtworkURL,
  ]);
}

export function buildListenTrackThumbnailCandidates(
  httpBaseURL: string,
  item: {
    videoId: string;
    thumbnailUrl?: string;
  },
  options: ListenImageCacheOptions = { size: LISTEN_IMAGE_CACHE_ARTWORK_SIZE },
) {
  const sourceArtworkURL = buildListenHighQualityThumbnailURL(
    item.thumbnailUrl ?? "",
  );
  const publicPosterURL = buildYouTubePosterURL(item.videoId);
  return dedupeListenImageCandidates([
    buildListenImageCacheURL(httpBaseURL, sourceArtworkURL, options),
    sourceArtworkURL,
    buildListenImageCacheURL(httpBaseURL, publicPosterURL, options),
    publicPosterURL,
  ]);
}

export function buildListenPosterCandidates(
  httpBaseURL: string,
  item: {
    videoId: string;
    thumbnailUrl?: string;
  },
  options: ListenImageCacheOptions = { size: LISTEN_IMAGE_CACHE_POSTER_SIZE },
) {
  return dedupeListenImageCandidates([
    ...buildListenTrackThumbnailCandidates(httpBaseURL, item, options),
    LISTEN_DEFAULT_COVER_IMAGE_URL,
  ]);
}

export function buildListenAvatarImageCandidates(
  httpBaseURL: string,
  item: {
    videoId?: string;
    thumbnailUrl?: string;
  },
) {
  return item.videoId?.trim()
    ? buildListenTrackThumbnailCandidates(
        httpBaseURL,
        {
          videoId: item.videoId,
          thumbnailUrl: item.thumbnailUrl,
        },
        { size: LISTEN_IMAGE_CACHE_AVATAR_SIZE },
      )
    : buildListenImageCandidates(httpBaseURL, item.thumbnailUrl ?? "", {
        size: LISTEN_IMAGE_CACHE_AVATAR_SIZE,
      });
}

export function isListenOnlineGroup(value: string): value is ListenOnlineGroup {
  return value === "live" || value === "playlist";
}

export function isListenLivePlaybackKind(
  value: string,
): value is ListenLivePlaybackKind {
  return (
    value === "youtube_music" ||
    value === "youtube" ||
    value === "stream" ||
    value === "hls"
  );
}

export function isListenMode(value: string): value is ListenMode {
  return value === "hush" || value === "muse" || value === "linger";
}

export function isListenPlayMode(value: string): value is ListenPlayMode {
  return value === "order" || value === "repeat" || value === "shuffle";
}

export function isListenOnlineQueueKind(
  value: string,
): value is ListenOnlineQueueKind {
  return value === "none" || value === "radio" || value === "playlist";
}

export function createDefaultListenOnlineQueueState(): ListenOnlineQueueState {
  return {
    kind: "none",
    title: "",
    items: [],
  };
}

export function createDefaultListenStorageState(): ListenStorageState {
  return {
    version: 2,
    mode: "hush",
    playbackMode: "hush",
    listOpen: true,
    playMode: "order",
    selectedLiveId: "",
    selectedOnlineId: "",
    browsePlaylistId: "",
    selectedLocalId: "",
    onlineQueueKind: "none",
    onlineQueueTitle: "",
    onlineQueueSeedVideoId: "",
    onlineQueuePlaylistId: "",
    onlineQueueItems: [],
    muted: false,
    volume: 1,
    localProgressByPath: {},
    onlineProgressByVideoId: {},
  };
}

export function sanitizeListenProgressMap(value: unknown) {
  if (!value || typeof value !== "object") {
    return {};
  }
  const result: Record<string, number> = {};
  for (const [key, raw] of Object.entries(value)) {
    const trimmedKey = key.trim();
    const seconds =
      typeof raw === "number" && Number.isFinite(raw)
        ? Math.max(0, Math.floor(raw))
        : 0;
    if (!trimmedKey || seconds <= 0) {
      continue;
    }
    result[trimmedKey] = seconds;
  }
  return result;
}

function sanitizeListenTrackArtists(value: unknown): ListenTrackArtist[] | undefined {
  if (!Array.isArray(value)) {
    return undefined;
  }
  const artists: ListenTrackArtist[] = [];
  const seen = new Set<string>();
  for (const raw of value) {
    if (!raw || typeof raw !== "object") {
      continue;
    }
    const record = raw as Record<string, unknown>;
    const name = String(record.name ?? "").trim();
    const browseId = String(record.browseId ?? "").trim();
    if (!name) {
      continue;
    }
    const key = browseId || name.toLocaleLowerCase();
    if (seen.has(key)) {
      continue;
    }
    seen.add(key);
    artists.push({
      name,
      browseId: browseId || undefined,
      thumbnailUrl: normalizeListenImageSourceURL(String(record.thumbnailUrl ?? "")) || undefined,
    });
  }
  return artists.length > 0 ? artists : undefined;
}

export function sanitizeListenOnlineItems(value: unknown): ListenOnlineItem[] {
  if (!Array.isArray(value)) {
    return [];
  }
  return dedupeOnlineItems(
    value
      .slice(0, LISTEN_STORED_QUEUE_ITEM_LIMIT)
      .map((raw, index): ListenOnlineItem | null => {
        if (!raw || typeof raw !== "object") {
          return null;
        }
        const record = raw as Record<string, unknown>;
        const videoId = String(record.videoId ?? "").trim();
        const id = String(record.id ?? "").trim() || videoId || `stored-${index}`;
        const group = String(record.group ?? "").trim();
        if (!videoId || !isListenOnlineGroup(group)) {
          return null;
        }
        const playbackRecord =
          record.playback && typeof record.playback === "object"
            ? (record.playback as Record<string, unknown>)
            : null;
        const playbackKind = String(playbackRecord?.kind ?? "").trim();
        const musicVideoType =
          String(record.musicVideoType ?? "").trim() || undefined;
        const musicVideoKnownNoVideo =
          isListenMusicVideoKnownNoVideo(musicVideoType);
        const thumbnailUrl =
          normalizeListenImageSourceURL(String(record.thumbnailUrl ?? "")) ||
          undefined;
        const thumbnailSuggestsVideo =
          doesListenThumbnailSuggestVideoContent(videoId, thumbnailUrl);
        const thumbnailKnownNoVideo =
          Boolean(thumbnailUrl) && !thumbnailSuggestsVideo;
        const hasVideo =
          hasListenMusicVideoContent(musicVideoType)
            ? true
            : musicVideoKnownNoVideo
              ? false
            : thumbnailSuggestsVideo
              ? true
            : thumbnailKnownNoVideo
              ? false
              : undefined;
        const videoAvailabilityKnown =
          hasVideo === true || hasVideo === false ? true : undefined;
        return {
          id,
          group,
          source: String(record.source ?? "").trim() || undefined,
          videoId,
          title: String(record.title ?? "").trim() || videoId,
          channel: String(record.channel ?? "").trim(),
          artists: sanitizeListenTrackArtists(record.artists),
          artistBrowseId:
            String(record.artistBrowseId ?? "").trim() || undefined,
          description: String(record.description ?? "").trim(),
          durationLabel: String(record.durationLabel ?? "").trim(),
          playCountLabel:
            String(record.playCountLabel ?? "").trim() || undefined,
          thumbnailUrl,
          musicVideoType,
          hasVideo,
          videoAvailabilityKnown,
          playback: playbackKind
            ? {
                kind: isListenLivePlaybackKind(playbackKind)
                  ? playbackKind
                  : "youtube_music",
                videoId:
                  String(playbackRecord?.videoId ?? "").trim() || undefined,
                url: String(playbackRecord?.url ?? "").trim() || undefined,
              }
            : undefined,
        };
      })
      .filter((item): item is ListenOnlineItem => Boolean(item)),
  );
}

export function readListenStorageState(): ListenStorageState {
  const fallback = createDefaultListenStorageState();
  if (typeof window === "undefined") {
    return fallback;
  }
  try {
    const raw = window.localStorage.getItem(LISTEN_STORAGE_KEY);
    if (!raw) {
      return fallback;
    }
    const parsed = JSON.parse(raw) as Partial<ListenStorageState>;
    const mode =
      typeof parsed.mode === "string" && isListenMode(parsed.mode)
        ? parsed.mode
        : fallback.mode;
    const playMode =
      typeof parsed.playMode === "string" && isListenPlayMode(parsed.playMode)
        ? parsed.playMode
        : fallback.playMode;
    const onlineQueueKind =
      typeof parsed.onlineQueueKind === "string" &&
      isListenOnlineQueueKind(parsed.onlineQueueKind)
        ? parsed.onlineQueueKind
        : fallback.onlineQueueKind;
    const onlineQueueItems = sanitizeListenOnlineItems(
      (parsed as { onlineQueueItems?: unknown }).onlineQueueItems,
    );
    return {
      version: 2,
      mode,
      playbackMode:
        typeof parsed.playbackMode === "string" &&
        isListenMode(parsed.playbackMode)
          ? parsed.playbackMode
          : mode,
      listOpen: parsed.listOpen !== false,
      playMode,
      selectedLiveId:
        typeof parsed.selectedLiveId === "string"
          ? parsed.selectedLiveId
          : fallback.selectedLiveId,
      selectedOnlineId:
        typeof parsed.selectedOnlineId === "string"
          ? parsed.selectedOnlineId
          : "",
      browsePlaylistId:
        typeof parsed.browsePlaylistId === "string"
          ? parsed.browsePlaylistId
          : typeof (parsed as { selectedPlaylistId?: unknown })
                .selectedPlaylistId === "string"
            ? String(
                (parsed as { selectedPlaylistId?: unknown }).selectedPlaylistId,
              )
            : "",
      selectedLocalId:
        typeof parsed.selectedLocalId === "string"
          ? parsed.selectedLocalId
          : "",
      onlineQueueKind,
      onlineQueueTitle:
        typeof parsed.onlineQueueTitle === "string"
          ? parsed.onlineQueueTitle
          : "",
      onlineQueueSeedVideoId:
        typeof parsed.onlineQueueSeedVideoId === "string"
          ? parsed.onlineQueueSeedVideoId
          : "",
      onlineQueuePlaylistId:
        typeof parsed.onlineQueuePlaylistId === "string"
          ? parsed.onlineQueuePlaylistId
          : "",
      onlineQueueItems,
      muted: parsed.muted === true,
      volume: clampVolume(
        typeof parsed.volume === "number" ? parsed.volume : fallback.volume,
      ),
      localProgressByPath: sanitizeListenProgressMap(
        parsed.localProgressByPath,
      ),
      onlineProgressByVideoId: sanitizeListenProgressMap(
        parsed.onlineProgressByVideoId,
      ),
    };
  } catch {
    return fallback;
  }
}

export function createInitialListenOnlineQueueState(
  value: ListenStorageState,
): ListenOnlineQueueState {
  const restoredItems = dedupeOnlineItems(value.onlineQueueItems);
  if (
    value.onlineQueueKind === "playlist" &&
    value.onlineQueuePlaylistId.trim()
  ) {
    return {
      kind: "playlist",
      title: value.onlineQueueTitle.trim(),
      items: restoredItems,
      playlistId: value.onlineQueuePlaylistId.trim(),
    };
  }
  if (
    value.onlineQueueKind === "radio" &&
    value.onlineQueueSeedVideoId.trim()
  ) {
    return {
      kind: "radio",
      title: value.onlineQueueTitle.trim(),
      items: restoredItems,
      seedVideoId: value.onlineQueueSeedVideoId.trim(),
    };
  }
  return createDefaultListenOnlineQueueState();
}

export function writeListenStorageState(value: ListenStorageState) {
  if (typeof window === "undefined") {
    return;
  }
  window.localStorage.setItem(LISTEN_STORAGE_KEY, JSON.stringify(value));
}

export function updateListenProgressMap(
  current: Record<string, number>,
  key: string,
  seconds: number,
) {
  const trimmedKey = key.trim();
  if (!trimmedKey) {
    return current;
  }
  const normalized = Number.isFinite(seconds)
    ? Math.max(0, Math.floor(seconds))
    : 0;
  if (normalized <= 0) {
    if (!(trimmedKey in current)) {
      return current;
    }
    const next = { ...current };
    delete next[trimmedKey];
    return next;
  }
  if (current[trimmedKey] === normalized) {
    return current;
  }
  return { ...current, [trimmedKey]: normalized };
}

export function dedupeOnlineItems(items: ListenOnlineItem[]) {
  const seen = new Set<string>();
  return items.filter((item) => {
    const key = item.videoId || item.id;
    if (seen.has(key)) {
      return false;
    }
    seen.add(key);
    return true;
  });
}

export function dedupePlaylistItems(items: ListenPlaylistItem[]) {
  const seen = new Set<string>();
  return items.filter((item) => {
    const key = item.playlistId || item.id;
    if (seen.has(key)) {
      return false;
    }
    seen.add(key);
    return true;
  });
}

export function dedupeLibraryShelves(items: ListenLibraryShelf[]) {
  const seen = new Set<string>();
  return items.filter((item) => {
    const key = item.id || `${item.kind}:${item.title}`;
    if (seen.has(key)) {
      return false;
    }
    seen.add(key);
    return true;
  });
}

export function isListenLibraryShelfKind(
  value: string,
): value is ListenLibraryShelfKind {
  return (
    value === "tracks" ||
    value === "playlists" ||
    value === "categories" ||
    value === "artists"
  );
}
