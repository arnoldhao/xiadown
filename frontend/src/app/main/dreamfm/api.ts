import { DREAM_FM_LIVE_GROUPS } from "@/app/main/dreamfm/catalog";
import { dedupeLibraryShelves,dedupeOnlineItems,dedupePlaylistItems,isDreamFMLibraryShelfKind,isDreamFMOnlineGroup } from "@/app/main/dreamfm/storage";
import type { DreamFMArtistItem,DreamFMArtistItemDTO,DreamFMArtistResponseDTO,DreamFMArtistSubscriptionResponseDTO,DreamFMCategoryItem,DreamFMCategoryItemDTO,DreamFMLibraryResponseDTO,DreamFMLibraryShelf,DreamFMLibraryShelfDTO,DreamFMLiveCatalog,DreamFMLiveCatalogDTO,DreamFMLiveGroup,DreamFMLiveStatus,DreamFMLiveStatusDTO,DreamFMLiveStatusResponseDTO,DreamFMLiveStatusValue,DreamFMLyricsData,DreamFMLyricsKind,DreamFMLyricsResponseDTO,DreamFMOnlineBrowseSource,DreamFMOnlineGroup,DreamFMOnlineItem,DreamFMPlaylistItem,DreamFMPlaylistItemDTO,DreamFMPlaylistLibraryAction,DreamFMPlaylistLibraryResponseDTO,DreamFMSearchItemDTO,DreamFMSearchResponseDTO,DreamFMTrackFavoriteResponseDTO,DreamFMTrackResponseDTO } from "@/app/main/dreamfm/types";

type DreamFMAPIErrorResponseDTO = {
  error?: {
    code?: string;
    message?: string;
    detail?: string;
    source?: string;
    retryable?: boolean;
  };
};

export class DreamFMAPIError extends Error {
  status: number;
  code: string;
  detail: string;
  source: string;
  retryable: boolean;

  constructor(message: string, options: {
    status: number;
    code?: string;
    detail?: string;
    source?: string;
    retryable?: boolean;
  }) {
    super(message);
    this.name = "DreamFMAPIError";
    this.status = options.status;
    this.code = options.code ?? "";
    this.detail = options.detail ?? "";
    this.source = options.source ?? "";
    this.retryable = options.retryable === true;
  }
}

export function getDreamFMErrorMessage(error: unknown) {
  if (error instanceof Error) {
    return error.message.trim();
  }
  return "";
}

export function getDreamFMErrorCode(error: unknown) {
  if (error instanceof DreamFMAPIError) {
    return error.code.trim();
  }
  return "";
}

export function getDreamFMErrorRetryable(error: unknown) {
  return error instanceof DreamFMAPIError && error.retryable;
}

async function buildDreamFMAPIError(response: Response, fallbackMessage: string) {
  const fallback = fallbackMessage.trim() || `DreamFM request failed: ${response.status}`;
  const contentType = response.headers.get("content-type") ?? "";
  if (contentType.includes("application/json")) {
    try {
      const payload = (await response.json()) as DreamFMAPIErrorResponseDTO;
      const error = payload.error;
      const message = error?.message?.trim() ?? "";
      const detail = formatDreamFMErrorDetail(error?.detail ?? "");
      const composedMessage =
        message && detail && message !== detail
          ? `${message}\n${detail}`
          : message || detail || fallback;
      return new DreamFMAPIError(composedMessage, {
        status: response.status,
        code: error?.code,
        detail,
        source: error?.source,
        retryable: error?.retryable === true,
      });
    } catch {
      return new DreamFMAPIError(fallback, { status: response.status });
    }
  }
  const detail = formatDreamFMErrorDetail(await response.text());
  return new DreamFMAPIError(detail || fallback, {
    status: response.status,
    detail,
  });
}

