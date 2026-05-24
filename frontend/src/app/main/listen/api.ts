import { LISTEN_LIVE_GROUPS } from "@/app/main/listen/catalog";
import { dedupeLibraryShelves,dedupeOnlineItems,dedupePlaylistItems,isListenLibraryShelfKind,isListenOnlineGroup } from "@/app/main/listen/storage";
import type { ListenArtistItem,ListenArtistItemDTO,ListenArtistResponseDTO,ListenArtistSubscriptionResponseDTO,ListenCategoryItem,ListenCategoryItemDTO,ListenLibraryResponseDTO,ListenLibraryShelf,ListenLibraryShelfDTO,ListenLiveCatalog,ListenLiveCatalogDTO,ListenLiveGroup,ListenLiveStatus,ListenLiveStatusDTO,ListenLiveStatusResponseDTO,ListenLiveStatusValue,ListenLyricsData,ListenLyricsKind,ListenLyricsResponseDTO,ListenOnlineBrowseSource,ListenOnlineGroup,ListenOnlineItem,ListenPlaylistItem,ListenPlaylistItemDTO,ListenPlaylistLibraryAction,ListenPlaylistLibraryResponseDTO,ListenSearchItemDTO,ListenSearchResponseDTO,ListenTrackArtist,ListenTrackFavoriteResponseDTO,ListenTrackResponseDTO } from "@/app/main/listen/types";

type ListenAPIErrorResponseDTO = {
  error?: {
    code?: string;
    message?: string;
    detail?: string;
    source?: string;
    retryable?: boolean;
  };
};

export type ListenLiveUserCatalogDTO = {
  columns?: Array<{
    id?: string;
    title?: string;
    sortOrder?: number;
  }>;
  channels?: Array<{
    id?: string;
    columnId?: string;
    title?: string;
    channel?: string;
    description?: string;
    source?: string;
    videoId?: string;
    thumbnailUrl?: string;
    enabled?: boolean;
    sortOrder?: number;
  }>;
};

export type ListenLiveUserColumn = {
  id: string;
  title: string;
  sortOrder: number;
};

export type ListenLiveUserChannel = {
  id: string;
  columnId: string;
  title: string;
  channel: string;
  description: string;
  source: string;
  videoId: string;
  thumbnailUrl: string;
  enabled: boolean;
  sortOrder: number;
};

export type ListenLiveUserCatalog = {
  columns: ListenLiveUserColumn[];
  channels: ListenLiveUserChannel[];
};

export type ListenLiveChannelPreviewDTO = {
  videoId?: string;
  title?: string;
  channel?: string;
  description?: string;
  durationLabel?: string;
  thumbnailUrl?: string;
};

export type ListenLiveChannelPreview = {
  videoId: string;
  title: string;
  channel: string;
  description: string;
  durationLabel: string;
  thumbnailUrl: string;
};

export function createEmptyListenLiveUserCatalog(): ListenLiveUserCatalog {
  return { columns: [], channels: [] };
}

export class ListenAPIError extends Error {
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
    this.name = "ListenAPIError";
    this.status = options.status;
    this.code = options.code ?? "";
    this.detail = options.detail ?? "";
    this.source = options.source ?? "";
    this.retryable = options.retryable === true;
  }
}

export function getListenErrorMessage(error: unknown) {
  if (error instanceof Error) {
    return error.message.trim();
  }
  return "";
}

export function getListenErrorCode(error: unknown) {
  if (error instanceof ListenAPIError) {
    return error.code.trim();
  }
  return "";
}

export function getListenErrorRetryable(error: unknown) {
  return error instanceof ListenAPIError && error.retryable;
}

async function buildListenAPIError(response: Response, fallbackMessage: string) {
  const fallback = fallbackMessage.trim() || `Listen request failed: ${response.status}`;
  const contentType = response.headers.get("content-type") ?? "";
  if (contentType.includes("application/json")) {
    try {
      const payload = (await response.json()) as ListenAPIErrorResponseDTO;
      const error = payload.error;
      const message = error?.message?.trim() ?? "";
      const detail = formatListenErrorDetail(error?.detail ?? "");
      const composedMessage =
        message && detail && message !== detail
          ? `${message}\n${detail}`
          : message || detail || fallback;
      return new ListenAPIError(composedMessage, {
        status: response.status,
        code: error?.code,
        detail,
        source: error?.source,
        retryable: error?.retryable === true,
      });
    } catch {
      return new ListenAPIError(fallback, { status: response.status });
    }
  }
  const detail = formatListenErrorDetail(await response.text());
  return new ListenAPIError(detail || fallback, {
    status: response.status,
    detail,
  });
}

