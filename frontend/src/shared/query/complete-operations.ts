import type {
  ListOperationsRequest,
  OperationListItemDTO,
} from "@/shared/contracts/library";

export const COMPLETE_OPERATIONS_PAGE_SIZE = 500;
export const COMPLETE_OPERATIONS_MAX_PAGES = 1_000;
export const COMPLETE_OPERATIONS_PAGE_OVERLAP = 8;

type CompleteOperationsRequest = Omit<ListOperationsRequest, "limit" | "offset">;

type OperationRealtimeEvent = {
  type?: string;
  payload?: unknown;
};

export type OperationPageFetcher = (
  request: ListOperationsRequest,
) => Promise<readonly OperationListItemDTO[]>;

export function shouldRefreshCompleteOperations(event: OperationRealtimeEvent) {
  if (event.type?.trim().toLowerCase() === "delete") {
    return true;
  }
  if (!event.payload || typeof event.payload !== "object") {
    return false;
  }
  const status = String((event.payload as Record<string, unknown>).status ?? "")
    .trim()
    .toLowerCase();
  return ["queued", "succeeded", "failed", "canceled", "cancelled"].includes(status);
}

function operationTimestamp(operation: OperationListItemDTO) {
  const parsed = Date.parse(operation.createdAt);
  return Number.isFinite(parsed) ? parsed : 0;
}

export function sortAndDedupeOperations(
  operationGroups: readonly (readonly OperationListItemDTO[])[],
) {
  const byId = new Map<string, OperationListItemDTO>();
  operationGroups.forEach((operations) => {
    operations.forEach((operation) => {
      const operationId = operation.operationId.trim();
      if (operationId) {
        // Later groups are fresher snapshots and intentionally replace earlier ones.
        byId.set(operationId, operation);
      }
    });
  });
  return [...byId.values()].sort((left, right) => {
    const timeDifference = operationTimestamp(right) - operationTimestamp(left);
    return timeDifference || left.operationId.localeCompare(right.operationId);
  });
}

export async function collectCompleteOperations(
  request: CompleteOperationsRequest,
  fetchPage: OperationPageFetcher,
  options: { pageSize?: number; maxPages?: number } = {},
): Promise<OperationListItemDTO[]> {
  const pageSize = Math.max(1, Math.floor(options.pageSize ?? COMPLETE_OPERATIONS_PAGE_SIZE));
  const maxPages = Math.max(1, Math.floor(options.maxPages ?? COMPLETE_OPERATIONS_MAX_PAGES));
  const overlap = Math.min(COMPLETE_OPERATIONS_PAGE_OVERLAP, pageSize - 1);
  const pageStep = pageSize - overlap;
  const pages: OperationListItemDTO[][] = [];
  const seen = new Set<string>();
  let offset = 0;

  for (let pageIndex = 0; pageIndex < maxPages; pageIndex += 1) {
    const page = [...await fetchPage({ ...request, limit: pageSize, offset })];
    pages.push(page);
    let additions = 0;
    page.forEach((operation) => {
      const operationId = operation.operationId.trim();
      if (operationId && !seen.has(operationId)) {
        seen.add(operationId);
        additions += 1;
      }
    });
    if (page.length < pageSize) {
      return sortAndDedupeOperations(pages);
    }
    if (additions === 0) {
      throw new Error("Operation history pagination made no forward progress.");
    }
    // A small overlap prevents one boundary insertion/deletion from silently
    // skipping a record while offset pages are being collected.
    offset += pageStep;
  }

  throw new Error(
    `Operation history exceeded the ${pageSize + (maxPages - 1) * pageStep} item pagination safety limit.`,
  );
}
