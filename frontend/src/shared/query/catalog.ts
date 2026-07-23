import { useMutation, useQuery, useQueryClient, type QueryClient } from "@tanstack/react-query";
import { Call } from "@wailsio/runtime";

import type {
  CatalogCollection,
  CatalogItem,
  CatalogItemActivity,
  CatalogItemDetail,
  CatalogItemLifecycleRequest,
  CatalogMetadataEntry,
  CatalogMigrationAudit,
  CatalogOverview,
  CatalogRepresentation,
  CatalogStorageRoot,
  CatalogTag,
  ListCatalogItemsRequest,
  ListCatalogItemsResponse,
  ListCatalogItemActivityRequest,
  ReplaceCatalogCollectionItemsRequest,
  SaveCatalogCollectionRequest,
  SaveCatalogMetadataEntryRequest,
  SaveCatalogRepresentationRequest,
  SelectCatalogStorageRootCommand,
  UpdateCatalogItemRequest,
} from "@/shared/contracts/catalog";

const CATALOG_HANDLER = "xiadown/internal/presentation/wails.CatalogHandler";

/** Stable actor written by user-initiated mutations from the desktop Library UI. */
export const LIBRARY_CATALOG_ACTOR_ID = "desktop-library";

export const catalogKeys = {
  all: ["catalog"] as const,
  overview: ["catalog", "overview"] as const,
  items: (request: ListCatalogItemsRequest) => ["catalog", "items", request] as const,
  completeItems: (request: Omit<ListCatalogItemsRequest, "limit" | "offset">) =>
    ["catalog", "items", "complete", request] as const,
  itemRoot: (itemId: string) => ["catalog", "item", itemId] as const,
  item: (itemId: string, userId = "") => ["catalog", "item", itemId, userId] as const,
  activityRoot: (itemId: string) => ["catalog", "item", itemId, "activity"] as const,
  activity: (itemId: string, limit = 20) => ["catalog", "item", itemId, "activity", limit] as const,
  representations: (itemId: string) => ["catalog", "representations", itemId] as const,
  metadata: (itemId: string, representationId = "") => ["catalog", "metadata", itemId, representationId] as const,
  collections: ["catalog", "collections"] as const,
  tags: ["catalog", "tags"] as const,
  storageRoots: ["catalog", "storage-roots"] as const,
  audit: ["catalog", "audit"] as const,
};

async function callCatalog<T>(method: string, request?: unknown): Promise<T> {
  return request === undefined
    ? await Call.ByName(`${CATALOG_HANDLER}.${method}`) as T
    : await Call.ByName(`${CATALOG_HANDLER}.${method}`, request) as T;
}

async function fetchCatalogItemPage(
  request: ListCatalogItemsRequest,
): Promise<ListCatalogItemsResponse> {
  const raw = await callCatalog<Partial<ListCatalogItemsResponse>>("ListCatalogItems", request);
  const items = Array.isArray(raw.items)
    ? raw.items.map(normalizeCatalogItem).filter(Boolean) as CatalogItem[]
    : [];
  return {
    items,
    total: finiteNumber(raw.total, items.length),
    limit: finiteNumber(raw.limit, request.limit ?? 500),
    offset: finiteNumber(raw.offset, request.offset ?? 0),
  };
}

export function useCatalogOverview(enabled = true) {
  return useQuery({
    queryKey: catalogKeys.overview,
    queryFn: () => callCatalog<CatalogOverview>("GetDefaultCatalogOverview"),
    enabled,
  });
}

export function useCatalogItems(
  request: ListCatalogItemsRequest = {},
  enabled = true,
) {
  return useQuery({
    queryKey: catalogKeys.items(request),
    queryFn: () => fetchCatalogItemPage(request),
    enabled,
    // Retain the exact previous total while a new offset is loading so the
    // controlled footer cannot clamp a requested page back to page one.
    placeholderData: (previousData) => previousData,
  });
}

