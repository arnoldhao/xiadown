import {
MediaPlayer,
MediaProvider,
MediaRemoteControl,
getTimeRangesEnd
} from "@vidstack/react";
import { Call,Events,System,Window } from "@wailsio/runtime";
import {
Download,
FolderOpen,
Heart,
Loader2,
PanelLeftClose,
PanelLeftOpen,
Pause,
Play,
Ratio,
Repeat2,
Shuffle,
SkipBack,
SkipForward,
Square,
Volume2,
VolumeX
} from "lucide-react";
import * as React from "react";

import {
getXiaText
} from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
import { LISTEN_DEFAULT_COVER_IMAGE_URL } from "@/shared/assets/default-cover";
import type { Pet } from "@/shared/contracts/pets";
import { messageBus } from "@/shared/message";
import { openExternalURL,useLyricsTranscriptionAvailable } from "@/shared/query/system";
import { useSettingsStore } from "@/shared/store/settings";
import { PetDisplay } from "@/shared/ui/pet-player";
import {
Tooltip,
TooltipContent,
TooltipProvider,
TooltipTrigger
} from "@/shared/ui/tooltip";
import {
LISTEN_HIDDEN_ENGINE_STYLE,
LISTEN_PLAYER_SURFACE_WIDTH_CLASS,
LISTEN_PRIMARY_PLAY_BUTTON_CLASS,
LISTEN_PRIMARY_PLAY_BUTTON_HOVER_CLASS,
LISTEN_PRIMARY_PLAY_BUTTON_SIZE_CLASS,
LISTEN_PRIMARY_PLAY_ICON_SIZE_CLASS,
} from "@/shared/styles/listen";

import { LISTEN_LIVE_PLAYER_EVENT,LISTEN_LIVE_PLAYER_SERVICE,LISTEN_NATIVE_PLAYER_EVENT,LISTEN_NATIVE_PLAYER_SERVICE } from "@/app/main/listen/catalog";
import { callListenTrackLyrics } from "@/app/main/listen/lyrics-api";
import { ListenLyricsSurface } from "@/app/main/listen/lyrics";
import {
readListenNativeVideoRadius,
useListenNativeVideoUnderlay,
} from "@/app/main/listen/native-video-underlay";
import { copyListenTextToClipboard,fetchListenLyricsCached,forgetListenLyricsCache,getListenErrorCode,getListenErrorMessage,isListenLyricsDataAvailable,listenArtistCountFromLabelParts,listenLyricsSummary,LISTEN_EMPTY_PROGRESS,LISTEN_INLINE_VIDEO_FALLBACK_ASPECT_RATIO,logListenLyrics,normalizeListenInlineVideoAspectRatio,normalizeListenLiveNativeState,readListenLyricsCache,readListenNativeEventURLVideoId,resolveListenNativeEventVideoAspectRatio,resolveListenPlaybackStatusLabel,resolveListenTrackVideoAvailability,resolveTrustedListenOnlineArtistLabel,splitListenArtistLabel,type ListenArtistLabelPart,type ListenLyricsTrackRequest,type ListenVideoAvailability } from "@/app/main/listen/playback-helpers";
import { ListenLocalPlaybackQueuePopup,ListenPlaybackQueuePopup,type ListenQueuePopupAnchor } from "@/app/main/listen/queue-popups";
import {
  ListenCompactCoverSurface,
  ListenLocalCoverSurface,
  ListenPlayerFooter,
  ListenPlayerIconButton,
  ListenPlayerMoreMenu,
  type ListenAirPlayAnchor,
  type ListenMediaMode,
} from "@/app/main/listen/playback-ui";
import { ListenArtworkVisualizer,ListenInlineVisualizer } from "@/app/main/listen/Visualizer";
import { fetchListenTrackInfo } from "@/app/main/listen/api";
import { clampVolume,formatProgressSeconds,resolveAudioSource } from "@/app/main/listen/local-library";
import { buildListenPosterCandidates,buildYouTubeWatchURL } from "@/app/main/listen/storage";
import type { ListenLocalItem,ListenLyricsData,ListenLyricsKind,ListenMode,ListenNativePlayerEvent,ListenObservedPlaybackAudioQuality,ListenOnlineItem,ListenPlayMode,ListenPlayerCommand,ListenRemotePlaybackState,ListenTrackArtist } from "@/app/main/listen/types";
import { ListenOnlineArtwork,ListenSourceBadge } from "@/app/main/listen/ui";
import {
  isEqualizerArtworkVisualizerMode,
  isEqualizerSpectrumVisualizerMode,
  type EqualizerVisualizerMode,
} from "@/shared/contracts/equalizer";
import { useEqualizerSnapshot,useEqualizerVisualizerFrame } from "@/shared/query/equalizer";

type ListenNativeVideoRect = ListenAirPlayAnchor & {
  centerX?: number;
  centerY?: number;
  stageWidth?: number;
  stageHeight?: number;
  viewportWidth?: number;
  viewportHeight?: number;
  radius?: number;
  interactive?: boolean;
  sequence?: number;
};
type ListenLocalAirPlayMediaElement = HTMLMediaElement & {
  webkitShowPlaybackTargetPicker?: () => void;
  remote?: {
    prompt?: () => Promise<void>;
  };
};
const LISTEN_MEDIA_SPLIT_MIN_WIDTH = 760;
const LISTEN_LIVE_VIDEO_ASPECT_RATIO = 16 / 9;
const LISTEN_LIVE_VIDEO_TOPBAR_HEIGHT = 74;
const LISTEN_LIVE_VIDEO_FRAME_GAP = 10;
const LISTEN_LIVE_VIDEO_MIN_WINDOW_WIDTH = 960;
const LISTEN_LIVE_VIDEO_MIN_WINDOW_HEIGHT = 640;
const LISTEN_LIVE_VIDEO_GEOMETRY_SETTLE_DELAYS_MS = [
  32,
  80,
  140,
  220,
  340,
  480,
  680,
  920,
] as const;
const LISTEN_LIVE_VIDEO_EMBED_SETTLE_MS = 360;
const LISTEN_LIVE_VIDEO_REVEAL_MS = 780;
function createListenNativeVideoSequence(requestId: number) {
  return Date.now() * 1000 + requestId;
}

function listenArtistBrowseTrack(
  track: ListenOnlineItem,
  artist: string,
  labelParts: ListenArtistLabelPart[],
): ListenOnlineItem | null {
  const artistName = artist.trim();
  if (!artistName) {
    return null;
  }
  const linkedArtist = listenTrackArtistByName(track.artists, artistName);
  if (linkedArtist) {
    return {
      ...track,
      channel: linkedArtist.name,
      artists: [linkedArtist],
      artistBrowseId: linkedArtist.browseId,
      artistSource: linkedArtist.browseId ? "api-linked" : undefined,
      thumbnailUrl: linkedArtist.thumbnailUrl,
    };
  }
  const keepOriginalArtistLink =
    (listenArtistCountFromLabelParts(labelParts) <= 1 &&
      artistName === track.channel.trim()) ||
    (track.artistSource === "api-linked-multiple" &&
      artistName === labelParts.find((part) => part.kind === "artist")?.text.trim());
  return {
    ...track,
    channel: artistName,
    artistBrowseId: keepOriginalArtistLink ? track.artistBrowseId : undefined,
    artistSource: keepOriginalArtistLink ? track.artistSource : undefined,
  };
}

function listenTrackArtistByName(
  artists: ListenTrackArtist[] | undefined,
  name: string,
): ListenTrackArtist | null {
  const normalizedName = name.trim();
  if (!normalizedName || !Array.isArray(artists)) {
    return null;
  }
  return (
    artists.find((artist) => artist.name.trim() === normalizedName) ?? null
  );
}

function listenArtistLabelPartsFromTrackArtists(
  artists: ListenTrackArtist[] | undefined,
): ListenArtistLabelPart[] {
  if (!Array.isArray(artists) || artists.length === 0) {
    return [];
  }
  const parts: ListenArtistLabelPart[] = [];
  const seen = new Set<string>();
  for (const artist of artists) {
    const name = artist.name.trim();
    if (!name) {
      continue;
    }
    const key = artist.browseId?.trim() || name.toLocaleLowerCase();
    if (seen.has(key)) {
      continue;
    }
    seen.add(key);
    if (parts.length > 0) {
      parts.push({ kind: "separator", text: ", " });
    }
    parts.push({ kind: "artist", text: name });
  }
  return parts;
}

function ListenEmptyPlaybackChrome(props: {
  mode: ListenMode;
  listOpen: boolean;
  onToggleList: () => void;
  reserveWindowControls: boolean;
  muted: boolean;
  volume: number;
  playMode: ListenPlayMode;
  pet: Pet | null;
  petImageURL: string;
  text: ReturnType<typeof getXiaText>;
  onToggleMute: () => void;
  onVolumeChange: (value: number) => void;
}) {
  const noop = React.useCallback(() => {}, []);
  return (
    <div className="relative h-full min-h-0 overflow-hidden">
      <ListenPlayerChrome
        mediaMode="cover"
        reserveWindowControls={props.reserveWindowControls}
        airPlaySupported={false}
        sourceBadge={<ListenSourceBadge mode={props.mode} text={props.text} />}
        headerCover={
          <ListenCompactCoverSurface
            srcCandidates={[LISTEN_DEFAULT_COVER_IMAGE_URL]}
            title=""
          />
        }
        cover={
          <ListenLocalCoverSurface
            src={LISTEN_DEFAULT_COVER_IMAGE_URL}
            title=""
          />
        }
        lyrics={
          <ListenLyricsSurface
            text={props.text}
            lyrics={null}
          />
        }
        hasVideo={false}
        videoHidden
        live={props.mode === "hush"}
        fullscreenLive={props.mode === "hush" && !props.listOpen}
        listOpen={props.listOpen}
        onToggleList={props.onToggleList}
        pet={props.pet}
        petImageURL={props.petImageURL}
        lyricsAvailable={false}
        lyricsLoading={false}
        disabled
        title=""
        subtitle=""
        progress={LISTEN_EMPTY_PROGRESS}
        playing={false}
        loading={false}
        playbackState="idle"
        muted={props.muted}
        volume={props.volume}
        playMode={props.playMode}
        text={props.text}
        onMediaModeChange={noop}
        onPrevious={noop}
        onNext={noop}
        onPlayModeChange={noop}
        onTogglePlayback={noop}
        onToggleMute={props.onToggleMute}
        onVolumeChange={props.onVolumeChange}
      />
    </div>
  );
}