function formatDreamFMErrorDetail(value: string) {
  const trimmed = value.trim();
  if (!trimmed) {
    return "";
  }
  const jsonStart = trimmed.indexOf("{");
  if (jsonStart < 0) {
    return trimmed;
  }
  const prefix = trimmed.slice(0, jsonStart).trim();
  const jsonEnd = findDreamFMJSONEnd(trimmed, jsonStart);
  const jsonText = trimmed.slice(jsonStart, jsonEnd + 1).trim();
  const suffix = trimmed.slice(jsonEnd + 1).trim();
  try {
    const parsed = JSON.parse(jsonText) as unknown;
    const formatted = JSON.stringify(parsed, null, 2);
    return [prefix, formatted, suffix].filter(Boolean).join("\n");
  } catch {
    return trimmed;
  }
}

function normalizeDreamFMSeconds(value: unknown, fallback: number) {
  return typeof value === "number" && Number.isFinite(value)
    ? Math.max(60, Math.floor(value))
    : fallback;
}

function buildEmbeddedDreamFMLiveCatalog(): DreamFMLiveCatalog {
  return {
    schemaVersion: 1,
    id: "dreamfm.live.channel",
    version: "2026.04.28.1",
    updatedAt: "2026-04-28T11:32:39.000Z",
    ttlSeconds: 300,
    groups: DREAM_FM_LIVE_GROUPS.map((group) => ({
      ...group,
      items: group.items.map((item) => ({
        ...item,
        playback: item.playback ? { ...item.playback } : undefined,
      })),
    })),
  };
}

export async function fetchDreamFMLiveCatalog(
  httpBaseURL: string,
  signal: AbortSignal,
): Promise<DreamFMLiveCatalog> {
  const baseURL = httpBaseURL.trim().replace(/\/+$/, "");
  if (!baseURL) {
    return buildEmbeddedDreamFMLiveCatalog();
  }

  try {
    const catalogResponse = await fetch(`${baseURL}/api/dreamfm/live/catalog`, {
      method: "GET",
      cache: "no-store",
      signal,
      headers: {
        Accept: "application/json",
      },
    });
    if (!catalogResponse.ok) {
      throw new Error(`dreamfm live channel failed: ${catalogResponse.status}`);
    }
    return mapDreamFMLiveCatalog(
      (await catalogResponse.json()) as DreamFMLiveCatalogDTO,
    );
  } catch (error) {
    if (signal.aborted) {
      throw error;
    }
    return buildEmbeddedDreamFMLiveCatalog();
  }
}

export async function fetchDreamFMLiveStatuses(
  httpBaseURL: string,
  videoIds: string[],
  signal: AbortSignal,
): Promise<Record<string, DreamFMLiveStatus>> {
  const uniqueVideoIds = Array.from(
    new Set(videoIds.map((videoId) => videoId.trim()).filter(Boolean)),
  );
  if (uniqueVideoIds.length === 0) {
    return {};
  }
  const baseURL = httpBaseURL.trim().replace(/\/+$/, "");
  if (!baseURL) {
    return {};
  }
  const query = new URLSearchParams();
  uniqueVideoIds.forEach((videoId) => query.append("id", videoId));
  const response = await fetch(`${baseURL}/api/dreamfm/live/status?${query.toString()}`, {
    method: "GET",
    signal,
    headers: {
      Accept: "application/json",
    },
  });
  if (!response.ok) {
    throw new Error(`dreamfm live status failed: ${response.status}`);
  }
  const payload = (await response.json()) as DreamFMLiveStatusResponseDTO;
  const statuses: Record<string, DreamFMLiveStatus> = {};
  (payload.statuses ?? []).forEach((item) => {
    const mapped = mapDreamFMLiveStatus(item);
    if (mapped) {
      statuses[mapped[0]] = mapped[1];
    }
  });
  return statuses;
}

function findDreamFMJSONEnd(value: string, start: number) {
  let depth = 0;
  let inString = false;
  let escaped = false;
  for (let index = start; index < value.length; index += 1) {
    const character = value[index];
    if (inString) {
      if (escaped) {
        escaped = false;
      } else if (character === "\\") {
        escaped = true;
      } else if (character === "\"") {
        inString = false;
      }
      continue;
    }
    if (character === "\"") {
      inString = true;
      continue;
    }
    if (character === "{") {
      depth += 1;
      continue;
    }
    if (character === "}") {
      depth -= 1;
      if (depth === 0) {
        return index;
      }
    }
  }
  return value.length - 1;
}

