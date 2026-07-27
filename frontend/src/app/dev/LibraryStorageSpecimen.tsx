import {
  CircleStop,
  Folder,
  FolderOpen,
  HardDrive,
  HeartPulse,
  Import,
  MapPin,
  MoreHorizontal,
  Plus,
  ScanSearch,
  Star,
  Trash2,
} from "lucide-react";
import * as React from "react";

import CatalogStorageRootEmojiPicker from "@/app/library/CatalogStorageRootEmojiPicker";
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
import { StatusBadge, type DreamStatusTone } from "@/shared/ui/status-badge";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/shared/ui/tooltip";

type StorageRootState = "online" | "offline";

interface StorageRootSpecimen {
  id: string;
  emoji: string;
  name: string;
  mode: string;
  state: StorageRootState;
  stateLabel: string;
  stateTone: DreamStatusTone;
  isDefault?: boolean;
  macosPath: string;
  windowsPath: string;
  audio: string;
  directorySize: string;
  files: string;
  video: string;
  scanning?: boolean;
}

const STORAGE_ROOTS: StorageRootSpecimen[] = [
  {
    id: "downloads",
    emoji: "📁",
    name: "XiaDown Downloads",
    mode: "Managed · system Downloads folder",
    state: "online",
    stateLabel: "Online",
    stateTone: "success",
    isDefault: true,
    macosPath: "~/Downloads",
    windowsPath: "C:\\Users\\Arnold\\Downloads",
    audio: "4,821",
    directorySize: "286 GB",
    files: "14,036",
    video: "6,942",
  },
  {
    id: "creator-ssd",
    emoji: "🎬",
    name: "Creator Projects",
    mode: "Referenced directory",
    state: "online",
    stateLabel: "Online",
    stateTone: "success",
    macosPath: "/Volumes/Creator SSD/XiaDown Library",
    windowsPath: "D:\\XiaDown Library",
    audio: "305",
    directorySize: "200 GB",
    files: "4,612",
    video: "900",
    scanning: true,
  },
  {
    id: "archive",
    emoji: "💿",
    name: "Archive Library",
    mode: "Referenced directory",
    state: "offline",
    stateLabel: "Offline",
    stateTone: "danger",
    macosPath: "/Volumes/Archive/XiaDown",
    windowsPath: "E:\\XiaDown",
    audio: "0",
    directorySize: "128 GB",
    files: "2,167",
    video: "0",
  },
];

function PlatformPath({
  macos,
  windows,
}: {
  macos: string;
  windows: string;
}) {
  return (
    <code>
      <span
        className="app-dream-storage-platform-path"
        data-platform-path="macos"
      >
        {macos}
      </span>
      <span
        className="app-dream-storage-platform-path"
        data-platform-path="windows"
      >
        {windows}
      </span>
    </code>
  );
}

