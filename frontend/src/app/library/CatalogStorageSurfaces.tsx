import {
  AlertTriangle,
  CircleStop,
  Folder,
  FolderOpen,
  HardDrive,
  HeartPulse,
  MapPin,
  MoreHorizontal,
  ScanSearch,
  Star,
  Trash2,
} from "lucide-react";
import * as React from "react";

import type {
  CatalogStorageRoot,
  CatalogStorageRootSyncState,
  CatalogStorageVolume,
} from "@/shared/contracts/catalog";
import { Button } from "@/shared/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/shared/ui/dropdown-menu";
import { StorageCapacityBar } from "@/shared/ui/storage-capacity";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/shared/ui/popover";
import {
  StatusBadge,
  type DreamStatusTone,
} from "@/shared/ui/status-badge";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/shared/ui/tooltip";
import { formatBytes } from "@/shared/utils/formatBytes";

import type { LibraryWorkspaceLabels } from "./types";

const OFFLINE_ROOT_STATUSES = new Set([
  "disconnected",
  "error",
  "failed",
  "missing",
  "offline",
  "unavailable",
]);
const READ_ONLY_ROOT_STATUSES = new Set([
  "read-only",
  "read_only",
  "readonly",
]);
const ONLINE_ROOT_STATUSES = new Set([
  "active",
  "available",
  "healthy",
  "online",
  "ready",
]);

const CatalogStorageRootEmojiPicker = React.lazy(
  () => import("./CatalogStorageRootEmojiPicker"),
);

function normalizedRootStatus(root: CatalogStorageRoot) {
  return root.status.trim().toLocaleLowerCase();
}

export function isCatalogStorageRootMounted(root: CatalogStorageRoot) {
  return !OFFLINE_ROOT_STATUSES.has(normalizedRootStatus(root));
}

function formatStorageBytes(value: number) {
  return value === 0 ? "0 B" : formatBytes(value);
}

interface CatalogStorageVolumeSummary extends CatalogStorageVolume {
  availableBytes: number;
  audioCount: number;
  fileCount: number;
  id: string;
  libraryBytes: number;
  rootCount: number;
  totalBytes: number;
  videoCount: number;
}

export interface CatalogStorageSummary {
  assetCount: number;
  availableBytes: number;
  fileCount: number;
  libraryBytes: number;
  mountedLibraryBytes: number;
  mountedRootCount: number;
  offlineRootCount: number;
  otherBytes: number;
  totalBytes: number;
  volumes: CatalogStorageVolumeSummary[];
}

/**
 * Starts with the operating system's mounted-volume inventory, then projects
 * every online storage root into its physical volume. Volumes without roots
 * deliberately remain in the result so Overview describes the computer,
 * rather than only the folders already registered with Library.
 */
