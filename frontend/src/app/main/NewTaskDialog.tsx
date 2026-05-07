import {
ArrowRight,
Download,
FileVideo,
FolderOpen,
Loader2,
Pencil,
SlidersHorizontal,
Zap
} from "lucide-react";
import * as React from "react";

import {
ConnectorBrandIcon
} from "@/features/settings/connectors";
import {
getXiaText
} from "@/features/xiadown/shared";
import type {
ParseYTDLPDownloadResponse,
PrepareYTDLPDownloadResponse
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
useTranscodePresets,
useTranscodePresetsForDownload
} from "@/shared/query/library";
import { Badge } from "@/shared/ui/badge";
import { Button } from "@/shared/ui/button";
import {
Dialog,
DialogContent,
DialogDescription,
DialogFooter,
DialogHeader,
DialogTitle
} from "@/shared/ui/dialog";
import { DreamSegmentSwitch } from "@/shared/ui/dream-segment-switch";
import { Input } from "@/shared/ui/input";
import { Select } from "@/shared/ui/select";
import { Tooltip,TooltipContent,TooltipTrigger } from "@/shared/ui/tooltip";
import { openFileDialog } from "@/shared/utils/dialogHelpers";
import { resolveDialogPath } from "@/shared/utils/resourceHelpers";

import { clampProgress,DependencyRepairCard } from "@/app/main/dependency-repair-card";
import { resolveUnknownErrorMessage } from "@/app/main/helpers";
import { TASK_DIALOG_DEPENDENCIES } from "@/app/main/main-constants";
import { applyTranscodePresetSelection,buildTranscodeCodecKey,filterTranscodePresetsForMediaType,formatSubtitleLabel,inferMediaTypeFromPath,pickDefaultFormat,pickDefaultTranscodePreset,resolveFileFormatLabel,resolveFormatMediaType,resolveOpenFileName,resolvePreparedConnectorType,resolveTranscodeCodecLabel,resolveTranscodeScaleLabel,resolveTranscodeScaleValue,selectAudioFormatId,splitFileNameForDisplay,uniqueOptions } from "@/app/main/new-task-dialog-helpers";
import type { DownloadDialogStep,DownloadDialogTab,DownloadQuality,NewTaskDialogMode,SourceMediaType } from "@/app/main/types";

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

