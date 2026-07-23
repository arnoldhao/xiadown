import {
  APP_STATION_LIMIT,
  APP_WORKSPACE_IDS,
  BUILT_IN_APP_STATION_DEFINITIONS,
  DEFAULT_APP_STATIONS,
  type AppStation,
  type AppWorkspaceAction,
  type AppWorkspaceId,
  type AppWorkspaceState,
  type CompanionDestination,
  type PersistedRecord,
  type PersistedValue,
  type WorkspaceLocation,
} from "@/app/workspace/types";

export function createInitialAppWorkspaceState(
  workspaceId: AppWorkspaceId = APP_WORKSPACE_IDS.library,
): AppWorkspaceState {
  return {
    schemaVersion: 2,
    activeWorkspaceId: normalizeWorkspaceIdentifier(workspaceId) || APP_WORKSPACE_IDS.library,
    locations: {},
    stations: DEFAULT_APP_STATIONS.map((station) => ({ ...station })),
    companion: { open: false, destination: null },
  };
}

export function normalizeAppStations(stations: readonly AppStation[] | unknown) {
  const byId = new Map<string, AppStation>();
  if (!Array.isArray(stations)) {
    return [];
  }
  stations.forEach((station, inputIndex) => {
    const normalized = normalizeAppStation(station, inputIndex);
    if (!normalized || byId.has(normalized.id)) {
      return;
    }
    byId.set(normalized.id, normalized);
  });

  let pinnedCount = 0;
  return [...byId.values()]
    .sort((left, right) => left.order - right.order)
    .map((station, order) => {
      const wantsPinned = station.pinned !== false;
      const pinned = wantsPinned && pinnedCount < APP_STATION_LIMIT;
      if (pinned) {
        pinnedCount += 1;
      }
      return {
        ...station,
        order,
        // Five is the Dock's visible-item limit, not a catalog limit. Keep all
        // stations available to the editor and only unpin overflow entries.
        pinned: wantsPinned && !pinned ? false : station.pinned,
      };
    });
}

export function companionDestinationIsAvailable(
  destination: CompanionDestination | null,
  workspaceId: AppWorkspaceId,
  location?: WorkspaceLocation,
) {
  if (!destination || destination.scope.kind === "global") {
    return true;
  }
  if (destination.scope.workspaceId !== workspaceId) {
    return false;
  }
  if (destination.scope.kind === "workspace") {
    return true;
  }
  const routeId = location?.routeId ?? BUILT_IN_APP_STATION_DEFINITIONS.find(
    (station) => station.workspaceId === workspaceId,
  )?.defaultRouteId;
  return destination.scope.routeId === routeId;
}

function persistedValuesEqual(
  left: PersistedValue | undefined,
  right: PersistedValue | undefined,
): boolean {
  if (left === right) {
    return true;
  }
  if (Array.isArray(left) || Array.isArray(right)) {
    return Array.isArray(left) &&
      Array.isArray(right) &&
      left.length === right.length &&
      left.every((value, index) => persistedValuesEqual(value, right[index]));
  }
  if (
    !left ||
    !right ||
    typeof left !== "object" ||
    typeof right !== "object"
  ) {
    return false;
  }
  const leftRecord = left as PersistedRecord;
  const rightRecord = right as PersistedRecord;
  const leftKeys = Object.keys(leftRecord);
  const rightKeys = Object.keys(rightRecord);
  return leftKeys.length === rightKeys.length && leftKeys.every(
    (key) => Object.prototype.hasOwnProperty.call(rightRecord, key) &&
      persistedValuesEqual(leftRecord[key], rightRecord[key]),
  );
}

function companionDestinationsEqual(
  left: CompanionDestination | null,
  right: CompanionDestination,
) {
  if (!left || left.id !== right.id || left.scope.kind !== right.scope.kind) {
    return false;
  }
  if (
    left.scope.kind !== "global" &&
    right.scope.kind !== "global" &&
    left.scope.workspaceId !== right.scope.workspaceId
  ) {
    return false;
  }
  if (
    left.scope.kind === "route" &&
    right.scope.kind === "route" &&
    left.scope.routeId !== right.scope.routeId
  ) {
    return false;
  }
  return persistedValuesEqual(left.context ?? {}, right.context ?? {});
}

export function normalizeAppWorkspaceState(
  candidate: Partial<AppWorkspaceState> | null | undefined,
  fallback = createInitialAppWorkspaceState(),
): AppWorkspaceState {
  const activeWorkspaceId =
    normalizeWorkspaceIdentifier(candidate?.activeWorkspaceId) || fallback.activeWorkspaceId;
  const locations = normalizeLocations(candidate?.locations ?? fallback.locations);
  const rawCompanion = candidate?.companion ?? fallback.companion;
  const destination = normalizeCompanionDestination(rawCompanion.destination);
  const location = locations[activeWorkspaceId];
  const open =
    rawCompanion.open === true &&
    Boolean(destination) &&
    companionDestinationIsAvailable(destination, activeWorkspaceId, location);

  return {
    schemaVersion: 2,
    activeWorkspaceId,
    locations,
    stations: normalizeAppStations(candidate?.stations ?? fallback.stations),
    companion: { open, destination },
  };
}

