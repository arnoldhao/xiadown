export const APP_STATION_LIMIT = 5;

export const APP_WORKSPACE_IDS = {
  library: "library",
  music: "music",
  sniff: "sniff",
  youtube: "youtube",
  rss: "rss",
} as const;

export type BuiltInAppWorkspaceId =
  (typeof APP_WORKSPACE_IDS)[keyof typeof APP_WORKSPACE_IDS];

/**
 * Product-level workspace identifier. The `app-` prefix deliberately keeps it
 * distinct from the existing library workspace domain.
 */
export type AppWorkspaceId = BuiltInAppWorkspaceId | (string & {});
export type AppStationId = string;
export type WorkspaceRouteId = string;
export type CompanionDestinationId = string;

/** Serializable station configuration; iconKey is resolved by the UI registry. */
export interface AppStation {
  id: AppStationId;
  workspaceId: AppWorkspaceId;
  label: string;
  iconKey?: string;
  order: number;
  enabled: boolean;
  editable?: boolean;
  /** Legacy v1 Dock preference, retained only while persisted state migrates. */
  pinned?: boolean;
  defaultRouteId?: WorkspaceRouteId;
}

/**
 * Product-owned station metadata. These fields are deliberately kept outside
 * persisted preferences so a stale local-storage value cannot rename a
 * built-in station, replace its icon, or redirect its default route.
 */
export interface AppStationDefinition {
  id: AppStationId;
  workspaceId: AppWorkspaceId;
  label: string;
  iconKey: string;
  defaultRouteId: WorkspaceRouteId;
  defaultPinned: boolean;
}

function builtInStationLabel(workspaceId: "library" | "music" | "sniff" | "youtube" | "rss") {
  return `${workspaceId.charAt(0).toUpperCase()}${workspaceId.slice(1)}`;
}

const YOUTUBE_STATION_LABEL = "YouTube";
const RSS_STATION_LABEL = "RSS";

/**
 * Authoritative built-in station catalog. UI code may localize `label`, but it
 * must not accept a user-authored replacement for any metadata in this list.
 */
export const BUILT_IN_APP_STATION_DEFINITIONS: readonly AppStationDefinition[] = [
  {
    id: "library",
    workspaceId: APP_WORKSPACE_IDS.library,
    label: builtInStationLabel(APP_WORKSPACE_IDS.library),
    iconKey: "library",
    defaultRouteId: "all",
    defaultPinned: true,
  },
  {
    id: "music",
    workspaceId: APP_WORKSPACE_IDS.music,
    label: builtInStationLabel(APP_WORKSPACE_IDS.music),
    iconKey: "music",
    defaultRouteId: "home",
    defaultPinned: true,
  },
  {
    id: "sniff",
    workspaceId: APP_WORKSPACE_IDS.sniff,
    label: builtInStationLabel(APP_WORKSPACE_IDS.sniff),
    iconKey: "sniff",
    defaultRouteId: "resources",
    defaultPinned: true,
  },
  {
    id: "rss",
    workspaceId: APP_WORKSPACE_IDS.rss,
    label: RSS_STATION_LABEL,
    iconKey: "rss",
    defaultRouteId: "all",
    defaultPinned: true,
  },
  {
    id: "youtube",
    workspaceId: APP_WORKSPACE_IDS.youtube,
    label: YOUTUBE_STATION_LABEL,
    iconKey: "youtube",
    defaultRouteId: "home",
    defaultPinned: false,
  },
] as const;

export const DEFAULT_APP_STATIONS: readonly AppStation[] = [
  {
    id: "library",
    workspaceId: APP_WORKSPACE_IDS.library,
    label: builtInStationLabel(APP_WORKSPACE_IDS.library),
    iconKey: "library",
    order: 0,
    enabled: true,
  },
  {
    id: "music",
    workspaceId: APP_WORKSPACE_IDS.music,
    label: builtInStationLabel(APP_WORKSPACE_IDS.music),
    iconKey: "music",
    order: 1,
    enabled: true,
    editable: true,
  },
  {
    id: "sniff",
    workspaceId: APP_WORKSPACE_IDS.sniff,
    label: builtInStationLabel(APP_WORKSPACE_IDS.sniff),
    iconKey: "sniff",
    order: 2,
    enabled: true,
    editable: true,
  },
  {
    id: "rss",
    workspaceId: APP_WORKSPACE_IDS.rss,
    label: RSS_STATION_LABEL,
    iconKey: "rss",
    order: 3,
    enabled: true,
    editable: true,
  },
] as const;

export type PersistedScalar = string | number | boolean | null;
export type PersistedValue =
  | PersistedScalar
  | PersistedValue[]
  | { [key: string]: PersistedValue };
export type PersistedRecord = Record<string, PersistedValue>;

export interface WorkspaceLocation {
  routeId: WorkspaceRouteId;
  params?: PersistedRecord;
}

export type CompanionScope =
  | { kind: "global" }
  | { kind: "workspace"; workspaceId: AppWorkspaceId }
  | {
      kind: "route";
      workspaceId: AppWorkspaceId;
      routeId: WorkspaceRouteId;
    };

/**
 * A serializable pointer to content rendered by the companion panel registry.
 * Component instances remain outside persisted state.
 */
export interface CompanionDestination {
  id: CompanionDestinationId;
  scope: CompanionScope;
  context?: PersistedRecord;
}

export interface CompanionState {
  open: boolean;
  destination: CompanionDestination | null;
}

export interface AppWorkspaceState {
  schemaVersion: 2;
  activeWorkspaceId: AppWorkspaceId;
  locations: Record<string, WorkspaceLocation>;
  stations: AppStation[];
  companion: CompanionState;
}

export type AppWorkspaceAction =
  | {
      type: "activate-workspace";
      workspaceId: AppWorkspaceId;
      location?: WorkspaceLocation;
    }
  | {
      type: "navigate";
      workspaceId?: AppWorkspaceId;
      location: WorkspaceLocation;
    }
  | { type: "set-stations"; stations: AppStation[] }
  | { type: "upsert-station"; station: AppStation }
  | { type: "remove-station"; stationId: AppStationId }
  | {
      type: "move-station";
      stationId: AppStationId;
      toIndex: number;
    }
  | {
      type: "open-companion";
      destination: CompanionDestination;
    }
  | { type: "close-companion"; clearDestination?: boolean }
  | {
      type: "toggle-companion";
      destination?: CompanionDestination;
    }
  | { type: "restore"; state: Partial<AppWorkspaceState> }
  | { type: "reset"; workspaceId?: AppWorkspaceId };