function formatListenErrorDetail(value: string) {
  const trimmed = value.trim();
  if (!trimmed) {
    return "";
  }
  const jsonStart = trimmed.indexOf("{");
  if (jsonStart < 0) {
    return trimmed;
  }
  const prefix = trimmed.slice(0, jsonStart).trim();
  const jsonEnd = findListenJSONEnd(trimmed, jsonStart);
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

function normalizeListenSeconds(value: unknown, fallback: number) {
  return typeof value === "number" && Number.isFinite(value)
    ? Math.max(60, Math.floor(value))
    : fallback;
}

function appendListenLanguageParam(query: URLSearchParams, language?: string) {
  const normalized = language?.trim() ?? "";
  if (normalized) {
    query.set("language", normalized);
  }
}

function buildEmbeddedListenLiveCatalog(): ListenLiveCatalog {
  return {
    schemaVersion: 1,
    id: "listen.live.channel",
    version: "2026.04.28.1",
    updatedAt: "2026-04-28T11:32:39.000Z",
    ttlSeconds: 300,
    groups: LISTEN_LIVE_GROUPS.map((group) => ({
      ...group,
      items: group.items.map((item) => ({
        ...item,
        playback: item.playback ? { ...item.playback } : undefined,
      })),
    })),
  };
}

export async function fetchListenLiveCatalog(
  httpBaseURL: string,
  signal: AbortSignal,
): Promise<ListenLiveCatalog> {
  const baseURL = httpBaseURL.trim().replace(/\/+$/, "");
  if (!baseURL) {
    return buildEmbeddedListenLiveCatalog();
  }

  try {
    const catalogResponse = await fetch(`${baseURL}/api/listen/live/catalog`, {
      method: "GET",
      cache: "no-store",
      signal,
      headers: {
        Accept: "application/json",
      },
    });
    if (!catalogResponse.ok) {
      throw new Error(`listen live channel failed: ${catalogResponse.status}`);
    }
    const catalog = mapListenLiveCatalog(
      (await catalogResponse.json()) as ListenLiveCatalogDTO,
    );
    return mergeListenLiveUserCatalog(baseURL, catalog, signal);
  } catch (error) {
    if (signal.aborted) {
      throw error;
    }
    return mergeListenLiveUserCatalog(baseURL, buildEmbeddedListenLiveCatalog(), signal);
  }
}

export async function fetchListenLiveUserCatalog(
  httpBaseURL: string,
  signal?: AbortSignal,
): Promise<ListenLiveUserCatalog> {
  const baseURL = httpBaseURL.trim().replace(/\/+$/, "");
  if (!baseURL) {
    return createEmptyListenLiveUserCatalog();
  }
  const response = await fetch(`${baseURL}/api/listen/live/user-catalog`, {
    method: "GET",
    cache: "no-store",
    signal,
    headers: {
      Accept: "application/json",
    },
  });
  if (!response.ok) {
    throw await buildListenAPIError(response, "listen live user catalog failed");
  }
  return normalizeListenLiveUserCatalog(
    (await response.json()) as ListenLiveUserCatalogDTO,
  );
}

export async function saveListenLiveUserCatalog(
  httpBaseURL: string,
  catalog: ListenLiveUserCatalog,
  signal?: AbortSignal,
) {
  const baseURL = httpBaseURL.trim().replace(/\/+$/, "");
  if (!baseURL) {
    return;
  }
  const response = await fetch(`${baseURL}/api/listen/live/user-catalog`, {
    method: "PUT",
    signal,
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
    },
    body: JSON.stringify(catalog),
  });
  if (!response.ok) {
    throw await buildListenAPIError(response, "listen live user catalog save failed");
  }
}

export async function fetchListenLiveChannelPreview(
  httpBaseURL: string,
  url: string,
  signal?: AbortSignal,
): Promise<ListenLiveChannelPreview> {
  const baseURL = httpBaseURL.trim().replace(/\/+$/, "");
  if (!baseURL || !url.trim()) {
    throw new Error("Invalid YouTube live link.");
  }
  const query = new URLSearchParams({ url: url.trim() });
  const response = await fetch(`${baseURL}/api/listen/live/preview?${query.toString()}`, {
    method: "GET",
    cache: "no-store",
    signal,
    headers: {
      Accept: "application/json",
    },
  });
  if (!response.ok) {
    throw await buildListenAPIError(response, "listen live channel preview failed");
  }
  return normalizeListenLiveChannelPreview(
    (await response.json()) as ListenLiveChannelPreviewDTO,
  );
}

