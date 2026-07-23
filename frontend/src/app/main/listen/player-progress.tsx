import * as React from "react";

import type { getXiaText } from "@/features/xiadown/shared";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/shared/ui/tooltip";

import { formatProgressSeconds } from "@/app/main/listen/local-library";

export function ListenPlayerProgress(props: {
  progress: {
    currentTime: number;
    duration: number;
    bufferedTime: number;
  };
  text: ReturnType<typeof getXiaText>;
  centerLabel?: string;
  live?: boolean;
  playing?: boolean;
  advertising?: boolean;
  advertisingLabel?: string;
  loading?: boolean;
  errorActive?: boolean;
  errorLabel?: string;
  errorTitle?: string;
  variant?: "default" | "footer";
  onSeek?: (seconds: number) => void;
}) {
  const duration = Number.isFinite(props.progress.duration)
    ? Math.max(0, props.progress.duration)
    : 0;
  const rawCurrentTime = Number.isFinite(props.progress.currentTime)
    ? Math.max(0, props.progress.currentTime)
    : 0;
  const currentTime = Math.max(
    0,
    Math.min(rawCurrentTime, duration),
  );
  const bufferedTime = Number.isFinite(props.progress.bufferedTime)
    ? Math.max(0, props.progress.bufferedTime)
    : 0;
  const bufferedPercent =
    duration > 0
      ? Math.max(0, Math.min(100, (bufferedTime / duration) * 100))
      : 0;
  const playedPercent =
    duration > 0 ? Math.max(0, Math.min(100, (currentTime / duration) * 100)) : 0;
  const timelineBufferedPercent = props.live ? 100 : bufferedPercent;
  const timelinePlayedPercent = props.live ? 100 : playedPercent;
  const displayedCurrentTime = props.live ? rawCurrentTime : currentTime;
  const canSeek = !props.live && duration > 0 && Boolean(props.onSeek);
  const remainingTime = props.live ? 0 : Math.max(0, duration - currentTime);
  const errorCode = props.errorLabel?.trim() || "";
  const errorMessage = props.errorTitle?.trim() || "";
  const hasError = Boolean(props.errorActive || errorCode || errorMessage);
  const advertising = Boolean(props.advertising && !hasError);
  const loading = Boolean(props.loading && !hasError && !advertising);
  const statusActive = hasError || advertising || loading;
  const errorLabel = errorCode
    ? `${props.text.listen.errorCodeLabel}: ${errorCode}`
    : props.text.listen.errorStatus;
  const errorTooltip = errorMessage || errorLabel;
  const label = advertising
    ? props.advertisingLabel?.trim() || props.text.listen.adBadge
    : props.text.listen.loading;
  const hasTimedAdProgress =
    advertising &&
    duration > 0 &&
    (playedPercent > 0 || bufferedPercent > 0);
  const handleSeekInput = React.useCallback(
    (event: React.FormEvent<HTMLInputElement>) => {
      if (!canSeek) {
        return;
      }
      const nextTime = Number(event.currentTarget.value);
      if (!Number.isFinite(nextTime)) {
        return;
      }
      props.onSeek?.(nextTime);
    },
    [canSeek, props.onSeek],
  );

  if (props.variant === "footer") {
    const durationAvailable = duration > 0;
    const compactCurrentTime = durationAvailable && !props.live
      ? currentTime
      : rawCurrentTime;
    const compactCanSeek = canSeek && !statusActive;
    const compactState = hasError
      ? "error"
      : advertising
        ? "advertising"
        : loading
          ? "loading"
          : props.live
            ? "live"
            : durationAvailable
              ? "ready"
              : "unknown";
    const trailingLabel = hasError
      ? errorLabel
      : advertising
        ? label
        : loading
          ? label
          : props.live
            ? props.text.listen.liveBadge
            : durationAvailable
              ? `-${formatProgressSeconds(Math.max(0, duration - compactCurrentTime))}`
              : "—:—";
    const compactProgressbar = !statusActive && (props.live || !durationAvailable);
    const compactProgressLabel = props.live
      ? `${props.text.listen.nowPlaying} · ${props.text.listen.liveBadge}`
      : props.text.listen.nowPlaying;

    return (
      <div
        className="listen-player-progress listen-player-progress--footer min-w-0 flex-1"
        data-playing={props.playing ? "true" : "false"}
        data-progress-state={compactState}
        data-variant="footer"
        aria-busy={loading || undefined}
      >
        <div className="flex min-w-0 items-center gap-2">
          <span className="listen-player-progress__current shrink-0">
            {formatProgressSeconds(compactCurrentTime)}
          </span>
          <div
            className="listen-player-progress-control wails-no-drag group/progress relative flex h-6 min-w-0 flex-1 items-center"
            role={compactProgressbar ? "progressbar" : undefined}
            aria-label={compactProgressbar ? compactProgressLabel : undefined}
            aria-valuemin={props.live && !statusActive ? 0 : undefined}
            aria-valuemax={props.live && !statusActive ? 100 : undefined}
            aria-valuenow={props.live && !statusActive ? 100 : undefined}
            aria-valuetext={
              props.live && !statusActive
                ? `${props.text.listen.liveBadge} · -0:00`
                : undefined
            }
            onPointerDown={(event) => event.stopPropagation()}
            onMouseDown={(event) => event.stopPropagation()}
          >
            <div className="listen-player-progress__track listen-player-progress__track--footer pointer-events-none absolute left-0 right-0 top-1/2 h-1.5 -translate-y-1/2 overflow-hidden">
              {hasError ? (
                <div className="listen-player-progress__state-fill h-full w-full" />
              ) : advertising && hasTimedAdProgress ? (
                <>
                  <div
                    className="listen-player-progress__buffer h-full"
                    style={{ width: `${bufferedPercent}%` }}
                  />
                  <div
                    className="listen-player-progress__played absolute inset-y-0 left-0"
                    style={{ width: `${playedPercent}%` }}
                  />
                </>
              ) : advertising ? (
                <div className="listen-player-progress__state-fill h-full w-full" />
              ) : loading || !durationAvailable ? (
                <div className="listen-player-progress__loading-fill h-full w-full" />
              ) : (
                <>
                  <div
                    className="listen-player-progress__buffer h-full"
                    style={{ width: `${bufferedPercent}%` }}
                  />
                  <div
                    className="listen-player-progress__played absolute inset-y-0 left-0"
                    style={{ width: `${playedPercent}%` }}
                  />
                </>
              )}
            </div>
            {compactCanSeek ? (
              <>
                <span
                  aria-hidden="true"
                  className="listen-player-progress__thumb pointer-events-none absolute top-1/2 h-3 w-3 -translate-x-1/2 -translate-y-1/2"
                  style={{ left: `${playedPercent}%` }}
                />
                <input
                  type="range"
                  min={0}
                  max={duration}
                  step={0.01}
                  value={currentTime}
                  aria-label={props.text.listen.seek}
                  aria-valuetext={`${formatProgressSeconds(currentTime)} / ${formatProgressSeconds(duration)}`}
                  className="listen-player-progress__input wails-no-drag relative z-10 h-6 min-w-0 flex-1 touch-none"
                  onInput={handleSeekInput}
                  onChange={handleSeekInput}
                />
              </>
            ) : (
              <span aria-hidden="true" className="relative z-10 h-6 w-full" />
            )}
          </div>
          <span
            className="listen-player-progress__duration max-w-24 shrink-0 truncate"
            role={statusActive ? "status" : undefined}
            title={hasError ? errorTooltip : undefined}
          >
            {trailingLabel}
          </span>
        </div>
      </div>
    );
  }

  if (statusActive) {
    return (
      <div
        className="listen-player-progress mt-4"
        data-progress-state={
          hasError ? "error" : advertising ? "advertising" : "loading"
        }
      >
        <div className="relative flex h-6 items-center">
          <div className="listen-player-progress__track pointer-events-none absolute left-0 right-0 top-1/2 h-1.5 -translate-y-1/2">
            {hasError ? null : advertising && hasTimedAdProgress ? (
              <>
                <div
                  className="listen-player-progress__buffer h-full"
                  style={{ width: `${bufferedPercent}%` }}
                />
                <div
                  className="listen-player-progress__played absolute inset-y-0 left-0"
                  style={{ width: `${playedPercent}%` }}
                />
              </>
            ) : advertising ? (
              <div className="listen-player-progress__state-fill h-full w-full" />
            ) : loading ? (
              <div className="listen-player-progress__loading-fill h-full w-full" />
            ) : (
              <div
                className="listen-player-progress__played absolute inset-y-0 left-0"
                style={{ width: `${playedPercent}%` }}
              />
            )}
          </div>
        </div>
        <div className="listen-player-progress__labels mt-0.5 grid h-4 grid-cols-[1fr_auto_1fr] items-center">
          {hasError ? (
            <>
              <Tooltip>
                <TooltipTrigger asChild>
                  <span className="listen-player-progress__status-label min-w-0 truncate">
                    {errorLabel}
                  </span>
                </TooltipTrigger>
                <TooltipContent side="top" multiline className="listen-player-progress__tooltip">
                  {errorTooltip}
                </TooltipContent>
              </Tooltip>
              <span aria-hidden="true" />
              <span aria-hidden="true" />
            </>
          ) : advertising ? (
            <>
              <span className="listen-player-progress__status-label min-w-0 truncate">
                {label}
              </span>
              <span aria-hidden="true" />
              <span aria-hidden="true" />
            </>
          ) : loading ? (
            <>
              <span aria-hidden="true" />
              <span className="listen-player-progress__loading-label justify-self-center">
                {label}
              </span>
              <span aria-hidden="true" />
            </>
          ) : null}
        </div>
      </div>
    );
  }

  return (
    <div className="listen-player-progress mt-4" data-progress-state="ready">
      <div
        className="listen-player-progress-control wails-no-drag group/progress relative flex h-6 items-center"
        role={props.live ? "progressbar" : undefined}
        aria-label={
          props.live
            ? `${props.text.listen.nowPlaying} · ${props.text.listen.liveBadge}`
            : undefined
        }
        aria-valuemin={props.live ? 0 : undefined}
        aria-valuemax={props.live ? 100 : undefined}
        aria-valuenow={props.live ? 100 : undefined}
        aria-valuetext={
          props.live ? `${props.text.listen.liveBadge} · -0:00` : undefined
        }
        onPointerDown={(event) => event.stopPropagation()}
        onMouseDown={(event) => event.stopPropagation()}
      >
        <div className="listen-player-progress__track pointer-events-none absolute left-0 right-0 top-1/2 h-1.5 -translate-y-1/2 overflow-hidden">
          <div
            className="listen-player-progress__buffer h-full"
            style={{ width: `${timelineBufferedPercent}%` }}
          />
          <div
            className="listen-player-progress__played absolute inset-y-0 left-0"
            style={{ width: `${timelinePlayedPercent}%` }}
          />
        </div>
        {canSeek ? (
          <span
            aria-hidden="true"
            className="listen-player-progress__thumb pointer-events-none absolute top-1/2 h-3.5 w-3.5 -translate-x-1/2 -translate-y-1/2"
            style={{ left: `${playedPercent}%` }}
          />
        ) : null}
        {props.live ? (
          <span aria-hidden="true" className="relative z-10 h-6 w-full" />
        ) : (
          <input
            type="range"
            min={0}
            max={duration || 0}
            step={0.01}
            value={currentTime}
            disabled={!canSeek}
            aria-label={props.text.listen.nowPlaying}
            className="listen-player-progress__input wails-no-drag relative z-10 h-6 w-full touch-none"
            onInput={handleSeekInput}
            onChange={handleSeekInput}
          />
        )}
      </div>
      <div className="listen-player-progress__labels mt-0.5 grid grid-cols-[1fr_auto_1fr] items-center">
        <span className="justify-self-start">{formatProgressSeconds(displayedCurrentTime)}</span>
        <span className="listen-player-progress__center-label max-w-[10rem] truncate px-2">
          {props.centerLabel}
        </span>
        <span className="justify-self-end">-{formatProgressSeconds(remainingTime)}</span>
      </div>
    </div>
  );
}
