import {
  ArrowRight,
  AudioLines,
  Download,
  FileCog,
  FolderOpen,
  Loader2,
  Pencil,
  SlidersHorizontal,
  Video,
  Zap,
} from "lucide-react";
import * as React from "react";

import { ConnectorBrandIcon } from "@/features/settings/connectors";
import { getXiaText } from "@/features/xiadown/shared";
import type {
  LibraryMediaInfoDTO,
  ParseYTDLPDownloadResponse,
  PrepareYTDLPDownloadResponse,
  ProbeTranscodeInputRequest,
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
  useParseYTDLPDownload,
  usePrepareYTDLPDownload,
  useProbeTranscodeInput,
  useTranscodePresets,
  useTranscodePresetsForDownload,
} from "@/shared/query/library";
import { Badge } from "@/shared/ui/badge";
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
  const parsed = parseJSONFromErrorMessage(message);
  if (parsed !== null) {
    return JSON.stringify(parsed, null, 2);
  }
  return message.trim();
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
  const createYTDLP = useCreateYTDLPJob();
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
  const [customSubtitleId, setCustomSubtitleId] = React.useState("");
  const [customPresetId, setCustomPresetId] = React.useState("");
  const [customParseError, setCustomParseError] = React.useState("");
  const [transcodeInputPath, setTranscodeInputPath] = React.useState("");
  const [transcodeSourceFileId, setTranscodeSourceFileId] = React.useState("");
  const [transcodeSourceTitle, setTranscodeSourceTitle] = React.useState("");
  const [transcodeSourceAuthor, setTranscodeSourceAuthor] = React.useState("");
  const [transcodePresetId, setTranscodePresetId] = React.useState("");
  const [transcodeScale, setTranscodeScale] = React.useState("");
  const [transcodeContainer, setTranscodeContainer] = React.useState("");
  const [transcodeCodec, setTranscodeCodec] = React.useState("");
  const [transcodeSubmitError, setTranscodeSubmitError] = React.useState("");
  const autoPreparedInitialUrlRef = React.useRef("");

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
  const customParseErrorDescription =
    downloadUseConnector && downloadPrepared?.connectorAvailable
      ? text.dialogs.parseFailedWithConnector
      : text.dialogs.parseFailedWithoutConnector;
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
  const downloadDomainLabel = (
    downloadPrepared?.domain ||
    downloadPrepared?.url ||
    downloadUrl
  )
    .trim()
    .toUpperCase();
  const downloadConnectorType = resolvePreparedConnectorType(downloadPrepared);
  const showDownloadFooter =
    downloadStep === "config" &&
    (downloadTab === "quick" ||
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
    setDownloadUseConnector(false);
    setDownloadTab("quick");
    setDownloadPrepareError("");
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
    setDownloadPrepared(null);
    setDownloadStep("input");
    setDownloadUseConnector(false);
    setDownloadTab("quick");
    setDownloadSubmitError("");
    setDownloadKeepOnlyTranscodedFile(true);
    setCustomParseResult(null);
    setCustomFormatId("");
    setCustomSubtitleId("");
    setCustomPresetId("");
    setCustomParseError("");
  };

  const handlePrepareDownload = React.useCallback(async (overrideUrl?: string) => {
    const url = (overrideUrl ?? downloadUrl).trim();
    if (!url) {
      return;
    }
    setDownloadPrepareError("");
    setDownloadSubmitError("");
    try {
      const prepared = await prepareDownload.mutateAsync({ url });
      setDownloadPrepared(prepared);
      setDownloadUrl(prepared.url || url);
      setDownloadUseConnector(Boolean(prepared.connectorAvailable));
      setDownloadStep("config");
      setDownloadTab("quick");
      setCustomParseResult(null);
      setCustomFormatId("");
      setCustomSubtitleId("");
      setCustomPresetId("");
      setDownloadKeepOnlyTranscodedFile(true);
      setCustomParseError("");
    } catch (error) {
      setDownloadPrepareError(
        resolveUnknownErrorMessage(error, text.common.unknown),
      );
    }
  }, [text.common.unknown, downloadUrl, prepareDownload]);

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
    setCustomParseError("");
    try {
      const parsed = await parseDownload.mutateAsync({
        url: downloadPrepared.url,
        connectorId: downloadPrepared.connectorId,
        useConnector:
          downloadUseConnector && downloadPrepared.connectorAvailable,
      });
      const defaultFormat = pickDefaultFormat(parsed.formats);
      setCustomParseResult(parsed);
      setCustomFormatId(defaultFormat?.id ?? "");
      setCustomSubtitleId("");
      setCustomPresetId("");
    } catch (error) {
      setCustomParseError(
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
        useConnector:
          downloadUseConnector && downloadPrepared.connectorAvailable,
      });
      props.onOpenChange(false);
    } catch (error) {
      setDownloadSubmitError(
        resolveUnknownErrorMessage(error, text.common.unknown),
      );
    }
  };

  const handleStartCustomDownload = async () => {
    if (!downloadPrepared || !customParseResult || !customSelectedFormat) {
      return;
    }
    const selectedSubtitleLang = customSelectedSubtitle?.language?.trim() ?? "";
    const needsAudioJoin =
      customSelectedFormat.hasVideo && !customSelectedFormat.hasAudio;
    setDownloadSubmitError("");
    try {
      await createYTDLP.mutateAsync({
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
        subtitleLangs: selectedSubtitleLang
          ? [selectedSubtitleLang]
          : undefined,
        subtitleAuto: Boolean(customSelectedSubtitle?.isAuto),
        subtitleFormat: customSelectedSubtitle?.ext || undefined,
        transcodePresetId: customPresetId || undefined,
        deleteSourceFileAfterTranscode: customPresetId
          ? downloadKeepOnlyTranscodedFile
          : undefined,
        connectorId: downloadPrepared.connectorId || undefined,
        useConnector:
          downloadUseConnector && downloadPrepared.connectorAvailable,
      });
      props.onOpenChange(false);
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
      props.onOpenChange(false);
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
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent
        className="max-w-[min(92vw,32rem)] gap-4 overflow-hidden"
        showCloseButton
      >
        <DialogHeader className="space-y-0 text-left">
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
                      void handlePrepareDownload();
                    }}
                  >
                    <Input
                      value={downloadUrl}
                      onChange={(event) => setDownloadUrl(event.target.value)}
                      placeholder={text.dialogs.downloadPlaceholder}
                      className="min-w-0 flex-1"
                    />
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <span className="inline-flex shrink-0">
                          <Button
                            type="submit"
                            size="compactIcon"
                            title={text.dialogs.requestDownload}
                            aria-label={text.dialogs.requestDownload}
                            disabled={
                              !downloadUrl.trim() ||
                              !ytdlpInstalled ||
                              prepareDownload.isPending
                            }
                          >
                            {prepareDownload.isPending ? (
                              <Loader2 className="h-4 w-4 animate-spin" />
                            ) : (
                              <ArrowRight className="h-4 w-4" />
                            )}
                          </Button>
                        </span>
                      </TooltipTrigger>
                      <TooltipContent>{text.dialogs.requestDownload}</TooltipContent>
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
                  <DialogListCardContent className="space-y-2 p-4">
                    <div className="flex min-w-0 items-center gap-2 text-xs font-medium text-muted-foreground">
                      {downloadConnectorType ? (
                        <ConnectorBrandIcon
                          connectorType={downloadConnectorType}
                          fallback="none"
                          className="h-3.5 w-3.5 shrink-0"
                        />
                      ) : null}
                      <span className="truncate">{downloadDomainLabel}</span>
                      {downloadPrepared?.reachable === false ? (
                        <Badge
                          variant="outline"
                          className="app-dream-status-badge-warning"
                        >
                          {text.common.unknown}
                        </Badge>
                      ) : null}
                    </div>
                    <div className="app-new-task-field-strip flex h-9 w-full min-w-0 items-center overflow-hidden">
                      <Input
                        size="default"
                        value={downloadPrepared?.url ?? downloadUrl}
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
                      <div className="app-new-task-field-switch-slot flex h-full w-12 shrink-0 items-center justify-center">
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <span className="flex items-center justify-center">
                              <InlineSwitch
                                checked={
                                  downloadPrepared?.connectorAvailable
                                    ? downloadUseConnector
                                    : false
                                }
                                onChange={(checked) => {
                                  if (downloadPrepared?.connectorAvailable) {
                                    setDownloadUseConnector(checked);
                                  }
                                }}
                                ariaLabel={text.dialogs.useConnector}
                                disabled={!downloadPrepared?.connectorAvailable}
                              />
                            </span>
                          </TooltipTrigger>
                          <TooltipContent>
                            {downloadPrepared?.connectorAvailable
                              ? text.dialogs.connectorAvailable
                              : text.dialogs.connectorUnavailable}
                          </TooltipContent>
                        </Tooltip>
                      </div>
                    </div>
                  </DialogListCardContent>
                </DialogListCard>

                <div className="flex justify-center">
                  <DreamSegmentSwitch
                    value={downloadTab}
                    className="app-new-task-download-mode-switch"
                    items={[
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
                    ]}
                    onValueChange={setDownloadTab}
                  />
                </div>

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

                {downloadTab === "custom" ? (
                  customParseResult ? (
                    <DialogListCard className="app-new-task-panel app-new-task-list-panel min-w-0 overflow-hidden">
                      <DialogListCardContent>
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
                            <option value="">{text.dialogs.noSubtitle}</option>
                            {customSubtitles.map((subtitle) => (
                              <option key={subtitle.id} value={subtitle.id}>
                                {formatSubtitleLabel(subtitle)}
                              </option>
                            ))}
                          </Select>
                        </DialogRow>
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
                          <div className="app-new-task-parse-error w-full min-w-0 text-xs">
                            <div className="border-b border-destructive/20 px-3 py-2 font-medium">
                              {customParseErrorDescription}
                            </div>
                            <pre className="max-h-24 overflow-y-auto whitespace-pre-wrap break-words px-3 py-2 font-mono text-[11px] leading-4 text-muted-foreground">{customParseErrorDetail}</pre>
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
              onClick={() => props.onOpenChange(false)}
            >
              {text.actions.cancelDialog}
            </Button>
            <Button
              type="button"
              onClick={() =>
                void (downloadTab === "quick"
                  ? handleStartQuickDownload()
                  : handleStartCustomDownload())
              }
              disabled={
                createYTDLP.isPending ||
                !downloadPrepared ||
                !ytdlpInstalled ||
                (downloadTab === "custom" &&
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
              onClick={() => props.onOpenChange(false)}
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
