import {
MediaPlayer,
MediaProvider,
MediaRemoteControl,
getTimeRangesEnd
} from "@vidstack/react";
import * as React from "react";

import {
getXiaText
} from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
import { DREAM_FM_HIDDEN_ENGINE_STYLE } from "@/shared/styles/dreamfm";

import { clampVolume,resolveAudioSource } from "@/app/main/dreamfm/local-library";
import type { DreamFMLocalPreviewTrack,DreamFMPlaybackProgressState } from "@/app/main/dreamfm/types";
import { DreamFMProgressBar,DreamFMTransportActions } from "@/app/main/dreamfm/ui";

export function DreamFMLocalPreviewPlayer(props: {
  track: DreamFMLocalPreviewTrack;
  text: ReturnType<typeof getXiaText>;
  className?: string;
}) {
  const playerRef = React.useRef<React.ElementRef<typeof MediaPlayer> | null>(
    null,
  );
  const localRemote = React.useMemo(() => new MediaRemoteControl(), []);
  const [playing, setPlaying] = React.useState(false);
  const [muted, setMuted] = React.useState(false);
  const [volume, setVolume] = React.useState(1);
  const lastNonZeroVolumeRef = React.useRef(1);
  const [progress, setProgress] = React.useState<DreamFMPlaybackProgressState>({
    currentTime: 0,
    duration: 0,
    bufferedTime: 0,
  });
  const track = props.track;
  const author = track.author?.trim() || props.text.dreamFm.local;

  const getLocalMediaElement = React.useCallback(() => {
    const provider = playerRef.current?.provider as
      | {
          media?: HTMLMediaElement;
          audio?: HTMLAudioElement;
        }
      | null
      | undefined;
    if (provider?.media instanceof HTMLMediaElement) {
      return provider.media;
    }
    if (provider?.audio instanceof HTMLAudioElement) {
      return provider.audio;
    }
    const root = playerRef.current as unknown as
      | {
          querySelector?: (selector: string) => Element | null;
        }
      | null
      | undefined;
    const element =
      typeof root?.querySelector === "function"
        ? root.querySelector("audio,video")
        : null;
    return element instanceof HTMLMediaElement ? element : null;
  }, []);

  const readLocalProgress = React.useCallback(() => {
    const player = playerRef.current;
    const media = getLocalMediaElement();
    const source = media ?? player;
    const currentTime =
      source && Number.isFinite(source.currentTime)
        ? Math.max(0, source.currentTime)
        : 0;
    const duration =
      source && Number.isFinite(source.duration)
        ? Math.max(0, source.duration)
        : 0;
    const buffered = (source as { buffered?: TimeRanges } | null)?.buffered;
    const bufferedTime = buffered
      ? Math.max(0, getTimeRangesEnd(buffered) ?? 0)
      : 0;

    setProgress((current) => {
      if (
        Math.abs(current.currentTime - currentTime) < 0.05 &&
        Math.abs(current.duration - duration) < 0.05 &&
        Math.abs(current.bufferedTime - bufferedTime) < 0.25
      ) {
        return current;
      }
      return { currentTime, duration, bufferedTime };
    });
  }, [getLocalMediaElement]);

  React.useEffect(() => {
    setPlaying(false);
    setProgress({ currentTime: 0, duration: 0, bufferedTime: 0 });
  }, [track.id]);

  React.useEffect(() => {
    const player = playerRef.current;
    if (!player) {
      return;
    }
    localRemote.setPlayer(player);
  }, [localRemote, track.id]);

  React.useEffect(() => {
    const media = getLocalMediaElement();
    const player = playerRef.current;
    const nextVolume = clampVolume(volume);
    const nextMuted = muted || volume <= 0;
    if (media) {
      media.volume = nextVolume;
      media.muted = nextMuted;
    }
    if (player) {
      player.volume = nextVolume;
      player.muted = nextMuted;
    }
  }, [getLocalMediaElement, muted, track.id, volume]);

  React.useEffect(() => {
    const timer = window.setInterval(readLocalProgress, 250);
    return () => window.clearInterval(timer);
  }, [readLocalProgress]);

  const handleSeek = React.useCallback(
    (seconds: number) => {
      const media = getLocalMediaElement();
      const player = playerRef.current;
      const source = media ?? player;
      const duration =
        source && Number.isFinite(source.duration)
          ? Math.max(0, source.duration)
          : Math.max(0, progress.duration);
      if (duration <= 0) {
        return;
      }
      const nextTime = Math.max(0, Math.min(seconds, duration));
      if (media) {
        media.currentTime = nextTime;
      }
      if (player) {
        player.currentTime = nextTime;
      }
      setProgress((current) => ({
        ...current,
        currentTime: nextTime,
        duration,
      }));
    },
    [getLocalMediaElement, progress.duration],
  );

  const handleTogglePlayback = React.useCallback<
    React.MouseEventHandler<HTMLButtonElement>
  >(
    (event) => {
      const media = getLocalMediaElement();
      const player = playerRef.current;
      if (playing) {
        media?.pause();
        if (player) {
          player.paused = true;
        }
        localRemote.pause(event.nativeEvent);
        return;
      }
      if (media) {
        void media.play().catch(() => {});
      }
      if (player) {
        player.paused = false;
      }
      localRemote.play(event.nativeEvent);
    },
    [getLocalMediaElement, localRemote, playing],
  );

  const handleToggleMute = React.useCallback(() => {
    setMuted((current) => {
      if (current || volume <= 0) {
        setVolume(lastNonZeroVolumeRef.current);
        return false;
      }
      lastNonZeroVolumeRef.current = volume > 0 ? volume : 1;
      return true;
    });
  }, [volume]);

  const handleVolumeChange = React.useCallback((value: number) => {
    const nextVolume = clampVolume(value);
    if (nextVolume > 0) {
      lastNonZeroVolumeRef.current = nextVolume;
    }
    setVolume(nextVolume);
    setMuted(nextVolume <= 0);
  }, []);

  return (
    <div
      className={cn(
        "relative h-full min-h-[16rem] overflow-hidden",
        props.className,
      )}
    >
      <MediaPlayer
        ref={playerRef}
        key={track.id}
        src={resolveAudioSource(track.previewURL, track.path)}
        title={track.title}
        viewType="audio"
        streamType="on-demand"
        load="eager"
        preload="metadata"
        playsInline
        onPlay={() => setPlaying(true)}
        onPause={() => setPlaying(false)}
        onTimeUpdate={() => readLocalProgress()}
        onCanPlay={() => readLocalProgress()}
        onEnded={() => {
          setPlaying(false);
          readLocalProgress();
        }}
        className="pointer-events-none"
        style={DREAM_FM_HIDDEN_ENGINE_STYLE}
      >
        <MediaProvider />
      </MediaPlayer>

      <div className="flex h-full min-h-0 flex-1 items-center justify-center overflow-hidden p-6 sm:p-8">
        <div className="flex w-full max-w-lg flex-col items-center justify-center text-center">
          <div className="min-w-0">
            <div className="overflow-hidden text-balance text-xl font-semibold leading-tight text-sidebar-foreground [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:2] sm:text-2xl">
              {track.title}
            </div>
            <div className="mt-3 truncate text-sm text-sidebar-foreground/58 sm:text-base">
              {author}
            </div>
          </div>

          <div className="mt-8 flex min-h-[2.75rem] w-full max-w-md items-start">
            <DreamFMProgressBar
              currentTime={progress.currentTime}
              duration={progress.duration}
              bufferedTime={progress.bufferedTime}
              ariaLabel={props.text.dreamFm.seek}
              onSeek={handleSeek}
              className="mx-auto max-w-md"
            />
          </div>

          <div className="mt-6">
            <DreamFMTransportActions
              hasTrack
              playing={playing}
              showQueueControls={false}
              loading={false}
              muted={muted}
              volume={volume}
              playMode="order"
              text={props.text}
              onPrevious={() => {}}
              onNext={() => {}}
              onTogglePlayMode={() => {}}
              onTogglePlayback={handleTogglePlayback}
              onToggleMute={handleToggleMute}
              onVolumeChange={handleVolumeChange}
            />
          </div>
        </div>
      </div>
    </div>
  );
}
