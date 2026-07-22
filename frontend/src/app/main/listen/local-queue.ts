export type ListenLocalQueueSnapshot = {
  queueIds: string[] | null;
  selectedId: string;
};

export type ListenLocalQueueEditResult = ListenLocalQueueSnapshot & {
  changed: boolean;
};

type ListenLocalQueueItem = { id: string };

export function shouldClearListenLocalSelection(options: {
  selectedId: string;
  loading: boolean;
  error: string;
  playableIds: ReadonlySet<string>;
}) {
  return (
    Boolean(options.selectedId) &&
    !options.loading &&
    !options.error &&
    !options.playableIds.has(options.selectedId)
  );
}

export function normalizeListenLocalQueueIds(
  value: readonly string[] | null,
): string[] | null {
  if (value === null) {
    return null;
  }
  return Array.from(
    new Set(value.map((id) => id.trim()).filter(Boolean)),
  );
}

export function pruneListenLocalQueueIds(
  value: readonly string[] | null,
  validIds: ReadonlySet<string>,
): string[] | null {
  const normalized = normalizeListenLocalQueueIds(value);
  return normalized === null
    ? null
    : normalized.filter((id) => validIds.has(id));
}

export function clearListenLocalQueueKeepingSelected(
  items: readonly ListenLocalQueueItem[],
  selectedId: string,
): ListenLocalQueueEditResult {
  const selected =
    items.find((item) => item.id === selectedId) ?? items[0] ?? null;
  const queueIds = selected ? [selected.id] : [];
  return {
    queueIds,
    selectedId: selected?.id ?? "",
    changed:
      items.length !== queueIds.length ||
      items.some((item, index) => item.id !== queueIds[index]) ||
      selectedId !== (selected?.id ?? ""),
  };
}

export function removeListenLocalQueueItem(
  items: readonly ListenLocalQueueItem[],
  selectedId: string,
  itemId: string,
): ListenLocalQueueEditResult {
  const removeIndex = items.findIndex((item) => item.id === itemId);
  if (removeIndex < 0) {
    return {
      queueIds: items.map((item) => item.id),
      selectedId,
      changed: false,
    };
  }
  const remaining = items.filter((item) => item.id !== itemId);
  const nextSelectedId =
    selectedId === itemId
      ? remaining[Math.min(removeIndex, remaining.length - 1)]?.id ?? ""
      : selectedId;
  return {
    queueIds: remaining.map((item) => item.id),
    selectedId: nextSelectedId,
    changed: true,
  };
}

export function moveListenLocalQueueItem(
  items: readonly ListenLocalQueueItem[],
  selectedId: string,
  itemId: string,
  direction: -1 | 1,
): ListenLocalQueueEditResult {
  const sourceIndex = items.findIndex((item) => item.id === itemId);
  const targetIndex = sourceIndex + direction;
  if (
    sourceIndex < 0 ||
    targetIndex < 0 ||
    targetIndex >= items.length
  ) {
    return {
      queueIds: items.map((item) => item.id),
      selectedId,
      changed: false,
    };
  }
  const queueIds = items.map((item) => item.id);
  [queueIds[sourceIndex], queueIds[targetIndex]] = [
    queueIds[targetIndex],
    queueIds[sourceIndex],
  ];
  return { queueIds, selectedId, changed: true };
}

export function listenLocalQueueSnapshotsEqual(
  left: ListenLocalQueueSnapshot,
  right: ListenLocalQueueSnapshot,
) {
  if (left.selectedId !== right.selectedId) {
    return false;
  }
  if (left.queueIds === null || right.queueIds === null) {
    return left.queueIds === right.queueIds;
  }
  return (
    left.queueIds.length === right.queueIds.length &&
    left.queueIds.every((id, index) => id === right.queueIds?.[index])
  );
}
