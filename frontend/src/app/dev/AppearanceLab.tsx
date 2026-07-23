import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  AlertTriangle,
  Check,
  ChevronLeft,
  ChevronRight,
  Download,
  MoreHorizontal,
  Music2,
  Pause,
  Palette,
  Play,
  Radar,
  Search,
  UserRound,
  X,
} from "lucide-react";
import * as React from "react";

import { LibraryIpodPreview } from "@/app/library/LibraryIpodPreview";
import {
  LibraryDeletedCompanionView,
  type LibraryDeletedCompanionItem,
} from "@/app/library/LibraryDeletedCompanion";
import {
  LibraryPreviewCompanion,
  LibraryPreviewCompanionFooter,
} from "@/app/library/LibraryPreviewCompanion";
import { TaskFolderArtwork } from "@/app/library/TaskFolderArtwork";
import {
  createLibraryWorkspaceLabels,
  type LibraryTaskPreviewItem,
  type LibraryWorkspaceItem,
} from "@/app/library/types";
import { MusicWorkspaceTransportBar } from "@/app/main/WorkspaceTransportBar";
import type { WorkspaceTransportLabels } from "@/app/main/WorkspaceTransportBar";
import { ListenLyricsSurface } from "@/app/main/listen/lyrics";
import { ListenPlayerTransport } from "@/app/main/listen/playback-controls";
import { ListenPlayerFooter } from "@/app/main/listen/playback-ui";
import { ListenWorkspaceFullscreenBackdrop } from "@/app/main/listen/workspace-player-shared";
import {
  ListenWorkspaceOnlineQueueCompanion,
  ListenWorkspaceQueueModeSwitch,
} from "@/app/main/listen/workspace-companion";
import type {
  ListenLyricsData,
  ListenNowPlayingStatus,
  ListenOnlineItem,
} from "@/app/main/listen/types";
import {
  LibraryDeviceDetailsContent,
  LibraryPairedDeviceRow,
} from "@/app/settings/LibraryDeviceDetailsContent";
import { TabButton } from "@/app/settings/settings-helpers";
import {
  YouTubePrimaryWatchPage,
  type YouTubePrimaryWatchLabels,
} from "@/app/youtube/YouTubeWorkspacePage";
import { YouTubeSubscriptionIconButton } from "@/app/youtube/YouTubeSubscriptionIconButton";
import type {
  YouTubePlaybackDescriptor,
  YouTubePlayerStatus,
  YouTubeVideoDetails,
  YouTubeWorkspaceVideo,
} from "@/app/youtube/types";
import { WindowControls } from "@/components/layout/WindowControls";
import { EqualizerControlCards } from "@/features/settings/equalizer/EqualizerControlCards";
import { getXiaText } from "@/features/xiadown/shared";
import {
  AppShell,
  CompanionPanel,
  PrimaryPane,
  WorkspaceSidebar,
  WorkspaceStage,
  type CompanionPresentation,
} from "@/app/workspace";
import type { Settings } from "@/shared/contracts/settings";
import type { EqualizerBand } from "@/shared/contracts/equalizer";
import type { PairedLibraryDevice } from "@/shared/contracts/library-access";
import { t } from "@/shared/i18n";
import {
  XIA_THEME_PACKS,
  type XiaSurfaceStyle,
  type XiaThemePackId,
} from "@/shared/styles/xiadown-theme";
import { applyXiaTheme } from "@/shared/styles/theme-runtime";
import { Button } from "@/shared/ui/button";
import {
  Dialog,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogListCard,
  DialogListCardContent,
  DialogRow,
  DialogTitle,
} from "@/shared/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/shared/ui/dropdown-menu";
import {
  GLASS_ELEVATIONS,
  GLASS_SHAPES,
  GLASS_TINTS,
  GlassGroup,
  GlassSurface,
} from "@/shared/ui/glass-surface";
import { Select } from "@/shared/ui/select";
import {
  SettingsCompactListCard,
  SettingsCompactRow,
  SettingsCompactSeparator,
} from "@/shared/ui/settings-layout";
import { StatusBadge } from "@/shared/ui/status-badge";
import {
  XIA_GLASS_MATERIALS,
  XIA_SURFACE_ROLES,
  type XiaSurfaceRole,
} from "@/shared/ui/surface-contract";
import { WorkspaceSearchControl } from "@/shared/ui/workspace-search-control";

import {
  applyAppearanceLabNativeVideoPreview,
  applyAppearanceLabPlatform,
  applyRootDatasetValue,
  type AppearanceLabNativeVideoPreview,
} from "./appearance-root-state";
import { DreamStyleCatalog } from "./DreamStyleCatalog";
import { PrimitiveFixtureGallery } from "./PrimitiveFixtureGallery";
import "@/app/library/library.css";
import "@/app/main/listen/listen.css";
import "@/app/youtube/youtube-uploader-page.css";
import "./appearance-lab.css";

type Appearance = "light" | "dark";
type LabPlatform = "macos" | "windows";
type LabAccentSource = "theme" | "system" | "custom";

const APPEARANCE_LAB_QUERY_CLIENT = new QueryClient({
  defaultOptions: {
    mutations: { retry: false },
    queries: { retry: false },
  },
});

interface PrismGlassFixtureProps {
  appearance: Appearance;
  companionOpen: boolean;
  nativeVideoPreview: AppearanceLabNativeVideoPreview;
  platform: LabPlatform;
  presentation: CompanionPresentation;
  reduceTransparency: boolean;
  surfaceStyle: XiaSurfaceStyle;
  onCompanionOpenChange: (open: boolean) => void;
  onNativeVideoPreviewChange: (
    preview: AppearanceLabNativeVideoPreview,
  ) => void;
  onPlatformChange: (platform: LabPlatform) => void;
  onPresentationChange: (presentation: CompanionPresentation) => void;
  onReduceTransparencyChange: (reduce: boolean) => void;
  onSurfaceStyleChange: (surfaceStyle: XiaSurfaceStyle) => void;
}

const PRISM_QUEUE = [
  ["Violet Hours", "Night Archive", "3:42"],
  ["Soft Current", "Mizu", "4:08"],
  ["Signals at Dawn", "Northbound", "3:17"],
  ["Glasshouse", "June Static", "5:01"],
  ["Quiet Geometry", "Mono Lake", "3:36"],
  ["Silver Thread", "Horizon Club", "4:22"],
  ["Blue Transit", "Small Hours", "3:54"],
  ["Afterimage", "Lumen Field", "4:41"],
  ["Paper Satellites", "June Static", "3:28"],
  ["Slow Bloom", "Mizu", "4:16"],
  ["Night Ferry", "Northbound", "5:08"],
  ["First Light", "Night Archive", "3:49"],
] as const;

const APPEARANCE_PLAYBACK_TEXT = getXiaText("en");

const APPEARANCE_TRANSPORT_LABELS: WorkspaceTransportLabels = {
  idleStatus: "Nothing playing",
  idleSubtitle: "Choose something to listen to",
  shuffle: "Shuffle",
  previous: "Previous",
  play: "Play",
  pause: "Pause",
  next: "Next",
  repeatOne: "Repeat one",
  live: "Live",
  lyrics: "Lyrics",
  upNext: "Up next",
  volume: "Volume",
  fullscreen: "Fullscreen",
  more: "More",
  favorite: "Favorite",
  download: "Download",
  openURL: "Open URL",
};

const APPEARANCE_NOW_PLAYING_STATUS: ListenNowPlayingStatus = {
  state: "playing",
  mediaId: "appearance-playback-track",
  title: "Midnight Download",
  subtitle: "XiaDown Radio",
  artists: [{ name: "XiaDown Radio", browseId: "appearance-artist" }],
  artworkURL: "/dreamcreator.png",
  artworkCandidates: ["/dreamcreator.png"],
  playbackSource: "youtube_music",
  playbackSourceLabel: "YouTube Music",
  mode: "muse",
  canControl: true,
  canPrevious: true,
  canNext: true,
  favoriteActive: true,
  canFavorite: true,
  volume: 0.72,
  muted: false,
  progress: { currentTime: 104, duration: 247, bufferedTime: 168 },
};

const APPEARANCE_PLAYBACK_QUEUE: ListenOnlineItem[] = [
  {
    id: "appearance-queue-current",
    group: "playlist",
    source: "appearance",
    videoId: "appearance-current",
    title: "Midnight Download",
    channel: "XiaDown Radio",
    artists: [{ name: "XiaDown Radio" }],
    description: "",
    durationLabel: "4:07",
    thumbnailUrl: "/dreamcreator.png",
  },
  {
    id: "appearance-queue-next",
    group: "playlist",
    source: "appearance",
    videoId: "appearance-next",
    title: "Signals at Dawn",
    channel: "Northbound",
    artists: [{ name: "Northbound" }],
    description: "",
    durationLabel: "3:17",
    thumbnailUrl: "/hush.png",
  },
  {
    id: "appearance-queue-later",
    group: "playlist",
    source: "appearance",
    videoId: "appearance-later",
    title: "Soft Current",
    channel: "Mizu",
    artists: [{ name: "Mizu" }],
    description: "",
    durationLabel: "4:08",
    thumbnailUrl: "/appicon.png",
  },
];

