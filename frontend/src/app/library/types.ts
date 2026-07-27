import type {
  LibraryDTO,
  LibraryFileDTO,
  OperationListItemDTO,
} from "@/shared/contracts/library";
import type { CatalogItem } from "@/shared/contracts/catalog";
import {
  normalizeLanguage,
  resolveI18nText,
  type TFunction,
} from "@/shared/i18n";

export type LibraryWorkspaceRoute =
  | "search"
  | "ended"
  | "all"
  | "video"
  | "audio"
  | "books"
  | "images"
  | "others";

export type LibraryItemCategory =
  | "task"
  | "video"
  | "audio"
  | "book"
  | "image"
  | "other";

export type LibraryOtherGroup =
  | "document"
  | "font"
  | "archive"
  | "subtitle"
  | "manifest"
  | "api"
  | "unknown"
  | "needs-review"
  | "missing";

/**
 * A bounded visual description of one file produced by a task. Real image
 * previews are optional: callers can render a kind/format page when an asset
 * cannot be decoded safely by the browser.
 */
export interface LibraryTaskPreviewItem {
  id: string;
  kind: string;
  previewURL?: string;
  label?: string;
}

/**
 * The complete, actionable projection of a file produced by a task. Unlike
 * `LibraryTaskPreviewItem`, unavailable records stay in this collection so a
 * task companion can explain their status without offering a broken preview.
 */
export interface LibraryTaskFileItem {
  fileId: string;
  previewItemId: string;
  title: string;
  kind: string;
  category: Exclude<LibraryItemCategory, "task">;
  status: string;
  format: string;
  canView: boolean;
  previewURL?: string;
  file?: LibraryFileDTO;
}

export interface LibraryCardPreview {
  kind: "pdf" | "log";
  sourceURL: string;
  /** Stable, path-free identity used by the bounded derived-preview cache. */
  cacheKey: string;
}

export interface LibraryWorkspaceItem {
  id: string;
  source: "file" | "task";
  libraryId: string;
  libraryName: string;
  title: string;
  subtitle: string;
  category: LibraryItemCategory;
  otherGroup?: LibraryOtherGroup;
  status: string;
  availability?: string;
  format: string;
  sizeBytes?: number;
  durationMs?: number;
  createdAt: string;
  updatedAt: string;
  path: string;
  coverURL: string;
  fallbackCoverURL?: string;
  cardPreview?: LibraryCardPreview;
  taskPreviewItems?: readonly LibraryTaskPreviewItem[];
  taskPreviewTotalCount?: number;
  taskFiles?: readonly LibraryTaskFileItem[];
  rootId: string;
  searchText: string;
  file?: LibraryFileDTO;
  catalogItem?: CatalogItem;
  operation?: OperationListItemDTO;
  library?: LibraryDTO;
}

