import { Activity, AlertCircle, CheckCircle2, Loader2, Power, ShieldAlert } from "lucide-react";

import { getXiaText } from "@/features/xiadown/shared";
import {
  DEFAULT_EQUALIZER_ARTWORK_VISUALIZER_MODE,
  DEFAULT_EQUALIZER_SPECTRUM_VISUALIZER_MODE,
  EQUALIZER_ARTWORK_VISUALIZER_MODES,
  EQUALIZER_SPECTRUM_VISUALIZER_MODES,
  type EqualizerSettings,
  type EqualizerStatusCode,
  type EqualizerVisualizerMode,
  type EqualizerVisualizerPlacement,
} from "@/shared/contracts/equalizer";
import {
  useApplyEqualizerPreset,
  useEqualizerSnapshot,
  useOpenEqualizerPermissionGuide,
  useResetEqualizer,
  useRetryEqualizer,
  useSetEqualizerBandGain,
  useSetEqualizerEnabled,
  useSetEqualizerPreamp,
  useSetEqualizerVisualizerMode,
} from "@/shared/query/equalizer";
import { Button } from "@/shared/ui/button";
import { DreamInlineSwitch } from "@/shared/ui/dream-inline-switch";
import { Select } from "@/shared/ui/select";
import {
  StatusBadge as DreamStatusBadge,
  type DreamStatusTone,
} from "@/shared/ui/status-badge";
import {
  SettingsCompactListCard,
  SettingsCompactRow,
  SettingsCompactSeparator,
} from "@/shared/ui/settings-layout";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/shared/ui/tooltip";

import { EqualizerControlCards } from "./EqualizerControlCards";

const VISUALIZER_PLACEMENT_OPTIONS: readonly EqualizerVisualizerPlacement[] = ["artwork", "spectrum", "off"];