const APPEARANCE_FOCUS_LYRICS: ListenLyricsData = {
  videoId: "appearance-focus-lyrics",
  kind: "synced",
  source: "Appearance Lab",
  timingQuality: "word",
  text: "The city settles\nHold the light while the quiet morning finds us\nWe begin again",
  lines: [
    {
      startMs: 0,
      durationMs: 1200,
      text: "The city settles",
    },
    {
      startMs: 1200,
      durationMs: 4400,
      text: "Hold the light while the quiet morning finds us",
      translationText: "Keep this moment close.",
      words: [
        {
          startMs: 1200,
          endMs: 5600,
          text: "Hold the light while the quiet morning finds us",
        },
      ],
    },
    {
      startMs: 5600,
      durationMs: 1800,
      text: "We begin again",
    },
  ],
};

const APPEARANCE_TASK_FOLDER_ITEMS = [
  {
    id: "appearance-folder-placeholder",
    kind: "audio",
    label: "Default audio page",
    previewURL: "xiadown-library-default:audio",
  },
  {
    id: "appearance-folder-real",
    kind: "thumbnail",
    label: "Downloaded cover",
    previewURL: "/dreamcreator.png",
  },
] satisfies LibraryTaskPreviewItem[];

const APPEARANCE_LIBRARY_LABELS = createLibraryWorkspaceLabels(
  (key) => t(key, "en"),
  "en",
);

const APPEARANCE_LIBRARY_PREVIEW_ITEM = {
  id: "appearance-library-preview",
  source: "file",
  libraryId: "appearance-library",
  libraryName: "Appearance Library",
  title: "Companion footer width contract",
  subtitle: "Four equal preview tabs",
  category: "image",
  status: "available",
  format: "PNG",
  createdAt: "2026-07-20T00:00:00.000Z",
  updatedAt: "2026-07-20T00:00:00.000Z",
  path: "/Appearance/companion-footer.png",
  coverURL: "/dreamcreator.png",
  rootId: "appearance-library-preview",
  searchText: "companion footer width contract",
} satisfies LibraryWorkspaceItem;

const APPEARANCE_TASK_HISTORY_ITEM = {
  id: "task:appearance-history",
  source: "task",
  libraryId: "appearance-library",
  libraryName: "Appearance Library",
  title: "Stamp archive task",
  subtitle: "download",
  category: "task",
  status: "succeeded",
  format: "DOWNLOAD",
  sizeBytes: 6_144,
  createdAt: "2026-07-18T08:00:00Z",
  updatedAt: "2026-07-18T08:02:00Z",
  path: "https://example.com/stamps",
  coverURL: "/dreamcreator.png",
  rootId: "appearance-history",
  searchText: "stamp archive task",
  operation: {
    operationId: "appearance-history",
    libraryId: "appearance-library",
    name: "Stamp archive task",
    kind: "download",
    status: "succeeded",
    correlation: {},
    outputFiles: [{ fileId: "appearance-current", kind: "image", format: "webp" }],
    detachedOutputFileIds: ["appearance-detached"],
    metrics: { fileCount: 1, totalSizeBytes: 4_096 },
    createdAt: "2026-07-18T08:00:00Z",
    finishedAt: "2026-07-18T08:02:00Z",
  },
  library: {
    version: "current",
    id: "appearance-library",
    name: "Appearance Library",
    createdAt: "2026-07-01T00:00:00Z",
    updatedAt: "2026-07-20T09:30:00Z",
    createdBy: { source: "appearance-lab" },
    files: [{
      id: "appearance-current",
      libraryId: "appearance-library",
      kind: "image",
      name: "current-stamp.webp",
      storage: { mode: "local_path", localPath: "/Appearance/current-stamp.webp" },
      origin: { kind: "download", operationId: "appearance-history" },
      lineage: {},
      metadata: {},
      media: { format: "webp", sizeBytes: 4_096 },
      state: { status: "active", deleted: false, archived: false },
      createdAt: "2026-07-18T08:02:00Z",
      updatedAt: "2026-07-18T08:02:00Z",
    }, {
      id: "appearance-detached",
      libraryId: "appearance-library",
      kind: "image",
      name: "removed-stamp.webp",
      storage: { mode: "local_path", localPath: "/Appearance/removed-stamp.webp" },
      origin: { kind: "download", operationId: "appearance-history" },
      lineage: {},
      metadata: {},
      media: { format: "webp", sizeBytes: 2_048 },
      state: { status: "active", deleted: false, archived: false },
      createdAt: "2026-07-18T08:02:00Z",
      updatedAt: "2026-07-20T09:30:00Z",
    }],
    records: {
      history: [{
        recordId: "appearance-history-snapshot",
        libraryId: "appearance-library",
        category: "operation",
        action: "download",
        displayName: "Stamp archive task",
        status: "succeeded",
        source: { kind: "download" },
        refs: { operationId: "appearance-history" },
        files: [
          { fileId: "appearance-current", kind: "image", format: "webp", sizeBytes: 4_096 },
          { fileId: "appearance-detached", kind: "image", format: "webp", sizeBytes: 2_048 },
        ],
        metrics: { fileCount: 2, totalSizeBytes: 6_144 },
        occurredAt: "2026-07-18T08:02:00Z",
        createdAt: "2026-07-18T08:00:00Z",
      }, {
        recordId: "appearance-resume-event",
        libraryId: "appearance-library",
        category: "operation_event",
        action: "operation_resumed",
        displayName: "Stamp archive task",
        status: "canceled",
        source: { kind: "user_action", actor: "desktop-library" },
        refs: { operationId: "appearance-history" },
        files: [],
        metrics: { fileCount: 0 },
        occurredAt: "2026-07-19T09:00:00Z",
        createdAt: "2026-07-19T09:00:00Z",
      }],
      workspaceStates: [],
      fileEvents: [{
        id: "appearance-detach-event",
        libraryId: "appearance-library",
        fileId: "appearance-detached",
        operationId: "appearance-history",
        eventType: "operation_output_detached",
        detail: {
          cause: { category: "task_output", operationId: "appearance-history", actor: "desktop-library" },
          before: { fileId: "appearance-detached", kind: "image", name: "removed-stamp.webp" },
          after: { fileId: "appearance-detached", kind: "image", name: "removed-stamp.webp" },
          changes: [{ field: "taskAssociation", before: "attached", after: "detached" }],
          deleteFile: false,
        },
        occurredAt: "2026-07-20T09:30:00Z",
        createdAt: "2026-07-20T09:30:00Z",
      }],
    },
  },
} satisfies LibraryWorkspaceItem;

const APPEARANCE_DELETED_LIBRARY_ITEMS = [{
  id: "appearance-deleted-task",
  kind: "task",
  source: "operation_history",
  libraryId: "appearance-library",
  title: "A very long deleted download task title that stays inside the fixed Companion width",
  category: "download",
  status: "deleted",
  deletedAt: "2026-07-20T10:20:00Z",
  canRestore: false,
  detail: {
    taskHistory: {
      recordId: "appearance-deleted-task-event",
      libraryId: "appearance-library",
      category: "operation_event",
      action: "operation_deleted",
      displayName: "A very long deleted download task title that stays inside the fixed Companion width",
      status: "deleted",
      source: { kind: "user_action", actor: "desktop-library" },
      refs: { operationId: "appearance-deleted-task" },
      metrics: { fileCount: 2 },
      occurredAt: "2026-07-20T10:20:00Z",
      createdAt: "2026-07-20T10:20:00Z",
    },
  },
}, {
  id: "appearance-deleted-file",
  kind: "file",
  source: "legacy_file",
  libraryId: "appearance-library",
  title: "Deleted stamp scan with a long descriptive filename.webp",
  category: "image",
  status: "deleted",
  deletedAt: "2026-07-20T09:45:00Z",
  canRestore: true,
  detail: {
    file: {
      id: "appearance-deleted-file",
      libraryId: "appearance-library",
      kind: "image",
      name: "deleted-stamp-scan-with-a-long-descriptive-filename.webp",
      storage: {
        mode: "local_path",
        localPath: "/Appearance/A deliberately long nested path/Deleted stamp scan with a long descriptive filename.webp",
      },
      origin: { kind: "download" },
      lineage: {},
      metadata: { title: "Deleted stamp scan" },
      media: { format: "webp", sizeBytes: 8_192 },
      state: { status: "deleted", deleted: true, archived: false },
      createdAt: "2026-07-18T08:00:00Z",
      updatedAt: "2026-07-20T09:45:00Z",
    },
  },
}, {
  id: "appearance-trashed-catalog-item",
  kind: "catalog_item",
  source: "catalog_trash",
  libraryId: "appearance-catalog",
  title: "Trashed catalog artwork",
  category: "image",
  status: "trashed",
  deletedAt: "2026-07-19T18:30:00Z",
  canRestore: true,
  revision: 4,
  detail: {
    catalogItem: {
      id: "appearance-trashed-catalog-item",
      catalogId: "appearance-catalog",
      category: "image",
      status: "trashed",
      title: "Trashed catalog artwork",
      sortTitle: "Trashed catalog artwork",
      revision: 4,
      trashedAt: "2026-07-19T18:30:00Z",
      createdAt: "2026-07-01T00:00:00Z",
      updatedAt: "2026-07-19T18:30:00Z",
    },
  },
}] satisfies LibraryDeletedCompanionItem[];