function normalizeListenLiveUserCatalog(
  catalog: ListenLiveUserCatalogDTO,
): ListenLiveUserCatalog {
  const columns = [...(catalog.columns ?? [])]
    .map((column, index) => ({
      id: String(column.id ?? "").trim(),
      title: String(column.title ?? "").trim(),
      sortOrder: Number.isFinite(column.sortOrder) ? Number(column.sortOrder) : index,
    }))
    .filter((column) => column.id && column.title)
    .sort((left, right) => left.sortOrder - right.sortOrder || left.title.localeCompare(right.title))
    .map((column, index) => ({ ...column, sortOrder: index }));
  const channels = [...(catalog.channels ?? [])]
    .map((channel, index) => ({
      id: String(channel.id ?? "").trim(),
      columnId: String(channel.columnId ?? "").trim(),
      title: String(channel.title ?? "").trim(),
      channel: String(channel.channel ?? "").trim(),
      description: String(channel.description ?? "").trim(),
      source: String(channel.source ?? "").trim() || "youtube_music",
      videoId: String(channel.videoId ?? "").trim(),
      thumbnailUrl: String(channel.thumbnailUrl ?? "").trim(),
      enabled: channel.enabled !== false,
      sortOrder: Number.isFinite(channel.sortOrder) ? Number(channel.sortOrder) : index,
    }))
    .filter((channel) =>
      channel.id &&
      channel.columnId &&
      channel.title &&
      channel.videoId,
    )
    .sort((left, right) =>
      left.columnId.localeCompare(right.columnId) ||
      left.sortOrder - right.sortOrder ||
      left.title.localeCompare(right.title),
    );
  const nextSortOrderByColumn = new Map<string, number>();
  return {
    columns,
    channels: channels.map((channel) => {
      const sortOrder = nextSortOrderByColumn.get(channel.columnId) ?? 0;
      nextSortOrderByColumn.set(channel.columnId, sortOrder + 1);
      return { ...channel, sortOrder };
    }),
  };
}

function normalizeListenLiveChannelPreview(
  preview: ListenLiveChannelPreviewDTO,
): ListenLiveChannelPreview {
  const videoId = String(preview.videoId ?? "").trim();
  return {
    videoId,
    title: String(preview.title ?? "").trim() || videoId,
    channel: String(preview.channel ?? "").trim() || "YouTube Live",
    description: String(preview.description ?? "").trim(),
    durationLabel: String(preview.durationLabel ?? "").trim() || "LIVE",
    thumbnailUrl: String(preview.thumbnailUrl ?? "").trim(),
  };
}

async function mergeListenLiveUserCatalog(
  baseURL: string,
  catalog: ListenLiveCatalog,
  signal: AbortSignal,
): Promise<ListenLiveCatalog> {
  try {
    const userCatalog = await fetchListenLiveUserCatalog(baseURL, signal);
    return mergeListenLiveCatalogWithUserCatalog(catalog, userCatalog);
  } catch {
    return catalog;
  }
}

function mergeListenLiveCatalogWithUserCatalog(
  catalog: ListenLiveCatalog,
  userCatalog: ListenLiveUserCatalog,
): ListenLiveCatalog {
  const columns = userCatalog.columns;
  const columnByID = new Map(columns.map((column) => [column.id, column]));
  const builtInGroupIDs = new Set(catalog.groups.map((group) => group.id));
  const channelsByColumn = new Map<string, ListenOnlineItem[]>();
  const channelEntries: Array<{
    columnId: string;
    sortOrder: number;
    item: ListenOnlineItem;
  } | null> = userCatalog.channels
    .filter((channel) => channel.enabled !== false)
    .map((channel, index) => {
      const columnId = channel.columnId.trim();
      const videoId = channel.videoId.trim();
      const title = channel.title.trim();
      if (
        (!builtInGroupIDs.has(columnId) && !columnByID.has(columnId)) ||
        !videoId ||
        !title
      ) {
        return null;
      }
      const item: ListenOnlineItem = {
        id: channel.id.trim() || `user-live-${videoId}`,
        group: "live",
        source: channel.source.trim() || "youtube_music",
        videoId,
        title,
        channel: channel.channel.trim(),
        description: channel.description.trim(),
        durationLabel: "LIVE",
        thumbnailUrl: channel.thumbnailUrl.trim() || undefined,
        playback: {
          kind: "youtube_music",
          videoId,
        },
      };
      return {
        columnId,
        sortOrder: Number.isFinite(channel.sortOrder) ? channel.sortOrder : index,
        item,
      };
    });
  channelEntries
    .filter((entry): entry is { columnId: string; sortOrder: number; item: ListenOnlineItem } => Boolean(entry))
    .sort((left, right) => left.sortOrder - right.sortOrder || left.item.title.localeCompare(right.item.title))
    .forEach((entry) => {
      const items = channelsByColumn.get(entry.columnId) ?? [];
      items.push(entry.item);
      channelsByColumn.set(entry.columnId, items);
    });

  const groups = catalog.groups.map((group) => {
    const items = channelsByColumn.get(group.id) ?? [];
    if (items.length === 0) {
      return group;
    }
    return {
      ...group,
      items: dedupeOnlineItems([...group.items, ...items]),
    };
  });
  const customGroups = columns
    .map((column) => ({
      id: `user-${column.id}`,
      title: column.title,
      items: channelsByColumn.get(column.id) ?? [],
    }))
    .filter((group) => group.items.length > 0);
  if (customGroups.length === 0 && groups.every((group, index) => group === catalog.groups[index])) {
    return catalog;
  }
  return {
    ...catalog,
    groups: [...groups, ...customGroups],
  };
}