export function EqualizerSection(props: {
  isMac: boolean;
  isWindows: boolean;
  text: ReturnType<typeof getXiaText>;
}) {
  const { isMac, isWindows, text } = props;
  const equalizerText = text.settings.equalizer;
  const supportsVisualizerSettings = isMac || isWindows;
  const query = useEqualizerSnapshot(supportsVisualizerSettings);
  const setEnabled = useSetEqualizerEnabled();
  const applyPreset = useApplyEqualizerPreset();
  const setBandGain = useSetEqualizerBandGain();
  const setPreamp = useSetEqualizerPreamp();
  const setVisualizerMode = useSetEqualizerVisualizerMode();
  const resetEqualizer = useResetEqualizer();
  const retryEqualizer = useRetryEqualizer();
  const openPermissionGuide = useOpenEqualizerPermissionGuide();
  const isMutating =
    setEnabled.isPending ||
    applyPreset.isPending ||
    setBandGain.isPending ||
    setPreamp.isPending ||
    setVisualizerMode.isPending ||
    resetEqualizer.isPending ||
    retryEqualizer.isPending ||
    openPermissionGuide.isPending;

  if (!supportsVisualizerSettings) {
    return (
      <SettingsCompactListCard>
        <SettingsCompactRow label={equalizerText.title} contentClassName="min-w-0">
          <div className="app-settings-feedback flex min-w-0 items-center justify-end gap-2">
            <ShieldAlert className="h-4 w-4 shrink-0" />
            <span className="app-equalizer-availability-label min-w-0 truncate">{equalizerText.macOSOnly}</span>
          </div>
        </SettingsCompactRow>
      </SettingsCompactListCard>
    );
  }

  if (query.isLoading && !query.data) {
    return (
      <SettingsCompactListCard>
        <SettingsCompactRow label={equalizerText.title}>
          <div className="app-settings-feedback flex items-center gap-2">
            <Loader2 className="h-4 w-4 app-motion-spin" />
            {text.settings.checking}
          </div>
        </SettingsCompactRow>
      </SettingsCompactListCard>
    );
  }

  const snapshot = query.data;
  if (!snapshot) {
    return (
      <SettingsCompactListCard>
        <SettingsCompactRow label={equalizerText.title}>
          <EqualizerStatusBadge code="error" label={equalizerText.status.error} />
        </SettingsCompactRow>
      </SettingsCompactListCard>
    );
  }

  const settings = snapshot.settings;
  const disabled = !settings.enabled || !snapshot.status.supported;
  const bandGains = snapshot.bands.map((_, index) => settings.bandGainsDb[index] ?? 0);
  const presetOptions =
    settings.preset === "custom"
      ? [...snapshot.presets, { id: "custom", name: equalizerText.custom, gainsDb: settings.bandGainsDb }]
      : snapshot.presets;
  const visualizerPlacement = settings.visualizerPlacement;
  const visualizerEffectModes =
    visualizerPlacement === "artwork"
      ? EQUALIZER_ARTWORK_VISUALIZER_MODES
      : visualizerPlacement === "spectrum"
        ? EQUALIZER_SPECTRUM_VISUALIZER_MODES
        : [];
  const selectedVisualizerEffect =
    visualizerPlacement === "artwork"
      ? settings.artworkVisualizerMode
      : visualizerPlacement === "spectrum"
        ? settings.spectrumVisualizerMode
        : settings.visualizerMode;
  const visualizerControls = (
    <>
      <SettingsCompactRow label={equalizerText.visualizer}>
        <div className="grid min-w-0 max-w-full grid-cols-3 gap-2">
          {VISUALIZER_PLACEMENT_OPTIONS.map((placement) => (
            <Button
              key={placement}
              type="button"
              variant="outline"
              tone={visualizerPlacement === placement ? "accent" : "neutral"}
              size="compact"
              disabled={!snapshot.status.supported || setVisualizerMode.isPending}
              className="app-settings-option-button min-w-0 px-2"
              onClick={() =>
                void setVisualizerMode
                  .mutateAsync(defaultVisualizerModeForPlacement(placement, settings))
                  .catch(console.warn)
              }
            >
              <span className="min-w-0 truncate">{visualizerPlacementLabel(placement, equalizerText)}</span>
            </Button>
          ))}
        </div>
      </SettingsCompactRow>

      {visualizerPlacement !== "off" ? (
        <>
          <SettingsCompactSeparator />

          <SettingsCompactRow
            label={
              visualizerPlacement === "artwork"
                ? equalizerText.visualizerArtworkEffect
                : equalizerText.visualizerSpectrumEffect
            }
          >
            <Select
              value={selectedVisualizerEffect}
              disabled={!snapshot.status.supported || setVisualizerMode.isPending}
              onChange={(event) =>
                void setVisualizerMode.mutateAsync(event.target.value as EqualizerVisualizerMode).catch(console.warn)
              }
              className="w-48"
            >
              {visualizerEffectModes.map((mode) => (
                <option key={mode} value={mode}>
                  {visualizerLabel(mode, equalizerText)}
                </option>
              ))}
            </Select>
          </SettingsCompactRow>
        </>
      ) : null}
    </>
  );

  if (isWindows) {
    return (
      <SettingsCompactListCard>
        {visualizerControls}
        {!snapshot.status.supported || snapshot.status.code === "error" ? (
          <>
            <SettingsCompactSeparator />
            <SettingsCompactRow label={text.settings.status} contentClassName="min-w-0">
              <div className="flex min-w-0 items-center justify-end">
                <EqualizerStatusBadge code={snapshot.status.code} label={statusLabel(snapshot.status.code, equalizerText)} />
              </div>
            </SettingsCompactRow>
          </>
        ) : null}
      </SettingsCompactListCard>
    );
  }

  return (
    <>
      <SettingsCompactListCard>
        <SettingsCompactRow label={equalizerText.enable}>
          <EqualizerSwitch
            checked={settings.enabled}
            disabled={isMutating || !snapshot.status.supported}
            onChange={(checked) => void setEnabled.mutateAsync(checked).catch(console.warn)}
            ariaLabel={equalizerText.enable}
          />
        </SettingsCompactRow>

        <SettingsCompactSeparator />

        <SettingsCompactRow label={text.settings.status} contentClassName="min-w-0">
          <TooltipProvider delayDuration={0}>
            <div className="flex min-w-0 flex-wrap items-center justify-end gap-2">
              <EqualizerStatusBadge
                code={snapshot.status.code}
                label={statusLabel(snapshot.status.code, equalizerText)}
                tooltip={statusBadgeTooltip(snapshot.status.code, equalizerText)}
              />
              {snapshot.status.permissionRequired ? (
                <Button
                  type="button"
                  variant="outline"
                  size="compact"
                  disabled={openPermissionGuide.isPending}
                  onClick={() =>
                    void openPermissionGuide
                      .mutateAsync({
                        permissionName: equalizerText.permissionName,
                        hint: equalizerText.permissionGuideHint,
                      })
                      .catch(console.warn)
                  }
                >
                  {equalizerText.openSettings}
                </Button>
              ) : null}
              {settings.enabled && !snapshot.status.running ? (
                <Button
                  type="button"
                  variant="outline"
                  size="compactIcon"
                  disabled={retryEqualizer.isPending}
                  onClick={() => void retryEqualizer.mutateAsync(undefined).catch(console.warn)}
                  aria-label={equalizerText.retry}
                  title={equalizerText.retry}
                >
                  {retryEqualizer.isPending ? <Loader2 className="h-4 w-4 app-motion-spin" /> : <Activity className="h-4 w-4" />}
                </Button>
              ) : null}
            </div>
          </TooltipProvider>
        </SettingsCompactRow>

        <SettingsCompactSeparator />

        {visualizerControls}

        <SettingsCompactSeparator />

        <SettingsCompactRow label={equalizerText.preset}>
          <Select
            value={settings.preset}
            disabled={disabled || applyPreset.isPending}
            onChange={(event) => void applyPreset.mutateAsync(event.target.value).catch(console.warn)}
            className="w-48"
          >
            {presetOptions.map((preset) => (
              <option key={preset.id} value={preset.id}>
                {presetLabel(preset.id, preset.name, equalizerText)}
              </option>
            ))}
          </Select>
        </SettingsCompactRow>
      </SettingsCompactListCard>

      <EqualizerControlCards
        bands={snapshot.bands}
        bandGainsDb={bandGains}
        disabled={disabled}
        enabled={settings.enabled}
        labels={{
          bands: equalizerText.bands,
          preamp: equalizerText.preamp,
          reset: equalizerText.reset,
        }}
        preampDb={settings.preampDb}
        resetPending={resetEqualizer.isPending}
        onBandGainChange={(index, gainDb) =>
          void setBandGain.mutateAsync({ index, gainDb }).catch(console.warn)
        }
        onPreampChange={(gainDb) =>
          void setPreamp.mutateAsync(gainDb).catch(console.warn)
        }
        onReset={() =>
          void resetEqualizer.mutateAsync(undefined).catch(console.warn)
        }
      />
    </>
  );
}

