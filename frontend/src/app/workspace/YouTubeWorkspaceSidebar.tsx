import type {
  WideWorkspaceSidebarProps,
  WorkspaceSidebarItemCatalog,
  WorkspaceSidebarNavigationItem,
  WorkspaceSidebarRegionCatalog,
  WorkspaceSidebarSectionCatalog,
} from "@/app/workspace/navigation-types";
import { WorkspaceSidebarNavigation } from "@/app/workspace/WorkspaceSidebarNavigation";
import { cn } from "@/lib/utils";

export const YOUTUBE_WORKSPACE_ROUTE_IDS = {
  search: "search",
  home: "home",
  subscriptions: "subscriptions",
  explore: "explore",
  shorts: "shorts",
  likedVideos: "liked-videos",
  watchLater: "watch-later",
  playlists: "playlists",
  history: "history",
} as const;

export type YouTubeWorkspaceStaticRouteId =
  (typeof YOUTUBE_WORKSPACE_ROUTE_IDS)[keyof typeof YOUTUBE_WORKSPACE_ROUTE_IDS];

export interface YouTubeWorkspaceSidebarCatalog
  extends WorkspaceSidebarRegionCatalog {
  sections: {
    discover: WorkspaceSidebarSectionCatalog;
    collections: WorkspaceSidebarSectionCatalog;
  };
  routes: {
    [Key in keyof typeof YOUTUBE_WORKSPACE_ROUTE_IDS]: WorkspaceSidebarItemCatalog;
  };
}

export interface YouTubeWorkspaceSidebarProps
  extends WideWorkspaceSidebarProps {
  catalog: YouTubeWorkspaceSidebarCatalog;
}

export function YouTubeWorkspaceSidebar({
  catalog,
  className,
  ...props
}: YouTubeWorkspaceSidebarProps) {
  const item = (
    route: keyof typeof YOUTUBE_WORKSPACE_ROUTE_IDS,
  ): WorkspaceSidebarNavigationItem => ({
    routeId: YOUTUBE_WORKSPACE_ROUTE_IDS[route],
    ...catalog.routes[route],
  });

  return (
    <WorkspaceSidebarNavigation
      {...props}
      ariaLabel={catalog.sidebarAriaLabel}
      className={cn("app-youtube-workspace-sidebar", className)}
      sections={[
        {
          id: "primary",
          items: [item("search"), item("home"), item("subscriptions")],
        },
        {
          id: "discover",
          label: catalog.sections.discover.label,
          items: [item("explore"), item("shorts")],
        },
        {
          id: "collections",
          label: catalog.sections.collections.label,
          items: [
            item("likedVideos"),
            item("watchLater"),
            item("playlists"),
            item("history"),
          ],
        },
      ]}
    />
  );
}
