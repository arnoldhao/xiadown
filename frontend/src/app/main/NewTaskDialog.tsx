import {
  AudioLines,
  ChevronDown,
  Download,
  FileCog,
  FolderOpen,
  Loader2,
  Pencil,
  Radar,
  SlidersHorizontal,
  Video,
  X,
  Zap,
} from "lucide-react";
import * as React from "react";

import { getXiaText } from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
import type {
  CreateYTDLPJobRequest,
  LibraryMediaInfoDTO,
  ParseYTDLPDownloadResponse,
  PreparedYTDLPDownloadURL,
  PrepareYTDLPDownloadResponse,
  ProbeTranscodeInputRequest,
  ResourceSniffFailure,
} from "@/shared/contracts/library";
import type { Settings } from "@/shared/contracts/settings";
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
  useParseResourceSniff,
  usePrepareYTDLPDownload,
  useProbeTranscodeInput,
  useResourceSniffSession,
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
import { TASK_DIALOG_DEPENDENCIES } from "@/app/main/main-constants";
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
  NewTaskDialogMode,
  NewTaskDialogTranscodeSource,
  SourceMediaType,
} from "@/app/main/types";

type DownloadEntryMode = "direct" | "sniff";

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
};

function downloadAppSessionModeCanExportCookies(mode: string) {
  const normalized = mode.trim().toLowerCase();
  return normalized === "cookies" || normalized === "app_session";
}