export async function fetchListenLiveStatuses(
  httpBaseURL: string,
  videoIds: string[],
  signal: AbortSignal,
): Promise<Record<string, ListenLiveStatus>> {
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
  const response = await fetch(`${baseURL}/api/listen/live/status?${query.toString()}`, {
    method: "GET",
    signal,
    headers: {
      Accept: "application/json",
    },
  });
  if (!response.ok) {
    throw await buildListenAPIError(
      response,
      `listen live status failed: ${response.status}`,
    );
  }
  const payload = (await response.json()) as ListenLiveStatusResponseDTO;
  const statuses: Record<string, ListenLiveStatus> = {};
  (payload.statuses ?? []).forEach((item) => {
    const mapped = mapListenLiveStatus(item);
    if (mapped) {
      statuses[mapped[0]] = mapped[1];
    }
  });
  return statuses;
}

function findListenJSONEnd(value: string, start: number) {
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

function mapListenLiveStatus(payload: ListenLiveStatusDTO): [string, ListenLiveStatus] | null {
  const videoId = String(payload.videoId || "").trim();
  if (!videoId) {
    return null;
  }
  const status = normalizeListenLiveStatusValue(payload.status);
  return [
    videoId,
    {
      videoId,
      status,
      detail: String(payload.detail || "").trim() || undefined,
    },
  ];
}

function normalizeListenLiveStatusValue(value: unknown): ListenLiveStatusValue {
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

export async function fetchListenSearch(
  httpBaseURL: string,
  query: string,
  signal: AbortSignal,
  language?: string,
  continuation = "",
): Promise<{
  items: ListenOnlineItem[];
  artists: ListenArtistItem[];
  playlists: ListenPlaylistItem[];
  continuation: string;
}> {
  const baseURL = httpBaseURL.trim().replace(/\/+$/, "");
  if (!baseURL || query.trim().length < 2) {
    return { items: [], artists: [], playlists: [], continuation: "" };
  }
  const requestQuery = new URLSearchParams({ q: query.trim() });
  if (continuation.trim()) {
    requestQuery.set("continuation", continuation.trim());
  }
  appendListenLanguageParam(requestQuery, language);
  const response = await fetch(
    `${baseURL}/api/listen/search?${requestQuery.toString()}`,
    {
      method: "GET",
      signal,
    },
  );
  if (!response.ok) {
    throw await buildListenAPIError(
      response,
      `listen search failed: ${response.status}`,
    );
  }
  const payload = (await response.json()) as ListenSearchResponseDTO;
  return {
    items: (payload.items ?? [])
      .filter((item) => isListenOnlineGroup(item.group) && item.videoId.trim())
      .map((item) =>
        mapListenRemoteItem(item, `ytmusic-search-${item.videoId}`),
      ),
    artists: (payload.artists ?? []).map(mapListenArtistItem),
    playlists: dedupePlaylistItems(
      (payload.playlists ?? []).map(mapListenPlaylistItem),
    ),
    continuation: payload.continuation?.trim() ?? "",
  };
}

export async function fetchListenRadio(
  httpBaseURL: string,
  videoId: string,
  signal: AbortSignal,
  language?: string,
): Promise<ListenOnlineItem[]> {
  const baseURL = httpBaseURL.trim().replace(/\/+$/, "");
  if (!baseURL || !videoId.trim()) {
    return [];
  }
  const requestQuery = new URLSearchParams({ id: videoId.trim() });
  appendListenLanguageParam(requestQuery, language);
  const response = await fetch(
    `${baseURL}/api/listen/radio?${requestQuery.toString()}`,
    {
      method: "GET",
      signal,
    },
  );
  if (!response.ok) {
    throw await buildListenAPIError(
      response,
      `listen radio failed: ${response.status}`,
    );
  }
  const payload = (await response.json()) as ListenSearchResponseDTO;
  return (payload.items ?? [])
    .filter((item) => isListenOnlineGroup(item.group) && item.videoId.trim())
    .map((item) => mapListenRemoteItem(item, `ytmusic-radio-${item.videoId}`));
}

export async function fetchListenLibrary(
  httpBaseURL: string,
  signal: AbortSignal,
  source: ListenOnlineBrowseSource = "home",
  options: {
    browseId?: string;
    params?: string;
    continuation?: string;
    language?: string;
  } = {},
): Promise<{
  playlists: ListenPlaylistItem[];
  artists: ListenArtistItem[];
  shelves: ListenLibraryShelf[];
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
  appendListenLanguageParam(requestQuery, options.language);
  const queryString = requestQuery.toString();
  const response = await fetch(
    `${baseURL}/api/listen/library${queryString ? `?${queryString}` : ""}`,
    {
      method: "GET",
      signal,
    },
  );
  if (!response.ok) {
    throw await buildListenAPIError(
      response,
      `Listen library failed: ${response.status}`,
    );
  }
  const payload = (await response.json()) as ListenLibraryResponseDTO;
  const recommendations = dedupeOnlineItems(
    (payload.recommendations ?? [])
      .filter((item) => isListenOnlineGroup(item.group) && item.videoId.trim())
      .map((item) =>
        mapListenRemoteItem(item, `ytmusic-home-${item.videoId}`),
      ),
  );
  const shelves = dedupeLibraryShelves(
    (payload.shelves ?? [])
      .map((item) => mapListenLibraryShelf(item))
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
      (payload.playlists ?? []).map(mapListenPlaylistItem),
    ),
    artists: (payload.artists ?? []).map(mapListenArtistItem),
    shelves:
      shelves.length > 0
        ? shelves
        : recommendations.length > 0
          ? [
              {
                id: "ytmusic-home-tracks",
                title: "",
                kind: "tracks",
                continuation: "",
                browseId: "",
                params: "",
                tracks: recommendations,
                playlists: [],
                categories: [],
                artists: [],
              },
            ]
          : [],
    continuation: payload.continuation?.trim() ?? "",
  };
}

export async function fetchListenPlaylistQueue(
  httpBaseURL: string,
  playlistId: string,
  signal: AbortSignal,
  language?: string,
): Promise<ListenOnlineItem[]> {
  const page = await fetchListenPlaylistPage(
    httpBaseURL,
    playlistId,
    signal,
    "",
    language,
  );
  return page.items;
}

export async function fetchListenPlaylistPage(
  httpBaseURL: string,
  playlistId: string,
  signal: AbortSignal,
  continuation = "",
  language?: string,
): Promise<{
  items: ListenOnlineItem[];
  continuation: string;
  title: string;
  author: string;
}> {
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
  appendListenLanguageParam(requestQuery, language);
  const response = await fetch(
    `${baseURL}/api/listen/playlist?${requestQuery.toString()}`,
    {
      method: "GET",
      signal,
    },
  );
  if (!response.ok) {
    throw await buildListenAPIError(
      response,
      `listen playlist failed: ${response.status}`,
    );
  }
  const payload = (await response.json()) as ListenSearchResponseDTO;
  return {
    items: (payload.items ?? [])
      .filter((item) => isListenOnlineGroup(item.group) && item.videoId.trim())
      .map((item) =>
        mapListenRemoteItem(item, `ytmusic-playlist-track-${item.videoId}`),
      ),
    continuation: payload.continuation?.trim() ?? "",
    title: payload.title?.trim() ?? "",
    author: payload.author?.trim() ?? "",
  };
}

export async function fetchListenArtist(
  httpBaseURL: string,
  artist: { id?: string; name: string },
  signal: AbortSignal,
  options: {
    continuation?: string;
    browseId?: string;
    params?: string;
    language?: string;
  } = {},
): Promise<{
  id: string;
  title: string;
  subtitle: string;
  thumbnailUrl: string;
  channelId: string;
  isSubscribed: boolean;
  mixPlaylistId: string;
  mixVideoId: string;
  items: ListenOnlineItem[];
  shelves: ListenLibraryShelf[];
  continuation: string;
}> {
  const baseURL = httpBaseURL.trim().replace(/\/+$/, "");
  const artistId = artist.id?.trim() ?? "";
  const artistName = artist.name.trim();
  const continuation = options.continuation?.trim() ?? "";
  const browseId = options.browseId?.trim() ?? "";
  const params = options.params?.trim() ?? "";
  if (!baseURL || (!artistId && !browseId && !continuation && artistName.length < 2)) {
    return {
      id: artistId,
      title: artistName,
      subtitle: "",
      thumbnailUrl: "",
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
  if (browseId) {
    requestQuery.set("browseId", browseId);
  }
  if (params) {
    requestQuery.set("params", params);
  }
  appendListenLanguageParam(requestQuery, options.language);
  const response = await fetch(
    `${baseURL}/api/listen/artist?${requestQuery.toString()}`,
    {
      method: "GET",
      signal,
    },
  );
  if (!response.ok) {
    throw await buildListenAPIError(
      response,
      `listen artist failed: ${response.status}`,
    );
  }
  const payload = (await response.json()) as ListenArtistResponseDTO;
  const shelves = dedupeLibraryShelves(
    (payload.shelves ?? [])
      .map((item) => mapListenLibraryShelf(item))
      .filter(
        (item) =>
          item.tracks.length > 0 ||
          item.playlists.length > 0 ||
          item.artists.length > 0,
      ),
  );
  const items = dedupeOnlineItems(
    (payload.items ?? [])
      .filter((item) => isListenOnlineGroup(item.group) && item.videoId.trim())
      .map((item) =>
        mapListenRemoteItem(item, `ytmusic-artist-${item.videoId}`),
      ),
  );
  const shelfTracks = dedupeOnlineItems(
    shelves.flatMap((shelf) => (shelf.kind === "tracks" ? shelf.tracks : [])),
  );
  return {
    id: payload.id?.trim() || artistId,
    title: payload.title?.trim() || artistName || artistId,
    subtitle: payload.subtitle?.trim() || "",
    thumbnailUrl: payload.thumbnailUrl?.trim() || "",
    channelId: payload.channelId?.trim() || "",
    isSubscribed: payload.isSubscribed === true,
    mixPlaylistId: payload.mixPlaylistId?.trim() || "",
    mixVideoId: payload.mixVideoId?.trim() || "",
    items: items.length > 0 ? items : shelfTracks,
    shelves,
    continuation: payload.continuation?.trim() ?? "",
  };
}

export async function updateListenArtistSubscription(
  httpBaseURL: string,
  channelId: string,
  subscribed: boolean,
  signal: AbortSignal,
): Promise<boolean> {
  const baseURL = httpBaseURL.trim().replace(/\/+$/, "");
  if (!baseURL || !channelId.trim()) {
    return subscribed;
  }
  const response = await fetch(`${baseURL}/api/listen/artist`, {
    method: "POST",
    signal,
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ channelId: channelId.trim(), subscribed }),
  });
  if (!response.ok) {
    throw await buildListenAPIError(
      response,
      `listen artist subscription failed: ${response.status}`,
    );
  }
  const payload = (await response.json()) as ListenArtistSubscriptionResponseDTO;
  return payload.subscribed === true;
}

export async function updateListenPlaylistLibrary(
  httpBaseURL: string,
  playlistId: string,
  action: ListenPlaylistLibraryAction,
  signal: AbortSignal,
): Promise<boolean> {
  const baseURL = httpBaseURL.trim().replace(/\/+$/, "");
  if (!baseURL || !playlistId.trim()) {
    return false;
  }
  const response = await fetch(`${baseURL}/api/listen/library/playlist`, {
    method: "POST",
    signal,
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ playlistId: playlistId.trim(), action }),
  });
  if (!response.ok) {
    throw await buildListenAPIError(
      response,
      `listen playlist library failed: ${response.status}`,
    );
  }
  const payload = (await response.json()) as ListenPlaylistLibraryResponseDTO;
  return payload.ok === true;
}