export interface LibraryWorkspaceLabels {
  locale: string;
  search: string;
  ended: string;
  all: string;
  tasks: string;
  video: string;
  audio: string;
  books: string;
  images: string;
  others: string;
  searchPlaceholder: string;
  sortLabel: string;
  sortNewest: string;
  sortOldest: string;
  sortName: string;
  sortSize: string;
  gridView: string;
  listView: string;
  emptyTitle: string;
  emptyDescription: string;
  itemCount: (count: number) => string;
  pageRange: (start: number, end: number, total: number) => string;
  perPage: (pageSize: number) => string;
  perPageUnit: string;
  pageOf: (page: number, pageCount: number) => string;
  previousPage: string;
  nextPage: string;
  view: string;
  cancelDialog: string;
  deleteItem: string;
  reset: string;
  preview: string;
  info: string;
  tags: string;
  relations: string;
  versions: string;
  metadata: string;
  activity: string;
  noSelection: string;
  noSelectionDescription: string;
  noRelations: string;
  noVersions: string;
  noActivity: string;
  outputCurrent: string;
  outputHistorical: string;
  outputDetached: string;
  operationRenamed: string;
  operationCanceled: string;
  operationResumed: string;
  status: string;
  statusLabel: (status: string) => string;
  progress: string;
  format: string;
  size: string;
  duration: string;
  elapsed: string;
  play: string;
  pause: string;
  seek: string;
  mute: string;
  unmute: string;
  volume: string;
  taskFiles: string;
  taskNoFiles: string;
  taskType: string;
  downloadURL: string;
  sourceFile: string;
  storageMode: string;
  downloadedSource: string;
  referencedImportSource: string;
  managedImportSource: string;
  generatedSource: string;
  unknownSource: string;
  storageRoot: string;
  importedAt: string;
  importBatch: string;
  associatedTask: string;
  unmanagedMode: string;
  copyDownloadURL: string;
  openLocation: string;
  renameTask: string;
  renameTaskNameLabel: string;
  renameTaskNamePlaceholder: string;
  renameFile: string;
  renameFileNameLabel: string;
  renameFileNamePlaceholder: string;
  renameNameRequired: string;
  renameNameInvalid: string;
  renameNameTooLong: string;
  deleteTaskTitle: string;
  deleteFileTitle: string;
  deleteFileMessage: (name: string) => string;
  deleteFiles: string;
  removeTaskOutputTitle: string;
  removeTaskOutputMessage: (name: string) => string;
  alsoDeleteFile: string;
  location: string;
  library: string;
  created: string;
  updated: string;
  title: string;
  description: string;
  category: string;
  saveChanges: string;
  trashItem: string;
  restoreItem: string;
  revision: string;
  itemDetails: string;
  noTags: string;
  newTag: string;
  tagPlaceholder: string;
  noCollections: string;
  collectionKind: string;
  newCollection: string;
  collectionPlaceholder: string;
  addToCollection: string;
  removeFromCollection: string;
  smartCollection: string;
  smartCollectionReadonly: string;
  assets: string;
  representations: string;
  asset: string;
  role: string;
  availability: string;
  mediaType: string;
  container: string;
  codec: string;
  resolution: string;
  bitrate: string;
  language: string;
  checksum: string;
  noAssets: string;
  noRepresentations: string;
  noMetadata: string;
  addMetadata: string;
  metadataNamespace: string;
  metadataKey: string;
  metadataValue: string;
  savingMetadata: string;
  source: string;
  provenance: string;
  confidence: string;
  locked: string;
  yes: string;
  no: string;
  value: string;
  auditHealthy: string;
  auditIssues: string;
  catalogAudit: string;
  auditedAt: string;
  loading: string;
  loadFailed: string;
  retry: string;
  revisionConflict: string;
  health: string;
  manage: string;
  managementTitle: string;
  managementDescription: string;
  summary: string;
  dataManagement: string;
  librarySize: string;
  storageRoots: string;
  storageOverview: string;
  mountedCapacity: string;
  storageLocationsOnline: (count: number) => string;
  storageRootsSummary: (roots: number, offline: number) => string;
  storageVolumeSummary: (roots: number, librarySize: string) => string;
  storageAvailableOfTotal: (available: string, total: string) => string;
  systemVolume: string;
  mountedLibraryData: string;
  otherVolumeData: string;
  totalCapacity: string;
  capacityUnavailable: string;
  onlineRootStatus: string;
  offlineRootStatus: string;
  readOnlyRootStatus: string;
  rootDirectorySize: string;
  noStorageRoots: string;
  noRootsOnVolume: string;
  addStorageRoot: string;
  storageRootName: string;
  referencedMode: string;
  managedMode: string;
  selectingFolder: string;
  checkRoot: string;
  checkingRoot: string;
  scanRoot: string;
  cancelRootScan: string;
  rootScanQueued: string;
  rootScanning: string;
  rootWatching: string;
  rootScanCancelling: string;
  rootScanCancelled: string;
  rootScanInterrupted: string;
  rootScanFailed: string;
  rootScanProgress: (processed: number, discovered: number) => string;
  itemStatuses: string;
  catalogHealth: string;
  activeItems: string;
  needsReviewItems: string;
  missingItems: string;
  trashedItems: string;
  itemsWithoutAssets: string;
  unavailableFiles: string;
  offlineRoots: string;
  rootErrors: string;
  auditFindings: string;
  preservedFiles: string;
  representationsCount: string;
  metadataCount: string;
  rootMode: string;
  lastChecked: string;
  never: string;
  defaultRoot: string;
  setDefaultRoot: string;
  editRoot: string;
  saveRoot: string;
  cancelRootEdit: string;
  relocateRoot: string;
  relocateManagedRoot: string;
  replaceReferencedRoot: string;
  openRoot: string;
  removeRoot: string;
  removeRootConfirm: string;
  rootFiles: string;
  rootAssets: string;
  rootUsed: string;
  rootAvailable: string;
  readOnlyRoots: string;
  dateTimeValue: (value: string) => string;
  relativeTimeValue: (value: string, now?: number) => string;
  catalogValueLabel: (value: string) => string;
  operationKindLabel: (value: string) => string;
  operationStageLabel: (value?: string) => string;
  otherGroups: Record<LibraryOtherGroup, string>;
}

