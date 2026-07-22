import type { ListenLocalItem } from "@/app/main/listen/types";

export const LISTEN_LOCAL_WORKSPACE_ROUTE_IDS = {
  home: "local-home",
  search: "local-search",
  recentlyAdded: "recently-added",
  artists: "artists",
  albums: "albums",
  songs: "songs",
} as const;

export type ListenLocalWorkspaceRoute =
  | (typeof LISTEN_LOCAL_WORKSPACE_ROUTE_IDS)[keyof typeof LISTEN_LOCAL_WORKSPACE_ROUTE_IDS]
  | `playlist:${string}`;

export type ParsedListenLocalWorkspaceRoute =
  | { kind: "home" | "search" | "recently-added" | "artists" | "albums" | "songs" }
  | { kind: "playlist"; playlistId: string };

export type ListenLocalCollectionGroup = {
  id: string;
  title: string;
  subtitle: string;
  coverURL: string;
  tracks: ListenLocalItem[];
};

export function parseListenLocalWorkspaceRoute(
  routeId: string | null | undefined,
): ParsedListenLocalWorkspaceRoute | null {
  const route = routeId?.trim() ?? "";
  if (route === LISTEN_LOCAL_WORKSPACE_ROUTE_IDS.home) {
    return { kind: "home" };
  }
  if (route === LISTEN_LOCAL_WORKSPACE_ROUTE_IDS.search) {
    return { kind: "search" };
  }
  if (
    route === LISTEN_LOCAL_WORKSPACE_ROUTE_IDS.recentlyAdded ||
    route === LISTEN_LOCAL_WORKSPACE_ROUTE_IDS.artists ||
    route === LISTEN_LOCAL_WORKSPACE_ROUTE_IDS.albums ||
    route === LISTEN_LOCAL_WORKSPACE_ROUTE_IDS.songs
  ) {
    return { kind: route };
  }
  if (!route.startsWith("playlist:")) {
    return null;
  }
  const playlistId = route.slice("playlist:".length).trim();
  return playlistId ? { kind: "playlist", playlistId } : null;
}

export function filterListenLocalWorkspaceTracks(
  tracks: readonly ListenLocalItem[],
  query: string,
) {
  const normalizedQuery = query.trim().toLocaleLowerCase();
  if (!normalizedQuery) {
    return [...tracks];
  }
  return tracks.filter((track) =>
    [
      track.title,
      track.author,
      track.album,
      track.albumArtist,
      track.genre,
      track.path,
    ].some((value) => value.toLocaleLowerCase().includes(normalizedQuery)),
  );
}

export function sortListenLocalTracksByRecent(
  tracks: readonly ListenLocalItem[],
) {
  return stableSort(tracks, (left, right) => {
    const timeDifference =
      resolveListenLocalAddedTimestamp(right) -
      resolveListenLocalAddedTimestamp(left);
    return timeDifference || compareText(left.title, right.title);
  });
}

export function sortListenLocalSongs(tracks: readonly ListenLocalItem[]) {
  return stableSort(
    tracks,
    (left, right) =>
      compareText(left.title, right.title) ||
      compareText(left.author, right.author),
  );
}

export function buildListenLocalArtistGroups(
  tracks: readonly ListenLocalItem[],
  unknownArtistLabel: string,
) {
  return buildGroups(
    tracks,
    (track) => track.albumArtist.trim() || track.author.trim(),
    unknownArtistLabel,
    (groupTracks) => {
      const albums = uniqueNonEmpty(groupTracks.map((track) => track.album));
      return albums.join(" · ");
    },
    compareArtistTracks,
  );
}

export function buildListenLocalAlbumGroups(
  tracks: readonly ListenLocalItem[],
  unknownAlbumLabel: string,
) {
  const grouped = new Map<string, ListenLocalCollectionGroup>();
  tracks.forEach((track) => {
    const title = track.album.trim() || unknownAlbumLabel;
    const artist = track.albumArtist.trim() || track.author.trim();
    const id = `album:${normalizeGroupKey(artist)}:${normalizeGroupKey(title)}`;
    const existing = grouped.get(id);
    if (existing) {
      existing.tracks.push(track);
      return;
    }
    grouped.set(id, {
      id,
      title,
      subtitle: artist,
      coverURL: track.coverURL,
      tracks: [track],
    });
  });
  return [...grouped.values()]
    .map((group) => ({
      ...group,
      tracks: stableSort(group.tracks, compareAlbumTracks),
    }))
    .sort(
      (left, right) =>
        compareText(left.title, right.title) ||
        compareText(left.subtitle, right.subtitle),
    );
}