export function ListenPlayback(props: {
  mode: ListenMode;
  active: boolean;
  listOpen: boolean;
  onToggleList: () => void;
  reserveWindowControls: boolean;
  airPlaySupported: boolean;
  selectedOnline: ListenOnlineItem | null;
  selectedLocal: ListenLocalItem | null;
  httpBaseURL: string;
  onlineCommand: ListenPlayerCommand | null;
  onlinePlaybackEnabled: boolean;
  localCommand: ListenPlayerCommand | null;
  onlineQueueItems: ListenOnlineItem[];
  onlineQueueTitle: string;
  selectedOnlineId: string;
  localQueueItems: ListenLocalItem[];
  selectedLocalId: string;
  onlinePlaying: boolean;
  localPlaying: boolean;
  localResumeTime: number;
  onlineResumeTime: number;
  onlineProgress: {
    currentTime: number;
    duration: number;
    bufferedTime: number;
  };
  onlineState: ListenRemotePlaybackState;
  onlineObservedPlaybackAudioQuality: ListenObservedPlaybackAudioQuality | "";
  favoriteActive: boolean;
  favoriteBusy: boolean;
  pet: Pet | null;
  petImageURL: string;
  localProgress: {
    currentTime: number;
    duration: number;
    bufferedTime: number;
  };
  muted: boolean;
  volume: number;
  playMode: ListenPlayMode;
  text: ReturnType<typeof getXiaText>;
  onEnded: () => void;
  onOnlinePlayingChange: (playing: boolean) => void;
  onOnlineStateChange: (state: ListenRemotePlaybackState) => void;
  onOnlineProgressChange: (
    videoId: string,
    currentTime: number,
    duration: number,
    bufferedTime: number,
    transient?: boolean,
  ) => void;
  onOnlineNativeTrackChange: (event: ListenNativePlayerEvent) => void;
  onSelectOnlineQueueTrack: (item: ListenOnlineItem) => void;
  onClearOnlineQueue: () => void;
  onRemoveOnlineQueueItem: (item: ListenOnlineItem) => void;
  onMoveOnlineQueueItem: (item: ListenOnlineItem, direction: -1 | 1) => void;
  onUndoOnlineQueueEdit: () => void;
  onRedoOnlineQueueEdit: () => void;
  onlineQueueCanUndo: boolean;
  onlineQueueCanRedo: boolean;
  onSelectLocalQueueTrack: (item: ListenLocalItem) => void;
  onLocalPlayingChange: (playing: boolean) => void;
  onLocalProgressChange: (
    currentTime: number,
    duration: number,
    bufferedTime: number,
  ) => void;
  onLocalPlaybackIntent: () => void;
  onPrevious: () => void;
  onNext: () => void;
  onTogglePlayMode: () => void;
  onPlayModeChange: (mode: ListenPlayMode) => void;
  onTogglePlayback: () => void;
  onToggleMute: () => void;
  onVolumeChange: (value: number) => void;
  onToggleFavorite: () => void;
  onOpenOnlineArtist: (track: ListenOnlineItem) => void;
  onDownloadTrack: (url: string) => void;
  onOpenLocalDirectory: () => void;
}) {
  const playerRef = React.useRef<React.ElementRef<typeof MediaPlayer> | null>(
    null,
  );
  const localRemote = React.useMemo(() => new MediaRemoteControl(), []);
  const localResumeRef = React.useRef(props.localResumeTime);
  const localRestoreAppliedRef = React.useRef("");
  const localReplayStartedAtRef = React.useRef<number | null>(null);
  const pendingLocalCommandRef = React.useRef<ListenPlayerCommand | null>(null);
  const localTrack = props.selectedLocal;
  const [localMediaMode, setLocalMediaMode] =
    React.useState<ListenMediaMode>("cover");
  const [localQueueOpen, setLocalQueueOpen] = React.useState(false);
  const [localQueueAnchor, setLocalQueueAnchor] =
    React.useState<ListenQueuePopupAnchor | null>(null);
  const [localLyricsState, setLocalLyricsState] = React.useState<{
    lyricsId: string;
    loading: boolean;
    data: ListenLyricsData | null;
    error: string;
  }>({
    lyricsId: "",
    loading: false,
    data: null,
    error: "",
  });
  const localLyricsAvailable = isListenLyricsDataAvailable(localLyricsState.data);
  const localFullscreenLyricsDefaultKeyRef = React.useRef("");
  const localLyricsRetryKeyRef = React.useRef("");
  const [localLyricsRetryToken, setLocalLyricsRetryToken] = React.useState(0);
  const syncedLyricsEnabled = useSettingsStore(
    (state) => state.settings?.syncedLyricsEnabled !== false,
  );
  const romanizedLyricsSetting = useSettingsStore(
    (state) => state.settings?.romanizedLyrics !== false,
  );
  const pinyinLyricsSetting = useSettingsStore(
    (state) => state.settings?.pinyinLyrics !== false,
  );
  const isMac = System.IsMac();
  const isWindows = System.IsWindows();
  const visualizerPlatformSupported = isMac || isWindows;
  const equalizerSnapshot = useEqualizerSnapshot(visualizerPlatformSupported && props.active);
  const visualizerMode = equalizerSnapshot.data?.settings.visualizerMode ?? "off";
  const equalizerStatus = equalizerSnapshot.data?.status;
  const visualizerConfigured = visualizerPlatformSupported && props.active && visualizerMode !== "off";
  const visualizerEnabled =
    visualizerConfigured &&
    (isWindows || equalizerSnapshot.data?.settings.enabled === true) &&
    equalizerStatus?.supported === true &&
    equalizerStatus.permissionRequired !== true &&
    equalizerStatus.code !== "error" &&
    equalizerStatus.code !== "unsupported";
  const localArtworkVisualizerKey = `${localTrack?.id ?? ""}:${visualizerMode}:${visualizerEnabled}:${props.localPlaying}`;
  const [localArtworkVisualizerState, setLocalArtworkVisualizerState] = React.useState({
    key: "",
    visible: false,
  });
  const localArtworkVisualizerVisible =
    localArtworkVisualizerState.key === localArtworkVisualizerKey && localArtworkVisualizerState.visible;
  const handleLocalArtworkVisualizerVisibleChange = React.useCallback((visible: boolean) => {
    setLocalArtworkVisualizerState({ key: localArtworkVisualizerKey, visible });
  }, [localArtworkVisualizerKey]);
  const lyricsTranscriptionAvailability = useLyricsTranscriptionAvailable(isMac);
  const lyricsTranscriptionAvailable =
    isMac && lyricsTranscriptionAvailability.data === true;
  const romanizedLyrics = lyricsTranscriptionAvailable && romanizedLyricsSetting;
  const pinyinLyrics = lyricsTranscriptionAvailable && pinyinLyricsSetting;

  const localLyricsTrack = React.useMemo<ListenLyricsTrackRequest | null>(() => {
    if (!localTrack) {
      return null;
    }
    return {
      lyricsId: `local:${localTrack.id}`,
      title: localTrack.lyricsTitle || localTrack.title,
      artist: localTrack.lyricsArtist || localTrack.author,
      durationLabel: localTrack.durationLabel,
    };
  }, [
    localTrack?.author,
    localTrack?.durationLabel,
    localTrack?.id,
    localTrack?.lyricsArtist,
    localTrack?.lyricsTitle,
    localTrack?.title,
  ]);
  const retryLocalLyrics = React.useCallback(() => {
    const lyricsId = String(localLyricsTrack?.lyricsId || "").trim();
    if (!lyricsId) {
      return;
    }
    localLyricsRetryKeyRef.current = lyricsId;
    forgetListenLyricsCache(lyricsId, props.text.locale);
    setLocalLyricsRetryToken((value) => value + 1);
  }, [localLyricsTrack?.lyricsId, props.text.locale]);

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

  const handleLocalAirPlay = React.useCallback(() => {
    const media = getLocalMediaElement() as
      | ListenLocalAirPlayMediaElement
      | null;
    if (!media) {
      return;
    }
    if (typeof media.webkitShowPlaybackTargetPicker === "function") {
      media.webkitShowPlaybackTargetPicker();
      return;
    }
    if (typeof media.remote?.prompt === "function") {
      void media.remote.prompt().catch((error) => {
        console.warn("[Listen] local AirPlay picker unavailable", error);
      });
    }
  }, [getLocalMediaElement]);

  const runLocalPlayerCommand = React.useCallback(
    (command: ListenPlayerCommand) => {
      const player = playerRef.current;
      if (!player) {
        pendingLocalCommandRef.current =
          command.command === "play" ||
          command.command === "replay" ||
          command.command === "resume" ||
          command.command === "seek"
            ? command
            : null;
        return;
      }
      if (command.command === "replay") {
        const media = getLocalMediaElement();
        pendingLocalCommandRef.current = null;
        if (media) {
          media.currentTime = 0;
          void media.play().catch(() => {});
        }
        player.currentTime = 0;
        player.paused = false;
        return;
      }
      if (command.command === "play" || command.command === "resume") {
        const media = getLocalMediaElement();
        pendingLocalCommandRef.current = null;
        if (media) {
          void media.play().catch(() => {});
        }
        player.paused = false;
        return;
      }
      if (command.command === "seek") {
        const media = getLocalMediaElement();
        const seconds = Math.max(0, command.startSeconds ?? 0);
        pendingLocalCommandRef.current = null;
        if (media) {
          media.currentTime = seconds;
        }
        player.currentTime = seconds;
        return;
      }
      const media = getLocalMediaElement();
      pendingLocalCommandRef.current = null;
      localReplayStartedAtRef.current = null;
      media?.pause();
      player.paused = true;
    },
    [getLocalMediaElement],
  );

  const handleLocalTogglePlayback = React.useCallback<
    React.MouseEventHandler<HTMLButtonElement>
  >((event) => {
    props.onLocalPlaybackIntent();
    if (props.localPlaying) {
      localRemote.pause(event.nativeEvent);
      return;
    }
    localRemote.play(event.nativeEvent);
  }, [localRemote, props.localPlaying, props.onLocalPlaybackIntent]);

  React.useEffect(() => {
    if (props.mode !== "linger" || !props.localCommand) {
      return;
    }
    runLocalPlayerCommand(props.localCommand);
  }, [props.localCommand, props.mode, runLocalPlayerCommand]);

  React.useEffect(() => {
    const player = playerRef.current;
    if (!player) {
      return;
    }
    localRemote.setPlayer(player);
  }, [localRemote, localTrack?.id, props.mode]);

  React.useEffect(() => {
    if (props.mode !== "linger") {
      return;
    }
    const media = getLocalMediaElement();
    if (!media) {
      return;
    }
    media.setAttribute("x-webkit-airplay", "allow");
    media.disableRemotePlayback = false;
  }, [getLocalMediaElement, localTrack?.id, props.mode]);

  React.useEffect(() => {
    if (props.mode !== "linger") {
      return;
    }
    const player = playerRef.current;
    const media = getLocalMediaElement();
    if (!player && !media) {
      return;
    }
    const nextVolume = clampVolume(props.volume);
    const nextMuted = props.muted || props.volume <= 0;
    if (media) {
      media.volume = nextVolume;
      media.muted = nextMuted;
    }
    if (player) {
      player.volume = nextVolume;
      player.muted = nextMuted;
    }
  }, [
    getLocalMediaElement,
    props.mode,
    props.muted,
    props.volume,
    localTrack?.id,
  ]);

  React.useEffect(() => {
    if (props.mode !== "linger" || !localTrack) {
      localReplayStartedAtRef.current = null;
      props.onLocalProgressChange(0, 0, 0);
      return;
    }

    const syncProgress = () => {
      const player = playerRef.current;
      const media = getLocalMediaElement();
      const source = media ?? player;
      let currentTime =
        source && Number.isFinite(source.currentTime)
          ? Math.max(0, source.currentTime)
          : 0;
      const duration =
        source && Number.isFinite(source.duration)
          ? Math.max(0, source.duration)
          : Math.max(0, props.localProgress.duration);
      const buffered = (source as { buffered?: TimeRanges } | null)?.buffered;
      const bufferedTime = buffered
        ? Math.max(0, getTimeRangesEnd(buffered) ?? 0)
        : Math.max(0, Math.min(props.localProgress.bufferedTime, duration));
      const replayStartedAt = localReplayStartedAtRef.current;
      const paused =
        media?.paused ?? (player ? Boolean(player.paused) : true);
      if (currentTime > 0.05) {
        localReplayStartedAtRef.current = null;
      } else if (
        replayStartedAt !== null &&
        props.playMode === "repeat" &&
        !paused
      ) {
        currentTime = Math.max(
          0,
          Math.min((performance.now() - replayStartedAt) / 1000, duration),
        );
      }
      props.onLocalProgressChange(currentTime, duration, bufferedTime);
    };

    syncProgress();
    const timer = window.setInterval(syncProgress, 250);
    return () => window.clearInterval(timer);
  }, [
    getLocalMediaElement,
    localTrack?.id,
    props.localProgress.bufferedTime,
    props.localProgress.duration,
    props.mode,
    props.onLocalProgressChange,
    props.playMode,
  ]);

  const handleLocalSeek = React.useCallback(
    (seconds: number) => {
      if (props.mode !== "linger" || !localTrack) {
        return;
      }
      const player = playerRef.current;
      const media = getLocalMediaElement();
      const source = media ?? player;
      const duration =
        source && Number.isFinite(source.duration)
          ? Math.max(0, source.duration)
          : Math.max(0, props.localProgress.duration);
      if (duration <= 0) {
        return;
      }
      const nextTime = Math.max(0, Math.min(seconds, duration));
      localReplayStartedAtRef.current = null;
      if (media) {
        media.currentTime = nextTime;
      }
      if (player) {
        player.currentTime = nextTime;
      }
      const buffered = (source as { buffered?: TimeRanges } | null)?.buffered;
      const bufferedTime = buffered
        ? Math.max(0, getTimeRangesEnd(buffered) ?? 0)
        : Math.max(0, Math.min(props.localProgress.bufferedTime, duration));
      props.onLocalProgressChange(nextTime, duration, bufferedTime);
    },
    [
      getLocalMediaElement,
      localTrack,
      props.localProgress.bufferedTime,
      props.localProgress.duration,
      props.mode,
      props.onLocalProgressChange,
    ],
  );

  const handleLocalTimeUpdate = React.useCallback(
    (currentTime: number) => {
      if (props.mode !== "linger" || !localTrack) {
        return;
      }
      const player = playerRef.current;
      const media = getLocalMediaElement();
      const source = media ?? player;
      const duration =
        source && Number.isFinite(source.duration)
          ? Math.max(0, source.duration)
          : Math.max(0, props.localProgress.duration);
      if (duration <= 0) {
        return;
      }
      const sourceTime =
        source && Number.isFinite(source.currentTime)
          ? Math.max(0, source.currentTime)
          : currentTime;
      const nextTime = Math.max(0, Math.min(sourceTime, duration));
      if (nextTime > 0.05) {
        localReplayStartedAtRef.current = null;
      }
      const buffered = (source as { buffered?: TimeRanges } | null)?.buffered;
      const bufferedTime = buffered
        ? Math.max(0, getTimeRangesEnd(buffered) ?? 0)
        : Math.max(0, Math.min(props.localProgress.bufferedTime, duration));
      props.onLocalProgressChange(nextTime, duration, bufferedTime);
    },
    [
      getLocalMediaElement,
      localTrack,
      props.localProgress.bufferedTime,
      props.localProgress.duration,
      props.mode,
      props.onLocalProgressChange,
    ],
  );

  const syncLocalReplayState = React.useCallback(() => {
    const player = playerRef.current;
    const media = getLocalMediaElement();
    const source = media ?? player;
    const duration =
      source && Number.isFinite(source.duration)
        ? Math.max(0, source.duration)
        : Math.max(0, props.localProgress.duration);
    const buffered = (source as { buffered?: TimeRanges } | null)?.buffered;
    const bufferedTime = buffered
      ? Math.max(0, getTimeRangesEnd(buffered) ?? 0)
      : Math.max(0, Math.min(props.localProgress.bufferedTime, duration));
    if (media) {
      media.currentTime = 0;
    }
    if (player) {
      player.currentTime = 0;
    }
    localReplayStartedAtRef.current = performance.now();
    props.onLocalProgressChange(0, duration, bufferedTime);
    props.onLocalPlayingChange(true);
  }, [
    getLocalMediaElement,
    props.localProgress.bufferedTime,
    props.localProgress.duration,
    props.onLocalPlayingChange,
    props.onLocalProgressChange,
  ]);

  React.useEffect(() => {
    localReplayStartedAtRef.current = null;
    localRestoreAppliedRef.current = "";
    localResumeRef.current = props.localResumeTime;
  }, [localTrack?.id, props.mode]);

  React.useEffect(() => {
    if (props.mode !== "linger" || !localTrack) {
      return;
    }
    const resumeSeconds = Math.max(0, localResumeRef.current);
    if (
      resumeSeconds <= 0 ||
      localRestoreAppliedRef.current === localTrack.id
    ) {
      return;
    }
    let attempts = 0;
    const timer = window.setInterval(() => {
      const player = playerRef.current;
      if (!player) {
        return;
      }
      const duration = Number.isFinite(player.duration)
        ? Math.max(0, player.duration)
        : 0;
      if (duration <= 0 && attempts < 30) {
        attempts += 1;
        return;
      }
      const target =
        duration > 0
          ? Math.min(resumeSeconds, Math.max(duration - 1, 0))
          : resumeSeconds;
      if (target > 0.5) {
        player.currentTime = target;
      }
      localRestoreAppliedRef.current = localTrack.id;
      window.clearInterval(timer);
    }, 160);
    return () => window.clearInterval(timer);
  }, [localTrack?.id, props.mode]);

  React.useEffect(() => {
    if (props.mode !== "linger" || !localTrack || !localLyricsTrack) {
      logListenLyrics("local skip fetch", {
        reason: props.mode !== "linger"
          ? "mode-not-linger"
          : !localTrack
            ? "missing-track"
            : "missing-lyrics-track",
        mode: props.mode,
      });
      setLocalLyricsState({
        lyricsId: "",
        loading: false,
        data: null,
        error: "",
      });
      if (props.mode !== "linger" || !localTrack) {
        setLocalMediaMode("cover");
      }
      setLocalQueueOpen(false);
      return;
    }
    const lyricsId = String(localLyricsTrack.lyricsId || "").trim();
    if (!lyricsId || !localLyricsTrack.title.trim()) {
      logListenLyrics("local skip fetch", {
        reason: !lyricsId ? "missing-lyrics-id" : "missing-title",
        lyricsId,
        title: localLyricsTrack.title,
      });
      setLocalLyricsState({
        lyricsId,
        loading: false,
        data: null,
        error: "",
      });
      return;
    }
    const forceRequest = localLyricsRetryKeyRef.current === lyricsId;
    if (forceRequest) {
      localLyricsRetryKeyRef.current = "";
    }
    const lyricsMode = { synced: syncedLyricsEnabled };
    const cachedLyrics = forceRequest ? null : readListenLyricsCache(lyricsId, props.text.locale, lyricsMode);
    const refreshCachedPlain = syncedLyricsEnabled && cachedLyrics?.kind === "plain";
    logListenLyrics("local request state", {
      lyricsId,
      title: localLyricsTrack.title,
      artist: localLyricsTrack.artist,
      durationLabel: localLyricsTrack.durationLabel || "",
      language: props.text.locale,
      forceRequest,
      cached: listenLyricsSummary(cachedLyrics),
      refreshCachedPlain,
      synced: syncedLyricsEnabled,
    });
    if (cachedLyrics && !refreshCachedPlain) {
      logListenLyrics("local use cached", {
        lyricsId,
        cached: listenLyricsSummary(cachedLyrics),
      });
      setLocalLyricsState({
        lyricsId,
        loading: false,
        data: cachedLyrics,
        error: "",
      });
      return;
    }
    let cancelled = false;
    setLocalLyricsState({
      lyricsId,
      loading: true,
      data: cachedLyrics,
      error: "",
    });
    void fetchListenLyricsCached(
      props.httpBaseURL,
      localLyricsTrack,
      props.localProgress.duration,
      props.text.locale,
      { force: forceRequest, refreshPlain: refreshCachedPlain, synced: syncedLyricsEnabled },
    )
      .then((data) => {
        if (cancelled) {
          logListenLyrics("local fetch ignored after cancel", {
            lyricsId,
            result: listenLyricsSummary(data),
          });
          return;
        }
        logListenLyrics("local fetch apply", {
          lyricsId,
          result: listenLyricsSummary(data),
        });
        setLocalLyricsState({
          lyricsId,
          loading: false,
          data,
          error: "",
        });
      })
      .catch((error: unknown) => {
        if (cancelled) {
          logListenLyrics("local fetch error ignored after cancel", {
            lyricsId,
            error: getListenErrorMessage(error),
            code: getListenErrorCode(error),
          });
          return;
        }
        logListenLyrics("local fetch error apply", {
          lyricsId,
          cached: listenLyricsSummary(cachedLyrics),
          error: getListenErrorMessage(error),
          code: getListenErrorCode(error),
        });
        setLocalLyricsState({
          lyricsId,
          loading: false,
          data: cachedLyrics,
          error: cachedLyrics
            ? ""
            : getListenErrorMessage(error) || props.text.listen.lyricsEmpty,
        });
      });
    return () => {
      cancelled = true;
    };
  }, [
    localLyricsTrack,
    props.text.listen.lyricsEmpty,
    props.text.locale,
    props.httpBaseURL,
    localTrack?.id,
    localLyricsRetryToken,
    props.mode,
    syncedLyricsEnabled,
  ]);

  React.useEffect(() => {
    if (props.mode !== "linger" || props.listOpen || !localTrack) {
      localFullscreenLyricsDefaultKeyRef.current = "";
      return;
    }
    const defaultKey = `${localTrack.id}:${localLyricsState.lyricsId}`;
    if (
      localMediaMode !== "cover" ||
      !localLyricsAvailable ||
      localFullscreenLyricsDefaultKeyRef.current === defaultKey
    ) {
      return;
    }
    localFullscreenLyricsDefaultKeyRef.current = defaultKey;
    setLocalMediaMode("lyrics");
  }, [
    localLyricsAvailable,
    localLyricsState.lyricsId,
    localMediaMode,
    localTrack,
    props.listOpen,
    props.mode,
  ]);

  if (props.mode !== "linger") {
    const track = props.selectedOnline;
    if (!track) {
      return (
        <ListenEmptyPlaybackChrome
          mode={props.mode}
          listOpen={props.listOpen}
          onToggleList={props.onToggleList}
          reserveWindowControls={props.reserveWindowControls}
          muted={props.muted}
          volume={props.volume}
          playMode={props.playMode}
          pet={props.pet}
          petImageURL={props.petImageURL}
          text={props.text}
          onToggleMute={props.onToggleMute}
          onVolumeChange={props.onVolumeChange}
        />
      );
    }

    return (
      <ListenYouTubePlayback
        mode={props.mode}
        active={props.active}
        listOpen={props.listOpen}
        onToggleList={props.onToggleList}
        reserveWindowControls={props.reserveWindowControls}
        airPlaySupported={props.airPlaySupported}
        track={track}
        httpBaseURL={props.httpBaseURL}
        command={props.onlineCommand}
        enabled={props.onlinePlaybackEnabled}
        queueItems={props.onlineQueueItems}
        queueTitle={props.onlineQueueTitle}
        selectedQueueId={props.selectedOnlineId}
        resumeSeconds={props.onlineResumeTime}
        progress={props.onlineProgress}
        playing={props.onlinePlaying}
        playMode={props.playMode}
        favoriteActive={props.favoriteActive}
        favoriteBusy={props.favoriteBusy}
        muted={props.muted}
        volume={props.volume}
        state={props.onlineState}
        observedPlaybackAudioQuality={
          props.mode === "muse" ? (props.onlineObservedPlaybackAudioQuality ?? "") : undefined
        }
        pet={props.pet}
        petImageURL={props.petImageURL}
        text={props.text}
        onEnded={props.onEnded}
        onPlayingChange={props.onOnlinePlayingChange}
        onStateChange={props.onOnlineStateChange}
        onProgressChange={props.onOnlineProgressChange}
        onNativeTrackChange={props.onOnlineNativeTrackChange}
        onSelectQueueTrack={props.onSelectOnlineQueueTrack}
        onClearQueue={props.onClearOnlineQueue}
        onRemoveQueueItem={props.onRemoveOnlineQueueItem}
        onMoveQueueItem={props.onMoveOnlineQueueItem}
        onUndoQueueEdit={props.onUndoOnlineQueueEdit}
        onRedoQueueEdit={props.onRedoOnlineQueueEdit}
        queueCanUndo={props.onlineQueueCanUndo}
        queueCanRedo={props.onlineQueueCanRedo}
        onPrevious={props.onPrevious}
        onNext={props.onNext}
        onTogglePlayMode={props.onTogglePlayMode}
        onPlayModeChange={props.onPlayModeChange}
        onTogglePlayback={props.onTogglePlayback}
        onToggleMute={props.onToggleMute}
        onVolumeChange={props.onVolumeChange}
        onToggleFavorite={props.onToggleFavorite}
        onOpenArtist={props.onOpenOnlineArtist}
        onDownloadTrack={props.onDownloadTrack}
        syncedLyricsEnabled={syncedLyricsEnabled}
        romanizedLyrics={romanizedLyrics}
        pinyinLyrics={pinyinLyrics}
        visualizerMode={visualizerMode}
        visualizerEnabled={visualizerEnabled}
      />
    );
  }

  const track = localTrack;
  if (!track) {
    return (
      <ListenEmptyPlaybackChrome
        mode="linger"
        listOpen={props.listOpen}
        onToggleList={props.onToggleList}
        reserveWindowControls={props.reserveWindowControls}
        muted={props.muted}
        volume={props.volume}
        playMode={props.playMode}
        pet={props.pet}
        petImageURL={props.petImageURL}
        text={props.text}
        onToggleMute={props.onToggleMute}
        onVolumeChange={props.onVolumeChange}
      />
    );
  }

  return (
    <div className={cn(
      "relative h-full min-h-0",
      localArtworkVisualizerVisible ? "overflow-visible" : "overflow-hidden",
    )}>
      <MediaPlayer
        ref={playerRef}
        key={track.id}
        src={resolveAudioSource(track.previewURL, track.path)}
        title={track.title}
        viewType="audio"
        streamType="on-demand"
        load="eager"
        preload="metadata"
        loop={false}
        playsInline
        onPlay={() => props.onLocalPlayingChange(true)}
        onPause={() => {
          localReplayStartedAtRef.current = null;
          props.onLocalPlayingChange(false);
        }}
        onReplay={() => syncLocalReplayState()}
        onTimeUpdate={(detail) => handleLocalTimeUpdate(detail.currentTime)}
        onEnded={() => {
          props.onLocalPlayingChange(false);
          props.onEnded();
        }}
        onCanPlay={() => {
          if (pendingLocalCommandRef.current) {
            runLocalPlayerCommand(pendingLocalCommandRef.current);
          }
        }}
        className="pointer-events-none"
        style={LISTEN_HIDDEN_ENGINE_STYLE}
      >
        <MediaProvider />
      </MediaPlayer>

      <ListenPlayerChrome
        mediaMode={localMediaMode}
        reserveWindowControls={props.reserveWindowControls}
        airPlaySupported={props.airPlaySupported}
        sourceBadge={<ListenSourceBadge mode="linger" text={props.text} />}
        headerCover={
          <ListenCompactCoverSurface
            key={track.id}
            srcCandidates={[track.coverURL || LISTEN_DEFAULT_COVER_IMAGE_URL]}
            title={track.title}
          />
        }
        cover={
          <ListenLocalCoverSurface
            key={track.id}
            src={track.coverURL || LISTEN_DEFAULT_COVER_IMAGE_URL}
            title={track.title}
            visualizerVisible={localArtworkVisualizerVisible}
            visualizer={
              isEqualizerArtworkVisualizerMode(visualizerMode) && visualizerEnabled ? (
                <ListenArtworkVisualizerBridge
                  mode={visualizerMode}
                  enabled={visualizerEnabled}
                  active={props.localPlaying}
                  onVisibleChange={handleLocalArtworkVisualizerVisibleChange}
                />
              ) : null
            }
          />
        }
        lyrics={
          <ListenLyricsSurface
            text={props.text}
            lyrics={localLyricsState.data}
            loading={localLyricsState.loading}
            error={localLyricsState.error}
            onRetry={localLyricsState.error ? retryLocalLyrics : undefined}
            currentTimeMs={Math.max(0, props.localProgress.currentTime * 1000)}
            timelineRunning={props.localPlaying}
            romanized={romanizedLyrics}
            pinyin={pinyinLyrics}
            onSeek={handleLocalSeek}
          />
        }
        hasVideo={false}
        videoHidden
        lyricsAvailable={localLyricsAvailable || Boolean(localLyricsState.error)}
        lyricsKind={localLyricsState.data?.kind}
        lyricsLoading={!localLyricsAvailable && localLyricsState.loading}
        title={track.title}
        subtitle={track.author}
        infoActions={
          <>
            <ListenPlayerIconButton
              label={props.text.actions.openDirectory}
              disabled={!track.path}
              onClick={props.onOpenLocalDirectory}
            >
              <FolderOpen className="h-4 w-4" />
            </ListenPlayerIconButton>
          </>
        }
        progress={props.localProgress}
        onSeek={handleLocalSeek}
        playing={props.localPlaying}
        loading={false}
        muted={props.muted}
        volume={props.volume}
        playMode={props.playMode}
        text={props.text}
        onAirPlay={props.airPlaySupported ? handleLocalAirPlay : undefined}
        onMediaModeChange={setLocalMediaMode}
        onPrevious={props.onPrevious}
        onNext={props.onNext}
        onPlayModeChange={props.onPlayModeChange}
        onTogglePlayback={handleLocalTogglePlayback}
        onToggleMute={props.onToggleMute}
        onVolumeChange={props.onVolumeChange}
        onToggleQueue={(anchor) => {
          setLocalQueueAnchor(anchor);
          setLocalQueueOpen((current) => !current);
        }}
        visualizerMode={visualizerMode}
        visualizerEnabled={visualizerEnabled}
        visualizerActive={props.localPlaying}
        queueOpen={localQueueOpen}
        queueOverlay={
          localQueueOpen ? (
            <ListenLocalPlaybackQueuePopup
              anchor={localQueueAnchor}
              queueTitle={props.text.listen.upNext}
              queueItems={props.localQueueItems}
              selectedQueueId={props.selectedLocalId}
              text={props.text}
              onSelectQueueTrack={props.onSelectLocalQueueTrack}
              onClose={() => setLocalQueueOpen(false)}
            />
          ) : null
        }
      />
    </div>
  );
}

