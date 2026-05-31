import * as React from "react";
import { System } from "@wailsio/runtime";
import {
  Activity,
  Archive,
  BrushCleaning,
  Circle,
  Copy,
  Database,
  Download,
  ExternalLink,
  Eye,
  FileText,
  FilterX,
  Film,
  Globe2,
  ImageIcon,
  Info,
  Loader2,
  Music,
  Radar,
  Radio,
  Search,
  Video,
  X,
} from "lucide-react";

import { WindowControls } from "@/components/layout/WindowControls";
import { MediaPreviewDialog, type MediaPreviewKind } from "@/app/media";
import { DEFAULT_COVER_IMAGE_URL } from "@/shared/assets/default-cover";
import { getXiaText } from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
import type {
  ResourceSniffRawResource,
  ResourceSniffSession,
} from "@/shared/contracts/library";
import type { Pet } from "@/shared/contracts/pets";
import { messageBus } from "@/shared/message";
import {
  useCancelResourceSniff,
  useClearResourceSniffResources,
  useCreateYTDLPJob,
  usePrepareResourceSniffRawDownload,
  usePrepareResourceSniffRawPreview,
  useResourceSniffResources,
  useResourceSniffSessions,
  useStartResourceSniff,
} from "@/shared/query/library";
import { openExternalURL } from "@/shared/query/system";
import { useSettingsStore } from "@/shared/store/settings";
import { Button } from "@/shared/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/shared/ui/dialog";
import {
  pickFunButtonEffect,
  type FunButtonEffect,
} from "@/shared/ui/fun-button-effect";
import { Input } from "@/shared/ui/input";
import { PetDisplay } from "@/shared/ui/pet-player";
import { Select } from "@/shared/ui/select";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/shared/ui/tooltip";
import type { PetAnimation } from "@/shared/pets/animation";
import { formatBytes } from "@/shared/utils/formatBytes";
import {
  resolveSniffDeskErrorDescription,
  resolveStartSniffFailureDescription,
} from "./error-prompts";

type XiaText = ReturnType<typeof getXiaText>;

type SniffKindFilter =
  | "all"
  | "video"
  | "audio"
  | "live"
  | "manifest"
  | "image"
  | "subtitle"
  | "api"
  | "document"
  | "font"
  | "archive"
  | "other";

type SniffSourceFilter =
  | "all"
  | "network"
  | "candidate"
  | "rejected";
type ConcreteSniffSourceFilter = Exclude<SniffSourceFilter, "all">;

type SniffDownloadFilter = "all" | "downloadable";
type LivePreviewSourceType = "hls" | "dash";

const DEFAULT_KIND_FILTERS: SniffKindFilter[] = [
  "all",
  "video",
  "audio",
  "live",
  "image",
  "subtitle",
];

const ADVANCED_KIND_FILTERS: SniffKindFilter[] = [
  ...DEFAULT_KIND_FILTERS,
  "manifest",
  "api",
  "other",
];

const ALL_KIND_FILTERS: SniffKindFilter[] = [
  ...DEFAULT_KIND_FILTERS,
  "manifest",
  "api",
  "document",
  "font",
  "archive",
  "other",
];

const SOURCE_FILTERS: SniffSourceFilter[] = [
  "all",
  "network",
  "candidate",
  "rejected",
];

const DOWNLOAD_FILTERS: SniffDownloadFilter[] = ["all", "downloadable"];
const SNIFF_RESOURCE_ROW_HEIGHT = 72;
const SNIFF_RESOURCE_ROW_OVERSCAN = 8;
const SNIFF_RESOURCE_LIST_BOTTOM_PADDING = 80;
const resourceSearchTextCache = new WeakMap<ResourceSniffRawResource, string>();

function formatTemplate(template: string, params: Record<string, string>) {
  return Object.entries(params).reduce(
    (output, [key, value]) => output.split(`{${key}}`).join(value),
    template,
  );
}

function normalized(value?: string) {
  return (value ?? "").trim().toLowerCase();
}

function sniffResourceURLPathExtension(value?: string) {
  const raw = (value ?? "").trim();
  if (!raw) {
    return "";
  }
  let path = raw.split("#")[0]?.split("?")[0] ?? raw;
  try {
    path = new URL(raw).pathname;
  } catch {
    // Non-URL values are handled by the conservative split above.
  }
  const leaf = path.split("/").pop() ?? path;
  let decodedLeaf = leaf;
  try {
    decodedLeaf = decodeURIComponent(leaf);
  } catch {
    decodedLeaf = leaf;
  }
  return decodedLeaf.toLowerCase().match(/\.([a-z0-9]+)$/i)?.[1] ?? "";
}

function sniffKindFiltersForScope(scope?: string): SniffKindFilter[] {
  switch (normalized(scope)) {
    case "advanced":
      return ADVANCED_KIND_FILTERS;
    case "all":
      return ALL_KIND_FILTERS;
    default:
      return DEFAULT_KIND_FILTERS;
  }
}

function displayURL(value?: string) {
  const raw = (value ?? "").trim();
  if (!raw) {
    return "-";
  }
  try {
    const parsed = new URL(raw);
    const path = `${parsed.pathname}${parsed.search}`;
    return `${parsed.hostname}${path || "/"}`;
  } catch {
    return raw;
  }
}

function dataURLMimeType(value?: string) {
  const raw = (value ?? "").trim();
  const match = raw.match(/^data:([^;,]+)[;,]/i);
  return match?.[1]?.trim() ?? "";
}

function displaySniffResourceURL(value?: string) {
  const mimeType = dataURLMimeType(value);
  if (mimeType) {
    return `${mimeType} data URL`;
  }
  return displayURL(value);
}

function displaySniffResourceTitle(resource: ResourceSniffRawResource) {
  if (dataURLMimeType(resource.url)) {
    return displaySniffResourceURL(resource.url);
  }
  return resource.domain || displaySniffResourceURL(resource.url);
}

function displayDomain(value?: string) {
  const raw = (value ?? "").trim();
  if (!raw) {
    return "";
  }
  try {
    return new URL(raw).hostname;
  } catch {
    return "";
  }
}

function isFetchableRemoteURL(value?: string) {
  const raw = (value ?? "").trim();
  if (!raw) {
    return false;
  }
  try {
    const parsed = new URL(raw);
    return parsed.protocol === "http:" || parsed.protocol === "https:";
  } catch {
    return false;
  }
}

async function copyTextToClipboard(value: string) {
  const text = value.trim();
  if (!text) {
    return;
  }
  if (navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text);
      return;
    } catch {
      // Fall back to the legacy command path below.
    }
  }
  const textarea = document.createElement("textarea");
  textarea.value = text;
  textarea.setAttribute("readonly", "true");
  textarea.style.position = "fixed";
  textarea.style.left = "-9999px";
  textarea.style.top = "0";
  document.body.appendChild(textarea);
  textarea.select();
  try {
    const clipboardCommand = ["co", "py"].join("");
    if (!document.execCommand(clipboardCommand)) {
      throw new Error("clipboard command failed");
    }
  } finally {
    document.body.removeChild(textarea);
  }
}

function isLiveSniffResource(resource: ResourceSniffRawResource) {
  return normalized(resource.kind) === "live";
}

function isSegmentSniffResource(resource: ResourceSniffRawResource) {
  const kind = normalized(resource.kind);
  const lowerMime = normalized(
    `${resource.mimeType ?? ""} ${resource.contentType ?? ""}`,
  );
  const ext = sniffResourceURLPathExtension(resource.url);
  const lowerURL = normalized(resource.url);
  return (
    kind === "segment" ||
    lowerMime.includes("iso.segment") ||
    lowerMime.includes("mp2t") ||
    [
      "m4s",
      "cmfv",
      "cmfa",
      "cmft",
      "cmfm",
      "f4f",
      "mp2t",
      "m2t",
      "m2ts",
      "mts",
    ].includes(ext) ||
    ((ext === "ts" || ext === "part") &&
      (lowerMime.startsWith("video/") ||
        lowerMime.startsWith("audio/") ||
        lowerMime.includes("octet-stream"))) ||
    lowerURL.includes("/fragments(") ||
    lowerURL.includes("/fragment(") ||
    lowerURL.includes("fragments(video=") ||
    lowerURL.includes("fragments(audio=")
  );
}