export function buildCatalogStorageSummary(
  mountedVolumes: CatalogStorageVolume[],
  roots: CatalogStorageRoot[],
): CatalogStorageSummary {
  const volumesById = new Map<string, CatalogStorageVolumeSummary>();
  const volumesByMountPath = new Map<string, CatalogStorageVolumeSummary>();
  let assetCount = 0;
  let fileCount = 0;
  let libraryBytes = 0;
  let mountedRootCount = 0;
  let offlineRootCount = 0;

  for (const volume of mountedVolumes) {
    const id = volume.id.trim();
    const mountPath = volume.mountPath.trim();
    if (!id || !mountPath || volume.totalBytes <= 0) continue;
    const summary = {
      ...volume,
      id,
      mountPath,
      availableBytes: Math.max(0, volume.availableBytes),
      audioCount: 0,
      fileCount: 0,
      libraryBytes: 0,
      rootCount: 0,
      totalBytes: Math.max(0, volume.totalBytes),
      videoCount: 0,
    };
    volumesById.set(id, summary);
    volumesByMountPath.set(catalogStorageVolumeMountKey(mountPath), summary);
  }

  for (const root of roots) {
    assetCount += Math.max(0, root.assetCount);
    fileCount += Math.max(0, root.fileCount);
    libraryBytes += Math.max(0, root.sizeBytes);
    if (!isCatalogStorageRootMounted(root)) {
      offlineRootCount += 1;
      continue;
    }
    mountedRootCount += 1;

    const volumeKey = root.volumeId?.trim() || root.id;
    const rootMountPath = catalogStorageVolumeMountPath(root.path);
    let current = volumesById.get(volumeKey) ??
      volumesByMountPath.get(catalogStorageVolumeMountKey(rootMountPath));
    if (!current) {
      current = {
        availableBytes: Math.max(0, root.availableBytes ?? 0),
        audioCount: Math.max(0, root.audioCount),
        fileCount: Math.max(0, root.fileCount),
        id: volumeKey,
        kind: "unknown",
        libraryBytes: Math.max(0, root.sizeBytes),
        mountPath: rootMountPath || root.path,
        name: "",
        readOnly: false,
        rootCount: 1,
        totalBytes: Math.max(0, root.totalBytes ?? 0),
        videoCount: Math.max(0, root.videoCount),
      };
      volumesById.set(volumeKey, current);
      volumesByMountPath.set(
        catalogStorageVolumeMountKey(current.mountPath),
        current,
      );
      continue;
    }

    current.libraryBytes += Math.max(0, root.sizeBytes);
    current.audioCount += Math.max(0, root.audioCount);
    current.fileCount += Math.max(0, root.fileCount);
    current.rootCount += 1;
    current.videoCount += Math.max(0, root.videoCount);
  }

  const volumes = [...volumesById.values()].map((volume) => {
    const totalBytes = Math.max(0, volume.totalBytes);
    const availableBytes = Math.min(
      totalBytes,
      Math.max(0, volume.availableBytes),
    );
    const usedBytes = Math.max(0, totalBytes - availableBytes);
    return {
      ...volume,
      availableBytes,
      libraryBytes: Math.min(usedBytes, Math.max(0, volume.libraryBytes)),
      totalBytes,
    };
  });
  const totalBytes = volumes.reduce(
    (total, volume) => total + volume.totalBytes,
    0,
  );
  const availableBytes = volumes.reduce(
    (total, volume) => total + volume.availableBytes,
    0,
  );
  const mountedLibraryBytes = volumes.reduce(
    (total, volume) => total + volume.libraryBytes,
    0,
  );
  const otherBytes = Math.max(
    0,
    totalBytes - availableBytes - mountedLibraryBytes,
  );

  return {
    assetCount,
    availableBytes,
    fileCount,
    libraryBytes,
    mountedLibraryBytes,
    mountedRootCount,
    offlineRootCount,
    otherBytes,
    totalBytes,
    volumes,
  };
}

function catalogStorageVolumeMountPath(path: string) {
  const trimmedPath = path.trim();
  const windowsDrive = /^([a-zA-Z]:)[\\/]/.exec(trimmedPath);
  if (windowsDrive?.[1]) return `${windowsDrive[1].toUpperCase()}\\`;

  const macOSVolume = /^\/Volumes\/[^/]+/.exec(trimmedPath);
  if (macOSVolume?.[0]) return macOSVolume[0];

  const linuxUserVolume = /^\/run\/media\/[^/]+\/[^/]+/.exec(trimmedPath);
  if (linuxUserVolume?.[0]) return linuxUserVolume[0];

  const linuxVolume = /^\/(?:media|mnt)\/[^/]+/.exec(trimmedPath);
  if (linuxVolume?.[0]) return linuxVolume[0];

  return trimmedPath.startsWith("/") ? "/" : "";
}

function catalogStorageVolumeMountKey(path: string) {
  const trimmedPath = path.trim();
  return /^[a-zA-Z]:\\$/.test(trimmedPath)
    ? trimmedPath.toLocaleLowerCase()
    : trimmedPath;
}

