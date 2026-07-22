export const LIBRARY_PAGE_SIZE_OPTIONS = [24, 48, 96] as const;
export const DEFAULT_LIBRARY_PAGE_SIZE = 48;

export interface LibraryPageRange {
  start: number;
  end: number;
}

export type LibraryPageToken = number | "ellipsis";

export function normalizeLibraryPageSize(value: number): number {
  if (!Number.isFinite(value) || value <= 0) {
    return DEFAULT_LIBRARY_PAGE_SIZE;
  }
  return Math.max(1, Math.floor(value));
}

export function libraryPageCount(total: number, pageSize: number): number {
  const normalizedTotal = Math.max(0, Math.floor(Number.isFinite(total) ? total : 0));
  return Math.max(1, Math.ceil(normalizedTotal / normalizeLibraryPageSize(pageSize)));
}

export function clampLibraryPage(page: number, total: number, pageSize: number): number {
  const normalizedPage = Number.isFinite(page) ? Math.max(1, Math.floor(page)) : 1;
  return Math.min(normalizedPage, libraryPageCount(total, pageSize));
}

export function libraryPageRange(
  page: number,
  pageSize: number,
  total: number,
): LibraryPageRange {
  const normalizedTotal = Math.max(0, Math.floor(Number.isFinite(total) ? total : 0));
  if (normalizedTotal === 0) {
    return { start: 0, end: 0 };
  }
  const normalizedSize = normalizeLibraryPageSize(pageSize);
  const normalizedPage = clampLibraryPage(page, normalizedTotal, normalizedSize);
  const start = (normalizedPage - 1) * normalizedSize + 1;
  return {
    start,
    end: Math.min(normalizedTotal, start + normalizedSize - 1),
  };
}

/**
 * Keeps the footer compact while retaining direct page-number navigation.
 * The active page and both edges always remain visible.
 */
export function libraryPageTokens(page: number, pageCount: number): LibraryPageToken[] {
  const normalizedCount = Math.max(1, Math.floor(pageCount));
  const normalizedPage = Math.min(
    normalizedCount,
    Math.max(1, Math.floor(Number.isFinite(page) ? page : 1)),
  );
  if (normalizedCount <= 7) {
    return Array.from({ length: normalizedCount }, (_, index) => index + 1);
  }

  const pages = new Set([1, normalizedCount]);
  for (let candidate = normalizedPage - 1; candidate <= normalizedPage + 1; candidate += 1) {
    if (candidate > 1 && candidate < normalizedCount) pages.add(candidate);
  }
  if (normalizedPage <= 3) {
    pages.add(2);
    pages.add(3);
    pages.add(4);
  }
  if (normalizedPage >= normalizedCount - 2) {
    pages.add(normalizedCount - 1);
    pages.add(normalizedCount - 2);
    pages.add(normalizedCount - 3);
  }

  const ordered = [...pages].sort((left, right) => left - right);
  const tokens: LibraryPageToken[] = [];
  ordered.forEach((value, index) => {
    if (index > 0 && value - ordered[index - 1] > 1) tokens.push("ellipsis");
    tokens.push(value);
  });
  return tokens;
}

export function sliceLibraryPage<T>(
  items: readonly T[],
  page: number,
  pageSize: number,
): T[] {
  const normalizedSize = normalizeLibraryPageSize(pageSize);
  const normalizedPage = clampLibraryPage(page, items.length, normalizedSize);
  const start = (normalizedPage - 1) * normalizedSize;
  return items.slice(start, start + normalizedSize);
}
