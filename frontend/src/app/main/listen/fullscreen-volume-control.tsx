import { Volume2, VolumeX } from "lucide-react";
import * as React from "react";

import type { getXiaText } from "@/features/xiadown/shared";
import { clampVolume } from "@/app/main/listen/local-library";
import { ListenPlayerIconButton } from "@/app/main/listen/playback-ui";

export function ListenFullscreenVolumeControl(props: {
  muted: boolean;
  volume: number;
  text: ReturnType<typeof getXiaText>;
  onToggleMute: () => void;
  onVolumeChange: (value: number) => void;
}) {
  const visibleVolume = props.muted ? 0 : clampVolume(props.volume);
  const volumePercent = Math.round(visibleVolume * 1000) / 10;
  const [open, setOpen] = React.useState(false);
  const regionRef = React.useRef<HTMLDivElement | null>(null);

  const openSlider = React.useCallback(() => {
    setOpen(true);
  }, []);

  React.useEffect(() => {
    if (!open) return;
    const closeOnOutsidePointer = (event: PointerEvent) => {
      const target = event.target;
      if (target instanceof Node && !regionRef.current?.contains(target)) {
        setOpen(false);
      }
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key !== "Escape" || event.defaultPrevented) return;
      event.preventDefault();
      event.stopPropagation();
      setOpen(false);
    };
    document.addEventListener("pointerdown", closeOnOutsidePointer, true);
    document.addEventListener("keydown", closeOnEscape, true);
    return () => {
      document.removeEventListener("pointerdown", closeOnOutsidePointer, true);
      document.removeEventListener("keydown", closeOnEscape, true);
    };
  }, [open]);

  return (
    <div
      ref={regionRef}
      className="listen-video-volume wails-no-drag group/listen-fullscreen-volume"
      data-open={open ? "true" : "false"}
      onPointerDownCapture={(event) => event.stopPropagation()}
      onMouseDownCapture={(event) => event.stopPropagation()}
    >
      <span
        className="listen-video-volume-trigger wails-no-drag"
        onPointerEnter={openSlider}
        onFocusCapture={openSlider}
      >
        <ListenPlayerIconButton
          label={
            props.muted || props.volume <= 0
              ? props.text.listen.unmute
              : props.text.listen.mute
          }
          tooltip={false}
          className="listen-video-action-button"
          onClick={props.onToggleMute}
        >
          {props.muted || props.volume <= 0 ? (
            <VolumeX className="h-4 w-4" />
          ) : (
            <Volume2 className="h-4 w-4" />
          )}
        </ListenPlayerIconButton>
      </span>
      {open ? (
        <div
          className="listen-video-volume-slider wails-no-drag"
          onPointerDownCapture={(event) => event.stopPropagation()}
          onMouseDownCapture={(event) => event.stopPropagation()}
        >
          <div className="listen-volume-slider wails-no-drag">
            <div className="listen-volume-slider-track" aria-hidden="true">
              <span style={{ width: `${volumePercent}%` }} />
            </div>
            <span
              aria-hidden="true"
              className="listen-volume-slider-thumb"
              style={{ left: `${volumePercent}%` }}
            />
            <input
              className="listen-volume-range wails-no-drag"
              type="range"
              min={0}
              max={1}
              step={0.01}
              value={visibleVolume}
              aria-label={props.text.listen.volume}
              title={props.text.listen.volume}
              onChange={(event) =>
                props.onVolumeChange(Number(event.target.value))
              }
            />
          </div>
        </div>
      ) : null}
    </div>
  );
}