export async function fetchListenTrackInfo(
  httpBaseURL: string,
  videoId: string,
  signal: AbortSignal,
  language?: string,
): Promise<ListenOnlineItem | null> {
  const baseURL = httpBaseURL.trim().replace(/\/+$/, "");
  if (!baseURL || !videoId.trim()) {
    return null;
  }
  const requestQuery = new URLSearchParams({ id: videoId.trim() });
  appendListenLanguageParam(requestQuery, language);
  const response = await fetch(
    `${baseURL}/api/listen/track?${requestQuery.toString()}`,
    {
      method: "GET",
      signal,
    },
  );
  if (!response.ok) {
    throw await buildListenAPIError(
      response,
      `listen track metadata failed: ${response.status}`,
    );
  }
  const payload = (await response.json()) as ListenTrackResponseDTO;
  if (!payload.item || !isListenOnlineGroup(payload.item.group)) {
    return null;
  }
  return mapListenRemoteItem(payload.item, `ytmusic-track-${videoId.trim()}`);
}

export async function fetchListenTrackLyrics(
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
  language?: string,
  options: { synced?: boolean } = {},
): Promise<ListenLyricsData> {
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
      : parseListenDurationLabelSeconds(track.durationLabel ?? "");
  if (duration > 0) {
    requestQuery.set("duration", String(Math.round(duration)));
  }
  if (options.synced === false) {
    requestQuery.set("synced", "false");
  }
  appendListenLanguageParam(requestQuery, language);
  const response = await fetch(
    `${baseURL}/api/listen/track/lyrics?${requestQuery.toString()}`,
    {
      method: "GET",
      signal,
    },
  );
  if (!response.ok) {
    throw await buildListenAPIError(
      response,
      `Listen lyrics failed: ${response.status}`,
    );
  }
  const payload = (await response.json()) as ListenLyricsResponseDTO;
  const data = mapListenLyricsResponse(payload, lyricsId);
  return data;
}

