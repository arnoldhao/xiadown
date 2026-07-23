export type RSSViewType =
  | "auto"
  | "article"
  | "social"
  | "image"
  | "video";

export type RSSEntryKind = Exclude<RSSViewType, "auto">;

export interface RSSSubscription {
  id: string;
  workspaceId: string;
  feedUrl: string;
  siteUrl?: string;
  title: string;
  description?: string;
  iconUrl?: string;
  viewType: RSSViewType;
  resolvedViewType?: RSSViewType;
  categoryId?: string;
  sortOrder?: number;
  enabled: boolean;
  unreadCount: number;
  lastFetchedAt?: string;
  lastSuccessAt?: string;
  lastError?: string;
  createdAt: string;
  updatedAt: string;
  revision: number;
}

export interface RSSMedia {
  url: string;
  mimeType?: string;
  kind: string;
  thumbnail?: string;
  width?: number;
  height?: number;
  durationMs?: number;
}

export interface RSSArticleProgress {
  fraction: number;
  anchor?: string;
  contentRevision: number;
}

export interface RSSStateFieldRevisions {
  read: number;
  starred: number;
  articleProgress: number;
  videoProgressSeconds: number;
}

export type RSSEntryStateField =
  | "read"
  | "starred"
  | "articleProgress"
  | "videoProgressSeconds";

export interface RSSEntry {
  id: string;
  subscriptionId: string;
  externalId: string;
  url?: string;
  title: string;
  author?: string;
  summary?: string;
  contentHtml?: string;
  kind: RSSEntryKind;
  imageUrls: string[];
  media: RSSMedia[];
  mediaUrl?: string;
  mediaType?: string;
  thumbnailUrl?: string;
  platform?: string;
  platformVideoId?: string;
  playbackUrl?: string;
  downloadTarget?: string;
  publishedAt?: string;
  sourceUpdatedAt?: string;
  readAt?: string;
  starredAt?: string;
  articleProgress?: RSSArticleProgress;
  videoProgressSeconds?: number;
  videoDurationSeconds?: number;
  videoCompleted?: boolean;
  fieldRevisions?: RSSStateFieldRevisions;
  stateRevision: number;
  readStateUpdatedAt?: string;
  revision: number;
  createdAt: string;
  modifiedAt: string;
}

export interface RSSEntryPage {
  items: RSSEntry[];
  total: number;
  nextOffset?: number;
  /** RSS change high-water captured with this page's count and rows. */
  snapshot?: number;
}

export interface RSSEntryState {
  entryId: string;
  subjectId: string;
  read: boolean;
  readAt?: string;
  starred: boolean;
  starredAt?: string;
  articleProgress?: RSSArticleProgress;
  videoProgressSeconds?: number;
  videoDurationSeconds?: number;
  videoCompleted: boolean;
  fieldRevisions: RSSStateFieldRevisions;
  revision: number;
  updatedAt: string;
  updatedBy?: string;
  mutationId?: string;
}

export interface RSSSaveEntryImageRequest {
  entryId: string;
  slot: string;
  suggestedName: string;
  dialogTitle: string;
  filterName: string;
  buttonText: string;
}

export interface RSSSaveEntryImageResult {
  saved: boolean;
}

export interface RSSPreviewSubscriptionRequest {
  url: string;
  viewType?: RSSViewType;
}

export interface RSSPreviewResult {
  subscription: RSSSubscription;
  entries: RSSEntry[];
  resolvedUrl: string;
  previewToken: string;
}

export interface RSSAddSubscriptionRequest {
  url: string;
  title?: string;
  viewType?: RSSViewType;
  previewToken?: string;
  allowPending?: boolean;
}

export type RSSDiscoverySort = "popular" | "title";

export interface RSSListDiscoveryRequest {
  query?: string;
  categoryId?: string;
  language?: string;
  sort?: RSSDiscoverySort;
  offset?: number;
  limit?: number;
  forceRefresh?: boolean;
}

export interface RSSDiscoveryCategory {
  id: string;
  count: number;
  examples: string[];
  iconUrl: string;
  iconLabel: string;
}

export interface RSSDiscoveryParameterOption {
  value: string;
  label: string;
}

export interface RSSDiscoveryParameter {
  name: string;
  description: string;
  defaultValue: string | null;
  exampleValue: string;
  optional: boolean;
  catchAll: boolean;
  type: string;
  options: RSSDiscoveryParameterOption[];
}

export interface RSSDiscoveryRoute {
  id: string;
  title: string;
  url: string;
  iconUrl?: string;
  provider: "rsshub" | "rss";
  description: string;
  sourceId: string;
  sourceName: string;
  sourceUrl: string;
  siteUrl: string;
  routePath: string;
  examplePath: string;
  categories: string[];
  heat: number;
  language: string;
  region: string;
  viewType: RSSViewType;
  parameters: RSSDiscoveryParameter[];
  needsParameters: boolean;
  requiresConfig: boolean;
  requiresPuppeteer: boolean;
}