function mapDreamFMLiveStatus(payload: DreamFMLiveStatusDTO): [string, DreamFMLiveStatus] | null {
  const videoId = String(payload.videoId || "").trim();
  if (!videoId) {
    return null;
  }
  const status = normalizeDreamFMLiveStatusValue(payload.status);
  return [
    videoId,
    {
      videoId,
      status,
      detail: String(payload.detail || "").trim() || undefined,
    },
  ];
}

function normalizeDreamFMLiveStatusValue(value: unknown): DreamFMLiveStatusValue {
  switch (String(value || "").trim()) {
    case "live":
      return "live";
    case "offline":
      return "offline";
    case "upcoming":
      return "upcoming";
    case "unavailable":
      return "unavailable";
    case "checking":
      return "checking";
    default:
      return "unknown";
  }
}

export async function fetchDreamFMSearch(
  httpBaseURL: string,
  query: string,
  signal: AbortSignal,
): Promise<{
  items: DreamFMOnlineItem[];
  artists: DreamFMArtistItem[];
  playlists: DreamFMPlaylistItem[];
}> {
  const baseURL = httpBaseURL.trim().replace(/\/+$/, "");
  if (!baseURL || query.trim().length < 2) {
    return { items: [], artists: [], playlists: [] };
  }
  const requestQuery = new URLSearchParams({ q: query.trim() });
  const response = await fetch(
    `${baseURL}/api/dreamfm/search?${requestQuery.toString()}`,
    {
      method: "GET",
      signal,
    },
  );
  if (!response.ok) {
    throw new Error(`dreamfm search failed: ${response.status}`);
  }
  const payload = (await response.json()) as DreamFMSearchResponseDTO;
  return {
    items: (payload.items ?? [])
      .filter((item) => isDreamFMOnlineGroup(item.group) && item.videoId.trim())
      .map((item) =>
        mapDreamFMRemoteItem(item, `ytmusic-search-${item.videoId}`),
      ),
    artists: (payload.artists ?? []).map(mapDreamFMArtistItem),
    playlists: dedupePlaylistItems(
      (payload.playlists ?? []).map(mapDreamFMPlaylistItem),
    ),
  };
}

export async function fetchDreamFMRadio(
  httpBaseURL: string,
  videoId: string,
  signal: AbortSignal,
): Promise<DreamFMOnlineItem[]> {
  const baseURL = httpBaseURL.trim().replace(/\/+$/, "");
  if (!baseURL || !videoId.trim()) {
    return [];
  }
  const requestQuery = new URLSearchParams({ id: videoId.trim() });
  const response = await fetch(
    `${baseURL}/api/dreamfm/radio?${requestQuery.toString()}`,
    {
      method: "GET",
      signal,
    },
  );
  if (!response.ok) {
    throw new Error(`dreamfm radio failed: ${response.status}`);
  }
  const payload = (await response.json()) as DreamFMSearchResponseDTO;
  return (payload.items ?? [])
    .filter((item) => isDreamFMOnlineGroup(item.group) && item.videoId.trim())
    .map((item) => mapDreamFMRemoteItem(item, `ytmusic-radio-${item.videoId}`));
}