const APPEARANCE_ULTRAWIDE_IMAGE_URL =
  "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='2400' height='500' viewBox='0 0 2400 500'%3E%3Cdefs%3E%3ClinearGradient id='g' x1='0' x2='1'%3E%3Cstop stop-color='%23182f4c'/%3E%3Cstop offset='.5' stop-color='%2347a6aa'/%3E%3Cstop offset='1' stop-color='%23e4b36b'/%3E%3C/linearGradient%3E%3C/defs%3E%3Crect width='2400' height='500' fill='url(%23g)'/%3E%3Ccircle cx='300' cy='250' r='150' fill='%23ffffff' fill-opacity='.68'/%3E%3Ccircle cx='2100' cy='250' r='150' fill='%23ffffff' fill-opacity='.68'/%3E%3C/svg%3E";

const APPEARANCE_LANDSCAPE_LONG_TITLE =
  "Lofi Music Project-this is what pure nostalgia sounds like ｜ emotional support lo-fi-medium-IwhR61zEFPo";
const APPEARANCE_LANDSCAPE_IMAGE_URL =
  "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='1280' height='720' viewBox='0 0 1280 720'%3E%3Cdefs%3E%3ClinearGradient id='g' x1='0' y1='1' x2='1' y2='0'%3E%3Cstop stop-color='%2314273c'/%3E%3Cstop offset='.45' stop-color='%235c8d9c'/%3E%3Cstop offset='1' stop-color='%23f0b4af'/%3E%3C/linearGradient%3E%3C/defs%3E%3Crect width='1280' height='720' fill='url(%23g)'/%3E%3Ccircle cx='760' cy='280' r='180' fill='%23f7d7d0' fill-opacity='.72'/%3E%3Crect x='0' y='540' width='1280' height='180' fill='%230d1b2a' fill-opacity='.42'/%3E%3C/svg%3E";

const APPEARANCE_EQUALIZER_BANDS = [
  { id: "32", frequencyHz: 32, q: 1, type: "lowShelf", display: "32", displayHz: "32 Hz" },
  { id: "64", frequencyHz: 64, q: 1, type: "peaking", display: "64", displayHz: "64 Hz" },
  { id: "125", frequencyHz: 125, q: 1, type: "peaking", display: "125", displayHz: "125 Hz" },
  { id: "250", frequencyHz: 250, q: 1, type: "peaking", display: "250", displayHz: "250 Hz" },
  { id: "500", frequencyHz: 500, q: 1, type: "peaking", display: "500", displayHz: "500 Hz" },
  { id: "1k", frequencyHz: 1_000, q: 1, type: "peaking", display: "1K", displayHz: "1 kHz" },
  { id: "4k", frequencyHz: 4_000, q: 1, type: "peaking", display: "4K", displayHz: "4 kHz" },
  { id: "16k", frequencyHz: 16_000, q: 1, type: "highShelf", display: "16K", displayHz: "16 kHz" },
] satisfies EqualizerBand[];

const APPEARANCE_EQUALIZER_GAINS = [2, 3.5, 1.5, -1, -2.5, 0.5, 2.5, 1];

const APPEARANCE_PAIRED_DEVICE = {
  grantId: "appearance-device",
  deviceId: "appearance-iphone",
  deviceName: "Arnold’s iPhone",
  scopes: ["library.read", "rss.read", "rss.state", "tasks.read"],
  status: "active",
  lastSeenAt: "2026-07-19T11:48:00.000Z",
  revision: 4,
  createdAt: "2026-07-12T09:00:00.000Z",
  updatedAt: "2026-07-19T11:48:00.000Z",
} satisfies PairedLibraryDevice;

const APPEARANCE_YOUTUBE_VIDEO: YouTubeWorkspaceVideo = {
  itemKind: "video",
  videoId: "appearance-watch",
  title: "Designing a calmer download workspace",
  channel: "XiaDown Studio",
  channelId: "appearance-channel",
  thumbnailUrl: "/dreamcreator.png",
  viewCount: 82_400,
  publishedLabel: "2 days ago",
  webUrl: "https://www.youtube.com/watch?v=appearance-watch",
};

const APPEARANCE_YOUTUBE_PLAYBACK: YouTubePlaybackDescriptor = {
  source: "youtube",
  mediaKind: "video",
  sessionId: "appearance-youtube-session",
  videoId: APPEARANCE_YOUTUBE_VIDEO.videoId,
  title: APPEARANCE_YOUTUBE_VIDEO.title,
  artist: APPEARANCE_YOUTUBE_VIDEO.channel,
  thumbnailUrl: APPEARANCE_YOUTUBE_VIDEO.thumbnailUrl,
  webUrl: APPEARANCE_YOUTUBE_VIDEO.webUrl,
};

const APPEARANCE_YOUTUBE_STATUS: YouTubePlayerStatus = {
  state: "paused",
  title: APPEARANCE_YOUTUBE_VIDEO.title,
  artist: APPEARANCE_YOUTUBE_VIDEO.channel,
  thumbnailUrl: APPEARANCE_YOUTUBE_VIDEO.thumbnailUrl,
};

const APPEARANCE_YOUTUBE_DETAILS: YouTubeVideoDetails = {
  videoId: APPEARANCE_YOUTUBE_VIDEO.videoId,
  title: APPEARANCE_YOUTUBE_VIDEO.title,
  channel: APPEARANCE_YOUTUBE_VIDEO.channel,
  channelId: APPEARANCE_YOUTUBE_VIDEO.channelId,
  channelAvatarUrl: "/appicon.png",
  thumbnailUrl: APPEARANCE_YOUTUBE_VIDEO.thumbnailUrl,
  viewCount: APPEARANCE_YOUTUBE_VIDEO.viewCount,
  likeCount: 7_320,
  publishedLabel: APPEARANCE_YOUTUBE_VIDEO.publishedLabel,
  description: "A production Watch surface shared by YouTube and RSS video playback.",
  webUrl: APPEARANCE_YOUTUBE_VIDEO.webUrl,
};

const APPEARANCE_YOUTUBE_LABELS = {
  back: "Back",
  uploader: "Uploader",
  more: "More",
  like: "Like",
  dislike: "Dislike",
  subscribe: "Subscribe",
  unsubscribe: "Unsubscribe",
  download: "Download",
  openURL: "Open on YouTube",
  unavailable: "Unavailable",
  videoInfo: "Video info",
  description: "Description",
  descriptionUnavailable: "No description",
  published: "Published",
  views: "Views",
  likes: "Likes",
  close: "Close",
} satisfies YouTubePrimaryWatchLabels;

const APPEARANCE_FULLSCREEN_MODES = [
  { id: "idle", label: "No selection", mediaMode: "cover", context: "media" },
  { id: "lyrics", label: "Lyrics", mediaMode: "lyrics", context: "lyrics" },
  { id: "queue", label: "Queue", mediaMode: "cover", context: "queue" },
  { id: "video", label: "Video", mediaMode: "video", context: "video" },
] as const;

const SURFACE_ROLE_SAMPLES = {
  canvas: {
    label: "Canvas",
    contract: "--app-surface-canvas",
    description: "The Settings window canvas, shared by every persistent Contrast pane.",
  },
  chrome: {
    label: "Chrome",
    contract: "XIA_SURFACE_ROLE_PRESETS.chrome",
    description: "Persistent navigation chrome: sampled in Glass, canvas-aligned in Contrast.",
  },
  content: {
    label: "Content",
    contract: "XIA_SURFACE_ROLE_PRESETS.content",
    description: "The readable primary plane: one dense veil in Glass, canvas in Contrast.",
  },
  status: {
    label: "Status",
    contract: "--app-surface-status-*",
    description: "Running, player and sniff state share one reusable activity recipe.",
  },
  overlay: {
    label: "Overlay",
    contract: "--app-surface-overlay-*",
    description: "Dropdown, sheet and dialog share one text-safe transient recipe.",
  },
  card: {
    label: "Card",
    contract: "--app-surface-card-*",
    description: "Local grouping inside content without creating another backdrop sampler.",
  },
  inset: {
    label: "Inset",
    contract: "--app-surface-inset-fill",
    description: "Recessed wells and tracks that stay subordinate to their host.",
  },
  control: {
    label: "Control",
    contract: "--app-surface-control-fill",
    description: "Interactive affordances layered above the owning surface.",
  },
} as const satisfies Record<
  XiaSurfaceRole,
  { label: string; contract: string; description: string }
>;

function SurfaceAxis({
  children,
  label,
}: React.PropsWithChildren<{ label: string }>) {
  return (
    <div className="appearance-lab__surface-axis">
      <strong>{label}</strong>
      <div>{children}</div>
    </div>
  );
}