// Desktop Library routes operate on the complete Catalog. Fetch server-sized
// pages instead of silently truncating the core Library at the 500-item API
// limit. Remote clients keep using the normal paginated public contract.
export function useCompleteCatalogItems(
  request: Omit<ListCatalogItemsRequest, "limit" | "offset"> = {},
  enabled = true,
) {
  return useQuery({
    queryKey: catalogKeys.completeItems(request),
    queryFn: () => listCompleteCatalogItems(request),
    enabled,
  });
}

export async function listCompleteCatalogItems(
  request: Omit<ListCatalogItemsRequest, "limit" | "offset"> = {},
): Promise<ListCatalogItemsResponse> {
  const pageSize = 500;
  const items: CatalogItem[] = [];
  let total = 0;
  let offset = 0;
  for (;;) {
    const page = await fetchCatalogItemPage({ ...request, limit: pageSize, offset });
    total = page.total;
    items.push(...page.items);
    if (page.items.length === 0 || items.length >= total) {
      return { items, total, limit: items.length, offset: 0 };
    }
    offset += page.items.length;
  }
}

export function restoreCatalogItem(
  request: CatalogItemLifecycleRequest,
): Promise<CatalogItemDetail> {
  return callCatalog<CatalogItemDetail>("RestoreCatalogItem", request);
}

export function useCatalogItem(itemId: string, userId = "", enabled = true) {
  return useQuery({
    queryKey: catalogKeys.item(itemId, userId),
    queryFn: () => callCatalog<CatalogItemDetail>("GetCatalogItem", {
      id: itemId,
      userId: userId || undefined,
    }),
    enabled: enabled && Boolean(itemId),
  });
}

export function listCatalogItemActivity(
  request: ListCatalogItemActivityRequest,
): Promise<CatalogItemActivity[]> {
  return callCatalog<CatalogItemActivity[]>("ListCatalogItemActivity", request);
}

export function useCatalogItemActivity(itemId: string, limit = 20, enabled = true) {
  return useQuery({
    queryKey: catalogKeys.activity(itemId, limit),
    queryFn: () => listCatalogItemActivity({ itemId, limit }),
    enabled: enabled && Boolean(itemId),
  });
}

export function useCatalogRepresentations(itemId: string, enabled = true) {
  return useQuery({
    queryKey: catalogKeys.representations(itemId),
    queryFn: () => callCatalog<CatalogRepresentation[]>("ListCatalogRepresentations", { itemId }),
    enabled: enabled && Boolean(itemId),
  });
}

export function useCatalogMetadataEntries(itemId: string, representationId = "", enabled = true) {
  return useQuery({
    queryKey: catalogKeys.metadata(itemId, representationId),
    queryFn: () => callCatalog<CatalogMetadataEntry[]>("ListCatalogMetadataEntries", {
      itemId,
      representationId: representationId || undefined,
    }),
    enabled: enabled && Boolean(itemId),
  });
}

export function useCatalogCollections(enabled = true) {
  return useQuery({
    queryKey: catalogKeys.collections,
    queryFn: () => callCatalog<CatalogCollection[]>("ListCatalogCollections"),
    enabled,
  });
}

export function buildCatalogCollectionMembershipRequest(
  collection: CatalogCollection,
  itemId: string,
  include: boolean,
): ReplaceCatalogCollectionItemsRequest {
  const itemIds = new Set(collection.itemIds);
  if (include) itemIds.add(itemId);
  else itemIds.delete(itemId);
  return {
    collectionId: collection.id,
    itemIds: [...itemIds],
    expectedRevision: collection.revision,
  };
}

export function useSaveCatalogCollection() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (request: SaveCatalogCollectionRequest) =>
      callCatalog<CatalogCollection>("SaveCatalogCollection", request),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: catalogKeys.collections });
    },
  });
}