export function appWorkspaceReducer(
  state: AppWorkspaceState,
  action: AppWorkspaceAction,
): AppWorkspaceState {
  switch (action.type) {
    case "activate-workspace": {
      const workspaceId = normalizeWorkspaceIdentifier(action.workspaceId);
      if (!workspaceId) {
        return state;
      }
      const location = action.location
        ? normalizeWorkspaceLocation(workspaceId, action.location)
        : null;
      const locations = location
        ? { ...state.locations, [workspaceId]: location }
        : state.locations;
      const companionOpen = companionDestinationIsAvailable(
        state.companion.destination,
        workspaceId,
        locations[workspaceId],
      )
        ? state.companion.open
        : false;
      return {
        ...state,
        activeWorkspaceId: workspaceId,
        locations,
        companion: { ...state.companion, open: companionOpen },
      };
    }
    case "navigate": {
      const workspaceId =
        normalizeWorkspaceIdentifier(action.workspaceId) || state.activeWorkspaceId;
      const location = normalizeWorkspaceLocation(workspaceId, action.location);
      if (!location) {
        return state;
      }
      const locations = { ...state.locations, [workspaceId]: location };
      const companionOpen =
        workspaceId === state.activeWorkspaceId &&
        companionDestinationIsAvailable(
          state.companion.destination,
          workspaceId,
          location,
        )
          ? state.companion.open
          : workspaceId === state.activeWorkspaceId
            ? false
            : state.companion.open;
      return {
        ...state,
        locations,
        companion: { ...state.companion, open: companionOpen },
      };
    }
    case "set-stations":
      return { ...state, stations: normalizeAppStations(action.stations) };
    case "upsert-station": {
      const stationId = normalizeIdentifier(action.station?.id);
      const existingIndex = state.stations.findIndex(
        (station) => station.id === stationId,
      );
      const normalizedStation = normalizeAppStation(
        action.station,
        existingIndex >= 0 ? state.stations[existingIndex]?.order ?? existingIndex : state.stations.length,
      );
      if (!normalizedStation) {
        return state;
      }
      const stations = [...state.stations];
      if (existingIndex >= 0) {
        stations[existingIndex] = normalizedStation;
      } else {
        stations.push(normalizedStation);
      }
      return { ...state, stations: normalizeAppStations(stations) };
    }
    case "remove-station": {
      const stationId = normalizeIdentifier(action.stationId);
      const stations = state.stations.filter((station) => station.id !== stationId);
      return stations.length === state.stations.length
        ? state
        : { ...state, stations: normalizeAppStations(stations) };
    }
    case "move-station": {
      const stationId = normalizeIdentifier(action.stationId);
      const sourceIndex = state.stations.findIndex(
        (station) => station.id === stationId,
      );
      if (sourceIndex < 0) {
        return state;
      }
      const stations = [...state.stations];
      const [station] = stations.splice(sourceIndex, 1);
      if (!station) {
        return state;
      }
      const requestedIndex = Number.isFinite(action.toIndex)
        ? Math.trunc(action.toIndex)
        : sourceIndex;
      const targetIndex = Math.max(
        0,
        Math.min(requestedIndex, stations.length),
      );
      stations.splice(targetIndex, 0, station);
      return {
        ...state,
        stations: stations.map((item, order) => ({ ...item, order })),
      };
    }
    case "open-companion": {
      const destination = normalizeCompanionDestination(action.destination);
      if (
        !destination ||
        !companionDestinationIsAvailable(
          destination,
          state.activeWorkspaceId,
          state.locations[state.activeWorkspaceId],
        )
      ) {
        return state;
      }
      return { ...state, companion: { open: true, destination } };
    }
    case "close-companion":
      return {
        ...state,
        companion: {
          open: false,
          destination: action.clearDestination
            ? null
            : state.companion.destination,
        },
      };
    case "toggle-companion": {
      if (action.destination) {
        const destination = normalizeCompanionDestination(action.destination);
        if (
          !destination ||
          !companionDestinationIsAvailable(
            destination,
            state.activeWorkspaceId,
            state.locations[state.activeWorkspaceId],
          )
        ) {
          return state;
        }
        const sameDestination = companionDestinationsEqual(
          state.companion.destination,
          destination,
        );
        return {
          ...state,
          companion: {
            open: sameDestination ? !state.companion.open : true,
            destination,
          },
        };
      }
      if (!state.companion.destination) {
        return state;
      }
      if (
        !companionDestinationIsAvailable(
          state.companion.destination,
          state.activeWorkspaceId,
          state.locations[state.activeWorkspaceId],
        )
      ) {
        return state;
      }
      return {
        ...state,
        companion: { ...state.companion, open: !state.companion.open },
      };
    }
    case "restore":
      return normalizeAppWorkspaceState(action.state, state);
    case "reset":
      return createInitialAppWorkspaceState(
        normalizeWorkspaceIdentifier(action.workspaceId) || APP_WORKSPACE_IDS.library,
      );
    default:
      return state;
  }
}