export function ListenYouTubePlayback(props: {
  mode: Exclude<ListenMode, "linger">;
  active: boolean;
  listOpen: boolean;
  onToggleList: () => void;
  reserveWindowControls: boolean;
  airPlaySupported: boolean;
  track: ListenOnlineItem;
  httpBaseURL: string;
  command: ListenPlayerCommand | null;
  enabled: boolean;
  queueItems: ListenOnlineItem[];
  queueTitle: string;
  selectedQueueId: string;
  resumeSeconds: number;
  progress: {
    currentTime: number;
    duration: number;
    bufferedTime: number;
  };
  playing: boolean;
  playMode: ListenPlayMode;
  favoriteActive: boolean;
  favoriteBusy: boolean;
  muted: boolean;
  volume: number;
  state: ListenRemotePlaybackState;
  observedPlaybackAudioQuality?: ListenObservedPlaybackAudioQuality | "";
  pet: Pet | null;
  petImageURL: string;
  text: ReturnType<typeof getXiaText>;
  onEnded: () => void;
  onPlayingChange: (playing: boolean) => void;
  onStateChange: (state: ListenRemotePlaybackState) => void;
  onProgressChange: (
    videoId: string,
    currentTime: number,
    duration: number,
    bufferedTime: number,
    transient?: boolean,
  ) => void;
  onNativeTrackChange: (event: ListenNativePlayerEvent) => void;
  onSelectQueueTrack: (item: ListenOnlineItem) => void;
  onClearQueue: () => void;
  onRemoveQueueItem: (item: ListenOnlineItem) => void;
  onMoveQueueItem: (item: ListenOnlineItem, direction: -1 | 1) => void;
  onUndoQueueEdit: () => void;
  onRedoQueueEdit: () => void;
  queueCanUndo: boolean;
  queueCanRedo: boolean;
  onPrevious: () => void;
  onNext: () => void;
  onTogglePlayMode: () => void;
  onPlayModeChange: (mode: ListenPlayMode) => void;
  onTogglePlayback: () => void;
  onToggleMute: () => void;
  onVolumeChange: (value: number) => void;
  onToggleFavorite: () => void;
  onOpenArtist: (track: ListenOnlineItem) => void;
  onDownloadTrack: (url: string) => void;
  syncedLyricsEnabled: boolean;
  romanizedLyrics: boolean;
  pinyinLyrics: boolean;
  visualizerMode: EqualizerVisualizerMode;
  visualizerEnabled: boolean;
}) {
  const resumeRef = React.useRef(props.resumeSeconds);
  const artworkVisualizerKey = `${props.track.id}:${props.visualizerMode}:${props.visualizerEnabled}:${props.playing}`;
  const [artworkVisualizerState, setArtworkVisualizerState] = React.useState({
    key: "",
    visible: false,
  });
  const artworkVisualizerVisible = artworkVisualizerState.key === artworkVisualizerKey && artworkVisualizerState.visible;
  const handleArtworkVisualizerVisibleChange = React.useCallback((visible: boolean) => {
    setArtworkVisualizerState({ key: artworkVisualizerKey, visible });
  }, [artworkVisualizerKey]);
  const lastPlayRequestRef = React.useRef("");
  const handledNativeCommandRef = React.useRef("");
  const intendedVideoSinceRef = React.useRef(Date.now());
  const liveMismatchReplayAtRef = React.useRef(0);
  const isLive = props.track.group === "live";
  const playerService = isLive
    ? LISTEN_LIVE_PLAYER_SERVICE
    : LISTEN_NATIVE_PLAYER_SERVICE;
  const inactivePlayerService = isLive
    ? LISTEN_NATIVE_PLAYER_SERVICE
    : LISTEN_LIVE_PLAYER_SERVICE;
  const playerEventName = isLive
    ? LISTEN_LIVE_PLAYER_EVENT
    : LISTEN_NATIVE_PLAYER_EVENT;
  const playerEventSource = isLive
    ? "listen-youtube-live-player"
    : "listen-youtube-music-player";
  const artistName = isLive
    ? props.track.channel.trim()
    : resolveTrustedListenOnlineArtistLabel(props.track);
  const artistLabelParts = React.useMemo(() => {
    if (!artistName) {
      return [];
    }
    if (!isLive) {
      const linkedArtistParts = listenArtistLabelPartsFromTrackArtists(props.track.artists);
      if (linkedArtistParts.length > 0) {
        return linkedArtistParts;
      }
    }
    if (
      isLive ||
      (props.track.artistBrowseId?.trim() &&
        props.track.artistSource !== "api-linked-multiple")
    ) {
      return [{ kind: "artist", text: artistName }] satisfies ListenArtistLabelPart[];
    }
    return splitListenArtistLabel(artistName);
  }, [artistName, isLive, props.track.artistBrowseId, props.track.artistSource, props.track.artists]);
  const handleSubtitleArtistClick = React.useCallback(
    (artist: string) => {
      if (isLive) {
        return;
      }
      const artistTrack = listenArtistBrowseTrack(props.track, artist, artistLabelParts);
      if (artistTrack) {
        props.onOpenArtist(artistTrack);
      }
    },
    [artistLabelParts, isLive, props.onOpenArtist, props.track],
  );
  const showFavoriteAction =
    props.mode === "muse" && !isLive;
  const trackVideoId = props.track.videoId.trim();
  const canCheckVideoAvailability = !isLive && trackVideoId !== "";
  const showDownloadAction = !isLive && props.track.videoId.trim() !== "";
  const sourceBadge = <ListenSourceBadge mode={props.mode} text={props.text} />;
  const trackPageURL = React.useMemo(() => {
    const videoId = props.track.videoId.trim();
    return videoId ? buildYouTubeWatchURL(videoId) : "";
  }, [props.track.videoId]);
  const [mediaMode, setMediaMode] = React.useState<ListenMediaMode>("cover");
  const mediaModeRef = React.useRef<ListenMediaMode>("cover");
  const fullscreenLyricsDefaultKeyRef = React.useRef("");
  const [videoAvailability, setVideoAvailability] =
    React.useState<ListenVideoAvailability>(() =>
      resolveListenTrackVideoAvailability(props.track, isLive),
    );
  const [videoAspectRatio, setVideoAspectRatio] = React.useState(
    LISTEN_INLINE_VIDEO_FALLBACK_ASPECT_RATIO,
  );
  const [playbackAdvertising, setPlaybackAdvertising] = React.useState(false);
  const [playbackAdvertisingLabel, setPlaybackAdvertisingLabel] =
    React.useState("");
  const [playbackAdvertisingProgress, setPlaybackAdvertisingProgress] =
    React.useState<{
      currentTime: number;
      duration: number;
      bufferedTime: number;
    } | null>(null);
  const [playbackErrorLabel, setPlaybackErrorLabel] = React.useState("");
  const [playbackErrorMessage, setPlaybackErrorMessage] = React.useState("");
  const [queueOpen, setQueueOpen] = React.useState(false);
  const [queueAnchor, setQueueAnchor] =
    React.useState<ListenQueuePopupAnchor | null>(null);
  const [lyricsState, setLyricsState] = React.useState<{
    videoId: string;
    loading: boolean;
    data: ListenLyricsData | null;
    error: string;
  }>({
    videoId: props.track.videoId,
    loading: false,
    data: null,
    error: "",
	  });
	  const lyricsRetryKeyRef = React.useRef("");
	  const [lyricsRetryToken, setLyricsRetryToken] = React.useState(0);
	  const [lyricsCurrentTimeMs, setLyricsCurrentTimeMs] = React.useState(() =>
	    Math.max(0, props.progress.currentTime * 1000),
	  );
  const hasPlayableBuffer =
    props.state === "playing" ||
    (isLive && props.playing && props.state === "buffering") ||
    props.progress.currentTime > 0.15 ||
    props.progress.bufferedTime > 0.15;
  const transportLoading =
    props.enabled &&
    (props.state === "loading" ||
      (props.state === "buffering" && !hasPlayableBuffer));
  const progressLoading =
    !playbackAdvertising &&
    props.state !== "error" &&
    (props.state === "loading" || props.state === "buffering");
  const hasVideo = isLive
    ? trackVideoId !== ""
    : videoAvailability === "available";
  const videoLoading =
    canCheckVideoAvailability && videoAvailability === "checking";
  const [liveNativeVideoAvailable, setLiveNativeVideoAvailable] =
    React.useState(true);
  const [liveNativeVideoShown, setLiveNativeVideoShown] =
    React.useState(false);
  const [liveNativeVideoSuspended, setLiveNativeVideoSuspended] =
    React.useState(false);
  const [liveNativeVideoSettled, setLiveNativeVideoSettled] =
    React.useState(false);
  const liveNativeVideoRectRef = React.useRef<ListenNativeVideoRect | null>(null);
  const liveNativeVideoRequestRef = React.useRef(0);
  const [inlineNativeVideoAvailable, setInlineNativeVideoAvailable] =
    React.useState(true);
  const [inlineNativeVideoShown, setInlineNativeVideoShown] =
    React.useState(false);
  const [inlineNativeVideoSettled, setInlineNativeVideoSettled] =
    React.useState(false);
  const inlineNativeVideoRectRef = React.useRef<ListenNativeVideoRect | null>(null);
  const inlineNativeVideoRequestRef = React.useRef(0);
  const liveFullscreenActive =
    props.active &&
    props.mode === "hush" &&
    isLive &&
    !props.listOpen;
  const liveNativeVideoHasSurface =
    liveFullscreenActive &&
    hasVideo &&
    props.enabled;
  const liveVideoModeEligible =
    liveFullscreenActive &&
    liveNativeVideoAvailable &&
    !liveNativeVideoSuspended &&
    liveNativeVideoHasSurface;
  const liveVideoModeActive =
    liveVideoModeEligible &&
    liveNativeVideoSettled;
  const liveVideoVisible =
    liveVideoModeActive &&
    liveNativeVideoShown &&
    hasPlayableBuffer;
  const inlineVideoHasSurface =
    props.active &&
    !isLive &&
    mediaMode === "video" &&
    hasVideo &&
    props.enabled;
  const inlineVideoModeEligible =
    inlineVideoHasSurface &&
    inlineNativeVideoAvailable;
  const inlineVideoModeActive =
    inlineVideoModeEligible &&
    inlineNativeVideoSettled;
  const inlineVideoRevealReady =
    inlineVideoModeActive &&
    hasPlayableBuffer;
  const inlineVideoVisible =
    inlineVideoRevealReady &&
    inlineNativeVideoShown;
  const retryLyrics = React.useCallback(() => {
    const videoId = props.track.videoId.trim();
    if (!videoId) {
      return;
    }
    lyricsRetryKeyRef.current = videoId;
    forgetListenLyricsCache(videoId, props.text.locale);
    setLyricsRetryToken((value) => value + 1);
  }, [props.text.locale, props.track.videoId]);

  React.useEffect(() => {
    resumeRef.current = props.resumeSeconds;
  }, [props.resumeSeconds, props.track.videoId]);

  React.useEffect(() => {
    setLyricsCurrentTimeMs(Math.max(0, props.progress.currentTime * 1000));
  }, [props.progress.currentTime, props.track.videoId]);

  React.useEffect(() => {
    intendedVideoSinceRef.current = Date.now();
    setQueueOpen(false);
    setVideoAspectRatio(LISTEN_INLINE_VIDEO_FALLBACK_ASPECT_RATIO);
    setInlineNativeVideoAvailable(true);
    setInlineNativeVideoShown(false);
    inlineNativeVideoRectRef.current = null;
    setPlaybackAdvertising(false);
    setPlaybackAdvertisingLabel("");
    setPlaybackAdvertisingProgress(null);
    setPlaybackErrorLabel("");
    setPlaybackErrorMessage("");
    setLyricsState({
      videoId: props.track.videoId,
      loading: false,
      data: null,
      error: "",
    });
    if (isLive) {
      setMediaMode("cover");
    }
  }, [isLive, props.track.videoId]);

  React.useEffect(() => {
    setVideoAvailability(resolveListenTrackVideoAvailability(props.track, isLive));
  }, [
    isLive,
    props.track.hasVideo,
    props.track.musicVideoType,
    props.track.thumbnailUrl,
    props.track.videoAvailabilityKnown,
    props.track.videoId,
  ]);

  React.useEffect(() => {
    if (mediaMode !== "lyrics") {
      return;
    }
    if (isLive) {
      logListenLyrics("online skip fetch", {
        reason: "live",
        videoId: props.track.videoId,
        title: props.track.title,
      });
      setLyricsState({
        videoId: props.track.videoId,
        loading: false,
        data: null,
        error: "",
      });
      setMediaMode("cover");
      return;
    }
    const videoId = props.track.videoId.trim();
    if (!videoId) {
      logListenLyrics("online skip fetch", {
        reason: "missing-video-id",
        title: props.track.title,
      });
      setLyricsState({
        videoId,
        loading: false,
        data: null,
        error: "",
      });
      return;
    }
    const forceRequest = lyricsRetryKeyRef.current === videoId;
    if (forceRequest) {
      lyricsRetryKeyRef.current = "";
    }
    const lyricsMode = { synced: props.syncedLyricsEnabled };
    const cachedLyrics = forceRequest ? null : readListenLyricsCache(videoId, props.text.locale, lyricsMode);
    const refreshCachedPlain = props.syncedLyricsEnabled && cachedLyrics?.kind === "plain";
    logListenLyrics("online request state", {
      videoId,
      title: props.track.title,
      artist: props.track.channel,
      duration: props.progress.duration,
      language: props.text.locale,
      forceRequest,
      cached: listenLyricsSummary(cachedLyrics),
      refreshCachedPlain,
      synced: props.syncedLyricsEnabled,
    });
    if (cachedLyrics && !refreshCachedPlain) {
      logListenLyrics("online use cached", {
        videoId,
        cached: listenLyricsSummary(cachedLyrics),
      });
      setLyricsState({
        videoId,
        loading: false,
        data: cachedLyrics,
        error: "",
      });
      return;
    }
    let cancelled = false;
    setLyricsState({
      videoId,
      loading: true,
      data: cachedLyrics,
      error: "",
    });
    void callListenTrackLyrics({
      track: props.track,
      durationSeconds: props.progress.duration,
      language: props.text.locale,
      synced: props.syncedLyricsEnabled,
    })
      .then((data) => {
        if (cancelled) {
          logListenLyrics("online fetch ignored after cancel", {
            videoId,
            result: listenLyricsSummary(data),
          });
          return;
        }
        logListenLyrics("online fetch apply", {
          videoId,
          result: listenLyricsSummary(data),
        });
        setLyricsState({
          videoId,
          loading: false,
          data,
          error: "",
        });
      })
      .catch((error: unknown) => {
        if (cancelled) {
          logListenLyrics("online fetch error ignored after cancel", {
            videoId,
            error: getListenErrorMessage(error),
            code: getListenErrorCode(error),
          });
          return;
        }
        logListenLyrics("online fetch error apply", {
          videoId,
          cached: listenLyricsSummary(cachedLyrics),
          error: getListenErrorMessage(error),
          code: getListenErrorCode(error),
        });
        setLyricsState({
          videoId,
          loading: false,
          data: cachedLyrics,
          error: cachedLyrics
            ? ""
            : getListenErrorMessage(error) || props.text.listen.lyricsEmpty,
        });
      });
    return () => {
      cancelled = true;
    };
  }, [
    isLive,
    mediaMode,
    props.text.listen.lyricsEmpty,
    props.text.locale,
    props.track.channel,
    props.track.durationLabel,
    props.track.title,
    props.track.videoId,
    props.syncedLyricsEnabled,
    lyricsRetryToken,
  ]);

  const callNativePlayer = React.useCallback(
    (method: string, ...args: unknown[]) =>
      Call.ByName(`${playerService}.${method}`, ...args),
    [playerService],
  );

  const shouldPollLyricsTime =
    props.enabled &&
    !isLive &&
    mediaMode === "lyrics" &&
    lyricsState.data?.kind === "synced";

  React.useEffect(() => {
    if (isLive) {
      return;
    }
    if (!shouldPollLyricsTime) {
      void callNativePlayer("StopLyricsPoll").catch(() => {});
      return;
    }
    void callNativePlayer("StartLyricsPoll").catch(() => {});
    return () => {
      void callNativePlayer("StopLyricsPoll").catch(() => {});
    };
  }, [callNativePlayer, isLive, props.track.videoId, shouldPollLyricsTime]);

  const handleFitLiveVideoWindow = React.useCallback(() => {
    const rect = liveNativeVideoRectRef.current;
    const fallbackFrameWidth = Math.max(
      1,
      window.innerWidth - LISTEN_LIVE_VIDEO_FRAME_GAP * 2,
    );
    const currentStageWidth = Math.max(
      1,
      rect?.stageWidth ?? rect?.width ?? fallbackFrameWidth,
    );
    const currentStageHeight = Math.max(
      1,
      rect?.stageHeight ??
        window.innerHeight -
          LISTEN_LIVE_VIDEO_TOPBAR_HEIGHT -
          LISTEN_LIVE_VIDEO_FRAME_GAP * 2,
    );
    const currentStageRatio = currentStageWidth / currentStageHeight;
    const tooWide = currentStageRatio > LISTEN_LIVE_VIDEO_ASPECT_RATIO;
    const targetStageWidth = tooWide
      ? currentStageHeight * LISTEN_LIVE_VIDEO_ASPECT_RATIO
      : currentStageWidth;
    const targetStageHeight = tooWide
      ? currentStageHeight
      : currentStageWidth / LISTEN_LIVE_VIDEO_ASPECT_RATIO;
    const deltaWidth = targetStageWidth - currentStageWidth;
    const deltaHeight = targetStageHeight - currentStageHeight;
    if (Math.abs(deltaWidth) < 1 && Math.abs(deltaHeight) < 1) {
      return;
    }
    const clampDimension = (
      value: number,
      minimum: number,
      maximum: number,
    ) => Math.min(Math.max(minimum, Math.round(value)), maximum);
    void Promise.all([
      Window.IsFullscreen(),
      Window.IsMaximised(),
      Window.Size(),
      Window.GetScreen().catch(() => null),
    ])
      .then(([fullscreen, maximised, size, screen]) => {
        if (fullscreen || maximised) {
          return;
        }
        const widthInset = Math.max(0, size.width - currentStageWidth);
        const heightInset = Math.max(0, size.height - currentStageHeight);
        const workAreaWidth = Number(screen?.WorkArea?.Width ?? 0);
        const workAreaHeight = Number(screen?.WorkArea?.Height ?? 0);
        const maxWidth =
          workAreaWidth > 0
            ? Math.max(LISTEN_LIVE_VIDEO_MIN_WINDOW_WIDTH, workAreaWidth - 16)
            : Math.max(
                LISTEN_LIVE_VIDEO_MIN_WINDOW_WIDTH,
                window.screen.availWidth - 16,
              );
        const maxHeight =
          workAreaHeight > 0
            ? Math.max(LISTEN_LIVE_VIDEO_MIN_WINDOW_HEIGHT, workAreaHeight - 16)
            : Math.max(
                LISTEN_LIVE_VIDEO_MIN_WINDOW_HEIGHT,
                window.screen.availHeight - 16,
              );
        let nextWidth = clampDimension(
          size.width + deltaWidth,
          LISTEN_LIVE_VIDEO_MIN_WINDOW_WIDTH,
          maxWidth,
        );
        let nextHeight = clampDimension(
          size.height + deltaHeight,
          LISTEN_LIVE_VIDEO_MIN_WINDOW_HEIGHT,
          maxHeight,
        );
        if (tooWide) {
          const adjustedStageWidth = Math.max(1, nextWidth - widthInset);
          nextHeight = clampDimension(
            heightInset +
              adjustedStageWidth / LISTEN_LIVE_VIDEO_ASPECT_RATIO,
            LISTEN_LIVE_VIDEO_MIN_WINDOW_HEIGHT,
            maxHeight,
          );
        } else {
          const adjustedStageHeight = Math.max(1, nextHeight - heightInset);
          nextWidth = clampDimension(
            widthInset +
              adjustedStageHeight * LISTEN_LIVE_VIDEO_ASPECT_RATIO,
            LISTEN_LIVE_VIDEO_MIN_WINDOW_WIDTH,
            maxWidth,
          );
        }
        if (
          Math.abs(nextWidth - size.width) < 1 &&
          Math.abs(nextHeight - size.height) < 1
        ) {
          return;
        }
        return Window.SetSize(nextWidth, nextHeight);
      })
      .catch((error) => {
        console.warn("[Listen] fit live video window unavailable", error);
      });
  }, []);

  React.useEffect(() => {
    setLiveNativeVideoAvailable(true);
    setLiveNativeVideoShown(false);
    setLiveNativeVideoSuspended(false);
    liveNativeVideoRectRef.current = null;
  }, [trackVideoId]);

  React.useEffect(() => {
    if (liveFullscreenActive) {
      return;
    }
    liveNativeVideoRectRef.current = null;
    liveNativeVideoRequestRef.current += 1;
    setLiveNativeVideoShown(false);
  }, [liveFullscreenActive]);

  React.useEffect(() => {
    if (!liveVideoModeEligible) {
      setLiveNativeVideoSettled(false);
      return;
    }
    setLiveNativeVideoSettled(false);
    const timer = window.setTimeout(() => {
      setLiveNativeVideoSettled(true);
    }, LISTEN_LIVE_VIDEO_EMBED_SETTLE_MS);
    return () => window.clearTimeout(timer);
  }, [
    liveVideoModeEligible,
    props.listOpen,
    trackVideoId,
  ]);

  const hideLiveEmbeddedVideo = React.useCallback(() => {
    const requestId = liveNativeVideoRequestRef.current + 1;
    liveNativeVideoRequestRef.current = requestId;
    const sequence = createListenNativeVideoSequence(requestId);
    setLiveNativeVideoShown(false);
    void callNativePlayer("HideEmbeddedVideoForSequence", { sequence }).catch(
      () => {},
    );
  }, [callNativePlayer]);

  const showLiveEmbeddedVideo = React.useCallback(
    (rect: ListenNativeVideoRect) => {
      const requestId = liveNativeVideoRequestRef.current + 1;
      const nextRect = {
        ...rect,
        interactive: false,
        sequence: createListenNativeVideoSequence(requestId),
      };
      liveNativeVideoRectRef.current = nextRect;
      if (!liveVideoModeActive) {
        liveNativeVideoRequestRef.current += 1;
        setLiveNativeVideoShown(false);
        return Promise.resolve(false);
      }
      liveNativeVideoRequestRef.current = requestId;
      setLiveNativeVideoShown(false);
      return callNativePlayer("ShowEmbeddedVideo", nextRect)
        .then((shown) => {
          if (liveNativeVideoRequestRef.current !== requestId) {
            return false;
          }
          const visible = Boolean(shown);
          setLiveNativeVideoShown(visible);
          return visible;
        })
        .catch((error) => {
          if (liveNativeVideoRequestRef.current !== requestId) {
            return false;
          }
          setLiveNativeVideoShown(false);
          setLiveNativeVideoAvailable(false);
          console.warn("[Listen] embedded live video unavailable", error);
          return false;
        });
    },
    [callNativePlayer, liveVideoModeActive],
  );

  React.useEffect(() => {
    if (!isLive) {
      return;
    }
    if (!liveVideoModeActive) {
      liveNativeVideoRectRef.current = null;
      hideLiveEmbeddedVideo();
      return;
    }
    const rect = liveNativeVideoRectRef.current;
    if (rect) {
      showLiveEmbeddedVideo(rect);
    }
  }, [
    hideLiveEmbeddedVideo,
    isLive,
    liveVideoModeActive,
    showLiveEmbeddedVideo,
  ]);

  React.useEffect(() => {
    if (!isLive || !liveVideoModeActive || liveNativeVideoShown) {
      return;
    }
    const retry = () => {
      const rect = liveNativeVideoRectRef.current;
      if (rect) {
        showLiveEmbeddedVideo(rect);
      }
    };
    retry();
    const interval = window.setInterval(retry, 500);
    return () => window.clearInterval(interval);
  }, [
    isLive,
    liveNativeVideoShown,
    liveVideoModeActive,
    showLiveEmbeddedVideo,
  ]);

  React.useEffect(() => {
    if (inlineVideoHasSurface) {
      return;
    }
    inlineNativeVideoRectRef.current = null;
    inlineNativeVideoRequestRef.current += 1;
    setInlineNativeVideoShown(false);
  }, [inlineVideoHasSurface]);

  React.useEffect(() => {
    if (!inlineVideoModeEligible) {
      setInlineNativeVideoSettled(false);
      return;
    }
    setInlineNativeVideoSettled(true);
  }, [inlineVideoModeEligible]);

  const hideInlineEmbeddedVideo = React.useCallback(() => {
    const requestId = inlineNativeVideoRequestRef.current + 1;
    inlineNativeVideoRequestRef.current = requestId;
    const sequence = createListenNativeVideoSequence(requestId);
    inlineNativeVideoRectRef.current = null;
    setInlineNativeVideoShown(false);
    void callNativePlayer("HideEmbeddedVideoForSequence", { sequence }).catch(
      () => {},
    );
  }, [callNativePlayer]);

  React.useEffect(() => {
    if (isLive || props.listOpen) {
      fullscreenLyricsDefaultKeyRef.current = "";
      return;
    }
    if (!props.active || !trackVideoId) {
      return;
    }
    const defaultKey = `${props.track.id}:${props.track.videoId}`;
    if (mediaMode === "lyrics") {
      fullscreenLyricsDefaultKeyRef.current = defaultKey;
      return;
    }
    if (mediaMode === "video") {
      fullscreenLyricsDefaultKeyRef.current = defaultKey;
      return;
    }
    if (
      mediaMode !== "cover" ||
      fullscreenLyricsDefaultKeyRef.current === defaultKey
    ) {
      return;
    }
    fullscreenLyricsDefaultKeyRef.current = defaultKey;
    setMediaMode("lyrics");
  }, [
    isLive,
    mediaMode,
    props.active,
    props.listOpen,
    props.track.id,
    props.track.videoId,
    trackVideoId,
  ]);

  const showInlineEmbeddedVideo = React.useCallback(
    (rect: ListenNativeVideoRect) => {
      const requestId = inlineNativeVideoRequestRef.current + 1;
      const nextRect = {
        ...rect,
        interactive: false,
        sequence: createListenNativeVideoSequence(requestId),
      };
      inlineNativeVideoRectRef.current = nextRect;
      if (!inlineVideoRevealReady) {
        inlineNativeVideoRequestRef.current += 1;
        setInlineNativeVideoShown(false);
        return Promise.resolve(false);
      }
      inlineNativeVideoRequestRef.current = requestId;
      setInlineNativeVideoShown(false);
      return callNativePlayer("ShowEmbeddedVideo", nextRect)
        .then((shown) => {
          if (inlineNativeVideoRequestRef.current !== requestId) {
            return false;
          }
          const visible = Boolean(shown);
          setInlineNativeVideoShown(visible);
          return visible;
        })
        .catch((error) => {
          if (inlineNativeVideoRequestRef.current !== requestId) {
            return false;
          }
          setInlineNativeVideoShown(false);
          setInlineNativeVideoAvailable(false);
          console.warn("[Listen] embedded music video unavailable", error);
          return false;
        });
    },
    [callNativePlayer, inlineVideoRevealReady],
  );

  React.useEffect(() => {
    if (isLive) {
      return;
    }
    if (!inlineVideoModeActive) {
      hideInlineEmbeddedVideo();
    }
  }, [
    hideInlineEmbeddedVideo,
    inlineVideoModeActive,
    isLive,
  ]);

  React.useEffect(() => {
    if (isLive || !inlineVideoModeActive || inlineVideoRevealReady) {
      return;
    }
    hideInlineEmbeddedVideo();
  }, [
    hideInlineEmbeddedVideo,
    inlineVideoModeActive,
    inlineVideoRevealReady,
    isLive,
    props.track.videoId,
  ]);

  React.useEffect(() => {
    mediaModeRef.current = mediaMode;
  }, [mediaMode]);

  React.useEffect(() => {
    if (!props.enabled || !canCheckVideoAvailability || videoAvailability !== "checking") {
      return;
    }
    const controller = new AbortController();
    const videoId = trackVideoId;
    void fetchListenTrackInfo(props.httpBaseURL, videoId, controller.signal, props.text.locale)
      .then((item) => {
        if (!item || controller.signal.aborted || item.videoId.trim() !== videoId) {
          return;
        }
        const resolved = resolveListenTrackVideoAvailability(item, false);
        if (resolved !== "checking") {
          setVideoAvailability(resolved);
        }
      })
      .catch((error: unknown) => {
        if (controller.signal.aborted) {
          return;
        }
        console.warn("[Listen] video availability metadata unavailable", {
          videoId,
          error,
        });
      });
    return () => controller.abort();
  }, [
    canCheckVideoAvailability,
    props.enabled,
    props.httpBaseURL,
    props.text.locale,
    trackVideoId,
    videoAvailability,
  ]);

  React.useEffect(() => {
    return () => {
      const requestRef = isLive
        ? liveNativeVideoRequestRef
        : inlineNativeVideoRequestRef;
      const requestId = requestRef.current + 1;
      requestRef.current = requestId;
      const sequence = createListenNativeVideoSequence(requestId);
      void Call.ByName(`${playerService}.HideEmbeddedVideoForSequence`, {
        sequence,
      }).catch(
        () => {},
      );
      if (mediaModeRef.current === "video") {
        void Call.ByName(`${playerService}.HideVideoWindowForSequence`, {
          sequence,
        }).catch(
          () => {},
        );
      }
    };
  }, [isLive, playerService]);

  const markNativePlayerUnavailable = React.useCallback(
    (error: unknown) => {
      console.warn(
        isLive
          ? "[Listen] YouTube live native player unavailable"
          : "[Listen] YouTube Music native player unavailable",
        {
          videoId: props.track.videoId,
          title: props.track.title,
          error,
        },
      );
      props.onStateChange("error");
      props.onPlayingChange(false);
    },
    [
      isLive,
      props.onPlayingChange,
      props.onStateChange,
      props.track.title,
      props.track.videoId,
    ],
  );

  const handleAirPlay = React.useCallback((anchor: ListenAirPlayAnchor) => {
    if (!props.airPlaySupported) {
      return;
    }
    void callNativePlayer("ShowAirPlayPicker", anchor).catch((error) => {
      console.warn("[Listen] AirPlay picker unavailable", error);
    });
  }, [callNativePlayer, props.airPlaySupported]);

  const handleOnlineMediaModeChange = React.useCallback(
    (mode: ListenMediaMode) => {
      setMediaMode(mode);
      if (mode !== "video") {
        hideInlineEmbeddedVideo();
      }
    },
    [hideInlineEmbeddedVideo],
  );

  const handleOpenTrackPage = React.useCallback(() => {
    if (!trackPageURL) {
      return;
    }
    void openExternalURL(trackPageURL).catch((error) => {
      console.warn("[Listen] open track page unavailable", {
        url: trackPageURL,
        error,
      });
    });
  }, [trackPageURL]);

  const handleCopyTrackLink = React.useCallback(() => {
    if (!trackPageURL) {
      return;
    }
    void copyListenTextToClipboard(trackPageURL)
      .then(() => {
        messageBus.publishToast({
          id: "listen-text-link",
          intent: "success",
          title: props.text.listen.linkCopied,
          source: "listen",
          autoCloseMs: 2200,
        });
      })
      .catch((error) => {
        console.warn("[Listen] text track link unavailable", {
          url: trackPageURL,
          error,
        });
      });
  }, [props.text.listen.linkCopied, trackPageURL]);

  const playNativeTrack = React.useCallback(
    (
      commandId: number,
      startSeconds: number,
      options: {
        volume?: number;
        muted?: boolean;
        restartFromStart?: boolean;
        forceReload?: boolean;
      } = {},
    ) => {
      const requestKey = `${props.track.videoId}:${commandId}`;
      if (lastPlayRequestRef.current === requestKey) {
        return;
      }
      lastPlayRequestRef.current = requestKey;
      setPlaybackAdvertising(false);
      setPlaybackAdvertisingLabel("");
      setPlaybackAdvertisingProgress(null);
      setPlaybackErrorLabel("");
      setPlaybackErrorMessage("");
      props.onStateChange("loading");
      props.onPlayingChange(false);
      const inactiveRequestRef = isLive
        ? inlineNativeVideoRequestRef
        : liveNativeVideoRequestRef;
      const inactiveRequestId = inactiveRequestRef.current + 1;
      inactiveRequestRef.current = inactiveRequestId;
      const inactiveSequence =
        createListenNativeVideoSequence(inactiveRequestId);
      void Call.ByName(`${inactivePlayerService}.Pause`).catch(() => {});
      void Call.ByName(`${inactivePlayerService}.HideVideoWindowForSequence`, {
        sequence: inactiveSequence,
      }).catch(() => {});
      void callNativePlayer("Play", {
        videoId: props.track.videoId,
        title: props.track.title,
        artist: props.track.channel,
        language: props.text.locale,
        startSeconds: startSeconds > 1 ? startSeconds : 0,
        restartFromStart: options.restartFromStart === true,
        forceReload: options.forceReload === true,
        volume: clampVolume(options.volume ?? props.volume),
        muted: options.muted ?? props.muted,
      }).catch(markNativePlayerUnavailable);
    },
    [
      callNativePlayer,
      inactivePlayerService,
      isLive,
      markNativePlayerUnavailable,
      props.muted,
      props.onPlayingChange,
      props.onStateChange,
      props.text.locale,
      props.track.channel,
      props.track.title,
      props.track.videoId,
      props.volume,
    ],
  );

  React.useEffect(() => {
    if (!props.command) {
      return;
    }
    const commandKey = `${props.track.videoId}:${props.command.id}:${props.command.command}`;
    if (handledNativeCommandRef.current === commandKey) {
      return;
    }
    if (!props.enabled) {
      return;
    }
    handledNativeCommandRef.current = commandKey;
    if (props.command.command === "play") {
      const hasExplicitStart = typeof props.command.startSeconds === "number";
      const startSeconds = hasExplicitStart
        ? Math.max(0, props.command.startSeconds ?? 0)
        : resumeRef.current;
      const canResumeCurrentTrack =
        !hasExplicitStart &&
        props.state === "paused";
      if (canResumeCurrentTrack) {
        props.onStateChange("buffering");
        props.onPlayingChange(true);
        void callNativePlayer("Resume").catch(markNativePlayerUnavailable);
        return;
      }
      playNativeTrack(props.command.id, startSeconds, {
        restartFromStart: hasExplicitStart && startSeconds <= 0.5,
        forceReload: props.command.forceReload === true,
      });
      return;
    }
    if (props.command.command === "resume") {
      props.onStateChange("buffering");
      props.onPlayingChange(true);
      void callNativePlayer("Resume").catch(markNativePlayerUnavailable);
      return;
    }
    if (props.command.command === "pause") {
      props.onStateChange("paused");
      props.onPlayingChange(false);
      void callNativePlayer("Pause").catch(markNativePlayerUnavailable);
      return;
    }
    if (props.command.command === "seek") {
      const startSeconds = Math.max(0, props.command.startSeconds ?? 0);
      props.onProgressChange(
        props.track.videoId,
        startSeconds,
        Math.max(0, props.progress.duration),
        Math.max(0, props.progress.bufferedTime),
      );
      if (props.playing || props.state === "playing" || props.state === "buffering") {
        props.onStateChange("buffering");
      }
      void callNativePlayer("Seek", { seconds: startSeconds }).catch(
        markNativePlayerUnavailable,
      );
      return;
    }
    props.onProgressChange(
      props.track.videoId,
      0,
      Math.max(0, props.progress.duration),
      0,
    );
    props.onStateChange("buffering");
    props.onPlayingChange(true);
    void callNativePlayer("Replay").catch(markNativePlayerUnavailable);
  }, [
    callNativePlayer,
    markNativePlayerUnavailable,
    playNativeTrack,
    props.command,
    props.enabled,
    props.onPlayingChange,
    props.onProgressChange,
    props.onStateChange,
    props.playing,
    props.progress.bufferedTime,
    props.progress.duration,
    props.state,
    props.track.videoId,
  ]);

  React.useEffect(() => {
    if (!props.enabled || !isLive) {
      return;
    }
    void callNativePlayer("SetVolume", {
      volume: clampVolume(props.volume),
      muted: props.muted,
    }).catch(() => {});
  }, [callNativePlayer, isLive, props.enabled, props.muted, props.volume]);

  React.useEffect(
    () => {
      if (!props.enabled) {
        return;
      }
      return () => {
        void Call.ByName(`${playerService}.Pause`).catch(
          () => {},
        );
      };
    },
    [playerService, props.enabled],
  );

  React.useEffect(() => {
    if (!props.enabled) {
      return;
    }
    const offPlayerEvent = Events.On(playerEventName, (event) => {
      const data = ((event as { data?: unknown }).data ??
        event) as ListenNativePlayerEvent;
      if (!data || data.source !== playerEventSource) {
        return;
      }
      if (data.type === "debug") {
        return;
      }
      if (data.type === "video-closed") {
        setMediaMode((current) => (current === "video" ? "cover" : current));
        return;
      }
      if (data.type === "remote-next") {
        if (isLive) {
          props.onNext();
        } else {
          props.onNativeTrackChange(data);
        }
        return;
      }
      if (data.type === "remote-previous") {
        if (isLive) {
          props.onPrevious();
        } else {
          props.onNativeTrackChange(data);
        }
        return;
      }
      const eventVideoId = String(data.observedVideoId || data.videoId || "").trim();
      const requestedVideoId = String(data.requestedVideoId || "").trim();
      const eventURLVideoId = readListenNativeEventURLVideoId(
        String(data.url || ""),
      );
      const liveEventBelongsToTrack =
        isLive &&
        (requestedVideoId
          ? requestedVideoId === props.track.videoId
          : eventURLVideoId === props.track.videoId) &&
        (!eventURLVideoId || eventURLVideoId === props.track.videoId);
	      const eventBelongsToCurrentTrack = isLive
	        ? liveEventBelongsToTrack ||
	          !eventVideoId ||
	          eventVideoId === props.track.videoId
	        : !eventVideoId ||
	          eventVideoId === props.track.videoId ||
	          requestedVideoId === props.track.videoId ||
	          eventURLVideoId === props.track.videoId;
	      if (!isLive && data.type === "lyrics-time") {
	        if (
	          eventBelongsToCurrentTrack &&
	          typeof data.currentTime === "number" &&
	          Number.isFinite(data.currentTime)
	        ) {
	          setLyricsCurrentTimeMs(Math.max(0, data.currentTime * 1000));
	        }
	        return;
	      }
	      const nextState = isLive
        ? normalizeListenLiveNativeState(data.state || "idle", data)
        : data.state || "idle";
      if (
        !isLive &&
        eventBelongsToCurrentTrack &&
        !data.advertising &&
        !data.ad
      ) {
        const nextAspectRatio = resolveListenNativeEventVideoAspectRatio(data);
        if (nextAspectRatio) {
          setVideoAspectRatio(nextAspectRatio);
        }
      }

      const syncVideoAvailabilityState = () => {
        if (isLive || !eventBelongsToCurrentTrack) {
          return;
        }
        const resolved = resolveListenTrackVideoAvailability(
          {
            ...props.track,
            thumbnailUrl: data.thumbnailUrl || props.track.thumbnailUrl,
          },
          false,
        );
        if (resolved !== "checking") {
          setVideoAvailability(resolved);
          return;
        }
      };

      const syncPlaybackPresentationState = () => {
        syncVideoAvailabilityState();
        if (nextState === "error") {
          const errorCode = String(data.errorCode ?? data.code ?? "").trim();
          const errorMessage = String(data.errorMessage || data.message || "").trim();
          setPlaybackAdvertising(false);
          setPlaybackAdvertisingLabel("");
          setPlaybackAdvertisingProgress(null);
          setPlaybackErrorLabel(errorCode);
          setPlaybackErrorMessage(errorMessage);
        } else {
          const advertising = Boolean(data.advertising || data.ad);
          setPlaybackAdvertising(advertising);
          setPlaybackAdvertisingLabel(advertising ? String(data.adLabel || "").trim() : "");
          if (advertising) {
            const nextAdProgress = {
              currentTime: Math.max(0, Number(data.currentTime || 0)),
              duration: Math.max(0, Number(data.duration || 0)),
              bufferedTime: Math.max(0, Number(data.bufferedTime || 0)),
            };
            setPlaybackAdvertisingProgress((current) => {
              if (
                current &&
                Math.abs(current.currentTime - nextAdProgress.currentTime) < 0.15 &&
                Math.abs(current.duration - nextAdProgress.duration) < 0.15 &&
                Math.abs(current.bufferedTime - nextAdProgress.bufferedTime) < 0.35
              ) {
                return current;
              }
              return nextAdProgress;
            });
          } else {
            setPlaybackAdvertisingProgress(null);
          }
          setPlaybackErrorLabel("");
          setPlaybackErrorMessage("");
        }
      };

      if (isLive && eventVideoId && eventVideoId !== props.track.videoId) {
        const switchedRecently = Date.now() - intendedVideoSinceRef.current < 1800;
        if (liveEventBelongsToTrack) {
          syncPlaybackPresentationState();
          props.onProgressChange(
            props.track.videoId,
            Number(data.currentTime || 0),
            Number(data.duration || 0),
            Number(data.bufferedTime || 0),
          );
          if (data.type !== "ready") {
            props.onStateChange(nextState);
            props.onPlayingChange(
              nextState === "playing" || nextState === "buffering",
            );
          }
          return;
        }
        if (
          !switchedRecently &&
          (nextState === "playing" || nextState === "buffering")
        ) {
          const now = Date.now();
          if (now - liveMismatchReplayAtRef.current > 5000) {
            liveMismatchReplayAtRef.current = now;
            props.onProgressChange(props.track.videoId, 0, 0, 0, true);
            props.onStateChange("buffering");
            props.onPlayingChange(true);
            playNativeTrack(now, 0);
          }
        }
        return;
      }

      syncPlaybackPresentationState();

      if (!isLive) {
        props.onNativeTrackChange(data);
        return;
      }

      if (data.type === "track-ended") {
        if (nextState === "error") {
          props.onStateChange("error");
          props.onPlayingChange(false);
          return;
        }
        if (data.advertising || data.ad) {
          return;
        }
        if (eventVideoId && eventVideoId !== props.track.videoId) {
          return;
        }
        props.onProgressChange(
          eventVideoId || props.track.videoId,
          Number(data.currentTime || props.progress.duration || 0),
          Number(data.duration || props.progress.duration || 0),
          Number(data.bufferedTime || 0),
        );
        if (isLive) {
          props.onStateChange("ended");
          props.onPlayingChange(false);
        }
        props.onEnded();
        return;
      }
      if (eventVideoId && eventVideoId !== props.track.videoId) {
        const switchedRecently = Date.now() - intendedVideoSinceRef.current < 1800;
        if (data.advertising || data.ad) {
          return;
        }
        if (
          switchedRecently ||
          data.type === "ready" ||
          nextState === "paused" ||
          nextState === "ended" ||
          nextState === "idle"
        ) {
          return;
        }
        props.onNativeTrackChange(data);
        return;
      }
      if (typeof data.currentTime === "number" || typeof data.duration === "number") {
        props.onProgressChange(
          eventVideoId || props.track.videoId,
          Number(data.currentTime || 0),
          Number(data.duration || 0),
          Number(data.bufferedTime || 0),
        );
      }
      if (data.type === "ready") {
        return;
      }
      props.onStateChange(nextState);
      props.onPlayingChange(
        nextState === "playing" || nextState === "buffering",
      );
      if (nextState === "ended") {
        props.onEnded();
      } else if (nextState === "error") {
        console.warn(
          isLive
            ? "[Listen] YouTube live playback error"
            : "[Listen] YouTube Music playback error",
          {
            code: data.code,
            message: data.message,
            reason: data.reason,
            videoId: eventVideoId || props.track.videoId,
            title: props.track.title,
          },
        );
      }
    });
    return () => offPlayerEvent();
  }, [
    callNativePlayer,
    markNativePlayerUnavailable,
    props.enabled,
    props.onEnded,
    props.onNext,
    props.onNativeTrackChange,
    props.onPlayingChange,
    props.onPrevious,
    props.onProgressChange,
    props.onStateChange,
    props.text.listen.errorStatus,
    props.progress.duration,
    props.state,
    props.track.videoId,
    props.track.title,
    isLive,
    playerEventName,
    playerEventSource,
    playNativeTrack,
  ]);

  React.useEffect(() => {
    if (
      !props.enabled ||
      playbackAdvertising ||
      props.state !== "loading" ||
      props.playing ||
      props.progress.currentTime > 0.15 ||
      props.progress.bufferedTime > 0.15 ||
      props.progress.duration > 0
    ) {
      return;
    }
    const videoId = props.track.videoId;
    const title = props.track.title;
    const timer = window.setTimeout(() => {
      console.warn("[Listen] YouTube playback start timed out", {
        videoId,
        title,
      });
      props.onStateChange("error");
      props.onPlayingChange(false);
    }, 25000);
    return () => window.clearTimeout(timer);
  }, [
    props.enabled,
    props.onPlayingChange,
    props.onStateChange,
    playbackAdvertising,
    props.playing,
    props.progress.bufferedTime,
    props.progress.currentTime,
    props.progress.duration,
    props.state,
    props.track.title,
    props.track.videoId,
  ]);

  const handleOnlineSeek = React.useCallback(
    (seconds: number) => {
      const duration = Number.isFinite(props.progress.duration)
        ? Math.max(0, props.progress.duration)
        : 0;
      if (!props.enabled || duration <= 0) {
        return;
      }
      const nextTime = Math.max(0, Math.min(seconds, duration));
      props.onProgressChange(
        props.track.videoId,
        nextTime,
        duration,
        Math.max(0, props.progress.bufferedTime),
      );
      if (props.playing) {
        props.onStateChange("buffering");
      }
      void callNativePlayer("Seek", { seconds: nextTime }).catch(
        markNativePlayerUnavailable,
      );
    },
    [
      callNativePlayer,
      markNativePlayerUnavailable,
      props.enabled,
      props.onProgressChange,
      props.onStateChange,
      props.playing,
      props.progress.bufferedTime,
      props.progress.duration,
      props.track.videoId,
    ],
  );

  const handleStopPlayback = React.useCallback(() => {
    setMediaMode((current) => (current === "video" ? "cover" : current));
    setPlaybackAdvertising(false);
    setPlaybackAdvertisingLabel("");
    setPlaybackAdvertisingProgress(null);
    setPlaybackErrorLabel("");
    setPlaybackErrorMessage("");
    props.onProgressChange(
      props.track.videoId,
      0,
      Math.max(0, props.progress.duration),
      0,
    );
    props.onStateChange("idle");
    props.onPlayingChange(false);
    void callNativePlayer("Reset").catch((error) => {
      console.warn("[Listen] stop playback unavailable", error);
    });
  }, [
    callNativePlayer,
    props.onPlayingChange,
    props.onProgressChange,
    props.onStateChange,
    props.progress.duration,
    props.track.videoId,
  ]);

  const playbackTimelineProgress = playbackAdvertising
    ? playbackAdvertisingProgress ?? LISTEN_EMPTY_PROGRESS
    : props.progress;

  return (
    <div className={cn(
      "relative h-full min-h-0",
      artworkVisualizerVisible ? "overflow-visible" : "overflow-hidden",
    )}>
      <ListenPlayerChrome
        mediaMode={mediaMode}
        listOpen={props.listOpen}
        onToggleList={props.onToggleList}
        reserveWindowControls={props.reserveWindowControls}
        airPlaySupported={props.airPlaySupported}
        sourceBadge={sourceBadge}
        observedPlaybackAudioQuality={props.observedPlaybackAudioQuality}
        headerCover={
          <ListenCompactCoverSurface
            key={props.track.id}
            srcCandidates={buildListenPosterCandidates(
              props.httpBaseURL,
              props.track,
            )}
            title={props.track.title}
          />
        }
        cover={
          <ListenOnlineArtwork
            key={props.track.id}
            httpBaseURL={props.httpBaseURL}
            track={props.track}
            className="!w-full"
            visualizerVisible={artworkVisualizerVisible}
            visualizer={
              isEqualizerArtworkVisualizerMode(props.visualizerMode) && props.visualizerEnabled ? (
                <ListenArtworkVisualizerBridge
                  mode={props.visualizerMode}
                  enabled={props.visualizerEnabled}
                  active={props.playing}
                  onVisibleChange={handleArtworkVisualizerVisibleChange}
                />
              ) : null
            }
          />
        }
        lyrics={
          <ListenLyricsSurface
            text={props.text}
            lyrics={lyricsState.data}
            loading={lyricsState.loading}
            error={lyricsState.error}
            onRetry={lyricsState.error ? retryLyrics : undefined}
            currentTimeMs={
              lyricsState.data?.kind === "synced"
                ? lyricsCurrentTimeMs
                : Math.max(0, props.progress.currentTime * 1000)
            }
            timelineRunning={
              props.playing &&
              props.state === "playing" &&
              !playbackAdvertising
            }
            romanized={props.romanizedLyrics}
            pinyin={props.pinyinLyrics}
            onSeek={isLive ? undefined : handleOnlineSeek}
          />
        }
        hasVideo={hasVideo}
        videoHidden={isLive}
        videoLoading={videoLoading}
        live={isLive}
        fullscreenLive={liveFullscreenActive}
        videoId={props.track.videoId}
        liveVideoModeActive={liveVideoModeActive}
        liveVideoVisible={liveVideoVisible}
        inlineVideoRevealReady={inlineVideoRevealReady}
        inlineVideoVisible={inlineVideoVisible}
        inlineVideoAspectRatio={videoAspectRatio}
        track={props.track}
        httpBaseURL={props.httpBaseURL}
        pet={props.pet}
        petImageURL={props.petImageURL}
        lyricsAvailable={!isLive}
        lyricsKind={lyricsState.data?.kind}
        lyricsLoading={lyricsState.loading}
        title={props.track.title}
        subtitle={artistName}
        subtitleArtistParts={artistName && !isLive ? artistLabelParts : undefined}
        onSubtitleClick={
          artistName && !isLive
            ? () => props.onOpenArtist(props.track)
            : undefined
        }
        onSubtitleArtistClick={
          artistName && !isLive ? handleSubtitleArtistClick : undefined
        }
        infoActions={
          <>
            {showFavoriteAction ? (
              <ListenPlayerIconButton
                label={props.text.listen.favorite}
                active={props.favoriteActive}
                className={cn(
                  props.favoriteActive && "text-sidebar-primary",
                )}
                disabled={props.favoriteBusy}
                onClick={props.onToggleFavorite}
              >
                <Heart
                  className={cn(
                    "h-4 w-4",
                    props.favoriteActive && "fill-current",
                  )}
                />
              </ListenPlayerIconButton>
            ) : null}
            {showDownloadAction ? (
              <ListenPlayerIconButton
                label={props.text.actions.download}
                onClick={() =>
                  props.onDownloadTrack(buildYouTubeWatchURL(props.track.videoId))
                }
              >
                <Download className="h-4 w-4" />
              </ListenPlayerIconButton>
            ) : null}
            <ListenPlayerMoreMenu
              text={props.text}
              disabled={!trackPageURL}
              onOpenPage={handleOpenTrackPage}
              onCopyLink={handleCopyTrackLink}
            />
          </>
        }
        progress={playbackTimelineProgress}
        advertising={playbackAdvertising}
        advertisingLabel={playbackAdvertisingLabel}
        progressLoading={progressLoading}
        errorActive={props.state === "error"}
        errorLabel={
          props.state === "error"
            ? playbackErrorLabel
            : ""
        }
        errorTitle={props.state === "error" ? playbackErrorMessage : ""}
        onSeek={isLive ? undefined : handleOnlineSeek}
        onStopPlayback={handleStopPlayback}
        onFitLiveVideoWindow={handleFitLiveVideoWindow}
        onLiveVideoRectChange={showLiveEmbeddedVideo}
        onInlineVideoRectChange={showInlineEmbeddedVideo}
        playing={props.playing}
        loading={transportLoading}
        playbackState={props.state}
        muted={props.muted}
        volume={props.volume}
        playMode={props.playMode}
        text={props.text}
        onAirPlay={props.airPlaySupported ? handleAirPlay : undefined}
        onMediaModeChange={handleOnlineMediaModeChange}
        onPrevious={props.onPrevious}
        onNext={props.onNext}
        onPlayModeChange={props.onPlayModeChange}
        onTogglePlayback={props.onTogglePlayback}
        onToggleMute={props.onToggleMute}
        onVolumeChange={props.onVolumeChange}
        onToggleQueue={(anchor) => {
          setQueueAnchor(anchor);
          setQueueOpen((current) => !current);
        }}
        visualizerMode={props.visualizerMode}
        visualizerEnabled={props.visualizerEnabled}
        visualizerActive={props.playing}
        queueOpen={queueOpen}
        queueOverlay={
          queueOpen ? (
            <ListenPlaybackQueuePopup
              anchor={queueAnchor}
              queueTitle={props.queueTitle}
              queueItems={props.queueItems}
              selectedQueueId={props.selectedQueueId}
              httpBaseURL={props.httpBaseURL}
              text={props.text}
              onClearQueue={isLive ? undefined : props.onClearQueue}
              onRemoveQueueItem={isLive ? undefined : props.onRemoveQueueItem}
              onMoveQueueItem={isLive ? undefined : props.onMoveQueueItem}
              onUndoQueueEdit={isLive ? undefined : props.onUndoQueueEdit}
              onRedoQueueEdit={isLive ? undefined : props.onRedoQueueEdit}
              queueCanUndo={!isLive && props.queueCanUndo}
              queueCanRedo={!isLive && props.queueCanRedo}
              onSelectQueueTrack={props.onSelectQueueTrack}
              onClose={() => setQueueOpen(false)}
            />
          ) : null
        }
      />
    </div>
  );
}

