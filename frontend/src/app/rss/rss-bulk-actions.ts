export interface RSSBulkActionFailure {
  id: string;
  error: unknown;
}

export interface RSSBulkActionResult {
  requested: number;
  succeededIDs: string[];
  failures: RSSBulkActionFailure[];
}

/**
 * Keeps a selection scoped to rows that still exist. The returned Set is a
 * fresh value so React state never depends on mutations to an older snapshot.
 */
export function reconcileRSSBulkSelection(
  selection: ReadonlySet<string>,
  availableIDs: readonly string[],
) {
  const available = new Set(availableIDs.map(normalizeID).filter(Boolean));
  return new Set(
    [...selection].map(normalizeID).filter((id) => available.has(id)),
  );
}

/** Replaces the selection with the requested state of the current visible scope. */
export function setRSSVisibleSelection(
  _selection: ReadonlySet<string>,
  visibleIDs: readonly string[],
  selected: boolean,
) {
  const next = new Set<string>();
  if (!selected) return next;
  for (const candidate of visibleIDs) {
    const id = normalizeID(candidate);
    if (!id) continue;
    next.add(id);
  }
  return next;
}

export function toggleRSSBulkSelection(
  selection: ReadonlySet<string>,
  candidate: string,
) {
  const id = normalizeID(candidate);
  const next = new Set([...selection].map(normalizeID).filter(Boolean));
  if (!id) return next;
  if (next.has(id)) next.delete(id);
  else next.add(id);
  return next;
}

/**
 * Runs a bounded batch and reports every failure instead of abandoning the
 * remaining subscriptions after the first rejected bridge call.
 */
export async function runRSSBulkAction(
  candidates: readonly string[],
  worker: (id: string) => Promise<unknown>,
  concurrency = 4,
): Promise<RSSBulkActionResult> {
  const ids = [...new Set(candidates.map(normalizeID).filter(Boolean))];
  const succeededIDs: string[] = [];
  const failures: RSSBulkActionFailure[] = [];
  const workerCount = Math.min(
    ids.length,
    Math.max(1, Math.min(8, Math.trunc(concurrency) || 1)),
  );
  let cursor = 0;

  await Promise.all(Array.from({ length: workerCount }, async () => {
    while (cursor < ids.length) {
      const index = cursor;
      cursor += 1;
      const id = ids[index];
      if (!id) continue;
      try {
        await worker(id);
        succeededIDs.push(id);
      } catch (error) {
        failures.push({ id, error });
      }
    }
  }));

  const order = new Map(ids.map((id, index) => [id, index]));
  succeededIDs.sort((left, right) => (order.get(left) ?? 0) - (order.get(right) ?? 0));
  failures.sort((left, right) => (order.get(left.id) ?? 0) - (order.get(right.id) ?? 0));
  return { requested: ids.length, succeededIDs, failures };
}

function normalizeID(value: string) {
  return value.trim();
}
