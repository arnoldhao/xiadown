import { Activity, AlertCircle, CheckCircle2, Loader2, Power, RotateCcw, ShieldAlert } from "lucide-react";

import { getXiaText } from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
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
import { Select } from "@/shared/ui/select";
import {
  SettingsCompactListCard,
  SettingsCompactRow,
  SettingsCompactSeparator,
} from "@/shared/ui/settings-layout";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/shared/ui/tooltip";

const MIN_GAIN = -12;
const MAX_GAIN = 12;
const CURVE_HEIGHT = 128;
const CURVE_WIDTH = 100;
const PREAMP_TICKS = [-12, -9, -6, -3, 0, 3, 6, 9, 12];
const CURVE_VERTICAL_FILL_RATIO = 0.85;
const VISUALIZER_PLACEMENT_OPTIONS: readonly EqualizerVisualizerPlacement[] = ["artwork", "spectrum", "off"];
const VISUALIZER_ACTIVE_STYLE = {
  backgroundColor: "hsl(var(--primary) / 0.12)",
  borderColor: "hsl(var(--primary) / 0.34)",
  boxShadow: "0 0 0 1px hsl(var(--primary) / 0.12)",
  color: "hsl(var(--primary))",
};

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
          <div className="flex min-w-0 items-center justify-end gap-2 text-sm text-muted-foreground">
            <ShieldAlert className="h-4 w-4 shrink-0" />
            <span className="min-w-0 truncate text-right">{equalizerText.macOSOnly}</span>
          </div>
        </SettingsCompactRow>
      </SettingsCompactListCard>
    );
  }

  if (query.isLoading && !query.data) {
    return (
      <SettingsCompactListCard>
        <SettingsCompactRow label={equalizerText.title}>
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Loader2 className="h-4 w-4 animate-spin" />
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
          <StatusBadge code="error" label={equalizerText.status.error} />
        </SettingsCompactRow>
      </SettingsCompactListCard>
    );
  }

  const settings = snapshot.settings;
  const disabled = !settings.enabled || !snapshot.status.supported;
  const bandCount = Math.max(snapshot.bands.length, 1);
  const bandGains = snapshot.bands.map((_, index) => settings.bandGainsDb[index] ?? 0);
  const bandGridStyle = { gridTemplateColumns: `repeat(${bandCount}, minmax(0, 1fr))` };
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
              size="compact"
              disabled={!snapshot.status.supported || setVisualizerMode.isPending}
              className={cn("min-w-0 px-2 text-[11px]", visualizerPlacement === placement ? "border-transparent" : "")}
              onClick={() =>
                void setVisualizerMode
                  .mutateAsync(defaultVisualizerModeForPlacement(placement, settings))
                  .catch(console.warn)
              }
              style={visualizerPlacement === placement ? VISUALIZER_ACTIVE_STYLE : undefined}
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
                <StatusBadge code={snapshot.status.code} label={statusLabel(snapshot.status.code, equalizerText)} />
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
              <StatusBadge
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
                  {retryEqualizer.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Activity className="h-4 w-4" />}
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

      <SettingsCompactListCard contentClassName="p-4">
        <div className={cn("space-y-4", disabled ? "opacity-50" : "")}>
          <div className="grid" style={bandGridStyle}>
            {snapshot.bands.map((band, index) => (
              <div key={band.id} className="min-w-0">
                <span className="block w-full truncate text-center font-mono text-[10px] text-muted-foreground">
                  {formatGain(bandGains[index] ?? 0)}
                </span>
              </div>
            ))}
          </div>
          <div className="relative h-32">
            <EqualizerSliderCurve gains={bandGains} />
            <div className="relative z-10 grid h-32" style={bandGridStyle}>
              {snapshot.bands.map((band, index) => (
                <div key={band.id} className="flex h-32 min-w-0 items-center justify-center">
                  <input
                    type="range"
                    min={MIN_GAIN}
                    max={MAX_GAIN}
                    step={0.5}
                    value={bandGains[index] ?? 0}
                    disabled={disabled}
                    onChange={(event) =>
                      void setBandGain
                        .mutateAsync({ index, gainDb: Number(event.target.value) })
                        .catch(console.warn)
                    }
                    aria-label={`${equalizerText.bands} ${band.display}`}
                    className="h-32 w-6 cursor-pointer accent-[hsl(var(--primary))] [direction:rtl] [writing-mode:vertical-lr] disabled:cursor-not-allowed"
                  />
                </div>
              ))}
            </div>
          </div>
          <div className="grid" style={bandGridStyle}>
            {snapshot.bands.map((band) => (
              <div key={band.id} className="min-w-0 text-center">
                <div className="truncate font-mono text-[11px] text-foreground">{band.display}</div>
                <div className="text-[9px] text-muted-foreground">{band.displayHz}</div>
              </div>
            ))}
          </div>
        </div>
      </SettingsCompactListCard>

      <SettingsCompactListCard contentClassName="p-4">
        <div className="flex min-w-0 items-center justify-between gap-3">
          <div className="min-w-0 truncate text-sm font-medium text-foreground">{equalizerText.preamp}</div>
          <span className="shrink-0 font-mono text-xs text-muted-foreground">{formatGain(settings.preampDb)}</span>
        </div>
        <div className="mt-3">
          <input
            type="range"
            min={MIN_GAIN}
            max={MAX_GAIN}
            step={0.5}
            value={settings.preampDb}
            disabled={disabled}
            onChange={(event) => void setPreamp.mutateAsync(Number(event.target.value)).catch(console.warn)}
            aria-label={equalizerText.preamp}
            className="h-5 w-full accent-[hsl(var(--primary))]"
          />
          <div className="mt-1 grid grid-cols-9 px-1 text-[10px] leading-none text-muted-foreground" aria-hidden="true">
            {PREAMP_TICKS.map((tick) => (
              <div key={tick} className="flex min-w-0 flex-col items-center gap-1">
                <span className="h-1.5 border-l border-border" />
                <span className="font-mono">{formatTick(tick)}</span>
              </div>
            ))}
          </div>
        </div>
        <div className="mt-3 flex justify-end">
          <Button
            type="button"
            variant="outline"
            size="compact"
            disabled={!settings.enabled || resetEqualizer.isPending}
            onClick={() => void resetEqualizer.mutateAsync(undefined).catch(console.warn)}
          >
            {resetEqualizer.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <RotateCcw className="h-4 w-4" />}
            {equalizerText.reset}
          </Button>
        </div>
      </SettingsCompactListCard>
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
    <button
      type="button"
      role="switch"
      aria-checked={props.checked}
      aria-label={props.ariaLabel}
      disabled={props.disabled === true}
      onClick={() => {
        if (props.disabled === true) {
          return;
        }
        props.onChange(!props.checked);
      }}
      className={cn(
        "app-dream-inline-switch disabled:cursor-not-allowed disabled:opacity-50",
        props.checked ? "justify-end" : "justify-start",
      )}
      data-state={props.checked ? "checked" : "unchecked"}
    >
      <span className="app-dream-inline-switch-knob" />
    </button>
  );
}

