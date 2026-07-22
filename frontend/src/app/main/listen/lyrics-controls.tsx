import {
  AudioLines,
  Minus,
  Plus,
  RotateCcw,
  ScanText,
  Search,
  SlidersHorizontal,
  Sparkles,
} from "lucide-react";
import * as React from "react";

import type { getXiaText } from "@/features/xiadown/shared";
import { DEFAULT_LISTEN_LYRICS_FOCUS_STYLE } from "@/app/main/listen/lyrics-focus-style";
import { ListenLyricsMatchDialog } from "@/app/main/listen/lyrics-match-dialog";
import {
  clearListenLyricsManualOverride,
  listenLyricsTrackPreferenceKey,
  listenLyricsVersionPreferenceKey,
  readListenLyricsManualOverride,
  saveListenLyricsManualOverride,
  saveListenLyricsOffset,
  saveListenLyricsRendererPreference,
  useListenLyricsOffsetPreference,
  useListenLyricsRendererPreference,
  type ListenLyricsManualOverride,
  type ListenLyricsRendererPreference,
} from "@/app/main/listen/lyrics-preferences";
import type { ListenLyricsWorkspaceTrack } from "@/app/main/listen/lyrics-workspace";
import {
  formatListenLyricsOffset,
  LISTEN_LYRICS_OFFSET_STEP_MS,
  normalizeListenLyricsWorkspaceOffset,
  resolveListenLyricsRenderTimeMs,
  stepListenLyricsWorkspaceOffset,
} from "@/app/main/listen/lyrics-workspace-state";
import type {
  ListenLyricsCandidate,
  ListenLyricsData,
} from "@/app/main/listen/types";
import { Button } from "@/shared/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from "@/shared/ui/dropdown-menu";
import { GlassGroup } from "@/shared/ui/glass-surface";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/shared/ui/tooltip";

type LyricsText = ReturnType<typeof getXiaText>;

const LISTEN_LYRICS_DISPLAY_MODES = [
  {
    value: "scroll",
    label: (text: LyricsText) => text.listen.lyricsDynamicMode,
    icon: AudioLines,
  },
  {
    value: "focus",
    label: (text: LyricsText) => text.listen.lyricsFocusMode,
    icon: ScanText,
  },
] as const satisfies ReadonlyArray<{
  value: ListenLyricsRendererPreference;
  label: (text: LyricsText) => string;
  icon: React.ComponentType<{ className?: string }>;
}>;

export type ListenLyricsControlsPlacement =
  | "overlay"
  | "companion"
  | "fullscreen";

export type ListenLyricsControlsProps = {
  placement: ListenLyricsControlsPlacement;
  text: LyricsText;
  track: ListenLyricsWorkspaceTrack;
  lyrics?: ListenLyricsData | null;
  currentTimeMs: number;
  timelineRunning?: boolean;
  playbackRate?: number;
  language?: string;
  synced?: boolean;
  romanized?: boolean;
  pinyin?: boolean;
  onLyricsChange: (lyrics: ListenLyricsData) => void | Promise<void>;
  onRestoreAutomatic: () => void | Promise<void>;
};

