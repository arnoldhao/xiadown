import { LISTEN_LIKED_SONGS_SHELF_ID } from "@/app/main/listen/catalog";
import type {
  ListenLibraryShelf,
  ListenMode,
  ListenOnlineBrowseSource,
} from "@/app/main/listen/types";

export const LISTEN_WORKSPACE_SEARCH_MODE_ORDER: readonly ListenMode[] = [
  "muse",
  "hush",
  "linger",
];

export type MusicWorkspaceScope = "online" | "local";

export type MusicWorkspaceRouteDescriptor = {
  scope: MusicWorkspaceScope;
  mode: ListenMode;
  browseSource?: ListenOnlineBrowseSource;
  content:
    | "browse"
    | "radio"
    | "search"
    | "playlists"
    | "local-home"
    | "local-search"
    | "local-library";
};

export function resolveMusicWorkspaceScopeRoute(
  scope: MusicWorkspaceScope,
  rememberedRouteId: string | null | undefined,
) {
  const rememberedRoute = rememberedRouteId?.trim() ?? "";
  if (
    rememberedRoute &&
    resolveMusicWorkspaceRoute(rememberedRoute).scope === scope
  ) {
    return rememberedRoute;
  }
  return scope === "local" ? "local-home" : "home";
}

export function resolveListenWorkspaceViewMode(options: {
  workspaceLayout: boolean | undefined;
  workspaceRouteId: string | undefined;
  fallbackMode: ListenMode;
}) {
  return options.workspaceLayout
    ? resolveMusicWorkspaceRoute(options.workspaceRouteId).mode
    : options.fallbackMode;
}

export function shouldLoadListenWorkspaceBrowse(options: {
  active: boolean;
  viewMode: ListenMode;
  targetMode: "muse" | "hush";
}) {
  return options.active && options.viewMode === options.targetMode;
}

const LISTEN_WORKSPACE_BROWSE_SOURCE_BY_ROUTE: Readonly<
  Partial<Record<string, ListenOnlineBrowseSource>>
> = {
  home: "home",
  explore: "explore",
  charts: "charts",
  moods: "moods",
  "new-releases": "new",
  podcasts: "podcasts",
  recent: "recent",
  history: "history",
  "online-playlists": "playlists",
  "liked-music": "home",
};

export function resolveMusicWorkspaceRoute(
  routeId: string | null | undefined,
): MusicWorkspaceRouteDescriptor {
  const route = routeId?.trim() || "home";
  if (route === "radio") {
    return { scope: "online", mode: "hush", content: "radio" };
  }
  if (route === "search") {
    return {
      scope: "online",
      mode: "muse",
      browseSource: "home",
      content: "search",
    };
  }
  if (route === "online-playlists") {
    return {
      scope: "online",
      mode: "muse",
      browseSource: "playlists",
      content: "playlists",
    };
  }
  if (route === "local-search") {
    return { scope: "local", mode: "linger", content: "local-search" };
  }
  if (route === "local-home") {
    return { scope: "local", mode: "linger", content: "local-home" };
  }
  if (
    route === "recently-added" ||
    route === "artists" ||
    route === "albums" ||
    route === "songs" ||
    route.startsWith("playlist:")
  ) {
    return { scope: "local", mode: "linger", content: "local-library" };
  }
  return {
    scope: "online",
    mode: "muse",
    browseSource: resolveListenWorkspaceBrowseSource(route) ?? "home",
    content: "browse",
  };
}

export function isListenWorkspaceOnlinePlaylistsRoute(
  workspaceLayout: boolean | undefined,
  routeId: string | undefined,
) {
  return workspaceLayout === true && routeId?.trim() === "online-playlists";
}

export function resolveListenWorkspaceBrowseSource(
  routeId: string | undefined,
): ListenOnlineBrowseSource | undefined {
  return LISTEN_WORKSPACE_BROWSE_SOURCE_BY_ROUTE[routeId?.trim() ?? ""];
}

export function isListenWorkspaceLikedMusicRoute(
  workspaceLayout: boolean | undefined,
  routeId: string | undefined,
) {
  return workspaceLayout === true && routeId?.trim() === "liked-music";
}

export function selectListenWorkspaceHomeShelves(
  shelves: ListenLibraryShelf[],
  workspaceLayout: boolean | undefined,
  routeId: string | undefined,
): ListenLibraryShelf[] {
  if (!isListenWorkspaceLikedMusicRoute(workspaceLayout, routeId)) {
    return shelves;
  }
  return shelves.filter((shelf) => shelf.id === LISTEN_LIKED_SONGS_SHELF_ID);
}
