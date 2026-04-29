import { DEFAULT_COVER_IMAGE_URL } from "@/shared/assets/default-cover";

import { DREAM_FM_STORAGE_KEY } from "@/app/main/dreamfm/catalog";
import { clampVolume } from "@/app/main/dreamfm/local-library";
import type { DreamFMLibraryShelf,DreamFMLibraryShelfKind,DreamFMMode,DreamFMOnlineGroup,DreamFMOnlineItem,DreamFMOnlineQueueKind,DreamFMOnlineQueueState,DreamFMPlayMode,DreamFMPlaylistItem,DreamFMStorageState } from "@/app/main/dreamfm/types";

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

export function buildDreamFMHighQualityThumbnailURL(thumbnailUrl: string) {
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
      parsedURL.hostname.includes("googleusercontent.com")
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

export function buildDreamFMImageCacheURL(httpBaseURL: string, imageUrl: string) {
  const trimmedURL = imageUrl.trim();
  if (!trimmedURL) {
    return "";
  }
  const normalizedURL = trimmedURL.startsWith("//")
    ? `https:${trimmedURL}`
    : trimmedURL;
  if (
    !normalizedURL.startsWith("http://") &&
    !normalizedURL.startsWith("https://")
  ) {
    return normalizedURL;
  }
  const baseURL = httpBaseURL.trim().replace(/\/+$/, "");
  if (!baseURL) {
    return "";
  }
  const query = new URLSearchParams({ url: normalizedURL });
  return `${baseURL}/api/dreamfm/image?${query.toString()}`;
}

function dedupeDreamFMImageCandidates(values: string[]) {
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

export function buildDreamFMImageCandidates(
  httpBaseURL: string,
  imageUrl: string,
) {
  const sourceArtworkURL = buildDreamFMHighQualityThumbnailURL(imageUrl);
  return dedupeDreamFMImageCandidates([
    buildDreamFMImageCacheURL(httpBaseURL, sourceArtworkURL),
    sourceArtworkURL,
  ]);
}

export function buildDreamFMTrackThumbnailCandidates(
  httpBaseURL: string,
  item: {
    videoId: string;
    thumbnailUrl?: string;
  },
) {
  const sourceArtworkURL = buildDreamFMHighQualityThumbnailURL(
    item.thumbnailUrl ?? "",
  );
  const publicPosterURL = buildYouTubePosterURL(item.videoId);
  return dedupeDreamFMImageCandidates([
    buildDreamFMImageCacheURL(httpBaseURL, sourceArtworkURL),
    sourceArtworkURL,
    buildDreamFMImageCacheURL(httpBaseURL, publicPosterURL),
    publicPosterURL,
  ]);
}

export function buildDreamFMPosterCandidates(
  httpBaseURL: string,
  item: {
    videoId: string;
    thumbnailUrl?: string;
  },
) {
  return dedupeDreamFMImageCandidates([
    ...buildDreamFMTrackThumbnailCandidates(httpBaseURL, item),
    DEFAULT_COVER_IMAGE_URL,
  ]);
}

export function isDreamFMOnlineGroup(value: string): value is DreamFMOnlineGroup {
  return value === "live" || value === "playlist";
}

export function isDreamFMMode(value: string): value is DreamFMMode {
  return value === "live" || value === "online" || value === "local";
}

export function isDreamFMPlayMode(value: string): value is DreamFMPlayMode {
  return value === "order" || value === "repeat" || value === "shuffle";
}

export function isDreamFMOnlineQueueKind(
  value: string,
): value is DreamFMOnlineQueueKind {
  return value === "none" || value === "radio" || value === "playlist";
}

export function createDefaultDreamFMOnlineQueueState(): DreamFMOnlineQueueState {
  return {
    kind: "none",
    title: "",
    items: [],
  };
}

export function createDefaultDreamFMStorageState(): DreamFMStorageState {
  return {
    version: 1,
    mode: "live",
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
    muted: false,
    volume: 1,
    localProgressByPath: {},
    onlineProgressByVideoId: {},
  };
}

export function sanitizeDreamFMProgressMap(value: unknown) {
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

export function readDreamFMStorageState(): DreamFMStorageState {
  const fallback = createDefaultDreamFMStorageState();
  if (typeof window === "undefined") {
    return fallback;
  }
  try {
    const raw = window.localStorage.getItem(DREAM_FM_STORAGE_KEY);
    if (!raw) {
      return fallback;
    }
    const parsed = JSON.parse(raw) as Partial<DreamFMStorageState>;
    const mode =
      typeof parsed.mode === "string" && isDreamFMMode(parsed.mode)
        ? parsed.mode
        : fallback.mode;
    const playMode =
      typeof parsed.playMode === "string" && isDreamFMPlayMode(parsed.playMode)
        ? parsed.playMode
        : fallback.playMode;
    const onlineQueueKind =
      typeof parsed.onlineQueueKind === "string" &&
      isDreamFMOnlineQueueKind(parsed.onlineQueueKind)
        ? parsed.onlineQueueKind
        : fallback.onlineQueueKind;
    return {
      version: 1,
      mode,
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
      muted: parsed.muted === true,
      volume: clampVolume(
        typeof parsed.volume === "number" ? parsed.volume : fallback.volume,
      ),
      localProgressByPath: sanitizeDreamFMProgressMap(
        parsed.localProgressByPath,
      ),
      onlineProgressByVideoId: sanitizeDreamFMProgressMap(
        parsed.onlineProgressByVideoId,
      ),
    };
  } catch {
    return fallback;
  }
}

export function createInitialDreamFMOnlineQueueState(
  value: DreamFMStorageState,
): DreamFMOnlineQueueState {
  if (
    value.onlineQueueKind === "playlist" &&
    value.onlineQueuePlaylistId.trim()
  ) {
    return {
      kind: "playlist",
      title: value.onlineQueueTitle.trim(),
      items: [],
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
      items: [],
      seedVideoId: value.onlineQueueSeedVideoId.trim(),
    };
  }
  return createDefaultDreamFMOnlineQueueState();
}

export function writeDreamFMStorageState(value: DreamFMStorageState) {
  if (typeof window === "undefined") {
    return;
  }
  window.localStorage.setItem(DREAM_FM_STORAGE_KEY, JSON.stringify(value));
}

export function updateDreamFMProgressMap(
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

export function dedupeOnlineItems(items: DreamFMOnlineItem[]) {
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

export function dedupePlaylistItems(items: DreamFMPlaylistItem[]) {
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

export function dedupeLibraryShelves(items: DreamFMLibraryShelf[]) {
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

export function isDreamFMLibraryShelfKind(
  value: string,
): value is DreamFMLibraryShelfKind {
  return (
    value === "tracks" ||
    value === "playlists" ||
    value === "categories" ||
    value === "artists"
  );
}