export function SurfaceRoleMatrix({
  surfaceStyle,
}: {
  surfaceStyle: XiaSurfaceStyle;
}) {
  return (
    <section
      aria-labelledby="appearance-lab-surface-contract-title"
      className="appearance-lab__section appearance-lab__section--contract"
      data-surface-style={surfaceStyle}
    >
      <div className="appearance-lab__section-heading appearance-lab__contract-heading">
        <span>02</span>
        <div>
          <h2 id="appearance-lab-surface-contract-title">Surface contract</h2>
          <p>Roles own the recipe; features only declare intent.</p>
        </div>
        <span className="appearance-lab__contract-mode" data-surface-style={surfaceStyle}>
          {surfaceStyle}
        </span>
      </div>

      <div className="appearance-lab__role-matrix">
        {XIA_SURFACE_ROLES.map((role) => {
          const sample = SURFACE_ROLE_SAMPLES[role];
          return (
            <article className="appearance-lab__role-card" key={role}>
              <GlassSurface
                className="appearance-lab__role-swatch"
                elevation="embedded"
                shape="control"
                surfaceRole={role}
              >
                <span className="appearance-lab__label">{sample.label}</span>
                <Button shape="control" size="compact" variant="ghost">Control</Button>
              </GlassSurface>
              <div className="appearance-lab__role-copy">
                <div>
                  <strong>{sample.label}</strong>
                  <code>{role}</code>
                </div>
                <p>{sample.description}</p>
                <code>{sample.contract}</code>
              </div>
            </article>
          );
        })}
      </div>

      <div className="appearance-lab__surface-axes">
        <SurfaceAxis label="Material">
          {XIA_GLASS_MATERIALS.map((material) => (
            <GlassSurface
              className="appearance-lab__surface-axis-sample"
              data-appearance-material-specimen={material}
              elevation="embedded"
              key={material}
              material={material}
              shape="control"
            >
              {material}
            </GlassSurface>
          ))}
        </SurfaceAxis>
        <SurfaceAxis label="Elevation">
          {GLASS_ELEVATIONS.map((elevation) => (
            <GlassSurface
              className="appearance-lab__surface-axis-sample"
              elevation={elevation}
              key={elevation}
              shape="card"
              surfaceRole="card"
            >
              {elevation}
            </GlassSurface>
          ))}
        </SurfaceAxis>
        <SurfaceAxis label="Shape">
          {GLASS_SHAPES.map((shape) => (
            <GlassSurface
              className="appearance-lab__surface-axis-sample"
              elevation="embedded"
              key={shape}
              shape={shape}
              surfaceRole="card"
            >
              {shape}
            </GlassSurface>
          ))}
        </SurfaceAxis>
        <SurfaceAxis label="Tint">
          {GLASS_TINTS.map((tint) => (
            <GlassSurface
              className="appearance-lab__surface-axis-sample"
              elevation="embedded"
              key={tint}
              shape="control"
              surfaceRole="card"
              tint={tint}
            >
              {tint}
            </GlassSurface>
          ))}
        </SurfaceAxis>
      </div>
    </section>
  );
}

