import type { ReactNode } from "react";

import type {
  WideWorkspaceSidebarProps,
  WorkspaceSidebarItemCatalog,
  WorkspaceSidebarRegionCatalog,
  WorkspaceSidebarSectionCatalog,
} from "@/app/workspace/navigation-types";
import { WorkspaceSidebarNavigation } from "@/app/workspace/WorkspaceSidebarNavigation";
import { cn } from "@/lib/utils";
import type { Pet } from "@/shared/contracts/pets";
import { PetDisplay } from "@/shared/ui/pet-player";

export interface SniffWorkspaceSidebarCatalog
  extends WorkspaceSidebarRegionCatalog {
  sections: {
    types: WorkspaceSidebarSectionCatalog;
    sources: WorkspaceSidebarSectionCatalog;
    resources: WorkspaceSidebarSectionCatalog;
  };
  /** @deprecated Search is now a persistent filter rather than a route. */
  routes?: {
    search?: WorkspaceSidebarItemCatalog;
  };
}

export interface SniffWorkspaceSidebarProps extends WideWorkspaceSidebarProps {
  catalog: SniffWorkspaceSidebarCatalog;
  filtersVisible: boolean;
  pet: Pet | null;
  petImageURL: string;
  waitingLabel: string;
  typesFilter?: ReactNode;
  sourcesFilter?: ReactNode;
  resourcesFilter?: ReactNode;
  searchControl?: ReactNode;
}

export function SniffWorkspaceSidebar({
  catalog,
  filtersVisible,
  pet,
  petImageURL,
  waitingLabel,
  typesFilter,
  sourcesFilter,
  resourcesFilter,
  searchControl,
  activeRouteId,
  className,
  ...props
}: SniffWorkspaceSidebarProps) {
  const filtersDisabled = !filtersVisible;
  return (
    <WorkspaceSidebarNavigation
      {...props}
      activeRouteId={activeRouteId}
      ariaLabel={catalog.sidebarAriaLabel}
      bottomOrder="control-first"
      className={cn("app-sniff-workspace-sidebar", className)}
      data-filters-available={filtersVisible ? "true" : "false"}
      sections={[
        {
          id: "search",
          content: (
            <SniffWorkspaceFilterSlot disabled={filtersDisabled}>
              {searchControl}
            </SniffWorkspaceFilterSlot>
          ),
        },
        {
          id: "types",
          label: catalog.sections.types.label,
          content: (
            <SniffWorkspaceFilterSlot disabled={filtersDisabled}>
              {typesFilter}
            </SniffWorkspaceFilterSlot>
          ),
        },
        {
          id: "sources",
          label: catalog.sections.sources.label,
          content: (
            <SniffWorkspaceFilterSlot disabled={filtersDisabled}>
              {sourcesFilter}
            </SniffWorkspaceFilterSlot>
          ),
        },
        {
          id: "resources",
          label: catalog.sections.resources.label,
          content: (
            <SniffWorkspaceFilterSlot disabled={filtersDisabled}>
              {resourcesFilter}
            </SniffWorkspaceFilterSlot>
          ),
        },
        ...(!filtersVisible
          ? [
              {
                id: "waiting",
                content: (
                  <div
                    className="app-sniff-workspace-sidebar__waiting"
                    role="status"
                  >
                    <PetDisplay
                      pet={pet}
                      imageUrl={petImageURL}
                      animation="idle"
                      alt=""
                      size={72}
                      className="app-sniff-workspace-sidebar__waiting-pet"
                      glowClassName="app-sniff-workspace-sidebar__waiting-pet-glow"
                    />
                    <span>{waitingLabel}</span>
                  </div>
                ),
              },
            ]
          : []),
      ]}
    />
  );
}

function SniffWorkspaceFilterSlot(props: {
  children?: ReactNode;
  disabled: boolean;
}) {
  return (
    <fieldset
      className="app-sniff-workspace-sidebar__filter-slot"
      disabled={props.disabled}
    >
      {props.children}
    </fieldset>
  );
}