function translationSegment(value: string) {
  return value
    .trim()
    .toLocaleLowerCase()
    .replace(/[\s-]+/g, "_")
    .replace(/_([a-z0-9])/g, (_, character: string) => character.toUpperCase());
}

function humanizeCatalogValue(value: string) {
  if (!/[_-]/.test(value)) return value;
  const words = value.trim().replace(/[_-]+/g, " ");
  return words.charAt(0).toLocaleUpperCase() + words.slice(1);
}

function catalogValueTranslationSegment(value: string) {
  const normalized = value.trim().toLocaleLowerCase();
  if (normalized.startsWith("ffprobe:")) return "embeddedFileMetadata";
  if (normalized.startsWith("internal-media-probe:")) return "mediaAnalysis";
  if (normalized === "desktop-user" || normalized === "desktop-library") {
    return "desktopLibrary";
  }
  if (
    normalized === "local-music-metadata" ||
    normalized === "music.local-metadata-editor"
  ) {
    return "localMusicMetadata";
  }
  if (normalized.startsWith("remote-provider")) return "remoteProvider";
  if (normalized === "checksum-scanner") return "checksumScanner";
  return translationSegment(normalized);
}

export function formatLibraryRelativeTime(
  value: string,
  locale: string,
  now = Date.now(),
) {
  const raw = value.trim();
  if (!raw) return "";
  const parsed = Date.parse(raw);
  if (!Number.isFinite(parsed)) return "";

  const delta = parsed - now;
  const absoluteDelta = Math.abs(delta);
  const units: Array<{ unit: Intl.RelativeTimeFormatUnit; milliseconds: number }> = [
    { unit: "year", milliseconds: 365 * 24 * 60 * 60 * 1000 },
    { unit: "month", milliseconds: 30 * 24 * 60 * 60 * 1000 },
    { unit: "week", milliseconds: 7 * 24 * 60 * 60 * 1000 },
    { unit: "day", milliseconds: 24 * 60 * 60 * 1000 },
    { unit: "hour", milliseconds: 60 * 60 * 1000 },
    { unit: "minute", milliseconds: 60 * 1000 },
    { unit: "second", milliseconds: 1000 },
  ];
  const matchedUnit =
    units.find((candidate) => absoluteDelta >= candidate.milliseconds) ??
    units[units.length - 1]!;
  const amount = Math.round(delta / matchedUnit.milliseconds);

  try {
    return new Intl.RelativeTimeFormat(locale, {
      numeric: "auto",
      style: "short",
    }).format(amount, matchedUnit.unit);
  } catch {
    return new Intl.RelativeTimeFormat("en", {
      numeric: "auto",
      style: "short",
    }).format(amount, matchedUnit.unit);
  }
}

/**
 * Healthy availability is the normal Library state, so it should not compete
 * with type and recency metadata. Exceptional and lifecycle states remain
 * visible, especially missing/offline records that require user attention.
 */
export function shouldShowLibraryStatusBadge(status: string) {
  const normalized = status.trim().toLocaleLowerCase().replace(/-/g, "_");
  return !["active", "available", "ready"].includes(normalized);
}

