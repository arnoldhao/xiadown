import {
  APP_WORKSPACE_IDS,
  BUILT_IN_APP_STATION_DEFINITIONS,
  type AppStation,
  type AppStationDefinition,
} from "@/app/workspace/types";

const WORKSPACE_SWITCH_ORDER = new Map<string, number>([
  [APP_WORKSPACE_IDS.library, 0],
  [APP_WORKSPACE_IDS.sniff, 1],
  [APP_WORKSPACE_IDS.music, 2],
  [APP_WORKSPACE_IDS.youtube, 3],
  [APP_WORKSPACE_IDS.rss, 4],
]);

function finiteStationOrder(value: unknown, fallback: number) {
  return typeof value === "number" && Number.isFinite(value)
    ? Math.max(0, Math.trunc(value))
    : fallback;
}

function stationHasIdentity(station: AppStation | null | undefined) {
  return Boolean(
    station &&
      typeof station.id === "string" &&
      station.id.trim() &&
      typeof station.workspaceId === "string" &&
      station.workspaceId.trim(),
  );
}

function canonicalBuiltInStation(
  definition: AppStationDefinition,
  persisted: AppStation | undefined,
  fallbackOrder: number,
): AppStation {
  return {
    id: definition.id,
    workspaceId: definition.workspaceId,
    label: definition.label,
    iconKey: definition.iconKey,
    order: finiteStationOrder(persisted?.order, fallbackOrder),
    enabled: persisted?.enabled !== false,
    pinned: persisted ? persisted.pinned !== false : definition.defaultPinned,
    defaultRouteId: definition.defaultRouteId,
  };
}

/**
 * Resolves the persisted station preferences against the product-owned
 * catalog. Built-in identity, labels, icons and routes always come from the
 * registry; persisted state contributes only availability and Dock
 * preferences. Missing built-ins are appended, which upgrades old
 * music/sniff-only state with an unpinned YouTube station automatically.
 *
 * Unknown stations are retained for forward compatibility. Their metadata is
 * assumed to have been supplied by the feature that registered them.
 */
export function resolveAppStationCatalog(
  stations: readonly AppStation[] | null | undefined,
  definitions: readonly AppStationDefinition[] =
    BUILT_IN_APP_STATION_DEFINITIONS,
): AppStation[] {
  const persistedStations = Array.isArray(stations)
    ? stations.filter(stationHasIdentity)
    : [];
  const persistedById = new Map(
    persistedStations.map((station) => [station.id.trim(), station] as const),
  );
  const consumedIds = new Set<string>();
  let nextOrder =
    persistedStations.reduce(
      (maximum, station) =>
        Math.max(maximum, finiteStationOrder(station.order, maximum)),
      -1,
    ) + 1;

  const builtIns = definitions.map((definition) => {
    const persisted =
      persistedById.get(definition.id) ??
      persistedStations.find(
        (station) => station.workspaceId === definition.workspaceId,
      );
    if (persisted) {
      consumedIds.add(persisted.id.trim());
    }
    const fallbackOrder = nextOrder;
    if (!persisted) {
      nextOrder += 1;
    }
    return canonicalBuiltInStation(definition, persisted, fallbackOrder);
  });

  const externalStations = persistedStations
    .filter((station) => !consumedIds.has(station.id.trim()))
    .map((station, index) => ({
      ...station,
      id: station.id.trim(),
      workspaceId: station.workspaceId.trim(),
      label: station.label.trim(),
      iconKey: station.iconKey?.trim() || undefined,
      order: finiteStationOrder(station.order, nextOrder + index),
      enabled: station.enabled !== false,
      pinned: station.pinned !== false,
      defaultRouteId: station.defaultRouteId?.trim() || undefined,
    }))
    .filter((station) => station.label.length > 0);

  return [...builtIns, ...externalStations]
    .sort((left, right) => left.order - right.order)
    .map((station, order) => ({ ...station, order }));
}

/** Station targets shown by every wide workspace footer. */
export function resolveWorkspaceSwitchStations(
  stations: readonly AppStation[],
): AppStation[] {
  return resolveAppStationCatalog(stations)
    .filter((station) => station.enabled)
    .sort((left, right) => {
      const leftOrder = WORKSPACE_SWITCH_ORDER.get(left.workspaceId);
      const rightOrder = WORKSPACE_SWITCH_ORDER.get(right.workspaceId);
      if (leftOrder !== undefined || rightOrder !== undefined) {
        return (leftOrder ?? Number.MAX_SAFE_INTEGER) -
          (rightOrder ?? Number.MAX_SAFE_INTEGER);
      }
      return left.order - right.order;
    });
}