export async function fetchDreamFMLibrary(
  httpBaseURL: string,
  signal: AbortSignal,
  source: DreamFMOnlineBrowseSource = "home",
  options: {
    browseId?: string;
    params?: string;
    continuation?: string;
  } = {},
): Promise<{
  playlists: DreamFMPlaylistItem[];
  artists: DreamFMArtistItem[];
  shelves: DreamFMLibraryShelf[];
  continuation: string;
}> {
  const baseURL = httpBaseURL.trim().replace(/\/+$/, "");
  if (!baseURL) {
    return {
      playlists: [],
      artists: [],
      shelves: [],
      continuation: "",
    };
  }
  const requestQuery = new URLSearchParams();
  if (source !== "home") {
    requestQuery.set("source", source);
  }
  const browseId = options.browseId?.trim() ?? "";
  const params = options.params?.trim() ?? "";
  const continuation = options.continuation?.trim() ?? "";
  if (browseId) {
    requestQuery.set("browseId", browseId);
  }
  if (params) {
    requestQuery.set("params", params);
  }
  if (continuation) {
    requestQuery.set("continuation", continuation);
  }
  const queryString = requestQuery.toString();
  const response = await fetch(
    `${baseURL}/api/dreamfm/library${queryString ? `?${queryString}` : ""}`,
    {
    method: "GET",
    signal,
    },
  );
  if (!response.ok) {
    throw await buildDreamFMAPIError(
      response,
      `DreamFM library failed: ${response.status}`,
    );
  }
  const payload = (await response.json()) as DreamFMLibraryResponseDTO;
  const recommendations = dedupeOnlineItems(
    (payload.recommendations ?? [])
      .filter((item) => isDreamFMOnlineGroup(item.group) && item.videoId.trim())
      .map((item) =>
        mapDreamFMRemoteItem(item, `ytmusic-home-${item.videoId}`),
      ),
  );
  const shelves = dedupeLibraryShelves(
    (payload.shelves ?? [])
      .map((item) => mapDreamFMLibraryShelf(item))
      .filter(
        (item) =>
          item.tracks.length > 0 ||
          item.playlists.length > 0 ||
          item.categories.length > 0 ||
          item.artists.length > 0,
      ),
  );
  return {
    playlists: dedupePlaylistItems(
      (payload.playlists ?? []).map(mapDreamFMPlaylistItem),
    ),
    artists: (payload.artists ?? []).map(mapDreamFMArtistItem),
    shelves:
      shelves.length > 0
        ? shelves
        : recommendations.length > 0
          ? [
              {
                id: "ytmusic-home-tracks",
                title: "",
                kind: "tracks",
                tracks: recommendations,
                playlists: [],
                categories: [],
                podcasts: [],
                artists: [],
              },
            ]
          : [],
    continuation: payload.continuation?.trim() ?? "",
  };
}

export async function fetchDreamFMPlaylistQueue(
  httpBaseURL: string,
  playlistId: string,
  signal: AbortSignal,
): Promise<DreamFMOnlineItem[]> {
  const page = await fetchDreamFMPlaylistPage(httpBaseURL, playlistId, signal);
  return page.items;
}

export async function fetchDreamFMPlaylistPage(
  httpBaseURL: string,
  playlistId: string,
  signal: AbortSignal,
  continuation = "",
): Promise<{ items: DreamFMOnlineItem[]; continuation: string; title: string; author: string }> {
  const baseURL = httpBaseURL.trim().replace(/\/+$/, "");
  const trimmedPlaylistId = playlistId.trim();
  const trimmedContinuation = continuation.trim();
  if (!baseURL || (!trimmedPlaylistId && !trimmedContinuation)) {
    return { items: [], continuation: "", title: "", author: "" };
  }
  const requestQuery = new URLSearchParams();
  if (trimmedPlaylistId) {
    requestQuery.set("id", trimmedPlaylistId);
  }
  if (trimmedContinuation) {
    requestQuery.set("continuation", trimmedContinuation);
  }
  const response = await fetch(
    `${baseURL}/api/dreamfm/playlist?${requestQuery.toString()}`,
    {
      method: "GET",
      signal,
    },
  );
  if (!response.ok) {
    throw new Error(`dreamfm playlist failed: ${response.status}`);
  }
  const payload = (await response.json()) as DreamFMSearchResponseDTO;
  return {
    items: (payload.items ?? [])
      .filter((item) => isDreamFMOnlineGroup(item.group) && item.videoId.trim())
      .map((item) =>
        mapDreamFMRemoteItem(item, `ytmusic-playlist-track-${item.videoId}`),
      ),
    continuation: payload.continuation?.trim() ?? "",
    title: payload.title?.trim() ?? "",
    author: payload.author?.trim() ?? "",
  };
}