export function RegressionContractSpecimen() {
  return (
    <section
      className="appearance-lab__section"
      data-appearance-fixture="regression-contracts"
    >
      <div className="appearance-lab__section-heading">
        <span>06</span>
        <div>
          <h2>Regression contracts</h2>
          <p>Production Settings, Library, Dialog and fullscreen anatomy under one review surface.</p>
        </div>
      </div>

      <div className="appearance-lab__playback-grid">
        <article className="appearance-lab__playback-card appearance-lab__playback-card--wide">
          <header>
            <div>
              <span className="appearance-lab__label">Settings rhythm</span>
              <strong>Stable tabs and comfortable product actions</strong>
            </div>
            <code>tabs · 2.5rem actions · 2.75rem choices</code>
          </header>
          <nav
            aria-label="Settings tab regression specimen"
            className="app-dream-tabs-bar appearance-lab__settings-tabs"
          >
            <TabButton
              active
              icon={<UserRound className="app-settings-tab-symbol" />}
              id="general"
              label="General"
              onClick={() => undefined}
            />
            <TabButton
              active={false}
              icon={<Palette className="app-settings-tab-symbol" />}
              id="appearance"
              label="Appearance"
              onClick={() => undefined}
            />
            <TabButton
              active={false}
              icon={<Download className="app-settings-tab-symbol" />}
              id="download"
              label="Download"
              onClick={() => undefined}
            />
          </nav>
          <div
            aria-label="Comfortable action regression specimen"
            className="appearance-lab__comfortable-actions"
          >
            <Button
              className="app-running-new-download-button"
              data-appearance-action="running-new-egg"
            >
              New egg
            </Button>
            <Button
              className="app-sniff-desk-start-button app-running-new-download-button"
              data-appearance-action="sniff-start"
            >
              Start sniffing
            </Button>
            <Button
              className="app-settings-theme-pack-button"
              data-appearance-action="theme-choice"
              tone="accent"
              variant="outline"
            >
              <span
                aria-hidden="true"
                className="app-settings-theme-preview"
                data-theme-pack-preview="graphite"
              >
                <span data-preview-role="shell" />
                <span data-preview-role="sidebar" />
                <span data-preview-role="accent" />
              </span>
              <span className="app-settings-theme-pack-label">Graphite</span>
            </Button>
            <Button
              className="app-sessions-account-action"
              data-appearance-action="session-verify"
            >
              <Check aria-hidden="true" />
              Verify login
            </Button>
            <Button
              className="app-sessions-account-action"
              data-appearance-action="session-sign-out"
              variant="destructive"
            >
              <X aria-hidden="true" />
              Sign out
            </Button>
          </div>
        </article>

        <article className="appearance-lab__playback-card appearance-lab__playback-card--wide">
          <header>
            <div>
              <span className="appearance-lab__label">Equalizer rhythm</span>
              <strong>Production preset and preamp cards</strong>
            </div>
            <code>block inset · full inset</code>
          </header>
          <div className="appearance-lab__equalizer-contract">
            <EqualizerControlCards
              bands={APPEARANCE_EQUALIZER_BANDS}
              bandGainsDb={APPEARANCE_EQUALIZER_GAINS}
              disabled={false}
              enabled
              labels={{ bands: "Bands", preamp: "Preamp", reset: "Reset" }}
              preampDb={1.5}
              onBandGainChange={() => undefined}
              onPreampChange={() => undefined}
              onReset={() => undefined}
            />
          </div>
        </article>

        <article className="appearance-lab__playback-card appearance-lab__playback-card--wide">
          <header>
            <div>
              <span className="appearance-lab__label">Paired devices</span>
              <strong>One list row and its shared detail presentation</strong>
            </div>
            <code>device list · detail dialog</code>
          </header>
          <div className="appearance-lab__device-contract">
            <SettingsCompactListCard data-library-paired-devices>
              <SettingsCompactRow label="Paired devices">
                <StatusBadge marker tone="success">1 active</StatusBadge>
              </SettingsCompactRow>
              <SettingsCompactSeparator />
              <LibraryPairedDeviceRow
                device={APPEARANCE_PAIRED_DEVICE}
                language="en"
                onView={() => undefined}
              />
            </SettingsCompactListCard>

            <Dialog open>
              <GlassSurface
                className="app-dialog-content app-library-device-dialog appearance-lab__device-dialog"
                data-appearance-fixture="library-device-details"
                elevation="modal"
                shape="panel"
                surfaceRole="overlay"
              >
                <DialogHeader>
                  <DialogTitle>{APPEARANCE_PAIRED_DEVICE.deviceName}</DialogTitle>
                  <DialogDescription>
                    Permissions and recent use remain on one undivided surface.
                  </DialogDescription>
                </DialogHeader>
                <LibraryDeviceDetailsContent
                  device={APPEARANCE_PAIRED_DEVICE}
                  language="en"
                  onToggleScope={() => undefined}
                  onCancelRevoke={() => undefined}
                  onRevoke={() => undefined}
                />
                <DialogFooter>
                  <Button variant="outline">Close</Button>
                </DialogFooter>
              </GlassSurface>
            </Dialog>
          </div>
        </article>

        <article className="appearance-lab__playback-card appearance-lab__playback-card--wide">
          <header>
            <div>
              <span className="appearance-lab__label">Library media</span>
              <strong>Task folder and production iPod silhouette</strong>
            </div>
            <code>production artwork · nested-radius regression</code>
          </header>
          <div className="appearance-lab__library-regressions">
            <div
              className="appearance-lab__library-companion-frame"
              data-appearance-library-companion="task"
              data-companion-width-contract="390px"
            >
              <div className="app-library-preview">
                <div className="app-library-preview__body">
                  <article
                    className="app-library-preview__overview app-library-preview__task"
                    data-preview-kind="task"
                  >
                    <div className="app-library-preview__hero">
                      <TaskFolderArtwork
                        className="app-library-preview__task-folder-artwork"
                        items={APPEARANCE_TASK_FOLDER_ITEMS}
                        presentation="companion-open"
                        totalCount={2}
                      />
                    </div>
                  </article>
                </div>
              </div>
            </div>
            <div
              className="appearance-lab__library-companion-frame"
              data-appearance-library-companion="task-versions"
              data-companion-width-contract="390px"
            >
              <QueryClientProvider client={APPEARANCE_LAB_QUERY_CLIENT}>
                <LibraryPreviewCompanion
                  initialTab="versions"
                  item={APPEARANCE_TASK_HISTORY_ITEM}
                  labels={APPEARANCE_LIBRARY_LABELS}
                />
              </QueryClientProvider>
            </div>
            <div
              className="appearance-lab__library-companion-frame"
              data-appearance-library-companion="task-activity"
              data-companion-width-contract="390px"
            >
              <QueryClientProvider client={APPEARANCE_LAB_QUERY_CLIENT}>
                <LibraryPreviewCompanion
                  initialTab="activity"
                  item={APPEARANCE_TASK_HISTORY_ITEM}
                  labels={APPEARANCE_LIBRARY_LABELS}
                />
              </QueryClientProvider>
            </div>
            <div
              className="appearance-lab__library-companion-frame"
              data-appearance-library-companion="deleted"
              data-companion-width-contract="390px"
            >
              <LibraryDeletedCompanionView
                items={APPEARANCE_DELETED_LIBRARY_ITEMS}
                total={APPEARANCE_DELETED_LIBRARY_ITEMS.length}
              />
            </div>
            <div
              className="appearance-lab__library-companion-frame"
              data-appearance-library-companion="video"
              data-companion-width-contract="390px"
            >
              <div className="app-library-preview">
                <div className="app-library-preview__body">
                  <article className="app-library-preview__overview">
                    <div className="app-library-preview__hero">
                      <div
                        className="app-library-preview__media"
                        data-appearance-library-ipod="video"
                        data-preview-kind="video"
                      >
                        <LibraryIpodPreview
                          category="video"
                          coverURL="/dreamcreator.png"
                          labels={APPEARANCE_LIBRARY_LABELS}
                          sourceURL=""
                          title="Companion video iPod"
                        />
                      </div>
                    </div>
                  </article>
                </div>
              </div>
            </div>
            <div
              className="appearance-lab__library-companion-frame"
              data-appearance-library-companion="image-ultrawide"
              data-companion-width-contract="390px"
            >
              <div className="app-library-preview">
                <div className="app-library-preview__body">
                  <article className="app-library-preview__overview">
                    <div className="app-library-preview__hero">
                      <div
                        className="app-library-preview__media"
                        data-appearance-library-ipod="image-ultrawide"
                        data-preview-kind="image"
                        data-source-dimensions="2400x500"
                      >
                        <LibraryIpodPreview
                          category="image"
                          coverURL={APPEARANCE_ULTRAWIDE_IMAGE_URL}
                          labels={APPEARANCE_LIBRARY_LABELS}
                          sourceURL={APPEARANCE_ULTRAWIDE_IMAGE_URL}
                          title="Ultrawide image · 2400 × 500"
                        />
                      </div>
                    </div>
                  </article>
                </div>
              </div>
            </div>
            <div
              className="appearance-lab__library-companion-frame"
              data-appearance-library-companion="image-landscape-long-title"
              data-companion-width-contract="390px"
            >
              <div className="app-library-preview">
                <div className="app-library-preview__body">
                  <article className="app-library-preview__overview">
                    <div className="app-library-preview__hero">
                      <div
                        className="app-library-preview__media"
                        data-appearance-library-ipod="image-landscape-long-title"
                        data-dialog-regression="16:9-long-title"
                        data-preview-kind="image"
                        data-source-dimensions="1280x720"
                      >
                        <LibraryIpodPreview
                          category="image"
                          coverURL={APPEARANCE_LANDSCAPE_IMAGE_URL}
                          labels={APPEARANCE_LIBRARY_LABELS}
                          sourceURL={APPEARANCE_LANDSCAPE_IMAGE_URL}
                          title={APPEARANCE_LANDSCAPE_LONG_TITLE}
                        />
                      </div>
                    </div>
                  </article>
                </div>
              </div>
            </div>
            <div
              className="appearance-lab__library-companion-frame"
              data-appearance-library-companion="footer-tabs"
              data-companion-width-contract="390px"
            >
              <CompanionPanel
                className="appearance-lab__library-footer-companion"
                destination={{ id: "library-preview", scope: { kind: "global" } }}
                footer={
                  <LibraryPreviewCompanionFooter
                    activeTab="preview"
                    item={APPEARANCE_LIBRARY_PREVIEW_ITEM}
                    onActiveTabChange={() => undefined}
                  />
                }
                open
                title="Preview"
              >
                <div
                  className="appearance-lab__library-footer-content"
                  data-companion-scroll-owner="library-preview"
                >
                  <span className="appearance-lab__label">Companion footer</span>
                  <strong>Four tabs share the fixed 390 px panel width.</strong>
                </div>
              </CompanionPanel>
            </div>
          </div>
        </article>

        <article className="appearance-lab__playback-card">
          <header>
            <div>
              <span className="appearance-lab__label">Undivided Dialog</span>
              <strong>One modal glass sample</strong>
            </div>
            <code>header · content · footer</code>
          </header>
          <Dialog open>
            <GlassSurface
              className="app-dialog-content appearance-lab__dialog"
              data-appearance-fixture="undivided-dialog"
              elevation="modal"
              shape="panel"
              surfaceRole="overlay"
            >
              <DialogHeader>
                <DialogTitle>Sync app sessions</DialogTitle>
                <DialogDescription>Choose a browser profile on the same glass surface.</DialogDescription>
              </DialogHeader>
              <DialogListCard>
                <DialogListCardContent>
                  <DialogRow><span>Chrome</span><strong>Default profile</strong></DialogRow>
                  <DialogRow><span>Safari</span><strong>Personal</strong></DialogRow>
                </DialogListCardContent>
              </DialogListCard>
              <DialogFooter>
                <Button variant="ghost">Cancel</Button>
                <Button>Sync</Button>
              </DialogFooter>
            </GlassSurface>
          </Dialog>
        </article>

        <article className="appearance-lab__playback-card appearance-lab__playback-card--wide">
          <header>
            <div>
              <span className="appearance-lab__label">YouTube / RSS Watch</span>
              <strong>One continuous production surface and quiet subscription actions</strong>
            </div>
            <code>watch title + content · icon only</code>
          </header>
          <div
            aria-label="YouTube and RSS page root surfaces"
            className="appearance-lab__watch-root-contracts"
          >
            <div
              className="youtube-workspace-page app-workspace-primary-subpane appearance-lab__watch-root-sample"
              data-appearance-watch-root="youtube"
            >
              <strong>YouTube page root</strong>
            </div>
            <div
              className="rss-workspace-page app-dream-window app-workspace-primary-subpane appearance-lab__watch-root-sample"
              data-appearance-watch-root="rss"
            >
              <strong>RSS page root</strong>
            </div>
          </div>
          <div
            className="appearance-lab__watch-contract"
            data-appearance-fixture="youtube-rss-watch"
          >
            <YouTubePrimaryWatchPage
              video={APPEARANCE_YOUTUBE_VIDEO}
              videoDetails={APPEARANCE_YOUTUBE_DETAILS}
              playback={APPEARANCE_YOUTUBE_PLAYBACK}
              status={APPEARANCE_YOUTUBE_STATUS}
              rating="none"
              subscribed={false}
              subscriptionBusy={false}
              infoOpen={false}
              player={null}
              transport={null}
              locale="en"
              fallbackChannel="YouTube"
              labels={APPEARANCE_YOUTUBE_LABELS}
              onBack={() => undefined}
              onOpenURL={() => undefined}
              onLike={() => undefined}
              onDislike={() => undefined}
              onToggleSubscription={() => undefined}
              onInfoOpenChange={() => undefined}
              onOpenUploader={() => undefined}
            />
          </div>
          <div
            className="appearance-lab__uploader-subscription"
            data-appearance-subscription-context="uploader"
          >
            <strong>XiaDown Studio</strong>
            <YouTubeSubscriptionIconButton
              subscribed
              label="Unsubscribe"
              className="youtube-uploader-subscribe"
              onClick={() => undefined}
            />
          </div>
        </article>

        <article className="appearance-lab__playback-card appearance-lab__playback-card--wide">
          <header>
            <div>
              <span className="appearance-lab__label">Fullscreen playback</span>
              <strong>One native material and circular production controls</strong>
            </div>
            <code>idle · lyrics · queue · video</code>
          </header>
          <div
            className="app-main-shell appearance-lab__fullscreen-modes"
            data-surface-style="glass"
            data-window-material="native"
          >
            {APPEARANCE_FULLSCREEN_MODES.map((mode) => (
              <div
                className="listen-workspace-fullscreen-player appearance-lab__fullscreen-mode"
                data-fullscreen-media-mode={mode.mediaMode}
                data-player-context={mode.context}
                data-workspace-fullscreen="true"
                key={mode.id}
              >
                <ListenWorkspaceFullscreenBackdrop
                  candidates={["/dreamcreator.png"]}
                  playing={false}
                />
                <div className="appearance-lab__fullscreen-mode-label">
                  <strong>{mode.label}</strong>
                  <span>{mode.context}</span>
                </div>
                <div
                  className="appearance-lab__fullscreen-transport"
                  data-appearance-fullscreen-transport="true"
                >
                  <ListenPlayerTransport
                    loading={false}
                    playing={mode.id !== "idle"}
                    playMode="order"
                    text={APPEARANCE_PLAYBACK_TEXT}
                    onNext={() => undefined}
                    onPlayModeChange={() => undefined}
                    onPrevious={() => undefined}
                    onTogglePlayback={() => undefined}
                  />
                </div>
                <ListenPlayerFooter
                  airPlaySupported={false}
                  hasVideo
                  lyricsAvailable
                  mediaMode={mode.mediaMode}
                  muted={false}
                  onMediaModeChange={() => undefined}
                  onRequestVideoFullscreen={mode.id === "video" ? () => undefined : undefined}
                  onToggleMute={() => undefined}
                  onToggleQueue={() => undefined}
                  onToggleVideoAppFullscreen={mode.id === "video" ? () => undefined : undefined}
                  presentation="fullscreen"
                  queueOpen={mode.id === "queue"}
                  reserveWindowControls={false}
                  text={APPEARANCE_PLAYBACK_TEXT}
                />
              </div>
            ))}
          </div>
        </article>
      </div>
    </section>
  );
}