function ListenPlayerChrome(props: {
  mediaMode: ListenMediaMode;
  listOpen?: boolean;
  onToggleList?: () => void;
  reserveWindowControls: boolean;
  airPlaySupported: boolean;
  sourceBadge?: React.ReactNode;
  headerCover?: React.ReactNode;
  cover: React.ReactNode;
  lyrics: React.ReactNode;
  hasVideo: boolean;
  videoHidden?: boolean;
  videoLoading?: boolean;
  live?: boolean;
  fullscreenLive?: boolean;
  videoId?: string;
  liveVideoModeActive?: boolean;
  liveVideoVisible?: boolean;
  inlineVideoRevealReady?: boolean;
  inlineVideoVisible?: boolean;
  inlineVideoAspectRatio?: number;
  track?: ListenOnlineItem;
  httpBaseURL?: string;
  pet?: Pet | null;
  petImageURL?: string;
  lyricsAvailable?: boolean;
  lyricsKind?: ListenLyricsKind;
  lyricsLoading?: boolean;
  disabled?: boolean;
  title: string;
  subtitle: string;
  subtitleArtistParts?: ListenArtistLabelPart[];
  onSubtitleClick?: () => void;
  onSubtitleArtistClick?: (artist: string) => void;
  infoActions?: React.ReactNode;
  progress: {
    currentTime: number;
    duration: number;
    bufferedTime: number;
  };
  advertising?: boolean;
  advertisingLabel?: string;
  progressLoading?: boolean;
  errorActive?: boolean;
  errorLabel?: string;
  errorTitle?: string;
  onSeek?: (seconds: number) => void;
  onStopPlayback?: () => void;
  onFitLiveVideoWindow?: () => void;
  onLiveVideoRectChange?: (
    rect: ListenNativeVideoRect,
  ) => boolean | void | Promise<boolean | void>;
  onInlineVideoRectChange?: (
    rect: ListenNativeVideoRect,
  ) => boolean | void | Promise<boolean | void>;
  playing: boolean;
  loading: boolean;
  playbackState?: ListenRemotePlaybackState;
  muted: boolean;
  volume: number;
  playMode: ListenPlayMode;
  observedPlaybackAudioQuality?: ListenObservedPlaybackAudioQuality | "";
  text: ReturnType<typeof getXiaText>;
  onMediaModeChange: (mode: ListenMediaMode) => void;
  onAirPlay?: (anchor: ListenAirPlayAnchor) => void;
  onPrevious: () => void;
  onNext: () => void;
  onPlayModeChange: (mode: ListenPlayMode) => void;
  onTogglePlayback: React.MouseEventHandler<HTMLButtonElement>;
  onToggleMute: () => void;
  onVolumeChange: (value: number) => void;
  onToggleQueue?: (anchor: ListenQueuePopupAnchor) => void;
  visualizerMode?: EqualizerVisualizerMode;
  visualizerEnabled?: boolean;
  visualizerActive?: boolean;
  queueOpen?: boolean;
  queueOverlay?: React.ReactNode;
}) {
  const inlineVideoActive =
    !props.live &&
    props.hasVideo &&
    props.mediaMode === "video" &&
    Boolean(props.onInlineVideoRectChange);
  const splitMode = props.mediaMode === "lyrics" || inlineVideoActive;
  const rootRef = React.useRef<HTMLDivElement | null>(null);
  const playerStackRef = React.useRef<HTMLDivElement | null>(null);
  const layoutKey = props.listOpen === true ? "list" : "fullscreen";
  const [layoutSnapshot, setLayoutSnapshot] = React.useState({
    key: "",
    width: 0,
    playerStackHeight: 0,
  });
  const layoutMeasured =
    layoutSnapshot.key === layoutKey && layoutSnapshot.width > 0;
  const layoutWidth = layoutMeasured ? layoutSnapshot.width : 0;
  const playerStackHeight = layoutMeasured
    ? layoutSnapshot.playerStackHeight
    : 0;
  const splitEnabled =
    splitMode && layoutMeasured && layoutWidth >= LISTEN_MEDIA_SPLIT_MIN_WIDTH;
  const singleColumnLyrics =
    props.mediaMode === "lyrics" && !inlineVideoActive && !splitEnabled;
  const playLabel = props.playing ? props.text.listen.pause : props.text.listen.play;
  const visualizerMode = props.visualizerMode ?? "off";
  const visualizerEnabled = props.visualizerEnabled === true && visualizerMode !== "off";
  const visualizerActive = props.visualizerActive === true && props.playing && !props.loading;
  const inlineVideoSurface = inlineVideoActive ? (
    <ListenInlineVideoSurface
      variant={splitEnabled ? "wide" : "compact"}
      active={
        inlineVideoActive &&
        layoutMeasured &&
        props.inlineVideoRevealReady === true
      }
      visible={layoutMeasured && props.inlineVideoVisible === true}
      aspectRatio={props.inlineVideoAspectRatio}
      pet={props.pet ?? null}
      petImageURL={props.petImageURL ?? ""}
      title={props.title}
      text={props.text}
      onRectChange={props.onInlineVideoRectChange}
    />
  ) : null;
  const activeMedia =
    inlineVideoSurface ??
    (props.mediaMode === "lyrics"
      ? props.lyrics
      : props.cover);
  const mediaStage =
    inlineVideoSurface ? (
      inlineVideoSurface
    ) : props.mediaMode !== "lyrics" ? (
      props.cover
    ) : (
      <div
        className={cn(
          "w-full transition-[opacity,transform] duration-300 ease-out",
          splitEnabled ? "h-full" : "aspect-square",
        )}
      >
        {activeMedia}
      </div>
    );

  React.useLayoutEffect(() => {
    const root = rootRef.current;
    const stack = playerStackRef.current;
    if (!root || !stack || typeof ResizeObserver === "undefined") {
      return;
    }
    const syncLayout = () => {
      const rootRect = root.getBoundingClientRect();
      const stackRect = stack.getBoundingClientRect();
      const width = rootRect.width;
      const stackHeight = stackRect.height;
      setLayoutSnapshot((current) => {
        if (
          current.key === layoutKey &&
          Math.abs(current.width - width) < 0.5 &&
          Math.abs(current.playerStackHeight - stackHeight) < 0.5
        ) {
          return current;
        }
        return {
          key: layoutKey,
          width,
          playerStackHeight: stackHeight,
        };
      });
    };
    syncLayout();
    const observer = new ResizeObserver(syncLayout);
    observer.observe(root);
    observer.observe(stack);
    return () => observer.disconnect();
  }, [layoutKey, props.mediaMode]);

  return (
    <TooltipProvider delayDuration={0}>
      <div
        ref={rootRef}
        data-listen-player-root="true"
        data-artwork-visualizer-visible={visualizerEnabled ? "true" : "false"}
        className={cn(
          "relative flex h-full min-h-0 flex-col",
          visualizerEnabled ? "overflow-visible" : "overflow-hidden",
        )}
      >
        {props.fullscreenLive ? (
          <ListenLiveVideoShell
            videoId={props.videoId ?? ""}
            liveVideoModeActive={props.liveVideoModeActive === true}
            liveVideoVisible={props.liveVideoVisible === true}
            track={props.track}
            httpBaseURL={props.httpBaseURL ?? ""}
            pet={props.pet ?? null}
            petImageURL={props.petImageURL ?? ""}
            title={props.title}
            subtitle={props.subtitle}
            listOpen={props.listOpen === true}
            reserveWindowControls={props.reserveWindowControls}
            playing={props.playing}
            loading={props.loading}
            playbackState={props.playbackState}
            disabled={props.disabled === true}
            muted={props.muted}
            volume={props.volume}
            text={props.text}
            onToggleList={props.onToggleList}
            onTogglePlayback={props.onTogglePlayback}
            onStopPlayback={props.onStopPlayback}
            onFitLiveVideoWindow={props.onFitLiveVideoWindow}
            onLiveVideoRectChange={props.onLiveVideoRectChange}
            onToggleMute={props.onToggleMute}
            onVolumeChange={props.onVolumeChange}
          />
        ) : (
        <div
          className={cn(
            "min-h-0 flex-1 px-3 pb-2 pt-1 sm:px-5 sm:pb-4",
            visualizerEnabled ? "overflow-visible" : "overflow-hidden",
          )}
        >
          <div
            className={cn(
              "mx-auto grid h-full min-h-0 w-full items-center gap-6",
              inlineVideoActive
                ? "transition-none"
                : "transition-[max-width,gap] duration-300 ease-out",
              splitMode
                ? splitEnabled
                  ? "max-w-7xl grid-cols-[18rem_minmax(0,1fr)] gap-6 lg:gap-8"
                  : "max-w-[18rem] justify-center"
                : "max-w-[18rem] justify-center",
            )}
          >
            <div
              className={cn(
                "min-w-0",
                inlineVideoActive
                  ? "transition-none"
                  : "transition-transform duration-300 ease-out",
                splitEnabled ? "justify-self-start" : "justify-self-center",
              )}
            >
              <div
                ref={playerStackRef}
                className={cn("mx-auto", LISTEN_PLAYER_SURFACE_WIDTH_CLASS)}
              >
                {singleColumnLyrics ? (
                  <div className="listen-single-lyrics-panel flex h-[min(42rem,calc(100vh-8.5rem))] max-h-full min-h-0 flex-col overflow-hidden animate-in fade-in-0 slide-in-from-bottom-2 duration-300">
                    <div className="mb-4 flex shrink-0 items-center gap-3 text-left">
                      {props.headerCover}
                      <div className="min-w-0 flex-1">
                        <ListenScrollingText
                          text={props.title}
                          className="text-[15px] font-semibold leading-6 text-sidebar-foreground"
                        />
                        <ListenSubtitleText
                          text={props.subtitle || props.text.listen.nowPlaying}
                          artistParts={props.subtitleArtistParts}
                          className="mt-0.5 text-[12px] font-medium leading-5 text-sidebar-foreground/55"
                          onClick={props.subtitle ? props.onSubtitleClick : undefined}
                          onArtistClick={props.onSubtitleArtistClick}
                        />
                      </div>
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <button
                            type="button"
                            className={cn(
                              LISTEN_PRIMARY_PLAY_BUTTON_CLASS,
                              LISTEN_PRIMARY_PLAY_BUTTON_SIZE_CLASS.small,
                              props.disabled
                                ? "cursor-not-allowed opacity-35"
                                : LISTEN_PRIMARY_PLAY_BUTTON_HOVER_CLASS,
                            )}
                            disabled={props.disabled}
                            aria-label={playLabel}
                            title={playLabel}
                            onClick={props.onTogglePlayback}
                          >
                            {props.loading ? (
                              <Loader2 className={cn(LISTEN_PRIMARY_PLAY_ICON_SIZE_CLASS.small, "animate-spin")} />
                            ) : props.playing ? (
                              <Pause className={cn(LISTEN_PRIMARY_PLAY_ICON_SIZE_CLASS.small, "fill-current")} />
                            ) : (
                              <Play className={cn("ml-0.5 fill-current", LISTEN_PRIMARY_PLAY_ICON_SIZE_CLASS.small)} />
                            )}
                          </button>
                        </TooltipTrigger>
                        <TooltipContent side="top">{playLabel}</TooltipContent>
                      </Tooltip>
                    </div>
                    <div className="min-h-0 flex-1 overflow-hidden">
                      {props.lyrics}
                    </div>
                  </div>
                ) : (
                  <>
                    <div className={cn(splitMode && splitEnabled && "hidden")}>
                      {splitMode && splitEnabled ? null : mediaStage}
                    </div>
                    {splitMode && splitEnabled ? (
                      <div className={cn(splitEnabled ? "block animate-in fade-in-0 zoom-in-95 duration-300" : "hidden")}>
                        {props.cover}
                      </div>
                    ) : null}
                    <ListenTrackInfoRow
                      title={props.title}
                      subtitle={props.subtitle}
                      subtitleArtistParts={props.subtitleArtistParts}
                      onSubtitleClick={props.onSubtitleClick}
                      onSubtitleArtistClick={props.onSubtitleArtistClick}
                      actions={props.infoActions}
                    />
                    <ListenPlayerInlineVisualizer
                      mode={visualizerMode}
                      enabled={visualizerEnabled}
                      active={visualizerActive}
                    />
                    <ListenPlayerProgress
                      progress={props.progress}
                      text={props.text}
                      live={props.live}
                      playing={props.playing}
                      advertising={props.advertising}
                      advertisingLabel={props.advertisingLabel}
                      loading={props.progressLoading}
                      errorActive={props.errorActive}
                      errorLabel={props.errorLabel}
                      errorTitle={props.errorTitle}
                      onSeek={props.onSeek}
                    />
                    <ListenPlayerTransport
                      playing={props.playing}
                      loading={props.loading}
                      playMode={props.playMode}
                      live={props.live}
                      text={props.text}
                      onPrevious={props.onPrevious}
                      onNext={props.onNext}
                      onPlayModeChange={props.onPlayModeChange}
                      onTogglePlayback={props.onTogglePlayback}
                      disabled={props.disabled}
                    />
                    <ListenPlayerVolume
                      muted={props.muted}
                      volume={props.volume}
                      text={props.text}
                      onToggleMute={props.onToggleMute}
                      onVolumeChange={props.onVolumeChange}
                    />
                  </>
                )}
              </div>
            </div>
            {splitMode ? (
              <div
                className={cn(
                  "min-h-0",
                  inlineVideoActive
                    ? "transition-none"
                    : "transition-[opacity,transform] duration-300 ease-out",
                  splitEnabled
                    ? "flex h-full translate-x-0 items-center justify-center opacity-100"
                    : "pointer-events-none hidden translate-x-4 opacity-0",
                )}
                style={
                  splitEnabled && playerStackHeight > 0
                    ? { height: playerStackHeight }
                    : undefined
                }
              >
                <div
                  className={cn(
                    "h-full w-full max-w-[46rem]",
                    inlineVideoActive
                      ? "animate-none"
                      : "animate-in fade-in-0 slide-in-from-right-3 duration-300",
                  )}
                >
                  {splitEnabled ? mediaStage : null}
                </div>
              </div>
            ) : null}
          </div>
        </div>
        )}
        {props.fullscreenLive ? null : props.queueOverlay}
        {props.fullscreenLive ? null : (
        <ListenPlayerFooter
          mediaMode={props.mediaMode}
          reserveWindowControls={props.reserveWindowControls}
          airPlaySupported={props.airPlaySupported}
          sourceBadge={props.sourceBadge}
          hasVideo={props.hasVideo}
          videoHidden={props.videoHidden || props.live}
          videoLoading={props.videoLoading}
          live={props.live}
          lyricsAvailable={props.lyricsAvailable !== false}
          lyricsKind={props.lyricsKind}
          lyricsLoading={props.lyricsLoading}
          queueOpen={props.queueOpen}
          sourceBadgeQuality={props.observedPlaybackAudioQuality}
          text={props.text}
          onAirPlay={props.onAirPlay}
          onMediaModeChange={props.onMediaModeChange}
          onToggleQueue={props.onToggleQueue}
        />
        )}
      </div>
    </TooltipProvider>
  );
}