export interface RSSDiscoveryResult {
  categories: RSSDiscoveryCategory[];
  routes: RSSDiscoveryRoute[];
  totalRouteCount: number;
  filteredRouteCount: number;
  offset: number;
  limit: number;
  hasMore: boolean;
  sourceUrl: string;
  fetchedAt: string;
}

export interface RSSUpdateSubscriptionRequest {
  id: string;
  title?: string;
  viewType?: RSSViewType;
  enabled?: boolean;
  /** Empty string removes the subscription from its category. */
  categoryId?: string;
  sortOrder?: number;
}

export interface RSSSubscriptionRequest {
  id: string;
}

export interface RSSRefreshRequest {
  id?: string;
}

export interface RSSRefreshResult {
  subscriptions: number;
  created: number;
  updated: number;
  failed: number;
}

export interface RSSBackfillHistoryRequest {
  subscriptionId?: string;
  kind?: RSSEntryKind;
}

export type RSSBackfillHistoryCapability =
  | "unknown"
  | "available"
  | "unsupported";

export interface RSSBackfillHistorySource {
  subscriptionId: string;
  attempted: boolean;
  capability: RSSBackfillHistoryCapability;
  exhausted: boolean;
  noProgress: number;
  created: number;
  updated: number;
  error?: string;
}

export interface RSSBackfillHistoryResult {
  subscriptions: number;
  attempted: number;
  supported: number;
  unsupported: number;
  exhausted: number;
  created: number;
  updated: number;
  failed: number;
  hasMore: boolean;
  sources: RSSBackfillHistorySource[];
}

export interface RSSListEntriesRequest {
  subscriptionId?: string;
  collectionId?: string;
  categoryId?: string;
  kind?: RSSEntryKind;
  query?: string;
  unreadOnly?: boolean;
  starredOnly?: boolean;
  limit?: number;
  offset?: number;
}

export interface RSSMarkAllReadRequest {
  subscriptionId?: string;
  collectionId?: string;
  categoryId?: string;
  kind?: RSSEntryKind;
  starredOnly?: boolean;
}

export interface RSSCategory {
  id: string;
  workspaceId: string;
  title: string;
  sortOrder: number;
  subscriptionCount: number;
  unreadCount: number;
  createdAt: string;
  updatedAt: string;
  revision: number;
}

export type RSSCollectionKind = "subscriptions" | "entries";

export interface RSSCollection {
  id: string;
  workspaceId: string;
  title: string;
  description?: string;
  kind: RSSCollectionKind;
  viewType: RSSViewType;
  sortOrder: number;
  itemCount: number;
  unreadCount: number;
  createdAt: string;
  updatedAt: string;
  revision: number;
}

export interface RSSCollectionItems {
  collectionId: string;
  kind: RSSCollectionKind;
  itemIds: string[];
}

export interface RSSCreateCategoryRequest {
  title: string;
  sortOrder?: number;
}

export interface RSSUpdateCategoryRequest {
  id: string;
  title?: string;
  sortOrder?: number;
  expectedRevision?: number;
}

export interface RSSReorderRequest {
  ids: string[];
}

export interface RSSReorderSubscriptionsRequest extends RSSReorderRequest {
  categoryId?: string;
}

export interface RSSCreateCollectionRequest {
  title: string;
  description?: string;
  kind: RSSCollectionKind;
  viewType?: RSSViewType;
  sortOrder?: number;
}

export interface RSSUpdateCollectionRequest {
  id: string;
  title?: string;
  description?: string;
  viewType?: RSSViewType;
  sortOrder?: number;
  expectedRevision?: number;
}

export interface RSSUpdateCollectionItemsRequest {
  id: string;
  itemIds: string[];
}

export interface RSSMarkAllReadResult {
  updated: number;
}

export interface RSSSetEntryReadRequest {
  id: string;
  read: boolean;
  expectedRevision?: number;
  mutationId?: string;
}

export interface RSSSetEntryStateRequest {
  id: string;
  field: RSSEntryStateField;
  read?: boolean;
  starred?: boolean;
  articleProgress?: RSSArticleProgress;
  videoProgressSeconds?: number;
  videoDurationSeconds?: number;
  expectedRevision: number;
  mutationId: string;
}

export interface RSSListChangesRequest {
  after?: number;
  limit?: number;
}

export interface RSSChange {
  sequence: number;
  entityType: "subscription" | "entry" | "entry_state" | "download";
  entityId: string;
  operation: "upsert" | "delete";
  revision: number;
  payload?: unknown;
  changedAt: string;
}

export interface RSSChangePage {
  changes: RSSChange[];
  epoch: string;
  cursor: number;
  highWater: number;
  hasMore: boolean;
}