export function NewTaskDialog(props: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  initialMode?: NewTaskDialogMode;
  initialUrl?: string;
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
  const transcodePresets = React.useMemo(
    () =>
      filterTranscodePresetsForMediaType(
        presetsQuery.data ?? [],
        transcodeMediaType,
      ),
    [presetsQuery.data, transcodeMediaType],
  );
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
  const transcodeFileFormat = resolveFileFormatLabel(transcodeInputPath);
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

  React.useEffect(() => {
    if (!props.open) {
      return;
    }
    setActiveMode(props.initialMode ?? "download");
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
    setTranscodeInputPath("");
    setTranscodePresetId("");
    setTranscodeScale("");
    setTranscodeContainer("");
    setTranscodeCodec("");
    setTranscodeSubmitError("");
    autoPreparedInitialUrlRef.current = "";
  }, [props.initialMode, props.initialUrl, props.open]);

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
      AllowsOtherFiletypes: true,
      CanChooseFiles: true,
      CanChooseDirectories: false,
    });
    const path = resolveDialogPath(selection);
    if (!path) {
      return;
    }
    const mediaType = inferMediaTypeFromPath(path);
    const defaultPreset = pickDefaultTranscodePreset(
      presetsQuery.data ?? [],
      mediaType,
    );
    setTranscodeInputPath(path);
    setTranscodePresetId(defaultPreset?.id ?? "");
    applyTranscodePresetSelection(defaultPreset, {
      setScale: setTranscodeScale,
      setContainer: setTranscodeContainer,
      setCodec: setTranscodeCodec,
    });
    setTranscodeSubmitError("");
  };

  const handleCreateTranscode = async () => {
    const inputPath = transcodeInputPath.trim();
    if (!inputPath) {
      return;
    }
    setTranscodeSubmitError("");
    try {
      await createTranscode.mutateAsync({
        inputPath,
        title: resolveOpenFileName(inputPath),
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
                icon: <FileVideo className="h-3.5 w-3.5" />,
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
          <div className="max-h-[min(68vh,34rem)] space-y-4 overflow-x-hidden overflow-y-auto pr-1">
            {activeMode === "download" && downloadStep === "input" ? (
              <div className="app-new-task-panel p-4">
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
              </div>
            ) : null}

            {activeMode === "download" && downloadStep === "config" ? (
              <>
                <div className="app-new-task-panel space-y-2 p-4">
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
                </div>

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
                  <div className="app-new-task-panel app-new-task-list-panel">
                    <div>
                      <div className="app-new-task-row flex items-center justify-between gap-4 p-3 text-sm">
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
                      </div>
                      <div className="app-new-task-row flex items-center justify-between gap-4 p-3 text-sm">
                        <span className="text-muted-foreground">
                          {text.dialogs.subtitles}
                        </span>
                        <InlineSwitch
                          checked={quickSubtitle}
                          onChange={setQuickSubtitle}
                          ariaLabel={text.dialogs.subtitles}
                        />
                      </div>
                      <div className="app-new-task-row flex items-center justify-between gap-4 p-3 text-sm">
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
                      </div>
                      {quickPresetId ? (
                        <div className="app-new-task-row flex items-center justify-between gap-4 p-3 text-sm">
                          <div className="text-muted-foreground">
                            {text.dialogs.keepOnlyTranscodedFile}
                          </div>
                          <InlineSwitch
                            checked={downloadKeepOnlyTranscodedFile}
                            onChange={setDownloadKeepOnlyTranscodedFile}
                            ariaLabel={text.dialogs.keepOnlyTranscodedFile}
                          />
                        </div>
                      ) : null}
                    </div>
                  </div>
                ) : null}

                {downloadTab === "custom" ? (
                  customParseResult ? (
                    <div className="app-new-task-panel app-new-task-list-panel min-w-0 overflow-hidden">
                      <div>
                        <div className="app-new-task-row app-new-task-select-row p-3 text-sm">
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
                        </div>
                        <div className="app-new-task-row app-new-task-select-row p-3 text-sm">
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
                        </div>
                        <div className="app-new-task-row app-new-task-select-row p-3 text-sm">
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
                        </div>
                        {customPresetId ? (
                          <div className="app-new-task-row flex items-center justify-between gap-4 p-3 text-sm">
                            <div className="text-muted-foreground">
                              {text.dialogs.keepOnlyTranscodedFile}
                            </div>
                            <InlineSwitch
                              checked={downloadKeepOnlyTranscodedFile}
                              onChange={setDownloadKeepOnlyTranscodedFile}
                              ariaLabel={text.dialogs.keepOnlyTranscodedFile}
                            />
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
              <div className="app-new-task-panel flex justify-center p-4">
                <Button
                  type="button"
                  size="compact"
                  onClick={() => void handleChooseFile()}
                >
                  <FolderOpen className="h-4 w-4" />
                  {text.actions.chooseFile}
                </Button>
              </div>
            ) : null}

            {activeMode === "transcode" && transcodeInputPath ? (
              <>
                <div className="app-new-task-panel space-y-2 p-4">
                  <div className="flex min-w-0 items-center gap-2 text-xs font-medium text-muted-foreground">
                    <span className="app-new-task-file-format flex h-5 min-w-10 shrink-0 items-center justify-center px-1.5 text-[9px] font-semibold leading-none">
                      {transcodeFileFormat}
                    </span>
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
                </div>

                <div className="app-new-task-panel app-new-task-list-panel">
                  <div>
                    <div className="app-new-task-row flex items-center justify-between gap-4 p-3 text-sm">
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
                    </div>
                    <div className="app-new-task-row flex items-center justify-between gap-4 p-3 text-sm">
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
                    </div>
                    <div className="app-new-task-row flex items-center justify-between gap-4 p-3 text-sm">
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
                    </div>
                  </div>
                </div>
                {!ffmpegInstalled ? (
                  <div className="app-dream-status-message px-3 py-2 text-xs" data-intent="warning">
                    {text.dependencies.missingDependency.replace(
                      "{name}",
                      "ffmpeg",
                    )}
                  </div>
                ) : null}
                {transcodeSubmitError ? (
                  <div className="app-dream-status-message px-3 py-2 text-xs" data-intent="danger">
                    {transcodeSubmitError}
                  </div>
                ) : null}
              </>
            ) : null}
          </div>
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