function ListenInlineVideoSurface(props: {
  variant: "compact" | "wide";
  active: boolean;
  visible: boolean;
  aspectRatio?: number;
  pet: Pet | null;
  petImageURL: string;
  title: string;
  text: ReturnType<typeof getXiaText>;
  onRectChange?: (
    rect: ListenNativeVideoRect,
  ) => boolean | void | Promise<boolean | void>;
}) {
  const frameRef = React.useRef<HTMLDivElement | null>(null);
  const stageRef = React.useRef<HTMLDivElement | null>(null);
  const aspectRatio = normalizeListenInlineVideoAspectRatio(
    props.aspectRatio ?? LISTEN_INLINE_VIDEO_FALLBACK_ASPECT_RATIO,
  );
  const [frameSize, setFrameSize] = React.useState({ width: 0, height: 0 });
  const [visualVisible, setVisualVisible] = React.useState(false);
  const rectRevealRequestRef = React.useRef(0);
  const {
    resetHole: resetNativeVideoHole,
    setHole: setNativeVideoHole,
  } = useListenNativeVideoUnderlay(props.active);
  const frameReady = frameSize.width > 1 && frameSize.height > 1;
  const geometrySignature = [
    props.variant,
    Math.round(aspectRatio * 1000) / 1000,
    Math.round(frameSize.width * 2) / 2,
    Math.round(frameSize.height * 2) / 2,
  ].join(":");

  React.useEffect(() => {
    if (!props.visible) {
      setVisualVisible(false);
    }
  }, [props.visible]);

  React.useLayoutEffect(() => {
    rectRevealRequestRef.current += 1;
    setVisualVisible(false);
  }, [geometrySignature]);

  React.useLayoutEffect(() => {
    const frame = frameRef.current;
    if (!frame || typeof ResizeObserver === "undefined") {
      return;
    }
    const sync = () => {
      const rect = frame.getBoundingClientRect();
      const width = Math.max(0, rect.width);
      const height = Math.max(0, rect.height);
      setFrameSize((current) => {
        if (
          Math.abs(current.width - width) < 0.5 &&
          Math.abs(current.height - height) < 0.5
        ) {
          return current;
        }
        return { width, height };
      });
    };
    sync();
    const observer = new ResizeObserver(sync);
    observer.observe(frame);
    return () => observer.disconnect();
  }, []);

  React.useLayoutEffect(() => {
    const onRectChange = props.onRectChange;
    if (!props.active || !onRectChange || !frameReady) {
      return;
    }
    const element = stageRef.current;
    if (!element) {
      return;
    }
    let readFrame = 0;
    let commitFrame = 0;
    let lastRectSignature = "";
    let revealRetryCount = 0;
    const timers: number[] = [];
    const readRadius = () => readListenNativeVideoRadius(element);
    const pushRect = (force = false) => {
      const rect = element.getBoundingClientRect();
      if (rect.width < 1 || rect.height < 1) {
        return;
      }
      const radius = readRadius();
      const frameRect = frameRef.current?.getBoundingClientRect() ?? rect;
      const viewportWidth = Math.max(1, window.innerWidth);
      const viewportHeight = Math.max(1, window.innerHeight);
      const centerX = rect.left + rect.width / 2;
      const centerY = rect.top + rect.height / 2;
      const signature = [
        Math.round(rect.left * 2) / 2,
        Math.round(rect.top * 2) / 2,
        Math.round(rect.width * 2) / 2,
        Math.round(rect.height * 2) / 2,
        Math.round(frameRect.width * 2) / 2,
        Math.round(frameRect.height * 2) / 2,
        Math.round(viewportWidth * 2) / 2,
        Math.round(viewportHeight * 2) / 2,
        Math.round(radius * 2) / 2,
      ].join(":");
      const geometryChanged = signature !== lastRectSignature;
      if (!force && !geometryChanged) {
        return;
      }
      lastRectSignature = signature;
      if (geometryChanged) {
        revealRetryCount = 0;
      }
      rectRevealRequestRef.current += 1;
      const requestToken = rectRevealRequestRef.current;
      setVisualVisible(false);
      resetNativeVideoHole();
      const scheduleRevealRetry = () => {
        if (rectRevealRequestRef.current !== requestToken) {
          return;
        }
        if (revealRetryCount >= 18) {
          return;
        }
        revealRetryCount += 1;
        timers.push(window.setTimeout(() => syncRect(true), 220));
      };
      const nativeRect = {
        x: rect.left,
        y: rect.top,
        width: rect.width,
        height: rect.height,
        centerX,
        centerY,
        stageWidth: Math.max(1, frameRect.width),
        stageHeight: Math.max(1, frameRect.height),
        viewportWidth,
        viewportHeight,
        radius,
      };
      let applyResult: boolean | void | Promise<boolean | void>;
      try {
        applyResult = onRectChange(nativeRect);
      } catch {
        return;
      }
      void Promise.resolve(applyResult).then((shown) => {
        if (rectRevealRequestRef.current !== requestToken || shown === false) {
          if (shown === false) {
            scheduleRevealRetry();
          }
          return;
        }
        revealRetryCount = 0;
        const nextRect = element.getBoundingClientRect();
        if (nextRect.width < 1 || nextRect.height < 1) {
          return;
        }
        setNativeVideoHole(nextRect, readRadius());
        setVisualVisible(true);
      }).catch(() => {
        scheduleRevealRetry();
      });
    };
    const cancelScheduledRect = () => {
      window.cancelAnimationFrame(readFrame);
      window.cancelAnimationFrame(commitFrame);
      readFrame = 0;
      commitFrame = 0;
    };
    const syncRect = (force = false) => {
      cancelScheduledRect();
      readFrame = window.requestAnimationFrame(() => {
        commitFrame = window.requestAnimationFrame(() => pushRect(force));
      });
    };
    const scheduleRect = () => syncRect();
    syncRect(true);
    LISTEN_LIVE_VIDEO_GEOMETRY_SETTLE_DELAYS_MS.forEach((delay) => {
      timers.push(window.setTimeout(scheduleRect, delay));
    });
    const observer =
      typeof ResizeObserver === "undefined"
        ? null
        : new ResizeObserver(scheduleRect);
    observer?.observe(element);
    if (frameRef.current) {
      observer?.observe(frameRef.current);
    }
    window.addEventListener("resize", scheduleRect);
    window.addEventListener("scroll", scheduleRect, true);
    window.visualViewport?.addEventListener("resize", scheduleRect);
    window.visualViewport?.addEventListener("scroll", scheduleRect);
    return () => {
      rectRevealRequestRef.current += 1;
      cancelScheduledRect();
      timers.forEach((timer) => window.clearTimeout(timer));
      observer?.disconnect();
      window.removeEventListener("resize", scheduleRect);
      window.removeEventListener("scroll", scheduleRect, true);
      window.visualViewport?.removeEventListener("resize", scheduleRect);
      window.visualViewport?.removeEventListener("scroll", scheduleRect);
      resetNativeVideoHole();
    };
  }, [
    aspectRatio,
    frameReady,
    frameSize.height,
    frameSize.width,
    props.active,
    props.onRectChange,
    props.variant,
    resetNativeVideoHole,
    setNativeVideoHole,
  ]);

  const stageStyle = React.useMemo<React.CSSProperties | undefined>(() => {
    if (frameSize.width <= 1 || frameSize.height <= 1) {
      return {
        aspectRatio,
      };
    }
    let width = frameSize.width;
    let height = width / aspectRatio;
    if (height > frameSize.height) {
      height = frameSize.height;
      width = height * aspectRatio;
    }
    return {
      width: `${Math.max(1, width)}px`,
      height: `${Math.max(1, height)}px`,
      aspectRatio,
    };
  }, [aspectRatio, frameSize.height, frameSize.width]);

  return (
    <div
      ref={frameRef}
      className={cn(
        "listen-inline-video-frame",
        props.variant === "wide"
          ? "listen-inline-video-frame-wide"
          : "listen-inline-video-frame-compact",
      )}
      data-native-video={visualVisible ? "underlay" : "pending"}
    >
      <div
        ref={stageRef}
        className="listen-inline-video-stage"
        style={stageStyle}
        data-native-video={visualVisible ? "underlay" : "pending"}
      >
        {!visualVisible ? (
          <div className="listen-inline-video-pending-layer">
            <PetDisplay
              pet={props.pet}
              imageUrl={props.petImageURL}
              animation="review"
              alt={props.title || props.text.listen.video}
              size={88}
              className="h-24 w-24"
            />
          </div>
        ) : null}
      </div>
    </div>
  );
}