export function PrismGlassFixture({
  appearance,
  companionOpen,
  nativeVideoPreview,
  platform,
  presentation,
  reduceTransparency,
  surfaceStyle,
  onCompanionOpenChange,
  onNativeVideoPreviewChange,
  onPlatformChange,
  onPresentationChange,
  onReduceTransparencyChange,
  onSurfaceStyleChange,
}: PrismGlassFixtureProps) {
  return (
    <section className="appearance-lab__section appearance-lab__section--prism">
      <div className="appearance-lab__section-heading">
        <span>01</span>
        <div>
          <h2>Xia Prism workspace</h2>
          <p>One three-pane hierarchy, resolved through Glass or Contrast.</p>
        </div>
      </div>

      <div className="appearance-lab__prism-controls">
        <GlassGroup
          aria-label="Surface style"
          elevation="embedded"
          surfaceRole="control"
        >
          <Button
            aria-pressed={surfaceStyle === "glass"}
            onClick={() => onSurfaceStyleChange("glass")}
            shape="control"
            size="compact"
            variant={surfaceStyle === "glass" ? "default" : "ghost"}
          >
            Glass
          </Button>
          <Button
            aria-pressed={surfaceStyle === "contrast"}
            onClick={() => onSurfaceStyleChange("contrast")}
            shape="control"
            size="compact"
            variant={surfaceStyle === "contrast" ? "default" : "ghost"}
          >
            Contrast
          </Button>
        </GlassGroup>

        <GlassGroup
          aria-label="Companion presentation"
          elevation="embedded"
          surfaceRole="control"
        >
          <Button
            aria-pressed={presentation === "docked"}
            onClick={() => onPresentationChange("docked")}
            shape="control"
            size="compact"
            variant={presentation === "docked" ? "default" : "ghost"}
          >
            Docked
          </Button>
          <Button
            aria-pressed={presentation === "overlay"}
            onClick={() => onPresentationChange("overlay")}
            shape="control"
            size="compact"
            variant={presentation === "overlay" ? "default" : "ghost"}
          >
            Overlay
          </Button>
        </GlassGroup>

        <GlassGroup
          aria-label="Platform preview"
          elevation="embedded"
          surfaceRole="control"
        >
          <Button
            aria-pressed={platform === "macos"}
            onClick={() => onPlatformChange("macos")}
            shape="control"
            size="compact"
            variant={platform === "macos" ? "default" : "ghost"}
          >
            macOS
          </Button>
          <Button
            aria-pressed={platform === "windows"}
            onClick={() => onPlatformChange("windows")}
            shape="control"
            size="compact"
            variant={platform === "windows" ? "default" : "ghost"}
          >
            Windows
          </Button>
        </GlassGroup>

        <GlassGroup
          aria-label="Native video playback"
          elevation="embedded"
          surfaceRole="control"
        >
          {(["off", "youtube", "rss"] as const).map((preview) => (
            <Button
              aria-pressed={nativeVideoPreview === preview}
              data-appearance-native-video-state={preview}
              key={preview}
              onClick={() => onNativeVideoPreviewChange(preview)}
              shape="control"
              size="compact"
              variant={nativeVideoPreview === preview ? "default" : "ghost"}
            >
              {preview === "off"
                ? "Video off"
                : preview === "youtube"
                  ? "YouTube video"
                  : "RSS video"}
            </Button>
          ))}
        </GlassGroup>

        <Button
          aria-pressed={companionOpen}
          onClick={() => onCompanionOpenChange(!companionOpen)}
          shape="control"
          size="compact"
          variant={companionOpen ? "secondary" : "outline"}
        >
          Companion
        </Button>
        <Button
          aria-pressed={reduceTransparency}
          onClick={() => onReduceTransparencyChange(!reduceTransparency)}
          shape="control"
          size="compact"
          variant={reduceTransparency ? "secondary" : "outline"}
        >
          Reduce transparency
        </Button>
      </div>

      <div
        className="app-main-shell appearance-lab__prism-window"
        data-appearance={appearance}
        data-appearance-fixture="native-video-shell-isolation"
        data-companion-open={companionOpen ? "true" : "false"}
        data-native-video-preview={nativeVideoPreview}
        data-platform={platform}
        data-presentation={presentation}
        data-reduce-transparency={reduceTransparency ? "true" : "false"}
        data-surface-style={surfaceStyle}
      >
        <AppShell
          className="appearance-lab__prism-shell"
          companionOpen={companionOpen}
          companionPresentation={presentation}
          navigation={
            <WorkspaceSidebar
              aria-label="Prism Music navigation"
              className="appearance-lab__prism-sidebar"
              header={
                <div className="appearance-lab__prism-sidebar-header">
                  <div className="appearance-lab__prism-brand">
                    <span><Music2 aria-hidden="true" /></span>
                    <div><strong>Xia Music</strong><small>Personal station</small></div>
                  </div>
                </div>
              }
              bottom={
                <div className="appearance-lab__prism-mini-player">
                  <span className="appearance-lab__prism-mini-art" data-artwork="violet">
                    <Music2 aria-hidden="true" />
                  </span>
                  <span><strong>Violet Hours</strong><small>Night Archive</small></span>
                  <Button aria-label="Pause preview" shape="circle" size="compactIcon" variant="ghost">
                    <Pause aria-hidden="true" />
                  </Button>
                </div>
              }
              footer={
                <button className="appearance-lab__prism-account" type="button">
                  <span><UserRound aria-hidden="true" /></span>
                  <span><strong>Arnold</strong><small>Library synced</small></span>
                  <ChevronRight aria-hidden="true" />
                </button>
              }
            >
              <nav className="appearance-lab__prism-navigation" aria-label="Music sections">
                <span className="appearance-lab__label">Discover</span>
                <button aria-current="page" type="button"><Play aria-hidden="true" />Listen Now</button>
                <button type="button"><Radar aria-hidden="true" />Browse</button>
                <button type="button"><Music2 aria-hidden="true" />Radio</button>
                <span className="appearance-lab__label">Library</span>
                <button type="button"><Download aria-hidden="true" />Downloaded</button>
                <button type="button"><Check aria-hidden="true" />Recently Added</button>
                <button type="button"><Search aria-hidden="true" />Search</button>
              </nav>
            </WorkspaceSidebar>
          }
          primaryMinWidth={360}
          surfaceStyle={surfaceStyle}
        >
          <WorkspaceStage
            className="appearance-lab__prism-stage"
            companionOpen={companionOpen}
            companionPresentation={presentation}
            primaryMinWidth={360}
          >
            <PrimaryPane
              className="appearance-lab__prism-primary"
              minimumWidth={360}
            >
              <header className="appearance-lab__prism-primary-header">
                <div>
                  <span className="appearance-lab__label">Listen now</span>
                  <h3>Made for the quiet hours</h3>
                </div>
                <div className="appearance-lab__prism-platform-marker">
                  {platform === "macos"
                    ? "macOS · Liquid Glass"
                    : "Windows · Acrylic"}
                </div>
              </header>

              <div className="appearance-lab__prism-primary-scroll">
                <article className="appearance-lab__prism-feature">
                  <div className="appearance-lab__prism-feature-art" data-artwork="violet">
                    <Music2 aria-hidden="true" />
                  </div>
                  <div className="appearance-lab__prism-feature-copy">
                    <span>FEATURED MIX</span>
                    <h4>Afterglow Radio</h4>
                    <p>Dream pop, soft electronics and late-night discoveries.</p>
                    <div>
                      <Button shape="capsule" size="compact"><Play aria-hidden="true" />Play</Button>
                      <Button shape="circle" size="compactIcon" variant="secondary"><MoreHorizontal aria-hidden="true" /></Button>
                    </div>
                  </div>
                </article>

                <div className="appearance-lab__prism-album-heading">
                  <h4>Recently played</h4>
                  <button type="button">See all</button>
                </div>
                <div className="appearance-lab__prism-albums">
                  {[
                    ["violet", "Violet Hours", "Night Archive"],
                    ["amber", "Soft Current", "Mizu"],
                    ["blue", "Signals at Dawn", "Northbound"],
                  ].map(([tone, title, subtitle]) => (
                    <article key={title}>
                      <div data-artwork={tone}><Music2 aria-hidden="true" /></div>
                      <strong>{title}</strong>
                      <span>{subtitle}</span>
                    </article>
                  ))}
                </div>
              </div>
            </PrimaryPane>

            <CompanionPanel
              actions={
                <Button aria-label="More queue actions" shape="circle" size="compactIcon" variant="ghost">
                  <MoreHorizontal aria-hidden="true" />
                </Button>
              }
              className="appearance-lab__prism-companion"
              data-platform={platform}
              destination={{ id: "queue", scope: { kind: "global" } }}
              footer={
                <div className="appearance-lab__prism-companion-footer">
                  <span className="appearance-lab__prism-mini-art" data-artwork="violet"><Music2 aria-hidden="true" /></span>
                  <span><strong>Violet Hours</strong><small>1:24 / 3:42</small></span>
                  <Button aria-label="Pause queue preview" shape="circle" size="compactIcon" variant="secondary"><Pause aria-hidden="true" /></Button>
                </div>
              }
              open={companionOpen}
              presentation={presentation}
              title="Up Next"
            >
              <div
                className="appearance-lab__prism-queue"
                data-companion-scroll-owner="queue"
              >
                <div className="appearance-lab__prism-queue-summary">
                  <span>Playing from</span>
                  <strong>Afterglow Radio</strong>
                </div>
                {PRISM_QUEUE.map(([title, artist, duration], index) => (
                  <button
                    className="appearance-lab__prism-queue-row"
                    data-active={index === 0 ? "true" : undefined}
                    key={title}
                    type="button"
                  >
                    <span className="appearance-lab__prism-queue-index">{index === 0 ? <Pause aria-hidden="true" /> : index + 1}</span>
                    <span><strong>{title}</strong><small>{artist}</small></span>
                    <time>{duration}</time>
                  </button>
                ))}
              </div>
            </CompanionPanel>
          </WorkspaceStage>
        </AppShell>

        <div
          {...({ inert: "" } as React.HTMLAttributes<HTMLDivElement>)}
          aria-hidden="true"
          className="appearance-lab__prism-window-controls"
          data-platform={platform}
        >
          <WindowControls
            owner="primary"
            platform={platform}
            runtimeEnabled={false}
          />
        </div>
      </div>
    </section>
  );
}