function catalogStorageVolumeName(
  volume: CatalogStorageVolume,
  labels: LibraryWorkspaceLabels,
) {
  const name = volume.name?.trim() ?? "";
  if (name) return name;
  const mountPath = volume.mountPath.trim();
  if (!mountPath || mountPath === "/") return labels.systemVolume;
  if (/^[A-Z]:\\$/.test(mountPath)) return mountPath.slice(0, 2);
  const segments = mountPath.split("/").filter(Boolean);
  return segments[segments.length - 1] ?? mountPath;
}

export function CatalogStorageOverview(props: {
  catalog: {
    description: string;
    name: string;
    statusLabel: string;
  };
  labels: LibraryWorkspaceLabels;
  metrics: ReadonlyArray<readonly [string, number | string]>;
  roots: CatalogStorageRoot[];
  volumes: CatalogStorageVolume[];
}) {
  const summary = buildCatalogStorageSummary(props.volumes, props.roots);
  const capacityKnown = summary.totalBytes > 0;

  return (
    <article
      aria-label={props.labels.storageOverview}
      className="app-dream-storage-overview"
      data-capacity-known={capacityKnown ? "true" : "false"}
      data-catalog-storage-overview="true"
    >
      <header className="app-dream-storage-overview__header">
        <div className="app-dream-storage-overview__identity">
          <span className="app-dream-storage-overview__eyebrow">
            {props.labels.storageOverview}
          </span>
          <h3>{props.catalog.name}</h3>
          {props.catalog.description ? (
            <p>{props.catalog.description}</p>
          ) : null}
        </div>
        <StatusBadge
          className="app-dream-storage-overview__catalog-status"
          tone="neutral"
        >
          {props.catalog.statusLabel}
        </StatusBadge>
      </header>

      <dl className="app-dream-storage-overview__library-metrics">
        {props.metrics.map(([label, value]) => (
          <div key={label}>
            <strong>{value}</strong>
            <span>{label}</span>
          </div>
        ))}
      </dl>

      <section className="app-dream-storage-overview__capacity-section">
        <header className="app-dream-storage-overview__volumes-heading">
          <div className="app-dream-storage-overview__capacity-copy">
            <span className="app-dream-storage-overview__eyebrow">
              {props.labels.mountedCapacity}
            </span>
            <h4>
              {props.labels.storageLocationsOnline(
                summary.volumes.length,
              )}
            </h4>
          </div>
          <p>
            {props.labels.storageRootsSummary(
              props.roots.length,
              summary.offlineRootCount,
            )}
          </p>
        </header>

        {summary.volumes.length > 0 ? (
          <div className="app-dream-storage-overview__volumes">
            {summary.volumes.map((volume) => {
              const volumeCapacityKnown = volume.totalBytes > 0;
              const volumeUsedBytes = Math.max(
                0,
                volume.totalBytes - volume.availableBytes,
              );
              const volumeOtherBytes = Math.max(
                0,
                volumeUsedBytes - volume.libraryBytes,
              );
              return (
                <div className="app-dream-storage-volume" key={volume.id}>
                  <header className="app-dream-storage-volume__header">
                    <span className="app-dream-storage-volume__identity">
                      <span className="app-dream-storage-volume__icon">
                        <HardDrive aria-hidden="true" />
                      </span>
                      <strong>
                        {catalogStorageVolumeName(volume, props.labels)}
                      </strong>
                    </span>
                    <strong className="app-dream-storage-volume__capacity">
                      {volumeCapacityKnown
                        ? props.labels.storageAvailableOfTotal(
                            formatStorageBytes(volume.availableBytes),
                            formatStorageBytes(volume.totalBytes),
                          )
                        : props.labels.capacityUnavailable}
                    </strong>
                  </header>
                  <StorageCapacityBar
                    availableBytes={volume.availableBytes}
                    label={catalogStorageVolumeName(
                      volume,
                      props.labels,
                    )}
                    libraryBytes={volume.libraryBytes}
                    otherBytes={volumeOtherBytes}
                    totalBytes={volume.totalBytes}
                    valueText={
                      volumeCapacityKnown
                        ? `${formatStorageBytes(volumeUsedBytes)} ${props.labels.rootUsed}`
                        : props.labels.capacityUnavailable
                    }
                  />
                  {volume.rootCount > 0 ? (
                    <dl className="app-dream-storage-volume__facts">
                      <div>
                        <span>{props.labels.librarySize}</span>
                        <strong>
                          {formatStorageBytes(volume.libraryBytes)}
                        </strong>
                      </div>
                      <div>
                        <span>{props.labels.rootFiles}</span>
                        <strong>
                          {volume.fileCount.toLocaleString(
                            props.labels.locale,
                          )}
                        </strong>
                      </div>
                      <div>
                        <span>{props.labels.video}</span>
                        <strong>
                          {volume.videoCount.toLocaleString(
                            props.labels.locale,
                          )}
                        </strong>
                      </div>
                      <div>
                        <span>{props.labels.audio}</span>
                        <strong>
                          {volume.audioCount.toLocaleString(
                            props.labels.locale,
                          )}
                        </strong>
                      </div>
                    </dl>
                  ) : (
                    <p className="app-dream-storage-volume__empty">
                      <Folder aria-hidden="true" />
                      {props.labels.noRootsOnVolume}
                    </p>
                  )}
                </div>
              );
            })}
          </div>
        ) : null}
      </section>
    </article>
  );
}