export function ListenLyricsControls(props: ListenLyricsControlsProps) {
  const renderer = useListenLyricsRendererPreference();
  const focusStyle = DEFAULT_LISTEN_LYRICS_FOCUS_STYLE;
  const offsetMs = useListenLyricsOffsetPreference(props.track, props.lyrics);
  const trackKey = listenLyricsTrackPreferenceKey(props.track);
  const lyricsVersionKey = props.lyrics
    ? listenLyricsVersionPreferenceKey(props.track, props.lyrics)
    : "";
  const [menuOpen, setMenuOpen] = React.useState(false);
  const [matchOpen, setMatchOpen] = React.useState(false);
  const [manualOverride, setManualOverride] =
    React.useState<ListenLyricsManualOverride | null>(null);
  const triggerRef = React.useRef<HTMLButtonElement>(null);
  const generationRef = React.useRef(0);
  const matchLaunchFrameRef = React.useRef<number | null>(null);
  const launchingMatchRef = React.useRef(false);

  React.useEffect(() => {
    generationRef.current += 1;
    setMenuOpen(false);
    setMatchOpen(false);
    launchingMatchRef.current = false;
    if (matchLaunchFrameRef.current !== null) {
      window.cancelAnimationFrame(matchLaunchFrameRef.current);
      matchLaunchFrameRef.current = null;
    }
    return () => {
      generationRef.current += 1;
      if (matchLaunchFrameRef.current !== null) {
        window.cancelAnimationFrame(matchLaunchFrameRef.current);
        matchLaunchFrameRef.current = null;
      }
    };
  }, [trackKey]);

  React.useEffect(() => {
    setManualOverride(readListenLyricsManualOverride(props.track));
  }, [lyricsVersionKey, trackKey]);

  const timingAvailable =
    props.lyrics?.kind === "synced" && props.lyrics.lines.length > 0;
  const hasManualOverride = Boolean(manualOverride);

  const applyOffset = React.useCallback(
    (next: number) => {
      if (!props.lyrics || !timingAvailable) {
        return;
      }
      saveListenLyricsOffset(
        props.track,
        props.lyrics,
        normalizeListenLyricsWorkspaceOffset(next),
      );
    },
    [props.lyrics, props.track, timingAvailable],
  );

  const handleConfirm = React.useCallback(
    async (candidate: ListenLyricsCandidate, nextLyrics: ListenLyricsData) => {
      const generation = generationRef.current;
      const previous = readListenLyricsManualOverride(props.track);
      const saved = saveListenLyricsManualOverride(props.track, {
        providerId: candidate.providerId,
        providerTrackId: candidate.providerTrackId,
        title: candidate.title,
        artist: candidate.artist,
        album: candidate.album,
        durationSeconds: candidate.durationSeconds,
        timingQuality: candidate.timingQuality ?? nextLyrics.timingQuality,
        confidence: candidate.confidence,
      });
      if (!saved) {
        throw new Error("E_LYRICS_OVERRIDE_PERSIST");
      }
      try {
        await props.onLyricsChange(nextLyrics);
      } catch (error) {
        if (previous) {
          restoreListenLyricsManualOverride(props.track, previous);
        } else {
          clearListenLyricsManualOverride(props.track);
        }
        if (generationRef.current === generation) {
          setManualOverride(readListenLyricsManualOverride(props.track));
        }
        throw error;
      }
      if (generationRef.current === generation) {
        setManualOverride(readListenLyricsManualOverride(props.track));
      }
    },
    [props.onLyricsChange, props.track],
  );

  const handleRestoreAutomatic = React.useCallback(async () => {
    const generation = generationRef.current;
    const previous = readListenLyricsManualOverride(props.track);
    const cleared = clearListenLyricsManualOverride(props.track);
    if (previous && !cleared) {
      throw new Error("E_LYRICS_OVERRIDE_CLEAR");
    }
    try {
      await props.onRestoreAutomatic();
      if (generationRef.current === generation) {
        setManualOverride(null);
      }
    } catch (error) {
      if (previous) {
        restoreListenLyricsManualOverride(props.track, previous);
      }
      if (generationRef.current === generation) {
        setManualOverride(readListenLyricsManualOverride(props.track));
      }
      throw error;
    }
  }, [props.onRestoreAutomatic, props.track]);

  const launchMatchDialog = React.useCallback(() => {
    launchingMatchRef.current = true;
    setMenuOpen(false);
    if (matchLaunchFrameRef.current !== null) {
      window.cancelAnimationFrame(matchLaunchFrameRef.current);
      matchLaunchFrameRef.current = null;
    }
  }, []);

  const controls = (
    <div
      className="listen-lyrics-controls wails-no-drag"
      data-placement={props.placement}
      data-timing-available={timingAvailable ? "true" : "false"}
      role="group"
      aria-label={props.text.listen.lyricsSettings}
    >
        <ListenLyricsModeSwitch
          value={renderer}
          text={props.text}
          disabled={!timingAvailable}
          onValueChange={saveListenLyricsRendererPreference}
        />
        <span className="listen-lyrics-controls__divider" aria-hidden="true" />
        <DropdownMenu open={menuOpen} onOpenChange={setMenuOpen}>
          <TooltipProvider delayDuration={220}>
            <Tooltip>
              <TooltipTrigger asChild>
                <DropdownMenuTrigger asChild>
                  <Button
                    ref={triggerRef}
                    type="button"
                    variant="ghost"
                    size="compactIcon"
                    shape="circle"
                    className="listen-lyrics-controls__menu-trigger"
                    data-active={menuOpen ? "true" : "false"}
                    aria-label={props.text.listen.lyricsSettings}
                  >
                    <SlidersHorizontal className="h-3.5 w-3.5" />
                  </Button>
                </DropdownMenuTrigger>
              </TooltipTrigger>
              <TooltipContent side="top">
                {props.text.listen.lyricsSettings}
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
          <DropdownMenuContent
            side="top"
            align="center"
            sideOffset={8}
            collisionPadding={12}
            className="listen-lyrics-controls__menu w-[17.5rem] p-1.5"
            onCloseAutoFocus={(event) => {
              if (launchingMatchRef.current) {
                event.preventDefault();
                const generation = generationRef.current;
                matchLaunchFrameRef.current = window.requestAnimationFrame(() => {
                  matchLaunchFrameRef.current = null;
                  if (generationRef.current !== generation) {
                    launchingMatchRef.current = false;
                    return;
                  }
                  setMatchOpen(true);
                  launchingMatchRef.current = false;
                });
              }
            }}
          >
            <DropdownMenuLabel className="flex items-center justify-between gap-3">
              <span>{props.text.listen.lyricsTimingOffset}</span>
              <output aria-live="polite">
                {formatListenLyricsOffset(offsetMs)}
              </output>
            </DropdownMenuLabel>
            <DropdownMenuItem
              disabled={!timingAvailable}
              onSelect={(event) => {
                event.preventDefault();
                applyOffset(
                  stepListenLyricsWorkspaceOffset(offsetMs, "earlier"),
                );
              }}
            >
              <Plus className="h-3.5 w-3.5" />
              <span>{props.text.listen.lyricsEarlier}</span>
              <DropdownMenuShortcut>
                {formatListenLyricsOffset(LISTEN_LYRICS_OFFSET_STEP_MS)}
              </DropdownMenuShortcut>
            </DropdownMenuItem>
            <DropdownMenuItem
              disabled={!timingAvailable}
              onSelect={(event) => {
                event.preventDefault();
                applyOffset(
                  stepListenLyricsWorkspaceOffset(offsetMs, "later"),
                );
              }}
            >
              <Minus className="h-3.5 w-3.5" />
              <span>{props.text.listen.lyricsLater}</span>
              <DropdownMenuShortcut>
                {formatListenLyricsOffset(-LISTEN_LYRICS_OFFSET_STEP_MS)}
              </DropdownMenuShortcut>
            </DropdownMenuItem>
            <DropdownMenuItem
              disabled={!timingAvailable || offsetMs === 0}
              onSelect={(event) => {
                event.preventDefault();
                applyOffset(0);
              }}
            >
              <RotateCcw className="h-3.5 w-3.5" />
              <span>{props.text.listen.lyricsTimingReset}</span>
            </DropdownMenuItem>

            <DropdownMenuSeparator />
            <DropdownMenuItem
              data-manual={hasManualOverride ? "true" : "false"}
              disabled={!props.track.title.trim()}
              onSelect={(event) => {
                event.preventDefault();
                launchMatchDialog();
              }}
            >
              {hasManualOverride ? (
                <Sparkles className="h-3.5 w-3.5" />
              ) : (
                <Search className="h-3.5 w-3.5" />
              )}
              <span>{props.text.listen.lyricsSearchOnline}</span>
            </DropdownMenuItem>
          </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );

  return (
    <>
      {props.placement === "overlay" ? (
        <GlassGroup
          asChild
          surfaceRole="control"
          elevation="floating"
          shape="capsule"
        >
          {controls}
        </GlassGroup>
      ) : (
        controls
      )}

      <ListenLyricsMatchDialog
        open={matchOpen}
        onOpenChange={setMatchOpen}
        text={props.text}
        track={props.track}
        language={props.language ?? props.text.locale}
        synced={props.synced}
        renderer={renderer}
        focusStyle={focusStyle}
        currentTimeMs={resolveListenLyricsRenderTimeMs(
          props.currentTimeMs,
          offsetMs,
        )}
        offsetMs={offsetMs}
        timelineRunning={props.timelineRunning}
        playbackRate={props.playbackRate}
        romanized={props.romanized}
        pinyin={props.pinyin}
        hasManualOverride={hasManualOverride}
        returnFocusRef={triggerRef}
        onConfirm={handleConfirm}
        onRestoreAutomatic={handleRestoreAutomatic}
      />
    </>
  );
}