function StatusBadge(props: {
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
  const badge = (
    <span
      className={cn(
        "inline-flex min-w-0 items-center gap-1.5 rounded-full border px-2 py-1 text-xs",
        props.code === "active"
          ? "border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-200"
          : props.code === "permission_needed" || props.code === "error"
            ? "border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-200"
            : "border-border bg-muted/40 text-muted-foreground",
      )}
    >
      <Icon className="h-3.5 w-3.5 shrink-0" />
      <span className="truncate">{props.label}</span>
    </span>
  );

  if (!props.tooltip) {
    return badge;
  }

  return (
    <Tooltip>
      <TooltipTrigger asChild>{badge}</TooltipTrigger>
      <TooltipContent side="top" multiline className="text-left text-xs leading-snug">
        {props.tooltip}
      </TooltipContent>
    </Tooltip>
  );
}

function EqualizerSliderCurve(props: { gains: number[] }) {
  const points = buildCurvePoints(props.gains);
  const path = buildCurvePath(points);
  return (
    <div className="pointer-events-none absolute inset-0 z-0" aria-hidden="true">
      <svg
        viewBox={`0 0 ${CURVE_WIDTH} ${CURVE_HEIGHT}`}
        preserveAspectRatio="none"
        className="absolute inset-0 h-full w-full overflow-visible"
      >
        <path
          d={path}
          fill="none"
          className="stroke-primary opacity-20 blur-[1.5px]"
          strokeWidth="7"
          strokeLinecap="butt"
          strokeLinejoin="round"
          vectorEffect="non-scaling-stroke"
        />
        <path
          d={path}
          fill="none"
          className="stroke-primary opacity-80"
          strokeWidth="2.5"
          strokeLinecap="butt"
          strokeLinejoin="round"
          vectorEffect="non-scaling-stroke"
        />
      </svg>
    </div>
  );
}

type CurvePoint = {
  x: number;
  y: number;
};

function buildCurvePoints(gains: number[]) {
  return gains.map((gain, index) => curvePoint(index, gains.length, gain));
}

function buildCurvePath(points: CurvePoint[]) {
  if (points.length === 0) {
    return `M0 ${(CURVE_HEIGHT / 2).toFixed(2)} L${CURVE_WIDTH.toFixed(2)} ${(CURVE_HEIGHT / 2).toFixed(2)}`;
  }
  let path = `M${points[0].x.toFixed(2)} ${points[0].y.toFixed(2)}`;
  for (let index = 0; index < points.length - 1; index += 1) {
    const p0 = index === 0 ? points[0] : points[index - 1];
    const p1 = points[index];
    const p2 = points[index + 1];
    const p3 = index + 2 < points.length ? points[index + 2] : p2;
    const control1 = {
      x: p1.x + (p2.x - p0.x) / 6,
      y: p1.y + (p2.y - p0.y) / 6,
    };
    const control2 = {
      x: p2.x - (p3.x - p1.x) / 6,
      y: p2.y - (p3.y - p1.y) / 6,
    };
    path += ` C${control1.x.toFixed(2)} ${control1.y.toFixed(2)} ${control2.x.toFixed(2)} ${control2.y.toFixed(2)} ${p2.x.toFixed(2)} ${p2.y.toFixed(2)}`;
  }
  return path;
}

function curvePoint(index: number, count: number, gain: number) {
  const safeCount = Math.max(count, 1);
  return {
    x: equalColumnCenter(index, safeCount),
    y: gainToCurveY(gain),
  };
}

function equalColumnCenter(index: number, count: number) {
  return (CURVE_WIDTH / Math.max(count, 1)) * (index + 0.5);
}

function gainToCurveY(gain: number) {
  const clampedGain = Math.min(Math.max(gain, MIN_GAIN), MAX_GAIN);
  const rangeSpan = MAX_GAIN - MIN_GAIN;
  const scaleY = (CURVE_HEIGHT * CURVE_VERTICAL_FILL_RATIO) / rangeSpan;
  return CURVE_HEIGHT / 2 - clampedGain * scaleY;
}

function formatGain(value: number) {
  const safe = Number.isFinite(value) ? value : 0;
  const normalized = Math.abs(safe) < 0.05 ? 0 : safe;
  return `${normalized > 0 ? "+" : ""}${normalized.toFixed(1)} dB`;
}

function formatTick(value: number) {
  return value > 0 ? `+${value}` : String(value);
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
