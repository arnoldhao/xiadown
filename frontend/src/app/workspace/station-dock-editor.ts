import { resolveAppStationCatalog } from "@/app/workspace/station-navigation";
import {
  APP_STATION_LIMIT,
  type AppStation,
  type AppStationId,
} from "@/app/workspace/types";

export interface StationDockEditorItem {
  stationId: AppStationId;
  visible: boolean;
}

export interface StationDockEditorValue {
  items: StationDockEditorItem[];
}

export const EMPTY_STATION_DOCK_EDITOR_VALUE: StationDockEditorValue = {
  items: [],
};

function normalizeEditorItems(
  items: readonly StationDockEditorItem[] | null | undefined,
) {
  const normalized: StationDockEditorItem[] = [];
  const seen = new Set<string>();
  let visibleCount = 0;
  for (const item of Array.isArray(items) ? items : []) {
    const stationId =
      typeof item?.stationId === "string" ? item.stationId.trim() : "";
    if (!stationId || seen.has(stationId)) {
      continue;
    }
    seen.add(stationId);
    const visible = item.visible === true && visibleCount < APP_STATION_LIMIT;
    if (visible) {
      visibleCount += 1;
    }
    normalized.push({ stationId, visible });
  }
  return [
    ...normalized.filter((item) => item.visible),
    ...normalized.filter((item) => !item.visible),
  ];
}

/** Builds a controlled Dock draft from the canonical station catalog. */
export function stationsToDockEditorValue(
  stations: readonly AppStation[] | null | undefined,
): StationDockEditorValue {
  const catalog = resolveAppStationCatalog(stations).filter(
    (station) => station.enabled,
  );
  return {
    items: normalizeEditorItems(
      catalog.map((station) => ({
        stationId: station.id,
        visible: station.pinned !== false,
      })),
    ),
  };
}

/**
 * Applies a Dock draft to the complete canonical station catalog in one value.
 * Only `pinned` and `order` are taken from the draft; built-in metadata is
 * restored by `resolveAppStationCatalog` before the result is returned.
 */
export function applyStationDockEditorValue(
  stations: readonly AppStation[] | null | undefined,
  value: StationDockEditorValue,
): AppStation[] {
  const catalog = resolveAppStationCatalog(stations);
  const catalogById = new Map(catalog.map((station) => [station.id, station]));
  const submittedItems = normalizeEditorItems(value?.items).filter(
    (item) => catalogById.get(item.stationId)?.enabled,
  );
  const submittedIds = new Set(submittedItems.map((item) => item.stationId));
  const missingEnabledItems = stationsToDockEditorValue(catalog).items.filter(
    (item) => !submittedIds.has(item.stationId),
  );
  const orderedItems = normalizeEditorItems([
    ...submittedItems,
    ...missingEnabledItems,
  ]);
  const orderedIds = new Set(orderedItems.map((item) => item.stationId));
  const updatedEnabledStations = orderedItems.flatMap((item) => {
    const station = catalogById.get(item.stationId);
    return station
      ? [{ ...station, pinned: item.visible }]
      : [];
  });
  const unavailableStations = catalog.filter(
    (station) => !orderedIds.has(station.id),
  );

  return [...updatedEnabledStations, ...unavailableStations].map(
    (station, order) => ({ ...station, order }),
  );
}

export function countVisibleStationDockItems(value: StationDockEditorValue) {
  return normalizeEditorItems(value?.items).filter((item) => item.visible)
    .length;
}

/** Toggles visibility and keeps visible stations together in Dock order. */
export function setStationDockEditorItemVisible(
  value: StationDockEditorValue,
  stationId: AppStationId,
  visible: boolean,
): StationDockEditorValue {
  const normalized = normalizeEditorItems(value?.items);
  const target = normalized.find((item) => item.stationId === stationId);
  if (!target || target.visible === visible) {
    return { items: normalized };
  }
  if (
    visible &&
    normalized.filter((item) => item.visible).length >= APP_STATION_LIMIT
  ) {
    return { items: normalized };
  }
  const remaining = normalized.filter((item) => item.stationId !== stationId);
  const visibleItems = remaining.filter((item) => item.visible);
  const hiddenItems = remaining.filter((item) => !item.visible);
  return {
    items: visible
      ? [...visibleItems, { ...target, visible: true }, ...hiddenItems]
      : [...visibleItems, { ...target, visible: false }, ...hiddenItems],
  };
}

/** Moves a visible station by one position without reordering hidden choices. */
export function moveStationDockEditorItem(
  value: StationDockEditorValue,
  stationId: AppStationId,
  direction: -1 | 1,
): StationDockEditorValue {
  const normalized = normalizeEditorItems(value?.items);
  const visibleItems = normalized.filter((item) => item.visible);
  const hiddenItems = normalized.filter((item) => !item.visible);
  const sourceIndex = visibleItems.findIndex(
    (item) => item.stationId === stationId,
  );
  const targetIndex = sourceIndex + direction;
  if (
    sourceIndex < 0 ||
    targetIndex < 0 ||
    targetIndex >= visibleItems.length
  ) {
    return { items: normalized };
  }
  const nextVisibleItems = [...visibleItems];
  const [item] = nextVisibleItems.splice(sourceIndex, 1);
  if (!item) {
    return { items: normalized };
  }
  nextVisibleItems.splice(targetIndex, 0, item);
  return { items: [...nextVisibleItems, ...hiddenItems] };
}