function rootPresentation(
  root: CatalogStorageRoot,
  labels: LibraryWorkspaceLabels,
): {
  label: string;
  state: "offline" | "online";
  tone: DreamStatusTone;
} {
  const status = normalizedRootStatus(root);
  if (OFFLINE_ROOT_STATUSES.has(status)) {
    return {
      label: labels.offlineRootStatus,
      state: "offline",
      tone: "danger",
    };
  }
  if (root.lastError) {
    return {
      label: labels.rootErrors,
      state: "online",
      tone: "danger",
    };
  }
  if (READ_ONLY_ROOT_STATUSES.has(status)) {
    return {
      label: labels.readOnlyRootStatus,
      state: "online",
      tone: "warning",
    };
  }
  if (ONLINE_ROOT_STATUSES.has(status)) {
    return {
      label: labels.onlineRootStatus,
      state: "online",
      tone: "success",
    };
  }
  return {
    label: labels.statusLabel(root.status),
    state: "online",
    tone: "neutral",
  };
}

export interface CatalogStorageRootCardProps {
  busy: {
    check: boolean;
    relocate: boolean;
    remove: boolean;
    scan: boolean;
    emoji: boolean;
  };
  emojiPickerOpen: boolean;
  labels: LibraryWorkspaceLabels;
  onCheck: () => void;
  onCancelScan: () => void;
  onOpen: () => void;
  onRelocate: () => void;
  onRemove: () => void;
  onScan: () => void;
  onEmojiChange: (emoji: string) => void;
  onEmojiPickerOpenChange: (open: boolean) => void;
  root: CatalogStorageRoot;
  syncState?: CatalogStorageRootSyncState;
}