function ListenLiveVideoShell(props: {
  videoId: string;
  liveVideoModeActive: boolean;
  liveVideoVisible: boolean;
  track?: ListenOnlineItem;
  httpBaseURL: string;
  pet: Pet | null;
  petImageURL: string;
  title: string;
  subtitle: string;
  listOpen: boolean;
  reserveWindowControls: boolean;
  playing: boolean;
  loading: boolean;
  playbackState?: ListenRemotePlaybackState;
  disabled?: boolean;
  muted: boolean;
  volume: number;
  text: ReturnType<typeof getXiaText>;
  onToggleList?: () => void;
  onTogglePlayback: React.MouseEventHandler<HTMLButtonElement>;
  onStopPlayback?: () => void;
  onFitLiveVideoWindow?: () => void;
  onLiveVideoRectChange?: (
    rect: ListenNativeVideoRect,
  ) => boolean | void | Promise<boolean | void>;
  onToggleMute: () => void;
  onVolumeChange: (value: number) => void;
}) {
  const coverAreaRef = React.useRef<HTMLDivElement | null>(null);
  const previousLiveVideoVisibleRef = React.useRef(props.liveVideoVisible);
  const rectRevealRequestRef = React.useRef(0);
  const [visualLiveVideoVisible, setVisualLiveVideoVisible] =
    React.useState(props.liveVideoVisible);
  const visualLiveVideoVisibleRef = React.useRef(visualLiveVideoVisible);
  const [liveVideoRevealActive, setLiveVideoRevealActive] =
    React.useState(false);
  const {
    resetHole: resetNativeVideoHole,
    setHole: setNativeVideoHole,
  } = useListenNativeVideoUnderlay(props.liveVideoModeActive);
  const playbackState =
    props.playbackState ??
    (props.loading ? "loading" : props.playing ? "playing" : "idle");
  const playLabel = props.playing ? props.text.listen.pause : props.text.listen.play;
  const playbackDisabled = props.disabled === true;
  const stopDisabled =
    playbackDisabled ||
    !props.onStopPlayback ||
    (!props.playing && !props.loading && playbackState === "idle");
  const statusLabel = resolveListenPlaybackStatusLabel(playbackState, props.text);
  const statusClass = resolveListenLivePlaybackStatusClass(playbackState);
  const listLabel = props.listOpen
    ? props.text.listen.collapseList
    : props.text.listen.openList;
  const titleLabel = props.title || props.text.listen.selectStation;
  const authorLabel = props.subtitle.trim();
  React.useEffect(() => {
    visualLiveVideoVisibleRef.current = visualLiveVideoVisible;
  }, [visualLiveVideoVisible]);

  React.useEffect(() => {
    const wasVisible = previousLiveVideoVisibleRef.current;
    previousLiveVideoVisibleRef.current = props.liveVideoVisible;
    if (!props.liveVideoVisible) {
      rectRevealRequestRef.current += 1;
      visualLiveVideoVisibleRef.current = false;
      setVisualLiveVideoVisible(false);
      setLiveVideoRevealActive(false);
      return;
    }
    if (!wasVisible) {
      setLiveVideoRevealActive(false);
    }
  }, [props.liveVideoVisible]);

  React.useLayoutEffect(() => {
    if (!props.liveVideoModeActive || !props.liveVideoVisible) {
      return;
    }
    const element = coverAreaRef.current;
    if (!element) {
      return;
    }
    let frame = 0;
    const revealCurrentRect = () => {
      const rect = element.getBoundingClientRect();
      if (rect.width < 1 || rect.height < 1) {
        return;
      }
      setNativeVideoHole(rect, readListenNativeVideoRadius(element));
      visualLiveVideoVisibleRef.current = true;
      setVisualLiveVideoVisible(true);
    };
    frame = window.requestAnimationFrame(revealCurrentRect);
    return () => window.cancelAnimationFrame(frame);
  }, [props.liveVideoModeActive, props.liveVideoVisible, props.videoId, setNativeVideoHole]);

  React.useLayoutEffect(() => {
    const onLiveVideoRectChange = props.onLiveVideoRectChange;
    if (!props.liveVideoModeActive || !onLiveVideoRectChange) {
      return;
    }
    const element = coverAreaRef.current;
    if (!element) {
      return;
    }
    let readFrame = 0;
    let commitFrame = 0;
    let lastRectSignature = "";
    const timers: number[] = [];
    const readRadius = () => readListenNativeVideoRadius(element);
    const pushRect = (force = false) => {
      const rect = element.getBoundingClientRect();
      if (rect.width < 1 || rect.height < 1) {
        return;
      }
      const radius = readRadius();
      const viewportWidth = Math.max(1, window.innerWidth);
      const viewportHeight = Math.max(1, window.innerHeight);
      const shellElement = element.closest(".listen-live-video-shell");
      const shellRect =
        shellElement instanceof Element
          ? shellElement.getBoundingClientRect()
          : null;
      const stageWidth = shellRect
        ? Math.max(1, shellRect.width - LISTEN_LIVE_VIDEO_FRAME_GAP * 2)
        : Math.max(1, viewportWidth - LISTEN_LIVE_VIDEO_FRAME_GAP * 2);
      const stageHeight = shellRect
        ? Math.max(
            1,
            shellRect.height -
              LISTEN_LIVE_VIDEO_TOPBAR_HEIGHT -
              LISTEN_LIVE_VIDEO_FRAME_GAP * 2,
          )
        : Math.max(
            1,
            viewportHeight -
              LISTEN_LIVE_VIDEO_TOPBAR_HEIGHT -
              LISTEN_LIVE_VIDEO_FRAME_GAP * 2,
          );
      const centerX = rect.left + rect.width / 2;
      const centerY = rect.top + rect.height / 2;
      const signature = [
        Math.round(rect.left * 2) / 2,
        Math.round(rect.top * 2) / 2,
        Math.round(rect.width * 2) / 2,
        Math.round(rect.height * 2) / 2,
        Math.round(centerX * 2) / 2,
        Math.round(centerY * 2) / 2,
        Math.round(stageWidth * 2) / 2,
        Math.round(stageHeight * 2) / 2,
        Math.round(viewportWidth * 2) / 2,
        Math.round(viewportHeight * 2) / 2,
        Math.round(radius * 2) / 2,
      ].join(":");
      if (!force && signature === lastRectSignature) {
        return;
      }
      lastRectSignature = signature;
      rectRevealRequestRef.current += 1;
      const requestToken = rectRevealRequestRef.current;
      const wasVisuallyVisible = visualLiveVideoVisibleRef.current;
      visualLiveVideoVisibleRef.current = false;
      resetNativeVideoHole();
      setVisualLiveVideoVisible(false);
      setLiveVideoRevealActive(false);
      const applyResult = onLiveVideoRectChange({
        x: rect.left,
        y: rect.top,
        width: rect.width,
        height: rect.height,
        centerX,
        centerY,
        stageWidth,
        stageHeight,
        viewportWidth,
        viewportHeight,
        radius,
      });
      void Promise.resolve(applyResult).then((shown) => {
        if (rectRevealRequestRef.current !== requestToken || shown === false) {
          return;
        }
        const nextRect = element.getBoundingClientRect();
        if (nextRect.width < 1 || nextRect.height < 1) {
          return;
        }
        setNativeVideoHole(nextRect, readRadius());
        if (!wasVisuallyVisible) {
          setLiveVideoRevealActive(true);
          timers.push(window.setTimeout(() => {
            if (rectRevealRequestRef.current !== requestToken) {
              return;
            }
            setLiveVideoRevealActive(false);
          }, LISTEN_LIVE_VIDEO_REVEAL_MS));
        }
        visualLiveVideoVisibleRef.current = true;
        setVisualLiveVideoVisible(true);
      }).catch(() => {});
    };
    const cancelScheduledRect = () => {
      window.cancelAnimationFrame(readFrame);
      window.cancelAnimationFrame(commitFrame);
      readFrame = 0;
      commitFrame = 0;
    };
    const syncRect = (force = false) => {
      cancelScheduledRect();
      readFrame = window.requestAnimationFrame(() => {
        commitFrame = window.requestAnimationFrame(() => pushRect(force));
      });
    };
    const scheduleRect = () => syncRect();
    syncRect(true);
    LISTEN_LIVE_VIDEO_GEOMETRY_SETTLE_DELAYS_MS.forEach((delay) => {
      timers.push(window.setTimeout(scheduleRect, delay));
    });
    const shell = element.closest(".listen-live-video-shell");
    const content = element.closest(".listen-content-surface");
    const animatedTargets = [shell, content].filter(
      (target): target is Element => target instanceof Element,
    );
    const observer =
      typeof ResizeObserver === "undefined"
        ? null
        : new ResizeObserver(scheduleRect);
    observer?.observe(element);
    animatedTargets.forEach((target) => {
      target.addEventListener("animationend", scheduleRect);
      target.addEventListener("transitionend", scheduleRect);
    });
    window.addEventListener("resize", scheduleRect);
    window.addEventListener("scroll", scheduleRect, true);
    window.visualViewport?.addEventListener("resize", scheduleRect);
    window.visualViewport?.addEventListener("scroll", scheduleRect);
    return () => {
      rectRevealRequestRef.current += 1;
      cancelScheduledRect();
      timers.forEach((timer) => window.clearTimeout(timer));
      observer?.disconnect();
      animatedTargets.forEach((target) => {
        target.removeEventListener("animationend", scheduleRect);
        target.removeEventListener("transitionend", scheduleRect);
      });
      window.removeEventListener("resize", scheduleRect);
      window.removeEventListener("scroll", scheduleRect, true);
      window.visualViewport?.removeEventListener("resize", scheduleRect);
      window.visualViewport?.removeEventListener("scroll", scheduleRect);
      resetNativeVideoHole();
    };
  }, [
    props.liveVideoModeActive,
    props.onLiveVideoRectChange,
    props.videoId,
    resetNativeVideoHole,
    setNativeVideoHole,
  ]);
  return (
    <div
      className={cn(
        "listen-live-video-shell listen-video-shell",
        props.reserveWindowControls && "listen-video-shell-windows",
      )}
    >
      <div className="wails-no-drag absolute left-3 top-3 z-40 sm:left-5">
        <ListenPlayerIconButton
          label={listLabel}
          tooltipSide="bottom"
          className="listen-video-expand-button"
          onClick={props.onToggleList}
        >
          {props.listOpen ? (
            <PanelLeftClose className="h-4 w-4" />
          ) : (
            <PanelLeftOpen className="h-4 w-4" />
          )}
        </ListenPlayerIconButton>
      </div>
      <header
        className={cn(
          "listen-video-topbar wails-drag",
          props.reserveWindowControls && "listen-video-topbar-windows",
        )}
      >
        <div className="listen-video-info-area">
          <ListenFullscreenChannelCover
            httpBaseURL={props.httpBaseURL}
            track={props.track}
            title={titleLabel}
          />
          <div className="listen-video-info">
            <div className="listen-video-title-line">
              <h1>
                <ListenScrollingText
                  text={titleLabel}
                  as="span"
                />
              </h1>
              {authorLabel ? (
                <>
                  <span className="listen-video-title-separator" aria-hidden="true">
                    ·
                  </span>
                  <span className="listen-video-author">
                    <ListenScrollingText text={authorLabel} as="span" />
                  </span>
                </>
              ) : null}
            </div>
            <div className="listen-video-status-cluster">
              {visualLiveVideoVisible && props.onFitLiveVideoWindow ? (
                <div className="listen-video-fit-group wails-no-drag">
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <button
                        type="button"
                        className="listen-video-fit-group-button"
                        aria-label={props.text.listen.fitWindow}
                        onClick={props.onFitLiveVideoWindow}
                      >
                        <Ratio className="h-3 w-3" />
                      </button>
                    </TooltipTrigger>
                    <TooltipContent side="bottom">
                      {props.text.listen.fitWindow}
                    </TooltipContent>
                  </Tooltip>
                </div>
              ) : null}
              <span className={cn("listen-video-playback-status", statusClass)}>
                <span>{statusLabel}</span>
              </span>
            </div>
          </div>
          <div className="listen-video-actions wails-no-drag">
            <ListenFullscreenVolumeControl
              muted={props.muted}
              volume={props.volume}
              text={props.text}
              onToggleMute={props.onToggleMute}
              onVolumeChange={props.onVolumeChange}
            />
            <ListenPlayerIconButton
              label={playLabel}
              tooltip={false}
              disabled={playbackDisabled}
              className="listen-video-action-button listen-video-action-button-primary"
              onClick={props.onTogglePlayback}
            >
              {props.loading ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : props.playing ? (
                <Pause className="h-4 w-4 fill-current" />
              ) : (
                <Play className="ml-0.5 h-4 w-4 fill-current" />
              )}
            </ListenPlayerIconButton>
            <ListenPlayerIconButton
              label={props.text.listen.stop}
              tooltip={false}
              disabled={stopDisabled}
              className="listen-video-action-button"
              onClick={props.onStopPlayback}
            >
              <Square className="h-3.5 w-3.5 fill-current" />
            </ListenPlayerIconButton>
          </div>
        </div>
      </header>
      <div
        ref={coverAreaRef}
        className="listen-video-cover-area"
        data-native-video={visualLiveVideoVisible ? "underlay" : "pending"}
        data-reveal={liveVideoRevealActive ? "true" : undefined}
      >
        {(!visualLiveVideoVisible || liveVideoRevealActive) ? (
          <div
            className="listen-video-pending-layer"
            data-handoff={liveVideoRevealActive ? "true" : undefined}
          >
            <PetDisplay
              pet={props.pet}
              imageUrl={props.petImageURL}
              animation="review"
              alt={props.title || props.text.listen.selectStation}
              size={88}
              className="h-24 w-24"
            />
          </div>
        ) : null}
      </div>
    </div>
  );
}