function ListenLyricsModeSwitch(props: {
  value: ListenLyricsRendererPreference;
  text: LyricsText;
  disabled: boolean;
  onValueChange: (value: ListenLyricsRendererPreference) => void;
}) {
  const buttonRefs = React.useRef<Array<HTMLButtonElement | null>>([]);
  const descriptionID = React.useId();
  return (
    <TooltipProvider delayDuration={220}>
      <div
        className="listen-lyrics-controls__mode-switch"
        role="radiogroup"
        aria-label={props.text.listen.lyricsDisplayMode}
        aria-disabled={props.disabled || undefined}
        aria-describedby={props.disabled ? descriptionID : undefined}
        title={
          props.disabled ? props.text.listen.lyricsModeUnavailable : undefined
        }
      >
        {LISTEN_LYRICS_DISPLAY_MODES.map((mode, index) => {
          const Icon = mode.icon;
          const active = mode.value === props.value;
          const label = mode.label(props.text);
          return (
            <Tooltip key={mode.value}>
              <TooltipTrigger asChild>
                <Button
                  ref={(node) => {
                    buttonRefs.current[index] = node;
                  }}
                  type="button"
                  variant="ghost"
                  size="compactIcon"
                  shape="circle"
                  role="radio"
                  aria-label={label}
                  aria-checked={active}
                  data-active={active ? "true" : "false"}
                  disabled={props.disabled}
                  tabIndex={props.disabled ? -1 : active ? 0 : -1}
                  className="listen-lyrics-controls__mode-button"
                  onClick={() => props.onValueChange(mode.value)}
                  onKeyDown={(event) => {
                    if (props.disabled) {
                      return;
                    }
                    let nextIndex = index;
                    if (event.key === "Home") {
                      nextIndex = 0;
                    } else if (event.key === "End") {
                      nextIndex = LISTEN_LYRICS_DISPLAY_MODES.length - 1;
                    } else if (
                      event.key === "ArrowLeft" ||
                      event.key === "ArrowUp"
                    ) {
                      nextIndex =
                        (index - 1 + LISTEN_LYRICS_DISPLAY_MODES.length) %
                        LISTEN_LYRICS_DISPLAY_MODES.length;
                    } else if (
                      event.key === "ArrowRight" ||
                      event.key === "ArrowDown"
                    ) {
                      nextIndex =
                        (index + 1) % LISTEN_LYRICS_DISPLAY_MODES.length;
                    } else {
                      return;
                    }
                    event.preventDefault();
                    props.onValueChange(
                      LISTEN_LYRICS_DISPLAY_MODES[nextIndex].value,
                    );
                    buttonRefs.current[nextIndex]?.focus();
                  }}
                >
                  <Icon className="h-3.5 w-3.5" />
                </Button>
              </TooltipTrigger>
              <TooltipContent side="top">{label}</TooltipContent>
            </Tooltip>
          );
        })}
        <span id={descriptionID} className="sr-only">
          {props.text.listen.lyricsModeUnavailable}
        </span>
      </div>
    </TooltipProvider>
  );
}

function restoreListenLyricsManualOverride(
  track: ListenLyricsWorkspaceTrack,
  override: ListenLyricsManualOverride,
) {
  saveListenLyricsManualOverride(track, {
    providerId: override.providerId,
    providerTrackId: override.providerTrackId,
    title: override.title,
    artist: override.artist,
    album: override.album,
    durationSeconds: override.durationSeconds,
    timingQuality: override.timingQuality,
    confidence: override.confidence,
  });
}
