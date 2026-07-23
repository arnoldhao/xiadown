import { Clipboard } from "@wailsio/runtime";
import {
  AudioLines,
  ChevronDown,
  ClipboardPaste,
  Download,
  FileCog,
  FolderOpen,
  LibraryBig,
  Loader2,
  Pencil,
  Radar,
  Search,
  SlidersHorizontal,
  Video,
  X,
  Zap,
} from "lucide-react";
import * as React from "react";

import { getXiaText } from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
import type { BrowserSourceSelection } from "@/shared/contracts/browserSources";
import type {
  CreateYTDLPJobRequest,
  LibraryMediaInfoDTO,
  ParseYTDLPDownloadResponse,
  PreparedYTDLPDownloadURL,
  PrepareYTDLPDownloadResponse,
  ProbeTranscodeInputRequest,
} from "@/shared/contracts/library";
import type { Settings } from "@/shared/contracts/settings";
import { messageBus } from "@/shared/message";
import {
  useDependencies,
  useDependencyInstallState,
  useDependencyUpdates,
  useInstallDependency,
} from "@/shared/query/dependencies";
import {
  useCreateTranscodeJob,
  useCreateYTDLPBatchJobs,
  useCreateYTDLPJob,
  useCancelResourceSniff,
  useParseYTDLPDownload,
  usePrepareYTDLPDownload,
  useProbeTranscodeInput,
  useStartResourceSniff,
  useTranscodePresets,
  useTranscodePresetsForDownload,
} from "@/shared/query/library";
import { Button } from "@/shared/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogListCard,
  DialogListCardContent,
  DialogRow,
  DialogScrollArea,
  DialogTitle,
} from "@/shared/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from "@/shared/ui/dropdown-menu";
import { DreamSegmentSwitch } from "@/shared/ui/dream-segment-switch";
import { DreamInlineSwitch } from "@/shared/ui/dream-inline-switch";
import { Input } from "@/shared/ui/input";
import { Select } from "@/shared/ui/select";
import { SiteBrandIcon } from "@/shared/ui/site-brand-icon";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/shared/ui/tooltip";
import { openFileDialog } from "@/shared/utils/dialogHelpers";
import {
  extractExtensionFromPath,
  resolveDialogPath,
} from "@/shared/utils/resourceHelpers";

import {
  clampProgress,
  DependencyRepairCard,
} from "@/app/main/dependency-repair-card";
import { NewSniffSourceSteps } from "@/app/main/NewSniffSourceSteps";
import {
  attachSniffWorkspaceStartSession,
  beginSniffWorkspaceStart,
  clearSniffWorkspaceStart,
} from "@/app/sniff-desk/workspace-filters";
import { resolveSniffDeskErrorDescription } from "@/app/sniff-desk/error-prompts";
import {
  AUDIO_FILE_EXTENSIONS,
  VIDEO_FILE_EXTENSIONS,
  formatCodecLabel,
  formatCompletedBitrate,
  formatCompletedDuration,
  formatCompletedFrameRate,
  formatCompletedResolution,
  getAppErrorCode,
  parseAppErrorMessage,
  resolveUnknownErrorMessage,
} from "@/app/main/helpers";
import { TASK_DIALOG_DEPENDENCIES_BY_MODE } from "@/app/main/main-constants";
import {
  applyTranscodePresetSelection,
  buildTranscodeCodecKey,
  filterTranscodePresetsForMediaType,
  formatAudioTrackLabel,
  formatSubtitleLabel,
  inferMediaTypeFromPath,
  pickDefaultFormat,
  resolveResourceSniffStartResolution,
  resolveFormatMediaType,
  alignPreparedDownloadTargets,
  resolveDownloadTargetOrigin,
  resolveOpenFileName,
  resolvePreparedSiteKey,
  resolveTranscodeCodecLabel,
  resolveTranscodeScaleLabel,
  resolveTranscodeScaleValue,
  selectAudioFormatId,
  splitFileNameForDisplay,
  uniqueOptions,
} from "@/app/main/new-task-dialog-helpers";
import type {
  DownloadDialogStep,
  DownloadDialogTab,
  DownloadQuality,
  NewTaskDialogDownloadTarget,
  NewTaskDialogMode,
  NewTaskDialogTranscodeSource,
  SourceMediaType,
} from "@/app/main/types";

type BatchDownloadItemState = {
  id: string;
  url: string;
  domain?: string;
  appSessionId?: string;
  appSessionAvailable?: boolean;
  appSessionCredentialMode?: string;
  useAppSession: boolean;
  quality: DownloadQuality;
  subtitles: boolean;
  transcodePresetId: string;
  deleteSourceFileAfterTranscode: boolean;
  source?: string;
  caller?: string;
};

function downloadAppSessionModeCanExportCookies(mode: string) {
  const normalized = mode.trim().toLowerCase();
  return normalized === "cookies" || normalized === "app_session";
}

function preparedURLToBatchItem(
  item: PreparedYTDLPDownloadURL,
  index: number,
  target?: NewTaskDialogDownloadTarget,
): BatchDownloadItemState {
  const canUseAppSession =
    Boolean(item.appSessionAvailable) &&
    downloadAppSessionModeCanExportCookies(item.appSessionCredentialMode ?? "");
  return {
    id: `${index}-${item.url}`,
    url: item.url,
    domain: item.domain,
    appSessionId: item.appSessionId,
    appSessionAvailable: item.appSessionAvailable,
    appSessionCredentialMode: item.appSessionCredentialMode,
    useAppSession: canUseAppSession,
    quality: "best",
    subtitles: false,
    transcodePresetId: "",
    deleteSourceFileAfterTranscode: true,
    source: target?.source,
    caller: target?.caller,
  };
}

function preparedDownloadFromPlaylistItems(
  current: PrepareYTDLPDownloadResponse,
  items: PreparedYTDLPDownloadURL[],
): PrepareYTDLPDownloadResponse {
  const first = items[0];
  return {
    ...current,
    mode: "batch",
    url: first?.url ?? current.url,
    domain: first?.domain ?? current.domain,
    icon: first?.icon ?? current.icon,
    appSessionId: first?.appSessionId ?? current.appSessionId,
    appSessionAvailable: first?.appSessionAvailable ?? current.appSessionAvailable,
    appSessionCredentialMode:
      first?.appSessionCredentialMode ?? current.appSessionCredentialMode,
    appSessionCredentialState:
      first?.appSessionCredentialState ?? current.appSessionCredentialState,
    reachable: first?.reachable ?? current.reachable,
    urls: items,
  };
}

function batchDownloadItemMediaType(item: BatchDownloadItemState): SourceMediaType {
  return item.quality === "audio" ? "audio" : "video";
}

function batchDownloadItemCanUseAppSession(item: BatchDownloadItemState) {
  return (
    Boolean(item.appSessionId?.trim()) &&
    Boolean(item.appSessionAvailable) &&
    downloadAppSessionModeCanExportCookies(item.appSessionCredentialMode ?? "")
  );
}

function replaceCountToken(template: string, count: number) {
  return template.replace("{count}", String(count));
}

type DownloadQualityLabels = Pick<
  ReturnType<typeof getXiaText>["dialogs"],
  "qualityBest" | "qualityBitrate" | "qualityAudio"
>;

function formatDownloadQualityLabel(
  quality: DownloadQuality,
  labels: DownloadQualityLabels,
) {
  switch (quality) {
    case "audio":
      return labels.qualityAudio;
    case "bitrate":
      return labels.qualityBitrate;
    case "best":
    default:
      return labels.qualityBest;
  }
}

export function InlineSwitch(props: {
  checked: boolean;
  onChange: (checked: boolean) => void;
  ariaLabel: string;
  disabled?: boolean;
}) {
  return (
    <DreamInlineSwitch
      ariaLabel={props.ariaLabel}
      checked={props.checked}
      disabled={props.disabled}
      onCheckedChange={props.onChange}
    />
  );
}

function parseJSONFromErrorMessage(message: string) {
  const trimmed = message.trim();
  if (!trimmed) {
    return null;
  }

  const candidates = [trimmed];
  const objectStart = trimmed.indexOf("{");
  const objectEnd = trimmed.lastIndexOf("}");
  if (objectStart >= 0 && objectEnd > objectStart) {
    candidates.push(trimmed.slice(objectStart, objectEnd + 1));
  }

  const arrayStart = trimmed.indexOf("[");
  const arrayEnd = trimmed.lastIndexOf("]");
  if (arrayStart >= 0 && arrayEnd > arrayStart) {
    candidates.push(trimmed.slice(arrayStart, arrayEnd + 1));
  }

  for (const candidate of candidates) {
    try {
      return JSON.parse(candidate);
    } catch {
      // Try the next JSON-looking candidate.
    }
  }

  return null;
}

function formatParseErrorDetail(message: string) {
  const appError = parseAppErrorMessage(message);
  const detail = appError.message || message;
  const parsed = parseJSONFromErrorMessage(detail);
  if (parsed !== null) {
    return JSON.stringify(parsed, null, 2);
  }
  return detail.trim();
}

function resolveParseErrorDescription(
  message: string,
  fallback: string,
  text: ReturnType<typeof getXiaText>,
) {
  const appError = parseAppErrorMessage(message);
  switch (appError.code) {
    case "resource_verification_required":
      return text.dialogs.resourceVerificationRequired;
    case "resource_no_media_detected":
      return text.dialogs.resourceNoMediaDetected;
    default:
      return fallback;
  }
}

function noDownloadableMediaErrorMessage() {
  return "[resource_no_media_detected] no downloadable formats found";
}

function hasDownloadableFormats(parsed: ParseYTDLPDownloadResponse) {
  return (parsed.formats ?? []).some((format) => format.id.trim().length > 0);
}

function stripLeadingWWW(value: string) {
  return value.trim().replace(/^www\./i, "");
}

function parseURLHostname(value: string) {
  const trimmed = value.trim();
  if (!trimmed) {
    return "";
  }
  const candidates = [
    trimmed,
    trimmed.startsWith("//") ? `https:${trimmed}` : "",
    /^[a-z][a-z0-9+.-]*:\/\//i.test(trimmed) ? "" : `https://${trimmed}`,
  ].filter(Boolean);
  for (const candidate of candidates) {
    try {
      return new URL(candidate).hostname;
    } catch {
      // Keep trying more permissive URL forms.
    }
  }
  return "";
}

function uniqueDisplayCandidates(values: string[]) {
  const seen = new Set<string>();
  return values
    .map((value) => value.trim())
    .filter((value) => {
      const key = value.toLowerCase();
      if (!key || seen.has(key)) {
        return false;
      }
      seen.add(key);
      return true;
    });
}

function buildDownloadUrlDisplayParts(rawUrl: string, domain?: string) {
  const value = rawUrl.trim();
  if (!value) {
    return { prefix: "", domain: "", suffix: "" };
  }

  const preparedDomain = (domain ?? "").trim();
  const parsedHostname = parseURLHostname(value);
  const candidates = uniqueDisplayCandidates([
    stripLeadingWWW(preparedDomain),
    preparedDomain,
    stripLeadingWWW(parsedHostname),
    parsedHostname,
  ]);
  const lowerValue = value.toLowerCase();

  for (const candidate of candidates) {
    const index = lowerValue.indexOf(candidate.toLowerCase());
    if (index >= 0) {
      return {
        prefix: value.slice(0, index),
        domain: value.slice(index, index + candidate.length),
        suffix: value.slice(index + candidate.length),
      };
    }
  }

  return {
    prefix: "",
    domain: stripLeadingWWW(preparedDomain || parsedHostname) || value,
    suffix: "",
  };
}

const TRANSCODE_VIDEO_EXTENSIONS = [...VIDEO_FILE_EXTENSIONS].sort();
const TRANSCODE_AUDIO_EXTENSIONS = [...AUDIO_FILE_EXTENSIONS].sort();
const TRANSCODE_INPUT_EXTENSIONS = new Set([
  ...TRANSCODE_VIDEO_EXTENSIONS,
  ...TRANSCODE_AUDIO_EXTENSIONS,
]);

function buildDialogPattern(extensions: string[]) {
  return extensions.map((extension) => `*.${extension}`).join(";");
}

function isSupportedTranscodeInputPath(path: string) {
  const extension = extractExtensionFromPath(path);
  return Boolean(extension && TRANSCODE_INPUT_EXTENSIONS.has(extension));
}

function normalizeProbeMediaType(value?: string): SourceMediaType | null {
  const normalized = (value ?? "").trim().toLowerCase();
  return normalized === "video" || normalized === "audio" ? normalized : null;
}