function ListenFullscreenChannelCover(props: {
  httpBaseURL: string;
  track?: ListenOnlineItem;
  title: string;
}) {
  return (
    <div className="listen-video-avatar-button" aria-hidden="true">
      <ListenLiveFlatCoverImage
        httpBaseURL={props.httpBaseURL}
        track={props.track}
        title={props.title}
        className="h-full w-full object-cover"
      />
    </div>
  );
}

function ListenLiveFlatCoverImage(props: {
  httpBaseURL: string;
  track?: ListenOnlineItem;
  title: string;
  className?: string;
}) {
  const imageKey = `${props.httpBaseURL}:${props.track?.id ?? ""}:${props.track?.thumbnailUrl ?? ""}:${props.track?.videoId ?? ""}`;
  const candidates = React.useMemo(() => (
    props.track
      ? buildListenPosterCandidates(props.httpBaseURL, props.track)
      : [LISTEN_DEFAULT_COVER_IMAGE_URL]
  ), [props.httpBaseURL, props.track]);
  const [candidateIndex, setCandidateIndex] = React.useState(0);
  const src =
    candidates[candidateIndex] ?? LISTEN_DEFAULT_COVER_IMAGE_URL;

  React.useEffect(() => {
    setCandidateIndex(0);
  }, [imageKey]);

  return (
    <img
      src={src}
      alt={props.title}
      className={cn("block bg-white", props.className)}
      draggable={false}
      onError={() => {
        setCandidateIndex((current) =>
          current + 1 < candidates.length ? current + 1 : current,
        );
      }}
    />
  );
}

function resolveListenLivePlaybackStatusClass(state: ListenRemotePlaybackState) {
  switch (state) {
    case "playing":
      return "is-playing";
    case "loading":
    case "buffering":
      return "is-loading";
    case "error":
      return "is-error";
    default:
      return "";
  }
}

function ListenFullscreenVolumeControl(props: {
  muted: boolean;
  volume: number;
  text: ReturnType<typeof getXiaText>;
  onToggleMute: () => void;
  onVolumeChange: (value: number) => void;
}) {
  const visibleVolume = props.muted ? 0 : clampVolume(props.volume);
  const volumePercent = Math.round(visibleVolume * 1000) / 10;
  const [open, setOpen] = React.useState(false);
  const closeTimerRef = React.useRef<number | null>(null);

  const clearCloseTimer = React.useCallback(() => {
    if (closeTimerRef.current === null) {
      return;
    }
    window.clearTimeout(closeTimerRef.current);
    closeTimerRef.current = null;
  }, []);

  const openSlider = React.useCallback(() => {
    clearCloseTimer();
    setOpen(true);
  }, [clearCloseTimer]);

  const scheduleClose = React.useCallback(() => {
    clearCloseTimer();
    closeTimerRef.current = window.setTimeout(() => {
      setOpen(false);
      closeTimerRef.current = null;
    }, 140);
  }, [clearCloseTimer]);

  React.useEffect(() => clearCloseTimer, [clearCloseTimer]);

  return (
    <div
      className="listen-video-volume wails-no-drag group/listen-fullscreen-volume"
      data-open={open ? "true" : "false"}
      onPointerLeave={scheduleClose}
      onPointerDownCapture={(event) => event.stopPropagation()}
      onMouseDownCapture={(event) => event.stopPropagation()}
      onBlurCapture={scheduleClose}
    >
      <span
        className="listen-video-volume-trigger wails-no-drag"
        onPointerEnter={openSlider}
        onFocusCapture={openSlider}
      >
        <ListenPlayerIconButton
          label={props.muted || props.volume <= 0 ? props.text.listen.unmute : props.text.listen.mute}
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
          onPointerEnter={clearCloseTimer}
          onPointerDownCapture={(event) => event.stopPropagation()}
          onMouseDownCapture={(event) => event.stopPropagation()}
          onFocusCapture={clearCloseTimer}
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
              onChange={(event) => props.onVolumeChange(Number(event.target.value))}
            />
          </div>
        </div>
      ) : null}
    </div>
  );
}

function ListenTrackInfoRow(props: {
  title: string;
  subtitle: string;
  subtitleArtistParts?: ListenArtistLabelPart[];
  onSubtitleClick?: () => void;
  onSubtitleArtistClick?: (artist: string) => void;
  actions?: React.ReactNode;
}) {
  return (
    <div className="mt-5 flex min-h-14 items-center justify-between gap-4">
      <div className="min-w-0 flex-1 overflow-hidden text-left">
        <ListenScrollingText
          text={props.title}
          className="text-lg font-semibold leading-6 text-sidebar-foreground"
        />
        <ListenSubtitleText
          text={props.subtitle}
          artistParts={props.subtitleArtistParts}
          className="mt-0.5 text-sm leading-5 text-sidebar-foreground/58"
          onClick={props.onSubtitleClick}
          onArtistClick={props.onSubtitleArtistClick}
        />
      </div>
      {props.actions ? (
        <div className="relative z-10 flex shrink-0 items-center gap-1.5">{props.actions}</div>
      ) : null}
    </div>
  );
}

function ListenPlayerInlineVisualizer(props: {
  mode: EqualizerVisualizerMode;
  enabled: boolean;
  active: boolean;
}) {
  const enabled = props.enabled && props.active && isEqualizerSpectrumVisualizerMode(props.mode);
  const frame = useEqualizerVisualizerFrame(enabled);
  const visible = enabled && frame.running;
  return (
    <div
      className="listen-player-inline-visualizer pointer-events-none w-full"
      data-visible={visible ? "true" : "false"}
      aria-hidden="true"
    >
      {visible ? (
        <ListenInlineVisualizer
          mode={props.mode}
          frame={frame}
          visible={visible}
          active={props.active}
          className="h-full w-full"
        />
      ) : null}
    </div>
  );
}

function ListenArtworkVisualizerBridge(props: {
  mode: EqualizerVisualizerMode;
  enabled: boolean;
  active: boolean;
  onVisibleChange?: (visible: boolean) => void;
}) {
  const enabled = props.enabled && props.active && isEqualizerArtworkVisualizerMode(props.mode);
  const frame = useEqualizerVisualizerFrame(enabled);
  const visible = enabled && frame.running;
  React.useEffect(() => {
    props.onVisibleChange?.(visible);
    return () => props.onVisibleChange?.(false);
  }, [props.onVisibleChange, visible]);
  if (!enabled) {
    return null;
  }
  return (
    <ListenArtworkVisualizer
      mode={props.mode}
      frame={frame}
      active={props.active}
      visible={visible}
    />
  );
}

function ListenSubtitleText(props: {
  text: string;
  artistParts?: ListenArtistLabelPart[];
  className?: string;
  onClick?: () => void;
  onArtistClick?: (artist: string) => void;
}) {
  const artistParts = props.artistParts ?? [];
  if (
    props.onArtistClick &&
    listenArtistCountFromLabelParts(artistParts) > 0
  ) {
    return (
      <ListenArtistScrollingText
        text={props.text}
        artistParts={artistParts}
        className={props.className}
        onArtistClick={props.onArtistClick}
      />
    );
  }
  return (
    <ListenScrollingText
      text={props.text}
      className={props.className}
      onClick={props.onClick}
    />
  );
}