export function libraryItemDisplayStatus(
  item: Pick<LibraryWorkspaceItem, "status" | "availability">,
) {
  const status = item.status.trim().toLocaleLowerCase();
  if (["trashed", "deleted", "archived"].includes(status)) {
    return item.status;
  }
  const availability = libraryItemAvailability(item);
  return availability !== "available"
    ? availability
    : item.status;
}

export function libraryItemAvailability(
  item: Pick<LibraryWorkspaceItem, "status" | "availability">,
) {
  const availability = item.availability?.trim().toLocaleLowerCase() ?? "";
  if (availability) return availability;
  const status = item.status.trim().toLocaleLowerCase();
  return status === "missing" || status === "trashed" || status === "deleted"
    ? "missing"
    : "available";
}

const OPERATION_STAGE_KEYS: Record<string, string> = {
  starting: "starting",
  preparing: "preparing",
  fetchingmetadata: "fetchingMetadata",
  transcoding: "transcoding",
  downloading: "downloading",
  downloadingvideo: "downloadingVideo",
  downloadingaudio: "downloadingAudio",
  downloadingsubtitles: "downloadingSubtitles",
  downloadingthumbnail: "downloadingThumbnail",
  muxing: "muxing",
  cleaningup: "cleaningUp",
  postprocessing: "postProcessing",
  queued: "queued",
  running: "running",
  completed: "completed",
  failed: "failed",
  canceled: "canceled",
  cancelled: "canceled",
};