function preparedURLToBatchItem(
  item: PreparedYTDLPDownloadURL,
  index: number,
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
    <button
      type="button"
      role="switch"
      aria-checked={props.checked}
      aria-label={props.ariaLabel}
      disabled={props.disabled}
      onClick={() => {
        if (!props.disabled) {
          props.onChange(!props.checked);
        }
      }}
      className="app-dream-inline-switch disabled:cursor-not-allowed disabled:opacity-50"
      data-state={props.checked ? "checked" : "unchecked"}
    >
      <span className="app-dream-inline-switch-knob" />
    </button>
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

function resolveResourceSniffFailureDescription(
  failure: ResourceSniffFailure,
  text: ReturnType<typeof getXiaText>,
) {
  const descriptions: Record<ResourceSniffFailure["code"], string> = {
    profile_connection_required: text.dialogs.profileConnectionRequired,
    verification_required: text.dialogs.resourceVerificationRequired,
    no_media_detected: text.dialogs.resourceNoMediaDetected,
    unsupported_douyin_lvdetail: text.dialogs.resourceDouyinLVDetail,
    douyin_recommend_login_required:
      text.dialogs.resourceDouyinRecommendLoginRequired,
  };
  return descriptions[failure.code];
}

function noDownloadableMediaErrorMessage() {
  return "[resource_no_media_detected] no downloadable formats found";
}

function hasDownloadableFormats(parsed: ParseYTDLPDownloadResponse) {
  return (parsed.formats ?? []).some((format) => format.id.trim().length > 0);
}

function normalizeResourceSniffPageUrl(value?: string) {
  const trimmed = (value ?? "").trim();
  if (!trimmed) {
    return "";
  }
  try {
    const url = new URL(trimmed);
    url.hash = "";
    return url.toString();
  } catch {
    return trimmed;
  }
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
  initialTranscodeSource?: NewTaskDialogTranscodeSource | null;
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
  const bunInstallState = useDependencyInstallState("bun", props.open);
  const prepareDownload = usePrepareYTDLPDownload();
  const parseDownload = useParseYTDLPDownload();
  const startResourceSniff = useStartResourceSniff();
  const parseResourceSniff = useParseResourceSniff();
  const cancelResourceSniff = useCancelResourceSniff();
  const createYTDLP = useCreateYTDLPJob();
  const createYTDLPBatch = useCreateYTDLPBatchJobs();
  const queueYTDLP = useCreateYTDLPJob();
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
  const [downloadPrepareIntent, setDownloadPrepareIntent] =
    React.useState<DownloadEntryMode | null>(null);
  const [downloadSubmitError, setDownloadSubmitError] = React.useState("");
  const [quickQuality, setQuickQuality] =
    React.useState<DownloadQuality>("best");
  const [quickSubtitle, setQuickSubtitle] = React.useState(false);
  const [quickPresetId, setQuickPresetId] = React.useState("");
  const [downloadKeepOnlyTranscodedFile, setDownloadKeepOnlyTranscodedFile] =
    React.useState(true);
  const [customParseResult, setCustomParseResult] =
    React.useState<ParseYTDLPDownloadResponse | null>(null);
  const [customParsePageUrl, setCustomParsePageUrl] = React.useState("");
  const [customFormatId, setCustomFormatId] = React.useState("");
  const [customAudioFormatId, setCustomAudioFormatId] = React.useState("");
  const [customSubtitleId, setCustomSubtitleId] = React.useState("");
  const [customPresetId, setCustomPresetId] = React.useState("");
  const [customParseError, setCustomParseError] = React.useState("");
  const [resourceSniffSessionId, setResourceSniffSessionId] =
    React.useState("");
  const [resourceSniffFailure, setResourceSniffFailure] =
    React.useState<ResourceSniffFailure | null>(null);
  const [resourceSniffTechnicalError, setResourceSniffTechnicalError] =
    React.useState("");
  const [resourceSniffFinalBrowserStatus, setResourceSniffFinalBrowserStatus] =
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
  const [closingDialog, setClosingDialog] = React.useState(false);
  const autoPreparedInitialUrlRef = React.useRef("");
  const dialogOpenRef = React.useRef(props.open);
  const dialogClosingRef = React.useRef(false);
  const preserveResourceSniffOnCloseRef = React.useRef(false);
  const parseRequestVersionRef = React.useRef(0);
  const resourceSniffStartVersionRef = React.useRef(0);
  const resourceSniffTransferStartVersionRef = React.useRef<number | null>(null);
  const customParsePageObservedRef = React.useRef(false);
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
      setResourceSniffFinalBrowserStatus("");
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
        ["bun", bunInstallState.data],
      ]),
    [bunInstallState.data, ffmpegInstallState.data, ytdlpInstallState.data],
  );
  const installStagesByName = React.useMemo(
    () =>
      new Map(
        TASK_DIALOG_DEPENDENCIES.map((name) => [
          name,
          (installStatesByName.get(name)?.stage ?? "idle").toString(),
        ]),
      ),
    [installStatesByName],
  );
  const installProgressByName = React.useMemo(
    () =>
      new Map(
        TASK_DIALOG_DEPENDENCIES.map((name) => [
          name,
          clampProgress(installStatesByName.get(name)?.progress),
        ]),
      ),
    [installStatesByName],
  );
  const ytdlpInstalled =
    (toolsByName.get("yt-dlp")?.status ?? "").trim().toLowerCase() ===
    "installed";
  const ffmpegInstalled =
    (toolsByName.get("ffmpeg")?.status ?? "").trim().toLowerCase() ===
    "installed";
  const taskDependenciesReady = TASK_DIALOG_DEPENDENCIES.every(
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
  const downloadShowsSniffMode = downloadTab === "sniff" && !downloadIsBatch;
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
  const activeDownloadMode =
    activeMode === "download" || activeMode === "sniff";
  const activeDownloadEntryMode: DownloadEntryMode =
    activeMode === "sniff" ? "sniff" : "direct";
  const activeDownloadActionLabel =
    activeDownloadEntryMode === "sniff"
      ? text.dialogs.startSniffMode
      : text.dialogs.directDownload;
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
  const resourceSniffSessionQuery = useResourceSniffSession(
    resourceSniffSessionId ? { sessionId: resourceSniffSessionId } : null,
    props.open &&
      activeMode === "sniff" &&
      downloadTab === "sniff" &&
      Boolean(resourceSniffSessionId),
  );
  const resourceSniffPolledSession = resourceSniffSessionQuery.data ?? null;
  React.useEffect(() => {
    if (
      !resourceSniffPolledSession ||
      resourceSniffSessionId.trim().length === 0
    ) {
      return;
    }
    const browserStatus = resourceSniffPolledSession.browserStatus?.trim() ?? "";
    if (browserStatus === "open") {
      setResourceSniffFinalBrowserStatus("");
      return;
    }
    if (browserStatus) {
      setResourceSniffFinalBrowserStatus(browserStatus);
      return;
    }
    if (
      resourceSniffPolledSession.state &&
      resourceSniffPolledSession.state !== "running"
    ) {
      setResourceSniffFinalBrowserStatus(resourceSniffPolledSession.state);
    }
  }, [resourceSniffPolledSession, resourceSniffSessionId]);
  const startedResourceSniffSession = startResourceSniff.data?.session ?? null;
  const resourceSniffStartedSession =
    startedResourceSniffSession?.sessionId === resourceSniffSessionId
      ? startedResourceSniffSession
      : null;
  const resourceSniffSession =
    resourceSniffPolledSession ?? resourceSniffStartedSession;
  React.useEffect(() => {
    if (downloadTab !== "sniff" || !customParseResult?.resourceSessionId) {
      customParsePageObservedRef.current = false;
      return;
    }
    const parsedPageUrl = normalizeResourceSniffPageUrl(customParsePageUrl);
    const currentPageUrl = normalizeResourceSniffPageUrl(
      resourceSniffSession?.currentUrl,
    );
    if (!parsedPageUrl || !currentPageUrl) {
      return;
    }
    if (parsedPageUrl === currentPageUrl) {
      customParsePageObservedRef.current = true;
      return;
    }
    if (!customParsePageObservedRef.current) {
      return;
    }
    customParsePageObservedRef.current = false;
    setCustomParseResult(null);
    setCustomFormatId("");
    setCustomAudioFormatId("");
    setCustomSubtitleId("");
    setCustomPresetId("");
    setCustomParsePageUrl("");
    setResourceSniffFailure(null);
    setResourceSniffTechnicalError("");
  }, [
    customParsePageUrl,
    customParseResult?.resourceSessionId,
    downloadTab,
    resourceSniffSession?.currentUrl,
  ]);
  const resourceSniffBrowserStatus =
    resourceSniffPolledSession?.browserStatus ||
    resourceSniffFinalBrowserStatus ||
    resourceSniffStartedSession?.browserStatus ||
    (resourceSniffSessionId ? "open" : "");
  const resourceSniffBrowserOpen = resourceSniffBrowserStatus === "open";
  const resourceSniffBrowserExited =
    Boolean(resourceSniffSessionId) &&
    Boolean(resourceSniffSession) &&
    resourceSniffBrowserStatus === "browser_closed";
  const resourceSniffHasActiveTab =
    resourceSniffBrowserOpen &&
    Boolean(resourceSniffSession?.activeTargetId) &&
    (resourceSniffSession?.tabCount ?? 0) > 0;
  const resourceSniffStatusLabel = resourceSniffBrowserExited
    ? text.dialogs.sniffBrowserExited
    : resourceSniffHasActiveTab
      ? text.dialogs.sniffBrowserOpen
      : resourceSniffSessionId
        ? text.dialogs.sniffNoActiveTab
        : text.dialogs.sniffReady;
  const resourceSniffCurrentPageLabel = resourceSniffHasActiveTab
    ? resourceSniffSession?.title || resourceSniffSession?.currentUrl || ""
    : resourceSniffSessionId
      ? text.dialogs.sniffNoActiveTab
      : "";
  const showResourceSniffCurrentPage =
    Boolean(resourceSniffSessionId) &&
    (resourceSniffHasActiveTab
      ? Boolean(resourceSniffSession?.currentUrl || resourceSniffSession?.title)
      : Boolean(resourceSniffSession));
  const resourceSniffAuthStatus =
    downloadTab === "sniff"
      ? resourceSniffSession?.authStatus?.trim() ?? ""
      : "";
  const resourceSniffCurrentUserLabel =
    resourceSniffAuthStatus === "logged_in"
      ? resourceSniffSession?.authUser?.trim() || text.dialogs.loggedIn
      : resourceSniffAuthStatus === "logged_out"
        ? text.dialogs.notLoggedIn
        : text.common.unknown;
  const showResourceSniffCurrentUser =
    resourceSniffHasActiveTab && Boolean(resourceSniffSession?.authSite?.trim());
  const resourceSniffUnsupportedDomain =
    downloadTab === "sniff"
      ? resourceSniffSession?.unoptimizedDomain?.trim() ?? ""
      : "";
  const resourceSniffUnsupportedDomainHint = resourceSniffUnsupportedDomain
    ? text.sniffDesk.urlUnsupportedDomain.replace(
        "{domain}",
        resourceSniffUnsupportedDomain,
      )
    : "";
  const resourceSniffNeedsDesk = Boolean(resourceSniffUnsupportedDomain);
  const downloadRequiresSniffDesk =
    downloadTab === "sniff" && resourceSniffNeedsDesk;
  const resourceSniffUnsupportedDomainNotice = resourceSniffUnsupportedDomainHint ? (
    <div
      className="app-dream-status-message app-new-task-sniff-unsupported-notice max-w-full px-3 py-1.5 text-center text-xs font-medium leading-5"
      data-intent="warning"
      role="note"
      title={resourceSniffUnsupportedDomainHint}
    >
      {resourceSniffUnsupportedDomainHint}
    </div>
  ) : null;
  const isSniffCardRefreshing =
    downloadTab === "sniff" &&
    parseResourceSniff.isPending &&
    Boolean(customParseResult);
  const resourceSniffIssueDescription = resourceSniffFailure
    ? resolveResourceSniffFailureDescription(resourceSniffFailure, text)
    : resourceSniffTechnicalError.trim();
  const resourceSniffHasIssue = resourceSniffIssueDescription.length > 0;
  const resourceSniffThumbnailUrl =
    downloadTab === "sniff"
      ? customParseResult?.thumbnailUrl?.trim() ?? ""
      : "";
  const showDownloadFooter =
    downloadStep === "config" &&
    (downloadIsBatch ||
      downloadTab === "quick" ||
      ((downloadTab === "custom" || downloadTab === "sniff") &&
        Boolean(customParseResult)));
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
    setDownloadTab(initialMode === "sniff" ? "sniff" : "quick");
    setDownloadPrepareError("");
    setDownloadPrepareIntent(null);
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
    parseResourceSniff.reset();
    setActiveResourceSniffSessionId("");
    setResourceSniffFailure(null);
    setResourceSniffTechnicalError("");
    if (initialMode === "transcode" && props.initialTranscodeSource?.inputPath.trim()) {
      applyTranscodeInputPath(
        props.initialTranscodeSource.inputPath.trim(),
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
    setTranscodeSubmitError("");
    autoPreparedInitialUrlRef.current = "";
  }, [
    props.initialMode,
    props.initialTranscodeSource?.author,
    props.initialTranscodeSource?.fileId,
    props.initialTranscodeSource?.inputPath,
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
    parseResourceSniff.reset();
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
    parseResourceSniff.reset();
    setDownloadPrepared(null);
    setBatchDownloadItems([]);
    setDownloadStep("input");
    setDownloadUseAppSession(false);
    setDownloadTab(activeMode === "sniff" ? "sniff" : "quick");
    setDownloadPrepareIntent(null);
    setDownloadSubmitError("");
    setDownloadKeepOnlyTranscodedFile(true);
    setCustomParseResult(null);
    setCustomFormatId("");
    setCustomAudioFormatId("");
    setCustomSubtitleId("");
    setCustomPresetId("");
    setCustomParseError("");
    setResourceSniffFailure(null);
    setResourceSniffTechnicalError("");
  };

  const resetParsedDownloadSelection = () => {
    parseRequestVersionRef.current += 1;
    parseDownload.reset();
    parseResourceSniff.reset();
    setCustomParseResult(null);
    setCustomFormatId("");
    setCustomAudioFormatId("");
    setCustomSubtitleId("");
    setCustomPresetId("");
    setCustomParseError("");
  };

  const handleActiveModeChange = (nextMode: NewTaskDialogMode) => {
    if (nextMode === activeMode) {
      return;
    }
    setActiveMode(nextMode);
    if (nextMode === "download") {
      setDownloadTab("quick");
      return;
    }
    if (nextMode === "sniff") {
      setDownloadTab("sniff");
    }
  };

  const startPreparedResourceSniff = React.useCallback(
    async (prepared: PrepareYTDLPDownloadResponse) => {
      const existingSessionID = resourceSniffSessionIdRef.current.trim();
      if (existingSessionID) {
        await cancelResourceSniffSession(existingSessionID);
      }
      resetParsedDownloadSelection();
      const startVersion = ++resourceSniffStartVersionRef.current;
      setResourceSniffFailure(null);
      setResourceSniffTechnicalError("");
      const startMutationPromise = startResourceSniff.mutateAsync({
        url: prepared.url,
      });
      const startPromise = startMutationPromise.then(
        (result) => result.session?.sessionId ?? "",
      );
      resourceSniffStartPromiseRef.current = startPromise;
      try {
        const result = await startMutationPromise;
        if (result.failure) {
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
            return;
          }
          setResourceSniffFailure(result.failure);
          setResourceSniffTechnicalError("");
          return;
        }
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
          return;
        }
        if (startResolution === "cancel") {
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
          return;
        }
        if (!dialogOpenRef.current && !preserveResourceSniffOnCloseRef.current) {
          return;
        }
        setResourceSniffFailure(null);
        setResourceSniffTechnicalError(
          resolveUnknownErrorMessage(error, text.common.unknown),
        );
      } finally {
        if (resourceSniffStartPromiseRef.current === startPromise) {
          resourceSniffStartPromiseRef.current = null;
        }
      }
    },
    [
      cancelResourceSniffSession,
      parseDownload,
      parseResourceSniff,
      startResourceSniff,
      text.common.unknown,
    ],
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

  const handlePrepareDownload = React.useCallback(
    async (
      mode: DownloadEntryMode,
      overrideUrl?: string,
      options?: { autoStartSniff?: boolean },
    ) => {
      const url = (overrideUrl ?? downloadUrl).trim();
      if (!url) {
        return;
      }
      const autoStartSniff = options?.autoStartSniff ?? true;
      setDownloadPrepareIntent(mode);
      setDownloadPrepareError("");
      setDownloadSubmitError("");
      setResourceSniffFailure(null);
      setResourceSniffTechnicalError("");
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
          const batchItems = preparedURLs.map(preparedURLToBatchItem);
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
        const nextTab: DownloadDialogTab = mode === "sniff" ? "sniff" : "quick";
        setActiveMode(nextTab === "sniff" ? "sniff" : "download");
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
        setDownloadTab(nextTab);
        setCustomParseResult(null);
        setCustomFormatId("");
        setCustomAudioFormatId("");
        setCustomSubtitleId("");
        setCustomPresetId("");
        setDownloadKeepOnlyTranscodedFile(true);
        setCustomParseError("");
        if (nextTab === "sniff" && autoStartSniff) {
          await startPreparedResourceSniff(prepared);
        }
      } catch (error) {
        setDownloadPrepareError(resolveDownloadErrorMessage(error));
      } finally {
        setDownloadPrepareIntent(null);
      }
    },
    [
      downloadUrl,
      prepareDownload,
      resolveDownloadErrorMessage,
      startPreparedResourceSniff,
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
    void handlePrepareDownload("direct", initialUrl, { autoStartSniff: false });
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
        const batchItems = playlistItems.map(preparedURLToBatchItem);
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
    setResourceSniffFailure(null);
    setResourceSniffTechnicalError("");
  };

  const handleOpenConnections = React.useCallback(() => {
    if (!props.onOpenConnections) {
      return;
    }
    void closeDialogAfterResourceCleanup().finally(() => {
      props.onOpenConnections?.();
    });
  }, [closeDialogAfterResourceCleanup, props.onOpenConnections]);

  const handleOpenSniffDesk = React.useCallback(() => {
    if (!props.onOpenSniffDesk) {
      return;
    }
    resourceSniffTransferStartVersionRef.current =
      resourceSniffStartVersionRef.current;
    preserveResourceSniffOnCloseRef.current = true;
    parseRequestVersionRef.current += 1;
    setActiveResourceSniffSessionId("");
    setResourceSniffFailure(null);
    setResourceSniffTechnicalError("");
    dialogOpenRef.current = false;
    props.onOpenChange(false);
    props.onOpenSniffDesk();
  }, [
    props.onOpenChange,
    props.onOpenSniffDesk,
    setActiveResourceSniffSessionId,
  ]);

  const handleStartResourceSniff = async () => {
    if (!downloadPrepared) {
      return;
    }
    await startPreparedResourceSniff(downloadPrepared);
  };

  const handleParseResourceSniff = async () => {
    const sessionId = resourceSniffSessionId.trim();
    if (!sessionId) {
      return;
    }
    const requestVersion = ++parseRequestVersionRef.current;
    const requestPageUrl = resourceSniffSession?.currentUrl?.trim() ?? "";
    const hasCurrentResult = Boolean(customParseResult);
    if (!hasCurrentResult) {
      setCustomParseResult(null);
      setCustomFormatId("");
      setCustomAudioFormatId("");
      setCustomSubtitleId("");
      setCustomPresetId("");
      setCustomParsePageUrl("");
    }
    setResourceSniffFailure(null);
    setResourceSniffTechnicalError("");
    try {
      const response = await parseResourceSniff.mutateAsync({ sessionId });
      if (
        requestVersion !== parseRequestVersionRef.current ||
        !dialogOpenRef.current
      ) {
        return;
      }
      if (response.failure) {
        setCustomParseResult(null);
        setCustomFormatId("");
        setCustomAudioFormatId("");
        setCustomSubtitleId("");
        setCustomPresetId("");
        setCustomParsePageUrl("");
        setResourceSniffFailure(response.failure);
        setResourceSniffTechnicalError("");
        return;
      }
      const parsed = response.media;
      if (!parsed) {
        throw new Error("resource sniff parse returned no media");
      }
      if (!hasDownloadableFormats(parsed)) {
        throw new Error("resource sniff parse returned media without formats");
      }
      const defaultFormat = pickDefaultFormat(parsed.formats);
      const parsedPageUrl = parsed.pageUrl?.trim() || requestPageUrl;
      customParsePageObservedRef.current = false;
      setCustomParseResult(parsed);
      setCustomParsePageUrl(parsedPageUrl);
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
      setCustomParseResult(null);
      setCustomFormatId("");
      setCustomAudioFormatId("");
      setCustomSubtitleId("");
      setCustomPresetId("");
      setCustomParsePageUrl("");
      setResourceSniffFailure(null);
      setResourceSniffTechnicalError(
        resolveUnknownErrorMessage(error, text.common.unknown),
      );
    }
  };

  const handleStartQuickDownload = async () => {
    if (!downloadPrepared) {
      return;
    }
    setDownloadSubmitError("");
    try {
      await createYTDLP.mutateAsync({
        url: downloadPrepared.url,
        source: "xiadown.download.dialog",
        caller: "main",
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
        items: batchDownloadItems.map((item) => ({
          url: item.url,
          source: "xiadown.download.dialog",
          caller: "main",
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
        })),
      });
      await closeDialogAfterResourceCleanup();
    } catch (error) {
      setDownloadSubmitError(resolveDownloadErrorMessage(error));
    }
  };

  const buildCustomDownloadRequest = (
    resourceReference: "none" | "session" | "media",
  ): CreateYTDLPJobRequest | null => {
    if (!downloadPrepared || !customParseResult || !customSelectedFormat) {
      return null;
    }
    const selectedSubtitleLang = customSelectedSubtitle?.language?.trim() ?? "";
    const needsAudioJoin =
      customSelectedFormat.hasVideo && !customSelectedFormat.hasAudio;
    return {
      url: downloadPrepared.url,
      source: "xiadown.download.dialog",
      caller: "main",
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
      resourceSessionId:
        resourceReference === "session"
          ? customParseResult.resourceSessionId || undefined
          : undefined,
      resourceMediaId:
        resourceReference === "media"
          ? customParseResult.resourceMediaId || undefined
          : undefined,
    };
  };

  const handleQueueResourceDownload = async () => {
    const request = buildCustomDownloadRequest("media");
    if (!request?.resourceMediaId) {
      return;
    }
    setDownloadSubmitError("");
    try {
      await queueYTDLP.mutateAsync(request);
      resetParsedDownloadSelection();
      setResourceSniffFailure(null);
      setResourceSniffTechnicalError("");
    } catch (error) {
      setDownloadSubmitError(resolveDownloadErrorMessage(error));
    }
  };

  const handleStartCustomDownload = async () => {
    if (!customParseResult || !customSelectedFormat) {
      return;
    }
    const request = buildCustomDownloadRequest(
      customParseResult.resourceSessionId ? "session" : "none",
    );
    if (!request) {
      return;
    }
    setDownloadSubmitError("");
    try {
      await createYTDLP.mutateAsync(request);
      if (customParseResult.resourceSessionId) {
        setActiveResourceSniffSessionId("");
      }
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
    for (const name of TASK_DIALOG_DEPENDENCIES) {
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
        className="max-w-[min(92vw,32rem)] gap-4 overflow-hidden"
        showCloseButton={false}
        onEscapeKeyDown={(event) => event.preventDefault()}
        onInteractOutside={(event) => event.preventDefault()}
      >
        <button
          type="button"
          className="app-dialog-close app-new-task-browser-close absolute right-4 top-4 transition-[background,color,box-shadow] focus:outline-none disabled:pointer-events-none"
          data-closing={closingDialog ? "true" : undefined}
          disabled={closingDialog}
          onClick={() => {
            void closeDialogAfterResourceCleanup();
          }}
        >
          {closingDialog ? (
            <Loader2 className="h-4 w-4 animate-spin" />
          ) : (
            <X className="h-4 w-4" />
          )}
          {closingDialog ? <span>{text.actions.closeBrowser}</span> : null}
        </button>
        <DialogHeader
          className={cn("space-y-0 text-left", closingDialog ? "pr-28" : "pr-10")}
        >
          <DialogTitle className="sr-only">
            {activeMode === "transcode"
              ? text.dialogs.transcodeTitle
              : activeMode === "sniff"
                ? text.dialogs.sniffMode
                : text.dialogs.downloadTitle}
          </DialogTitle>
          <DialogDescription className="sr-only">
            {text.productSubtitle}
          </DialogDescription>
          <DreamSegmentSwitch
            value={activeMode}
            className="app-new-task-mode-switch mr-auto"
            items={[
              {
                value: "download",
                label: text.actions.download,
                icon: <Download className="h-3.5 w-3.5" />,
              },
              {
                value: "sniff",
                label: text.dialogs.sniffMode,
                icon: <Radar className="h-3.5 w-3.5" />,
              },
              {
                value: "transcode",
                label: text.actions.transcode,
                icon: <FileCog className="h-3.5 w-3.5" />,
              },
            ]}
            onValueChange={handleActiveModeChange}
          />
        </DialogHeader>

        {!taskDependenciesReady ? (
          <DependencyRepairCard
            text={text}
            dependencyNames={TASK_DIALOG_DEPENDENCIES}
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
                      void handlePrepareDownload(activeDownloadEntryMode);
                    }}
                  >
                    <Input
                      value={downloadUrl}
                      onChange={(event) => {
                        setDownloadUrl(event.target.value);
                        if (downloadPrepareError) {
                          setDownloadPrepareError("");
                        }
                      }}
                      placeholder={text.dialogs.downloadPlaceholder}
                      className="app-new-task-url-input min-w-0 flex-1"
                    />
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
                            {prepareDownload.isPending &&
                            downloadPrepareIntent === activeDownloadEntryMode ? (
                              <Loader2 className="h-4 w-4 animate-spin" />
                            ) : activeDownloadEntryMode === "sniff" ? (
                              <Radar className="h-4 w-4" />
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
                    <div className="app-dream-status-message mt-2 px-3 py-2 text-xs" data-intent="warning">
                      {text.dependencies.missingDependency.replace(
                        "{name}",
                        "yt-dlp",
                      )}
                    </div>
                  ) : null}
                  {downloadPrepareError ? (
                    <div className="app-dream-status-message mt-2 px-3 py-2 text-xs" data-intent="danger">
                      {downloadPrepareError}
                    </div>
                  ) : null}
                </DialogListCardContent>
              </DialogListCard>
            ) : null}

            {activeDownloadMode && downloadStep === "config" ? (
              <>
                {!downloadIsBatch ? (
                  <DialogListCard className="app-new-task-panel">
                    <DialogListCardContent className="p-3">
                      <div
                        className="app-new-task-field-strip app-new-task-url-card-strip h-9 w-full min-w-0 overflow-hidden"
                        data-mode={
                          downloadShowsSniffMode ? "sniff" : "app-session"
                        }
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
                            data-visible={
                              downloadShowsSniffMode ? "false" : "true"
                            }
                            aria-hidden={
                              downloadShowsSniffMode ? "true" : undefined
                            }
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
                          <div
                            className="app-new-task-url-card-mode-panel app-new-task-url-card-mode-panel-sniff"
                            data-panel="sniff"
                            data-visible={
                              downloadShowsSniffMode ? "true" : "false"
                            }
                            aria-hidden={
                              downloadShowsSniffMode ? undefined : "true"
                            }
                          >
                            <Radar className="h-3.5 w-3.5 shrink-0" />
                            <span className="app-new-task-url-card-mode-label">
                              {text.dialogs.sniffMode}
                            </span>
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
                ) : downloadTab !== "sniff" ? (
                  <div className="flex justify-center">
                    <DreamSegmentSwitch
                      value={downloadTab}
                      className="app-new-task-download-mode-switch"
                      items={downloadTabItems}
                      onValueChange={handleDownloadTabChange}
                    />
                  </div>
                ) : null}

                {downloadIsBatch ? (
                  <DialogListCard className="app-new-task-panel app-new-task-list-panel">
                    <DialogListCardContent className="max-h-[12.25rem] overflow-y-auto">
                      <table className="w-full table-fixed text-sm">
                        <colgroup>
                          <col />
                          <col className="w-[6.25rem]" />
                          <col className="w-[6.75rem]" />
                        </colgroup>
                        <thead className="sticky top-0 z-10 bg-background/95 backdrop-blur">
                          <tr className="border-b border-border/60 text-[11px] font-medium text-muted-foreground">
                            <th className="px-3 py-2 text-center font-medium" scope="col">
                              {text.completed.taskDataFields.url}
                            </th>
                            <th className="px-3 py-2 text-center font-medium" scope="col">
                              {text.dialogs.useAppSession}
                            </th>
                            <th className="px-3 py-2 text-center font-medium" scope="col">
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
                                className="border-b border-border/60 transition-colors last:border-b-0 hover:bg-muted/30"
                              >
                                <td className="px-3 py-3 align-middle">
                                  <Tooltip>
                                    <TooltipTrigger asChild>
                                      <div
                                        className="min-w-0 truncate text-xs"
                                        dir="ltr"
                                      >
                                        {displayParts.prefix ? (
                                          <span className="text-muted-foreground">
                                            {displayParts.prefix}
                                          </span>
                                        ) : null}
                                        <span className="font-medium text-foreground">
                                          {displayParts.domain || item.url}
                                        </span>
                                        {displayParts.suffix ? (
                                          <span className="text-muted-foreground">
                                            {displayParts.suffix}
                                          </span>
                                        ) : null}
                                      </div>
                                    </TooltipTrigger>
                                    <TooltipContent
                                      align="start"
                                      className="font-mono"
                                      multiline
                                      side="bottom"
                                    >
                                      {item.url}
                                    </TooltipContent>
                                  </Tooltip>
                                </td>
                                <td className="px-3 py-3 text-center align-middle">
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
                                <td className="px-3 py-3 text-right align-middle">
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
                                          <span className="min-w-0 flex-1 truncate text-left">
                                            {selectedBatchQualityLabel}
                                          </span>
                                          <ChevronDown className="h-3.5 w-3.5 shrink-0 opacity-70" />
                                        </Button>
                                      </DropdownMenuTrigger>
                                      <DropdownMenuContent
                                        align="end"
                                        className="w-[18rem] p-2"
                                      >
                                        <div className="space-y-1.5">
                                          <div className="flex items-center justify-between gap-3 rounded-md px-2 py-1.5">
                                            <span className="text-muted-foreground">
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
                                          <div className="flex items-center justify-between gap-3 rounded-md px-2 py-1.5">
                                            <span className="text-muted-foreground">
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
                                          <div className="flex items-center justify-between gap-3 rounded-md px-2 py-1.5">
                                            <span className="text-muted-foreground">
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
                                              className="h-8 w-40 text-xs"
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
                                            <div className="flex items-center justify-between gap-3 rounded-md px-2 py-1.5">
                                              <span className="text-muted-foreground">
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
                      <DialogRow className="app-new-task-row flex items-center justify-between gap-4 p-3 text-sm">
                        <span className="text-muted-foreground">
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
                      <DialogRow className="app-new-task-row flex items-center justify-between gap-4 p-3 text-sm">
                        <span className="text-muted-foreground">
                          {text.dialogs.subtitles}
                        </span>
                        <InlineSwitch
                          checked={quickSubtitle}
                          onChange={setQuickSubtitle}
                          ariaLabel={text.dialogs.subtitles}
                        />
                      </DialogRow>
                      <DialogRow className="app-new-task-row flex items-center justify-between gap-4 p-3 text-sm">
                        <span className="text-muted-foreground">
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
                        <DialogRow className="app-new-task-row flex items-center justify-between gap-4 p-3 text-sm">
                          <div className="text-muted-foreground">
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

                {downloadTab === "custom" || downloadTab === "sniff" ? (
                  customParseResult ? (
                    <>
                      <DialogListCard
                        className={`app-new-task-panel app-new-task-list-panel min-w-0 overflow-hidden ${
                          downloadTab === "sniff" ? "app-new-task-media-card" : ""
                        }`}
                        data-has-cover={
                          resourceSniffThumbnailUrl ? "true" : "false"
                        }
                        data-refreshing={
                          isSniffCardRefreshing ? "true" : "false"
                        }
                      >
                        <div
                          className="app-new-task-media-card-visuals"
                          data-refreshing={
                            isSniffCardRefreshing ? "true" : undefined
                          }
                          aria-hidden="true"
                        >
                          {resourceSniffThumbnailUrl ? (
                            <div
                              className="app-new-task-media-card-cover-stage"
                              data-refreshing={
                                isSniffCardRefreshing ? "true" : undefined
                              }
                            >
                              <img
                                src={resourceSniffThumbnailUrl}
                                alt=""
                                className="app-new-task-media-card-cover-blur"
                                loading="lazy"
                                decoding="async"
                                draggable={false}
                              />
                              <div className="app-new-task-media-card-cover-detail-wrap">
                                <img
                                  src={resourceSniffThumbnailUrl}
                                  alt=""
                                  className="app-new-task-media-card-cover-detail"
                                  loading="lazy"
                                  decoding="async"
                                  draggable={false}
                                />
                              </div>
                              <div className="app-new-task-media-card-cover-softener" />
                              <div className="app-new-task-media-card-cover-gradient" />
                              <div className="app-new-task-media-card-cover-texture" />
                              <div className="app-new-task-media-card-cover-sweep" />
                            </div>
                          ) : null}
                        </div>
                        <DialogListCardContent
                          className={
                            downloadTab === "sniff"
                              ? "app-new-task-media-card-content"
                              : undefined
                          }
                        >
                          {downloadTab === "sniff" ? (
                            <>
                              <DialogRow className="app-new-task-row app-new-task-media-meta-row flex items-center justify-between gap-4 p-3 text-sm">
                                <span className="text-muted-foreground">
                                  {text.dialogs.mediaTitle}
                                </span>
                                <span
                                  className="min-w-0 max-w-[68%] truncate text-right font-medium text-foreground"
                                  title={
                                    customParseResult.title ||
                                    text.common.unknown
                                  }
                                >
                                  {customParseResult.title ||
                                    text.common.unknown}
                                </span>
                              </DialogRow>
                              <DialogRow className="app-new-task-row app-new-task-media-meta-row flex items-center justify-between gap-4 p-3 text-sm">
                                <span className="text-muted-foreground">
                                  {text.dialogs.mediaAuthor}
                                </span>
                                <span
                                  className="min-w-0 max-w-[68%] truncate text-right font-medium text-foreground"
                                  title={
                                    customParseResult.author ||
                                    text.common.unknown
                                  }
                                >
                                  {customParseResult.author ||
                                    text.common.unknown}
                                </span>
                              </DialogRow>
                            </>
                          ) : null}
                          <DialogRow className="app-new-task-row app-new-task-select-row p-3 text-sm">
                            <span className="app-new-task-select-row-label text-muted-foreground">
                              {text.dialogs.quality}
                            </span>
                            <Select
                              className="app-new-task-select"
                              value={customFormatId}
                              onChange={(event) =>
                                setCustomFormatId(event.target.value)
                              }
                            >
                              <option value="">
                                {text.dialogs.selectFormat}
                              </option>
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
                            <DialogRow className="app-new-task-row app-new-task-select-row p-3 text-sm">
                              <span className="app-new-task-select-row-label text-muted-foreground">
                                {text.dialogs.audioTrack}
                              </span>
                              <Select
                                className="app-new-task-select"
                                value={customAudioFormatId}
                                onChange={(event) =>
                                  setCustomAudioFormatId(event.target.value)
                                }
                              >
                                <option value="">
                                  {text.dialogs.audioTrack}
                                </option>
                                {customAudioFormats.map((format) => (
                                  <option key={format.id} value={format.id}>
                                    {formatAudioTrackLabel(format)}
                                  </option>
                                ))}
                              </Select>
                            </DialogRow>
                          ) : null}
                          {downloadTab !== "sniff" ||
                          customSubtitles.length > 0 ? (
                            <DialogRow className="app-new-task-row app-new-task-select-row p-3 text-sm">
                              <span className="app-new-task-select-row-label text-muted-foreground">
                                {text.dialogs.subtitles}
                              </span>
                              <Select
                                className="app-new-task-select"
                                value={customSubtitleId}
                                onChange={(event) =>
                                  setCustomSubtitleId(event.target.value)
                                }
                              >
                                <option value="">
                                  {text.dialogs.noSubtitle}
                                </option>
                                {customSubtitles.map((subtitle) => (
                                  <option key={subtitle.id} value={subtitle.id}>
                                    {formatSubtitleLabel(subtitle)}
                                  </option>
                                ))}
                              </Select>
                            </DialogRow>
                          ) : null}
                          <DialogRow className="app-new-task-row app-new-task-select-row p-3 text-sm">
                            <span className="app-new-task-select-row-label text-muted-foreground">
                              {text.actions.transcode}
                            </span>
                            <Select
                              className="app-new-task-select"
                              value={customPresetId}
                              onChange={(event) =>
                                setCustomPresetId(event.target.value)
                              }
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
                            <DialogRow className="app-new-task-row flex items-center justify-between gap-4 p-3 text-sm">
                              <div className="text-muted-foreground">
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
                      {downloadRequiresSniffDesk ? (
                        <div className="flex flex-wrap items-center justify-center gap-2">
                          {resourceSniffUnsupportedDomainNotice}
                          <Button
                            type="button"
                            size="compact"
                            variant="outline"
                            onClick={handleOpenSniffDesk}
                            disabled={!props.onOpenSniffDesk}
                          >
                            <Radar className="h-4 w-4" />
                            {text.dialogs.enterSniffDesk}
                          </Button>
                        </div>
                      ) : null}
                      {downloadTab === "sniff" && resourceSniffHasIssue ? (
                        <div className="app-new-task-parse-error w-full min-w-0 px-3 py-2 text-xs font-medium">
                          {resourceSniffIssueDescription}
                        </div>
                      ) : null}
                    </>
                  ) : downloadTab === "sniff" ? (
                    <div className="space-y-3">
                      {resourceSniffSessionId ? (
                        <DialogListCard className="app-new-task-panel app-new-task-list-panel min-w-0 overflow-hidden">
                          <DialogListCardContent>
                            <DialogRow className="app-new-task-row flex items-center justify-between gap-4 p-3 text-sm">
                              <span className="text-muted-foreground">
                                {text.dialogs.currentStatus}
                              </span>
                              <div className="flex max-w-[68%] items-center justify-end gap-2">
                                <span className="min-w-0 truncate text-right font-medium text-foreground">
                                  {resourceSniffStatusLabel}
                                </span>
                                <Button
                                  type="button"
                                  variant="outline"
                                  size="compact"
                                  className="h-7 shrink-0 px-2"
                                  onClick={() => void handleStartResourceSniff()}
                                  disabled={startResourceSniff.isPending}
                                >
                                  {startResourceSniff.isPending ? (
                                    <Loader2 className="h-3.5 w-3.5 animate-spin" />
                                  ) : null}
                                  {text.dialogs.restartSniffShort}
                                </Button>
                              </div>
                            </DialogRow>
                            {showResourceSniffCurrentPage ? (
                              <DialogRow className="app-new-task-row flex items-center justify-between gap-4 p-3 text-sm">
                                <span className="text-muted-foreground">
                                  {text.dialogs.currentPage}
                                </span>
                                <span
                                  className="max-w-[62%] truncate text-right text-xs text-muted-foreground"
                                  title={
                                    resourceSniffHasActiveTab
                                      ? resourceSniffSession?.currentUrl
                                      : undefined
                                  }
                                >
                                  {resourceSniffCurrentPageLabel}
                                </span>
                              </DialogRow>
                            ) : null}
                            {showResourceSniffCurrentUser ? (
                              <DialogRow className="app-new-task-row flex items-center justify-between gap-4 p-3 text-sm">
                                <span className="text-muted-foreground">
                                  {text.dialogs.currentUser}
                                </span>
                                <span
                                  className="max-w-[62%] truncate text-right text-xs text-muted-foreground"
                                  title={resourceSniffCurrentUserLabel}
                                >
                                  {resourceSniffCurrentUserLabel}
                                </span>
                              </DialogRow>
                            ) : null}
                          </DialogListCardContent>
                        </DialogListCard>
                      ) : null}

                      <div className="flex flex-col items-center justify-center gap-2 py-4">
                        {resourceSniffSessionId ? (
                          <div className="flex flex-wrap items-center justify-center gap-2">
                            <Button
                              type="button"
                              size="compact"
                              onClick={() =>
                                resourceSniffHasActiveTab
                                  ? void handleParseResourceSniff()
                                  : void handleStartResourceSniff()
                              }
                              disabled={
                                parseResourceSniff.isPending ||
                                startResourceSniff.isPending ||
                                (resourceSniffHasActiveTab &&
                                  resourceSniffNeedsDesk)
                              }
                            >
                              {parseResourceSniff.isPending ||
                              startResourceSniff.isPending ? (
                                <Loader2 className="h-4 w-4 animate-spin" />
                              ) : null}
                              {resourceSniffHasActiveTab
                                ? resourceSniffHasIssue && !resourceSniffNeedsDesk
                                  ? text.dialogs.parseAgain
                                  : text.dialogs.parse
                                : text.dialogs.restartSniffShort}
                            </Button>
                            {resourceSniffNeedsDesk ? (
                              <Button
                                type="button"
                                size="compact"
                                variant="outline"
                                onClick={handleOpenSniffDesk}
                                disabled={!props.onOpenSniffDesk}
                              >
                                <Radar className="h-4 w-4" />
                                {text.dialogs.enterSniffDesk}
                              </Button>
                            ) : null}
                          </div>
                        ) : (
                          <Button
                            type="button"
                            size="compact"
                            onClick={() => void handleStartResourceSniff()}
                            disabled={startResourceSniff.isPending}
                          >
                            {startResourceSniff.isPending ? (
                              <Loader2 className="h-4 w-4 animate-spin" />
                            ) : null}
                            {text.dialogs.startSniff}
                          </Button>
                        )}
                        {resourceSniffUnsupportedDomainNotice ? (
                          <div className="flex w-full justify-center">
                            {resourceSniffUnsupportedDomainNotice}
                          </div>
                        ) : null}
                        {resourceSniffHasIssue ? (
                          <div className="w-full">
                            <div className="app-new-task-parse-error w-full min-w-0 px-3 py-2 text-xs font-medium">
                              {resourceSniffIssueDescription}
                            </div>
                          </div>
                        ) : null}
                      </div>
                    </div>
                  ) : (
                    <div className="flex flex-col items-center justify-center gap-2 py-4">
                      <Button
                        type="button"
                        size="compact"
                        onClick={() => void handleParseDownload()}
                        disabled={parseDownload.isPending}
                      >
                        {parseDownload.isPending ? (
                          <Loader2 className="h-4 w-4 animate-spin" />
                        ) : null}
                        {customParseError
                          ? text.dialogs.parseAgain
                          : text.dialogs.parse}
                      </Button>
                      {customParseError ? (
                        <div className="w-full">
                          <div
                            className="app-new-task-parse-error w-full min-w-0 truncate px-3 py-2 text-xs font-medium"
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
                  <div className="app-dream-status-message px-3 py-2 text-xs" data-intent="danger">
                    {downloadSubmitError}
                  </div>
                ) : null}
              </>
            ) : null}

            {activeMode === "transcode" && !transcodeInputPath ? (
              <DialogListCard className="app-new-task-panel">
                <DialogListCardContent className="flex justify-center p-4">
                  <Button
                    type="button"
                    size="compact"
                    onClick={() => void handleChooseFile()}
                  >
                    <FolderOpen className="h-4 w-4" />
                    {text.actions.chooseFile}
                  </Button>
                </DialogListCardContent>
              </DialogListCard>
            ) : null}

            {activeMode === "transcode" && transcodeInputPath ? (
              <>
                <DialogListCard className="app-new-task-panel">
                  <DialogListCardContent className="space-y-2 p-4">
                    <div className="flex min-w-0 items-center gap-2 text-xs font-medium text-muted-foreground">
                      <span className="flex min-w-0 flex-1 items-baseline">
                        <span className="min-w-0 truncate text-foreground">
                          {transcodeFileName.stem}
                        </span>
                        {transcodeFileName.extension ? (
                          <span className="shrink-0 text-muted-foreground">
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
                        className="h-full min-w-0 flex-1 truncate rounded-none border-0 bg-transparent py-0 text-xs leading-none shadow-none"
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
                    <DialogListCardContent className="flex min-h-12 items-center gap-3 p-3 text-xs">
                      {transcodeProbeChecking ? (
                        <Loader2 className="h-4 w-4 shrink-0 animate-spin text-muted-foreground" />
                      ) : probedTranscodeMediaType === "audio" ? (
                        <AudioLines className="h-4 w-4 shrink-0 text-muted-foreground" />
                      ) : probedTranscodeMediaType === "video" ? (
                        <Video className="h-4 w-4 shrink-0 text-muted-foreground" />
                      ) : (
                        <FileCog className="h-4 w-4 shrink-0 text-muted-foreground" />
                      )}
                      <div className="min-w-0 flex-1">
                        <div className="flex min-w-0 items-center gap-2">
                          {transcodeProbeSummary ? (
                            <span className="min-w-0 truncate font-medium text-foreground">
                              {transcodeProbeSummary}
                            </span>
                          ) : transcodeProbeStatusLabel ? (
                            <span className="min-w-0 truncate font-medium text-foreground">
                              {transcodeProbeStatusLabel}
                            </span>
                          ) : null}
                        </div>
                        {transcodeProbeError ? (
                          <div className="mt-1 truncate text-muted-foreground">
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
                      <DialogRow className="app-new-task-row flex items-center justify-between gap-4 p-3 text-sm">
                        <span className="text-muted-foreground">
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
                      <DialogRow className="app-new-task-row flex items-center justify-between gap-4 p-3 text-sm">
                        <span className="text-muted-foreground">
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
                      <DialogRow className="app-new-task-row flex items-center justify-between gap-4 p-3 text-sm">
                        <span className="text-muted-foreground">
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
                  <div className="app-dream-status-message px-3 py-2 text-xs" data-intent="warning">
                    {text.dependencies.missingDependency.replace(
                      "{name}",
                      "ffmpeg",
                    )}
                  </div>
                ) : null}
                {transcodeNoCompatiblePreset ? (
                  <div className="app-dream-status-message px-3 py-2 text-xs" data-intent="warning">
                    {text.dialogs.noCompatibleTranscodePreset}
                  </div>
                ) : null}
                {transcodeSubmitError ? (
                  <div className="app-dream-status-message px-3 py-2 text-xs" data-intent="danger">
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
            {downloadTab === "sniff" && customParseResult?.resourceSessionId ? (
              <Button
                type="button"
                variant="ghost"
                onClick={() => void handleParseResourceSniff()}
                disabled={
                  downloadRequiresSniffDesk ||
                  parseResourceSniff.isPending ||
                  !resourceSniffSessionId.trim() ||
                  !resourceSniffHasActiveTab
                }
              >
                {parseResourceSniff.isPending ? (
                  <Loader2 className="h-4 w-4 animate-spin" />
                ) : null}
                {text.dialogs.parseAgain}
              </Button>
            ) : null}
            {downloadTab === "sniff" && customParseResult?.resourceMediaId ? (
              <Button
                type="button"
                variant="ghost"
                onClick={() => void handleQueueResourceDownload()}
                disabled={
                  downloadRequiresSniffDesk ||
                  queueYTDLP.isPending ||
                  createYTDLP.isPending ||
                  !downloadPrepared ||
                  !customSelectedFormat
                }
              >
                {queueYTDLP.isPending ? (
                  <Loader2 className="h-4 w-4 animate-spin" />
                ) : null}
                {text.actions.addToDownloadQueue}
              </Button>
            ) : null}
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
                downloadRequiresSniffDesk ||
                createYTDLP.isPending ||
                createYTDLPBatch.isPending ||
                queueYTDLP.isPending ||
                !downloadPrepared ||
                !ytdlpInstalled ||
                (downloadIsBatch && batchDownloadItems.length === 0) ||
                ((downloadTab === "custom" || downloadTab === "sniff") &&
                  (!customParseResult || !customSelectedFormat))
              }
            >
              {createYTDLP.isPending || createYTDLPBatch.isPending ? (
                <Loader2 className="h-4 w-4 animate-spin" />
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
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : null}
              {text.actions.startTask}
            </Button>
          </DialogFooter>
        ) : null}
      </DialogContent>
    </Dialog>
  );
}