function ListenArtistScrollingText(props: {
  text: string;
  artistParts: ListenArtistLabelPart[];
  className?: string;
  onArtistClick: (artist: string) => void;
}) {
  const containerRef = React.useRef<HTMLDivElement | null>(null);
  const contentRef = React.useRef<HTMLSpanElement | null>(null);
  const [overflow, setOverflow] = React.useState(0);
  const normalizedText = props.text.trim();
  const scrolling = overflow > 1;
  const style = scrolling
    ? ({
        "--listen-marquee-shift": `-${Math.ceil(overflow + 18)}px`,
        "--listen-marquee-duration": `${Math.min(
          14,
          Math.max(7, (overflow + 180) / 30),
        )}s`,
      } as React.CSSProperties)
    : undefined;

  React.useLayoutEffect(() => {
    const container = containerRef.current;
    const contentElement = contentRef.current;
    if (!container || !contentElement) {
      return;
    }
    const syncOverflow = () => {
      setOverflow(
        Math.max(0, contentElement.scrollWidth - container.clientWidth),
      );
    };
    syncOverflow();
    if (typeof ResizeObserver === "undefined") {
      return;
    }
    const observer = new ResizeObserver(syncOverflow);
    observer.observe(container);
    observer.observe(contentElement);
    return () => observer.disconnect();
  }, [normalizedText, props.artistParts]);

  return (
    <div
      ref={containerRef}
      className={cn(
        "group/listen-marquee relative block w-full max-w-full min-w-0 overflow-hidden whitespace-nowrap text-left",
        props.className,
      )}
      title={normalizedText}
    >
      <span
        ref={contentRef}
        className={cn(
          "inline-block max-w-none pr-4 align-top",
          scrolling && "listen-marquee-text",
        )}
        style={style}
      >
        {props.artistParts.map((part, index) =>
          part.kind === "separator" ? (
            <span key={`separator-${index}`} aria-hidden="true">
              {part.text}
            </span>
          ) : (
            <button
              key={`artist-${index}-${part.text}`}
              type="button"
              className="inline rounded-sm bg-transparent p-0 text-left font-[inherit] leading-[inherit] text-inherit underline-offset-4 transition hover:text-sidebar-foreground hover:underline focus-visible:outline-none"
              title={part.text}
              onClick={() => props.onArtistClick(part.text)}
            >
              {part.text}
            </button>
          ),
        )}
      </span>
    </div>
  );
}

function ListenScrollingText(props: {
  text: string;
  className?: string;
  onClick?: () => void;
  as?: "div" | "span";
}) {
  const containerRef = React.useRef<HTMLElement | null>(null);
  const contentRef = React.useRef<HTMLSpanElement | null>(null);
  const [overflow, setOverflow] = React.useState(0);
  const normalizedText = props.text.trim();
  const scrolling = overflow > 1;
  const style = scrolling
    ? ({
        "--listen-marquee-shift": `-${Math.ceil(overflow + 18)}px`,
        "--listen-marquee-duration": `${Math.min(
          14,
          Math.max(7, (overflow + 180) / 30),
        )}s`,
      } as React.CSSProperties)
    : undefined;
  const className = cn(
    "group/listen-marquee relative block w-full max-w-full min-w-0 overflow-hidden whitespace-nowrap text-left",
    props.onClick &&
      "rounded-md underline-offset-4 transition hover:text-sidebar-foreground hover:underline focus-visible:outline-none",
    props.className,
  );
  const content = (
    <span
      ref={contentRef}
      className={cn(
        "inline-block max-w-none pr-4 align-top",
        scrolling && "listen-marquee-text",
      )}
      style={style}
    >
      {normalizedText}
    </span>
  );

  React.useLayoutEffect(() => {
    const container = containerRef.current;
    const contentElement = contentRef.current;
    if (!container || !contentElement) {
      return;
    }
    const syncOverflow = () => {
      setOverflow(
        Math.max(0, contentElement.scrollWidth - container.clientWidth),
      );
    };
    syncOverflow();
    if (typeof ResizeObserver === "undefined") {
      return;
    }
    const observer = new ResizeObserver(syncOverflow);
    observer.observe(container);
    observer.observe(contentElement);
    return () => observer.disconnect();
  }, [normalizedText]);

  if (props.onClick) {
    return (
      <button
        ref={containerRef as React.RefObject<HTMLButtonElement>}
        type="button"
        className={className}
        title={normalizedText}
        onClick={props.onClick}
      >
        {content}
      </button>
    );
  }

  if (props.as === "span") {
    return (
      <span
        ref={containerRef as React.RefObject<HTMLSpanElement>}
        className={className}
        title={normalizedText}
      >
        {content}
      </span>
    );
  }

  return (
    <div
      ref={containerRef as React.RefObject<HTMLDivElement>}
      className={className}
      title={normalizedText}
    >
      {content}
    </div>
  );
}

function ListenPlayerProgress(props: {
  progress: {
    currentTime: number;
    duration: number;
    bufferedTime: number;
  };
  text: ReturnType<typeof getXiaText>;
  live?: boolean;
  playing?: boolean;
  advertising?: boolean;
  advertisingLabel?: string;
  loading?: boolean;
  errorActive?: boolean;
  errorLabel?: string;
  errorTitle?: string;
  onSeek?: (seconds: number) => void;
}) {
  const duration = Number.isFinite(props.progress.duration)
    ? Math.max(0, props.progress.duration)
    : 0;
  const currentTime = Math.max(
    0,
    Math.min(
      Number.isFinite(props.progress.currentTime)
        ? props.progress.currentTime
        : 0,
      duration,
    ),
  );
  const bufferedPercent =
    duration > 0
      ? Math.max(0, Math.min(100, (props.progress.bufferedTime / duration) * 100))
      : 0;
  const playedPercent =
    duration > 0 ? Math.max(0, Math.min(100, (currentTime / duration) * 100)) : 0;
  const canSeek = duration > 0 && Boolean(props.onSeek);
  const remainingTime = Math.max(0, duration - currentTime);
  const errorCode = props.errorLabel?.trim() || "";
  const errorMessage = props.errorTitle?.trim() || "";
  const hasError = Boolean(props.errorActive || errorCode || errorMessage);
  const advertising = Boolean(props.advertising && !hasError);
  const loading = Boolean(props.loading && !hasError && !advertising);
  const statusActive = props.live || hasError || advertising || loading;
  const errorLabel = errorCode
    ? `${props.text.listen.errorCodeLabel}: ${errorCode}`
    : props.text.listen.errorStatus;
  const errorTooltip = errorMessage || errorLabel;
  const label = advertising
    ? props.advertisingLabel?.trim() || props.text.listen.adBadge
    : loading
      ? props.text.listen.loading
      : props.text.listen.liveBadge;
  const hasTimedAdProgress =
    advertising &&
    duration > 0 &&
    (playedPercent > 0 || bufferedPercent > 0);
  const livePlaying = Boolean(props.playing && !loading && !advertising && !hasError);
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

  if (statusActive) {
    return (
      <div className="mt-4">
        <div className="relative flex h-6 items-center">
          <div className="pointer-events-none absolute left-0 right-0 top-1/2 h-1.5 -translate-y-1/2 rounded-full bg-sidebar-foreground/10">
            {hasError ? null : advertising && hasTimedAdProgress ? (
              <>
                <div
                  className="h-full rounded-full bg-sidebar-foreground/12"
                  style={{ width: `${bufferedPercent}%` }}
                />
                <div
                  className="absolute inset-y-0 left-0 rounded-full bg-sidebar-primary"
                  style={{ width: `${playedPercent}%` }}
                />
              </>
            ) : advertising ? (
              <div className="h-full w-full rounded-full bg-red-500/42 dark:bg-red-400/42" />
            ) : loading ? (
              <div className="h-full w-full animate-pulse rounded-full bg-sidebar-primary/45" />
            ) : props.live ? (
              <div
                className={cn(
                  "relative h-full w-full rounded-full",
                  livePlaying
                    ? "bg-sidebar-primary/72"
                    : "bg-sidebar-primary/34",
                )}
              >
                <span
                  className={cn(
                    "absolute right-0 top-1/2 block h-2.5 w-2.5 -translate-y-1/2 translate-x-1/2 rounded-full bg-sidebar-primary shadow-[0_0_0_3px_hsl(var(--sidebar-background)/0.88)]",
                    livePlaying ? "opacity-100" : "opacity-55",
                  )}
                >
                  {livePlaying ? (
                    <span className="absolute inset-0 rounded-full bg-sidebar-primary/55 animate-ping" />
                  ) : null}
                </span>
              </div>
            ) : (
              <div
                className="absolute inset-y-0 left-0 rounded-full bg-sidebar-primary"
                style={{ width: `${playedPercent}%` }}
              />
            )}
          </div>
        </div>
        <div className="mt-0.5 grid h-4 grid-cols-[1fr_auto_1fr] items-center text-[11px] font-medium tabular-nums text-sidebar-foreground/46">
          {hasError ? (
            <>
              <Tooltip>
                <TooltipTrigger asChild>
                  <span className="min-w-0 truncate text-left font-semibold text-red-600 dark:text-red-300">
                    {errorLabel}
                  </span>
                </TooltipTrigger>
                <TooltipContent side="top" multiline className="text-left text-xs leading-snug">
                  {errorTooltip}
                </TooltipContent>
              </Tooltip>
              <span aria-hidden="true" />
              <span aria-hidden="true" />
            </>
          ) : advertising ? (
            <>
              <span className="min-w-0 truncate text-left font-semibold text-red-600 dark:text-red-300">
                {label}
              </span>
              <span aria-hidden="true" />
              <span aria-hidden="true" />
            </>
          ) : loading ? (
            <>
              <span aria-hidden="true" />
              <span className="justify-self-center font-semibold text-sidebar-foreground/55">
                {label}
              </span>
              <span aria-hidden="true" />
            </>
          ) : (
            <>
              <span aria-hidden="true" />
              <span className="justify-self-center font-semibold uppercase tracking-[0.12em] text-red-600 dark:text-red-300">
                {label}
              </span>
              <span aria-hidden="true" />
            </>
          )}
        </div>
      </div>
    );
  }

  return (
    <div className="mt-4">
      <div
        className="listen-player-progress-control wails-no-drag group/progress relative flex h-6 items-center"
        onPointerDown={(event) => event.stopPropagation()}
        onMouseDown={(event) => event.stopPropagation()}
      >
        <div className="pointer-events-none absolute left-0 right-0 top-1/2 h-1.5 -translate-y-1/2 overflow-hidden rounded-full bg-sidebar-foreground/10">
          <div
            className="h-full rounded-full bg-sidebar-foreground/12"
            style={{ width: `${bufferedPercent}%` }}
          />
          <div
            className="absolute inset-y-0 left-0 rounded-full bg-sidebar-primary"
            style={{ width: `${playedPercent}%` }}
          />
        </div>
        {canSeek ? (
          <span
            aria-hidden="true"
            className="pointer-events-none absolute top-1/2 h-3.5 w-3.5 -translate-x-1/2 -translate-y-1/2 scale-75 rounded-full border border-sidebar-background bg-sidebar-primary opacity-0 shadow-[0_5px_14px_-8px_hsl(var(--sidebar-primary)/0.9)] transition-[left,opacity,transform] duration-150 ease-out group-hover/progress:scale-100 group-hover/progress:opacity-100 group-focus-within/progress:scale-100 group-focus-within/progress:opacity-100"
            style={{ left: `${playedPercent}%` }}
          />
        ) : null}
        <input
          type="range"
          min={0}
          max={duration || 0}
          step={0.01}
          value={currentTime}
          disabled={!canSeek}
          aria-label={props.text.listen.nowPlaying}
          className="wails-no-drag relative z-10 h-6 w-full cursor-pointer touch-none opacity-0 disabled:cursor-not-allowed"
          onInput={handleSeekInput}
          onChange={handleSeekInput}
        />
      </div>
      <div className="mt-0.5 flex items-center justify-between text-[11px] font-medium tabular-nums text-sidebar-foreground/46">
        <span>{formatProgressSeconds(currentTime)}</span>
        <span>-{formatProgressSeconds(remainingTime)}</span>
      </div>
    </div>
  );
}

function ListenPlayerTransport(props: {
  playing: boolean;
  loading: boolean;
  playMode: ListenPlayMode;
  live?: boolean;
  disabled?: boolean;
  text: ReturnType<typeof getXiaText>;
  onPrevious: () => void;
  onNext: () => void;
  onPlayModeChange: (mode: ListenPlayMode) => void;
  onTogglePlayback: React.MouseEventHandler<HTMLButtonElement>;
}) {
  const shuffleActive = props.playMode === "shuffle";
  const repeatActive = !props.live && props.playMode === "repeat";
  const playLabel = props.playing ? props.text.listen.pause : props.text.listen.play;

  if (props.live) {
    return (
      <div className="mt-3 flex h-14 items-center justify-center">
        <Tooltip>
          <TooltipTrigger asChild>
            <button
              type="button"
              className={cn(
                LISTEN_PRIMARY_PLAY_BUTTON_CLASS,
                LISTEN_PRIMARY_PLAY_BUTTON_SIZE_CLASS.medium,
                props.disabled
                  ? "cursor-not-allowed opacity-35"
                  : LISTEN_PRIMARY_PLAY_BUTTON_HOVER_CLASS,
              )}
              disabled={props.disabled}
              aria-label={playLabel}
              title={playLabel}
              onClick={props.onTogglePlayback}
            >
              {props.loading ? (
                <Loader2 className={cn(LISTEN_PRIMARY_PLAY_ICON_SIZE_CLASS.medium, "animate-spin")} />
              ) : props.playing ? (
                <Pause className={cn(LISTEN_PRIMARY_PLAY_ICON_SIZE_CLASS.medium, "fill-current")} />
              ) : (
                <Play className={cn("ml-0.5 fill-current", LISTEN_PRIMARY_PLAY_ICON_SIZE_CLASS.medium)} />
              )}
            </button>
          </TooltipTrigger>
          <TooltipContent side="top">{playLabel}</TooltipContent>
        </Tooltip>
      </div>
    );
  }

  return (
    <div className="mt-3 grid h-14 grid-cols-[3.5rem_1fr_3.5rem] items-center">
      <div className="justify-self-start">
        <ListenTransportIconButton
          label={props.text.listen.playModeShuffle}
          active={shuffleActive}
          disabled={props.disabled}
          size="small"
          onClick={() => props.onPlayModeChange(shuffleActive ? "order" : "shuffle")}
        >
          <Shuffle className="h-4 w-4" />
        </ListenTransportIconButton>
      </div>
      <div className="flex items-center justify-center gap-3">
        <ListenTransportIconButton
          label={props.text.listen.previous}
          disabled={props.disabled}
          onClick={props.onPrevious}
        >
          <SkipBack className="h-5 w-5" />
        </ListenTransportIconButton>
        <Tooltip>
          <TooltipTrigger asChild>
            <button
              type="button"
              className={cn(
                LISTEN_PRIMARY_PLAY_BUTTON_CLASS,
                LISTEN_PRIMARY_PLAY_BUTTON_SIZE_CLASS.medium,
                props.disabled
                  ? "cursor-not-allowed opacity-35"
                  : LISTEN_PRIMARY_PLAY_BUTTON_HOVER_CLASS,
              )}
              disabled={props.disabled}
              aria-label={playLabel}
              title={playLabel}
              onClick={props.onTogglePlayback}
            >
              {props.loading ? (
                <Loader2 className={cn(LISTEN_PRIMARY_PLAY_ICON_SIZE_CLASS.medium, "animate-spin")} />
              ) : props.playing ? (
                <Pause className={cn(LISTEN_PRIMARY_PLAY_ICON_SIZE_CLASS.medium, "fill-current")} />
              ) : (
                <Play className={cn("ml-0.5 fill-current", LISTEN_PRIMARY_PLAY_ICON_SIZE_CLASS.medium)} />
              )}
            </button>
          </TooltipTrigger>
          <TooltipContent side="top">{playLabel}</TooltipContent>
        </Tooltip>
        <ListenTransportIconButton
          label={props.text.listen.next}
          disabled={props.disabled}
          onClick={props.onNext}
        >
          <SkipForward className="h-5 w-5" />
        </ListenTransportIconButton>
      </div>
      <div className="justify-self-end">
        <ListenTransportIconButton
          label={props.text.listen.playModeRepeat}
          active={repeatActive}
          disabled={props.live || props.disabled}
          size="small"
          onClick={() => props.onPlayModeChange(repeatActive ? "order" : "repeat")}
        >
          <Repeat2 className="h-4 w-4" />
        </ListenTransportIconButton>
      </div>
    </div>
  );
}

function ListenTransportIconButton(props: {
  label: string;
  active?: boolean;
  disabled?: boolean;
  size?: "normal" | "small";
  children: React.ReactNode;
  onClick?: React.MouseEventHandler<HTMLButtonElement>;
}) {
  const small = props.size === "small";
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          data-active={props.active ? "true" : "false"}
          disabled={props.disabled}
          className={cn(
            "relative flex items-center justify-center rounded-full text-sidebar-foreground/55 transition-[transform,background-color,color,opacity] duration-200 ease-out hover:scale-[1.05] hover:bg-sidebar-background/54 hover:text-sidebar-foreground active:scale-95 focus-visible:outline-none",
            "data-[active=true]:text-sidebar-primary",
            "disabled:pointer-events-none disabled:opacity-35",
            small ? "h-8 w-8" : "h-10 w-10",
          )}
          aria-label={props.label}
          title={props.label}
          onClick={props.onClick}
        >
          {props.children}
          {props.active ? (
            <span className="absolute bottom-0 h-1 w-1 rounded-full bg-sidebar-primary" />
          ) : null}
        </button>
      </TooltipTrigger>
      <TooltipContent side="top">
        {props.label}
      </TooltipContent>
    </Tooltip>
  );
}

function ListenPlayerVolume(props: {
  muted: boolean;
  volume: number;
  text: ReturnType<typeof getXiaText>;
  onToggleMute: () => void;
  onVolumeChange: (value: number) => void;
}) {
  const visibleVolume = props.muted ? 0 : clampVolume(props.volume);
  const volumePercent = Math.round(visibleVolume * 1000) / 10;

  return (
    <div className="mt-4 flex h-8 items-center gap-3 text-sidebar-foreground/48">
      <ListenPlayerIconButton
        label={props.muted || props.volume <= 0 ? props.text.listen.unmute : props.text.listen.mute}
        className="h-8 w-8 shadow-none"
        onClick={props.onToggleMute}
      >
        {props.muted || props.volume <= 0 ? (
          <VolumeX className="h-4 w-4" />
        ) : (
          <Volume2 className="h-4 w-4" />
        )}
      </ListenPlayerIconButton>
      <div className="group/volume-slider relative flex h-6 min-w-0 flex-1 items-center">
        <div className="pointer-events-none absolute left-0 right-0 top-1/2 h-1.5 -translate-y-1/2 overflow-hidden rounded-full bg-sidebar-foreground/10">
          <div
            className="absolute inset-y-0 left-0 rounded-full bg-sidebar-primary transition-[width] duration-150 ease-out"
            style={{ width: `${volumePercent}%` }}
          />
        </div>
        <span
          aria-hidden="true"
          className="pointer-events-none absolute top-1/2 h-3.5 w-3.5 -translate-x-1/2 -translate-y-1/2 scale-75 rounded-full border border-sidebar-background bg-sidebar-primary opacity-0 shadow-[0_5px_14px_-8px_hsl(var(--sidebar-primary)/0.9)] transition-[left,opacity,transform] duration-150 ease-out group-hover/volume-slider:scale-100 group-hover/volume-slider:opacity-100 group-focus-within/volume-slider:scale-100 group-focus-within/volume-slider:opacity-100"
          style={{ left: `${volumePercent}%` }}
        />
        <input
          type="range"
          min={0}
          max={1}
          step={0.01}
          value={visibleVolume}
          aria-label={props.text.listen.volume}
          title={props.text.listen.volume}
          className="relative z-10 h-6 w-full cursor-pointer opacity-0"
          onChange={(event) => props.onVolumeChange(Number(event.target.value))}
        />
      </div>
      <ListenPlayerIconButton
        label={props.text.listen.volume}
        className="h-8 w-8 shadow-none"
        onClick={() => props.onVolumeChange(1)}
      >
        <Volume2 className="h-4 w-4" />
      </ListenPlayerIconButton>
    </div>
  );
}
