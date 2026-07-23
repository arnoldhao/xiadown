import type { RSSBulkActionResult } from "./rss-bulk-actions";

export function moveOrganizationID(
  order: readonly string[],
  id: string,
  direction: -1 | 1,
) {
  const next = [...order];
  const index = next.indexOf(id);
  const target = index + direction;
  if (index < 0 || target < 0 || target >= next.length) return next;
  [next[index], next[target]] = [next[target], next[index]];
  return next;
}

export function selectionAfterRSSBulkAction(result: RSSBulkActionResult) {
  return new Set(result.failures.map((failure) => failure.id));
}