export async function fetchDreamFMArtist(
  httpBaseURL: string,
  artist: { id?: string; name: string },
  signal: AbortSignal,
  options: { continuation?: string } = {},
): Promise<{
  id: string;
  title: string;
  subtitle: string;
  channelId: string;
  isSubscribed: boolean;
  mixPlaylistId: string;
  mixVideoId: string;
  items: DreamFMOnlineItem[];
  shelves: DreamFMLibraryShelf[];
  continuation: string;
}> {
  const baseURL = httpBaseURL.trim().replace(/\/+$/, "");
  const artistId = artist.id?.trim() ?? "";
  const artistName = artist.name.trim();
  const continuation = options.continuation?.trim() ?? "";
  if (!baseURL || (!artistId && !continuation && artistName.length < 2)) {
    return {
      id: artistId,
      title: artistName,
      subtitle: "",
      channelId: "",
      isSubscribed: false,
      mixPlaylistId: "",
      mixVideoId: "",
      items: [],
      shelves: [],
      continuation: "",
    };
  }
  const requestQuery = new URLSearchParams();
  if (artistId) {
    requestQuery.set("id", artistId);
  }
  if (artistName) {
    requestQuery.set("name", artistName);
  }
  if (continuation) {
    requestQuery.set("continuation", continuation);
  }
  const response = await fetch(
    `${baseURL}/api/dreamfm/artist?${requestQuery.toString()}`,
    {
      method: "GET",
      signal,
    },
  );
  if (!response.ok) {
    throw new Error(`dreamfm artist failed: ${response.status}`);
  }
  const payload = (await response.json()) as DreamFMArtistResponseDTO;
  const shelves = dedupeLibraryShelves(
    (payload.shelves ?? [])
      .map((item) => mapDreamFMLibraryShelf(item))
      .filter(
        (item) =>
          item.tracks.length > 0 ||
          item.playlists.length > 0 ||
          item.artists.length > 0,
      ),
  );
  const items = dedupeOnlineItems(
    (payload.items ?? [])
      .filter((item) => isDreamFMOnlineGroup(item.group) && item.videoId.trim())
      .map((item) =>
        mapDreamFMRemoteItem(item, `ytmusic-artist-${item.videoId}`),
      ),
  );
  const shelfTracks = dedupeOnlineItems(
    shelves.flatMap((shelf) => (shelf.kind === "tracks" ? shelf.tracks : [])),
  );
  return {
    id: payload.id?.trim() || artistId,
    title: payload.title?.trim() || artistName || artistId,
    subtitle: payload.subtitle?.trim() || "",
    channelId: payload.channelId?.trim() || "",
    isSubscribed: payload.isSubscribed === true,
    mixPlaylistId: payload.mixPlaylistId?.trim() || "",
    mixVideoId: payload.mixVideoId?.trim() || "",
    items: items.length > 0 ? items : shelfTracks,
    shelves,
    continuation: payload.continuation?.trim() ?? "",
  };
}

export async function updateDreamFMArtistSubscription(
  httpBaseURL: string,
  channelId: string,
  subscribed: boolean,
  signal: AbortSignal,
): Promise<boolean> {
  const baseURL = httpBaseURL.trim().replace(/\/+$/, "");
  if (!baseURL || !channelId.trim()) {
    return subscribed;
  }
  const response = await fetch(`${baseURL}/api/dreamfm/artist`, {
    method: "POST",
    signal,
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ channelId: channelId.trim(), subscribed }),
  });
  if (!response.ok) {
    throw new Error(`dreamfm artist subscription failed: ${response.status}`);
  }
  const payload = (await response.json()) as DreamFMArtistSubscriptionResponseDTO;
  return payload.subscribed === true;
}

export async function updateDreamFMPlaylistLibrary(
  httpBaseURL: string,
  playlistId: string,
  action: DreamFMPlaylistLibraryAction,
  signal: AbortSignal,
): Promise<boolean> {
  const baseURL = httpBaseURL.trim().replace(/\/+$/, "");
  if (!baseURL || !playlistId.trim()) {
    return false;
  }
  const response = await fetch(`${baseURL}/api/dreamfm/library/playlist`, {
    method: "POST",
    signal,
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ playlistId: playlistId.trim(), action }),
  });
  if (!response.ok) {
    throw new Error(`dreamfm playlist library failed: ${response.status}`);
  }
  const payload = (await response.json()) as DreamFMPlaylistLibraryResponseDTO;
  return payload.ok === true;
}

