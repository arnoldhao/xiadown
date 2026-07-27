import type {
  WideWorkspaceSidebarProps,
  WorkspaceSidebarItemCatalog,
  WorkspaceSidebarRegionCatalog,
  WorkspaceSidebarSectionCatalog,
} from "@/app/workspace/navigation-types";
import { WorkspaceSidebarNavigation } from "@/app/workspace/WorkspaceSidebarNavigation";
import { cn } from "@/lib/utils";

export const LIBRARY_WORKSPACE_ROUTE_IDS = {
  search: "search",
  running: "running",
  ended: "ended",
  appSessions: "app-sessions",
  all: "all",
  video: "video",
  audio: "audio",
  books: "books",
  images: "images",
  others: "others",
  petGallery: "pet-gallery",
} as const;

export type LibraryWorkspaceRouteId =
  (typeof LIBRARY_WORKSPACE_ROUTE_IDS)[keyof typeof LIBRARY_WORKSPACE_ROUTE_IDS];

export interface LibraryWorkspaceSidebarCatalog
  extends WorkspaceSidebarRegionCatalog {
  sections: {
    library: WorkspaceSidebarSectionCatalog;
    more: WorkspaceSidebarSectionCatalog;
  };
  routes: {
    [Key in keyof typeof LIBRARY_WORKSPACE_ROUTE_IDS]: WorkspaceSidebarItemCatalog;
  };
}

export interface LibraryWorkspaceSidebarProps
  extends WideWorkspaceSidebarProps {
  catalog: LibraryWorkspaceSidebarCatalog;
}

/**
 * Navigation-only shell for the Library workspace. Page rendering, status
 * cards and the information footer remain owned by MainApp so existing
 * Running, App Sessions and Pet Gallery surfaces can be reused during the
 * migration.
 */
export function LibraryWorkspaceSidebar({
  catalog,
  className,
  ...props
}: LibraryWorkspaceSidebarProps) {
  const item = (route: keyof typeof LIBRARY_WORKSPACE_ROUTE_IDS) => ({
    routeId: LIBRARY_WORKSPACE_ROUTE_IDS[route],
    ...catalog.routes[route],
  });

  return (
    <WorkspaceSidebarNavigation
      {...props}
      ariaLabel={catalog.sidebarAriaLabel}
      className={cn("app-library-workspace-sidebar", className)}
      sections={[
        {
          id: "primary",
          items: [item("search"), item("running"), item("ended")],
        },
        {
          id: "library",
          label: catalog.sections.library.label,
          items: [
            item("all"),
            item("video"),
            item("audio"),
            item("books"),
            item("images"),
            item("others"),
          ],
        },
        {
          id: "more",
          label: catalog.sections.more.label,
          items: [item("appSessions"), item("petGallery")],
        },
      ]}
    />
  );
}
