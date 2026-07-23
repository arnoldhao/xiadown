import { Loader2, RotateCcw } from "lucide-react";

import type { EqualizerBand } from "@/shared/contracts/equalizer";
import { Button } from "@/shared/ui/button";
import { SettingsCompactListCard } from "@/shared/ui/settings-layout";

const MIN_GAIN = -12;
const MAX_GAIN = 12;
const CURVE_HEIGHT = 128;
const CURVE_WIDTH = 100;
const PREAMP_TICKS = [-12, -9, -6, -3, 0, 3, 6, 9, 12];
const CURVE_VERTICAL_FILL_RATIO = 0.85;

export interface EqualizerControlCardsProps {
  bands: readonly EqualizerBand[];
  bandGainsDb: readonly number[];
  preampDb: number;
  enabled: boolean;
  disabled?: boolean;
  resetPending?: boolean;
  labels: {
    bands: string;
    preamp: string;
    reset: string;
  };
  onBandGainChange: (index: number, gainDb: number) => void;
  onPreampChange: (gainDb: number) => void;
  onReset: () => void;
}

/** The two visual Equalizer cards shared by Settings and Appearance Lab. */
export function EqualizerControlCards(props: EqualizerControlCardsProps) {
  const bandCount = Math.max(props.bands.length, 1);
  const gains = props.bands.map(
    (_, index) => props.bandGainsDb[index] ?? 0,
  );
  const bandGridStyle = {
    gridTemplateColumns: `repeat(${bandCount}, minmax(0, 1fr))`,
  };

  return (
    <>
      <SettingsCompactListCard
        contentClassName="app-equalizer-preset-card-content"
        data-equalizer-control-card="preset"
      >
        <div
          className="app-equalizer-bands space-y-4"
          data-disabled={props.disabled || undefined}
        >
          <div className="grid" style={bandGridStyle}>
            {props.bands.map((band, index) => (
              <div key={band.id} className="min-w-0">
                <span className="app-equalizer-gain block w-full truncate">
                  {formatGain(gains[index] ?? 0)}
                </span>
              </div>
            ))}
          </div>
          <div className="relative h-32">
            <EqualizerSliderCurve gains={gains} />
            <div className="relative z-10 grid h-32" style={bandGridStyle}>
              {props.bands.map((band, index) => (
                <div
                  key={band.id}
                  className="flex h-32 min-w-0 items-center justify-center"
                >
                  <input
                    type="range"
                    min={MIN_GAIN}
                    max={MAX_GAIN}
                    step={0.5}
                    value={gains[index] ?? 0}
                    disabled={props.disabled}
                    onChange={(event) =>
                      props.onBandGainChange(index, Number(event.target.value))
                    }
                    aria-label={`${props.labels.bands} ${band.display}`}
                    className="app-equalizer-vertical-slider h-32 w-6"
                  />
                </div>
              ))}
            </div>
          </div>
          <div className="grid" style={bandGridStyle}>
            {props.bands.map((band) => (
              <div
                key={band.id}
                className="app-equalizer-band-caption min-w-0"
              >
                <div className="app-equalizer-band-label truncate">
                  {band.display}
                </div>
                <div className="app-equalizer-frequency">{band.displayHz}</div>
              </div>
            ))}
          </div>
        </div>
      </SettingsCompactListCard>

      <SettingsCompactListCard
        contentClassName="app-equalizer-preamp-card-content"
        data-equalizer-control-card="preamp"
      >
        <div className="flex min-w-0 items-center justify-between gap-3">
          <div className="app-equalizer-preamp-label min-w-0 truncate">
            {props.labels.preamp}
          </div>
          <span className="app-equalizer-preamp-value shrink-0">
            {formatGain(props.preampDb)}
          </span>
        </div>
        <div className="mt-3">
          <input
            type="range"
            min={MIN_GAIN}
            max={MAX_GAIN}
            step={0.5}
            value={props.preampDb}
            disabled={props.disabled}
            onChange={(event) => props.onPreampChange(Number(event.target.value))}
            aria-label={props.labels.preamp}
            className="app-equalizer-slider h-5 w-full"
          />
          <div
            className="app-equalizer-ticks mt-1 grid grid-cols-9 px-1"
            aria-hidden="true"
          >
            {PREAMP_TICKS.map((tick) => (
              <div
                key={tick}
                className="flex min-w-0 flex-col items-center gap-1"
              >
                <span className="app-equalizer-tick-mark h-1.5" />
                <span>{formatTick(tick)}</span>
              </div>
            ))}
          </div>
        </div>
        <div className="mt-3 flex justify-end">
          <Button
            type="button"
            variant="outline"
            size="compact"
            disabled={!props.enabled || props.resetPending}
            onClick={props.onReset}
          >
            {props.resetPending ? (
              <Loader2 className="h-4 w-4 app-motion-spin" />
            ) : (
              <RotateCcw className="h-4 w-4" />
            )}
            {props.labels.reset}
          </Button>
        </div>
      </SettingsCompactListCard>
    </>
  );
}

function EqualizerSliderCurve(props: { gains: readonly number[] }) {
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
          className="app-equalizer-curve-glow"
          strokeWidth="7"
          strokeLinecap="butt"
          strokeLinejoin="round"
          vectorEffect="non-scaling-stroke"
        />
        <path
          d={path}
          fill="none"
          className="app-equalizer-curve-line"
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

function buildCurvePoints(gains: readonly number[]) {
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