function isDownloadableSniffResource(resource: ResourceSniffRawResource) {
  const kind = normalized(resource.kind);
  if (!resource.downloadable || !kind || kind === "segment" || kind === "live") {
    return false;
  }
  return isFetchableRemoteURL(resource.url);
}

function shouldHideNoisyInlineResource(
  resource: ResourceSniffRawResource,
  scope?: string,
) {
  return normalized(scope) !== "all" && Boolean(dataURLMimeType(resource.url));
}

function displayTime(value?: string) {
  const raw = (value ?? "").trim();
  if (!raw) {
    return "-";
  }
  const date = new Date(raw);
  if (Number.isNaN(date.getTime())) {
    return raw;
  }
  return date.toLocaleTimeString([], {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

function buildResourcePreviewDataURL(mimeType?: string, dataBase64?: string) {
  const data = (dataBase64 ?? "").trim();
  if (!data) {
    return "";
  }
  const mime = (mimeType ?? "").trim() || "image/png";
  return `data:${mime};base64,${data}`;
}

function buildSniffResourceListPreviewSrc(resource: ResourceSniffRawResource) {
  if (normalized(resource.kind) !== "image" || !isFetchableRemoteURL(resource.url)) {
    return "";
  }
  return buildResourcePreviewDataURL(
    resource.previewMimeType || resource.mimeType || resource.contentType,
    resource.previewDataBase64,
  );
}

function sniffResourceSizeBytes(resource: ResourceSniffRawResource) {
  return resource.sizeBytes || resource.previewSizeBytes;
}

function buildSniffResourcePreviewURL(baseURL: string, leaseId: string, fileName?: string) {
  const trimmedBase = baseURL.replace(/\/+$/, "");
  const trimmedLease = leaseId.trim();
  if (!trimmedBase || !trimmedLease) {
    return "";
  }
  const safeName = (fileName ?? "preview").trim() || "preview";
  return `${trimmedBase}/api/sniff/resource-preview/${encodeURIComponent(trimmedLease)}/${encodeURIComponent(safeName)}`;
}

function isClosedBrowserStatus(value?: string) {
  const status = normalized(value);
  return status === "browser_closed" || status === "closed";
}

function resourceSearchText(resource: ResourceSniffRawResource) {
  const cached = resourceSearchTextCache.get(resource);
  if (cached !== undefined) {
    return cached;
  }
  const searchText = [
    resource.url,
    resource.pageUrl,
    resource.domain,
    resource.kind,
    resource.source,
    resource.mimeType,
    resource.contentType,
    resource.resourceType,
    resource.reason,
  ]
    .filter(Boolean)
    .join(" ")
    .toLowerCase();
  resourceSearchTextCache.set(resource, searchText);
  return searchText;
}

function resourceRowEqual(
  left: ResourceSniffRawResource,
  right: ResourceSniffRawResource,
) {
  return (
    left.id === right.id &&
    left.source === right.source &&
    left.kind === right.kind &&
    left.url === right.url &&
    left.pageUrl === right.pageUrl &&
    left.domain === right.domain &&
    left.mimeType === right.mimeType &&
    left.contentType === right.contentType &&
    left.resourceType === right.resourceType &&
    left.status === right.status &&
    left.sizeBytes === right.sizeBytes &&
    left.score === right.score &&
    left.reason === right.reason &&
    left.targetId === right.targetId &&
    left.seenAt === right.seenAt &&
    left.downloadable === right.downloadable &&
    left.previewAvailable === right.previewAvailable &&
    left.previewKind === right.previewKind &&
    left.previewMimeType === right.previewMimeType &&
    left.previewSizeBytes === right.previewSizeBytes &&
    left.previewDataBase64 === right.previewDataBase64
  );
}

function resolveKindLabel(text: XiaText, kind?: string) {
  switch (normalized(kind)) {
    case "video":
      return text.sniffDesk.kindVideo;
    case "audio":
      return text.sniffDesk.kindAudio;
    case "live":
      return text.sniffDesk.kindLive;
    case "manifest":
      return text.sniffDesk.kindManifest;
    case "image":
      return text.sniffDesk.kindImage;
    case "subtitle":
      return text.sniffDesk.kindSubtitle;
    case "api":
      return text.sniffDesk.kindApi;
    case "document":
      return text.sniffDesk.kindDocument;
    case "font":
      return text.sniffDesk.kindFont;
    case "archive":
      return text.sniffDesk.kindArchive;
    default:
      return text.sniffDesk.kindOther;
  }
}

function resolveSourceLabel(text: XiaText, source?: string) {
  switch (normalized(source)) {
    case "network":
      return text.sniffDesk.sourceNetwork;
    case "candidate":
      return text.sniffDesk.sourceCandidate;
    case "rejected":
      return text.sniffDesk.sourceRejected;
    case "api_response":
      return text.sniffDesk.sourceApiResponse;
    case "subtitle":
      return text.sniffDesk.sourceSubtitle;
    default:
      return source || "-";
  }
}

function resolveSniffSourceFilter(
  resource: ResourceSniffRawResource,
): ConcreteSniffSourceFilter {
  const source = normalized(resource.source);
  if (source === "candidate" || source === "rejected") {
    return source;
  }
  return "network";
}

function resolveReasonLabel(text: XiaText, reason?: string) {
  const raw = (reason ?? "").trim();
  const value = normalized(raw);
  if (!value) {
    return text.sniffDesk.reasonUnknown;
  }
  const httpStatus = value.match(/^http_status_(\d+)$/);
  if (httpStatus?.[1]) {
    return formatTemplate(text.sniffDesk.reasonHttpStatus, {
      status: httpStatus[1],
    });
  }
  switch (value) {
    case "empty_url":
      return text.sniffDesk.reasonEmptyURL;
    case "too_small":
      return text.sniffDesk.reasonTooSmall;
    case "not_video":
      return text.sniffDesk.reasonNotVideo;
    case "no_video_signal":
      return text.sniffDesk.reasonNoVideoSignal;
    case "low_score":
      return text.sniffDesk.reasonLowScore;
    case "weak_candidate":
      return text.sniffDesk.reasonWeakCandidate;
    default:
      return formatTemplate(text.sniffDesk.reasonOther, { reason: raw });
  }
}

function resolveSessionStatusLabel(text: XiaText, session?: ResourceSniffSession | null) {
  const browserStatus = normalized(session?.browserStatus);
  if (browserStatus === "open") {
    return text.sniffDesk.statusOpen;
  }
  if (browserStatus === "closing") {
    return text.sniffDesk.statusClosing;
  }
  if (browserStatus === "tab_closed") {
    return text.sniffDesk.statusTabClosed;
  }
  if (isClosedBrowserStatus(browserStatus)) {
    return text.sniffDesk.statusClosed;
  }
  return session?.state || "-";
}

function resolvePrimarySession(
  sessions: ResourceSniffSession[],
  preferredSessionId: string,
) {
  if (preferredSessionId) {
    const preferred = sessions.find((session) => session.sessionId === preferredSessionId);
    if (preferred) {
      return preferred;
    }
  }
  return (
    sessions.find((session) => !isClosedBrowserStatus(session.browserStatus)) ??
    sessions[0] ??
    null
  );
}

function KindIcon(props: { kind?: string; className?: string }) {
  switch (normalized(props.kind)) {
    case "video":
      return <Video className={props.className} />;
    case "audio":
      return <Music className={props.className} />;
    case "live":
      return <Radio className={props.className} />;
    case "manifest":
      return <Film className={props.className} />;
    case "image":
      return <ImageIcon className={props.className} />;
    case "api":
      return <Database className={props.className} />;
    case "subtitle":
    case "document":
    case "font":
      return <FileText className={props.className} />;
    case "archive":
      return <Archive className={props.className} />;
    default:
      return <Circle className={props.className} />;
  }
}

function isImageResource(resource: ResourceSniffRawResource) {
  return normalized(resource.kind) === "image";
}

function isImagePreviewableResource(resource: ResourceSniffRawResource) {
  return isImageResource(resource) && isFetchableRemoteURL(resource.url);
}

function sniffLivePreviewSourceType(
  resource?: ResourceSniffRawResource | null,
): LivePreviewSourceType | undefined {
  if (!resource) {
    return undefined;
  }
  const ext = sniffResourceURLPathExtension(resource.url);
  const lowerMime = normalized(resource.mimeType || resource.contentType);
  if (ext === "m3u8" || lowerMime.includes("mpegurl")) {
    return "hls";
  }
  if (ext === "mpd" || lowerMime.includes("dash+xml")) {
    return "dash";
  }
  return undefined;
}

function isLivePreviewResource(resource: ResourceSniffRawResource) {
  return (
    isLiveSniffResource(resource) &&
    isFetchableRemoteURL(resource.url) &&
    Boolean(sniffLivePreviewSourceType(resource))
  );
}

function isFlvPreviewResource(resource: ResourceSniffRawResource) {
  if (isSegmentSniffResource(resource) || !isFetchableRemoteURL(resource.url)) {
    return false;
  }
  const kind = normalized(resource.kind);
  if (kind !== "video" && kind !== "live") {
    return false;
  }
  const ext = sniffResourceURLPathExtension(resource.url);
  const lowerMime = normalized(resource.mimeType || resource.contentType);
  return ext === "flv" || lowerMime.includes("x-flv");
}

function isVideoPreviewResource(resource: ResourceSniffRawResource) {
  const kind = normalized(resource.kind);
  if (kind !== "video") {
    return false;
  }
  if (!isFetchableRemoteURL(resource.url)) {
    return false;
  }
  const ext = sniffResourceURLPathExtension(resource.url);
  const lowerMime = normalized(resource.mimeType || resource.contentType);
  if (ext === "flv" || lowerMime.includes("x-flv")) {
    return false;
  }
  if (
    ext === "m3u8" ||
    ext === "mpd" ||
    lowerMime.includes("mpegurl") ||
    lowerMime.includes("dash+xml")
  ) {
    return Boolean(sniffLivePreviewSourceType(resource));
  }
  return (
    lowerMime.startsWith("video/") ||
    ext === "mp4" ||
    ext === "m4v" ||
    ext === "mov" ||
    ext === "webm"
  );
}

function canPreviewSniffResource(resource: ResourceSniffRawResource) {
  return (
    isImagePreviewableResource(resource) ||
    isFlvPreviewResource(resource) ||
    isVideoPreviewResource(resource) ||
    isLivePreviewResource(resource)
  );
}

function SniffStat(props: { label: string; value: React.ReactNode }) {
  return (
    <div className="app-sniff-desk-stat min-w-0 px-3 py-2">
      <div className="truncate text-[10px] font-semibold uppercase text-muted-foreground">
        {props.label}
      </div>
      <div className="mt-0.5 truncate text-xs font-semibold text-foreground">
        {props.value}
      </div>
    </div>
  );
}

const SniffResourceRow = React.memo(function SniffResourceRow(props: {
  resource: ResourceSniffRawResource;
  text: XiaText;
  downloading: boolean;
  previewSrc?: string;
  onDownload: (resource: ResourceSniffRawResource) => void;
  onCopy: (resource: ResourceSniffRawResource) => void;
  onOpenPreview: (resource: ResourceSniffRawResource) => void;
}) {
  const { resource, text, downloading, onDownload, onCopy, onOpenPreview } = props;
  const mime = resource.mimeType || resource.contentType || resource.resourceType || "-";
  const sourceLabel = resolveSourceLabel(text, resource.source);
  const canOpenURL = isFetchableRemoteURL(resource.url);
  const canDownload = isDownloadableSniffResource(resource);
  const canCopy = isLiveSniffResource(resource) && canOpenURL;
  const canPreview = canPreviewSniffResource(resource);
  const previewSrc = props.previewSrc?.trim() || buildSniffResourceListPreviewSrc(resource);

  const previewIcon = previewSrc ? (
    <img
      src={previewSrc}
      alt=""
      className="app-sniff-desk-preview-thumb h-full w-full"
      draggable={false}
    />
  ) : (
    <KindIcon kind={resource.kind} className="h-4 w-4" />
  );

  return (
    <div
      className="app-sniff-desk-resource-row grid h-full min-h-0 grid-cols-[minmax(0,1fr)_auto] gap-3 px-3 py-2.5"
      data-downloadable={canDownload ? "true" : "false"}
    >
      <div className="grid min-w-0 grid-cols-[auto_minmax(0,1fr)] gap-3">
        <div
          className="app-sniff-desk-kind-icon mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center"
          data-preview-loaded={previewSrc ? "true" : undefined}
        >
          {previewIcon}
        </div>
        <div className="min-w-0">
          <div className="flex min-w-0 items-center gap-2">
            <span className="app-sniff-desk-kind-badge shrink-0 px-2 py-1 text-[11px] font-semibold">
              {resolveKindLabel(text, resource.kind)}
            </span>
            <span className="truncate text-xs font-semibold text-foreground">
              {displaySniffResourceTitle(resource)}
            </span>
          </div>
          <div className="mt-1 truncate font-mono text-[11px] text-muted-foreground">
            {displaySniffResourceURL(resource.url)}
          </div>
          <div className="mt-1 flex min-w-0 flex-nowrap items-center gap-x-3 overflow-hidden text-[11px] text-muted-foreground">
            <span className="truncate">
              {sourceLabel}
            </span>
            <span className="truncate">
              {text.sniffDesk.mime}: {mime}
            </span>
            <span className="truncate">
              {text.sniffDesk.size}: {formatBytes(sniffResourceSizeBytes(resource))}
            </span>
            {resource.status ? <span>{resource.status}</span> : null}
            {resource.score ? (
              <span>
                {text.sniffDesk.score}: {resource.score}
              </span>
            ) : null}
            {resource.reason ? (
              <span className="truncate">
                {text.sniffDesk.reason}: {resolveReasonLabel(text, resource.reason)}
              </span>
            ) : null}
            {resource.seenAt ? <span>{displayTime(resource.seenAt)}</span> : null}
          </div>
        </div>
      </div>

      <div className="flex items-center gap-1">
        <Tooltip>
          <TooltipTrigger asChild>
            {canPreview ? (
              <Button
                type="button"
                size="compactIcon"
                variant="ghost"
                aria-label={text.sniffDesk.preview}
                onClick={() => onOpenPreview(resource)}
              >
                <Eye className="h-4 w-4" />
              </Button>
            ) : (
              <Button
                type="button"
                size="compactIcon"
                variant="ghost"
                aria-label={text.actions.open}
                disabled={!canOpenURL}
                onClick={() => {
                  if (canOpenURL) {
                    void openExternalURL(resource.url);
                  }
                }}
              >
                <ExternalLink className="h-4 w-4" />
              </Button>
            )}
          </TooltipTrigger>
          <TooltipContent>
            {canPreview ? text.sniffDesk.preview : text.actions.open}
          </TooltipContent>
        </Tooltip>
        <Tooltip>
          <TooltipTrigger asChild>
            {canCopy ? (
              <Button
                type="button"
                size="compactIcon"
                variant="default"
                aria-label={text.sniffDesk.copyLink}
                onClick={() => onCopy(resource)}
              >
                <Copy className="h-4 w-4" />
              </Button>
            ) : (
              <Button
                type="button"
                size="compactIcon"
                variant={canDownload ? "default" : "ghost"}
                aria-label={text.actions.download}
                disabled={!canDownload || downloading}
                onClick={() => onDownload(resource)}
              >
                {downloading ? (
                  <Loader2 className="h-4 w-4 animate-spin" />
                ) : (
                  <Download className="h-4 w-4" />
                )}
              </Button>
            )}
          </TooltipTrigger>
          <TooltipContent>
            {canCopy ? text.sniffDesk.copyLink : text.actions.download}
          </TooltipContent>
        </Tooltip>
      </div>
    </div>
  );
}, (previous, next) => (
  previous.text === next.text &&
  previous.downloading === next.downloading &&
  previous.previewSrc === next.previewSrc &&
  previous.onDownload === next.onDownload &&
  previous.onCopy === next.onCopy &&
  previous.onOpenPreview === next.onOpenPreview &&
  resourceRowEqual(previous.resource, next.resource)
));

function useSniffVirtualRows(itemCount: number, resetKey: string) {
  const scrollRef = React.useRef<HTMLDivElement | null>(null);
  const animationFrameRef = React.useRef<number | null>(null);
  const nextScrollTopRef = React.useRef(0);
  const [scrollState, setScrollState] = React.useState({
    scrollTop: 0,
    viewportHeight: 0,
  });

  const updateScrollState = React.useCallback((scrollTop: number, viewportHeight?: number) => {
    setScrollState((current) => {
      const nextViewportHeight = viewportHeight ?? current.viewportHeight;
      if (
        current.scrollTop === scrollTop &&
        current.viewportHeight === nextViewportHeight
      ) {
        return current;
      }
      return {
        scrollTop,
        viewportHeight: nextViewportHeight,
      };
    });
  }, []);

  const handleScroll = React.useCallback((event: React.UIEvent<HTMLDivElement>) => {
    nextScrollTopRef.current = event.currentTarget.scrollTop;
    if (animationFrameRef.current !== null) {
      return;
    }
    animationFrameRef.current = window.requestAnimationFrame(() => {
      animationFrameRef.current = null;
      updateScrollState(nextScrollTopRef.current);
    });
  }, [updateScrollState]);

  React.useEffect(() => {
    const node = scrollRef.current;
    if (!node) {
      return undefined;
    }

    const measure = () => {
      updateScrollState(node.scrollTop, node.clientHeight);
    };

    measure();
    const resizeObserver =
      typeof ResizeObserver === "undefined" ? null : new ResizeObserver(measure);
    resizeObserver?.observe(node);
    window.addEventListener("resize", measure);
    return () => {
      resizeObserver?.disconnect();
      window.removeEventListener("resize", measure);
    };
  }, [updateScrollState]);

  React.useEffect(() => {
    const node = scrollRef.current;
    if (!node) {
      return;
    }
    node.scrollTop = 0;
    nextScrollTopRef.current = 0;
    updateScrollState(0, node.clientHeight);
  }, [resetKey, updateScrollState]);

  React.useEffect(() => () => {
    if (animationFrameRef.current !== null) {
      window.cancelAnimationFrame(animationFrameRef.current);
    }
  }, []);

  const startIndex = Math.max(
    0,
    Math.floor(scrollState.scrollTop / SNIFF_RESOURCE_ROW_HEIGHT) -
      SNIFF_RESOURCE_ROW_OVERSCAN,
  );
  const visibleCount = Math.max(
    SNIFF_RESOURCE_ROW_OVERSCAN * 2 + 1,
    Math.ceil(scrollState.viewportHeight / SNIFF_RESOURCE_ROW_HEIGHT) +
      SNIFF_RESOURCE_ROW_OVERSCAN * 2,
  );
  const endIndex = Math.min(itemCount, startIndex + visibleCount);

  return {
    scrollRef,
    handleScroll,
    startIndex,
    endIndex,
    totalHeight:
      itemCount * SNIFF_RESOURCE_ROW_HEIGHT + SNIFF_RESOURCE_LIST_BOTTOM_PADDING,
  };
}

function SniffResourceVirtualList(props: {
  resources: ResourceSniffRawResource[];
  text: XiaText;
  downloadingResourceId: string;
  onDownload: (resource: ResourceSniffRawResource) => void;
  onCopy: (resource: ResourceSniffRawResource) => void;
  onOpenPreview: (resource: ResourceSniffRawResource) => void;
  resetKey: string;
}) {
  const virtualRows = useSniffVirtualRows(props.resources.length, props.resetKey);
  const visibleResources = props.resources.slice(
    virtualRows.startIndex,
    virtualRows.endIndex,
  );

  return (
    <div
      ref={virtualRows.scrollRef}
      className="app-sniff-desk-virtual-list h-full overflow-y-auto"
      onScroll={virtualRows.handleScroll}
    >
      <div
        className="app-sniff-desk-virtual-spacer relative min-h-full w-full"
        style={{ height: virtualRows.totalHeight }}
      >
        {visibleResources.map((resource, offset) => {
          const index = virtualRows.startIndex + offset;
          return (
            <div
              key={resource.id}
              className="app-sniff-desk-virtual-row absolute inset-x-0"
              style={{
                height: SNIFF_RESOURCE_ROW_HEIGHT,
                transform: `translateY(${index * SNIFF_RESOURCE_ROW_HEIGHT}px)`,
              }}
            >
              <SniffResourceRow
                resource={resource}
                text={props.text}
                downloading={props.downloadingResourceId === resource.id}
                previewSrc={buildSniffResourceListPreviewSrc(resource)}
                onDownload={props.onDownload}
                onCopy={props.onCopy}
                onOpenPreview={props.onOpenPreview}
              />
            </div>
          );
        })}
      </div>
    </div>
  );
}

function SniffDeskPetPrompt(props: {
  pet: Pet | null;
  petImageURL: string;
  label?: string;
  animation?: PetAnimation;
  variant?: "page" | "card";
}) {
  const pagePrompt = props.variant === "page";

  return (
    <div
      className={cn(
        pagePrompt
          ? "app-sniff-desk-page-prompt flex min-h-0 flex-1 items-center justify-center px-4 pb-16"
          : "flex h-full min-h-0 items-center justify-center px-4 py-8",
      )}
    >
      <div
        className={cn(
          "inline-flex max-w-[20rem] flex-col items-center",
          pagePrompt ? "gap-3" : "gap-2",
        )}
      >
        <div
          className={cn(
            "max-w-full items-center",
            pagePrompt
              ? "flex flex-col gap-2"
              : "grid grid-cols-[3rem_minmax(0,1fr)] gap-2",
          )}
        >
          <PetDisplay
            pet={props.pet}
            imageUrl={props.petImageURL}
            animation={props.animation ?? "review"}
            alt=""
            size={pagePrompt ? 96 : 48}
            className={cn("shrink-0", pagePrompt ? "h-24 w-24" : "h-12 w-12")}
            glowClassName="opacity-0"
          />
          {props.label ? (
            <span
              className={cn(
                "min-w-0 text-xs font-medium text-muted-foreground",
                pagePrompt ? "text-center leading-5" : "leading-4",
              )}
            >
              {props.label}
            </span>
          ) : null}
        </div>
      </div>
    </div>
  );
}

export function SniffDeskPage(props: {
  text: XiaText;
  active: boolean;
  pet: Pet | null;
  petImageURL: string;
  httpBaseURL: string;
}) {
  const { text, active } = props;
  const isWindows = System.IsWindows();
  const resourceSniffScope = useSettingsStore(
    (state) => state.settings?.resourceSniffScope ?? "default",
  );
  const sessionsQuery = useResourceSniffSessions(active);
  const startSniff = useStartResourceSniff();
  const cancelSniff = useCancelResourceSniff();
  const clearResources = useClearResourceSniffResources();
  const prepareRawPreview = usePrepareResourceSniffRawPreview();
  const prepareRawDownload = usePrepareResourceSniffRawDownload();
  const createYTDLP = useCreateYTDLPJob();
  const urlInputRef = React.useRef<HTMLInputElement | null>(null);
  const bottomControlRef = React.useRef<HTMLDivElement | null>(null);
  const [url, setURL] = React.useState("");
  const [controlMode, setControlMode] = React.useState<"idle" | "input">("idle");
  const [startEffect] = React.useState<FunButtonEffect>(() =>
    pickFunButtonEffect(),
  );
  const [detailsOpen, setDetailsOpen] = React.useState(false);
  const [clearConfirmOpen, setClearConfirmOpen] = React.useState(false);
  const [preferredSessionId, setPreferredSessionId] = React.useState("");
  const [query, setQuery] = React.useState("");
  const [kindFilter, setKindFilter] = React.useState<SniffKindFilter>("all");
  const [sourceFilter, setSourceFilter] = React.useState<SniffSourceFilter>("all");
  const [downloadFilter, setDownloadFilter] = React.useState<SniffDownloadFilter>("all");
  const [downloadingResourceId, setDownloadingResourceId] = React.useState("");
  const [previewDialogResource, setPreviewDialogResource] =
    React.useState<ResourceSniffRawResource | null>(null);
  const [previewDialogImageLoadedId, setPreviewDialogImageLoadedId] =
    React.useState("");
  const [previewDialogImageFailedId, setPreviewDialogImageFailedId] =
    React.useState("");
  const [streamPreviewURLs, setStreamPreviewURLs] = React.useState<Record<string, string>>({});
  const [streamPreviewLoadingResourceIds, setStreamPreviewLoadingResourceIds] =
    React.useState<Record<string, boolean>>({});
  const streamPreviewURLsRef = React.useRef<Record<string, string>>({});
  const streamPreviewLoadingIdsRef = React.useRef<Set<string>>(new Set());
  const streamPreviewFailedIdsRef = React.useRef<Set<string>>(new Set());
  const deferredQuery = React.useDeferredValue(query);
  const kindFilters = React.useMemo(
    () => sniffKindFiltersForScope(resourceSniffScope),
    [resourceSniffScope],
  );

  const sessions = sessionsQuery.data ?? [];
  const currentSession = React.useMemo(
    () => resolvePrimarySession(sessions, preferredSessionId),
    [preferredSessionId, sessions],
  );
  const currentSessionId = currentSession?.sessionId || preferredSessionId;
  const resourcesQuery = useResourceSniffResources(
    currentSessionId ? { sessionId: currentSessionId } : null,
    active && Boolean(currentSessionId),
  );
  const resources = resourcesQuery.data?.resources ?? [];

  React.useEffect(() => {
    if (!kindFilters.includes(kindFilter)) {
      setKindFilter("all");
    }
  }, [kindFilter, kindFilters]);

  React.useEffect(() => {
    if (!active) {
      return;
    }
    if (sessions.length === 0) {
      setPreferredSessionId("");
      return;
    }
    const next = resolvePrimarySession(sessions, preferredSessionId);
    if (next && next.sessionId !== preferredSessionId) {
      setPreferredSessionId(next.sessionId);
    }
  }, [active, preferredSessionId, sessions]);

  React.useEffect(() => {
    if (controlMode !== "input" || currentSession) {
      return;
    }
    const frame = window.requestAnimationFrame(() => {
      urlInputRef.current?.focus();
    });
    return () => window.cancelAnimationFrame(frame);
  }, [controlMode, currentSession]);

  React.useEffect(() => {
    if (controlMode !== "input" || currentSession || startSniff.isPending) {
      return undefined;
    }
    const handlePointerDown = (event: PointerEvent) => {
      const target = event.target;
      if (!(target instanceof Node)) {
        return;
      }
      if (bottomControlRef.current?.contains(target)) {
        return;
      }
      setControlMode("idle");
      setURL("");
    };
    document.addEventListener("pointerdown", handlePointerDown, true);
    return () => {
      document.removeEventListener("pointerdown", handlePointerDown, true);
    };
  }, [controlMode, currentSession, startSniff.isPending]);

  React.useEffect(() => {
    streamPreviewURLsRef.current = streamPreviewURLs;
  }, [streamPreviewURLs]);

  React.useEffect(() => {
    setStreamPreviewURLs({});
    streamPreviewURLsRef.current = {};
    streamPreviewLoadingIdsRef.current.clear();
    streamPreviewFailedIdsRef.current.clear();
    setStreamPreviewLoadingResourceIds({});
    setPreviewDialogResource(null);
    setPreviewDialogImageLoadedId("");
    setPreviewDialogImageFailedId("");
  }, [currentSessionId]);

  const resourceView = React.useMemo(() => {
    const normalizedQuery = normalized(deferredQuery);
    let downloadableCount = 0;
    const filteredResources: ResourceSniffRawResource[] = [];
    for (const resource of resources) {
      if (isSegmentSniffResource(resource)) {
        continue;
      }
      if (shouldHideNoisyInlineResource(resource, resourceSniffScope)) {
        continue;
      }
      if (isDownloadableSniffResource(resource)) {
        downloadableCount += 1;
      }
      if (kindFilter !== "all" && normalized(resource.kind) !== kindFilter) {
        continue;
      }
      if (sourceFilter !== "all" && resolveSniffSourceFilter(resource) !== sourceFilter) {
        continue;
      }
      if (downloadFilter === "downloadable" && !isDownloadableSniffResource(resource)) {
        continue;
      }
      if (normalizedQuery && !resourceSearchText(resource).includes(normalizedQuery)) {
        continue;
      }
      filteredResources.push(resource);
    }
    return { downloadableCount, filteredResources };
  }, [deferredQuery, downloadFilter, kindFilter, resources, resourceSniffScope, sourceFilter]);
  const { downloadableCount, filteredResources } = resourceView;
  const currentPage = currentSession?.currentUrl || currentSession?.url || "";
  const sessionDomain =
    currentSession?.unoptimizedDomain ||
    displayDomain(currentPage) ||
    displayURL(currentPage);
  const sessionTitle =
    currentSession?.title ||
    currentSession?.unoptimizedDomain ||
    currentPage ||
    text.sniffDesk.emptySessions;
  const sessionStatus = resolveSessionStatusLabel(text, currentSession);
  const sessionClosing =
    normalized(currentSession?.state) === "closing" ||
    normalized(currentSession?.browserStatus) === "closing";
  const hasActiveFilters =
    query.trim() !== "" ||
    kindFilter !== "all" ||
    sourceFilter !== "all" ||
    downloadFilter !== "all";
  const virtualListResetKey = [
    currentSessionId,
    deferredQuery,
    kindFilter,
    sourceFilter,
    downloadFilter,
  ].join("\u0000");
  const pagePrompt: {
    label: string;
    animation?: PetAnimation;
  } | null = (() => {
    if (resourcesQuery.isFetching && resources.length === 0) {
      return { label: text.sniffDesk.loading, animation: "running" };
    }
    if (filteredResources.length === 0) {
      if (resources.length > 0 && hasActiveFilters) {
        return { label: text.sniffDesk.emptyFilteredResources, animation: "review" };
      }
      return { label: text.sniffDesk.emptyResources, animation: "review" };
    }
    return null;
  })();

  const handleStartSniff = React.useCallback(async () => {
    const trimmed = url.trim();
    if (!trimmed) {
      messageBus.publishToast({
        intent: "danger",
        title: text.sniffDesk.startFailed,
        description: text.sniffDesk.urlRequired,
      });
      return;
    }
    if (startSniff.isPending) {
      return;
    }
    try {
      const result = await startSniff.mutateAsync({ url: trimmed });
      if (result.session?.sessionId) {
        setPreferredSessionId(result.session.sessionId);
        setControlMode("idle");
        setDetailsOpen(false);
        setURL("");
      }
      if (result.failure) {
        messageBus.publishToast({
          intent: "danger",
          title: text.sniffDesk.startFailed,
          description: resolveStartSniffFailureDescription(text, result.failure),
        });
      }
    } catch (error) {
      messageBus.publishToast({
        intent: "danger",
        title: text.sniffDesk.startFailed,
        description: resolveSniffDeskErrorDescription(text, error),
      });
    }
  }, [startSniff, text, url]);

  const handleCancelSession = React.useCallback(async () => {
    if (!currentSession) {
      return;
    }
    try {
      await cancelSniff.mutateAsync({ sessionId: currentSession.sessionId });
      setPreferredSessionId("");
      setDetailsOpen(false);
      setControlMode("idle");
    } catch (error) {
      messageBus.publishToast({
        intent: "danger",
        title: text.actions.closeBrowser,
        description: resolveSniffDeskErrorDescription(text, error),
      });
    }
  }, [cancelSniff, currentSession, text]);

  const resetFilters = React.useCallback(() => {
    setQuery("");
    setKindFilter("all");
    setSourceFilter("all");
    setDownloadFilter("all");
  }, []);

  const handleClearResources = React.useCallback(async () => {
    if (!currentSessionId || clearResources.isPending) {
      return;
    }
    try {
      await clearResources.mutateAsync({ sessionId: currentSessionId });
      resetFilters();
      setStreamPreviewURLs({});
      streamPreviewURLsRef.current = {};
      streamPreviewLoadingIdsRef.current.clear();
      streamPreviewFailedIdsRef.current.clear();
      setStreamPreviewLoadingResourceIds({});
      setPreviewDialogResource(null);
      setClearConfirmOpen(false);
      messageBus.publishToast({
        intent: "success",
        title: text.sniffDesk.clearResourcesSucceeded,
      });
    } catch (error) {
      messageBus.publishToast({
        intent: "danger",
        title: text.sniffDesk.clearResourcesFailed,
        description: resolveSniffDeskErrorDescription(text, error),
      });
    }
  }, [clearResources, currentSessionId, resetFilters, text]);

  const ensureStreamPreviewResource = React.useCallback(
    async (
      resource: ResourceSniffRawResource,
      options?: { silent?: boolean },
    ) => {
      if (!currentSessionId || !canPreviewSniffResource(resource)) {
        return "";
      }
      if (options?.silent && streamPreviewFailedIdsRef.current.has(resource.id)) {
        return "";
      }
      const cached = streamPreviewURLsRef.current[resource.id]?.trim();
      if (cached) {
        return cached;
      }
      if (streamPreviewLoadingIdsRef.current.has(resource.id)) {
        return "";
      }
      streamPreviewFailedIdsRef.current.delete(resource.id);
      streamPreviewLoadingIdsRef.current.add(resource.id);
      setStreamPreviewLoadingResourceIds((current) => ({
        ...current,
        [resource.id]: true,
      }));
      try {
        const preview = await prepareRawPreview.mutateAsync({
          sessionId: currentSessionId,
          resourceId: resource.id,
        });
        const src = buildSniffResourcePreviewURL(
          props.httpBaseURL,
          preview.leaseId,
          preview.fileName,
        );
        if (!src) {
          throw new Error(text.sniffDesk.previewFailed);
        }
        setStreamPreviewURLs((current) => {
          const next = {
            ...current,
            [resource.id]: src,
          };
          streamPreviewURLsRef.current = next;
          return next;
        });
        streamPreviewFailedIdsRef.current.delete(resource.id);
        return src;
      } catch (error) {
        streamPreviewFailedIdsRef.current.add(resource.id);
        if (!options?.silent) {
          messageBus.publishToast({
            intent: "danger",
            title: text.sniffDesk.previewFailed,
            description: resolveSniffDeskErrorDescription(text, error),
          });
        }
        return "";
      } finally {
        streamPreviewLoadingIdsRef.current.delete(resource.id);
        setStreamPreviewLoadingResourceIds((current) => {
          if (!current[resource.id]) {
            return current;
          }
          const next = { ...current };
          delete next[resource.id];
          return next;
        });
      }
    },
    [currentSessionId, prepareRawPreview, props.httpBaseURL, text],
  );

  const handleOpenPreviewResource = React.useCallback(
    (resource: ResourceSniffRawResource) => {
      if (!canPreviewSniffResource(resource)) {
        return;
      }
      setPreviewDialogResource(resource);
      setPreviewDialogImageLoadedId("");
      setPreviewDialogImageFailedId("");
      void ensureStreamPreviewResource(resource);
    },
    [ensureStreamPreviewResource],
  );

  const handleCopyResourceURL = React.useCallback(
    async (resource: ResourceSniffRawResource) => {
      if (!resource.url?.trim()) {
        return;
      }
      try {
        await copyTextToClipboard(resource.url);
        messageBus.publishToast({
          intent: "success",
          title: text.sniffDesk.linkCopied,
        });
      } catch (error) {
        messageBus.publishToast({
          intent: "danger",
          title: text.sniffDesk.copyFailed,
          description: resolveSniffDeskErrorDescription(text, error),
        });
      }
    },
    [text],
  );

  const handleDownloadResource = React.useCallback(
    async (resource: ResourceSniffRawResource) => {
      if (!currentSessionId || !isDownloadableSniffResource(resource)) {
        return;
      }
      setDownloadingResourceId(resource.id);
      try {
        const prepared = await prepareRawDownload.mutateAsync({
          sessionId: currentSessionId,
          resourceId: resource.id,
        });
        const selectedFormat = prepared.formats[0];
        const selectedQuality =
          selectedFormat?.hasVideo === false && selectedFormat?.hasAudio
            ? "audio"
            : "best";
        await createYTDLP.mutateAsync({
          url: prepared.pageUrl || resource.pageUrl || resource.url,
          source: "xiadown.sniff.desk",
          caller: "main",
          mode: "custom",
          title: prepared.title || resource.domain || displayURL(resource.url),
          extractor: prepared.extractor || "sniff",
          author: prepared.author || undefined,
          thumbnailUrl: prepared.thumbnailUrl || undefined,
          writeThumbnail: false,
          quality: selectedQuality,
          formatId: selectedFormat?.id,
          resourceMediaId: prepared.resourceMediaId,
        });
        messageBus.publishToast({
          intent: "success",
          title: text.sniffDesk.downloadStarted,
        });
      } catch (error) {
        messageBus.publishToast({
          intent: "danger",
          title: text.sniffDesk.downloadFailed,
          description: resolveSniffDeskErrorDescription(text, error),
        });
      } finally {
        setDownloadingResourceId("");
      }
    },
    [createYTDLP, currentSessionId, prepareRawDownload, text],
  );

  const previewDialogIsImage = previewDialogResource
    ? isImageResource(previewDialogResource)
    : false;
  const previewDialogIsFlv = previewDialogResource
    ? isFlvPreviewResource(previewDialogResource)
    : false;
  const previewDialogIsVideo = previewDialogResource
    ? isVideoPreviewResource(previewDialogResource)
    : false;
  const previewDialogStreamSourceType = sniffLivePreviewSourceType(previewDialogResource);
  const previewDialogIsLive = previewDialogResource
    ? isLivePreviewResource(previewDialogResource)
    : false;
  const previewDialogSrc = previewDialogResource
    ? streamPreviewURLs[previewDialogResource.id]?.trim() ?? ""
    : "";
  const previewDialogLoading = previewDialogResource
    ? Boolean(streamPreviewLoadingResourceIds[previewDialogResource.id])
    : false;
  const previewDialogURL = previewDialogResource?.url?.trim()
    ? displayURL(previewDialogResource.url)
    : "";
  const previewDialogImageFailed = Boolean(
    previewDialogResource &&
      previewDialogImageFailedId === previewDialogResource.id,
  );
  const previewDialogImageLoaded = Boolean(
    previewDialogResource &&
      previewDialogImageLoadedId === previewDialogResource.id,
  );
  const previewDialogDownloading = Boolean(
    previewDialogResource &&
      downloadingResourceId === previewDialogResource.id,
  );
  const previewDialogKind: MediaPreviewKind = previewDialogIsFlv
    ? "flv"
    : previewDialogIsLive
      ? "live"
      : previewDialogIsVideo
        ? "video"
        : previewDialogIsImage
          ? "image"
          : "unsupported";

  return (
    <div className="app-main-page app-sniff-desk-page relative flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden bg-background">
      <header
        className={cn(
          "app-sniff-desk-page-toolbar wails-drag flex min-h-[var(--app-page-top-drag-height)] shrink-0 items-center justify-between gap-4 px-5",
          isWindows
            ? "min-h-[var(--app-page-top-drag-height)] pb-3 pt-4"
            : "pb-3 pt-4",
        )}
      >
        <div className="flex min-w-0 items-center gap-2 text-sm font-semibold text-foreground">
          <Radar className="h-4 w-4 shrink-0 text-primary" />
          <span className="truncate">{text.sniffDesk.title}</span>
        </div>

        <div
          className={cn(
            "flex min-w-0 items-center justify-end gap-2",
            isWindows && "min-w-[var(--app-windows-caption-control-width)]",
          )}
        >
          {isWindows ? <WindowControls platform="windows" /> : null}
        </div>
      </header>

      <div className="app-sniff-desk-content flex min-h-0 flex-1 flex-col overflow-hidden px-6 pb-5 pt-2">
        {currentSession ? (
          <section className="app-sniff-desk-toolbar mb-3 flex flex-nowrap items-center justify-between gap-2 overflow-hidden">
            <div className="app-sniff-desk-filter-strip flex min-w-0 flex-1 flex-nowrap items-center gap-2 overflow-x-auto overflow-y-hidden">
              <div className="app-dream-search-control app-dream-control-shell h-9 w-[12.5rem] shrink-0 px-3">
                <Search className="h-4 w-4 text-muted-foreground" />
                <Input
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  placeholder={text.sniffDesk.searchPlaceholder}
                  size="compact"
                  className="app-control-input-compact h-auto rounded-none border-0 bg-transparent px-0 shadow-none"
                />
              </div>
              <Select
                value={kindFilter}
                onChange={(event) => setKindFilter(event.target.value as SniffKindFilter)}
                aria-label={text.sniffDesk.kindFilter}
                className="h-9 w-[8.5rem]"
              >
                {kindFilters.map((kind) => (
                  <option key={kind} value={kind}>
                    {kind === "all"
                      ? text.sniffDesk.allKinds
                      : resolveKindLabel(text, kind)}
                  </option>
                ))}
              </Select>
              <Select
                value={sourceFilter}
                onChange={(event) => setSourceFilter(event.target.value as SniffSourceFilter)}
                aria-label={text.sniffDesk.sourceFilter}
                className="h-9 w-[8.5rem]"
              >
                {SOURCE_FILTERS.map((source) => (
                  <option key={source} value={source}>
                    {source === "all"
                      ? text.sniffDesk.allSources
                      : resolveSourceLabel(text, source)}
                  </option>
                ))}
              </Select>
              <Select
                value={downloadFilter}
                onChange={(event) => setDownloadFilter(event.target.value as SniffDownloadFilter)}
                aria-label={text.sniffDesk.downloadFilter}
                className="h-9 w-[9.5rem]"
              >
                {DOWNLOAD_FILTERS.map((filter) => (
                  <option key={filter} value={filter}>
                    {filter === "all"
                      ? text.sniffDesk.allDownloads
                      : text.sniffDesk.downloadableOnly}
                  </option>
                ))}
              </Select>
            </div>
            <div className="app-sniff-desk-toolbar-actions ml-auto flex shrink-0 items-center gap-1">
              {hasActiveFilters ? (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      type="button"
                      size="compactIcon"
                      variant="ghost"
                      className="h-9 w-9"
                      aria-label={text.sniffDesk.resetFilters}
                      onClick={resetFilters}
                    >
                      <FilterX className="h-4 w-4" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>{text.sniffDesk.resetFilters}</TooltipContent>
                </Tooltip>
              ) : null}
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    type="button"
                    size="compactIcon"
                    variant="ghost"
                    className="h-9 w-9 text-muted-foreground hover:text-foreground"
                    aria-label={text.sniffDesk.clearResources}
                    disabled={resources.length === 0 || clearResources.isPending}
                    onClick={() => setClearConfirmOpen(true)}
                  >
                    {clearResources.isPending ? (
                      <Loader2 className="h-4 w-4 animate-spin" />
                    ) : (
                      <BrushCleaning className="h-4 w-4" />
                    )}
                  </Button>
                </TooltipTrigger>
                <TooltipContent>{text.sniffDesk.clearResources}</TooltipContent>
              </Tooltip>
            </div>
          </section>
        ) : null}

        {!currentSession ? (
          <SniffDeskPetPrompt
            pet={props.pet}
            petImageURL={props.petImageURL}
            label={text.sniffDesk.emptySessions}
            animation="idle"
            variant="page"
          />
        ) : (
          <section className="app-sniff-desk-table-shell min-h-0 flex-1 overflow-hidden">
            {pagePrompt ? (
              <SniffDeskPetPrompt
                pet={props.pet}
                petImageURL={props.petImageURL}
                label={pagePrompt.label}
                animation={pagePrompt.animation}
                variant="card"
              />
            ) : (
              <SniffResourceVirtualList
                resources={filteredResources}
                text={text}
                downloadingResourceId={downloadingResourceId}
                onDownload={handleDownloadResource}
                onCopy={handleCopyResourceURL}
                onOpenPreview={handleOpenPreviewResource}
                resetKey={virtualListResetKey}
              />
            )}
          </section>
        )}
      </div>

      <MediaPreviewDialog
        open={Boolean(previewDialogResource)}
        onOpenChange={(open) => {
          if (!open) {
            setPreviewDialogResource(null);
            setPreviewDialogImageLoadedId("");
            setPreviewDialogImageFailedId("");
          }
        }}
        dialogTitle={text.sniffDesk.preview}
        description={previewDialogURL}
        descriptionCopyValue={previewDialogResource?.url?.trim() ?? ""}
        descriptionCopyLabel={text.sniffDesk.copyLink}
        onDescriptionCopy={() => {
          if (previewDialogResource) {
            return handleCopyResourceURL(previewDialogResource);
          }
        }}
        labels={{
          ...text.completed,
          loading: text.sniffDesk.loading,
          unsupported: text.sniffDesk.previewUnsupported,
        }}
        kind={previewDialogKind}
        mediaUrl={
          previewDialogIsImage && previewDialogImageFailed
            ? ""
            : previewDialogSrc
        }
        title={
          previewDialogResource
            ? displaySniffResourceTitle(previewDialogResource)
            : ""
        }
        imageAlt=""
        imageClassName="app-media-preview-dialog-image"
        loading={previewDialogLoading}
        loaded={
          ((previewDialogIsVideo || previewDialogIsLive || previewDialogIsFlv) &&
            Boolean(previewDialogSrc)) ||
          previewDialogImageLoaded
        }
        posterUrl={DEFAULT_COVER_IMAGE_URL}
        streamType={
          previewDialogIsLive ||
          (previewDialogIsFlv &&
            previewDialogResource &&
            isLiveSniffResource(previewDialogResource))
            ? "live"
            : undefined
        }
        sourceType={previewDialogStreamSourceType}
        persistProgress={previewDialogIsLive ? false : undefined}
        persistKey={
          previewDialogResource && (previewDialogIsVideo || previewDialogIsFlv)
            ? `sniff-desk:${previewDialogResource.id}`
            : undefined
        }
        videoClassName="app-media-preview-video"
        preventDismiss
        closeLabel={text.actions.close}
        actionSlot={
          <>
            <Button
              type="button"
              variant="outline"
              disabled={!previewDialogResource?.url}
              onClick={() => {
                if (previewDialogResource?.url) {
                  void openExternalURL(previewDialogResource.url);
                }
              }}
            >
              <ExternalLink className="h-4 w-4" />
              {text.actions.open}
            </Button>
            {previewDialogIsLive ? (
              <Button
                type="button"
                variant="default"
                disabled={!previewDialogResource?.url}
                onClick={() => {
                  if (previewDialogResource) {
                    void handleCopyResourceURL(previewDialogResource);
                  }
                }}
              >
                <Copy className="h-4 w-4" />
                {text.sniffDesk.copyLink}
              </Button>
            ) : (
              <Button
                type="button"
                variant={
                  previewDialogResource &&
                  isDownloadableSniffResource(previewDialogResource)
                    ? "default"
                    : "outline"
                }
                disabled={
                  !previewDialogResource ||
                  !isDownloadableSniffResource(previewDialogResource) ||
                  previewDialogDownloading
                }
                onClick={() => {
                  if (previewDialogResource) {
                    void handleDownloadResource(previewDialogResource);
                  }
                }}
              >
                {previewDialogDownloading ? (
                  <Loader2 className="h-4 w-4 animate-spin" />
                ) : (
                  <Download className="h-4 w-4" />
                )}
                {text.actions.download}
              </Button>
            )}
          </>
        }
        onImageLoad={() => {
          if (previewDialogResource) {
            setPreviewDialogImageLoadedId(previewDialogResource.id);
            setPreviewDialogImageFailedId("");
          }
        }}
        onImageError={() => {
          if (previewDialogResource) {
            setPreviewDialogImageFailedId(previewDialogResource.id);
            setPreviewDialogImageLoadedId("");
          }
        }}
      />

      <Dialog
        open={clearConfirmOpen}
        onOpenChange={(open) => {
          if (!clearResources.isPending) {
            setClearConfirmOpen(open);
          }
        }}
      >
        <DialogContent className="w-[min(24rem,calc(100vw-2rem))] max-w-none">
          <DialogHeader>
            <DialogTitle>{text.sniffDesk.clearResourcesTitle}</DialogTitle>
            <DialogDescription>{text.sniffDesk.clearResourcesMessage}</DialogDescription>
          </DialogHeader>
          <div className="app-dialog-footer flex flex-nowrap items-center justify-between gap-2">
            <DialogClose asChild>
              <Button type="button" variant="outline" disabled={clearResources.isPending}>
                {text.actions.cancelDialog}
              </Button>
            </DialogClose>
            <Button
              type="button"
              variant="default"
              disabled={!currentSessionId || clearResources.isPending}
              onClick={() => void handleClearResources()}
            >
              {clearResources.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
              {!clearResources.isPending ? <BrushCleaning className="h-4 w-4" /> : null}
              {text.sniffDesk.clearResources}
            </Button>
          </div>
        </DialogContent>
      </Dialog>

      <div className="app-sniff-desk-floating-layer pointer-events-none absolute inset-x-0 bottom-5 z-40 flex justify-center px-5">
        <div
          ref={bottomControlRef}
          className="app-sniff-desk-bottom-control pointer-events-auto flex min-w-0 flex-col items-center"
          data-state={currentSession ? "session" : controlMode}
        >
          {currentSession && detailsOpen ? (
            <div className="app-sniff-desk-detail-popover mb-2 w-full px-4 py-3">
              <div className="mb-3 flex min-w-0 items-center gap-3">
                <div
                  className="app-sniff-desk-session-icon flex h-9 w-9 shrink-0 items-center justify-center"
                  data-active="true"
                >
                  <Activity className="h-4 w-4" />
                </div>
                <div className="min-w-0">
                  <div className="truncate text-sm font-semibold text-foreground">
                    {sessionTitle}
                  </div>
                  <div className="mt-0.5 truncate font-mono text-[11px] text-muted-foreground">
                    {currentPage ? displayURL(currentPage) : "-"}
                  </div>
                </div>
              </div>
              <div className="grid grid-cols-2 gap-2">
                <SniffStat label={text.sniffDesk.sessions} value={sessionStatus} />
                <SniffStat
                  label={text.sniffDesk.resources}
                  value={formatTemplate(text.sniffDesk.resourceCount, {
                    count: String(resources.length),
                  })}
                />
                <SniffStat
                  label={text.sniffDesk.downloadableOnly}
                  value={formatTemplate(text.sniffDesk.resourceCount, {
                    count: String(downloadableCount),
                  })}
                />
                <SniffStat
                  label={text.sniffDesk.updatedAt}
                  value={resourcesQuery.data?.updatedAt ? displayTime(resourcesQuery.data.updatedAt) : "-"}
                />
              </div>
            </div>
          ) : null}

          {currentSession ? (
            <div className="app-sniff-desk-session-control grid h-11 w-full grid-cols-[minmax(0,1fr)_auto] items-center gap-3 px-3.5">
              <div className="flex min-w-0 items-center gap-2.5">
                <Activity className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                <span className="truncate text-xs font-semibold text-foreground">
                  {sessionDomain || sessionTitle}
                </span>
                <span className="app-sniff-desk-status-badge shrink-0 px-2 py-0.5 text-[10px] font-semibold">
                  {sessionStatus}
                </span>
              </div>
              <div className="flex shrink-0 items-center gap-1">
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      type="button"
                      size="compactIcon"
                      variant="ghost"
                      className="app-sniff-desk-control-action h-8 w-8"
                      aria-label={text.sniffDesk.details}
                      onClick={() => setDetailsOpen((open) => !open)}
                    >
                      <Info className="h-3.5 w-3.5" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>{text.sniffDesk.details}</TooltipContent>
                </Tooltip>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      type="button"
                      size="compactIcon"
                      variant="ghost"
                      className="app-sniff-desk-control-action app-sniff-desk-control-action-danger h-8 w-8"
                      aria-label={text.actions.closeBrowser}
                      disabled={cancelSniff.isPending || sessionClosing}
                      onClick={() => void handleCancelSession()}
                    >
                      {cancelSniff.isPending || sessionClosing ? (
                        <Loader2 className="h-4 w-4 animate-spin" />
                      ) : (
                        <X className="h-4 w-4" />
                      )}
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>{text.actions.closeBrowser}</TooltipContent>
                </Tooltip>
              </div>
            </div>
          ) : controlMode === "input" ? (
            <form
              className="app-sniff-desk-session-control grid h-11 w-full grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-2.5 px-3.5"
              onSubmit={(event) => {
                event.preventDefault();
                void handleStartSniff();
              }}
            >
              <Globe2 className="h-3.5 w-3.5 text-muted-foreground" />
              <Input
                ref={urlInputRef}
                value={url}
                onChange={(event) => setURL(event.target.value)}
                placeholder={text.sniffDesk.startPlaceholder}
                size="compact"
                className="app-control-input-compact h-auto rounded-none border-0 bg-transparent px-0 shadow-none"
              />
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    type="submit"
                    size="compactIcon"
                    className="app-sniff-desk-control-action app-sniff-desk-control-action-primary h-8 w-8"
                    aria-label={text.sniffDesk.startSniff}
                    disabled={!url.trim() || startSniff.isPending}
                  >
                    {startSniff.isPending ? (
                      <Loader2 className="h-4 w-4 animate-spin" />
                    ) : (
                      <Radar className="h-4 w-4" />
                    )}
                  </Button>
                </TooltipTrigger>
                <TooltipContent>{text.sniffDesk.startSniff}</TooltipContent>
              </Tooltip>
            </form>
          ) : (
            <Button
              type="button"
              variant="default"
              className="app-sniff-desk-start-button app-running-new-download-button h-10 px-4 text-sm font-semibold"
              data-effect={startEffect}
              onClick={() => setControlMode("input")}
            >
              <Radar className="h-4 w-4" />
              {text.sniffDesk.startSniff}
            </Button>
          )}
        </div>
      </div>
    </div>
  );
}
