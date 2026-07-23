import * as React from "react";

import {
  mapListenLocalTrackDTO,
  normalizeListenHTTPBaseURL,
  type ListenLocalTrackDTO,
} from "@/app/main/listen/local-library";
import type { ListenLocalItem } from "@/app/main/listen/types";

export const LISTEN_LOCAL_PLAYLISTS_CHANGED_EVENT =
  "xiadown:listen-local-playlists-changed";

let listenLocalPlaylistHookInstance = 0;

type ListenLocalPlaylistDTO = {
  id?: string;
  name?: string;
  revision?: number;
  itemCount?: number;
  createdAt?: string;
  updatedAt?: string;
};

type ListenLocalPlaylistItemDTO = {
  id?: string;
  position?: number;
  addedAt?: string;
  track?: ListenLocalTrackDTO;
};

type ListenLocalPlaylistDetailDTO = {
  playlist?: ListenLocalPlaylistDTO;
  items?: ListenLocalPlaylistItemDTO[];
};

export type ListenLocalPlaylist = {
  id: string;
  name: string;
  revision: number;
  itemCount: number;
  createdAt: string;
  updatedAt: string;
};

export type ListenLocalPlaylistItem = {
  id: string;
  position: number;
  addedAt: string;
  track: ListenLocalItem;
};

export type ListenLocalPlaylistDetail = {
  playlist: ListenLocalPlaylist;
  items: ListenLocalPlaylistItem[];
};

export type ListenLocalPlaylistsState = {
  playlists: ListenLocalPlaylist[];
  loading: boolean;
  mutating: boolean;
  reload: () => Promise<void>;
  get: (id: string) => Promise<ListenLocalPlaylistDetail>;
  create: (name: string) => Promise<ListenLocalPlaylist>;
  rename: (
    id: string,
    name: string,
    expectedRevision: number,
  ) => Promise<ListenLocalPlaylist>;
  remove: (id: string, expectedRevision: number) => Promise<void>;
  addTracks: (
    id: string,
    fileIds: string[],
    expectedRevision: number,
  ) => Promise<ListenLocalPlaylistDetail>;
  reorder: (
    id: string,
    itemIds: string[],
    expectedRevision: number,
  ) => Promise<ListenLocalPlaylistDetail>;
  removeTrack: (
    id: string,
    itemId: string,
    expectedRevision: number,
  ) => Promise<ListenLocalPlaylistDetail>;
};

export type ListenLocalPlaylistsClient = {
  list: () => Promise<ListenLocalPlaylist[]>;
  get: (id: string) => Promise<ListenLocalPlaylistDetail>;
  create: (name: string) => Promise<ListenLocalPlaylist>;
  rename: (
    id: string,
    name: string,
    expectedRevision: number,
  ) => Promise<ListenLocalPlaylist>;
  remove: (id: string, expectedRevision: number) => Promise<void>;
  addTracks: (
    id: string,
    fileIds: string[],
    expectedRevision: number,
  ) => Promise<ListenLocalPlaylistDetail>;
  reorder: (
    id: string,
    itemIds: string[],
    expectedRevision: number,
  ) => Promise<ListenLocalPlaylistDetail>;
  removeTrack: (
    id: string,
    itemId: string,
    expectedRevision: number,
  ) => Promise<ListenLocalPlaylistDetail>;
};

export class ListenLocalPlaylistRequestError extends Error {
  constructor(
    message: string,
    readonly status: number,
    readonly code: string,
  ) {
    super(message);
    this.name = "ListenLocalPlaylistRequestError";
  }
}

export function isListenLocalPlaylistRevisionConflict(error: unknown) {
  return (
    error instanceof ListenLocalPlaylistRequestError &&
    error.status === 409 &&
    error.code === "playlist_revision_conflict"
  );
}

type ListenLocalPlaylistsReloadTarget = {
  setPlaylists: (playlists: ListenLocalPlaylist[]) => void;
  setLoading: (loading: boolean) => void;
};

/**
 * Coordinates overlapping list requests so only the newest request may update
 * the observable playlist state. The transport itself may not be abortable, so
 * superseded responses are ignored by generation instead.
 */
export function createListenLocalPlaylistsReloadCoordinator() {
  let generation = 0;

  return {
    invalidate() {
      generation += 1;
    },
    async reload(
      client: Pick<ListenLocalPlaylistsClient, "list"> | null,
      target: ListenLocalPlaylistsReloadTarget,
    ) {
      const requestGeneration = ++generation;
      if (!client) {
        target.setPlaylists([]);
        target.setLoading(false);
        return;
      }

      target.setLoading(true);
      try {
        const nextPlaylists = await client.list();
        if (requestGeneration === generation) {
          target.setPlaylists(nextPlaylists);
        }
      } finally {
        if (requestGeneration === generation) {
          target.setLoading(false);
        }
      }
    },
  };
}

