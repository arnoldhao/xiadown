import type { LibraryFileDTO } from "@/shared/contracts/library";

export type CatalogItemCategory = "video" | "audio" | "book" | "image" | "other";
export type CatalogItemStatus = "active" | "needs_review" | "missing" | "trashed";

export interface Catalog {
  id: string;
  name: string;
  description?: string;
  status: string;
  isDefault: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface CatalogCount {
  all: number;
  video: number;
  audio: number;
  books: number;
  images: number;
  others: number;
}

export interface CatalogStatusCount {
  active: number;
  needsReview: number;
  missing: number;
  trashed: number;
}

export interface CatalogHealthCount {
  assetLinks: number;
  itemsWithoutAssets: number;
  unavailableAssetFiles: number;
  legacyFilesWithErrors: number;
  offlineStorageRoots: number;
  readOnlyStorageRoots: number;
  storageRootsWithErrors: number;
}

export interface CatalogOverview {
  catalog: Catalog;
  /** Sum of known unique files linked to this catalog. */
  totalSizeBytes: number;
  categories: CatalogCount;
  statuses: CatalogStatusCount;
  health: CatalogHealthCount;
}

export interface CatalogItem {
  id: string;
  catalogId: string;
  category: CatalogItemCategory;
  /** Optional fine-grained content kind used to group category=other items. */
  kind?: string;
  format?: string;
  durationMs?: number;
  sizeBytes?: number;
  /** Opaque IDs only. Local paths remain confined to desktop item detail. */
  primaryAssetId?: string;
  primaryFileId?: string;
  artworkAssetId?: string;
  artworkFileId?: string;
  status: CatalogItemStatus;
  title: string;
  sortTitle: string;
  description?: string;
  revision: number;
  trashedAt?: string;
  createdAt: string;
  updatedAt: string;
}

export interface CatalogItemAsset {
  id: string;
  itemId: string;
  fileId: string;
  storageRootId?: string;
  role: string;
  label?: string;
  position: number;
  fileAvailable: boolean;
  file?: LibraryFileDTO;
  createdAt: string;
  updatedAt: string;
}

export type CatalogRepresentationKind =
  | "original"
  | "optimized"
  | "thumbnail"
  | "transcript"
  | "subtitle"
  | "artwork"
  | "preview"
  | "attachment";

export type CatalogRepresentationPurpose =
  | "primary"
  | "playback"
  | "preview"
  | "accessibility"
  | "artwork"
  | "attachment"
  | "indexing";

export type CatalogRepresentationAvailability =
  | "available"
  | "processing"
  | "offline"
  | "missing"
  | "corrupt";

export interface CatalogRepresentation {
  id: string;
  catalogId: string;
  itemId: string;
  assetId: string;
  storageRootId?: string;
  kind: CatalogRepresentationKind | string;
  purpose: CatalogRepresentationPurpose | string;
  mediaType?: string;
  container?: string;
  codec?: string;
  width?: number;
  height?: number;
  durationMs?: number;
  bitrateBps?: number;
  language?: string;
  checksumAlgorithm?: string;
  checksum?: string;
  sizeBytes?: number;
  availability: CatalogRepresentationAvailability | string;
  revision: number;
  createdAt: string;
  updatedAt: string;
}

export interface ListCatalogRepresentationsRequest {
  itemId: string;
}

export type CatalogMetadataSource =
  | "user"
  | "embedded"
  | "sidecar"
  | "remote"
  | "derived"
  | "migration"
  | "system";

export type CatalogMetadataValueType =
  | "string"
  | "integer"
  | "number"
  | "boolean"
  | "date"
  | "datetime"
  | "duration_ms"
  | "object"
  | "array"
  | "json";

export interface CatalogMetadataEntry {
  id: string;
  catalogId: string;
  itemId: string;
  representationId?: string;
  namespace: string;
  key: string;
  valueType: CatalogMetadataValueType | string;
  valueJson: string;
  language?: string;
  position: number;
  source: CatalogMetadataSource | string;
  provenance: string;
  confidence?: number;
  locked: boolean;
  revision: number;
  createdAt: string;
  updatedAt: string;
}

export interface ListCatalogMetadataEntriesRequest {
  itemId: string;
  representationId?: string;
}

export interface CatalogTag {
  id: string;
  catalogId: string;
  name: string;
  normalizedName: string;
  createdAt: string;
  updatedAt: string;
}

export interface SaveCatalogTagRequest {
  id?: string;
  name: string;
}

export interface ReplaceCatalogItemTagsRequest {
  itemId: string;
  tagIds: string[];
}

export interface CatalogUserState {
  id: string;
  catalogId: string;
  itemId: string;
  userId: string;
  favorite: boolean;
  rating: number;
  progress: number;
  positionMs: number;
  locator?: string;
  completed: boolean;
  revision: number;
  lastOpenedAt?: string;
  createdAt: string;
  updatedAt: string;
}

export interface GetCatalogUserStateRequest {
  itemId: string;
  userId: string;
}

export interface UpdateCatalogUserStateRequest {
  itemId: string;
  userId: string;
  expectedRevision: number;
  favorite?: boolean;
  rating?: number;
  progress?: number;
  positionMs?: number;
  locator?: string;
  completed?: boolean;
  openedNow?: boolean;
  actorId?: string;
}

export interface CatalogItemDetail {
  item: CatalogItem;
  assets: CatalogItemAsset[];
  representations: CatalogRepresentation[];
  metadata: CatalogMetadataEntry[];
  tags: CatalogTag[];
  userState?: CatalogUserState;
}

export interface ListCatalogItemsRequest {
  category?: string;
  status?: string;
  /** Keep recoverable trash out of normal browsing while retaining other health states. */
  excludeTrashed?: boolean;
  query?: string;
  sort?: string;
  limit?: number;
  offset?: number;
}

export interface ListCatalogItemsResponse {
  items: CatalogItem[];
  total: number;
  limit: number;
  offset: number;
}

export interface GetCatalogItemRequest {
  id: string;
  userId?: string;
}

export interface UpdateCatalogItemRequest {
  id: string;
  expectedRevision: number;
  title?: string;
  description?: string;
  category?: CatalogItemCategory;
  actorId?: string;
  userId?: string;
}

export interface CatalogItemLifecycleRequest {
  id: string;
  expectedRevision: number;
  actorId?: string;
  userId?: string;
}

export type CatalogItemActivityAction =
  | "catalog_item_updated"
  | "catalog_item_trashed"
  | "catalog_item_restored";

export interface ListCatalogItemActivityRequest {
  itemId: string;
  limit?: number;
}

export interface CatalogItemActivity {
  action: CatalogItemActivityAction;
  revision: number;
  actor: string;
  occurredAt: string;
}

export interface CatalogCollection {
  id: string;
  catalogId: string;
  name: string;
  description?: string;
  kind: string;
  smartQuery?: string;
  revision: number;
  itemIds: string[];
  itemIdsTruncated?: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface SaveCatalogCollectionRequest {
  id?: string;
  expectedRevision?: number;
  name: string;
  description?: string;
  kind?: string;
  smartQuery?: string;
}

export interface ReplaceCatalogCollectionItemsRequest {
  collectionId: string;
  itemIds: string[];
  expectedRevision: number;
}

export interface CatalogStorageRoot {
  id: string;
  catalogId: string;
  name: string;
  path: string;
  volumeId?: string;
  mode: string;
  status: string;
  lastCheckedAt?: string;
  lastError?: string;
  createdAt: string;
  updatedAt: string;
}

export interface SaveCatalogStorageRootRequest {
  id?: string;
  name: string;
  path: string;
  volumeId?: string;
  mode?: string;
}

export interface CheckCatalogStorageRootRequest {
  id: string;
}

export interface SelectCatalogStorageRootCommand {
  name: string;
  mode: "referenced" | "managed";
}

export interface CatalogMigrationAuditCount {
  legacyFiles: number;
  legacyMappings: number;
  assetLinks: number;
  items: number;
  activeItems: number;
  missingItems: number;
  trashedItems: number;
  needsReviewItems: number;
  preservedFileIds: number;
  preservedPhysicalReferences: number;
  representations: number;
  metadataEntries: number;
}

export interface CatalogMigrationAuditFinding {
  unmappedLegacyFiles: number;
  duplicateMappings: number;
  danglingAssets: number;
  missingMappingSources: number;
  missingMappingTargets: number;
  mappingAssetMismatches: number;
  changedPhysicalReferences: number;
  assetsWithoutRepresentations: number;
  danglingRepresentations: number;
  representationMismatches: number;
  danglingMetadataEntries: number;
  metadataRepresentationMismatches: number;
  total: number;
}

export interface CatalogMigrationAuditIssue {
  kind: string;
  sourceId?: string;
  targetId?: string;
  assetId?: string;
  representationId?: string;
  metadataEntryId?: string;
  description: string;
}

export interface CatalogMigrationAudit {
  catalogId: string;
  migrationId: string;
  healthy: boolean;
  counts: CatalogMigrationAuditCount;
  findings: CatalogMigrationAuditFinding;
  issues: CatalogMigrationAuditIssue[];
  auditedAt: string;
}

export interface CatalogMigrationAuditRequest {
  migrationId?: string;
}

export interface SaveCatalogRepresentationRequest {
  id?: string;
  itemId: string;
  assetId: string;
  expectedRevision?: number;
  kind: CatalogRepresentationKind | string;
  purpose?: CatalogRepresentationPurpose | string;
  mediaType?: string;
  container?: string;
  codec?: string;
  width?: number;
  height?: number;
  durationMs?: number;
  bitrateBps?: number;
  language?: string;
  checksumAlgorithm?: string;
  checksum?: string;
  sizeBytes?: number;
  availability?: CatalogRepresentationAvailability | string;
  actorId?: string;
}

export interface SaveCatalogMetadataEntryRequest {
  id?: string;
  itemId: string;
  representationId?: string;
  expectedRevision?: number;
  namespace: string;
  key: string;
  valueType: CatalogMetadataValueType | string;
  valueJson: string;
  language?: string;
  position?: number;
  source: CatalogMetadataSource | string;
  provenance: string;
  confidence?: number;
  locked?: boolean;
  actorId?: string;
}