export async function fetchDreamFMTrackInfo(
  httpBaseURL: string,
  videoId: string,
  signal: AbortSignal,
): Promise<DreamFMOnlineItem | null> {
  const baseURL = httpBaseURL.trim().replace(/\/+$/, "");
  if (!baseURL || !videoId.trim()) {
    return null;
  }
  const requestQuery = new URLSearchParams({ id: videoId.trim() });
  const response = await fetch(
    `${baseURL}/api/dreamfm/track?${requestQuery.toString()}`,
    {
      method: "GET",
      signal,
    },
  );
  if (!response.ok) {
    throw new Error(`dreamfm track metadata failed: ${response.status}`);
  }
  const payload = (await response.json()) as DreamFMTrackResponseDTO;
  if (!payload.item || !isDreamFMOnlineGroup(payload.item.group)) {
    return null;
  }
  return mapDreamFMRemoteItem(payload.item, `ytmusic-track-${videoId.trim()}`);
}

export async function fetchDreamFMTrackLyrics(
  httpBaseURL: string,
  track: {
    videoId?: string;
    lyricsId?: string;
    title: string;
    channel?: string;
    artist?: string;
    durationLabel?: string;
  },
  signal: AbortSignal,
  durationSeconds = 0,
): Promise<DreamFMLyricsData> {
  const baseURL = httpBaseURL.trim().replace(/\/+$/, "");
  const videoId = track.videoId?.trim() ?? "";
  const lyricsId = track.lyricsId?.trim() || videoId;
  if (!baseURL || (!videoId && !track.title.trim())) {
    return {
      videoId: lyricsId,
      kind: "unavailable",
      source: "",
      text: "",
      lines: [],
    };
  }
  const requestQuery = new URLSearchParams();
  if (videoId) {
    requestQuery.set("id", videoId);
  }
  if (lyricsId) {
    requestQuery.set("key", lyricsId);
  }
  const title = track.title.trim();
  const artist = (track.artist ?? track.channel ?? "").trim();
  if (title) {
    requestQuery.set("title", title);
  }
  if (artist) {
    requestQuery.set("artist", artist);
  }
  const duration =
    durationSeconds > 0
      ? durationSeconds
      : parseDreamFMDurationLabelSeconds(track.durationLabel ?? "");
  if (duration > 0) {
    requestQuery.set("duration", String(Math.round(duration)));
  }
  const response = await fetch(
    `${baseURL}/api/dreamfm/track/lyrics?${requestQuery.toString()}`,
    {
      method: "GET",
      signal,
    },
  );
  if (!response.ok) {
    throw await buildDreamFMAPIError(
      response,
      `DreamFM lyrics failed: ${response.status}`,
    );
  }
  return mapDreamFMLyricsResponse(
    (await response.json()) as DreamFMLyricsResponseDTO,
    lyricsId,
  );
}

export async function fetchDreamFMTrackFavorite(
  httpBaseURL: string,
  videoId: string,
  signal: AbortSignal,
): Promise<{ liked: boolean; known: boolean }> {
  const baseURL = httpBaseURL.trim().replace(/\/+$/, "");
  if (!baseURL || !videoId.trim()) {
    return { liked: false, known: false };
  }
  const requestQuery = new URLSearchParams({ id: videoId.trim() });
  const response = await fetch(
    `${baseURL}/api/dreamfm/track/favorite?${requestQuery.toString()}`,
    {
      method: "GET",
      signal,
    },
  );
  if (!response.ok) {
    throw new Error(`dreamfm track favorite failed: ${response.status}`);
  }
  const payload = (await response.json()) as DreamFMTrackFavoriteResponseDTO;
  return {
    liked: payload.liked === true,
    known: payload.known === true,
  };
}