/**
 * Keeps a successful command independent from its best-effort synchronization.
 * A failed refresh must not make callers retry an already-applied mutation.
 */
export async function runListenLocalPlaylistMutation<T>(
  operation: () => Promise<T>,
  refresh: () => Promise<void>,
  notifyChanged: () => void,
) {
  const result = await operation();
  try {
    notifyChanged();
  } catch {
    // Notification is best effort and must not change the command result.
  }
  try {
    void refresh().catch(() => undefined);
  } catch {
    // Also tolerate refresh implementations that throw before returning a promise.
  }
  return result;
}

export function createListenLocalPlaylistsClient(
  httpBaseURL: string,
  fetcher: typeof fetch = fetch,
): ListenLocalPlaylistsClient {
  const baseURL = normalizeListenHTTPBaseURL(httpBaseURL);
  const playlistURL = (id = "") =>
    `${baseURL}/api/listen/local/playlists${
      id ? `/${encodeURIComponent(id)}` : ""
    }`;
  return {
    list: async () => {
      if (!baseURL) {
        return [];
      }
      const payload = await requestJSON<{ items?: ListenLocalPlaylistDTO[] }>(
        playlistURL(),
        {},
        fetcher,
      );
      return (payload.items ?? []).map(mapPlaylist);
    },
    get: async (id) =>
      mapPlaylistDetail(
        await requestJSON<ListenLocalPlaylistDetailDTO>(
          playlistURL(id),
          {},
          fetcher,
        ),
        baseURL,
      ),
    create: async (name) =>
      mapPlaylist(
        await requestJSON<ListenLocalPlaylistDTO>(
          playlistURL(),
          { method: "POST", body: JSON.stringify({ name }) },
          fetcher,
        ),
      ),
    rename: async (id, name, expectedRevision) =>
      mapPlaylist(
        await requestJSON<ListenLocalPlaylistDTO>(
          playlistURL(id),
          { method: "PATCH", body: JSON.stringify({ name, expectedRevision }) },
          fetcher,
        ),
      ),
    remove: (id, expectedRevision) =>
      requestVoid(
        `${playlistURL(id)}?expectedRevision=${encodeURIComponent(expectedRevision)}`,
        { method: "DELETE" },
        fetcher,
      ),
    addTracks: async (id, fileIds, expectedRevision) =>
      mapPlaylistDetail(
        await requestJSON<ListenLocalPlaylistDetailDTO>(
          `${playlistURL(id)}/items`,
          { method: "POST", body: JSON.stringify({ fileIds, expectedRevision }) },
          fetcher,
        ),
        baseURL,
      ),
    reorder: async (id, itemIds, expectedRevision) =>
      mapPlaylistDetail(
        await requestJSON<ListenLocalPlaylistDetailDTO>(
          `${playlistURL(id)}/items`,
          { method: "PUT", body: JSON.stringify({ itemIds, expectedRevision }) },
          fetcher,
        ),
        baseURL,
      ),
    removeTrack: async (id, itemId, expectedRevision) =>
      mapPlaylistDetail(
        await requestJSON<ListenLocalPlaylistDetailDTO>(
          `${playlistURL(id)}/items?itemId=${encodeURIComponent(itemId)}&expectedRevision=${encodeURIComponent(expectedRevision)}`,
          { method: "DELETE" },
          fetcher,
        ),
        baseURL,
      ),
  };
}

