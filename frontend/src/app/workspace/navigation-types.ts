import type {
  HTMLAttributes,
  KeyboardEventHandler,
  MouseEventHandler,
  ReactNode,
} from "react";

import type { WorkspaceRouteId } from "@/app/workspace/types";

export interface WorkspaceSidebarItemCatalog {
  label: string;
  ariaLabel?: string;
  tooltip?: string;
  icon?: ReactNode;
  badge?: ReactNode;
  disabled?: boolean;
}

export interface WorkspaceSidebarNavigationItem
  extends WorkspaceSidebarItemCatalog {
  routeId: WorkspaceRouteId;
  onContextMenu?: MouseEventHandler<HTMLButtonElement>;
  onKeyDown?: KeyboardEventHandler<HTMLButtonElement>;
}

export interface WorkspaceSidebarChromeSlots {
  header?: ReactNode;
  controlPanel?: ReactNode;
  activity?: ReactNode;
  account?: ReactNode;
  workspaceSwitcher?: ReactNode;
  newAction?: ReactNode;
}

export interface WideWorkspaceSidebarProps
  extends Omit<HTMLAttributes<HTMLElement>, "onNavigate">,
    WorkspaceSidebarChromeSlots {
  activeRouteId?: WorkspaceRouteId | null;
  glass?: boolean;
  onNavigate: (routeId: WorkspaceRouteId) => void;
}

export interface WorkspaceSidebarRegionCatalog {
  sidebarAriaLabel: string;
}

export interface WorkspaceSidebarSectionCatalog {
  label: string;
}

export interface WorkspaceSidebarSection {
  id: string;
  label?: string;
  labelRouteId?: WorkspaceRouteId;
  labelIcon?: ReactNode;
  action?: ReactNode;
  items?: readonly WorkspaceSidebarNavigationItem[];
  content?: ReactNode;
}