function StorageRootMenu({
  isDefault,
  mode,
  name,
}: {
  isDefault?: boolean;
  mode: string;
  name: string;
}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          aria-label={`Manage ${name}`}
          className="app-dream-storage-root-card__menu-trigger"
          shape="square"
          size="compactIcon"
          variant="ghost"
        >
          <MoreHorizontal aria-hidden="true" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="center">
        <DropdownMenuLabel>Manage root</DropdownMenuLabel>
        <DropdownMenuSeparator />
            <DropdownMenuItem onSelect={() => undefined}>
          <MapPin aria-hidden="true" />
          {mode.startsWith("Managed")
            ? "Relocate managed folder"
            : "Choose another folder"}
        </DropdownMenuItem>
        {!isDefault ? (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuItem onSelect={() => undefined} tone="destructive">
              <Trash2 aria-hidden="true" />
              Remove root
            </DropdownMenuItem>
          </>
        ) : null}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function StorageRootCard({ root }: { root: StorageRootSpecimen }) {
  const [emoji, setEmoji] = React.useState(root.emoji);
  const [emojiPickerOpen, setEmojiPickerOpen] = React.useState(false);
  return (
    <article
      className="app-dream-storage-root-card"
      data-default={root.isDefault ? "true" : undefined}
      data-root-id={root.id}
      data-state={root.state}
    >
      <div className="app-dream-storage-root-card__information">
        <header className="app-dream-storage-root-card__header">
          <div className="app-dream-storage-root-card__main">
            <Popover
              open={emojiPickerOpen}
              onOpenChange={setEmojiPickerOpen}
            >
              <PopoverTrigger asChild>
                <button
                  aria-label={`Change icon for ${root.name}`}
                  className="app-dream-storage-root-card__device"
                  type="button"
                >
                  <span aria-hidden="true">{emoji}</span>
                </button>
              </PopoverTrigger>
              <PopoverContent
                align="start"
                className="app-storage-root-emoji-popover"
              >
                <CatalogStorageRootEmojiPicker
                  labels={{
                    loading: "Loading",
                    search: "Search",
                  }}
                  onEmojiSelect={(nextEmoji) => {
                    setEmoji(nextEmoji);
                    setEmojiPickerOpen(false);
                  }}
                />
              </PopoverContent>
            </Popover>
            <div className="app-dream-storage-root-card__identity">
              <div className="app-dream-storage-root-card__path">
                <h4>
                  <PlatformPath
                    macos={root.macosPath}
                    windows={root.windowsPath}
                  />
                </h4>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      aria-label={`Open ${root.name}`}
                      shape="square"
                      size="compactIcon"
                      variant="ghost"
                    >
                      <FolderOpen aria-hidden="true" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>Open folder</TooltipContent>
                </Tooltip>
              </div>
              <div className="app-dream-storage-root-card__meta">
                {root.isDefault ? (
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <span
                        aria-label="Current default"
                        className="app-dream-storage-root-card__default"
                        role="img"
                        tabIndex={0}
                      >
                        <Star aria-hidden="true" />
                      </span>
                    </TooltipTrigger>
                    <TooltipContent>Current default</TooltipContent>
                  </Tooltip>
                ) : null}
                <span className="app-dream-storage-root-card__mode">
                  {root.mode}
                </span>
              </div>
            </div>
          </div>
          <div className="app-dream-storage-root-card__header-actions">
            <StatusBadge marker tone={root.stateTone}>
              {root.stateLabel}
            </StatusBadge>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  aria-label={`Check ${root.name}`}
                  shape="square"
                  size="compactIcon"
                  variant="ghost"
                >
                  <HeartPulse aria-hidden="true" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>Check now</TooltipContent>
            </Tooltip>
            {!root.mode.startsWith("Managed") ? (
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    aria-label={`${root.scanning ? "Cancel scan" : "Scan"} ${root.name}`}
                    shape="square"
                    size="compactIcon"
                    variant="ghost"
                  >
                    {root.scanning ? (
                      <CircleStop aria-hidden="true" />
                    ) : (
                      <ScanSearch aria-hidden="true" />
                    )}
                  </Button>
                </TooltipTrigger>
                <TooltipContent>
                  {root.scanning ? "Cancel scan" : "Scan contents"}
                </TooltipContent>
              </Tooltip>
            ) : null}
            <StorageRootMenu
              isDefault={root.isDefault}
              mode={root.mode}
              name={root.name}
            />
          </div>
        </header>
        {root.scanning ? (
          <div
            className="app-dream-storage-root-card__sync"
            data-state="scanning"
            role="status"
          >
            <div className="app-dream-storage-root-card__sync-copy">
              <strong>Scanning</strong>
              <span>3,248 processed / 4,612 discovered</span>
            </div>
            <progress
              aria-label="Scanning Creator Projects"
              max={4612}
              value={3248}
            />
          </div>
        ) : null}
      </div>

      <dl className="app-dream-storage-root-card__facts">
        <div className="app-dream-storage-root-card__fact">
          <span>Directory size</span>
          <strong>{root.directorySize}</strong>
        </div>
        <div className="app-dream-storage-root-card__fact">
          <span>Files</span>
          <strong>{root.files}</strong>
        </div>
        <div className="app-dream-storage-root-card__fact">
          <span>Video</span>
          <strong>{root.video}</strong>
        </div>
        <div className="app-dream-storage-root-card__fact">
          <span>Audio</span>
          <strong>{root.audio}</strong>
        </div>
      </dl>
    </article>
  );
}

export function LibraryStorageSpecimen() {
  return (
    <section
      aria-labelledby="appearance-lab-storage-title"
      className="appearance-lab__section"
      data-appearance-fixture="library-storage-contract"
    >
      <div className="appearance-lab__section-heading">
        <span>06</span>
        <div>
          <h2 id="appearance-lab-storage-title">Library storage system</h2>
          <p>
            Mounted-volume usage and whole-library totals stay in the overview;
            root cards describe directories only.
          </p>
        </div>
      </div>

      <div className="app-dream-storage-system">
        <article
          className="app-dream-storage-overview"
          data-storage-component="overview"
        >
          <header className="app-dream-storage-overview__header">
            <div className="app-dream-storage-overview__identity">
              <span className="app-dream-storage-overview__eyebrow">
                Library storage system
              </span>
              <h3>XiaDown Library</h3>
              <p>Default catalog · local asset index</p>
            </div>
            <StatusBadge
              className="app-dream-storage-overview__catalog-status"
              tone="neutral"
            >
              Active
            </StatusBadge>
          </header>

          <dl className="app-dream-storage-overview__library-metrics">
            <div>
              <strong>614 GB</strong>
              <span>Library size</span>
            </div>
            <div>
              <strong>18,906</strong>
              <span>All</span>
            </div>
            <div>
              <strong>7,842</strong>
              <span>Video</span>
            </div>
            <div>
              <strong>5,126</strong>
              <span>Audio</span>
            </div>
            <div>
              <strong>1,204</strong>
              <span>Books</span>
            </div>
            <div>
              <strong>3,918</strong>
              <span>Images</span>
            </div>
            <div>
              <strong>816</strong>
              <span>Others</span>
            </div>
          </dl>

          <section className="app-dream-storage-overview__capacity-section">
            <header className="app-dream-storage-overview__volumes-heading">
              <div className="app-dream-storage-overview__capacity-copy">
                <span className="app-dream-storage-overview__eyebrow">
                  Mounted capacity
                </span>
                <h4>3 volumes mounted</h4>
              </div>
              <p>3 roots registered · 1 offline</p>
            </header>

            <div className="app-dream-storage-overview__volumes">
              <div className="app-dream-storage-volume">
                <header className="app-dream-storage-volume__header">
                  <span className="app-dream-storage-volume__identity">
                    <span className="app-dream-storage-volume__icon">
                      <HardDrive aria-hidden="true" />
                    </span>
                    <strong>Macintosh HD</strong>
                  </span>
                  <strong className="app-dream-storage-volume__capacity">
                    420 GB of 1.00 TB
                  </strong>
                </header>
                <StorageCapacityBar
                  availableBytes={420}
                  label="Macintosh HD"
                  libraryBytes={286}
                  otherBytes={294}
                  totalBytes={1000}
                  valueText="580 GB used"
                />
                <dl className="app-dream-storage-volume__facts">
                  <div>
                    <span>Library size</span>
                    <strong>286 GB</strong>
                  </div>
                  <div>
                    <span>Files</span>
                    <strong>14,036</strong>
                  </div>
                  <div>
                    <span>Video</span>
                    <strong>6,942</strong>
                  </div>
                  <div>
                    <span>Audio</span>
                    <strong>4,821</strong>
                  </div>
                </dl>
              </div>
              <div className="app-dream-storage-volume">
                <header className="app-dream-storage-volume__header">
                  <span className="app-dream-storage-volume__identity">
                    <span className="app-dream-storage-volume__icon">
                      <HardDrive aria-hidden="true" />
                    </span>
                    <strong>Creator SSD</strong>
                  </span>
                  <strong className="app-dream-storage-volume__capacity">
                    912 GB of 1.50 TB
                  </strong>
                </header>
                <StorageCapacityBar
                  availableBytes={912}
                  label="Creator SSD"
                  libraryBytes={200}
                  otherBytes={388}
                  totalBytes={1500}
                  valueText="588 GB used"
                />
                <dl className="app-dream-storage-volume__facts">
                  <div>
                    <span>Library size</span>
                    <strong>200 GB</strong>
                  </div>
                  <div>
                    <span>Files</span>
                    <strong>4,612</strong>
                  </div>
                  <div>
                    <span>Video</span>
                    <strong>900</strong>
                  </div>
                  <div>
                    <span>Audio</span>
                    <strong>305</strong>
                  </div>
                </dl>
              </div>
              <div className="app-dream-storage-volume">
                <header className="app-dream-storage-volume__header">
                  <span className="app-dream-storage-volume__identity">
                    <span className="app-dream-storage-volume__icon">
                      <HardDrive aria-hidden="true" />
                    </span>
                    <strong>Project Archive</strong>
                  </span>
                  <strong className="app-dream-storage-volume__capacity">
                    1.82 TB of 2.00 TB
                  </strong>
                </header>
                <StorageCapacityBar
                  availableBytes={1820}
                  label="Project Archive"
                  libraryBytes={0}
                  otherBytes={180}
                  totalBytes={2000}
                  valueText="180 GB used"
                />
                <p className="app-dream-storage-volume__empty">
                  <Folder aria-hidden="true" />
                  No storage root
                </p>
              </div>
            </div>
          </section>
        </article>

        <div className="app-dream-storage-root-heading">
          <div className="app-dream-storage-root-heading__copy">
            <h3>Storage roots</h3>
            <p>
              Downloads is the managed default; every additional folder is
              registered as a reference on either platform.
            </p>
          </div>
          <Button size="compact">
            <Plus aria-hidden="true" />
            Add root
          </Button>
        </div>

        <div className="app-dream-storage-root-grid">
          {STORAGE_ROOTS.map((root) => (
            <StorageRootCard key={root.id} root={root} />
          ))}
        </div>

        <div className="app-dream-storage-root-heading">
          <div className="app-dream-storage-root-heading__copy">
            <h3>Automation attached to every root</h3>
            <p>
              Reference import registration, scheduled health checks and
              directory relocation reuse the same root identity.
            </p>
          </div>
          <StatusBadge icon={<Import />} tone="accent">
            Root-aware imports
          </StatusBadge>
        </div>
      </div>
    </section>
  );
}