export function useListenLocalPlaylists(httpBaseURL: string): ListenLocalPlaylistsState {
  const [playlists, setPlaylists] = React.useState<ListenLocalPlaylist[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [mutationCount, setMutationCount] = React.useState(0);
  const instanceIdRef = React.useRef(0);
  if (instanceIdRef.current === 0) {
    instanceIdRef.current = ++listenLocalPlaylistHookInstance;
  }
  const reloadCoordinatorRef = React.useRef<ReturnType<
    typeof createListenLocalPlaylistsReloadCoordinator
  > | null>(null);
  if (!reloadCoordinatorRef.current) {
    reloadCoordinatorRef.current = createListenLocalPlaylistsReloadCoordinator();
  }
  const baseURL = normalizeListenHTTPBaseURL(httpBaseURL);
  const client = React.useMemo(
    () => createListenLocalPlaylistsClient(baseURL),
    [baseURL],
  );

  const reload = React.useCallback(async () => {
    await reloadCoordinatorRef.current?.reload(baseURL ? client : null, {
      setPlaylists,
      setLoading,
    });
  }, [baseURL, client]);

  React.useEffect(() => {
    void reload().catch(() => undefined);
    return () => reloadCoordinatorRef.current?.invalidate();
  }, [reload]);

  React.useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }
    const handleChange = (event: Event) => {
      const source = (event as CustomEvent<{ source?: number }>).detail?.source;
      if (source === instanceIdRef.current) {
        return;
      }
      void reload().catch(() => undefined);
    };
    window.addEventListener(LISTEN_LOCAL_PLAYLISTS_CHANGED_EVENT, handleChange);
    return () =>
      window.removeEventListener(
        LISTEN_LOCAL_PLAYLISTS_CHANGED_EVENT,
        handleChange,
      );
  }, [reload]);

  const runMutation = React.useCallback(
    async <T,>(operation: () => Promise<T>) => {
      setMutationCount((current) => current + 1);
      try {
        return await runListenLocalPlaylistMutation(
          operation,
          reload,
          () => {
            if (typeof window !== "undefined") {
              window.dispatchEvent(
                new CustomEvent(LISTEN_LOCAL_PLAYLISTS_CHANGED_EVENT, {
                  detail: { source: instanceIdRef.current },
                }),
              );
            }
          },
        );
      } finally {
        setMutationCount((current) => Math.max(0, current - 1));
      }
    },
    [reload],
  );

  const get = React.useCallback(
    (id: string) => client.get(id),
    [client],
  );

  return {
    playlists,
    loading,
    mutating: mutationCount > 0,
    reload,
    get,
    create: (name) =>
      runMutation(() => client.create(name)),
    rename: (id, name, expectedRevision) =>
      runMutation(() => client.rename(id, name, expectedRevision)),
    remove: (id, expectedRevision) =>
      runMutation(() => client.remove(id, expectedRevision)),
    addTracks: (id, fileIds, expectedRevision) =>
      runMutation(() => client.addTracks(id, fileIds, expectedRevision)),
    reorder: (id, itemIds, expectedRevision) =>
      runMutation(() => client.reorder(id, itemIds, expectedRevision)),
    removeTrack: (id, itemId, expectedRevision) =>
      runMutation(() => client.removeTrack(id, itemId, expectedRevision)),
  };
}

function mapPlaylist(input: ListenLocalPlaylistDTO): ListenLocalPlaylist {
  return {
    id: input.id?.trim() ?? "",
    name: input.name?.trim() ?? "",
    revision: finitePositiveInteger(input.revision),
    itemCount: finiteNonNegativeInteger(input.itemCount),
    createdAt: input.createdAt?.trim() ?? "",
    updatedAt: input.updatedAt?.trim() ?? "",
  };
}

function mapPlaylistDetail(
  input: ListenLocalPlaylistDetailDTO,
  baseURL: string,
): ListenLocalPlaylistDetail {
  return {
    playlist: mapPlaylist(input.playlist ?? {}),
    items: (input.items ?? [])
      .filter((item): item is ListenLocalPlaylistItemDTO & { track: ListenLocalTrackDTO } => Boolean(item.track))
      .map((item, index) => {
        const position = finiteNonNegativeInteger(item.position ?? index);
        const track = mapListenLocalTrackDTO(item.track, baseURL);
        return {
          id: item.id?.trim() || `legacy:${position}:${track.id}`,
          position,
          addedAt: item.addedAt?.trim() ?? "",
          track,
        };
      })
      .sort((left, right) => left.position - right.position),
  };
}

function finiteNonNegativeInteger(value?: number) {
  return typeof value === "number" && Number.isFinite(value) && value >= 0
    ? Math.floor(value)
    : 0;
}

function finitePositiveInteger(value?: number) {
  return typeof value === "number" && Number.isFinite(value) && value >= 1
    ? Math.floor(value)
    : 1;
}

async function requestJSON<T>(
  url: string,
  init: RequestInit = {},
  fetcher: typeof fetch = fetch,
): Promise<T> {
  const response = await fetcher(url, {
    ...init,
    headers: {
      Accept: "application/json",
      ...(init.body ? { "Content-Type": "application/json" } : {}),
      ...init.headers,
    },
  });
  if (!response.ok) {
    throw await resolveResponseError(response);
  }
  return (await response.json()) as T;
}

async function requestVoid(
  url: string,
  init: RequestInit,
  fetcher: typeof fetch = fetch,
) {
  const response = await fetcher(url, {
    ...init,
    headers: { Accept: "application/json" },
  });
  if (!response.ok) {
    throw await resolveResponseError(response);
  }
}

async function resolveResponseError(response: Response) {
  try {
    const payload = (await response.json()) as { error?: { code?: string; message?: string } };
    return new ListenLocalPlaylistRequestError(
      payload.error?.message?.trim() || `Local playlist request failed: ${response.status}`,
      response.status,
      payload.error?.code?.trim() ?? "",
    );
  } catch {
    return new ListenLocalPlaylistRequestError(
      `Local playlist request failed: ${response.status}`,
      response.status,
      "",
    );
  }
}
