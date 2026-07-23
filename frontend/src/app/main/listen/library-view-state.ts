import type {
  ListenArtistBrowseState,
  ListenArtistItem,
  ListenLibraryShelf,
  ListenMode,
  ListenOnlineBrowseDetail,
  ListenOnlineBrowseSource,
  ListenPlaylistItem,
} from "@/app/main/listen/types";
import { resolveMusicWorkspaceRoute } from "@/app/main/listen/workspace-routes";

export type ListenLibraryPageCacheEntry = {
  playlists: ListenPlaylistItem[];
  artists: ListenArtistItem[];
  shelves: ListenLibraryShelf[];
  continuation: string;
  reloadToken: number;
};

export function resolveListenLibraryPageCacheKey(
  source: ListenOnlineBrowseSource,
  detail: ListenOnlineBrowseDetail | null,
  language: string,
) {
  const locale = language.trim() || "en";
  if (!detail) {
    return `source:${source}:locale:${locale}`;
  }
  return [
    "detail",
    source,
    locale,
    detail.browseId.trim(),
    detail.params.trim(),
  ].join(":");
}

export function isSameListenArtistBrowseIdentity(
  left: Pick<ListenArtistBrowseState, "id" | "name">,
  right: Pick<ListenArtistBrowseState, "id" | "name">,
) {
  return (
    left.id.trim() === right.id.trim() &&
    left.name.trim() === right.name.trim()
  );
}

export type ListenLibraryViewPhase =
  | "disconnected"
  | "loading"
  | "error"
  | "empty"
  | "ready";

export function resolveListenLibraryViewPhase(options: {
  workspaceLayout: boolean | undefined;
  workspaceRouteId: string | undefined;
  mode: ListenMode;
  onlineBrowseSource: ListenOnlineBrowseSource;
  accountConnected: boolean;
  requestReady: boolean;
  settled: boolean;
  loading: boolean;
  error: boolean;
  hasVisibleContent: boolean;
}): ListenLibraryViewPhase {
  const workspaceRoute = options.workspaceLayout
    ? resolveMusicWorkspaceRoute(options.workspaceRouteId)
    : null;
  const targetBrowseSource =
    workspaceRoute?.mode === "muse" ? workspaceRoute.browseSource : undefined;
  const sourcePending =
    targetBrowseSource !== undefined &&
    targetBrowseSource !== options.onlineBrowseSource;
  const modePending =
    workspaceRoute !== null && workspaceRoute.mode !== options.mode;

  if (!options.accountConnected) {
    return "disconnected";
  }

  // The route prop is updated before Listen's route-sync effect commits the
  // matching browse source. Treat that render as navigation, never as a
  // successfully loaded empty page.
  if (
    modePending ||
    sourcePending ||
    !options.requestReady ||
    !options.settled ||
    options.loading
  ) {
    return "loading";
  }
  if (options.error) {
    return "error";
  }
  return options.hasVisibleContent ? "ready" : "empty";
}

export function isListenLibraryRequestReady(options: {
  accountConnected: boolean;
  httpBaseURL: string;
}) {
  return options.accountConnected && options.httpBaseURL.trim() !== "";
}

export function isListenLibraryPageRequestCurrent(options: {
  activeCacheKey: string;
  requestCacheKey: string;
  aborted: boolean;
}) {
  return (
    !options.aborted &&
    options.activeCacheKey !== "" &&
    options.activeCacheKey === options.requestCacheKey
  );
}