export async function fetchListenTrackFavorite(
  httpBaseURL: string,
  videoId: string,
  signal: AbortSignal,
  language?: string,
): Promise<{ liked: boolean; known: boolean }> {
  const baseURL = httpBaseURL.trim().replace(/\/+$/, "");
  if (!baseURL || !videoId.trim()) {
    return { liked: false, known: false };
  }
  const requestQuery = new URLSearchParams({ id: videoId.trim() });
  appendListenLanguageParam(requestQuery, language);
  const response = await fetch(
    `${baseURL}/api/listen/track/favorite?${requestQuery.toString()}`,
    {
      method: "GET",
      signal,
    },
  );
  if (!response.ok) {
    throw await buildListenAPIError(
      response,
      `listen track favorite failed: ${response.status}`,
    );
  }
  const payload = (await response.json()) as ListenTrackFavoriteResponseDTO;
  return {
    liked: payload.liked === true,
    known: payload.known === true,
  };
}

function mapListenLyricsResponse(
  payload: ListenLyricsResponseDTO,
  fallbackVideoId: string,
): ListenLyricsData {
  const kind = normalizeListenLyricsKind(payload.kind);
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
        romanizedText: line.romanizedText?.trim() || undefined,
        romanizedKind: normalizeListenLyricRomanizedKind(line.romanizedKind),
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

function normalizeListenLyricRomanizedKind(
  value: unknown,
): ListenLyricsData["lines"][number]["romanizedKind"] {
  return value === "pinyin" || value === "romanized" ? value : undefined;
}

function normalizeListenLyricsKind(value: string | undefined): ListenLyricsKind {
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

function parseListenDurationLabelSeconds(value: string) {
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

export async function fetchListenTrackFavoriteStatuses(
  httpBaseURL: string,
  videoIds: string[],
  signal: AbortSignal,
  language?: string,
): Promise<Record<string, boolean>> {
  const baseURL = httpBaseURL.trim().replace(/\/+$/, "");
  const ids = Array.from(
    new Set(videoIds.map((item) => item.trim()).filter(Boolean)),
  ).slice(0, 50);
  if (!baseURL || ids.length === 0) {
    return {};
  }
  const requestQuery = new URLSearchParams({ ids: ids.join(",") });
  appendListenLanguageParam(requestQuery, language);
  const response = await fetch(
    `${baseURL}/api/listen/track/favorite?${requestQuery.toString()}`,
    {
      method: "GET",
      signal,
    },
  );
  if (!response.ok) {
    throw await buildListenAPIError(
      response,
      `listen track favorites failed: ${response.status}`,
    );
  }
  const payload = (await response.json()) as ListenTrackFavoriteResponseDTO;
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

export async function updateListenTrackFavorite(
  httpBaseURL: string,
  videoId: string,
  liked: boolean,
  signal: AbortSignal,
): Promise<boolean> {
  const baseURL = httpBaseURL.trim().replace(/\/+$/, "");
  if (!baseURL || !videoId.trim()) {
    return false;
  }
  const response = await fetch(`${baseURL}/api/listen/track/favorite`, {
    method: "POST",
    signal,
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ videoId: videoId.trim(), liked }),
  });
  if (!response.ok) {
    throw await buildListenAPIError(
      response,
      `listen track favorite update failed: ${response.status}`,
    );
  }
  const payload = (await response.json()) as ListenTrackFavoriteResponseDTO;
  return payload.liked === true;
}

export function mapListenRemoteItem(
  item: ListenSearchItemDTO,
  fallbackId: string,
): ListenOnlineItem {
  return {
    id: item.id || fallbackId,
    group: item.group as ListenOnlineGroup,
    source: item.source,
    videoId: item.videoId,
    title: item.title || item.videoId,
    channel: item.channel || "",
    artists: normalizeListenTrackArtists(item.artists),
    artistBrowseId: item.artistBrowseId,
    artistSource: item.artistSource,
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

function normalizeListenTrackArtists(
  artists: ListenTrackArtist[] | undefined,
): ListenTrackArtist[] | undefined {
  if (!Array.isArray(artists) || artists.length === 0) {
    return undefined;
  }
  const result: ListenTrackArtist[] = [];
  const seen = new Set<string>();
  for (const artist of artists) {
    const name = String(artist.name ?? "").trim();
    const browseId = String(artist.browseId ?? "").trim();
    if (!name) {
      continue;
    }
    const key = browseId || name.toLocaleLowerCase();
    if (seen.has(key)) {
      continue;
    }
    seen.add(key);
    result.push({
      name,
      browseId: browseId || undefined,
      thumbnailUrl: String(artist.thumbnailUrl ?? "").trim() || undefined,
    });
  }
  return result.length > 0 ? result : undefined;
}

function mapListenLiveCatalog(payload: ListenLiveCatalogDTO): ListenLiveCatalog {
  const groups = (payload.groups ?? [])
    .map((group, groupIndex): ListenLiveGroup => {
      const groupId = String(group.id || `live-${groupIndex + 1}`).trim();
      const title = String(group.title || groupId).trim();
      const items = dedupeOnlineItems(
        (group.items ?? [])
          .map((item, itemIndex) => {
            const videoId = String(
              item.videoId || item.playback?.videoId || "",
            ).trim();
            return mapListenRemoteItem(
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
    id: String(payload.id || "listen.live.channel").trim(),
    version: String(payload.version || "").trim(),
    updatedAt: String(payload.updatedAt || "").trim(),
    ttlSeconds: normalizeListenSeconds(payload.ttlSeconds, 21600),
    groups,
  };
}

export function mapListenLibraryShelf(
  item: ListenLibraryShelfDTO,
): ListenLibraryShelf {
  const kind = isListenLibraryShelfKind(item.kind) ? item.kind : "tracks";
  return {
    id: item.id || `${kind}:${item.title || "ytmusic"}`,
    title: item.title || "",
    kind,
    continuation: item.continuation?.trim() ?? "",
    browseId: item.browseId?.trim() ?? "",
    params: item.params?.trim() ?? "",
    tracks:
      kind === "tracks"
        ? dedupeOnlineItems(
            (item.tracks ?? [])
              .filter(
                (track) =>
                  isListenOnlineGroup(track.group) && track.videoId.trim(),
              )
              .map((track) =>
                mapListenRemoteItem(track, `ytmusic-home-${track.videoId}`),
              ),
          )
        : [],
    playlists:
      kind === "playlists"
        ? dedupePlaylistItems(
            (item.playlists ?? []).map(mapListenPlaylistItem),
          )
        : [],
    categories:
      kind === "categories"
        ? (item.categories ?? []).map(mapListenCategoryItem)
        : [],
    artists: kind === "artists" ? (item.artists ?? []).map(mapListenArtistItem) : [],
  };
}

export function mapListenPlaylistItem(
  item: ListenPlaylistItemDTO,
): ListenPlaylistItem {
  return {
    id: item.id || item.playlistId,
    playlistId: item.playlistId,
    title: item.title || item.playlistId,
    channel: item.channel || "YouTube Music",
    description: item.description || "",
    thumbnailUrl: item.thumbnailUrl,
  };
}

export function mapListenArtistItem(
  item: ListenArtistItemDTO,
): ListenArtistItem {
  return {
    id: item.id || item.browseId,
    browseId: item.browseId,
    name: item.name || item.browseId,
    subtitle: item.subtitle || "YouTube Music",
    thumbnailUrl: item.thumbnailUrl,
  };
}

export function mapListenCategoryItem(
  item: ListenCategoryItemDTO,
): ListenCategoryItem {
  return {
    id: item.id || [item.browseId, item.params].filter(Boolean).join("_"),
    browseId: item.browseId,
    params: item.params ?? "",
    title: item.title || item.browseId,
    colorHex: item.colorHex,
    thumbnailUrl: item.thumbnailUrl,
  };
}