export function createLibraryWorkspaceLabels(
  t: TFunction,
  locale = "en",
): LibraryWorkspaceLabels {
  const label = (key: string) => t(`xiadown.libraryCatalog.${key}`);
  const completedLabel = (key: string) => t(`xiadown.completed.${key}`);
  const itemCount = (count: number) => label("itemCount").replace("{count}", String(count));
  const pageUnit = completedLabel("pageSuffix") || completedLabel("page").replace(/^[\u7b2c]/u, "");
  const catalogValueLabel = (value: string) => {
    const raw = value.trim();
    if (!raw) return "–";
    switch (raw.toLocaleLowerCase()) {
      case "video": return label("video");
      case "audio": return label("audio");
      case "book":
      case "books": return label("books");
      case "image":
      case "images": return label("images");
      case "other":
      case "others": return label("others");
    }
    const key = `xiadown.libraryCatalog.valueLabels.${catalogValueTranslationSegment(raw)}`;
    const translated = t(key);
    return translated === key ? humanizeCatalogValue(raw) : translated;
  };
  return {
  locale,
  search: label("search"),
  ended: t("xiadown.views.ended"),
  all: label("all"),
  tasks: label("tasks"),
  video: label("video"),
  audio: label("audio"),
  books: label("books"),
  images: label("images"),
  others: label("others"),
  searchPlaceholder: label("searchPlaceholder"),
  sortLabel: label("sortLabel"),
  sortNewest: label("sortNewest"),
  sortOldest: label("sortOldest"),
  sortName: label("sortName"),
  sortSize: label("sortSize"),
  gridView: label("gridView"),
  listView: label("listView"),
  emptyTitle: label("emptyTitle"),
  emptyDescription: label("emptyDescription"),
  itemCount,
  pageRange: (start, end, total) => total > 0
    ? `${start}–${end} · ${itemCount(total)}`
    : itemCount(0),
  perPage: (pageSize) => `${completedLabel("perPage")} ${pageSize} ${completedLabel("itemUnit")}`,
  perPageUnit: `${completedLabel("itemUnit")}/${pageUnit}`,
  pageOf: (page, pageCount) => `${completedLabel("page")} ${page}/${pageCount} ${completedLabel("pageSuffix")}`.trim(),
  previousPage: completedLabel("previousPage"),
  nextPage: completedLabel("nextPage"),
  view: t("xiadown.actions.view"),
  cancelDialog: t("xiadown.actions.cancelDialog"),
  deleteItem: t("xiadown.actions.deleteItem"),
  reset: t("xiadown.actions.reset"),
  preview: label("preview"),
  info: label("info"),
  tags: label("tags"),
  relations: label("relations"),
  versions: label("versions"),
  metadata: label("metadata"),
  activity: label("activity"),
  noSelection: label("noSelection"),
  noSelectionDescription: label("noSelectionDescription"),
  noRelations: label("noRelations"),
  noVersions: label("noVersions"),
  noActivity: label("noActivity"),
  outputCurrent: label("outputCurrent"),
  outputHistorical: label("outputHistorical"),
  outputDetached: label("outputDetached"),
  operationRenamed: label("operationRenamed"),
  operationCanceled: label("operationCanceled"),
  operationResumed: label("operationResumed"),
  status: label("status"),
  statusLabel: (status) => {
    const normalized = status.trim().toLocaleLowerCase().replace(/-/g, "_");
    if (["active", "available", "ready"].includes(normalized)) return label("activeItems");
    if (["needs_review", "review"].includes(normalized)) return label("needsReviewItems");
    if (normalized === "offline") return catalogValueLabel("offline");
    if (normalized === "checking") return catalogValueLabel("processing");
    if (["missing", "unavailable"].includes(normalized)) return label("missingItems");
    if (["trashed", "deleted", "archived"].includes(normalized)) return label("trashedItems");
    if (normalized === "queued" || normalized === "pending") return t("library.status.queued");
    if (["running", "processing"].includes(normalized)) return t("library.status.running");
    if (["succeeded", "success", "completed", "complete"].includes(normalized)) {
      return t("library.status.succeeded");
    }
    if (["failed", "error", "corrupt"].includes(normalized)) return t("library.status.failed");
    if (["canceled", "cancelled", "paused", "stopped"].includes(normalized)) {
      return t("library.status.canceled");
    }
    return status.trim() || "–";
  },
  progress: label("progress"),
  format: label("format"),
  size: label("size"),
  duration: label("duration"),
  elapsed: t("xiadown.running.elapsed"),
  play: t("xiadown.listen.play"),
  pause: t("xiadown.listen.pause"),
  seek: t("xiadown.listen.seek"),
  mute: t("xiadown.listen.mute"),
  unmute: t("xiadown.listen.unmute"),
  volume: t("xiadown.listen.volume"),
  taskFiles: completedLabel("outputs"),
  taskNoFiles: completedLabel("taskNoFiles"),
  taskType: completedLabel("taskDataFields.kind"),
  downloadURL: completedLabel("taskDataFields.url"),
  sourceFile: completedLabel("taskDataFields.inputPath"),
  storageMode: label("storageMode"),
  downloadedSource: label("downloadedSource"),
  referencedImportSource: label("referencedImportSource"),
  managedImportSource: label("managedImportSource"),
  generatedSource: label("generatedSource"),
  unknownSource: label("unknownSource"),
  storageRoot: label("storageRoot"),
  importedAt: label("importedAt"),
  importBatch: label("importBatch"),
  associatedTask: label("associatedTask"),
  unmanagedMode: label("unmanagedMode"),
  copyDownloadURL: completedLabel("copyDownloadUrl"),
  openLocation: t("xiadown.actions.openDirectory"),
  renameTask: completedLabel("renameTask"),
  renameTaskNameLabel: completedLabel("renameTaskNameLabel"),
  renameTaskNamePlaceholder: completedLabel("renameTaskNamePlaceholder"),
  renameFile: completedLabel("renameFile"),
  renameFileNameLabel: completedLabel("renameFileNameLabel"),
  renameFileNamePlaceholder: completedLabel("renameFileNamePlaceholder"),
  renameNameRequired: completedLabel("renameNameRequired"),
  renameNameInvalid: completedLabel("renameNameInvalid"),
  renameNameTooLong: completedLabel("renameNameTooLong"),
  deleteTaskTitle: completedLabel("deleteTaskTitle"),
  deleteFileTitle: completedLabel("deleteFileTitle"),
  deleteFileMessage: (name) => completedLabel("deleteFileMessage").replace("{name}", name),
  deleteFiles: completedLabel("deleteFiles"),
  removeTaskOutputTitle: t("xiadown.completed.removeTaskOutputTitle"),
  removeTaskOutputMessage: (name) => t("xiadown.completed.removeTaskOutputMessage").replace("{name}", name),
  alsoDeleteFile: t("xiadown.completed.alsoDeleteFile"),
  location: label("location"),
  library: label("library"),
  created: label("created"),
  updated: label("updated"),
  title: label("title"),
  description: label("description"),
  category: label("category"),
  saveChanges: label("saveChanges"),
  trashItem: label("trashItem"),
  restoreItem: label("restoreItem"),
  revision: label("revision"),
  itemDetails: label("itemDetails"),
  noTags: label("noTags"),
  newTag: label("newTag"),
  tagPlaceholder: label("tagPlaceholder"),
  noCollections: label("noCollections"),
  collectionKind: label("collectionKind"),
  newCollection: label("newCollection"),
  collectionPlaceholder: label("collectionPlaceholder"),
  addToCollection: label("addToCollection"),
  removeFromCollection: label("removeFromCollection"),
  smartCollection: label("smartCollection"),
  smartCollectionReadonly: label("smartCollectionReadonly"),
  assets: label("assets"),
  representations: label("representations"),
  asset: label("asset"),
  role: label("role"),
  availability: label("availability"),
  mediaType: label("mediaType"),
  container: label("container"),
  codec: label("codec"),
  resolution: label("resolution"),
  bitrate: label("bitrate"),
  language: label("language"),
  checksum: label("checksum"),
  noAssets: label("noAssets"),
  noRepresentations: label("noRepresentations"),
  noMetadata: label("noMetadata"),
  addMetadata: label("addMetadata"),
  metadataNamespace: label("metadataNamespace"),
  metadataKey: label("metadataKey"),
  metadataValue: label("metadataValue"),
  savingMetadata: label("savingMetadata"),
  source: label("source"),
  provenance: label("provenance"),
  confidence: label("confidence"),
  locked: label("locked"),
  yes: label("yes"),
  no: label("no"),
  value: label("value"),
  auditHealthy: label("auditHealthy"),
  auditIssues: label("auditIssues"),
  catalogAudit: label("catalogAudit"),
  auditedAt: label("auditedAt"),
  loading: label("loading"),
  loadFailed: label("loadFailed"),
  retry: label("retry"),
  revisionConflict: label("revisionConflict"),
  health: label("health"),
  manage: label("manage"),
  managementTitle: label("managementTitle"),
  managementDescription: label("managementDescription"),
  summary: label("summary"),
  dataManagement: label("dataManagement"),
  librarySize: label("librarySize"),
  storageRoots: label("storageRoots"),
  storageOverview: label("storageOverview"),
  mountedCapacity: label("mountedCapacity"),
  storageLocationsOnline: (count) =>
    label("storageLocationsOnline").replace("{count}", String(count)),
  storageRootsSummary: (roots, offline) =>
    label("storageRootsSummary")
      .replace("{roots}", String(roots))
      .replace("{offline}", String(offline)),
  storageVolumeSummary: (roots, librarySize) =>
    label("storageVolumeSummary")
      .replace("{roots}", String(roots))
      .replace("{librarySize}", librarySize),
  storageAvailableOfTotal: (available, total) =>
    label("storageAvailableOfTotal")
      .replace("{available}", available)
      .replace("{total}", total),
  systemVolume: label("systemVolume"),
  mountedLibraryData: label("mountedLibraryData"),
  otherVolumeData: label("otherVolumeData"),
  totalCapacity: label("totalCapacity"),
  capacityUnavailable: label("capacityUnavailable"),
  onlineRootStatus: label("onlineRootStatus"),
  offlineRootStatus: label("offlineRootStatus"),
  readOnlyRootStatus: label("readOnlyRootStatus"),
  rootDirectorySize: label("rootDirectorySize"),
  noStorageRoots: label("noStorageRoots"),
  noRootsOnVolume: label("noRootsOnVolume"),
  addStorageRoot: label("addStorageRoot"),
  storageRootName: label("storageRootName"),
  referencedMode: label("referencedMode"),
  managedMode: label("managedMode"),
  selectingFolder: label("selectingFolder"),
  checkRoot: label("checkRoot"),
  checkingRoot: label("checkingRoot"),
  scanRoot: label("scanRoot"),
  cancelRootScan: label("cancelRootScan"),
  rootScanQueued: label("rootScanQueued"),
  rootScanning: label("rootScanning"),
  rootWatching: label("rootWatching"),
  rootScanCancelling: label("rootScanCancelling"),
  rootScanCancelled: label("rootScanCancelled"),
  rootScanInterrupted: label("rootScanInterrupted"),
  rootScanFailed: label("rootScanFailed"),
  rootScanProgress: (processed, discovered) =>
    label("rootScanProgress")
      .replace("{processed}", String(processed))
      .replace("{discovered}", String(discovered)),
  itemStatuses: label("itemStatuses"),
  catalogHealth: label("catalogHealth"),
  activeItems: label("activeItems"),
  needsReviewItems: label("needsReviewItems"),
  missingItems: label("missingItems"),
  trashedItems: label("trashedItems"),
  itemsWithoutAssets: label("itemsWithoutAssets"),
  unavailableFiles: label("unavailableFiles"),
  offlineRoots: label("offlineRoots"),
  rootErrors: label("rootErrors"),
  auditFindings: label("auditFindings"),
  preservedFiles: label("preservedFiles"),
  representationsCount: label("representationsCount"),
  metadataCount: label("metadataCount"),
  rootMode: label("rootMode"),
  lastChecked: label("lastChecked"),
  never: label("never"),
  defaultRoot: label("defaultRoot"),
  setDefaultRoot: label("setDefaultRoot"),
  editRoot: label("editRoot"),
  saveRoot: label("saveRoot"),
  cancelRootEdit: label("cancelRootEdit"),
  relocateRoot: label("relocateRoot"),
  relocateManagedRoot: label("relocateManagedRoot"),
  replaceReferencedRoot: label("replaceReferencedRoot"),
  openRoot: label("openRoot"),
  removeRoot: label("removeRoot"),
  removeRootConfirm: label("removeRootConfirm"),
  rootFiles: label("rootFiles"),
  rootAssets: label("rootAssets"),
  rootUsed: label("rootUsed"),
  rootAvailable: label("rootAvailable"),
  readOnlyRoots: label("readOnlyRoots"),
  dateTimeValue: (value) => {
    const parsed = Date.parse(value);
    return Number.isFinite(parsed)
      ? new Intl.DateTimeFormat(locale, {
          dateStyle: "medium",
          timeStyle: "short",
        }).format(parsed)
      : value;
  },
  relativeTimeValue: (value, now) => formatLibraryRelativeTime(value, locale, now),
  catalogValueLabel,
  operationKindLabel: (value) => {
    const raw = value.trim();
    if (!raw) return "";
    const normalized = raw.toLocaleLowerCase();
    if (normalized.includes("download")) return t("xiadown.running.downloadBadge");
    if (normalized.includes("transcode")) return t("xiadown.running.transcodeBadge");
    if (normalized.includes("import")) return t("xiadown.libraryData.importTab");
    return catalogValueLabel(raw);
  },
  operationStageLabel: (value) => {
    const raw = value?.trim() ?? "";
    if (!raw) return "";
    const resolved = resolveI18nText(raw, normalizeLanguage(locale));
    if (resolved !== raw) return resolved;
    const normalized = translationSegment(raw).toLocaleLowerCase();
    const stageKey = OPERATION_STAGE_KEYS[normalized];
    return stageKey ? t(`xiadown.running.stageLabels.${stageKey}`) : raw;
  },
  otherGroups: {
    document: label("otherGroups.document"),
    font: label("otherGroups.font"),
    archive: label("otherGroups.archive"),
    subtitle: label("otherGroups.subtitle"),
    manifest: label("otherGroups.manifest"),
    api: label("otherGroups.api"),
    unknown: label("otherGroups.unknown"),
    "needs-review": label("otherGroups.needsReview"),
    missing: label("otherGroups.missing"),
  },
  };
}
