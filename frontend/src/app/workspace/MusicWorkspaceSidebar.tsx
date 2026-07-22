import type { ReactNode } from "react";

import type {
  WideWorkspaceSidebarProps,
  WorkspaceSidebarItemCatalog,
  WorkspaceSidebarNavigationItem,
  WorkspaceSidebarRegionCatalog,
  WorkspaceSidebarSectionCatalog,
} from "@/app/workspace/navigation-types";
import { WorkspaceSidebarNavigation } from "@/app/workspace/WorkspaceSidebarNavigation";
import { cn } from "@/lib/utils";

export const MUSIC_WORKSPACE_ROUTE_IDS = {
  search: "search",
  home: "home",
  radio: "radio",
  newReleases: "new-releases",
  charts: "charts",
  moods: "moods",
  podcasts: "podcasts",
  recent: "recent",
  history: "history",
  onlinePlaylists: "online-playlists",
  localSearch: "local-search",
  localHome: "local-home",
  recentlyAdded: "recently-added",
  artists: "artists",
  albums: "albums",
  songs: "songs",
} as const;

export type MusicWorkspaceStaticRouteId =
  (typeof MUSIC_WORKSPACE_ROUTE_IDS)[keyof typeof MUSIC_WORKSPACE_ROUTE_IDS];

export type MusicWorkspaceScope = "online" | "local";

export interface MusicWorkspaceSidebarCatalog
  extends WorkspaceSidebarRegionCatalog {
  sections: {
    explore: WorkspaceSidebarSectionCatalog;
    library: WorkspaceSidebarSectionCatalog;
    playlists: WorkspaceSidebarSectionCatalog;
  };
  routes: {
    [Key in keyof typeof MUSIC_WORKSPACE_ROUTE_IDS]: WorkspaceSidebarItemCatalog;
  };
}

export interface MusicWorkspaceSidebarProps extends WideWorkspaceSidebarProps {
  catalog: MusicWorkspaceSidebarCatalog;
  scope: MusicWorkspaceScope;
  /** @deprecated Playlist routes remain valid, but are no longer listed here. */
  playlistItems?: readonly WorkspaceSidebarNavigationItem[];
  /** @deprecated Playlist routes remain valid, but are no longer listed here. */
  playlistsSlot?: ReactNode;
}

export function MusicWorkspaceSidebar({
  catalog,
  scope,
  playlistItems,
  playlistsSlot,
  className,
  ...props
}: MusicWorkspaceSidebarProps) {
  // Keep accepting the previous injection API while route-specific playlist
  // content moves into the primary page.
  void playlistItems;
  void playlistsSlot;

  const item = (
    route: keyof typeof MUSIC_WORKSPACE_ROUTE_IDS,
  ): WorkspaceSidebarNavigationItem => ({
    routeId: MUSIC_WORKSPACE_ROUTE_IDS[route],
    ...catalog.routes[route],
  });

  const sections =
    scope === "local"
      ? [
          {
            id: "primary",
            items: [item("localSearch"), item("localHome")],
          },
          {
            id: "library",
            label: catalog.sections.library.label,
            items: [
              item("recentlyAdded"),
              item("artists"),
              item("albums"),
              item("songs"),
            ],
          },
        ]
      : [
          {
            id: "primary",
            items: [item("search"), item("home"), item("radio")],
          },
          {
            id: "explore",
            label: catalog.sections.explore.label,
            items: [
              item("newReleases"),
              item("charts"),
              item("moods"),
              item("podcasts"),
            ],
          },
          {
            id: "library",
            label: catalog.sections.library.label,
            items: [
              item("recent"),
              item("history"),
              item("onlinePlaylists"),
            ],
          },
        ];

  return (
    <WorkspaceSidebarNavigation
      {...props}
      ariaLabel={catalog.sidebarAriaLabel}
      className={cn("app-music-workspace-sidebar", className)}
      sections={sections}
    />
  );
}