export function moveListenLocalPlaylistTrack(
  fileIds: readonly string[],
  fileId: string,
  direction: -1 | 1,
) {
  const sourceIndex = fileIds.indexOf(fileId);
  const targetIndex = sourceIndex + direction;
  if (
    sourceIndex < 0 ||
    targetIndex < 0 ||
    targetIndex >= fileIds.length
  ) {
    return [...fileIds];
  }
  const result = [...fileIds];
  const [item] = result.splice(sourceIndex, 1);
  if (!item) {
    return result;
  }
  result.splice(targetIndex, 0, item);
  return result;
}

export function resolveListenLocalPlaybackQueue(
  tracks: readonly ListenLocalItem[],
  queueIds: readonly string[] | null,
) {
  if (queueIds === null) {
    return [...tracks];
  }
  const tracksById = new Map(tracks.map((track) => [track.id, track]));
  const queue = queueIds
    .map((id) => tracksById.get(id))
    .filter((track): track is ListenLocalItem => Boolean(track));
  return queue;
}

export function buildListenLocalPlaybackQueueIds(
  selectedId: string,
  queue: ReadonlyArray<{ id: string }>,
) {
  const normalizedSelectedId = selectedId.trim();
  const queueIds = Array.from(
    new Set(queue.map((track) => track.id.trim()).filter(Boolean)),
  );
  if (normalizedSelectedId && !queueIds.includes(normalizedSelectedId)) {
    queueIds.unshift(normalizedSelectedId);
  }
  return queueIds;
}

function resolveListenLocalAddedTimestamp(track: ListenLocalItem) {
  return track.createdAtUnix > 0 ? track.createdAtUnix : track.modTimeUnix;
}

function buildGroups(
  tracks: readonly ListenLocalItem[],
  getTitle: (track: ListenLocalItem) => string,
  fallbackTitle: string,
  getSubtitle: (tracks: ListenLocalItem[]) => string,
  compareTracks: (left: ListenLocalItem, right: ListenLocalItem) => number,
) {
  const grouped = new Map<string, ListenLocalCollectionGroup>();
  tracks.forEach((track) => {
    const title = getTitle(track).trim() || fallbackTitle;
    const id = `artist:${normalizeGroupKey(title)}`;
    const existing = grouped.get(id);
    if (existing) {
      existing.tracks.push(track);
      return;
    }
    grouped.set(id, {
      id,
      title,
      subtitle: "",
      coverURL: track.coverURL,
      tracks: [track],
    });
  });
  return [...grouped.values()]
    .map((group) => {
      const sortedTracks = stableSort(group.tracks, compareTracks);
      return {
        ...group,
        subtitle: getSubtitle(sortedTracks),
        coverURL:
          sortedTracks.find((track) => track.coverURL.trim())?.coverURL ?? "",
        tracks: sortedTracks,
      };
    })
    .sort((left, right) => compareText(left.title, right.title));
}

function compareArtistTracks(left: ListenLocalItem, right: ListenLocalItem) {
  return (
    compareText(left.album, right.album) ||
    compareAlbumTracks(left, right)
  );
}

function compareAlbumTracks(left: ListenLocalItem, right: ListenLocalItem) {
  return (
    compareNumber(left.discNumber, right.discNumber) ||
    compareNumber(left.trackNumber, right.trackNumber) ||
    compareText(left.title, right.title)
  );
}

function compareNumber(left: number, right: number) {
  const normalizedLeft = left > 0 ? left : Number.MAX_SAFE_INTEGER;
  const normalizedRight = right > 0 ? right : Number.MAX_SAFE_INTEGER;
  return normalizedLeft - normalizedRight;
}

function compareText(left: string, right: string) {
  return left.localeCompare(right, undefined, {
    numeric: true,
    sensitivity: "base",
  });
}

function normalizeGroupKey(value: string) {
  return value.trim().toLocaleLowerCase().replace(/\s+/g, " ");
}

function uniqueNonEmpty(values: readonly string[]) {
  const seen = new Set<string>();
  const result: string[] = [];
  values.forEach((value) => {
    const trimmed = value.trim();
    const key = trimmed.toLocaleLowerCase();
    if (!trimmed || seen.has(key)) {
      return;
    }
    seen.add(key);
    result.push(trimmed);
  });
  return result;
}

function stableSort(
  tracks: readonly ListenLocalItem[],
  compare: (left: ListenLocalItem, right: ListenLocalItem) => number,
) {
  return tracks
    .map((track, index) => ({ track, index }))
    .sort(
      (left, right) =>
        compare(left.track, right.track) || left.index - right.index,
    )
    .map(({ track }) => track);
}