export function AppearanceLab() {
  const [appearance, setAppearance] = React.useState<Appearance>("light");
  const [themePackId, setThemePackId] =
    React.useState<XiaThemePackId>("graphite");
  const [accentSource, setAccentSource] =
    React.useState<LabAccentSource>("theme");
  const [customAccent, setCustomAccent] = React.useState("#4f46e5");
  const [systemAccent, setSystemAccent] = React.useState("#0f766e");
  const [fontFamily, setFontFamily] = React.useState("");
  const [fontSize, setFontSize] = React.useState(0);
  const [prismPresentation, setPrismPresentation] =
    React.useState<CompanionPresentation>("docked");
  const [prismPlatform, setPrismPlatform] =
    React.useState<LabPlatform>("macos");
  const [reduceTransparency, setReduceTransparency] = React.useState(false);
  const [nativeVideoPreview, setNativeVideoPreview] =
    React.useState<AppearanceLabNativeVideoPreview>("off");
  const [companionOpen, setCompanionOpen] = React.useState(true);
  const [searchValue, setSearchValue] = React.useState("");
  const [surfaceStyle, setSurfaceStyle] =
    React.useState<XiaSurfaceStyle>("glass");

  React.useEffect(() => {
    const root = document.documentElement;
    const wasDark = root.classList.contains("dark");
    const previousPack = root.dataset.xiadownThemePack;
    const previousAccentMode = root.dataset.xiadownAccentMode;
    const previousSurfaceStyle = root.dataset.xiadownSurfaceStyle;
    const previousColorScheme = root.dataset.colorScheme;
    const previousSidebarStyle = root.dataset.xiadownSidebarStyle;
    const previousStyle = root.getAttribute("style");

    applyXiaTheme({
      appearanceConfig: {
        appearance: {
          accentMode: accentSource === "theme" ? "theme" : "color",
          surfaceStyle,
          themePackId,
        },
      },
      effectiveAppearance: appearance,
      fontFamily,
      fontSize,
      systemThemeColor: systemAccent,
      themeColor:
        accentSource === "system"
          ? "system"
          : accentSource === "custom"
            ? customAccent
            : "",
    } as unknown as Settings);

    return () => {
      root.classList.toggle("dark", wasDark);
      if (previousPack) {
        root.dataset.xiadownThemePack = previousPack;
      } else {
        delete root.dataset.xiadownThemePack;
      }
      if (previousAccentMode) {
        root.dataset.xiadownAccentMode = previousAccentMode;
      } else {
        delete root.dataset.xiadownAccentMode;
      }
      if (previousSurfaceStyle) {
        root.dataset.xiadownSurfaceStyle = previousSurfaceStyle;
      } else {
        delete root.dataset.xiadownSurfaceStyle;
      }
      if (previousColorScheme) {
        root.dataset.colorScheme = previousColorScheme;
      } else {
        delete root.dataset.colorScheme;
      }
      if (previousSidebarStyle) {
        root.dataset.xiadownSidebarStyle = previousSidebarStyle;
      } else {
        delete root.dataset.xiadownSidebarStyle;
      }
      if (previousStyle === null) {
        root.removeAttribute("style");
      } else {
        root.setAttribute("style", previousStyle);
      }
    };
  }, [
    accentSource,
    appearance,
    customAccent,
    fontFamily,
    fontSize,
    surfaceStyle,
    systemAccent,
    themePackId,
  ]);

  React.useEffect(() => {
    const root = document.documentElement;
    const previous = root.dataset.reduceTransparency;

    if (reduceTransparency) {
      root.dataset.reduceTransparency = "true";
    } else {
      delete root.dataset.reduceTransparency;
    }

    return () => {
      if (previous) {
        root.dataset.reduceTransparency = previous;
      } else {
        delete root.dataset.reduceTransparency;
      }
    };
  }, [reduceTransparency]);

  React.useEffect(
    () => applyAppearanceLabPlatform(document.documentElement, prismPlatform),
    [prismPlatform],
  );

  React.useEffect(
    () =>
      applyRootDatasetValue(
        document.documentElement,
        "windowMaterial",
        "native",
      ),
    [],
  );

  React.useEffect(
    () =>
      applyAppearanceLabNativeVideoPreview(
        document.documentElement,
        nativeVideoPreview,
      ),
    [nativeVideoPreview],
  );

  return (
    <main className="appearance-lab" data-surface-style={surfaceStyle}>
      <GlassSurface
        asChild
        elevation="embedded"
        shape="panel"
        surfaceRole="chrome"
      >
        <header className="appearance-lab__header">
          <div>
            <span className="appearance-lab__eyebrow">XiaDown Design System</span>
            <h1>Liquid Glass Appearance Lab</h1>
            <p>Material, hierarchy, contrast, shape and interaction review surface.</p>
          </div>
          <GlassGroup
            aria-label="Appearance controls"
            className="appearance-lab__switcher"
            shape="capsule"
            surfaceRole="control"
          >
            <Select
              aria-label="Theme pack"
              onChange={(event) =>
                setThemePackId(event.currentTarget.value as XiaThemePackId)
              }
              value={themePackId}
            >
              {XIA_THEME_PACKS.map((pack) => (
                <option key={pack.id} value={pack.id}>
                  {pack.id}
                </option>
              ))}
            </Select>
            <Select
              aria-label="Accent source"
              onChange={(event) =>
                setAccentSource(
                  event.currentTarget.value as LabAccentSource,
                )
              }
              value={accentSource}
            >
              <option value="theme">Theme accent</option>
              <option value="system">System accent</option>
              <option value="custom">Custom accent</option>
            </Select>
            {accentSource !== "theme" ? (
              <input
                aria-label={
                  accentSource === "system"
                    ? "Simulated system accent color"
                    : "Custom accent color"
                }
                className="appearance-lab__accent-input"
                onChange={(event) => {
                  if (accentSource === "system") {
                    setSystemAccent(event.currentTarget.value);
                  } else {
                    setCustomAccent(event.currentTarget.value);
                  }
                }}
                type="color"
                value={
                  accentSource === "system" ? systemAccent : customAccent
                }
              />
            ) : null}
            <Select
              aria-label="Font family"
              onChange={(event) => setFontFamily(event.currentTarget.value)}
              value={fontFamily}
            >
              <option value="">System font</option>
              <option value="Georgia">Georgia</option>
              <option value="Courier New">Courier New</option>
            </Select>
            <Select
              aria-label="Font size"
              onChange={(event) =>
                setFontSize(Number(event.currentTarget.value))
              }
              value={fontSize > 0 ? String(fontSize) : ""}
            >
              <option value="">Dream default</option>
              <option value="13">13 px</option>
              <option value="15">15 px</option>
              <option value="18">18 px</option>
            </Select>
            <Button
              aria-pressed={appearance === "light"}
              onClick={() => setAppearance("light")}
              shape="capsule"
              size="compact"
              variant={appearance === "light" ? "default" : "ghost"}
            >
              Light
            </Button>
            <Button
              aria-pressed={appearance === "dark"}
              onClick={() => setAppearance("dark")}
              shape="capsule"
              size="compact"
              variant={appearance === "dark" ? "default" : "ghost"}
            >
              Dark
            </Button>
          </GlassGroup>
        </header>
      </GlassSurface>

      <section className="appearance-lab__scene">
        <div className="appearance-lab__ambient appearance-lab__ambient--one" />
        <div className="appearance-lab__ambient appearance-lab__ambient--two" />

        <PrismGlassFixture
          appearance={appearance}
          companionOpen={companionOpen}
          nativeVideoPreview={nativeVideoPreview}
          onCompanionOpenChange={setCompanionOpen}
          onNativeVideoPreviewChange={setNativeVideoPreview}
          onPlatformChange={setPrismPlatform}
          onPresentationChange={setPrismPresentation}
          onReduceTransparencyChange={setReduceTransparency}
          onSurfaceStyleChange={setSurfaceStyle}
          platform={prismPlatform}
          presentation={prismPresentation}
          reduceTransparency={reduceTransparency}
          surfaceStyle={surfaceStyle}
        />

        <SurfaceRoleMatrix surfaceStyle={surfaceStyle} />

        <section className="appearance-lab__section">
          <div className="appearance-lab__section-heading">
            <span>03</span>
            <div>
              <h2>Material hierarchy</h2>
              <p>Content stays tonal; controls and navigation float above it.</p>
            </div>
          </div>
          <div className="appearance-lab__material-grid">
            <article className="appearance-lab__content-card">
              <span className="appearance-lab__label">Tonal content</span>
              <h3>Library card</h3>
              <p>No backdrop sampling inside the content layer.</p>
            </article>
            <GlassSurface
              data-appearance-material-specimen="regular"
              material="regular"
              shape="card"
            >
              <article className="appearance-lab__sample">
                <span className="appearance-lab__label">Regular</span>
                <h3>Floating status</h3>
                <p>Lightweight navigation and transient controls.</p>
              </article>
            </GlassSurface>
            <GlassSurface
              data-appearance-material-specimen="panel"
              material="panel"
              shape="panel"
            >
              <article className="appearance-lab__sample">
                <span className="appearance-lab__label">Panel</span>
                <h3>Dialog surface</h3>
                <p>Thicker material for text-heavy presentations.</p>
              </article>
            </GlassSurface>
            <div className="appearance-lab__media-sample">
              <GlassSurface
                className="appearance-lab__clear-control"
                data-appearance-material-specimen="clear"
                material="clear"
                shape="capsule"
              >
                <Play aria-hidden="true" />
                Clear on media
              </GlassSurface>
            </div>
          </div>
        </section>

        <section className="appearance-lab__section">
          <div className="appearance-lab__section-heading">
            <span>04</span>
            <div>
              <h2>Controls and states</h2>
              <p>Small Mac controls remain rounded rectangles; emphasis is semantic.</p>
            </div>
          </div>
          <GlassSurface
            className="appearance-lab__controls-panel"
            elevation="embedded"
            shape="panel"
            surfaceRole="card"
          >
            <div className="appearance-lab__control-row">
              <Button shape="control">Primary</Button>
              <Button shape="control" variant="secondary">Secondary</Button>
              <Button shape="control" variant="outline">Outline</Button>
              <Button shape="control" variant="ghost">Ghost</Button>
              <Button shape="control" tone="destructive" variant="outline">
                Destructive
              </Button>
              <Button disabled shape="control">Disabled</Button>
            </div>
            <div className="appearance-lab__control-row">
              <WorkspaceSearchControl
                autoFocus={false}
                className="appearance-lab__workspace-search"
                clearLabel="Clear search"
                onValueChange={setSearchValue}
                placeholder="Search library"
                submitLabel="Search"
                value={searchValue}
              />
              <Select aria-label="Quality">
                <option>Best quality</option>
                <option>Balanced</option>
              </Select>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button shape="control" variant="outline">
                    Menu
                    <MoreHorizontal aria-hidden="true" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="start">
                  <DropdownMenuLabel>Material</DropdownMenuLabel>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem>Regular surface</DropdownMenuItem>
                  <DropdownMenuItem>Panel surface</DropdownMenuItem>
                  <DropdownMenuItem>Clear on media</DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
              <GlassGroup
                aria-label="View options"
                elevation="embedded"
                surfaceRole="control"
              >
                <Button aria-label="Previous" shape="square" size="compactIcon" variant="ghost">
                  <ChevronLeft />
                </Button>
                <Button shape="control" size="compact" variant="default">Albums</Button>
                <Button aria-label="Next" shape="square" size="compactIcon" variant="ghost">
                  <ChevronRight />
                </Button>
              </GlassGroup>
            </div>
            <div className="appearance-lab__control-row" aria-label="Semantic status badges">
              <StatusBadge tone="busy" icon={<Radar />}>Working</StatusBadge>
              <StatusBadge tone="success" icon={<Check />}>Available</StatusBadge>
              <StatusBadge tone="warning" marker>Needs review</StatusBadge>
              <StatusBadge tone="danger" icon={<AlertTriangle />}>Missing</StatusBadge>
              <StatusBadge tone="muted" marker>Trashed</StatusBadge>
            </div>
          </GlassSurface>
        </section>

        <section className="appearance-lab__section">
          <div className="appearance-lab__section-heading">
            <span>05</span>
            <div>
              <h2>Playback &amp; status surfaces</h2>
              <p>Production playback anatomy, queue media semantics and transient status recipes.</p>
            </div>
          </div>

          <div
            className="appearance-lab__playback-grid"
            data-appearance-fixture="playback-surfaces"
          >
            <article className="appearance-lab__playback-card appearance-lab__playback-card--wide">
              <header>
                <div>
                  <span className="appearance-lab__label">Workspace transport</span>
                  <strong>Track identity and three-tier controls</strong>
                </div>
                <code>34px artwork · 27 / 30 / 34px controls</code>
              </header>
              <div className="appearance-lab__production-transport">
                <MusicWorkspaceTransportBar
                  status={APPEARANCE_NOW_PLAYING_STATUS}
                  labels={APPEARANCE_TRANSPORT_LABELS}
                  volume={0.72}
                  muted={false}
                  onCommand={() => undefined}
                  onDownload={() => undefined}
                  onFavorite={() => undefined}
                  onFullscreen={() => undefined}
                  onOpenArtist={() => undefined}
                  onOpenLyrics={() => undefined}
                  onOpenPlayer={() => undefined}
                  onOpenQueue={() => undefined}
                  onOpenURL={() => undefined}
                  onToggleMute={() => undefined}
                  onVolumeChange={() => undefined}
                />
              </div>
            </article>

            <article className="appearance-lab__playback-card">
              <header>
                <div>
                  <span className="appearance-lab__label">Now Playing</span>
                  <strong>Mode / skip / primary hierarchy</strong>
                </div>
                <code>2 · 2.5 · 3rem</code>
              </header>
              <div className="appearance-lab__now-playing-controls">
                <ListenPlayerTransport
                  playing
                  loading={false}
                  playMode="order"
                  text={APPEARANCE_PLAYBACK_TEXT}
                  onNext={() => undefined}
                  onPlayModeChange={() => undefined}
                  onPrevious={() => undefined}
                  onTogglePlayback={() => undefined}
                />
              </div>
              <div className="appearance-lab__now-playing-footer">
                <ListenPlayerFooter
                  mediaMode="lyrics"
                  presentation="companion"
                  reserveWindowControls={false}
                  airPlaySupported={false}
                  sourceBadge={<Music2 aria-hidden="true" />}
                  sourceLabel="YouTube Music"
                  hasVideo={false}
                  lyricsAvailable
                  text={APPEARANCE_PLAYBACK_TEXT}
                  companionControls={
                    <ListenWorkspaceQueueModeSwitch
                      playMode="order"
                      text={APPEARANCE_PLAYBACK_TEXT}
                      onChange={() => undefined}
                    />
                  }
                  onMediaModeChange={() => undefined}
                  onOpenSource={() => undefined}
                  onRequestFullscreen={() => undefined}
                  onToggleQueue={() => undefined}
                />
              </div>
            </article>

            <article className="appearance-lab__playback-card">
              <header>
                <div>
                  <span className="appearance-lab__label">Up Next</span>
                  <strong>Track artwork remains rectangular</strong>
                </div>
                <code>track ≠ artist</code>
              </header>
              <div className="appearance-lab__queue-specimen">
                <ListenWorkspaceOnlineQueueCompanion
                  queueTitle="Up next"
                  queueItems={APPEARANCE_PLAYBACK_QUEUE}
                  selectedQueueId="appearance-queue-current"
                  httpBaseURL=""
                  playMode="order"
                  text={APPEARANCE_PLAYBACK_TEXT}
                  showFooter={false}
                  onPlayModeChange={() => undefined}
                  onSelectQueueTrack={() => undefined}
                />
              </div>
            </article>

            <article className="appearance-lab__playback-card appearance-lab__playback-card--wide">
              <header>
                <div>
                  <span className="appearance-lab__label">Focus lyrics</span>
                  <strong>Wrapped karaoke paint with exclusive row clips</strong>
                </div>
                <code>one semantic copy</code>
              </header>
              <div className="appearance-lab__focus-lyrics-specimen">
                <ListenLyricsSurface
                  renderer="focus"
                  variant="companion"
                  text={APPEARANCE_PLAYBACK_TEXT}
                  lyrics={APPEARANCE_FOCUS_LYRICS}
                  currentTimeMs={3600}
                  timelineRunning={false}
                  onSeek={() => undefined}
                />
              </div>
            </article>
          </div>

          <div className="appearance-lab__statuses">
            <GlassSurface
              className="appearance-lab__status"
              shape="card"
              surfaceRole="status"
            >
              <Radar aria-hidden="true" />
              <div><strong>Sniffing media</strong><span>12 resources found</span></div>
              <span className="appearance-lab__status-dot" data-tone="busy" />
            </GlassSurface>
            <GlassSurface
              className="appearance-lab__status"
              shape="card"
              surfaceRole="status"
            >
              <Check aria-hidden="true" />
              <div><strong>Download complete</strong><span>Ready in Library</span></div>
              <span className="appearance-lab__status-dot" data-tone="success" />
            </GlassSurface>
            <GlassSurface
              className="appearance-lab__status"
              shape="card"
              surfaceRole="status"
            >
              <AlertTriangle aria-hidden="true" />
              <div><strong>Connection lost</strong><span>Retrying automatically</span></div>
              <span className="appearance-lab__status-dot" data-tone="error" />
            </GlassSurface>
          </div>
        </section>

        <RegressionContractSpecimen />

        <PrimitiveFixtureGallery />
        <DreamStyleCatalog />
      </section>
    </main>
  );
}