export function useReplaceCatalogCollectionItems() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (request: ReplaceCatalogCollectionItemsRequest) =>
      callCatalog<CatalogCollection>("ReplaceCatalogCollectionItems", request),
    onSuccess: async (updated) => {
      queryClient.setQueryData<CatalogCollection[]>(catalogKeys.collections, (current) =>
        (current ?? []).map((collection) => collection.id === updated.id ? updated : collection),
      );
      await queryClient.invalidateQueries({ queryKey: catalogKeys.collections });
    },
  });
}

export function useCatalogTags(enabled = true) {
  return useQuery({
    queryKey: catalogKeys.tags,
    queryFn: () => callCatalog<CatalogTag[]>("ListCatalogTags"),
    enabled,
  });
}

export function useCatalogStorageRoots(enabled = true) {
  return useQuery({
    queryKey: catalogKeys.storageRoots,
    queryFn: () => callCatalog<CatalogStorageRoot[]>("ListCatalogStorageRoots"),
    enabled,
  });
}

export function useSelectCatalogStorageRoot() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (request: SelectCatalogStorageRootCommand) =>
      callCatalog<CatalogStorageRoot>("SelectAndSaveCatalogStorageRoot", request),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: catalogKeys.storageRoots }),
        queryClient.invalidateQueries({ queryKey: catalogKeys.overview }),
      ]);
    },
  });
}

export function useCatalogMigrationAudit(enabled = true) {
  return useQuery({
    queryKey: catalogKeys.audit,
    queryFn: () => callCatalog<CatalogMigrationAudit>("GetCatalogMigrationAudit", {}),
    enabled,
  });
}

export function isCatalogItemDetailQueryKey(queryKey: readonly unknown[], itemId: string) {
  return queryKey.length === 4 &&
    queryKey[0] === "catalog" && queryKey[1] === "item" && queryKey[2] === itemId &&
    queryKey[3] !== "activity";
}

export async function refreshCatalogItemQueries(
  queryClient: QueryClient,
  itemId: string,
  detail?: CatalogItemDetail,
) {
  if (detail) {
    queryClient.setQueriesData<CatalogItemDetail>(
      { predicate: (query) => isCatalogItemDetailQueryKey(query.queryKey, itemId) },
      detail,
    );
  }
  await Promise.all([
    queryClient.invalidateQueries({ queryKey: ["catalog", "items"] }),
    queryClient.invalidateQueries({ queryKey: catalogKeys.overview }),
    queryClient.invalidateQueries({ queryKey: catalogKeys.audit }),
    queryClient.invalidateQueries({ queryKey: catalogKeys.activityRoot(itemId) }),
  ]);
}

function useRefreshCatalogItem() {
  const queryClient = useQueryClient();
  return (itemId: string, detail?: CatalogItemDetail) =>
    refreshCatalogItemQueries(queryClient, itemId, detail);
}

export function useUpdateCatalogItem() {
  const refresh = useRefreshCatalogItem();
  return useMutation({
    mutationFn: (request: UpdateCatalogItemRequest) =>
      callCatalog<CatalogItemDetail>("UpdateCatalogItem", request),
    onSuccess: (detail) => refresh(detail.item.id, detail),
  });
}

export function useTrashCatalogItem() {
  const refresh = useRefreshCatalogItem();
  return useMutation({
    mutationFn: (request: CatalogItemLifecycleRequest) =>
      callCatalog<CatalogItemDetail>("TrashCatalogItem", request),
    onSuccess: (detail) => refresh(detail.item.id, detail),
  });
}

export function useRestoreCatalogItem() {
  const refresh = useRefreshCatalogItem();
  return useMutation({
    mutationFn: (request: CatalogItemLifecycleRequest) =>
      callCatalog<CatalogItemDetail>("RestoreCatalogItem", request),
    // Restore may commit before a later projection step reports a warning.
    // Always refetch the trash list so Deleted cannot retain a stale card.
    onSettled: (detail, _error, request) => refresh(request.id, detail),
  });
}

export function useSaveCatalogTag() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (request: { id?: string; name: string }) =>
      callCatalog<CatalogTag>("SaveCatalogTag", request),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: catalogKeys.tags }),
  });
}