function normalizeIdentifier(value: unknown) {
  return typeof value === "string" ? value.trim() : "";
}

function normalizeWorkspaceIdentifier(value: unknown) {
  const identifier = normalizeIdentifier(value);
  return identifier === "default" ? APP_WORKSPACE_IDS.library : identifier;
}

function finiteOrder(value: unknown, fallback: number) {
  return typeof value === "number" && Number.isFinite(value)
    ? Math.max(0, Math.trunc(value))
    : fallback;
}

function normalizeAppStation(
  station: AppStation | null | undefined,
  fallbackOrder: number,
): AppStation | null {
  const id = normalizeIdentifier(station?.id);
  const workspaceId = normalizeWorkspaceIdentifier(station?.workspaceId);
  const label = typeof station?.label === "string" ? station.label.trim() : "";
  if (!station || !id || !workspaceId || !label) {
    return null;
  }
  return {
    id,
    workspaceId,
    label,
    iconKey: normalizeIdentifier(station.iconKey) || undefined,
    order: finiteOrder(station.order, fallbackOrder),
    enabled: station.enabled !== false,
    editable: station.editable === true ? true : undefined,
    pinned: typeof station.pinned === "boolean" ? station.pinned : undefined,
    defaultRouteId: normalizeIdentifier(station.defaultRouteId) || undefined,
  };
}

function normalizeLocation(location: WorkspaceLocation): WorkspaceLocation | null {
  const routeId = normalizeIdentifier(location?.routeId);
  if (!routeId) {
    return null;
  }
  return {
    routeId,
    params: location?.params ? { ...location.params } : undefined,
  };
}

function normalizeWorkspaceLocation(
  workspaceId: AppWorkspaceId,
  location: WorkspaceLocation,
) {
  return normalizeLocation(
    workspaceId === APP_WORKSPACE_IDS.library
      ? { ...location, routeId: migrateLegacyLibraryRoute(location.routeId) }
      : location,
  );
}

function normalizeLocations(value: unknown): Record<string, WorkspaceLocation> {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return {};
  }
  const locations: Record<string, WorkspaceLocation> = {};
  Object.entries(value)
    .sort(([left], [right]) => Number(left !== "default") - Number(right !== "default"))
    .forEach(([workspaceId, rawLocation]) => {
    const id = normalizeWorkspaceIdentifier(workspaceId);
    if (!id || !rawLocation || typeof rawLocation !== "object") {
      return;
    }
    const location = rawLocation as WorkspaceLocation;
    const routeId = normalizeIdentifier(location.routeId);
    if (!routeId) {
      return;
    }
    const normalizedLocation = normalizeLocation(
      id === APP_WORKSPACE_IDS.library
        ? { ...location, routeId: migrateLegacyLibraryRoute(routeId) }
        : location,
    );
    if (normalizedLocation) {
      locations[id] = normalizedLocation;
    }
    });
  return locations;
}

function normalizeCompanionDestination(
  destination: CompanionDestination | null | undefined,
): CompanionDestination | null {
  if (!destination || !normalizeIdentifier(destination.id) || !destination.scope) {
    return null;
  }
  if (destination.scope.kind === "global") {
    return {
      id: normalizeIdentifier(destination.id),
      scope: { kind: "global" },
      context: destination.context ? { ...destination.context } : undefined,
    };
  }
  const workspaceId = normalizeWorkspaceIdentifier(destination.scope.workspaceId);
  if (!workspaceId) {
    return null;
  }
  if (destination.scope.kind === "workspace") {
    return {
      id: normalizeIdentifier(destination.id),
      scope: { kind: "workspace", workspaceId },
      context: destination.context ? { ...destination.context } : undefined,
    };
  }
  const routeId =
    workspaceId === APP_WORKSPACE_IDS.library
      ? migrateLegacyLibraryRoute(destination.scope.routeId)
      : normalizeIdentifier(destination.scope.routeId);
  if (destination.scope.kind !== "route" || !routeId) {
    return null;
  }
  return {
    id: normalizeIdentifier(destination.id),
    scope: { kind: "route", workspaceId, routeId },
    context: destination.context ? { ...destination.context } : undefined,
  };
}

function migrateLegacyLibraryRoute(value: unknown) {
  const routeId = normalizeIdentifier(value);
  switch (routeId) {
    case "pending":
    case "tasks":
      return "tasks";
    case "completed":
      return "all";
    case "connections":
      return "app-sessions";
    case "petsGallery":
      return "pet-gallery";
    case "search":
    case "running":
    case "app-sessions":
    case "all":
    case "video":
    case "audio":
    case "books":
    case "images":
    case "others":
    case "pet-gallery":
      return routeId;
    default:
      return "all";
  }
}