export function CatalogStorageRootCard(
  props: CatalogStorageRootCardProps,
) {
  const { labels, root } = props;
  const presentation = rootPresentation(root, labels);
  const locationPath = root.locationPath || root.path;
  const syncState = props.syncState;
  const scanActive = syncState?.status === "queued" ||
    syncState?.status === "scanning" ||
    syncState?.status === "cancelling";
  const syncLabel = syncState
    ? rootSyncStatusLabel(syncState, labels)
    : "";
  const showSyncProgress = syncState?.status === "queued" ||
    syncState?.status === "scanning" ||
    syncState?.status === "cancelling" ||
    syncState?.status === "failed" ||
    syncState?.status === "interrupted" ||
    syncState?.status === "cancelled";
  const relocationLabel =
    root.mode === "managed"
      ? labels.relocateManagedRoot
      : labels.replaceReferencedRoot;
  const emojiTriggerRef = React.useRef<HTMLButtonElement>(null);
  const emojiPortalContainer = props.emojiPickerOpen
    ? emojiTriggerRef.current?.closest<HTMLElement>('[role="dialog"]') ?? null
    : null;

  return (
    <article
      className="app-dream-storage-root-card app-catalog-storage-root-card"
      data-default={root.isDefault ? "true" : undefined}
      data-root-id={root.id}
      data-state={presentation.state}
    >
      <div className="app-dream-storage-root-card__information">
        <header className="app-dream-storage-root-card__header">
          <div className="app-dream-storage-root-card__main">
            <Popover
              open={props.emojiPickerOpen}
              onOpenChange={props.onEmojiPickerOpenChange}
            >
              <Tooltip>
                <TooltipTrigger asChild>
                  <PopoverTrigger asChild>
                    <button
                      ref={emojiTriggerRef}
                      aria-label={`${labels.editRoot}: ${root.name}`}
                      className="app-dream-storage-root-card__device"
                      disabled={props.busy.emoji}
                      type="button"
                    >
                      <span aria-hidden="true">{root.emoji || "📁"}</span>
                    </button>
                  </PopoverTrigger>
                </TooltipTrigger>
                <TooltipContent>{labels.editRoot}</TooltipContent>
              </Tooltip>
              <PopoverContent
                align="start"
                className="app-storage-root-emoji-popover"
                portalContainer={emojiPortalContainer}
              >
                {props.emojiPickerOpen ? (
                  <React.Suspense
                    fallback={(
                      <div className="app-storage-root-emoji-picker__state">
                        {labels.loading}
                      </div>
                    )}
                  >
                    <CatalogStorageRootEmojiPicker
                      labels={labels}
                      onEmojiSelect={(emoji) => {
                        props.onEmojiPickerOpenChange(false);
                        props.onEmojiChange(emoji);
                      }}
                    />
                  </React.Suspense>
                ) : null}
              </PopoverContent>
            </Popover>
            <div className="app-dream-storage-root-card__identity">
              <div className="app-dream-storage-root-card__path">
                <h4>
                  <code title={locationPath}>{locationPath}</code>
                </h4>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      aria-label={`${labels.openRoot}: ${locationPath}`}
                      onClick={props.onOpen}
                      shape="square"
                      size="compactIcon"
                      type="button"
                      variant="ghost"
                    >
                      <FolderOpen aria-hidden="true" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>{labels.openRoot}</TooltipContent>
                </Tooltip>
              </div>
              <div className="app-dream-storage-root-card__meta">
                {root.isDefault ? (
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <span
                        aria-label={labels.defaultRoot}
                        className="app-dream-storage-root-card__default"
                        role="img"
                        tabIndex={0}
                      >
                        <Star aria-hidden="true" />
                      </span>
                    </TooltipTrigger>
                    <TooltipContent>{labels.defaultRoot}</TooltipContent>
                  </Tooltip>
                ) : null}
                <span className="app-dream-storage-root-card__mode">
                  {root.mode === "managed"
                    ? labels.managedMode
                    : labels.referencedMode}
                  {syncState?.status === "watching"
                    ? ` · ${syncLabel}`
                    : ""}
                </span>
              </div>
            </div>
          </div>
          <div className="app-dream-storage-root-card__header-actions">
            <StatusBadge
              marker
              title={`${labels.lastChecked}: ${
                root.lastCheckedAt
                  ? labels.dateTimeValue(root.lastCheckedAt)
                  : labels.never
              }`}
              tone={presentation.tone}
            >
              {presentation.label}
            </StatusBadge>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  aria-label={`${labels.checkRoot}: ${root.name}`}
                  disabled={props.busy.check}
                  onClick={props.onCheck}
                  shape="square"
                  size="compactIcon"
                  type="button"
                  variant="ghost"
                >
                  <HeartPulse
                    aria-hidden="true"
                    className={
                      props.busy.check ? "app-motion-spin" : undefined
                    }
                  />
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                {props.busy.check
                  ? labels.checkingRoot
                  : labels.checkRoot}
              </TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  aria-label={`${scanActive ? labels.cancelRootScan : labels.scanRoot}: ${root.name}`}
                  disabled={props.busy.scan || syncState?.status === "cancelling"}
                  onClick={scanActive ? props.onCancelScan : props.onScan}
                  shape="square"
                  size="compactIcon"
                  type="button"
                  variant="ghost"
                >
                  {scanActive ? (
                    <CircleStop aria-hidden="true" />
                  ) : (
                    <ScanSearch aria-hidden="true" />
                  )}
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                {scanActive ? labels.cancelRootScan : labels.scanRoot}
              </TooltipContent>
            </Tooltip>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button
                  aria-label={`${labels.manage}: ${root.name}`}
                  className="app-dream-storage-root-card__menu-trigger"
                  shape="square"
                  size="compactIcon"
                  variant="ghost"
                >
                  <MoreHorizontal aria-hidden="true" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="center">
                <DropdownMenuLabel>{labels.manage}</DropdownMenuLabel>
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  disabled={props.busy.relocate}
                  onSelect={props.onRelocate}
                >
                  <MapPin aria-hidden="true" />
                  {relocationLabel}
                </DropdownMenuItem>
                {!root.isDefault ? (
                  <>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem
                      disabled={props.busy.remove}
                      onSelect={props.onRemove}
                      tone="destructive"
                    >
                      <Trash2 aria-hidden="true" />
                      {labels.removeRoot}
                    </DropdownMenuItem>
                  </>
                ) : null}
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </header>

        {root.lastError ? (
          <p className="app-catalog-roots__error" role="alert">
            <AlertTriangle aria-hidden="true" />
            {root.lastError}
          </p>
        ) : null}
        {showSyncProgress && syncState ? (
          <div
            className="app-dream-storage-root-card__sync"
            data-state={syncState.status}
            role={syncState.status === "failed" ? "alert" : "status"}
          >
            <div className="app-dream-storage-root-card__sync-copy">
              <strong>{syncLabel}</strong>
              <span>
                {labels.rootScanProgress(
                  syncState.processedCount,
                  syncState.discoveredCount,
                )}
              </span>
            </div>
            {scanActive ? (
              <progress
                aria-label={syncLabel}
                max={Math.max(1, syncState.discoveredCount)}
                value={
                  syncState.processedCount < syncState.discoveredCount
                    ? syncState.processedCount
                    : undefined
                }
              />
            ) : null}
          </div>
        ) : null}
      </div>

      <dl className="app-dream-storage-root-card__facts">
        <div className="app-dream-storage-root-card__fact">
          <span>{labels.rootDirectorySize}</span>
          <strong>{formatStorageBytes(root.sizeBytes)}</strong>
        </div>
        <div className="app-dream-storage-root-card__fact">
          <span>{labels.rootFiles}</span>
          <strong>{root.fileCount.toLocaleString(labels.locale)}</strong>
        </div>
        <div className="app-dream-storage-root-card__fact">
          <span>{labels.video}</span>
          <strong>{root.videoCount.toLocaleString(labels.locale)}</strong>
        </div>
        <div className="app-dream-storage-root-card__fact">
          <span>{labels.audio}</span>
          <strong>{root.audioCount.toLocaleString(labels.locale)}</strong>
        </div>
      </dl>
    </article>
  );
}

function rootSyncStatusLabel(
  state: CatalogStorageRootSyncState,
  labels: LibraryWorkspaceLabels,
) {
  switch (state.status) {
    case "queued":
      return labels.rootScanQueued;
    case "scanning":
      return labels.rootScanning;
    case "watching":
      return labels.rootWatching;
    case "cancelling":
      return labels.rootScanCancelling;
    case "cancelled":
      return labels.rootScanCancelled;
    case "interrupted":
      return labels.rootScanInterrupted;
    case "failed":
      return labels.rootScanFailed;
    default:
      return labels.scanRoot;
  }
}