function mapDreamFMLyricsResponse(
  payload: DreamFMLyricsResponseDTO,
  fallbackVideoId: string,
): DreamFMLyricsData {
  const kind = normalizeDreamFMLyricsKind(payload.kind);
  return {
    videoId: payload.videoId?.trim() || fallbackVideoId,
    kind,
    source: payload.source?.trim() ?? "",
    text: payload.text ?? "",
    lines: (payload.lines ?? [])
      .map((line) => ({
        startMs: finitePositiveNumber(line.startMs),
        durationMs: finitePositiveNumber(line.durationMs),
        text: line.text ?? "",
        words: (line.words ?? [])
          .map((word) => ({
            startMs: finitePositiveNumber(word.startMs),
            text: word.text ?? "",
          }))
          .filter((word) => word.text.trim()),
      }))
      .filter((line) => line.text.trim() || kind === "synced"),
  };
}

function normalizeDreamFMLyricsKind(value: string | undefined): DreamFMLyricsKind {
  switch (value) {
    case "synced":
    case "plain":
      return value;
    default:
      return "unavailable";
  }
}

function finitePositiveNumber(value: number) {
  return Number.isFinite(value) ? Math.max(0, value) : 0;
}

function parseDreamFMDurationLabelSeconds(value: string) {
  const parts = value
    .trim()
    .split(":")
    .map((part) => Number.parseInt(part, 10));
  if (
    parts.length < 2 ||
    parts.length > 3 ||
    parts.some((part) => !Number.isFinite(part) || part < 0)
  ) {
    return 0;
  }
  return parts.reduce((total, part) => total * 60 + part, 0);
}

export async function fetchDreamFMTrackFavoriteStatuses(
  httpBaseURL: string,
  videoIds: string[],
  signal: AbortSignal,
): Promise<Record<string, boolean>> {
  const baseURL = httpBaseURL.trim().replace(/\/+$/, "");
  const ids = Array.from(
    new Set(videoIds.map((item) => item.trim()).filter(Boolean)),
  ).slice(0, 50);
  if (!baseURL || ids.length === 0) {
    return {};
  }
  const requestQuery = new URLSearchParams({ ids: ids.join(",") });
  const response = await fetch(
    `${baseURL}/api/dreamfm/track/favorite?${requestQuery.toString()}`,
    {
      method: "GET",
      signal,
    },
  );
  if (!response.ok) {
    throw new Error(`dreamfm track favorites failed: ${response.status}`);
  }
  const payload = (await response.json()) as DreamFMTrackFavoriteResponseDTO;
  const result: Record<string, boolean> = {};
  for (const item of payload.favorites ?? []) {
    const itemVideoId = item.videoId?.trim() ?? "";
    if (!itemVideoId || item.known !== true) {
      continue;
    }
    result[itemVideoId] = item.liked === true;
  }
  return result;
}

export async function updateDreamFMTrackFavorite(
  httpBaseURL: string,
  videoId: string,
  liked: boolean,
  signal: AbortSignal,
): Promise<boolean> {
  const baseURL = httpBaseURL.trim().replace(/\/+$/, "");
  if (!baseURL || !videoId.trim()) {
    return false;
  }
  const response = await fetch(`${baseURL}/api/dreamfm/track/favorite`, {
    method: "POST",
    signal,
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ videoId: videoId.trim(), liked }),
  });
  if (!response.ok) {
    throw new Error(`dreamfm track favorite update failed: ${response.status}`);
  }
  const payload = (await response.json()) as DreamFMTrackFavoriteResponseDTO;
  return payload.liked === true;
}

export function mapDreamFMRemoteItem(
  item: DreamFMSearchItemDTO,
  fallbackId: string,
): DreamFMOnlineItem {
  return {
    id: item.id || fallbackId,
    group: item.group as DreamFMOnlineGroup,
    source: item.source,
    videoId: item.videoId,
    title: item.title || item.videoId,
    channel: item.channel || "",
    artistBrowseId: item.artistBrowseId,
    description:
      item.description === "YouTube search" ? "" : item.description || "",
    durationLabel: item.durationLabel || "",
    playCountLabel: item.playCountLabel || "",
    thumbnailUrl: item.thumbnailUrl,
    musicVideoType: item.musicVideoType,
    hasVideo: item.hasVideo,
    videoAvailabilityKnown: item.videoAvailabilityKnown,
    playback: item.playback?.kind
      ? {
          kind: item.playback.kind,
          videoId: item.playback.videoId,
          url: item.playback.url,
        }
      : undefined,
  };
}