function resolveTranscodeProbeSummary(media?: LibraryMediaInfoDTO | null) {
  if (!media) {
    return "";
  }
  const codecSummary = [
    media.videoCodec ? formatCodecLabel(media.videoCodec) : "",
    media.audioCodec ? formatCodecLabel(media.audioCodec) : "",
  ]
    .filter(Boolean)
    .join(" / ");
  const parts = [
    media.format?.trim().toUpperCase() ?? "",
    codecSummary || (media.codec ? formatCodecLabel(media.codec) : ""),
    formatCompletedResolution(media.width, media.height),
    formatCompletedFrameRate(media.frameRate),
    formatCompletedBitrate(
      media.bitrateKbps ?? media.videoBitrateKbps ?? media.audioBitrateKbps,
    ),
    formatCompletedDuration(media.durationMs),
  ].filter(Boolean);
  return parts.join(" · ");
}

export function NewTaskDialog(props: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  initialMode?: NewTaskDialogMode;
  initialUrl?: string;
  initialDownloadSource?: string;
  initialDownloadCaller?: string;
  initialDownloadTargets?: readonly NewTaskDialogDownloadTarget[];
  initialTranscodeSource?: NewTaskDialogTranscodeSource | null;
  transcodeLibrarySources?: readonly NewTaskDialogTranscodeSource[];
  transcodeLibraryLoading?: boolean;
  transcodeLibraryError?: string;
  onRetryTranscodeLibrary?: () => void;
  settings: Settings | null;
  onOpenConnections?: () => void;
  onOpenSniffDesk?: () => void;
}) {
  const text = getXiaText(props.settings?.language);
  const tools = useDependencies({
    refetchInterval: props.open ? 1_500 : false,
  });
  const dependencyUpdates = useDependencyUpdates();
  const installDependency = useInstallDependency();
  const ytdlpInstallState = useDependencyInstallState("yt-dlp", props.open);
  const ffmpegInstallState = useDependencyInstallState("ffmpeg", props.open);
  const prepareDownload = usePrepareYTDLPDownload();
  const parseDownload = useParseYTDLPDownload();
  const startResourceSniff = useStartResourceSniff();
  const cancelResourceSniff = useCancelResourceSniff();
  const createYTDLP = useCreateYTDLPJob();
  const createYTDLPBatch = useCreateYTDLPBatchJobs();
  const presetsQuery = useTranscodePresets();
  const createTranscode = useCreateTranscodeJob();
  const [activeMode, setActiveMode] = React.useState<NewTaskDialogMode>(
    props.initialMode ?? "download",
  );
  const [downloadStep, setDownloadStep] =
    React.useState<DownloadDialogStep>("input");
  const [downloadUrl, setDownloadUrl] = React.useState(props.initialUrl ?? "");
  const [downloadPrepared, setDownloadPrepared] =
    React.useState<PrepareYTDLPDownloadResponse | null>(null);
  const [batchDownloadItems, setBatchDownloadItems] = React.useState<
    BatchDownloadItemState[]
  >([]);
  const [downloadUseAppSession, setDownloadUseAppSession] = React.useState(false);
  const [downloadTab, setDownloadTab] =
    React.useState<DownloadDialogTab>("quick");
  const [downloadPrepareError, setDownloadPrepareError] = React.useState("");
  const [downloadSubmitError, setDownloadSubmitError] = React.useState("");
  const [quickQuality, setQuickQuality] =
    React.useState<DownloadQuality>("best");
  const [quickSubtitle, setQuickSubtitle] = React.useState(false);
  const [quickPresetId, setQuickPresetId] = React.useState("");
  const [downloadKeepOnlyTranscodedFile, setDownloadKeepOnlyTranscodedFile] =
    React.useState(true);
  const [customParseResult, setCustomParseResult] =
    React.useState<ParseYTDLPDownloadResponse | null>(null);
  const [customFormatId, setCustomFormatId] = React.useState("");
  const [customAudioFormatId, setCustomAudioFormatId] = React.useState("");
  const [customSubtitleId, setCustomSubtitleId] = React.useState("");
  const [customPresetId, setCustomPresetId] = React.useState("");
  const [customParseError, setCustomParseError] = React.useState("");
  const [resourceSniffSessionId, setResourceSniffSessionId] =
    React.useState("");
  const [transcodeInputPath, setTranscodeInputPath] = React.useState("");
  const [transcodeSourceFileId, setTranscodeSourceFileId] = React.useState("");
  const [transcodeSourceTitle, setTranscodeSourceTitle] = React.useState("");
  const [transcodeSourceAuthor, setTranscodeSourceAuthor] = React.useState("");
  const [transcodePresetId, setTranscodePresetId] = React.useState("");
  const [transcodeScale, setTranscodeScale] = React.useState("");
  const [transcodeContainer, setTranscodeContainer] = React.useState("");
  const [transcodeCodec, setTranscodeCodec] = React.useState("");
  const [transcodeSubmitError, setTranscodeSubmitError] = React.useState("");
  const [transcodeLibraryOpen, setTranscodeLibraryOpen] = React.useState(false);
  const [transcodeLibraryQuery, setTranscodeLibraryQuery] = React.useState("");
  const [closingDialog, setClosingDialog] = React.useState(false);
  const autoPreparedInitialUrlRef = React.useRef("");
  const dialogOpenRef = React.useRef(props.open);
  const dialogClosingRef = React.useRef(false);
  const preserveResourceSniffOnCloseRef = React.useRef(false);
  const parseRequestVersionRef = React.useRef(0);
  const resourceSniffStartVersionRef = React.useRef(0);
  const resourceSniffConfirmingRef = React.useRef(false);
  const resourceSniffTransferStartVersionRef = React.useRef<number | null>(null);
  const resourceSniffSessionIdRef = React.useRef("");
  const resourceSniffStartPromiseRef = React.useRef<Promise<string> | null>(
    null,
  );
  const cancelResourceSniffRef = React.useRef(cancelResourceSniff);

  React.useEffect(() => {
    dialogOpenRef.current = props.open;
    if (props.open) {
      preserveResourceSniffOnCloseRef.current = false;
    }
  }, [props.open]);

  React.useEffect(() => {
    resourceSniffSessionIdRef.current = resourceSniffSessionId;
  }, [resourceSniffSessionId]);

  React.useEffect(() => {
    cancelResourceSniffRef.current = cancelResourceSniff;
  }, [cancelResourceSniff]);

  const setActiveResourceSniffSessionId = React.useCallback(
    (sessionId: string) => {
      const trimmed = sessionId.trim();
      resourceSniffSessionIdRef.current = trimmed;
      setResourceSniffSessionId(trimmed);
    },
    [],
  );

  const cancelResourceSniffSession = React.useCallback(
    async (sessionId: string) => {
      const trimmed = sessionId.trim();
      if (!trimmed) {
        return false;
      }
      if (resourceSniffSessionIdRef.current === trimmed) {
        setActiveResourceSniffSessionId("");
      }
      try {
        await cancelResourceSniffRef.current.mutateAsync({ sessionId: trimmed });
        return true;
      } catch {
        return false;
      }
    },
    [setActiveResourceSniffSessionId],
  );

  const cancelActiveResourceSniff = React.useCallback(async () => {
    const sessionId = resourceSniffSessionIdRef.current;
    if (!sessionId) {
      return false;
    }
    return cancelResourceSniffSession(sessionId);
  }, [cancelResourceSniffSession]);

  const closeDialogAfterResourceCleanup = React.useCallback(async () => {
    if (dialogClosingRef.current) {
      return;
    }
    dialogClosingRef.current = true;
    setClosingDialog(true);
    try {
      parseRequestVersionRef.current += 1;
      resourceSniffStartVersionRef.current += 1;
      dialogOpenRef.current = false;
      void cancelActiveResourceSniff();
      props.onOpenChange(false);
    } finally {
      dialogClosingRef.current = false;
      setClosingDialog(false);
    }
  }, [cancelActiveResourceSniff, props.onOpenChange]);

  const handleDialogOpenChange = React.useCallback(
    (open: boolean) => {
      if (!open) {
        void closeDialogAfterResourceCleanup();
        return;
      }
      dialogOpenRef.current = true;
      props.onOpenChange(open);
    },
    [closeDialogAfterResourceCleanup, props.onOpenChange],
  );

  const toolsByName = React.useMemo(
    () => new Map((tools.data ?? []).map((item) => [item.name, item])),
    [tools.data],
  );
  const updatesByName = React.useMemo(
    () =>
      new Map((dependencyUpdates.data ?? []).map((item) => [item.name, item])),
    [dependencyUpdates.data],
  );
  const installStatesByName = React.useMemo(
    () =>
      new Map([
        ["yt-dlp", ytdlpInstallState.data],
        ["ffmpeg", ffmpegInstallState.data],
      ]),
    [ffmpegInstallState.data, ytdlpInstallState.data],
  );
  const taskDependencies = TASK_DIALOG_DEPENDENCIES_BY_MODE[activeMode];
  const installStagesByName = React.useMemo(
    () =>
      new Map<string, string>(
        taskDependencies.map((name) => [
          name,
          (installStatesByName.get(name)?.stage ?? "idle").toString(),
        ] as [string, string]),
      ),
    [installStatesByName, taskDependencies],
  );
  const installProgressByName = React.useMemo(
    () =>
      new Map<string, number>(
        taskDependencies.map((name) => [
          name,
          clampProgress(installStatesByName.get(name)?.progress),
        ] as [string, number]),
      ),
    [installStatesByName, taskDependencies],
  );
  const ytdlpInstalled =
    (toolsByName.get("yt-dlp")?.status ?? "").trim().toLowerCase() ===
    "installed";
  const ffmpegInstalled =
    (toolsByName.get("ffmpeg")?.status ?? "").trim().toLowerCase() ===
    "installed";
  const taskDependenciesReady = taskDependencies.every(
    (name) =>
      (toolsByName.get(name)?.status ?? "").trim().toLowerCase() ===
      "installed",
  );
  const quickMediaType: SourceMediaType =
    quickQuality === "audio" ? "audio" : "video";
  const quickPresetsQuery = useTranscodePresetsForDownload({
    mediaType: quickMediaType,
  });
  const customFormats = customParseResult?.formats ?? [];
  const customVideoFormats = customFormats.filter((format) => format.hasVideo);
  const customAudioFormats = customFormats.filter(
    (format) => format.hasAudio && !format.hasVideo,
  );
  const customSubtitles = customParseResult?.subtitles ?? [];
  const customSelectedFormat =
    customFormats.find((format) => format.id === customFormatId) ?? null;
  const customCanSelectAudioTrack = Boolean(
    customSelectedFormat?.hasVideo &&
      !customSelectedFormat.hasAudio &&
      customAudioFormats.length > 0,
  );
  const customSelectedSubtitle =
    customSubtitles.find((subtitle) => subtitle.id === customSubtitleId) ??
    null;
  const customMediaType = customSelectedFormat
    ? resolveFormatMediaType(customSelectedFormat)
    : null;
  const customPresetsQuery = useTranscodePresetsForDownload(
    customMediaType ? { mediaType: customMediaType } : null,
  );
  const customParseErrorDetail = React.useMemo(
    () => formatParseErrorDetail(customParseError),
    [customParseError],
  );
  const customParseAppError = React.useMemo(
    () => parseAppErrorMessage(customParseError),
    [customParseError],
  );
  const downloadIsBatch =
    downloadPrepared?.mode === "batch" || batchDownloadItems.length > 1;
  const batchDownloadCountLabel = replaceCountToken(
    text.dialogs.batchDownloadCount,
    batchDownloadItems.length,
  );
  const preparedDownloadUrl = (downloadPrepared?.url ?? downloadUrl).trim();
  const downloadUrlDisplayParts = buildDownloadUrlDisplayParts(
    preparedDownloadUrl,
    downloadPrepared?.domain,
  );
  const downloadSiteKey = resolvePreparedSiteKey(downloadPrepared);
  const downloadAppSessionMode = (
    downloadPrepared?.appSessionCredentialMode ?? ""
  )
    .trim()
    .toLowerCase();
  const downloadMatchesCookieExportAppSession =
    Boolean(downloadPrepared?.appSessionId?.trim()) &&
    downloadAppSessionModeCanExportCookies(downloadAppSessionMode);
  const downloadCookieExportAppSessionState = !downloadMatchesCookieExportAppSession
    ? "unmatched"
    : downloadPrepared?.appSessionAvailable
      ? "available"
      : "unavailable";
  const downloadAppSessionCanExportCookies =
    downloadCookieExportAppSessionState === "available";
  const downloadAppSessionStatusLabel =
    downloadCookieExportAppSessionState === "available"
      ? text.dialogs.appSessionCanEnable
      : downloadCookieExportAppSessionState === "unavailable"
        ? text.dialogs.appSessionNotConfigured
        : text.dialogs.noAvailableAppSession;
  const downloadTabItems: Array<{
    value: DownloadDialogTab;
    label: string;
    icon: React.ReactNode;
  }> = [
    {
      value: "quick",
      label: text.dialogs.quickMode,
      icon: <Zap className="h-3.5 w-3.5" />,
    },
    {
      value: "custom",
      label: text.dialogs.customMode,
      icon: <SlidersHorizontal className="h-3.5 w-3.5" />,
    },
  ];
  const batchDownloadTabItems = [
    {
      value: "batch",
      label: `${text.dialogs.batchDownload} · ${batchDownloadItems.length}`,
      icon: <Download className="h-3.5 w-3.5" />,
      tooltip: batchDownloadCountLabel,
    },
  ] as const;
  const activeDownloadMode = activeMode === "download";
  const activeDownloadActionLabel = text.dialogs.directDownload;
  const customParseErrorDescription =
    resolveParseErrorDescription(
      customParseError,
      downloadUseAppSession && downloadAppSessionCanExportCookies
        ? text.dialogs.parseFailedWithAppSession
        : text.dialogs.parseFailedWithoutAppSession,
      text,
    );
  const customParseErrorLine = customParseAppError.code
    ? customParseErrorDescription
    : customParseErrorDetail || customParseErrorDescription;
  const transcodeMediaType = React.useMemo(
    () => inferMediaTypeFromPath(transcodeInputPath),
    [transcodeInputPath],
  );
  const transcodeProbeRequest =
    React.useMemo<ProbeTranscodeInputRequest | null>(() => {
      const inputPath = transcodeInputPath.trim();
      if (activeMode !== "transcode" || !inputPath || !ffmpegInstalled) {
        return null;
      }
      return {
        fileId: transcodeSourceFileId || undefined,
        inputPath: transcodeSourceFileId ? undefined : inputPath,
        source: "xiadown.transcode.dialog",
      };
    }, [activeMode, ffmpegInstalled, transcodeInputPath, transcodeSourceFileId]);
  const transcodeProbeQuery = useProbeTranscodeInput(transcodeProbeRequest);
  const transcodeProbe = transcodeProbeQuery.data ?? null;
  const probedTranscodeMediaType = normalizeProbeMediaType(
    transcodeProbe?.mediaType,
  );
  const effectiveTranscodeMediaType =
    probedTranscodeMediaType ?? transcodeMediaType;
  const compatibleTranscodePresetIds = React.useMemo(
    () => new Set(transcodeProbe?.compatiblePresetIds ?? []),
    [transcodeProbe?.compatiblePresetIds],
  );
  const transcodeProbeReady =
    Boolean(transcodeProbe) && !transcodeProbeQuery.isError;
  const transcodePresets = React.useMemo(
    () => {
      if (transcodeInputPath.trim() && !transcodeProbeReady) {
        return [];
      }
      const presets = filterTranscodePresetsForMediaType(
        presetsQuery.data ?? [],
        effectiveTranscodeMediaType,
      );
      if (!transcodeProbe) {
        return presets;
      }
      return presets.filter((preset) =>
        compatibleTranscodePresetIds.has(preset.id),
      );
    },
    [
      compatibleTranscodePresetIds,
      effectiveTranscodeMediaType,
      presetsQuery.data,
      transcodeInputPath,
      transcodeProbe,
      transcodeProbeReady,
    ],
  );
  const transcodeProbeSummary = resolveTranscodeProbeSummary(
    transcodeProbe?.media,
  );
  const transcodeProbeError = transcodeProbeQuery.isError
    ? resolveUnknownErrorMessage(transcodeProbeQuery.error, text.common.unknown)
    : "";
  const transcodeProbeChecking =
    Boolean(transcodeProbeRequest) &&
    transcodeProbeQuery.isFetching &&
    !transcodeProbe;
  const transcodePresetCatalogReady =
    presetsQuery.isFetched || Boolean(presetsQuery.data);
  const showTranscodeOptions =
    activeMode === "transcode" &&
    Boolean(transcodeInputPath) &&
    transcodeProbeReady &&
    transcodePresets.length > 0;
  const transcodeNoCompatiblePreset =
    activeMode === "transcode" &&
    Boolean(transcodeInputPath) &&
    transcodeProbeReady &&
    transcodePresetCatalogReady &&
    transcodePresets.length === 0;
  const transcodeProbeStatusLabel = transcodeProbeChecking
    ? text.dialogs.inspectingFile
    : transcodeProbeError
      ? text.dialogs.fileInspectFailed
      : transcodeNoCompatiblePreset
        ? text.dialogs.noCompatibleTranscodePreset
        : "";
  const transcodeSizeOptions = React.useMemo(
    () =>
      uniqueOptions(
        transcodePresets.map((preset) => {
          const value = resolveTranscodeScaleValue(preset);
          return {
            value,
            label: resolveTranscodeScaleLabel(value, text),
          };
        }),
      ),
    [text, transcodePresets],
  );
  const transcodeContainerOptions = React.useMemo(
    () =>
      uniqueOptions(
        transcodePresets
          .filter(
            (preset) => resolveTranscodeScaleValue(preset) === transcodeScale,
          )
          .map((preset) => ({
            value: preset.container,
            label: preset.container.toUpperCase(),
          })),
      ),
    [transcodePresets, transcodeScale],
  );
  const transcodeCodecOptions = React.useMemo(
    () =>
      uniqueOptions(
        transcodePresets
          .filter(
            (preset) =>
              resolveTranscodeScaleValue(preset) === transcodeScale &&
              preset.container === transcodeContainer,
          )
          .map((preset) => ({
            value: buildTranscodeCodecKey(preset),
            label: resolveTranscodeCodecLabel(preset),
          })),
      ),
    [transcodeContainer, transcodePresets, transcodeScale],
  );
  const selectedTranscodePreset = React.useMemo(
    () =>
      transcodePresets.find(
        (preset) =>
          resolveTranscodeScaleValue(preset) === transcodeScale &&
          preset.container === transcodeContainer &&
          buildTranscodeCodecKey(preset) === transcodeCodec,
      ) ?? null,
    [transcodeCodec, transcodeContainer, transcodePresets, transcodeScale],
  );
  const transcodeFileName = splitFileNameForDisplay(transcodeInputPath);
  const transcodeLibraryGroups = React.useMemo(() => {
    const query = transcodeLibraryQuery.trim().toLocaleLowerCase();
    const grouped = new Map<string, NewTaskDialogTranscodeSource[]>();
    for (const source of props.transcodeLibrarySources ?? []) {
      if (!source.fileId?.trim()) {
        continue;
      }
      const searchText = [
        source.title,
        source.author,
        source.libraryName,
        source.format,
        source.displayLabel,
      ]
        .filter(Boolean)
        .join(" ")
        .toLocaleLowerCase();
      if (query && !searchText.includes(query)) {
        continue;
      }
      const group = source.libraryName?.trim() || text.workspace.libraryStation;
      const items = grouped.get(group) ?? [];
      items.push(source);
      grouped.set(group, items);
    }
    return [...grouped.entries()]
      .sort(([left], [right]) => left.localeCompare(right, text.locale))
      .map(([name, items]) => ({
        name,
        items: items.sort((left, right) =>
          (left.title || left.displayLabel || "").localeCompare(
            right.title || right.displayLabel || "",
            text.locale,
          ),
        ),
      }));
  }, [props.transcodeLibrarySources, text.locale, text.workspace.libraryStation, transcodeLibraryQuery]);
  const showDownloadFooter =
    downloadStep === "config" &&
    (downloadIsBatch ||
      downloadTab === "quick" ||
      (downloadTab === "custom" && Boolean(customParseResult)));
  const showTranscodeFooter =
    activeMode === "transcode" && Boolean(transcodeInputPath);

  const applyTranscodeInputPath = (
    path: string,
    source?: NewTaskDialogTranscodeSource | null,
  ) => {
    setTranscodeInputPath(path);
    setTranscodeSourceFileId(source?.fileId?.trim() ?? "");
    setTranscodeSourceTitle(source?.title?.trim() ?? "");
    setTranscodeSourceAuthor(source?.author?.trim() ?? "");
    setTranscodePresetId("");
    applyTranscodePresetSelection(null, {
      setScale: setTranscodeScale,
      setContainer: setTranscodeContainer,
      setCodec: setTranscodeCodec,
    });
    setTranscodeSubmitError("");
  };

  React.useEffect(() => {
    if (!props.open) {
      return;
    }
    const initialMode = props.initialMode ?? "download";
    setActiveMode(initialMode);
    setDownloadStep("input");
    setDownloadUrl(props.initialUrl ?? "");
    setDownloadPrepared(null);
    setBatchDownloadItems([]);
    setDownloadUseAppSession(false);
    setDownloadTab("quick");
    setDownloadPrepareError("");
    setDownloadSubmitError("");
    setQuickQuality("best");
    setQuickSubtitle(false);
    setQuickPresetId("");
    setDownloadKeepOnlyTranscodedFile(true);
    setCustomParseResult(null);
    setCustomFormatId("");
    setCustomAudioFormatId("");
    setCustomSubtitleId("");
    setCustomPresetId("");
    setCustomParseError("");
    parseRequestVersionRef.current += 1;
    parseDownload.reset();
    setActiveResourceSniffSessionId("");
    if (
      initialMode === "transcode" &&
      (props.initialTranscodeSource?.inputPath?.trim() ||
        props.initialTranscodeSource?.fileId?.trim())
    ) {
      applyTranscodeInputPath(
        props.initialTranscodeSource.displayLabel?.trim() ||
          props.initialTranscodeSource.inputPath?.trim() ||
          props.initialTranscodeSource.title?.trim() ||
          props.initialTranscodeSource.fileId?.trim() ||
          "",
        props.initialTranscodeSource,
      );
    } else {
      setTranscodeInputPath("");
      setTranscodeSourceFileId("");
      setTranscodeSourceTitle("");
      setTranscodeSourceAuthor("");
      setTranscodePresetId("");
      setTranscodeScale("");
      setTranscodeContainer("");
      setTranscodeCodec("");
    }
    setTranscodeLibraryOpen(false);
    setTranscodeLibraryQuery("");
    setTranscodeSubmitError("");
    autoPreparedInitialUrlRef.current = "";
  }, [
    props.initialMode,
    props.initialTranscodeSource?.author,
    props.initialTranscodeSource?.fileId,
    props.initialTranscodeSource?.inputPath,
    props.initialTranscodeSource?.displayLabel,
    props.initialTranscodeSource?.title,
    props.initialUrl,
    props.open,
  ]);

  React.useEffect(() => {
    if (props.open) {
      return;
    }
    parseRequestVersionRef.current += 1;
    resourceSniffStartVersionRef.current += 1;
    parseDownload.reset();
    if (preserveResourceSniffOnCloseRef.current) {
      setActiveResourceSniffSessionId("");
      return;
    }
    void cancelActiveResourceSniff();
  }, [cancelActiveResourceSniff, props.open, setActiveResourceSniffSessionId]);

  React.useEffect(
    () => () => {
      if (preserveResourceSniffOnCloseRef.current) {
        return;
      }
      void cancelActiveResourceSniff();
    },
    [cancelActiveResourceSniff],
  );

  React.useEffect(() => {
    if (!quickPresetId) {
      return;
    }
    if (
      !(quickPresetsQuery.data ?? []).some(
        (preset) => preset.id === quickPresetId,
      )
    ) {
      setQuickPresetId("");
    }
  }, [quickPresetId, quickPresetsQuery.data]);

  React.useEffect(() => {
    if (!customPresetId) {
      return;
    }
    if (
      !(customPresetsQuery.data ?? []).some(
        (preset) => preset.id === customPresetId,
      )
    ) {
      setCustomPresetId("");
    }
  }, [customPresetId, customPresetsQuery.data]);

  React.useEffect(() => {
    if (!customParseResult) {
      if (customAudioFormatId) {
        setCustomAudioFormatId("");
      }
      return;
    }
    const selectedFormat =
      customParseResult.formats.find((format) => format.id === customFormatId) ??
      null;
    const audioFormats = customParseResult.formats.filter(
      (format) => format.hasAudio && !format.hasVideo,
    );
    if (
      !selectedFormat?.hasVideo ||
      selectedFormat.hasAudio ||
      audioFormats.length === 0
    ) {
      if (customAudioFormatId) {
        setCustomAudioFormatId("");
      }
      return;
    }
    if (audioFormats.some((format) => format.id === customAudioFormatId)) {
      return;
    }
    setCustomAudioFormatId(
      selectAudioFormatId(customParseResult.formats) || audioFormats[0]?.id || "",
    );
  }, [customAudioFormatId, customFormatId, customParseResult]);

  React.useEffect(() => {
    if (
      !transcodeInputPath ||
      !transcodeProbeReady ||
      transcodePresets.length === 0 ||
      selectedTranscodePreset
    ) {
      return;
    }
    const recommendedPreset =
      transcodePresets.find(
        (preset) => preset.id === transcodeProbe?.recommendedPresetId,
      ) ?? transcodePresets[0];
    applyTranscodePresetSelection(recommendedPreset, {
      setScale: setTranscodeScale,
      setContainer: setTranscodeContainer,
      setCodec: setTranscodeCodec,
    });
  }, [
    selectedTranscodePreset,
    transcodeInputPath,
    transcodePresets,
    transcodeProbe?.recommendedPresetId,
    transcodeProbeReady,
  ]);

  React.useEffect(() => {
    if (!transcodeInputPath) {
      return;
    }
    if (
      transcodeScale &&
      transcodeSizeOptions.some((option) => option.value === transcodeScale)
    ) {
      return;
    }
    setTranscodeScale(transcodeSizeOptions[0]?.value ?? "");
  }, [transcodeInputPath, transcodeScale, transcodeSizeOptions]);

  React.useEffect(() => {
    if (!transcodeInputPath) {
      return;
    }
    if (
      transcodeContainer &&
      transcodeContainerOptions.some(
        (option) => option.value === transcodeContainer,
      )
    ) {
      return;
    }
    setTranscodeContainer(transcodeContainerOptions[0]?.value ?? "");
  }, [transcodeContainer, transcodeContainerOptions, transcodeInputPath]);

  React.useEffect(() => {
    if (!transcodeInputPath) {
      return;
    }
    if (
      transcodeCodec &&
      transcodeCodecOptions.some((option) => option.value === transcodeCodec)
    ) {
      return;
    }
    setTranscodeCodec(transcodeCodecOptions[0]?.value ?? "");
  }, [transcodeCodec, transcodeCodecOptions, transcodeInputPath]);

  React.useEffect(() => {
    setTranscodePresetId(selectedTranscodePreset?.id ?? "");
  }, [selectedTranscodePreset?.id]);

  const resetDownloadConfig = () => {
    parseRequestVersionRef.current += 1;
    parseDownload.reset();
    setDownloadPrepared(null);
    setBatchDownloadItems([]);
    setDownloadStep("input");
    setDownloadUseAppSession(false);
    setDownloadTab("quick");
    setDownloadSubmitError("");
    setDownloadKeepOnlyTranscodedFile(true);
    setCustomParseResult(null);
    setCustomFormatId("");
    setCustomAudioFormatId("");
    setCustomSubtitleId("");
    setCustomPresetId("");
    setCustomParseError("");
  };

  const resetParsedDownloadSelection = () => {
    parseRequestVersionRef.current += 1;
    parseDownload.reset();
    setCustomParseResult(null);
    setCustomFormatId("");
    setCustomAudioFormatId("");
    setCustomSubtitleId("");
    setCustomPresetId("");
    setCustomParseError("");
  };

  const transferResourceSniffToDesk = React.useCallback(
    (requestVersion: number) => {
      if (!props.onOpenSniffDesk) {
        return;
      }
      resourceSniffTransferStartVersionRef.current = requestVersion;
      preserveResourceSniffOnCloseRef.current = true;
      parseRequestVersionRef.current += 1;
      setActiveResourceSniffSessionId("");
      dialogOpenRef.current = false;
      props.onOpenChange(false);
      props.onOpenSniffDesk();
    },
    [
      props.onOpenChange,
      props.onOpenSniffDesk,
      setActiveResourceSniffSessionId,
    ],
  );

  const handleOpenSniffDesk = React.useCallback(() => {
    transferResourceSniffToDesk(resourceSniffStartVersionRef.current);
  }, [transferResourceSniffToDesk]);

  const startResourceSniffFromSelection = React.useCallback(
    async (
      selection: BrowserSourceSelection,
      options?: { transferToDesk?: boolean },
    ) => {
      const existingSessionID = resourceSniffSessionIdRef.current.trim();
      if (existingSessionID) {
        await cancelResourceSniffSession(existingSessionID);
      }
      resetParsedDownloadSelection();
      const startVersion = ++resourceSniffStartVersionRef.current;
      const handoffRequestId = options?.transferToDesk
        ? beginSniffWorkspaceStart()
        : "";
      const currentBrowserMode = selection.mode === "current_browser";
      const startMutationPromise = startResourceSniff.mutateAsync({
        url: "",
        mode: currentBrowserMode ? "current_browser" : "managed_profile",
        browserId: selection.browserId,
        ...(currentBrowserMode ? {} : { profileId: selection.profileId }),
      });
      const startPromise = startMutationPromise.then(
        (result) => result.session?.sessionId ?? "",
      );
      resourceSniffStartPromiseRef.current = startPromise;
      if (options?.transferToDesk) {
        transferResourceSniffToDesk(startVersion);
      }
      try {
        const result = await startMutationPromise;
        const sessionId = result.session?.sessionId.trim() ?? "";
        if (!sessionId) {
          throw new Error("resource sniff start returned no session");
        }
        const startResolution = resolveResourceSniffStartResolution({
          requestVersion: startVersion,
          currentVersion: resourceSniffStartVersionRef.current,
          dialogOpen: dialogOpenRef.current,
          transferRequestVersion: resourceSniffTransferStartVersionRef.current,
        });
        if (startResolution === "preserve") {
          resourceSniffTransferStartVersionRef.current = null;
          attachSniffWorkspaceStartSession(handoffRequestId, sessionId);
          return;
        }
        if (startResolution === "cancel") {
          clearSniffWorkspaceStart(handoffRequestId);
          await cancelResourceSniffSession(sessionId);
          return;
        }
        setActiveResourceSniffSessionId(sessionId);
      } catch (error) {
        if (
          resolveResourceSniffStartResolution({
            requestVersion: startVersion,
            currentVersion: resourceSniffStartVersionRef.current,
            dialogOpen: dialogOpenRef.current,
            transferRequestVersion:
              resourceSniffTransferStartVersionRef.current,
          }) === "preserve"
        ) {
          resourceSniffTransferStartVersionRef.current = null;
          clearSniffWorkspaceStart(handoffRequestId);
          messageBus.publishToast({
            intent: "danger",
            title: text.sniffDesk.startFailed,
            description: resolveSniffDeskErrorDescription(text, error),
          });
          return;
        }
        if (!dialogOpenRef.current && !preserveResourceSniffOnCloseRef.current) {
          return;
        }
        messageBus.publishToast({
          intent: "danger",
          title: text.sniffDesk.startFailed,
          description: resolveSniffDeskErrorDescription(text, error),
        });
      } finally {
        if (resourceSniffStartPromiseRef.current === startPromise) {
          resourceSniffStartPromiseRef.current = null;
        }
      }
    },
    [
      cancelResourceSniffSession,
      parseDownload,
      startResourceSniff,
      text,
      transferResourceSniffToDesk,
    ],
  );

  const handleConfirmResourceSniffSource = React.useCallback(
    async (selection: BrowserSourceSelection) => {
      const supportedSelection = selection.mode === "current_browser"
        ? selection.browserId === "chrome"
        : selection.mode === "xiadown_profile" &&
          selection.browserId.trim().length > 0;
      if (!supportedSelection || resourceSniffConfirmingRef.current) {
        return;
      }
      resourceSniffConfirmingRef.current = true;
      try {
        await startResourceSniffFromSelection(selection, {
          transferToDesk: Boolean(props.onOpenSniffDesk),
        });
      } finally {
        resourceSniffConfirmingRef.current = false;
      }
    },
    [props.onOpenSniffDesk, startResourceSniffFromSelection],
  );

  const resolveDownloadErrorMessage = React.useCallback(
    (error: unknown) => {
      switch (getAppErrorCode(error)) {
        case "download_url_required":
          return text.dialogs.downloadUrlRequired;
        case "download_url_invalid":
          return text.dialogs.downloadUrlInvalid;
        case "download_url_unsupported":
          return text.dialogs.downloadUrlUnsupported;
        case "download_url_multiple":
          return text.dialogs.downloadUrlMultiple;
        case "download_batch_empty":
          return text.dialogs.downloadBatchEmpty;
        case "download_batch_too_large":
          return text.dialogs.downloadBatchTooLarge;
        default:
          return resolveUnknownErrorMessage(error, text.common.unknown);
      }
    },
    [
      text.common.unknown,
      text.dialogs.downloadBatchEmpty,
      text.dialogs.downloadBatchTooLarge,
      text.dialogs.downloadUrlInvalid,
      text.dialogs.downloadUrlMultiple,
      text.dialogs.downloadUrlRequired,
      text.dialogs.downloadUrlUnsupported,
    ],
  );

  const handlePasteDownloadURL = React.useCallback(async () => {
    try {
      let value = "";
      try {
        value = await Clipboard.Text();
      } catch {
        value = (await navigator.clipboard?.readText?.()) ?? "";
      }
      if (!value.trim()) {
        setDownloadPrepareError(text.dialogs.clipboardEmpty);
        return;
      }
      setDownloadUrl(value.trim());
      setDownloadPrepareError("");
    } catch {
      setDownloadPrepareError(text.dialogs.clipboardUnavailable);
    }
  }, [text.dialogs.clipboardEmpty, text.dialogs.clipboardUnavailable]);

  const handlePrepareDownload = React.useCallback(
    async (overrideUrl?: string) => {
      const url = (overrideUrl ?? downloadUrl).trim();
      if (!url) {
        return;
      }
      setDownloadPrepareError("");
      setDownloadSubmitError("");
      try {
        const prepared = await prepareDownload.mutateAsync({ url });
        const preparedURLs =
          prepared.urls && prepared.urls.length > 0
            ? prepared.urls
            : prepared.url
              ? [
                  {
                    url: prepared.url,
                    domain: prepared.domain,
                    appSessionId: prepared.appSessionId,
                    appSessionAvailable: prepared.appSessionAvailable,
                    appSessionCredentialMode:
                      prepared.appSessionCredentialMode,
                    appSessionCredentialState:
                      prepared.appSessionCredentialState,
                  },
                ]
              : [];
        if (prepared.mode === "batch" || preparedURLs.length > 1) {
          const alignedTargets = alignPreparedDownloadTargets(
            preparedURLs,
            props.initialDownloadTargets,
          );
          const batchItems = preparedURLs.map((item, index) =>
            preparedURLToBatchItem(item, index, alignedTargets[index]),
          );
          setActiveMode("download");
          setDownloadPrepared(prepared);
          setBatchDownloadItems(batchItems);
          setDownloadUrl(batchItems.map((item) => item.url).join("\n"));
          setDownloadUseAppSession(false);
          setDownloadStep("config");
          setDownloadTab("quick");
          setCustomParseResult(null);
          setCustomFormatId("");
          setCustomAudioFormatId("");
          setCustomSubtitleId("");
          setCustomPresetId("");
          setDownloadKeepOnlyTranscodedFile(true);
          setCustomParseError("");
          return;
        }
        setActiveMode("download");
        setDownloadPrepared(prepared);
        setBatchDownloadItems([]);
        setDownloadUrl(prepared.url || url);
        setDownloadUseAppSession(
          Boolean(
            prepared.appSessionAvailable &&
              downloadAppSessionModeCanExportCookies(
                prepared.appSessionCredentialMode ?? "",
              ),
          ),
        );
        setDownloadStep("config");
        setDownloadTab("quick");
        setCustomParseResult(null);
        setCustomFormatId("");
        setCustomAudioFormatId("");
        setCustomSubtitleId("");
        setCustomPresetId("");
        setDownloadKeepOnlyTranscodedFile(true);
        setCustomParseError("");
      } catch (error) {
        setDownloadPrepareError(resolveDownloadErrorMessage(error));
      }
    },
    [
      downloadUrl,
      prepareDownload,
      resolveDownloadErrorMessage,
      props.initialDownloadTargets,
    ],
  );

  React.useEffect(() => {
    const initialUrl = (props.initialUrl ?? "").trim();
    if (
      !props.open ||
      activeMode !== "download" ||
      downloadStep !== "input" ||
      !initialUrl ||
      autoPreparedInitialUrlRef.current === initialUrl ||
      !ytdlpInstalled ||
      prepareDownload.isPending
    ) {
      return;
    }
    autoPreparedInitialUrlRef.current = initialUrl;
    void handlePrepareDownload(initialUrl);
  }, [
    activeMode,
    downloadStep,
    handlePrepareDownload,
    prepareDownload.isPending,
    props.initialUrl,
    props.open,
    ytdlpInstalled,
  ]);

  const handleParseDownload = async () => {
    if (!downloadPrepared) {
      return;
    }
    const requestVersion = ++parseRequestVersionRef.current;
    setCustomParseError("");
    try {
      const parsed = await parseDownload.mutateAsync({
        url: downloadPrepared.url,
        appSessionId: downloadPrepared.appSessionId,
        useAppSession: downloadUseAppSession && downloadAppSessionCanExportCookies,
      });
      if (
        requestVersion !== parseRequestVersionRef.current ||
        !dialogOpenRef.current
      ) {
        return;
      }
      const playlistItems = (parsed.playlistItems ?? []).filter((item) =>
        item.url.trim(),
      );
      if (playlistItems.length > 0) {
        const playlistPrepared = preparedDownloadFromPlaylistItems(
          downloadPrepared,
          playlistItems,
        );
        const batchItems = playlistItems.map((item, index) =>
          preparedURLToBatchItem(item, index),
        );
        setActiveMode("download");
        setDownloadPrepared(playlistPrepared);
        setBatchDownloadItems(batchItems);
        setDownloadUrl(batchItems.map((item) => item.url).join("\n"));
        setDownloadUseAppSession(false);
        setDownloadStep("config");
        setDownloadTab("quick");
        setCustomParseResult(null);
        setCustomFormatId("");
        setCustomAudioFormatId("");
        setCustomSubtitleId("");
        setCustomPresetId("");
        setDownloadKeepOnlyTranscodedFile(true);
        setCustomParseError("");
        return;
      }
      if (!hasDownloadableFormats(parsed)) {
        setCustomParseResult(null);
        setCustomFormatId("");
        setCustomAudioFormatId("");
        setCustomSubtitleId("");
        setCustomPresetId("");
        setCustomParseError(noDownloadableMediaErrorMessage());
        return;
      }
      const defaultFormat = pickDefaultFormat(parsed.formats);
      setCustomParseResult(parsed);
      setCustomFormatId(defaultFormat?.id ?? "");
      setCustomAudioFormatId("");
      setCustomSubtitleId("");
      setCustomPresetId("");
    } catch (error) {
      if (
        requestVersion !== parseRequestVersionRef.current ||
        !dialogOpenRef.current
      ) {
        return;
      }
      setCustomParseError(
        resolveUnknownErrorMessage(error, text.common.unknown),
      );
    }
  };

  const handleDownloadTabChange = (value: string) => {
    const nextTab = value as DownloadDialogTab;
    if (!downloadTabItems.some((item) => item.value === nextTab)) {
      return;
    }
    if (nextTab === downloadTab) {
      return;
    }
    setDownloadTab(nextTab);
    resetParsedDownloadSelection();
  };

  const handleOpenConnections = React.useCallback(() => {
    if (!props.onOpenConnections) {
      return;
    }
    void closeDialogAfterResourceCleanup().finally(() => {
      props.onOpenConnections?.();
    });
  }, [closeDialogAfterResourceCleanup, props.onOpenConnections]);

  React.useEffect(() => {
    if (
      !props.open ||
      activeMode !== "sniff" ||
      !resourceSniffSessionId.trim() ||
      !props.onOpenSniffDesk
    ) {
      return;
    }
    handleOpenSniffDesk();
  }, [
    activeMode,
    handleOpenSniffDesk,
    props.onOpenSniffDesk,
    props.open,
    resourceSniffSessionId,
  ]);


  const handleStartQuickDownload = async () => {
    if (!downloadPrepared) {
      return;
    }
    setDownloadSubmitError("");
    try {
      await createYTDLP.mutateAsync({
        url: downloadPrepared.url,
        source: props.initialDownloadSource?.trim() || "xiadown.download.dialog",
        caller: props.initialDownloadCaller?.trim() || "main",
        mode: "quick",
        quality: quickQuality,
        writeThumbnail: true,
        subtitleAll: quickSubtitle,
        subtitleAuto: quickSubtitle,
        transcodePresetId: quickPresetId || undefined,
        deleteSourceFileAfterTranscode: quickPresetId
          ? downloadKeepOnlyTranscodedFile
          : undefined,
        appSessionId: downloadPrepared.appSessionId || undefined,
        useAppSession: downloadUseAppSession && downloadAppSessionCanExportCookies,
      });
      await closeDialogAfterResourceCleanup();
    } catch (error) {
      setDownloadSubmitError(resolveDownloadErrorMessage(error));
    }
  };

  const updateBatchDownloadItem = React.useCallback(
    (id: string, patch: Partial<BatchDownloadItemState>) => {
      setBatchDownloadItems((items) =>
        items.map((item) => {
          if (item.id !== id) {
            return item;
          }
          const next = { ...item, ...patch };
          if (patch.quality && patch.quality !== item.quality) {
            next.transcodePresetId = "";
          }
          return next;
        }),
      );
    },
    [],
  );

  const handleStartBatchDownload = async () => {
    if (batchDownloadItems.length === 0) {
      return;
    }
    setDownloadSubmitError("");
    try {
      await createYTDLPBatch.mutateAsync({
        items: batchDownloadItems.map((item) => {
          const origin = resolveDownloadTargetOrigin(item, {
            source: props.initialDownloadSource,
            caller: props.initialDownloadCaller,
          });
          return {
            url: item.url,
            ...origin,
            mode: "quick",
            quality: item.quality,
            writeThumbnail: true,
            subtitleAll: item.subtitles,
            subtitleAuto: item.subtitles,
            transcodePresetId: item.transcodePresetId || undefined,
            deleteSourceFileAfterTranscode: item.transcodePresetId
              ? item.deleteSourceFileAfterTranscode
              : undefined,
            appSessionId: item.appSessionId || undefined,
            useAppSession:
              item.useAppSession && batchDownloadItemCanUseAppSession(item),
          };
        }),
      });
      await closeDialogAfterResourceCleanup();
    } catch (error) {
      setDownloadSubmitError(resolveDownloadErrorMessage(error));
    }
  };

  const buildCustomDownloadRequest = (): CreateYTDLPJobRequest | null => {
    if (!downloadPrepared || !customParseResult || !customSelectedFormat) {
      return null;
    }
    const selectedSubtitleLang = customSelectedSubtitle?.language?.trim() ?? "";
    const needsAudioJoin =
      customSelectedFormat.hasVideo && !customSelectedFormat.hasAudio;
    return {
      url: downloadPrepared.url,
      source: props.initialDownloadSource?.trim() || "xiadown.download.dialog",
      caller: props.initialDownloadCaller?.trim() || "main",
      mode: "custom",
      title: customParseResult.title || undefined,
      extractor: customParseResult.extractor || undefined,
      author: customParseResult.author || undefined,
      thumbnailUrl: customParseResult.thumbnailUrl || undefined,
      writeThumbnail: true,
      quality: customSelectedFormat.hasVideo ? "best" : "audio",
      formatId: customSelectedFormat.id,
      audioFormatId: needsAudioJoin
        ? customAudioFormatId || selectAudioFormatId(customFormats) || "bestaudio"
        : undefined,
      subtitleLangs: selectedSubtitleLang ? [selectedSubtitleLang] : undefined,
      subtitleAuto: Boolean(customSelectedSubtitle?.isAuto),
      subtitleFormat: customSelectedSubtitle?.ext || undefined,
      transcodePresetId: customPresetId || undefined,
      deleteSourceFileAfterTranscode: customPresetId
        ? downloadKeepOnlyTranscodedFile
        : undefined,
      appSessionId: downloadPrepared.appSessionId || undefined,
      useAppSession: downloadUseAppSession && downloadAppSessionCanExportCookies,
    };
  };

  const handleStartCustomDownload = async () => {
    if (!customParseResult || !customSelectedFormat) {
      return;
    }
    const request = buildCustomDownloadRequest();
    if (!request) {
      return;
    }
    setDownloadSubmitError("");
    try {
      await createYTDLP.mutateAsync(request);
      await closeDialogAfterResourceCleanup();
    } catch (error) {
      setDownloadSubmitError(resolveDownloadErrorMessage(error));
    }
  };

  const handleChooseFile = async () => {
    const selection = await openFileDialog({
      Title: text.dialogs.transcodeTitle,
      AllowsOtherFiletypes: false,
      CanChooseFiles: true,
      CanChooseDirectories: false,
      Filters: [
        {
          DisplayName: `${text.dialogs.formatGroupVideo} / ${text.dialogs.formatGroupAudio}`,
          Pattern: buildDialogPattern([
            ...TRANSCODE_VIDEO_EXTENSIONS,
            ...TRANSCODE_AUDIO_EXTENSIONS,
          ]),
        },
        {
          DisplayName: text.dialogs.formatGroupVideo,
          Pattern: buildDialogPattern(TRANSCODE_VIDEO_EXTENSIONS),
        },
        {
          DisplayName: text.dialogs.formatGroupAudio,
          Pattern: buildDialogPattern(TRANSCODE_AUDIO_EXTENSIONS),
        },
      ],
    });
    const path = resolveDialogPath(selection);
    if (!path) {
      return;
    }
    if (!isSupportedTranscodeInputPath(path)) {
      setTranscodeSubmitError(text.dialogs.noCompatibleTranscodePreset);
      return;
    }
    applyTranscodeInputPath(path, null);
    setTranscodeLibraryOpen(false);
  };

  const handleChooseLibrarySource = (
    source: NewTaskDialogTranscodeSource,
  ) => {
    const displayLabel =
      source.displayLabel?.trim() ||
      source.inputPath?.trim() ||
      source.title?.trim() ||
      source.fileId?.trim() ||
      "";
    if (!displayLabel || !source.fileId?.trim()) {
      return;
    }
    applyTranscodeInputPath(displayLabel, source);
    setTranscodeLibraryOpen(false);
    setTranscodeLibraryQuery("");
  };

  const handleCreateTranscode = async () => {
    const inputPath = transcodeInputPath.trim();
    if (!inputPath) {
      return;
    }
    setTranscodeSubmitError("");
    try {
      await createTranscode.mutateAsync({
        fileId: transcodeSourceFileId || undefined,
        inputPath: transcodeSourceFileId ? undefined : inputPath,
        title: transcodeSourceTitle || resolveOpenFileName(inputPath),
        author: transcodeSourceAuthor || undefined,
        presetId: selectedTranscodePreset?.id || transcodePresetId || undefined,
        source: "xiadown.transcode.dialog",
      });
      await closeDialogAfterResourceCleanup();
    } catch (error) {
      setTranscodeSubmitError(
        resolveUnknownErrorMessage(error, text.common.unknown),
      );
    }
  };

  const handleInstallTaskDependencies = async () => {
    for (const name of taskDependencies) {
      const status = (toolsByName.get(name)?.status ?? "").trim().toLowerCase();
      if (status === "installed") {
        continue;
      }
      await installDependency.mutateAsync({ name });
    }
  };

  return (
    <Dialog open={props.open} onOpenChange={handleDialogOpenChange}>
      <DialogContent
        className="app-new-task-dialog gap-4 overflow-hidden"
        data-library-open={activeMode === "transcode" && transcodeLibraryOpen ? "true" : undefined}
        showCloseButton={false}
        onEscapeKeyDown={(event) => event.preventDefault()}
        onInteractOutside={(event) => event.preventDefault()}
      >
        <button
          type="button"
          aria-label={text.actions.close}
          title={text.actions.close}
          className="app-dialog-close app-new-task-browser-close absolute right-4 top-4 disabled:pointer-events-none"
          data-closing={closingDialog ? "true" : undefined}
          disabled={closingDialog}
          onClick={() => {
            void closeDialogAfterResourceCleanup();
          }}
        >
          {closingDialog ? (
            <Loader2 className="h-4 w-4 app-motion-spin" />
          ) : (
            <X className="h-4 w-4" />
          )}
          {closingDialog ? <span>{text.actions.closeBrowser}</span> : null}
        </button>
        <DialogHeader
          className={cn("app-new-task-header space-y-0", closingDialog ? "pr-28" : "pr-10")}
        >
          <DialogTitle className="app-new-task-title flex items-center gap-2">
            {activeMode === "transcode" ? (
              <FileCog className="app-new-task-title-icon h-4 w-4" />
            ) : activeMode === "sniff" ? (
              <Radar className="app-new-task-title-icon h-4 w-4" />
            ) : (
              <Download className="app-new-task-title-icon h-4 w-4" />
            )}
            <span>
              {activeMode === "transcode"
                ? text.dialogs.transcodeTitle
                : activeMode === "sniff"
                  ? text.dialogs.sniffTitle
                  : text.dialogs.downloadTitle}
            </span>
          </DialogTitle>
          <DialogDescription className="sr-only">
            {text.productSubtitle}
          </DialogDescription>
        </DialogHeader>

        {!taskDependenciesReady ? (
          <DependencyRepairCard
            text={text}
            dependencyNames={taskDependencies}
            toolsByName={toolsByName}
            updatesByName={updatesByName}
            installStagesByName={installStagesByName}
            installProgressByName={installProgressByName}
            installPending={installDependency.isPending}
            onInstallAll={handleInstallTaskDependencies}
            title={text.dialogs.dependenciesRequiredTitle}
            description={text.dialogs.dependenciesRequiredDescription}
          />
        ) : (
          <DialogScrollArea className="max-h-[min(68vh,34rem)] space-y-4">
            {activeDownloadMode && downloadStep === "input" ? (
              <DialogListCard className="app-new-task-panel">
                <DialogListCardContent className="p-4">
                  <form
                    className="flex gap-2"
                    onSubmit={(event) => {
                      event.preventDefault();
                      void handlePrepareDownload();
                    }}
                  >
                    <div className="app-new-task-field-strip flex min-w-0 flex-1 items-center overflow-hidden">
                      <Input
                        value={downloadUrl}
                        onChange={(event) => {
                          setDownloadUrl(event.target.value);
                          if (downloadPrepareError) {
                            setDownloadPrepareError("");
                          }
                        }}
                        placeholder={text.dialogs.downloadPlaceholder}
                        className="app-new-task-url-input h-full min-w-0 flex-1"
                      />
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <Button
                            type="button"
                            variant="ghost"
                            size="compactIcon"
                            className="app-new-task-field-action !h-full !w-9 shrink-0"
                            aria-label={text.actions.paste}
                            onClick={() => void handlePasteDownloadURL()}
                          >
                            <ClipboardPaste className="h-3.5 w-3.5" />
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent>{text.actions.paste}</TooltipContent>
                      </Tooltip>
                    </div>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <span className="inline-flex shrink-0">
                          <Button
                            type="submit"
                            size="compactIcon"
                            title={activeDownloadActionLabel}
                            aria-label={activeDownloadActionLabel}
                            disabled={
                              !downloadUrl.trim() ||
                              !ytdlpInstalled ||
                              prepareDownload.isPending
                            }
                          >
                            {prepareDownload.isPending ? (
                              <Loader2 className="h-4 w-4 app-motion-spin" />
                            ) : (
                              <Download className="h-4 w-4" />
                            )}
                          </Button>
                        </span>
                      </TooltipTrigger>
                      <TooltipContent>
                        {activeDownloadActionLabel}
                      </TooltipContent>
                    </Tooltip>
                  </form>
                  {!ytdlpInstalled ? (
                    <div className="app-dream-status-message mt-2 px-3 py-2" data-intent="warning">
                      {text.dependencies.missingDependency.replace(
                        "{name}",
                        "yt-dlp",
                      )}
                    </div>
                  ) : null}
                  {downloadPrepareError ? (
                    <div className="app-dream-status-message mt-2 px-3 py-2" data-intent="danger">
                      {downloadPrepareError}
                    </div>
                  ) : null}
                </DialogListCardContent>
              </DialogListCard>
            ) : null}

            {activeMode === "sniff" ? (
              <NewSniffSourceSteps
                confirming={startResourceSniff.isPending}
                onConfirm={handleConfirmResourceSniffSource}
              />
            ) : null}

            {activeDownloadMode && downloadStep === "config" ? (
              <>
                {!downloadIsBatch ? (
                  <DialogListCard className="app-new-task-panel">
                    <DialogListCardContent className="p-3">
                      <div
                        className="app-new-task-field-strip app-new-task-url-card-strip h-9 w-full min-w-0 overflow-hidden"
                        data-mode="app-session"
                      >
                        <div className="app-new-task-url-card-link flex h-full min-w-0 items-center">
                          <div
                            className="app-new-task-url-card-url flex h-full min-w-0 flex-1 items-center gap-1.5 px-3"
                            title={preparedDownloadUrl}
                          >
                            <SiteBrandIcon
                              siteKey={downloadSiteKey}
                              fallback="globe"
                              className="app-new-task-url-card-icon h-3.5 w-3.5 shrink-0"
                            />
                            <span
                              className="app-new-task-url-card-text min-w-0 flex-1"
                              dir="ltr"
                            >
                              {downloadUrlDisplayParts.prefix ? (
                                <span className="app-new-task-url-card-url-muted">
                                  {downloadUrlDisplayParts.prefix}
                                </span>
                              ) : null}
                              <span className="app-new-task-url-card-url-domain">
                                {downloadUrlDisplayParts.domain ||
                                  preparedDownloadUrl}
                              </span>
                              {downloadUrlDisplayParts.suffix ? (
                                <span className="app-new-task-url-card-url-muted">
                                  {downloadUrlDisplayParts.suffix}
                                </span>
                              ) : null}
                            </span>
                          </div>
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <Button
                                type="button"
                                variant="ghost"
                                size="compactIcon"
                                className="app-new-task-field-action !h-full !w-9 shrink-0"
                                aria-label={text.dialogs.modifyLink}
                                onClick={() => {
                                  if (downloadPrepared?.url) {
                                    setDownloadUrl(downloadPrepared.url);
                                  }
                                  resetDownloadConfig();
                                }}
                              >
                                <Pencil className="h-3.5 w-3.5" />
                              </Button>
                            </TooltipTrigger>
                            <TooltipContent>
                              {text.dialogs.modifyLink}
                            </TooltipContent>
                          </Tooltip>
                        </div>
                        <div className="app-new-task-url-card-mode-slot relative h-full min-w-0 overflow-hidden">
                          <div
                            className="app-new-task-url-card-mode-panel"
                            data-panel="app-session"
                            data-visible="true"
                          >
                            <span className="app-new-task-url-card-mode-label">
                              {downloadAppSessionStatusLabel}
                            </span>
                            {downloadAppSessionCanExportCookies ? (
                              <Tooltip>
                                <TooltipTrigger asChild>
                                  <span className="flex shrink-0 items-center justify-center">
                                    <InlineSwitch
                                      checked={downloadUseAppSession}
                                      onChange={setDownloadUseAppSession}
                                      ariaLabel={
                                        text.dialogs.appSessionCookiesDownload
                                      }
                                    />
                                  </span>
                                </TooltipTrigger>
                                <TooltipContent>
                                  {text.dialogs.appSessionCookiesDownload}
                                </TooltipContent>
                              </Tooltip>
                            ) : downloadMatchesCookieExportAppSession ? (
                              <Tooltip>
                                <TooltipTrigger asChild>
                                  <span className="inline-flex shrink-0">
                                    <Button
                                      type="button"
                                      variant="ghost"
                                      size="compactIcon"
                                      className="app-new-task-url-card-manage-button !h-7 !w-7"
                                      aria-label={text.dialogs.openConnections}
                                      disabled={!props.onOpenConnections}
                                      onClick={handleOpenConnections}
                                    >
                                      <SlidersHorizontal className="h-3.5 w-3.5" />
                                    </Button>
                                  </span>
                                </TooltipTrigger>
                                <TooltipContent>
                                  {text.dialogs.openConnections}
                                </TooltipContent>
                              </Tooltip>
                            ) : null}
                          </div>
                        </div>
                      </div>
                    </DialogListCardContent>
                  </DialogListCard>
                ) : null}

                {downloadIsBatch ? (
                  <div className="flex justify-center">
                    <DreamSegmentSwitch
                      value="batch"
                      className="app-new-task-download-mode-switch"
                      items={batchDownloadTabItems}
                      onValueChange={() => undefined}
                    />
                  </div>
                ) : (
                  <div className="flex justify-center">
                    <DreamSegmentSwitch
                      value={downloadTab}
                      className="app-new-task-download-mode-switch"
                      items={downloadTabItems}
                      onValueChange={handleDownloadTabChange}
                    />
                  </div>
                )}

                {downloadIsBatch ? (
                  <DialogListCard className="app-new-task-panel app-new-task-list-panel">
                    <DialogListCardContent className="max-h-[12.25rem] overflow-y-auto">
                      <table className="app-new-task-batch-table w-full table-fixed">
                        <colgroup>
                          <col />
                          <col className="w-[6.25rem]" />
                          <col className="w-[6.75rem]" />
                        </colgroup>
                        <thead className="app-new-task-batch-table-head sticky top-0 z-10">
                          <tr className="app-new-task-batch-table-head-row">
                            <th className="px-3 py-2" scope="col">
                              {text.completed.taskDataFields.url}
                            </th>
                            <th className="px-3 py-2" scope="col">
                              {text.dialogs.useAppSession}
                            </th>
                            <th className="px-3 py-2" scope="col">
                              {text.dialogs.quality}
                            </th>
                          </tr>
                        </thead>
                        <tbody>
                          {batchDownloadItems.map((item) => {
                            const displayParts = buildDownloadUrlDisplayParts(
                              item.url,
                              item.domain,
                            );
                            const batchPresets = filterTranscodePresetsForMediaType(
                              presetsQuery.data ?? [],
                              batchDownloadItemMediaType(item),
                            );
                            const selectedBatchPreset =
                              batchPresets.find(
                                (preset) => preset.id === item.transcodePresetId,
                              ) ?? null;
                            const selectedBatchQualityLabel =
                              formatDownloadQualityLabel(
                                item.quality,
                                text.dialogs,
                              );
                            const selectedBatchParamsSummary = [
                              selectedBatchQualityLabel,
                              item.subtitles
                                ? text.dialogs.subtitles
                                : text.dialogs.noSubtitle,
                              selectedBatchPreset?.name ??
                                text.dialogs.noTranscode,
                            ].join(" / ");
                            const canUseBatchAppSession =
                              batchDownloadItemCanUseAppSession(item);
                            const batchAppSessionTooltip = canUseBatchAppSession
                              ? text.dialogs.appSessionCookiesDownload
                              : item.appSessionId
                                ? text.dialogs.appSessionNotConfigured
                                : text.dialogs.noAvailableAppSession;

                            return (
                              <tr
                                key={item.id}
                                className="app-new-task-batch-row"
                              >
                                <td className="px-3 py-3 align-middle">
                                  <Tooltip>
                                    <TooltipTrigger asChild>
                                      <div
                                        className="app-new-task-batch-url min-w-0 truncate"
                                        dir="ltr"
                                      >
                                        {displayParts.prefix ? (
                                          <span className="app-new-task-secondary-text">
                                            {displayParts.prefix}
                                          </span>
                                        ) : null}
                                        <span className="app-new-task-primary-text">
                                          {displayParts.domain || item.url}
                                        </span>
                                        {displayParts.suffix ? (
                                          <span className="app-new-task-secondary-text">
                                            {displayParts.suffix}
                                          </span>
                                        ) : null}
                                      </div>
                                    </TooltipTrigger>
                                    <TooltipContent
                                      align="start"
                                      className="app-new-task-monospace"
                                      multiline
                                      side="bottom"
                                    >
                                      {item.url}
                                    </TooltipContent>
                                  </Tooltip>
                                </td>
                                <td className="app-new-task-batch-toggle-cell px-3 py-3 align-middle">
                                  <Tooltip>
                                    <TooltipTrigger asChild>
                                      <span className="inline-flex items-center justify-center">
                                        <InlineSwitch
                                          checked={
                                            canUseBatchAppSession &&
                                            item.useAppSession
                                          }
                                          disabled={!canUseBatchAppSession}
                                          onChange={(checked) =>
                                            updateBatchDownloadItem(item.id, {
                                              useAppSession: checked,
                                            })
                                          }
                                          ariaLabel={
                                            text.dialogs
                                              .appSessionCookiesDownload
                                          }
                                        />
                                      </span>
                                    </TooltipTrigger>
                                    <TooltipContent>
                                      {batchAppSessionTooltip}
                                    </TooltipContent>
                                  </Tooltip>
                                </td>
                                <td className="app-new-task-batch-actions-cell px-3 py-3 align-middle">
                                  <div className="flex justify-end">
                                    <DropdownMenu>
                                      <DropdownMenuTrigger asChild>
                                        <Button
                                          type="button"
                                          variant="outline"
                                          size="compact"
                                          className="h-8 w-[6.75rem] justify-between px-2"
                                          title={selectedBatchParamsSummary}
                                          aria-label={text.dialogs.quality}
                                        >
                                          <span className="app-new-task-batch-control-label min-w-0 flex-1 truncate">
                                            {selectedBatchQualityLabel}
                                          </span>
                                          <ChevronDown className="app-new-task-disclosure-icon h-3.5 w-3.5 shrink-0" />
                                        </Button>
                                      </DropdownMenuTrigger>
                                      <DropdownMenuContent
                                        align="end"
                                        className="w-[18rem] p-2"
                                      >
                                        <div className="space-y-1.5">
                                          <div className="app-new-task-detail-row flex items-center justify-between gap-3 px-2 py-1.5">
                                            <span className="app-new-task-secondary-text">
                                              {text.dialogs.quality}
                                            </span>
                                            <div className="flex flex-wrap items-center justify-end gap-1.5">
                                              <Button
                                                type="button"
                                                variant={
                                                  item.quality === "best"
                                                    ? "default"
                                                    : "outline"
                                                }
                                                size="compact"
                                                onClick={() =>
                                                  updateBatchDownloadItem(
                                                    item.id,
                                                    { quality: "best" },
                                                  )
                                                }
                                              >
                                                {text.dialogs.qualityBest}
                                              </Button>
                                              <Button
                                                type="button"
                                                variant={
                                                  item.quality === "bitrate"
                                                    ? "default"
                                                    : "outline"
                                                }
                                                size="compact"
                                                onClick={() =>
                                                  updateBatchDownloadItem(
                                                    item.id,
                                                    { quality: "bitrate" },
                                                  )
                                                }
                                              >
                                                {text.dialogs.qualityBitrate}
                                              </Button>
                                              <Button
                                                type="button"
                                                variant={
                                                  item.quality === "audio"
                                                    ? "default"
                                                    : "outline"
                                                }
                                                size="compact"
                                                onClick={() =>
                                                  updateBatchDownloadItem(
                                                    item.id,
                                                    { quality: "audio" },
                                                  )
                                                }
                                              >
                                                {text.dialogs.qualityAudio}
                                              </Button>
                                            </div>
                                          </div>
                                          <div className="app-new-task-detail-row flex items-center justify-between gap-3 px-2 py-1.5">
                                            <span className="app-new-task-secondary-text">
                                              {text.dialogs.subtitles}
                                            </span>
                                            <InlineSwitch
                                              checked={item.subtitles}
                                              onChange={(checked) =>
                                                updateBatchDownloadItem(item.id, {
                                                  subtitles: checked,
                                                })
                                              }
                                              ariaLabel={text.dialogs.subtitles}
                                            />
                                          </div>
                                          <div className="app-new-task-detail-row flex items-center justify-between gap-3 px-2 py-1.5">
                                            <span className="app-new-task-secondary-text">
                                              {text.actions.transcode}
                                            </span>
                                            <Select
                                              value={item.transcodePresetId}
                                              onChange={(event) =>
                                                updateBatchDownloadItem(item.id, {
                                                  transcodePresetId:
                                                    event.target.value,
                                                })
                                              }
                                              className="app-new-task-compact-select h-8 w-40"
                                              aria-label={text.actions.transcode}
                                            >
                                              <option value="">
                                                {text.dialogs.noTranscode}
                                              </option>
                                              {batchPresets.map((preset) => (
                                                <option
                                                  key={preset.id}
                                                  value={preset.id}
                                                >
                                                  {preset.name}
                                                </option>
                                              ))}
                                            </Select>
                                          </div>
                                          {item.transcodePresetId ? (
                                            <div className="app-new-task-detail-row flex items-center justify-between gap-3 px-2 py-1.5">
                                              <span className="app-new-task-secondary-text">
                                                {
                                                  text.dialogs
                                                    .keepOnlyTranscodedFile
                                                }
                                              </span>
                                              <InlineSwitch
                                                checked={
                                                  item.deleteSourceFileAfterTranscode
                                                }
                                                onChange={(checked) =>
                                                  updateBatchDownloadItem(
                                                    item.id,
                                                    {
                                                      deleteSourceFileAfterTranscode:
                                                        checked,
                                                    },
                                                  )
                                                }
                                                ariaLabel={
                                                  text.dialogs
                                                    .keepOnlyTranscodedFile
                                                }
                                              />
                                            </div>
                                          ) : null}
                                        </div>
                                      </DropdownMenuContent>
                                    </DropdownMenu>
                                  </div>
                                </td>
                              </tr>
                            );
                          })}
                        </tbody>
                      </table>
                    </DialogListCardContent>
                  </DialogListCard>
                ) : null}

                {downloadTab === "quick" && !downloadIsBatch ? (
                  <DialogListCard className="app-new-task-panel app-new-task-list-panel">
                    <DialogListCardContent>
                      <DialogRow className="app-new-task-row flex items-center justify-between gap-4 p-3">
                        <span className="app-new-task-row-label">
                          {text.dialogs.quality}
                        </span>
                        <div className="flex flex-wrap items-center justify-end gap-2">
                          <Button
                            type="button"
                            variant={
                              quickQuality === "best" ? "default" : "outline"
                            }
                            size="compact"
                            onClick={() => setQuickQuality("best")}
                          >
                            {text.dialogs.qualityBest}
                          </Button>
                          <Button
                            type="button"
                            variant={
                              quickQuality === "bitrate" ? "default" : "outline"
                            }
                            size="compact"
                            onClick={() => setQuickQuality("bitrate")}
                          >
                            {text.dialogs.qualityBitrate}
                          </Button>
                          <Button
                            type="button"
                            variant={
                              quickQuality === "audio" ? "default" : "outline"
                            }
                            size="compact"
                            onClick={() => setQuickQuality("audio")}
                          >
                            {text.dialogs.qualityAudio}
                          </Button>
                        </div>
                      </DialogRow>
                      <DialogRow className="app-new-task-row flex items-center justify-between gap-4 p-3">
                        <span className="app-new-task-row-label">
                          {text.dialogs.subtitles}
                        </span>
                        <InlineSwitch
                          checked={quickSubtitle}
                          onChange={setQuickSubtitle}
                          ariaLabel={text.dialogs.subtitles}
                        />
                      </DialogRow>
                      <DialogRow className="app-new-task-row flex items-center justify-between gap-4 p-3">
                        <span className="app-new-task-row-label">
                          {text.actions.transcode}
                        </span>
                        <Select
                          className="w-56 max-w-[58vw]"
                          value={quickPresetId}
                          onChange={(event) =>
                            setQuickPresetId(event.target.value)
                          }
                        >
                          <option value="">{text.dialogs.noTranscode}</option>
                          {(quickPresetsQuery.data ?? []).map((preset) => (
                            <option key={preset.id} value={preset.id}>
                              {preset.name}
                            </option>
                          ))}
                        </Select>
                      </DialogRow>
                      {quickPresetId ? (
                        <DialogRow className="app-new-task-row flex items-center justify-between gap-4 p-3">
                          <div className="app-new-task-row-label">
                            {text.dialogs.keepOnlyTranscodedFile}
                          </div>
                          <InlineSwitch
                            checked={downloadKeepOnlyTranscodedFile}
                            onChange={setDownloadKeepOnlyTranscodedFile}
                            ariaLabel={text.dialogs.keepOnlyTranscodedFile}
                          />
                        </DialogRow>
                      ) : null}
                    </DialogListCardContent>
                  </DialogListCard>
                ) : null}

                {downloadTab === "custom" ? (
                  customParseResult ? (
                    <DialogListCard className="app-new-task-panel app-new-task-list-panel min-w-0 overflow-hidden">
                      <DialogListCardContent>
                        <DialogRow className="app-new-task-row app-new-task-select-row p-3">
                          <span className="app-new-task-select-row-label">
                            {text.dialogs.quality}
                          </span>
                          <Select
                            className="app-new-task-select"
                            value={customFormatId}
                            onChange={(event) => setCustomFormatId(event.target.value)}
                          >
                            <option value="">{text.dialogs.selectFormat}</option>
                            {customVideoFormats.length > 0 ? (
                              <optgroup label={text.dialogs.formatGroupVideo}>
                                {customVideoFormats.map((format) => (
                                  <option key={format.id} value={format.id}>
                                    {format.label}
                                  </option>
                                ))}
                              </optgroup>
                            ) : null}
                            {customAudioFormats.length > 0 ? (
                              <optgroup label={text.dialogs.formatGroupAudio}>
                                {customAudioFormats.map((format) => (
                                  <option key={format.id} value={format.id}>
                                    {format.label}
                                  </option>
                                ))}
                              </optgroup>
                            ) : null}
                          </Select>
                        </DialogRow>
                        {customCanSelectAudioTrack ? (
                          <DialogRow className="app-new-task-row app-new-task-select-row p-3">
                            <span className="app-new-task-select-row-label">
                              {text.dialogs.audioTrack}
                            </span>
                            <Select
                              className="app-new-task-select"
                              value={customAudioFormatId}
                              onChange={(event) => setCustomAudioFormatId(event.target.value)}
                            >
                              <option value="">{text.dialogs.audioTrack}</option>
                              {customAudioFormats.map((format) => (
                                <option key={format.id} value={format.id}>
                                  {formatAudioTrackLabel(format)}
                                </option>
                              ))}
                            </Select>
                          </DialogRow>
                        ) : null}
                        <DialogRow className="app-new-task-row app-new-task-select-row p-3">
                          <span className="app-new-task-select-row-label">
                            {text.dialogs.subtitles}
                          </span>
                          <Select
                            className="app-new-task-select"
                            value={customSubtitleId}
                            onChange={(event) => setCustomSubtitleId(event.target.value)}
                          >
                            <option value="">{text.dialogs.noSubtitle}</option>
                            {customSubtitles.map((subtitle) => (
                              <option key={subtitle.id} value={subtitle.id}>
                                {formatSubtitleLabel(subtitle)}
                              </option>
                            ))}
                          </Select>
                        </DialogRow>
                        <DialogRow className="app-new-task-row app-new-task-select-row p-3">
                          <span className="app-new-task-select-row-label">
                            {text.actions.transcode}
                          </span>
                          <Select
                            className="app-new-task-select"
                            value={customPresetId}
                            onChange={(event) => setCustomPresetId(event.target.value)}
                          >
                            <option value="">{text.dialogs.noTranscode}</option>
                            {(customPresetsQuery.data ?? []).map((preset) => (
                              <option key={preset.id} value={preset.id}>
                                {preset.name}
                              </option>
                            ))}
                          </Select>
                        </DialogRow>
                        {customPresetId ? (
                          <DialogRow className="app-new-task-row flex items-center justify-between gap-4 p-3">
                            <div className="app-new-task-row-label">
                              {text.dialogs.keepOnlyTranscodedFile}
                            </div>
                            <InlineSwitch
                              checked={downloadKeepOnlyTranscodedFile}
                              onChange={setDownloadKeepOnlyTranscodedFile}
                              ariaLabel={text.dialogs.keepOnlyTranscodedFile}
                            />
                          </DialogRow>
                        ) : null}
                      </DialogListCardContent>
                    </DialogListCard>
                  ) : (
                    <div className="flex flex-col items-center justify-center gap-2 py-4">
                      <Button
                        type="button"
                        size="compact"
                        onClick={() => void handleParseDownload()}
                        disabled={parseDownload.isPending}
                      >
                        {parseDownload.isPending ? (
                          <Loader2 className="h-4 w-4 app-motion-spin" />
                        ) : null}
                        {customParseError
                          ? text.dialogs.parseAgain
                          : text.dialogs.parse}
                      </Button>
                      {customParseError ? (
                        <div className="w-full">
                          <div
                            className="app-new-task-parse-error w-full min-w-0 truncate px-3 py-2"
                            title={customParseErrorDetail || customParseErrorLine}
                          >
                            {customParseErrorLine}
                          </div>
                        </div>
                      ) : null}
                    </div>
                  )
                ) : null}

                {downloadSubmitError ? (
                  <div className="app-dream-status-message px-3 py-2" data-intent="danger">
                    {downloadSubmitError}
                  </div>
                ) : null}
              </>
            ) : null}

            {activeMode === "transcode" && !transcodeInputPath && !transcodeLibraryOpen ? (
              <DialogListCard className="app-new-task-panel">
                <DialogListCardContent className="grid gap-3 p-4 sm:grid-cols-2">
                  <Button
                    type="button"
                    variant="outline"
                    className="app-new-task-transcode-source-choice flex-col"
                    onClick={() => void handleChooseFile()}
                  >
                    <FolderOpen className="h-4 w-4" />
                    {text.actions.chooseFile}
                  </Button>
                  <Button
                    type="button"
                    variant="outline"
                    className="app-new-task-transcode-source-choice flex-col"
                    onClick={() => setTranscodeLibraryOpen(true)}
                  >
                    <LibraryBig className="h-4 w-4" />
                    {text.dialogs.selectFromLibrary}
                  </Button>
                </DialogListCardContent>
              </DialogListCard>
            ) : null}

            {activeMode === "transcode" && !transcodeInputPath && transcodeLibraryOpen ? (
              <DialogListCard className="app-new-task-panel min-w-0 overflow-hidden">
                <DialogListCardContent className="space-y-4 p-4">
                  <div className="flex items-center gap-2">
                    <Button
                      type="button"
                      variant="ghost"
                      size="compact"
                      className="shrink-0"
                      onClick={() => {
                        setTranscodeLibraryOpen(false);
                        setTranscodeLibraryQuery("");
                      }}
                    >
                      {text.actions.back}
                    </Button>
                    <div className="app-new-task-field-strip flex min-w-0 flex-1 items-center overflow-hidden">
                      <Search className="app-new-task-secondary-text ml-3 h-3.5 w-3.5 shrink-0" />
                      <Input
                        value={transcodeLibraryQuery}
                        onChange={(event) => setTranscodeLibraryQuery(event.target.value)}
                        placeholder={text.dialogs.selectFromLibrary}
                        aria-label={text.dialogs.selectFromLibrary}
                        className="app-new-task-library-search h-full min-w-0 flex-1"
                        autoFocus
                      />
                    </div>
                  </div>
                  {props.transcodeLibraryLoading ? (
                    <div
                      className="app-new-task-library-feedback flex min-h-40 items-center justify-center gap-2 px-4"
                      role="status"
                    >
                      <Loader2 className="h-4 w-4 app-motion-spin" aria-hidden="true" />
                      <span>{text.listen.loading}</span>
                    </div>
                  ) : props.transcodeLibraryError?.trim() ? (
                    <div
                      className="app-new-task-library-feedback flex min-h-40 flex-col items-center justify-center gap-3 px-4"
                      role="alert"
                    >
                      <span>{props.transcodeLibraryError}</span>
                      {props.onRetryTranscodeLibrary ? (
                        <Button
                          type="button"
                          size="compact"
                          variant="outline"
                          onClick={props.onRetryTranscodeLibrary}
                        >
                          {text.listen.retry}
                        </Button>
                      ) : null}
                    </div>
                  ) : transcodeLibraryGroups.length > 0 ? (
                    <div className="max-h-[min(52vh,28rem)] space-y-5 overflow-y-auto pr-1">
                      {transcodeLibraryGroups.map((group) => (
                        <section className="space-y-2" key={group.name}>
                          <h3 className="app-new-task-library-group-title">
                            {group.name}
                          </h3>
                          <div className="grid gap-2 sm:grid-cols-2">
                            {group.items.map((source) => {
                              const label = source.title || source.displayLabel || source.fileId;
                              const metadata = [
                                source.author,
                                source.format?.toUpperCase(),
                                formatCompletedDuration(source.durationMs),
                              ].filter(Boolean).join(" · ");
                              return (
                                <button
                                  type="button"
                                  className="app-new-task-library-item flex min-w-0 items-center gap-3 p-2"
                                  key={source.fileId}
                                  onClick={() => handleChooseLibrarySource(source)}
                                >
                                  <div className="app-new-task-library-artwork flex h-14 w-20 shrink-0 items-center justify-center overflow-hidden">
                                    {source.coverURL ? (
                                      <img
                                        src={source.coverURL}
                                        alt=""
                                        className="h-full w-full object-cover"
                                      />
                                    ) : (
                                      <Video className="app-new-task-secondary-text h-5 w-5" />
                                    )}
                                  </div>
                                  <span className="min-w-0 flex-1">
                                    <span className="app-new-task-library-item-title block truncate">
                                      {label}
                                    </span>
                                    {metadata ? (
                                      <span className="app-new-task-library-item-meta mt-1 block truncate">
                                        {metadata}
                                      </span>
                                    ) : null}
                                  </span>
                                </button>
                              );
                            })}
                          </div>
                        </section>
                      ))}
                    </div>
                  ) : (
                    <div className="app-new-task-library-feedback flex min-h-40 items-center justify-center px-4">
                      {text.dialogs.transcodeLibraryEmpty}
                    </div>
                  )}
                </DialogListCardContent>
              </DialogListCard>
            ) : null}

            {activeMode === "transcode" && transcodeInputPath ? (
              <>
                <DialogListCard className="app-new-task-panel">
                  <DialogListCardContent className="space-y-2 p-4">
                    <div className="app-new-task-file-heading flex min-w-0 items-center gap-2">
                      <span className="flex min-w-0 flex-1 items-baseline">
                        <span className="app-new-task-primary-text min-w-0 truncate">
                          {transcodeFileName.stem}
                        </span>
                        {transcodeFileName.extension ? (
                          <span className="app-new-task-secondary-text shrink-0">
                            {transcodeFileName.extension}
                          </span>
                        ) : null}
                      </span>
                    </div>
                    <div className="app-new-task-field-strip flex h-9 w-full min-w-0 items-center overflow-hidden">
                      <Input
                        size="default"
                        value={transcodeInputPath}
                        readOnly
                        className="app-new-task-path-input h-full min-w-0 flex-1 truncate py-0"
                      />
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <Button
                            type="button"
                            variant="ghost"
                            size="compactIcon"
                            className="app-new-task-field-action !h-full !w-9 shrink-0"
                            aria-label={text.dialogs.modifyFile}
                            onClick={() => void handleChooseFile()}
                          >
                            <Pencil className="h-3.5 w-3.5" />
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent>{text.dialogs.modifyFile}</TooltipContent>
                      </Tooltip>
                    </div>
                  </DialogListCardContent>
                </DialogListCard>

                {ffmpegInstalled ? (
                  <DialogListCard className="app-new-task-panel">
                    <DialogListCardContent className="app-new-task-probe flex min-h-12 items-center gap-3 p-3">
                      {transcodeProbeChecking ? (
                        <Loader2 className="app-new-task-secondary-text h-4 w-4 shrink-0 app-motion-spin" />
                      ) : probedTranscodeMediaType === "audio" ? (
                        <AudioLines className="app-new-task-secondary-text h-4 w-4 shrink-0" />
                      ) : probedTranscodeMediaType === "video" ? (
                        <Video className="app-new-task-secondary-text h-4 w-4 shrink-0" />
                      ) : (
                        <FileCog className="app-new-task-secondary-text h-4 w-4 shrink-0" />
                      )}
                      <div className="min-w-0 flex-1">
                        <div className="flex min-w-0 items-center gap-2">
                          {transcodeProbeSummary ? (
                            <span className="app-new-task-primary-text min-w-0 truncate">
                              {transcodeProbeSummary}
                            </span>
                          ) : transcodeProbeStatusLabel ? (
                            <span className="app-new-task-primary-text min-w-0 truncate">
                              {transcodeProbeStatusLabel}
                            </span>
                          ) : null}
                        </div>
                        {transcodeProbeError ? (
                          <div className="app-new-task-secondary-text mt-1 truncate">
                            {transcodeProbeError}
                          </div>
                        ) : null}
                      </div>
                    </DialogListCardContent>
                  </DialogListCard>
                ) : null}

                {showTranscodeOptions ? (
                  <DialogListCard className="app-new-task-panel app-new-task-list-panel">
                    <DialogListCardContent>
                      <DialogRow className="app-new-task-row flex items-center justify-between gap-4 p-3">
                        <span className="app-new-task-row-label">
                          {text.dialogs.size}
                        </span>
                        <Select
                          className="w-40 max-w-[58vw]"
                          value={transcodeScale}
                          onChange={(event) =>
                            setTranscodeScale(event.target.value)
                          }
                        >
                          {transcodeSizeOptions.map((option) => (
                            <option key={option.value} value={option.value}>
                              {option.label}
                            </option>
                          ))}
                        </Select>
                      </DialogRow>
                      <DialogRow className="app-new-task-row flex items-center justify-between gap-4 p-3">
                        <span className="app-new-task-row-label">
                          {text.dialogs.container}
                        </span>
                        <Select
                          className="w-40 max-w-[58vw]"
                          value={transcodeContainer}
                          onChange={(event) =>
                            setTranscodeContainer(event.target.value)
                          }
                        >
                          {transcodeContainerOptions.map((option) => (
                            <option key={option.value} value={option.value}>
                              {option.label}
                            </option>
                          ))}
                        </Select>
                      </DialogRow>
                      <DialogRow className="app-new-task-row flex items-center justify-between gap-4 p-3">
                        <span className="app-new-task-row-label">
                          {text.dialogs.codec}
                        </span>
                        <Select
                          className="w-40 max-w-[58vw]"
                          value={transcodeCodec}
                          onChange={(event) =>
                            setTranscodeCodec(event.target.value)
                          }
                        >
                          {transcodeCodecOptions.map((option) => (
                            <option key={option.value} value={option.value}>
                              {option.label}
                            </option>
                          ))}
                        </Select>
                      </DialogRow>
                    </DialogListCardContent>
                  </DialogListCard>
                ) : null}
                {!ffmpegInstalled ? (
                  <div className="app-dream-status-message px-3 py-2" data-intent="warning">
                    {text.dependencies.missingDependency.replace(
                      "{name}",
                      "ffmpeg",
                    )}
                  </div>
                ) : null}
                {transcodeNoCompatiblePreset ? (
                  <div className="app-dream-status-message px-3 py-2" data-intent="warning">
                    {text.dialogs.noCompatibleTranscodePreset}
                  </div>
                ) : null}
                {transcodeSubmitError ? (
                  <div className="app-dream-status-message px-3 py-2" data-intent="danger">
                    {transcodeSubmitError}
                  </div>
                ) : null}
              </>
            ) : null}
          </DialogScrollArea>
        )}

        {taskDependenciesReady &&
        activeDownloadMode &&
        showDownloadFooter ? (
          <DialogFooter>
            <Button
              type="button"
              variant="ghost"
              onClick={() => {
                void closeDialogAfterResourceCleanup();
              }}
              disabled={closingDialog}
            >
              {text.actions.cancelDialog}
            </Button>
            <Button
              type="button"
              onClick={() =>
                void (downloadIsBatch
                  ? handleStartBatchDownload()
                  : downloadTab === "quick"
                    ? handleStartQuickDownload()
                    : handleStartCustomDownload())
              }
              disabled={
                createYTDLP.isPending ||
                createYTDLPBatch.isPending ||
                !downloadPrepared ||
                !ytdlpInstalled ||
                (downloadIsBatch && batchDownloadItems.length === 0) ||
                (downloadTab === "custom" &&
                  (!customParseResult || !customSelectedFormat))
              }
            >
              {createYTDLP.isPending || createYTDLPBatch.isPending ? (
                <Loader2 className="h-4 w-4 app-motion-spin" />
              ) : null}
              {text.actions.startTask}
            </Button>
          </DialogFooter>
        ) : null}

        {taskDependenciesReady && showTranscodeFooter ? (
          <DialogFooter>
            <Button
              type="button"
              variant="ghost"
              onClick={() => {
                void closeDialogAfterResourceCleanup();
              }}
              disabled={closingDialog}
            >
              {text.actions.cancelDialog}
            </Button>
            <Button
              type="button"
              onClick={() => void handleCreateTranscode()}
              disabled={
                !transcodeInputPath ||
                !ffmpegInstalled ||
                !transcodeProbeReady ||
                transcodeProbeChecking ||
                transcodeProbeQuery.isError ||
                transcodeNoCompatiblePreset ||
                createTranscode.isPending ||
                !selectedTranscodePreset
              }
            >
              {createTranscode.isPending ? (
                <Loader2 className="h-4 w-4 app-motion-spin" />
              ) : null}
              {text.actions.startTask}
            </Button>
          </DialogFooter>
        ) : null}
      </DialogContent>
    </Dialog>
  );
}