export function useReplaceCatalogItemTags() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (request: { itemId: string; tagIds: string[] }) =>
      callCatalog<CatalogTag[]>("ReplaceCatalogItemTags", request),
    onSuccess: async (tags, request) => {
      queryClient.setQueriesData<CatalogItemDetail>(
        { predicate: (query) => isCatalogItemDetailQueryKey(query.queryKey, request.itemId) },
        (current) => current ? { ...current, tags } : current,
      );
      await queryClient.invalidateQueries({ queryKey: catalogKeys.itemRoot(request.itemId) });
    },
  });
}

export function useSaveCatalogRepresentation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (request: SaveCatalogRepresentationRequest) =>
      callCatalog<CatalogRepresentation>("SaveCatalogRepresentation", request),
    onSuccess: async (_, request) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: catalogKeys.itemRoot(request.itemId) }),
        queryClient.invalidateQueries({ queryKey: catalogKeys.representations(request.itemId) }),
        queryClient.invalidateQueries({ queryKey: catalogKeys.audit }),
      ]);
    },
  });
}

export function useSaveCatalogMetadataEntry() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (request: SaveCatalogMetadataEntryRequest) =>
      callCatalog<CatalogMetadataEntry>("SaveCatalogMetadataEntry", request),
    onSuccess: async (_, request) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: catalogKeys.itemRoot(request.itemId) }),
        queryClient.invalidateQueries({ queryKey: ["catalog", "metadata", request.itemId] }),
        queryClient.invalidateQueries({ queryKey: catalogKeys.audit }),
      ]);
    },
  });
}

export function useCheckCatalogStorageRoot() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      callCatalog<CatalogStorageRoot>("CheckCatalogStorageRoot", { id }),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: catalogKeys.storageRoots }),
        queryClient.invalidateQueries({ queryKey: catalogKeys.overview }),
      ]);
    },
  });
}

function normalizeCatalogItem(value: unknown): CatalogItem | null {
  if (!value || typeof value !== "object") return null;
  const item = value as Partial<CatalogItem>;
  const category = normalizeCategory(item.category);
  const status = normalizeStatus(item.status);
  const id = stringValue(item.id);
  const catalogId = stringValue(item.catalogId);
  const title = stringValue(item.title);
  if (!id || !catalogId || !title || !category || !status) return null;
  return {
    id,
    catalogId,
    category,
    kind: stringValue(item.kind) || undefined,
    format: stringValue(item.format) || undefined,
    durationMs: optionalFiniteNumber(item.durationMs),
    sizeBytes: optionalFiniteNumber(item.sizeBytes),
    primaryAssetId: stringValue(item.primaryAssetId) || undefined,
    primaryFileId: stringValue(item.primaryFileId) || undefined,
    artworkAssetId: stringValue(item.artworkAssetId) || undefined,
    artworkFileId: stringValue(item.artworkFileId) || undefined,
    status,
    title,
    sortTitle: stringValue(item.sortTitle) || title,
    description: stringValue(item.description) || undefined,
    revision: finiteNumber(item.revision, 1),
    trashedAt: stringValue(item.trashedAt) || undefined,
    createdAt: stringValue(item.createdAt),
    updatedAt: stringValue(item.updatedAt),
  };
}

function normalizeCategory(value: unknown): CatalogItem["category"] | null {
  return value === "video" || value === "audio" || value === "book" || value === "image" || value === "other" ? value : null;
}

function normalizeStatus(value: unknown): CatalogItem["status"] | null {
  return value === "active" || value === "needs_review" || value === "missing" || value === "trashed" ? value : null;
}

function stringValue(value: unknown) {
  return typeof value === "string" ? value.trim() : "";
}

function finiteNumber(value: unknown, fallback: number) {
  return typeof value === "number" && Number.isFinite(value) ? value : fallback;
}

function optionalFiniteNumber(value: unknown) {
  return typeof value === "number" && Number.isFinite(value) && value >= 0 ? value : undefined;
}