function mapDreamFMLiveCatalog(payload: DreamFMLiveCatalogDTO): DreamFMLiveCatalog {
  const groups = (payload.groups ?? [])
    .map((group, groupIndex): DreamFMLiveGroup => {
      const groupId = String(group.id || `live-${groupIndex + 1}`).trim();
      const title = String(group.title || groupId).trim();
      const items = dedupeOnlineItems(
        (group.items ?? [])
          .map((item, itemIndex) => {
            const videoId = String(
              item.videoId || item.playback?.videoId || "",
            ).trim();
            return mapDreamFMRemoteItem(
              {
                ...item,
                group: "live",
                videoId,
                durationLabel: item.durationLabel || "LIVE",
                playback: item.playback?.kind
                  ? { ...item.playback, videoId: item.playback.videoId || videoId }
                  : { kind: "youtube_music", videoId },
              },
              `${groupId}-${videoId || itemIndex + 1}`,
            );
          })
          .filter((item) => item.videoId.trim()),
      );
      return { id: groupId, title, items };
    })
    .filter((group) => group.id && group.items.length > 0);

  return {
    schemaVersion:
      typeof payload.schemaVersion === "number" && Number.isFinite(payload.schemaVersion)
        ? Math.max(1, Math.floor(payload.schemaVersion))
        : 1,
    id: String(payload.id || "dreamfm.live.channel").trim(),
    version: String(payload.version || "").trim(),
    updatedAt: String(payload.updatedAt || "").trim(),
    ttlSeconds: normalizeDreamFMSeconds(payload.ttlSeconds, 21600),
    groups,
  };
}

export function mapDreamFMLibraryShelf(
  item: DreamFMLibraryShelfDTO,
): DreamFMLibraryShelf {
  const kind = isDreamFMLibraryShelfKind(item.kind) ? item.kind : "tracks";
  return {
    id: item.id || `${kind}:${item.title || "ytmusic"}`,
    title: item.title || "",
    kind,
    tracks:
      kind === "tracks"
        ? dedupeOnlineItems(
            (item.tracks ?? [])
              .filter(
                (track) =>
                  isDreamFMOnlineGroup(track.group) && track.videoId.trim(),
              )
              .map((track) =>
                mapDreamFMRemoteItem(track, `ytmusic-home-${track.videoId}`),
              ),
          )
        : [],
    playlists:
      kind === "playlists"
        ? dedupePlaylistItems(
            (item.playlists ?? []).map(mapDreamFMPlaylistItem),
          )
        : [],
    categories:
      kind === "categories"
        ? (item.categories ?? []).map(mapDreamFMCategoryItem)
        : [],
    podcasts: [],
    artists: kind === "artists" ? (item.artists ?? []).map(mapDreamFMArtistItem) : [],
  };
}

export function mapDreamFMPlaylistItem(
  item: DreamFMPlaylistItemDTO,
): DreamFMPlaylistItem {
  return {
    id: item.id || item.playlistId,
    playlistId: item.playlistId,
    title: item.title || item.playlistId,
    channel: item.channel || "YouTube Music",
    description: item.description || "",
    thumbnailUrl: item.thumbnailUrl,
  };
}

export function mapDreamFMArtistItem(
  item: DreamFMArtistItemDTO,
): DreamFMArtistItem {
  return {
    id: item.id || item.browseId,
    browseId: item.browseId,
    name: item.name || item.browseId,
    subtitle: item.subtitle || "YouTube Music",
    thumbnailUrl: item.thumbnailUrl,
  };
}

export function mapDreamFMCategoryItem(
  item: DreamFMCategoryItemDTO,
): DreamFMCategoryItem {
  return {
    id: item.id || [item.browseId, item.params].filter(Boolean).join("_"),
    browseId: item.browseId,
    params: item.params ?? "",
    title: item.title || item.browseId,
    colorHex: item.colorHex,
    thumbnailUrl: item.thumbnailUrl,
  };
}