function EqualizerSwitch(props: {
  checked: boolean;
  disabled?: boolean;
  onChange: (checked: boolean) => void;
  ariaLabel: string;
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

function EqualizerStatusBadge(props: {
  code: EqualizerStatusCode;
  label: string;
  tooltip?: string;
}) {
  const Icon =
    props.code === "active"
      ? CheckCircle2
      : props.code === "off"
        ? Power
        : props.code === "standby"
          ? Activity
          : AlertCircle;
  const tone: DreamStatusTone =
    props.code === "active"
      ? "success"
      : props.code === "permission_needed" || props.code === "error"
        ? "warning"
        : props.code === "standby"
          ? "busy"
          : "muted";
  const badge = (
    <DreamStatusBadge
      icon={<Icon />}
      tone={tone}
    >
      {props.label}
    </DreamStatusBadge>
  );

  if (!props.tooltip) {
    return badge;
  }

  return (
    <Tooltip>
      <TooltipTrigger asChild>{badge}</TooltipTrigger>
      <TooltipContent side="top" multiline className="app-settings-tooltip-text">
        {props.tooltip}
      </TooltipContent>
    </Tooltip>
  );
}

function statusLabel(code: EqualizerStatusCode, text: ReturnType<typeof getXiaText>["settings"]["equalizer"]) {
  switch (code) {
    case "active":
      return text.status.active;
    case "standby":
      return text.status.standby;
    case "permission_needed":
      return text.status.permissionNeeded;
    case "unsupported":
      return text.status.unsupported;
    case "error":
      return text.status.error;
    default:
      return text.status.off;
  }
}

function statusBadgeTooltip(code: EqualizerStatusCode, text: ReturnType<typeof getXiaText>["settings"]["equalizer"]) {
  switch (code) {
    case "standby":
      return text.messages.standby;
    case "permission_needed":
      return text.messages.permissionNeeded;
    case "unsupported":
      return text.messages.unsupported;
    case "error":
      return text.messages.error;
    default:
      return "";
  }
}

function presetLabel(
  id: string,
  fallback: string,
  text: ReturnType<typeof getXiaText>["settings"]["equalizer"],
) {
  return (text.presets as Record<string, string>)[id] ?? fallback;
}

function defaultVisualizerModeForPlacement(
  placement: EqualizerVisualizerPlacement,
  settings: EqualizerSettings,
): EqualizerVisualizerMode {
  if (placement === "off") {
    return "off";
  }
  if (placement === "artwork") {
    return settings.artworkVisualizerMode || DEFAULT_EQUALIZER_ARTWORK_VISUALIZER_MODE;
  }
  return settings.spectrumVisualizerMode || DEFAULT_EQUALIZER_SPECTRUM_VISUALIZER_MODE;
}

function visualizerPlacementLabel(
  placement: EqualizerVisualizerPlacement,
  text: ReturnType<typeof getXiaText>["settings"]["equalizer"],
) {
  return text.visualizerGroups[placement];
}

function visualizerLabel(
  mode: string,
  text: ReturnType<typeof getXiaText>["settings"]["equalizer"],
) {
  return (text.visualizers as Record<string, string>)[mode] ?? mode;
}
