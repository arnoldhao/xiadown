import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Call } from "@wailsio/runtime";

import type {
  CleanDataManagementRequest,
  CleanDataManagementResponse,
  CleanDataManagementResult,
  DataManagementCategory,
  DataManagementCategoryId,
  DataManagementItem,
  DataManagementSnapshot,
  ResetApplicationResponse,
} from "@/shared/contracts/dataManagement";

const DATA_MANAGEMENT_HANDLER =
  "xiadown/internal/presentation/wails.DataManagementHandler";

export const DATA_MANAGEMENT_QUERY_KEY = ["settings", "data-management"] as const;

const DATA_MANAGEMENT_CATEGORY_ORDER: DataManagementCategoryId[] = [
  "core",
  "reclaimable",
  "obsolete",
];

const SUCCESSFUL_CLEAN_STATUSES = new Set(["cleared", "already_missing"]);

export interface DataManagementCleanSettlement {
  succeededIds: string[];
  failedIds: string[];
}

function categoryId(value: unknown): DataManagementCategoryId | null {
  const normalized = stringValue(value);
  return DATA_MANAGEMENT_CATEGORY_ORDER.includes(
    normalized as DataManagementCategoryId,
  )
    ? (normalized as DataManagementCategoryId)
    : null;
}

function stringValue(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}

function numberValue(value: unknown): number {
  const result = Number(value);
  return Number.isFinite(result) && result >= 0 ? result : 0;
}

function arrayValue(value: unknown): unknown[] {
  return Array.isArray(value) ? value : [];
}

export function normalizeDataManagementSnapshot(raw: unknown): DataManagementSnapshot {
  const value = raw && typeof raw === "object" ? (raw as Record<string, unknown>) : {};
  const normalizedCategories = arrayValue(value.categories)
    .map((category): DataManagementCategory | null => {
      if (!category || typeof category !== "object") {
        return null;
      }
      const candidate = category as Record<string, unknown>;
      const id = categoryId(candidate.id);
      if (!id) {
        return null;
      }
      const items = arrayValue(candidate.items)
        .map((item): DataManagementItem | null => {
          if (!item || typeof item !== "object") {
            return null;
          }
          const resource = item as Record<string, unknown>;
          const resourceId = stringValue(resource.id);
          if (!resourceId) {
            return null;
          }
          return {
            id: resourceId,
            labelKey: stringValue(resource.labelKey),
            descriptionKey: stringValue(resource.descriptionKey),
            sizeBytes: numberValue(resource.sizeBytes),
            itemCount: numberValue(resource.itemCount),
            state: stringValue(resource.state),
            risk: stringValue(resource.risk) || "protected",
            clearable: resource.clearable === true,
            selectedByDefault: resource.selectedByDefault === true,
          };
        })
        .filter((item): item is DataManagementItem => item !== null);
      const visibleItems =
        id === "obsolete"
          ? items.filter(
              (item) =>
                item.state !== "empty" ||
                item.sizeBytes > 0 ||
                item.itemCount > 0 ||
                item.clearable,
            )
          : items;
      return {
        id,
        labelKey: stringValue(candidate.labelKey),
        totalBytes: numberValue(candidate.totalBytes),
        items: visibleItems,
      };
    })
    .filter((category): category is DataManagementCategory => category !== null);
  const categoryById = new Map(
    normalizedCategories.map((category) => [category.id, category]),
  );
  const categories = DATA_MANAGEMENT_CATEGORY_ORDER.map(
    (id): DataManagementCategory =>
      categoryById.get(id) ?? {
        id,
        totalBytes: 0,
        items: [],
      },
  );
  return {
    totalBytes: numberValue(value.totalBytes),
    safeReclaimableBytes: numberValue(value.safeReclaimableBytes),
    scannedAt: stringValue(value.scannedAt),
    categories,
  };
}

export function settleDataManagementCleanResults(
  requestedIds: string[],
  results: CleanDataManagementResult[],
): DataManagementCleanSettlement {
  const requested = [...new Set(requestedIds.map(stringValue).filter(Boolean))];
  const requestedSet = new Set(requested);
  const succeededById = new Map<string, boolean>();

  for (const result of results) {
    const resourceId = stringValue(result.resourceId);
    if (!resourceId || !requestedSet.has(resourceId)) {
      continue;
    }
    const succeeded = SUCCESSFUL_CLEAN_STATUSES.has(
      stringValue(result.status).toLowerCase(),
    );
    // A duplicate failure must never be hidden by another successful row.
    succeededById.set(
      resourceId,
      succeededById.get(resourceId) === false ? false : succeeded,
    );
  }

  return requested.reduce<DataManagementCleanSettlement>(
    (settlement, resourceId) => {
      if (succeededById.get(resourceId) === true) {
        settlement.succeededIds.push(resourceId);
      } else {
        // Missing, denied, failed, and unknown results all fail closed.
        settlement.failedIds.push(resourceId);
      }
      return settlement;
    },
    { succeededIds: [], failedIds: [] },
  );
}

export function useDataManagementSnapshot(enabled: boolean) {
  return useQuery({
    queryKey: DATA_MANAGEMENT_QUERY_KEY,
    queryFn: async () =>
      normalizeDataManagementSnapshot(
        await Call.ByName(`${DATA_MANAGEMENT_HANDLER}.GetSnapshot`),
      ),
    enabled,
    staleTime: 5_000,
    refetchOnWindowFocus: false,
  });
}

export function useCleanDataManagement() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (
      request: CleanDataManagementRequest,
    ): Promise<CleanDataManagementResponse> => {
      const raw = await Call.ByName(`${DATA_MANAGEMENT_HANDLER}.Clean`, request);
      const value = raw && typeof raw === "object" ? (raw as Record<string, unknown>) : {};
      return {
        results: arrayValue(value.results).map((item) => {
          const result = item && typeof item === "object" ? (item as Record<string, unknown>) : {};
          return {
            resourceId: stringValue(result.resourceId),
            status: stringValue(result.status),
            bytesFreed: numberValue(result.bytesFreed),
            message: stringValue(result.message),
          };
        }),
        snapshot: normalizeDataManagementSnapshot(value.snapshot),
      };
    },
    onSuccess: (response) => {
      queryClient.setQueryData(DATA_MANAGEMENT_QUERY_KEY, response.snapshot);
    },
  });
}

export function useResetApplication() {
  return useMutation({
    mutationFn: async (): Promise<ResetApplicationResponse> => {
      const raw = await Call.ByName(
        `${DATA_MANAGEMENT_HANDLER}.ResetApplication`,
      );
      const value =
        raw && typeof raw === "object" ? (raw as Record<string, unknown>) : {};
      return { scheduled: value.scheduled === true };
    },
  });
}
