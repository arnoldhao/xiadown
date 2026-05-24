import {
  AudioLines,
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

import { ConnectorBrandIcon } from "@/features/settings/connectors";
import { getXiaText } from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
import type {
  CreateYTDLPJobRequest,
  LibraryMediaInfoDTO,
  ParseYTDLPDownloadResponse,
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
import { DreamSegmentSwitch } from "@/shared/ui/dream-segment-switch";
import { Input } from "@/shared/ui/input";
import { Select } from "@/shared/ui/select";
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
  parseAppErrorMessage,
  resolveUnknownErrorMessage,
} from "@/app/main/helpers";
import { TASK_DIALOG_DEPENDENCIES } from "@/app/main/main-constants";
import {
  applyTranscodePresetSelection,
  buildTranscodeCodecKey,
  filterTranscodePresetsForMediaType,
  formatSubtitleLabel,
  inferMediaTypeFromPath,
  pickDefaultFormat,
  resolveResourceSniffStartResolution,
  resolveFormatMediaType,
  resolveOpenFileName,
  resolvePreparedConnectorType,
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

function usesProfileConnector(prepared: PrepareYTDLPDownloadResponse | null) {
  return (
    (prepared?.connectorCredentialMode ?? "").trim().toLowerCase() ===
    "profile"
  );
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
  const [downloadUseConnector, setDownloadUseConnector] = React.useState(false);
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
  const resourceSniffTransferRequestVersionRef = React.useRef<number | null>(
    null,
  );
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
  const preparedDownloadUrl = (downloadPrepared?.url ?? downloadUrl).trim();
  const downloadUrlDisplayParts = buildDownloadUrlDisplayParts(
    preparedDownloadUrl,
    downloadPrepared?.domain,
  );
  const downloadConnectorType = resolvePreparedConnectorType(downloadPrepared);
  const downloadConnectorMode = (
    downloadPrepared?.connectorCredentialMode ?? ""
  )
    .trim()
    .toLowerCase();
  const downloadUsesProfileConnector = usesProfileConnector(downloadPrepared);
  const downloadShowsSniffMode =
    downloadUsesProfileConnector || downloadTab === "sniff";
  const downloadMatchesCookieConnector =
    Boolean(downloadPrepared?.connectorId?.trim()) &&
    downloadConnectorMode === "cookies";
  const downloadCookieConnectorState = !downloadMatchesCookieConnector
    ? "unmatched"
    : downloadPrepared?.connectorAvailable
      ? "available"
      : "unavailable";
  const downloadConnectorCanUseCookies =
    downloadCookieConnectorState === "available" &&
    !downloadUsesProfileConnector;
  const downloadConnectorStatusLabel =
    downloadCookieConnectorState === "available"
      ? text.dialogs.connectorCanEnable
      : downloadCookieConnectorState === "unavailable"
        ? text.dialogs.connectorNotConfigured
        : text.dialogs.noAvailableConnector;
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
  const customParseErrorDescription =
    resolveParseErrorDescription(
      customParseError,
      downloadUseConnector && downloadConnectorCanUseCookies
        ? text.dialogs.parseFailedWithConnector
        : text.dialogs.parseFailedWithoutConnector,
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
      activeMode === "download" &&
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
    (downloadTab === "quick" ||
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
    setDownloadUseConnector(false);
    setDownloadTab("quick");
    setDownloadPrepareError("");
    setDownloadPrepareIntent(null);
    setDownloadSubmitError("");
    setQuickQuality("best");
    setQuickSubtitle(false);
    setQuickPresetId("");
    setDownloadKeepOnlyTranscodedFile(true);
    setCustomParseResult(null);
    setCustomFormatId("");
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
    void cancelActiveResourceSniff();
    parseRequestVersionRef.current += 1;
    parseDownload.reset();
    parseResourceSniff.reset();
    setDownloadPrepared(null);
    setDownloadStep("input");
    setDownloadUseConnector(false);
    setDownloadTab("quick");
    setDownloadPrepareIntent(null);
    setDownloadSubmitError("");
    setDownloadKeepOnlyTranscodedFile(true);
    setCustomParseResult(null);
    setCustomFormatId("");
    setCustomSubtitleId("");
    setCustomPresetId("");
    setCustomParseError("");
    setActiveResourceSniffSessionId("");
    setResourceSniffFailure(null);
    setResourceSniffTechnicalError("");
  };

  const resetParsedDownloadSelection = () => {
    parseRequestVersionRef.current += 1;
    parseDownload.reset();
    parseResourceSniff.reset();
    setCustomParseResult(null);
    setCustomFormatId("");
    setCustomSubtitleId("");
    setCustomPresetId("");
    setCustomParseError("");
  };

  const startPreparedResourceSniff = React.useCallback(
    async (prepared: PrepareYTDLPDownloadResponse) => {
      const existingSessionID = resourceSniffSessionIdRef.current.trim();
      if (existingSessionID) {
        await cancelResourceSniffSession(existingSessionID);
      }
      resetParsedDownloadSelection();
      const requestVersion = parseRequestVersionRef.current;
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
              requestVersion,
              currentVersion: parseRequestVersionRef.current,
              dialogOpen: dialogOpenRef.current,
              transferRequestVersion:
                resourceSniffTransferRequestVersionRef.current,
            }) === "preserve"
          ) {
            resourceSniffTransferRequestVersionRef.current = null;
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
          requestVersion,
          currentVersion: parseRequestVersionRef.current,
          dialogOpen: dialogOpenRef.current,
          transferRequestVersion: resourceSniffTransferRequestVersionRef.current,
        });
        if (startResolution === "preserve") {
          resourceSniffTransferRequestVersionRef.current = null;
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
            requestVersion,
            currentVersion: parseRequestVersionRef.current,
            dialogOpen: dialogOpenRef.current,
            transferRequestVersion:
              resourceSniffTransferRequestVersionRef.current,
          }) === "preserve"
        ) {
          resourceSniffTransferRequestVersionRef.current = null;
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
        const usesProfile = usesProfileConnector(prepared);
        const nextTab: DownloadDialogTab =
          mode === "sniff" || usesProfile ? "sniff" : "quick";
        setDownloadPrepared(prepared);
        setDownloadUrl(prepared.url || url);
        setDownloadUseConnector(
          Boolean(prepared.connectorAvailable && !usesProfile),
        );
        setDownloadStep("config");
        setDownloadTab(nextTab);
        setCustomParseResult(null);
        setCustomFormatId("");
        setCustomSubtitleId("");
        setCustomPresetId("");
        setDownloadKeepOnlyTranscodedFile(true);
        setCustomParseError("");
        if (nextTab === "sniff" && autoStartSniff) {
          await startPreparedResourceSniff(prepared);
        }
      } catch (error) {
        setDownloadPrepareError(
          resolveUnknownErrorMessage(error, text.common.unknown),
        );
      } finally {
        setDownloadPrepareIntent(null);
      }
    },
    [
      downloadUrl,
      prepareDownload,
      startPreparedResourceSniff,
      text.common.unknown,
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
        connectorId: downloadPrepared.connectorId,
        useConnector: downloadUseConnector && downloadConnectorCanUseCookies,
      });
      if (
        requestVersion !== parseRequestVersionRef.current ||
        !dialogOpenRef.current
      ) {
        return;
      }
      if (!hasDownloadableFormats(parsed)) {
        setCustomParseResult(null);
        setCustomFormatId("");
        setCustomSubtitleId("");
        setCustomPresetId("");
        setCustomParseError(noDownloadableMediaErrorMessage());
        return;
      }
      const defaultFormat = pickDefaultFormat(parsed.formats);
      setCustomParseResult(parsed);
      setCustomFormatId(defaultFormat?.id ?? "");
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
    resourceSniffTransferRequestVersionRef.current =
      parseRequestVersionRef.current;
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
        connectorId: downloadPrepared.connectorId || undefined,
        useConnector: downloadUseConnector && downloadConnectorCanUseCookies,
      });
      await closeDialogAfterResourceCleanup();
    } catch (error) {
      setDownloadSubmitError(
        resolveUnknownErrorMessage(error, text.common.unknown),
      );
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
        ? selectAudioFormatId(customFormats) || "bestaudio"
        : undefined,
      subtitleLangs: selectedSubtitleLang ? [selectedSubtitleLang] : undefined,
      subtitleAuto: Boolean(customSelectedSubtitle?.isAuto),
      subtitleFormat: customSelectedSubtitle?.ext || undefined,
      transcodePresetId: customPresetId || undefined,
      deleteSourceFileAfterTranscode: customPresetId
        ? downloadKeepOnlyTranscodedFile
        : undefined,
      connectorId: downloadPrepared.connectorId || undefined,
      useConnector: downloadUseConnector && downloadConnectorCanUseCookies,
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
      setDownloadSubmitError(
        resolveUnknownErrorMessage(error, text.common.unknown),
      );
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
      setDownloadSubmitError(
        resolveUnknownErrorMessage(error, text.common.unknown),
      );
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
            {activeMode === "download"
              ? text.dialogs.downloadTitle
              : text.dialogs.transcodeTitle}
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
                value: "transcode",
                label: text.actions.transcode,
                icon: <FileCog className="h-3.5 w-3.5" />,
              },
            ]}
            onValueChange={setActiveMode}
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
            {activeMode === "download" && downloadStep === "input" ? (
              <DialogListCard className="app-new-task-panel">
                <DialogListCardContent className="p-4">
                  <form
                    className="flex gap-2"
                    onSubmit={(event) => {
                      event.preventDefault();
                      void handlePrepareDownload("direct");
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
                            type="button"
                            size="compactIcon"
                            title={text.dialogs.startSniffMode}
                            aria-label={text.dialogs.startSniffMode}
                            onClick={() => void handlePrepareDownload("sniff")}
                            disabled={
                              !downloadUrl.trim() ||
                              !ytdlpInstalled ||
                              prepareDownload.isPending
                            }
                          >
                            {prepareDownload.isPending &&
                            downloadPrepareIntent === "sniff" ? (
                              <Loader2 className="h-4 w-4 animate-spin" />
                            ) : (
                              <Radar className="h-4 w-4" />
                            )}
                          </Button>
                        </span>
                      </TooltipTrigger>
                      <TooltipContent>
                        {text.dialogs.startSniffMode}
                      </TooltipContent>
                    </Tooltip>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <span className="inline-flex shrink-0">
                          <Button
                            type="submit"
                            size="compactIcon"
                            title={text.dialogs.directDownload}
                            aria-label={text.dialogs.directDownload}
                            disabled={
                              !downloadUrl.trim() ||
                              !ytdlpInstalled ||
                              prepareDownload.isPending
                            }
                          >
                            {prepareDownload.isPending &&
                            downloadPrepareIntent === "direct" ? (
                              <Loader2 className="h-4 w-4 animate-spin" />
                            ) : (
                              <Download className="h-4 w-4" />
                            )}
                          </Button>
                        </span>
                      </TooltipTrigger>
                      <TooltipContent>
                        {text.dialogs.directDownload}
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

            {activeMode === "download" && downloadStep === "config" ? (
              <>
                <DialogListCard className="app-new-task-panel">
                  <DialogListCardContent className="p-3">
                    <div
                      className="app-new-task-field-strip app-new-task-url-card-strip h-9 w-full min-w-0 overflow-hidden"
                      data-mode={downloadShowsSniffMode ? "sniff" : "connector"}
                    >
                      <div className="app-new-task-url-card-link flex h-full min-w-0 items-center">
                        <div
                          className="app-new-task-url-card-url flex h-full min-w-0 flex-1 items-center gap-1.5 px-3"
                          title={preparedDownloadUrl}
                        >
                          <ConnectorBrandIcon
                            connectorType={downloadConnectorType}
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
                          <TooltipContent>{text.dialogs.modifyLink}</TooltipContent>
                        </Tooltip>
                      </div>
                      <div className="app-new-task-url-card-mode-slot relative h-full min-w-0 overflow-hidden">
                        <div
                          className="app-new-task-url-card-mode-panel"
                          data-panel="connector"
                          data-visible={
                            downloadShowsSniffMode ? "false" : "true"
                          }
                          aria-hidden={
                            downloadShowsSniffMode ? "true" : undefined
                          }
                        >
                          <span className="app-new-task-url-card-mode-label">
                            {downloadConnectorStatusLabel}
                          </span>
                          {downloadConnectorCanUseCookies ? (
                            <Tooltip>
                              <TooltipTrigger asChild>
                                <span className="flex shrink-0 items-center justify-center">
                                  <InlineSwitch
                                    checked={downloadUseConnector}
                                    onChange={setDownloadUseConnector}
                                    ariaLabel={text.dialogs.connectorCookiesDownload}
                                  />
                                </span>
                              </TooltipTrigger>
                              <TooltipContent>
                                {text.dialogs.connectorCookiesDownload}
                              </TooltipContent>
                            </Tooltip>
                          ) : downloadMatchesCookieConnector ? (
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

                {!downloadUsesProfileConnector && downloadTab !== "sniff" ? (
                  <div className="flex justify-center">
                    <DreamSegmentSwitch
                      value={downloadTab}
                      className="app-new-task-download-mode-switch"
                      items={downloadTabItems}
                      onValueChange={handleDownloadTabChange}
                    />
                  </div>
                ) : null}

                {downloadTab === "quick" ? (
                  <DialogListCard className="app-new-task-panel app-new-task-list-panel">
                    <DialogListCardContent>
                      <DialogRow className="app-new-task-row flex items-center justify-between gap-4 p-3 text-sm">
                        <span className="text-muted-foreground">
                          {text.dialogs.quality}
                        </span>
                        <div className="flex items-center gap-2">
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
        activeMode === "download" &&
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
                void (downloadTab === "quick"
                  ? handleStartQuickDownload()
                  : handleStartCustomDownload())
              }
              disabled={
                downloadRequiresSniffDesk ||
                createYTDLP.isPending ||
                queueYTDLP.isPending ||
                !downloadPrepared ||
                !ytdlpInstalled ||
                ((downloadTab === "custom" || downloadTab === "sniff") &&
                  (!customParseResult || !customSelectedFormat))
              }
            >
              {createYTDLP.isPending ? (
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
