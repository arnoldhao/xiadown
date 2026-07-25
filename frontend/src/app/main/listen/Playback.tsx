import { Call,Events,System,Window } from "@wailsio/runtime";
import {
Download,
FolderOpen,
Heart,
Loader2,
Pause,
Play,
SkipForward,
} from "lucide-react";
import * as React from "react";

import {
getXiaText
} from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
import { LISTEN_DEFAULT_COVER_IMAGE_URL } from "@/shared/assets/default-cover";
import type { Pet } from "@/shared/contracts/pets";
import { messageBus } from "@/shared/message";
import { openExternalURL } from "@/shared/query/system";
import { useSettingsStore } from "@/shared/store/settings";
import { GlassSurface } from "@/shared/ui/glass-surface";
import { Button } from "@/shared/ui/button";
import {
Tooltip,
TooltipContent,
TooltipProvider,
TooltipTrigger
} from "@/shared/ui/tooltip";
import {
LISTEN_PLAYER_FOOTER_ICON_BUTTON_CLASS,
LISTEN_PLAYER_SURFACE_WIDTH_CLASS,
LISTEN_PRIMARY_PLAY_BUTTON_CLASS,
LISTEN_PRIMARY_PLAY_BUTTON_HOVER_CLASS,
LISTEN_PRIMARY_PLAY_BUTTON_SIZE_CLASS,
LISTEN_PRIMARY_PLAY_ICON_SIZE_CLASS,
} from "@/shared/styles/listen";
import { LISTEN_LIVE_PLAYER_EVENT,LISTEN_LIVE_PLAYER_SERVICE,LISTEN_NATIVE_PLAYER_EVENT,LISTEN_NATIVE_PLAYER_SERVICE,LISTEN_YOUTUBE_REGION_UNAVAILABLE_ERROR_CODE,LISTEN_YOUTUBE_VERIFICATION_REQUIRED_ERROR_CODE } from "@/app/main/listen/catalog";
import { listenArtistBrowseTrack } from "@/app/main/listen/artist-navigation";
import { callListenLyricsCandidate,callListenLyricsForTrackCached,callListenTrackLyricsCached,resolveListenLyricsOnlineArtist } from "@/app/main/listen/lyrics-api";
import { ListenLyricsControls } from "@/app/main/listen/lyrics-controls";
import { resolveListenLyricsErrorPresentation } from "@/app/main/listen/lyrics-errors";
import { normalizeListenLyricsPlaybackRate } from "@/app/main/listen/lyrics-clock";
import { readListenLyricsManualOverride,type ListenLyricsManualOverride } from "@/app/main/listen/lyrics-preferences";
import { useListenTrackLyricsPrefetch } from "@/app/main/listen/lyrics-prefetch";
import { ListenLyricsSurface } from "@/app/main/listen/lyrics";
import { ListenLyricsWorkspace } from "@/app/main/listen/lyrics-workspace";
import {
  createListenNativeVideoSequence,
  LISTEN_LIVE_VIDEO_ASPECT_RATIO,
  LISTEN_LIVE_VIDEO_EMBED_SETTLE_MS,
  LISTEN_LIVE_VIDEO_FRAME_GAP,
  LISTEN_LIVE_VIDEO_MIN_WINDOW_HEIGHT,
  LISTEN_LIVE_VIDEO_MIN_WINDOW_WIDTH,
  LISTEN_LIVE_VIDEO_TOPBAR_HEIGHT,
  ListenInlineVideoSurface,
  ListenLiveVideoShell,
  type ListenNativeVideoRect,
} from "@/app/main/listen/native-video-surfaces";
import { copyListenTextToClipboard,forgetListenLyricsCache,forgetListenLyricsCacheVariants,getListenErrorCode,getListenErrorMessage,isListenLiveEventForSession,isListenLyricsDataAvailable,listenLyricsSummary,LISTEN_EMPTY_PROGRESS,LISTEN_INLINE_VIDEO_FALLBACK_ASPECT_RATIO,logListenLyrics,normalizeListenLiveNativeState,readListenLyricsCache,readListenNativeEventURLVideoId,resolveListenLyricsCurrentState,resolveListenNativeEventVideoAspectRatio,resolveListenPlaybackActivity,resolveListenPlaybackStatusLabel,resolveListenTrackVideoAvailability,resolveTrustedListenOnlineArtistLabel,splitListenArtistLabel,type ListenArtistLabelPart,type ListenLyricsTrackRequest,type ListenVideoAvailability } from "@/app/main/listen/playback-helpers";
import { useListenRadioFullscreenVideoDefault } from "@/app/main/listen/radio-fullscreen-video";
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
import { clampVolume } from "@/app/main/listen/local-library";
import { ListenPlayerProgress } from "@/app/main/listen/player-progress";
import {
  ListenPlayerTransport,
  ListenPlayerVolume,
  ListenScrollingText,
  ListenSubtitleText,
  ListenTrackInfoRow,
} from "@/app/main/listen/playback-controls";
import { buildListenPosterCandidates,buildYouTubeWatchURL } from "@/app/main/listen/storage";
import type { ListenExternalCommand,ListenLocalItem,ListenLyricsData,ListenLyricsKind,ListenMode,ListenNativePlayerEvent,ListenObservedPlaybackAudioQuality,ListenOnlineItem,ListenPlaybackSource,ListenPlayMode,ListenPlayerCommand,ListenPlayerCompanionMode,ListenPlayerPresentation,ListenRemotePlaybackState,ListenTrackArtist } from "@/app/main/listen/types";
import { ListenOnlineArtwork } from "@/app/main/listen/ui";
import {
  ListenWorkspaceLocalQueueCompanion,
  ListenWorkspaceLyricsCompanion,
  ListenWorkspaceOnlineQueueCompanion,
  ListenWorkspaceQueueModeSwitch,
} from "@/app/main/listen/workspace-companion";
import {
  ListenPlayerSourceBadge,
  ListenWorkspaceFullscreenBackdrop,
  listenPlaybackSourceFromMode,
  resolveListenPlayerSourceLabel,
  resolveListenFullscreenQualityLabel,
} from "@/app/main/listen/workspace-player-shared";
import {
  isEqualizerArtworkVisualizerMode,
  isEqualizerSpectrumVisualizerMode,
  type EqualizerVisualizerMode,
} from "@/shared/contracts/equalizer";
import { useEqualizerSnapshot,useEqualizerVisualizerFrame } from "@/shared/query/equalizer";
import {
  playbackSessionByID,
  usePlaybackCoordinator,
  type PlaybackSessionRequest,
  type PlaybackSnapshot,
} from "@/shared/playback";

function listenLyricsMatchesManualOverride(
  lyrics: ListenLyricsData | null,
  override: ListenLyricsManualOverride,
) {
  return Boolean(
    lyrics &&
      lyrics.providerId?.trim().toLowerCase() === override.providerId &&
      lyrics.providerTrackId?.trim() === override.providerTrackId,
  );
}

const LISTEN_MEDIA_SPLIT_MIN_WIDTH = 760;

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
  presentation: ListenPlayerPresentation;
  workspaceFullscreen?: boolean;
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
  onOpenPlaybackSource?: (source: ListenPlaybackSource) => void;
  onRequestPlayerFullscreen?: () => void;
}) {
  const noop = React.useCallback(() => {}, []);
  const playbackSource = listenPlaybackSourceFromMode(props.mode);
  return (
    <div className="relative h-full min-h-0 overflow-hidden">
      <ListenPlayerChrome
        mediaMode="cover"
        presentation={props.presentation}
        workspaceFullscreen={props.workspaceFullscreen}
        backdropCandidates={[LISTEN_DEFAULT_COVER_IMAGE_URL]}
        reserveWindowControls={props.reserveWindowControls}
        airPlaySupported={false}
        sourceBadge={<ListenPlayerSourceBadge source={playbackSource} text={props.text} />}
        sourceLabel={resolveListenPlayerSourceLabel(playbackSource, props.text)}
        onOpenSource={props.onOpenPlaybackSource ? () => props.onOpenPlaybackSource?.(playbackSource) : undefined}
        onRequestFullscreen={props.onRequestPlayerFullscreen}
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
        fullscreenLive={
          props.presentation === "page" &&
          props.mode === "hush" &&
          !props.listOpen
        }
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
  presentation: ListenPlayerPresentation;
  companionMode?: ListenPlayerCompanionMode;
  workspaceFullscreen?: boolean;
  presentationCommand?: ListenExternalCommand | null;
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
  onlinePlaybackErrorCode?: string;
  onlinePlaybackErrorMessage?: string;
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
  onOnlinePlaybackErrorCodeChange?: (code: string) => void;
  onOnlinePlaybackErrorMessageChange?: (message: string) => void;
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
  onClearLocalQueue: () => void;
  onRemoveLocalQueueItem: (item: ListenLocalItem) => void;
  onMoveLocalQueueItem: (item: ListenLocalItem, direction: -1 | 1) => void;
  onUndoLocalQueueEdit: () => void;
  onRedoLocalQueueEdit: () => void;
  localQueueCanUndo: boolean;
  localQueueCanRedo: boolean;
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
  onOpenPlaybackSource?: (source: ListenPlaybackSource) => void;
  onRequestPlayerFullscreen?: () => void;
  onExitPlayerFullscreen?: () => void;
}) {
  const playbackCoordinatorState = usePlaybackCoordinator();
  const handledLocalCommandRef = React.useRef(0);
  const pendingLocalStartRef = React.useRef<{
    sessionID: string;
    promise: Promise<PlaybackSnapshot>;
  } | null>(null);
  const handledLocalEndedSessionRef = React.useRef("");
  const localTrack = props.selectedLocal;
  const localSessionID = localTrack ? `music-local:${localTrack.id}` : "";
  const localPlaybackSession = localSessionID
    ? playbackSessionByID(playbackCoordinatorState.snapshot, localSessionID)
    : null;
  const localPlaybackState: ListenRemotePlaybackState =
    localPlaybackSession && localPlaybackSession.item.id === localTrack?.id
      ? localPlaybackSession.state
      : props.localPlaying
        ? "loading"
        : "paused";
  const localPlaybackActivity = resolveListenPlaybackActivity(localPlaybackState);
  const localTimelineRunning = localPlaybackActivity.timelineRunning;
  const localTransportLoading = localPlaybackActivity.loading;
  const [localMediaMode, setLocalMediaMode] =
    React.useState<ListenMediaMode>("cover");
  const [localQueueOpen, setLocalQueueOpen] = React.useState(false);
  const [localQueueAnchor, setLocalQueueAnchor] =
    React.useState<ListenQueuePopupAnchor | null>(null);
  const handledLocalPresentationCommandRef = React.useRef(0);
  const [localLyricsState, setLocalLyricsState] = React.useState<{
    lyricsId: string;
    loading: boolean;
    data: ListenLyricsData | null;
    error: string;
    errorCode?: string;
    errorRetryable?: boolean;
  }>({
    lyricsId: "",
    loading: false,
    data: null,
    error: "",
  });
  const localFullscreenLyricsDefaultKeyRef = React.useRef("");
  const localLyricsRetryKeyRef = React.useRef("");
  const localLyricsRequestGenerationRef = React.useRef(0);
  const [localLyricsRetryToken, setLocalLyricsRetryToken] = React.useState(0);

  React.useEffect(() => {
    const command = props.presentationCommand;
    if (!command || handledLocalPresentationCommandRef.current === command.id) {
      return;
    }
    handledLocalPresentationCommandRef.current = command.id;
    if (
      command.command === "show-lyrics" &&
      props.presentation !== "companion"
    ) {
      setLocalQueueOpen(false);
      setLocalMediaMode("lyrics");
    } else if (
      command.command === "show-queue" &&
      props.presentation !== "companion"
    ) {
      setLocalQueueOpen(true);
    }
  }, [props.presentation, props.presentationCommand]);
  React.useLayoutEffect(() => {
    if (!props.companionMode) {
      return;
    }
    setLocalQueueOpen(false);
    setLocalMediaMode("cover");
  }, [props.companionMode]);
  // Normal playback always prefers timed lyrics; providers fall back to plain
  // text only when no acceptable synchronized version exists.
  const syncedLyricsEnabled = true;
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
  const romanizedLyrics = romanizedLyricsSetting;
  const pinyinLyrics = pinyinLyricsSetting;

  const localLyricsTrack = React.useMemo<ListenLyricsTrackRequest | null>(() => {
    if (!localTrack) {
      return null;
    }
    return {
      lyricsId: `local:${localTrack.id}`,
      title: localTrack.lyricsTitle || localTrack.title,
      artist: localTrack.lyricsArtist || localTrack.author,
      album: localTrack.album,
      localPath: localTrack.path,
      durationLabel: localTrack.durationLabel,
    };
  }, [
    localTrack?.author,
    localTrack?.album,
    localTrack?.durationLabel,
    localTrack?.id,
    localTrack?.lyricsArtist,
    localTrack?.lyricsTitle,
    localTrack?.path,
    localTrack?.title,
  ]);
  const localLyricsWorkspaceTrack = React.useMemo(() => {
    if (!localLyricsTrack) {
      return null;
    }
    return {
      lyricsId: localLyricsTrack.lyricsId,
      title: localLyricsTrack.title,
      artist: localLyricsTrack.artist,
      album: localLyricsTrack.album,
      localPath: localLyricsTrack.localPath,
      durationSeconds: props.localProgress.duration,
    };
  }, [localLyricsTrack, props.localProgress.duration]);
  const localLyricsCurrentState = resolveListenLyricsCurrentState(
    localLyricsState,
    localLyricsState.lyricsId,
    String(localLyricsTrack?.lyricsId ?? ""),
  );
  const localLyricsAvailable = isListenLyricsDataAvailable(
    localLyricsCurrentState.data,
  );
  const retryLocalLyrics = React.useCallback(() => {
    const lyricsId = String(localLyricsTrack?.lyricsId || "").trim();
    if (!lyricsId) {
      return;
    }
    localLyricsRequestGenerationRef.current += 1;
    localLyricsRetryKeyRef.current = lyricsId;
    forgetListenLyricsCache(lyricsId, props.text.locale, {
      synced: syncedLyricsEnabled,
    });
    setLocalLyricsRetryToken((value) => value + 1);
  }, [localLyricsTrack?.lyricsId, props.text.locale, syncedLyricsEnabled]);

  const handleLocalLyricsChange = React.useCallback((data: ListenLyricsData) => {
    const lyricsId = String(localLyricsWorkspaceTrack?.lyricsId ?? "").trim();
    if (!lyricsId) {
      return;
    }
    localLyricsRequestGenerationRef.current += 1;
    forgetListenLyricsCache(lyricsId, props.text.locale, {
      synced: syncedLyricsEnabled,
    });
    setLocalLyricsState({
      lyricsId,
      loading: false,
      data,
      error: "",
    });
  }, [
    localLyricsWorkspaceTrack?.lyricsId,
    props.text.locale,
    syncedLyricsEnabled,
  ]);

  const handleLocalLyricsRestoreAutomatic = React.useCallback(async () => {
    if (!localLyricsWorkspaceTrack) {
      return;
    }
    forgetListenLyricsCacheVariants(
      String(localLyricsWorkspaceTrack.lyricsId ?? ""),
    );
    retryLocalLyrics();
  }, [
    localLyricsWorkspaceTrack,
    retryLocalLyrics,
  ]);

  const buildLocalSessionRequest = React.useCallback(
    (options: { startSeconds?: number; forceReload?: boolean } = {}): PlaybackSessionRequest | null => {
      if (!localTrack || !localSessionID) {
        return null;
      }
      const uri = localTrack.path.trim() || localTrack.previewURL.trim();
      if (!uri) {
        return null;
      }
      return {
        sessionId: localSessionID,
        item: {
          id: localTrack.id,
          kind: "audio",
          source: { provider: "local", uri },
          title: localTrack.title,
          artist: localTrack.author,
          artworkUrl: localTrack.coverURL,
        },
        startSeconds: Math.max(0, options.startSeconds ?? props.localResumeTime),
        volume: clampVolume(props.volume),
        muted: props.muted || props.volume <= 0,
        forceReload: options.forceReload,
      };
    }, [
      localSessionID,
      localTrack,
      props.localResumeTime,
      props.muted,
      props.volume,
    ],
  );

  const runLocalPlayerCommand = React.useCallback(
    async (command: ListenPlayerCommand) => {
      if (props.mode !== "linger" || !localTrack || !localSessionID) {
        return;
      }
      const activeSession = playbackCoordinatorState.snapshot.active;
      const session = playbackSessionByID(
        playbackCoordinatorState.snapshot,
        localSessionID,
      );
      const isActive = activeSession?.id === localSessionID;
      if (command.command === "pause") {
        if (isActive && session?.capabilities.playPause) {
          await playbackCoordinatorState.commands.pause();
          return;
        }
        const pendingStart = pendingLocalStartRef.current;
        if (!activeSession && pendingStart?.sessionID === localSessionID) {
          const started = await pendingStart.promise.catch(() => null);
          if (
            started?.active?.id === localSessionID &&
            started.active.capabilities.playPause
          ) {
            await playbackCoordinatorState.commands.pause();
          }
        }
        return;
      }
      if (command.command === "seek" && isActive && session?.capabilities.seek) {
        await playbackCoordinatorState.commands.seek(
          Math.max(0, command.startSeconds ?? 0),
        );
        return;
      }
      if (
        (command.command === "play" || command.command === "resume") &&
        isActive &&
        session?.state !== "ended" &&
        session?.capabilities.playPause
      ) {
        await playbackCoordinatorState.commands.play();
        return;
      }
      const request = buildLocalSessionRequest({
        startSeconds:
          command.command === "replay"
            ? 0
            : command.command === "seek"
              ? command.startSeconds
              : undefined,
        forceReload: command.command === "replay" || command.forceReload,
      });
      if (request) {
        const pendingStart = {
          sessionID: localSessionID,
          promise: playbackCoordinatorState.commands.startPersistent(request),
        };
        pendingLocalStartRef.current = pendingStart;
        try {
          await pendingStart.promise;
        } finally {
          if (pendingLocalStartRef.current === pendingStart) {
            pendingLocalStartRef.current = null;
          }
        }
      }
    },
    [
      buildLocalSessionRequest,
      localSessionID,
      localTrack,
      playbackCoordinatorState.commands,
      playbackCoordinatorState.snapshot,
      props.mode,
    ],
  );

  const handleLocalTogglePlayback = React.useCallback<
    React.MouseEventHandler<HTMLButtonElement>
  >(() => {
    props.onLocalPlaybackIntent();
    void runLocalPlayerCommand({
      id: Date.now(),
      command: localPlaybackActivity.transportActive ? "pause" : "play",
    }).catch(() => {});
  }, [
    localPlaybackActivity.transportActive,
    props.onLocalPlaybackIntent,
    runLocalPlayerCommand,
  ]);

  const handleLocalSeek = React.useCallback(
    (seconds: number) => {
      if (props.mode !== "linger" || !localTrack) {
        return;
      }
      const duration = Math.max(0, props.localProgress.duration);
      const nextTime = duration > 0
        ? Math.max(0, Math.min(seconds, duration))
        : Math.max(0, seconds);
      props.onLocalProgressChange(
        nextTime,
        duration,
        Math.max(props.localProgress.bufferedTime, nextTime),
      );
      void runLocalPlayerCommand({
        id: Date.now(),
        command: "seek",
        startSeconds: nextTime,
      }).catch(() => {});
    }, [
      localTrack,
      props.localProgress.bufferedTime,
      props.localProgress.duration,
      props.mode,
      props.onLocalProgressChange,
      runLocalPlayerCommand,
    ],
  );

  const handleLocalPrevious = React.useCallback(() => {
    const active = playbackCoordinatorState.snapshot.active;
    if (active?.id === localSessionID && active.capabilities.previous) {
      void playbackCoordinatorState.commands.previous().catch(() => props.onPrevious());
      return;
    }
    props.onPrevious();
  }, [
    localSessionID,
    playbackCoordinatorState.commands,
    playbackCoordinatorState.snapshot.active,
    props.onPrevious,
  ]);

  const handleLocalNext = React.useCallback(() => {
    const active = playbackCoordinatorState.snapshot.active;
    if (active?.id === localSessionID && active.capabilities.next) {
      void playbackCoordinatorState.commands.next().catch(() => props.onNext());
      return;
    }
    props.onNext();
  }, [
    localSessionID,
    playbackCoordinatorState.commands,
    playbackCoordinatorState.snapshot.active,
    props.onNext,
  ]);

  React.useEffect(() => {
    const command = props.localCommand;
    if (
      props.mode !== "linger" ||
      !command ||
      handledLocalCommandRef.current === command.id
    ) {
      return;
    }
    handledLocalCommandRef.current = command.id;
    void runLocalPlayerCommand(command).catch(() => {});
  }, [props.localCommand, props.mode, runLocalPlayerCommand]);

  React.useEffect(() => {
    const active = playbackCoordinatorState.snapshot.active;
    if (
      props.mode === "linger" ||
      !active?.id.startsWith("music-local:") ||
      !active.capabilities.playPause ||
      active.state === "paused" ||
      active.state === "ended"
    ) {
      return;
    }
    // Changing the Music source is a playback handoff, unlike navigating to
    // another workspace. Pause the backend-owned local session before the
    // legacy online source starts so there can never be two audible engines.
    void playbackCoordinatorState.commands.pause().catch(() => {});
  }, [
    playbackCoordinatorState.commands,
    playbackCoordinatorState.snapshot.active,
    props.mode,
  ]);

  React.useEffect(() => {
    if (props.mode !== "linger" || !localTrack || !localSessionID) {
      return;
    }
    const session = playbackSessionByID(
      playbackCoordinatorState.snapshot,
      localSessionID,
    );
    if (!session || session.item.id !== localTrack.id) {
      return;
    }
    const duration = Math.max(session.duration, session.item.duration ?? 0);
    props.onLocalProgressChange(
      session.position,
      duration,
      Math.max(session.position, Math.min(props.localProgress.bufferedTime, duration)),
    );
    props.onLocalPlayingChange(
      session.state === "playing" || session.state === "buffering",
    );
    if (session.state !== "ended") {
      handledLocalEndedSessionRef.current = "";
    }
    if (
      session.state === "ended" &&
      playbackCoordinatorState.snapshot.active?.id === localSessionID &&
      handledLocalEndedSessionRef.current !== localSessionID
    ) {
      handledLocalEndedSessionRef.current = localSessionID;
      props.onEnded();
    }
  }, [
    localSessionID,
    localTrack,
    playbackCoordinatorState.snapshot,
    props.localProgress.bufferedTime,
    props.mode,
    props.onEnded,
    props.onLocalPlayingChange,
    props.onLocalProgressChange,
  ]);

  React.useEffect(() => {
    if (
      props.mode !== "linger" ||
      playbackCoordinatorState.snapshot.active?.id !== localSessionID
    ) {
      return;
    }
    const active = playbackCoordinatorState.snapshot.active;
    if (!active.capabilities.volume) {
      return;
    }
    const volume = clampVolume(props.volume);
    if (
      Math.abs(active.volume - volume) < 0.001 &&
      active.muted === (props.muted || volume <= 0)
    ) {
      return;
    }
    void playbackCoordinatorState.commands
      .setVolume(volume, props.muted || volume <= 0)
      .catch(() => {});
  }, [
    localSessionID,
    playbackCoordinatorState.commands,
    playbackCoordinatorState.snapshot.active,
    props.mode,
    props.muted,
    props.volume,
  ]);

  React.useEffect(() => {
    const requestGeneration = localLyricsRequestGenerationRef.current + 1;
    localLyricsRequestGenerationRef.current = requestGeneration;
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
    const manualTrack = {
      id: lyricsId,
      lyricsId,
      title: localLyricsTrack.title,
      artist: localLyricsTrack.artist,
      album: localLyricsTrack.album,
      localPath: localLyricsTrack.localPath,
      durationSeconds: props.localProgress.duration,
    };
    const manualOverride = readListenLyricsManualOverride(manualTrack);
    const storedLyrics = forceRequest ? null : readListenLyricsCache(lyricsId, props.text.locale, lyricsMode);
    const cachedLyrics = manualOverride && !listenLyricsMatchesManualOverride(storedLyrics, manualOverride)
      ? null
      : storedLyrics;
    const refreshCachedPlain = false;
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
    const lyricsRequest = manualOverride
      ? callListenLyricsCandidate({
          track: manualTrack,
          candidate: manualOverride,
          language: props.text.locale,
          synced: syncedLyricsEnabled,
        })
      : callListenLyricsForTrackCached({
          track: manualTrack,
          cacheID: lyricsId,
          durationSeconds: props.localProgress.duration,
          language: props.text.locale,
          synced: syncedLyricsEnabled,
        });
    void lyricsRequest
      .then((data) => {
        if (
          cancelled ||
          localLyricsRequestGenerationRef.current !== requestGeneration
        ) {
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
        if (
          cancelled ||
          localLyricsRequestGenerationRef.current !== requestGeneration
        ) {
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
        const presentation = resolveListenLyricsErrorPresentation(
          props.text,
          error,
        );
        setLocalLyricsState({
          lyricsId,
          loading: false,
          data: cachedLyrics,
          error: cachedLyrics
            ? ""
            : presentation.message,
          errorCode: cachedLyrics ? undefined : presentation.code,
          errorRetryable: cachedLyrics ? undefined : presentation.retryable,
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
    props.localProgress.duration,
    localTrack?.id,
    localLyricsRetryToken,
    props.mode,
    syncedLyricsEnabled,
  ]);

  React.useEffect(() => {
    if (
      props.presentation !== "fullscreen" ||
      props.mode !== "linger" ||
      props.listOpen ||
      localQueueOpen ||
      !localTrack
    ) {
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
    localQueueOpen,
    localTrack,
    props.listOpen,
    props.mode,
    props.presentation,
  ]);

  if (props.mode !== "linger") {
    const track = props.selectedOnline;
    if (!track) {
      if (props.companionMode === "queue") {
        return (
          <ListenWorkspaceOnlineQueueCompanion
            queueTitle={props.onlineQueueTitle}
            queueItems={props.onlineQueueItems}
            selectedQueueId={props.selectedOnlineId}
            httpBaseURL={props.httpBaseURL}
            playMode={props.playMode}
            text={props.text}
            onPlayModeChange={props.onPlayModeChange}
            onClearQueue={props.onClearOnlineQueue}
            onRemoveQueueItem={props.onRemoveOnlineQueueItem}
            onMoveQueueItem={props.onMoveOnlineQueueItem}
            onUndoQueueEdit={props.onUndoOnlineQueueEdit}
            onRedoQueueEdit={props.onRedoOnlineQueueEdit}
            queueCanUndo={props.onlineQueueCanUndo}
            queueCanRedo={props.onlineQueueCanRedo}
            onSelectQueueTrack={props.onSelectOnlineQueueTrack}
          />
        );
      }
      if (props.companionMode === "lyrics") {
        return (
          <ListenWorkspaceLyricsCompanion
            artworkCandidates={[LISTEN_DEFAULT_COVER_IMAGE_URL]}
            title={props.text.listen.nowPlaying}
            text={props.text}
          >
            <ListenLyricsSurface
              variant="companion"
              text={props.text}
              lyrics={null}
            />
          </ListenWorkspaceLyricsCompanion>
        );
      }
      return (
        <ListenEmptyPlaybackChrome
          mode={props.mode}
          presentation={props.presentation}
          workspaceFullscreen={props.workspaceFullscreen}
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
          onOpenPlaybackSource={props.onOpenPlaybackSource}
          onRequestPlayerFullscreen={props.onRequestPlayerFullscreen}
        />
      );
    }

    return (
      <ListenYouTubePlayback
        mode={props.mode}
        presentation={props.presentation}
        companionMode={props.companionMode}
        workspaceFullscreen={props.workspaceFullscreen}
        active={props.active}
        presentationCommand={props.presentationCommand}
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
        playbackErrorCode={props.onlinePlaybackErrorCode}
        playbackErrorMessage={props.onlinePlaybackErrorMessage}
        observedPlaybackAudioQuality={
          props.mode === "muse" ? (props.onlineObservedPlaybackAudioQuality ?? "") : undefined
        }
        pet={props.pet}
        petImageURL={props.petImageURL}
        text={props.text}
        onEnded={props.onEnded}
        onPlayingChange={props.onOnlinePlayingChange}
        onStateChange={props.onOnlineStateChange}
        onPlaybackErrorCodeChange={props.onOnlinePlaybackErrorCodeChange}
        onPlaybackErrorMessageChange={props.onOnlinePlaybackErrorMessageChange}
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
        onOpenPlaybackSource={props.onOpenPlaybackSource}
        onRequestPlayerFullscreen={props.onRequestPlayerFullscreen}
        onExitPlayerFullscreen={props.onExitPlayerFullscreen}
      />
    );
  }

  const track = localTrack;
  const renderLocalLyricsControls = (
    placement: "overlay" | "companion" | "fullscreen",
    controlsTrack: NonNullable<typeof localLyricsWorkspaceTrack>,
  ) => (
    <ListenLyricsControls
      key={`${controlsTrack.lyricsId ?? controlsTrack.title}:${placement}`}
      placement={placement}
      text={props.text}
      track={controlsTrack}
      lyrics={localLyricsCurrentState.data}
      currentTimeMs={Math.max(0, props.localProgress.currentTime * 1000)}
      timelineRunning={localTimelineRunning}
      language={props.text.locale}
      synced={syncedLyricsEnabled}
      romanized={romanizedLyrics}
      pinyin={pinyinLyrics}
      onLyricsChange={handleLocalLyricsChange}
      onRestoreAutomatic={handleLocalLyricsRestoreAutomatic}
    />
  );
  if (props.companionMode === "queue") {
    return (
      <ListenWorkspaceLocalQueueCompanion
        queueTitle={props.text.listen.upNext}
        queueItems={props.localQueueItems}
        selectedQueueId={props.selectedLocalId}
        playMode={props.playMode}
        text={props.text}
        onPlayModeChange={props.onPlayModeChange}
        onClearQueue={props.onClearLocalQueue}
        onRemoveQueueItem={props.onRemoveLocalQueueItem}
        onMoveQueueItem={props.onMoveLocalQueueItem}
        onUndoQueueEdit={props.onUndoLocalQueueEdit}
        onRedoQueueEdit={props.onRedoLocalQueueEdit}
        queueCanUndo={props.localQueueCanUndo}
        queueCanRedo={props.localQueueCanRedo}
        onSelectQueueTrack={props.onSelectLocalQueueTrack}
      />
    );
  }

  if (props.companionMode === "lyrics") {
    return (
      <ListenWorkspaceLyricsCompanion
        artworkCandidates={[
          track?.coverURL || LISTEN_DEFAULT_COVER_IMAGE_URL,
          LISTEN_DEFAULT_COVER_IMAGE_URL,
        ]}
        title={track?.title || props.text.listen.nowPlaying}
        text={props.text}
        lyricsControls={
          localLyricsWorkspaceTrack
            ? renderLocalLyricsControls("companion", localLyricsWorkspaceTrack)
            : undefined
        }
      >
        {localLyricsWorkspaceTrack ? (
          <ListenLyricsWorkspace
            variant="companion"
            surfaceActive={props.active && props.companionMode === "lyrics"}
            text={props.text}
            track={localLyricsWorkspaceTrack}
            current={{
              lyrics: localLyricsCurrentState.data,
              loading: localLyricsCurrentState.loading,
              error: localLyricsCurrentState.error,
              errorCode: localLyricsCurrentState.errorCode,
              errorRetryable: localLyricsCurrentState.errorRetryable,
              onRetry: retryLocalLyrics,
            }}
            currentTimeMs={Math.max(0, props.localProgress.currentTime * 1000)}
            timelineRunning={localTimelineRunning}
            romanized={romanizedLyrics}
            pinyin={pinyinLyrics}
            onSeek={track ? handleLocalSeek : undefined}
          />
        ) : (
          <ListenLyricsSurface
            variant="companion"
            text={props.text}
            lyrics={null}
          />
        )}
      </ListenWorkspaceLyricsCompanion>
    );
  }

  if (!track) {
    return (
      <ListenEmptyPlaybackChrome
        mode="linger"
        presentation={props.presentation}
        workspaceFullscreen={props.workspaceFullscreen}
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
        onOpenPlaybackSource={props.onOpenPlaybackSource}
        onRequestPlayerFullscreen={props.onRequestPlayerFullscreen}
      />
    );
  }

  const activeLocalLyricsWorkspaceTrack = localLyricsWorkspaceTrack ?? {
    lyricsId: `local:${track.id}`,
    title: track.lyricsTitle || track.title,
    artist: track.lyricsArtist || track.author,
    album: track.album,
    localPath: track.path,
    durationSeconds: props.localProgress.duration,
  };
  const localLyricsControls = localMediaMode === "lyrics"
    ? renderLocalLyricsControls(
        props.presentation === "page"
          ? "overlay"
          : props.presentation === "fullscreen"
            ? "fullscreen"
            : "companion",
        activeLocalLyricsWorkspaceTrack,
      )
    : null;

  return (
    <div className={cn(
      "relative h-full min-h-0",
      localArtworkVisualizerVisible ? "overflow-visible" : "overflow-hidden",
    )}>
      <ListenPlayerChrome
        mediaMode={localMediaMode}
        presentation={props.presentation}
        workspaceFullscreen={props.workspaceFullscreen}
        backdropCandidates={[
          track.coverURL || LISTEN_DEFAULT_COVER_IMAGE_URL,
          LISTEN_DEFAULT_COVER_IMAGE_URL,
        ]}
        reserveWindowControls={props.reserveWindowControls}
        airPlaySupported={false}
        sourceBadge={<ListenPlayerSourceBadge source="local" text={props.text} />}
        sourceLabel={resolveListenPlayerSourceLabel("local", props.text)}
        onOpenSource={props.onOpenPlaybackSource ? () => props.onOpenPlaybackSource?.("local") : undefined}
        onRequestFullscreen={props.onRequestPlayerFullscreen}
        onExitFullscreen={props.onExitPlayerFullscreen}
        lyricsControls={
          props.presentation === "page" ? undefined : localLyricsControls
        }
        queueControls={
          <ListenWorkspaceQueueModeSwitch
            playMode={props.playMode}
            text={props.text}
            onChange={props.onPlayModeChange}
          />
        }
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
                  active={localTimelineRunning}
                  onVisibleChange={handleLocalArtworkVisualizerVisibleChange}
                />
              ) : null
            }
          />
        }
        lyrics={
          <ListenLyricsWorkspace
            variant={props.presentation === "companion" || props.workspaceFullscreen ? "companion" : "player"}
            surfaceActive={props.active && localMediaMode === "lyrics"}
            text={props.text}
            track={activeLocalLyricsWorkspaceTrack}
            current={{
              lyrics: localLyricsCurrentState.data,
              loading: localLyricsCurrentState.loading,
              error: localLyricsCurrentState.error,
              errorCode: localLyricsCurrentState.errorCode,
              errorRetryable: localLyricsCurrentState.errorRetryable,
              onRetry: retryLocalLyrics,
            }}
            currentTimeMs={Math.max(0, props.localProgress.currentTime * 1000)}
            timelineRunning={localTimelineRunning}
            romanized={romanizedLyrics}
            pinyin={pinyinLyrics}
            controls={
              props.presentation === "page" ? localLyricsControls : undefined
            }
            onSeek={handleLocalSeek}
          />
        }
        hasVideo={false}
        videoHidden
        lyricsAvailable
        lyricsKind={localLyricsCurrentState.data?.kind}
        lyricsLoading={!localLyricsAvailable && localLyricsCurrentState.loading}
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
        progressLoading={localTransportLoading}
        onSeek={handleLocalSeek}
        playing={props.localPlaying}
        loading={localTransportLoading}
        playbackState={localPlaybackState}
        muted={props.muted}
        volume={props.volume}
        playMode={props.playMode}
        text={props.text}
        onMediaModeChange={(mode) => {
          setLocalQueueOpen(false);
          setLocalMediaMode(mode);
        }}
        onPrevious={handleLocalPrevious}
        onNext={handleLocalNext}
        onPlayModeChange={props.onPlayModeChange}
        onTogglePlayback={handleLocalTogglePlayback}
        onToggleMute={props.onToggleMute}
        onVolumeChange={props.onVolumeChange}
        onToggleQueue={(anchor) => {
          setLocalQueueAnchor(anchor);
          setLocalQueueOpen((current) => {
            const next = !current;
            if (next) {
              setLocalMediaMode("cover");
            }
            return next;
          });
        }}
        visualizerMode={visualizerMode}
        visualizerEnabled={visualizerEnabled}
        visualizerActive={localTimelineRunning}
        queueOpen={localQueueOpen}
        workspaceQueue={
          <ListenWorkspaceLocalQueueCompanion
            queueTitle={props.text.listen.upNext}
            queueItems={props.localQueueItems}
            selectedQueueId={props.selectedLocalId}
            playMode={props.playMode}
            text={props.text}
            onPlayModeChange={props.onPlayModeChange}
            onClearQueue={props.onClearLocalQueue}
            onRemoveQueueItem={props.onRemoveLocalQueueItem}
            onMoveQueueItem={props.onMoveLocalQueueItem}
            onUndoQueueEdit={props.onUndoLocalQueueEdit}
            onRedoQueueEdit={props.onRedoLocalQueueEdit}
            queueCanUndo={props.localQueueCanUndo}
            queueCanRedo={props.localQueueCanRedo}
            showFooter={props.presentation !== "companion"}
            onSelectQueueTrack={props.onSelectLocalQueueTrack}
          />
        }
        queueOverlay={
          localQueueOpen ? (
            <ListenLocalPlaybackQueuePopup
              anchor={localQueueAnchor}
              queueTitle={props.text.listen.upNext}
              queueItems={props.localQueueItems}
              selectedQueueId={props.selectedLocalId}
              text={props.text}
              onClearQueue={props.onClearLocalQueue}
              onRemoveQueueItem={props.onRemoveLocalQueueItem}
              onMoveQueueItem={props.onMoveLocalQueueItem}
              onUndoQueueEdit={props.onUndoLocalQueueEdit}
              onRedoQueueEdit={props.onRedoLocalQueueEdit}
              queueCanUndo={props.localQueueCanUndo}
              queueCanRedo={props.localQueueCanRedo}
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
  presentation: ListenPlayerPresentation;
  companionMode?: ListenPlayerCompanionMode;
  workspaceFullscreen?: boolean;
  presentationCommand?: ListenExternalCommand | null;
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
  playbackErrorCode?: string;
  playbackErrorMessage?: string;
  observedPlaybackAudioQuality?: ListenObservedPlaybackAudioQuality | "";
  pet: Pet | null;
  petImageURL: string;
  text: ReturnType<typeof getXiaText>;
  onEnded: () => void;
  onPlayingChange: (playing: boolean) => void;
  onStateChange: (state: ListenRemotePlaybackState) => void;
  onPlaybackErrorCodeChange?: (code: string) => void;
  onPlaybackErrorMessageChange?: (message: string) => void;
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
  onOpenPlaybackSource?: (source: ListenPlaybackSource) => void;
  onRequestPlayerFullscreen?: () => void;
  onExitPlayerFullscreen?: () => void;
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
  useListenTrackLyricsPrefetch({ enabled: props.active && !isLive, track: props.track, durationSeconds: props.progress.duration, language: props.text.locale, synced: props.syncedLyricsEnabled });
  const livePlaybackSessionIDRef = React.useRef("");
  React.useEffect(() => {
    livePlaybackSessionIDRef.current = "";
  }, [isLive, props.track.videoId]);
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
      const artistTrack = listenArtistBrowseTrack(
        props.track,
        { name: artist },
        artistLabelParts,
      );
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
  const playbackSource = listenPlaybackSourceFromMode(props.mode);
  const artworkRevisionKey = [
    props.track.id,
    props.track.videoId,
    props.track.thumbnailUrl?.trim() ?? "",
  ].join(":");
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
  const handledPresentationCommandRef = React.useRef(0);
  const [lyricsState, setLyricsState] = React.useState<{
    videoId: string;
    loading: boolean;
    data: ListenLyricsData | null;
    error: string;
    errorCode?: string;
    errorRetryable?: boolean;
  }>({
    videoId: props.track.videoId,
    loading: false,
    data: null,
    error: "",
	  });
	  const onlineLyricsCurrentState = resolveListenLyricsCurrentState(
	    lyricsState,
	    lyricsState.videoId,
	    props.track.videoId,
	  );
	  const lyricsRetryKeyRef = React.useRef("");
	  const lyricsRequestGenerationRef = React.useRef(0);
	  const [lyricsRetryToken, setLyricsRetryToken] = React.useState(0);
  const [lyricsCurrentTimeMs, setLyricsCurrentTimeMs] = React.useState(() =>
	    Math.max(0, props.progress.currentTime * 1000),
	  );
  const [lyricsPlaybackRate, setLyricsPlaybackRate] = React.useState(1);
  const onlineLyricsWorkspaceTrack = React.useMemo(() => ({
    videoId: props.track.videoId,
    title: props.track.title,
    artist: resolveListenLyricsOnlineArtist(props.track),
    durationSeconds: props.progress.duration,
  }), [
    props.progress.duration,
    props.track.artists,
    props.track.channel,
    props.track.title,
    props.track.videoId,
  ]);
  const hasPlayableBuffer =
    props.state === "playing" ||
    (isLive && props.playing && props.state === "buffering") ||
    props.progress.currentTime > 0.15 ||
    props.progress.bufferedTime > 0.15;
  const playbackActivity = resolveListenPlaybackActivity(props.state);
  const transportLoading = props.enabled && playbackActivity.loading;
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
	const [embeddedVideoFullscreen, setEmbeddedVideoFullscreen] =
		React.useState(false);
	const [embeddedVideoFullscreenPending, setEmbeddedVideoFullscreenPending] =
		React.useState(false);
	const embeddedVideoGeometrySuspended =
		embeddedVideoFullscreen || embeddedVideoFullscreenPending;
  const inlineNativeVideoRectRef = React.useRef<ListenNativeVideoRect | null>(null);
  const inlineNativeVideoRequestRef = React.useRef(0);
  const liveFullscreenActive =
    props.active &&
    props.presentation === "page" &&
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
    props.presentation === "fullscreen" &&
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
  const inlineVideoVisible = inlineVideoRevealReady && inlineNativeVideoShown;
  useListenRadioFullscreenVideoDefault({
    presentation: props.presentation,
    workspaceFullscreen: props.workspaceFullscreen === true,
    active: props.active,
    enabled: props.enabled,
    live: isLive,
    trackKey: `${props.track.id}:${trackVideoId}`,
    hasVideo,
    nativeVideoAvailable: inlineNativeVideoAvailable,
    queueOpen,
    mediaMode,
    setQueueOpen,
    setMediaMode,
  });
  const retryLyrics = React.useCallback(() => {
    const videoId = props.track.videoId.trim();
    if (!videoId) {
      return;
    }
    lyricsRequestGenerationRef.current += 1;
    lyricsRetryKeyRef.current = videoId;
    forgetListenLyricsCache(videoId, props.text.locale, {
      synced: props.syncedLyricsEnabled,
    });
    setLyricsRetryToken((value) => value + 1);
  }, [
    props.syncedLyricsEnabled,
    props.text.locale,
    props.track.videoId,
  ]);

  const handleOnlineLyricsChange = React.useCallback((data: ListenLyricsData) => {
    const videoId = props.track.videoId.trim();
    if (!videoId) {
      return;
    }
    lyricsRequestGenerationRef.current += 1;
    forgetListenLyricsCache(videoId, props.text.locale, {
      synced: props.syncedLyricsEnabled,
    });
    setLyricsState({
      videoId,
      loading: false,
      data,
      error: "",
    });
  }, [
    props.syncedLyricsEnabled,
    props.text.locale,
    props.track.videoId,
  ]);

  const handleOnlineLyricsRestoreAutomatic = React.useCallback(async () => {
    forgetListenLyricsCacheVariants(onlineLyricsWorkspaceTrack.videoId);
    retryLyrics();
  }, [
    onlineLyricsWorkspaceTrack,
    retryLyrics,
  ]);

  React.useEffect(() => {
    resumeRef.current = props.resumeSeconds;
  }, [props.resumeSeconds, props.track.videoId]);

  React.useEffect(() => {
    if (onlineLyricsCurrentState.data?.kind === "synced") {
      return;
    }
    setLyricsCurrentTimeMs(Math.max(0, props.progress.currentTime * 1000));
  }, [
    onlineLyricsCurrentState.data?.kind,
    props.progress.currentTime,
    props.track.videoId,
  ]);

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
    props.onPlaybackErrorCodeChange?.("");
    props.onPlaybackErrorMessageChange?.("");
    setLyricsPlaybackRate(1);
    setLyricsState({
      videoId: props.track.videoId,
      loading: false,
      data: null,
      error: "",
    });
    if (isLive) {
      setMediaMode("cover");
    }
  }, [
    isLive,
    props.onPlaybackErrorCodeChange,
    props.onPlaybackErrorMessageChange,
    props.track.videoId,
  ]);

	React.useEffect(() => {
		setEmbeddedVideoFullscreen(false);
		setEmbeddedVideoFullscreenPending(false);
	}, [isLive]);

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
    const requestGeneration = lyricsRequestGenerationRef.current + 1;
    lyricsRequestGenerationRef.current = requestGeneration;
    if (
      mediaMode !== "lyrics" &&
      props.companionMode !== "lyrics"
    ) {
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
    const manualTrack = {
      videoId,
      title: props.track.title,
      artist: props.track.channel,
      durationSeconds: props.progress.duration,
    };
    const manualOverride = readListenLyricsManualOverride(manualTrack);
    const storedLyrics = forceRequest ? null : readListenLyricsCache(videoId, props.text.locale, lyricsMode);
    const cachedLyrics = manualOverride && !listenLyricsMatchesManualOverride(storedLyrics, manualOverride)
      ? null
      : storedLyrics;
    const refreshCachedPlain = false;
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
    const lyricsRequest = manualOverride
      ? callListenLyricsCandidate({
          track: manualTrack,
          candidate: manualOverride,
          language: props.text.locale,
          synced: props.syncedLyricsEnabled,
        })
      : callListenTrackLyricsCached({
          track: props.track,
          durationSeconds: props.progress.duration,
          language: props.text.locale,
          synced: props.syncedLyricsEnabled,
        });
    void lyricsRequest
      .then((data) => {
        if (
          cancelled ||
          lyricsRequestGenerationRef.current !== requestGeneration
        ) {
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
        if (
          cancelled ||
          lyricsRequestGenerationRef.current !== requestGeneration
        ) {
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
        const presentation = resolveListenLyricsErrorPresentation(
          props.text,
          error,
        );
        setLyricsState({
          videoId,
          loading: false,
          data: cachedLyrics,
          error: cachedLyrics
            ? ""
            : presentation.message,
          errorCode: cachedLyrics ? undefined : presentation.code,
          errorRetryable: cachedLyrics ? undefined : presentation.retryable,
        });
      });
    return () => {
      cancelled = true;
    };
  }, [
    isLive,
    mediaMode,
    props.companionMode,
    props.text.listen.lyricsEmpty,
    props.text.locale,
    props.track.channel,
    props.track.durationLabel,
    props.progress.duration,
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

  React.useEffect(() => {
    const command = props.presentationCommand;
    if (!command || handledPresentationCommandRef.current === command.id) {
      return;
    }
    handledPresentationCommandRef.current = command.id;
    if (
      command.command === "show-lyrics" &&
      !isLive &&
      props.presentation !== "companion"
    ) {
      setQueueOpen(false);
      setMediaMode("lyrics");
    } else if (
      command.command === "show-queue" &&
      props.presentation !== "companion"
    ) {
      setQueueOpen(true);
    } else if (
      command.command === "show-video" &&
      props.presentation === "fullscreen"
    ) {
      setQueueOpen(false);
      setMediaMode("video");
    } else if (command.command === "open-artist" && !isLive) {
      const artistTrack = command.artist
        ? listenArtistBrowseTrack(props.track, command.artist, artistLabelParts)
        : props.track;
      props.onOpenArtist(artistTrack ?? props.track);
    }
  }, [
    artistLabelParts,
    isLive,
    props.onOpenArtist,
    props.presentation,
    props.presentationCommand,
    props.track,
  ]);
  React.useLayoutEffect(() => {
    if (!props.companionMode) {
      return;
    }
    setQueueOpen(false);
    setMediaMode("cover");
  }, [props.companionMode]);
  React.useEffect(() => {
    if (props.presentation === "fullscreen") {
      return;
    }
    setQueueOpen(false);
    setMediaMode((current) => (current === "video" ? "cover" : current));
  }, [props.presentation]);

  const shouldPollLyricsTime =
    props.enabled &&
    !isLive &&
    (props.companionMode === "lyrics" ||
      (props.companionMode !== "queue" && mediaMode === "lyrics")) &&
    onlineLyricsCurrentState.data?.kind === "synced";

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
        presentation: props.workspaceFullscreen
          ? ("app-fullscreen" as const)
          : ("embedded-video" as const),
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
    [callNativePlayer, liveVideoModeActive, props.workspaceFullscreen],
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
    if (!props.workspaceFullscreen || isLive || props.listOpen || queueOpen) {
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
    queueOpen,
    props.track.id,
    props.track.videoId,
    props.workspaceFullscreen,
    trackVideoId,
  ]);

  const showInlineEmbeddedVideo = React.useCallback(
    (rect: ListenNativeVideoRect) => {
      const requestId = inlineNativeVideoRequestRef.current + 1;
      const nextRect = {
        ...rect,
        interactive: false,
        sequence: createListenNativeVideoSequence(requestId),
        presentation: props.workspaceFullscreen
          ? ("app-fullscreen" as const)
          : ("embedded-video" as const),
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
          if (!visible) {
            setInlineNativeVideoAvailable(false);
          }
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
    [callNativePlayer, inlineVideoRevealReady, props.workspaceFullscreen],
  );

  React.useEffect(() => {
    if (!inlineVideoModeActive) {
      hideInlineEmbeddedVideo();
    }
  }, [
    hideInlineEmbeddedVideo,
    inlineVideoModeActive,
  ]);

  React.useEffect(() => {
    if (!inlineVideoModeActive || inlineVideoRevealReady) {
      return;
    }
    hideInlineEmbeddedVideo();
  }, [
    hideInlineEmbeddedVideo,
    inlineVideoModeActive,
    inlineVideoRevealReady,
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
      const requestRef =
        mediaModeRef.current === "video"
          ? inlineNativeVideoRequestRef
          : isLive
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
      if (mode === "video" && props.presentation !== "fullscreen") {
        return;
      }
      setQueueOpen(false);
      setMediaMode(mode);
      if (mode !== "video") {
        hideInlineEmbeddedVideo();
      }
    },
    [hideInlineEmbeddedVideo, props.presentation],
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
      props.onPlaybackErrorCodeChange?.("");
      props.onPlaybackErrorMessageChange?.("");
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
      })
        .then(() => {
          if (!isLive) {
            return;
          }
          return callNativePlayer("Status")
            .then((rawStatus) => {
              const status = rawStatus as ListenNativePlayerEvent;
              if (
                status.provider === "stream" &&
                status.videoId?.trim() === props.track.videoId &&
                status.sessionId?.trim()
              ) {
                livePlaybackSessionIDRef.current = status.sessionId.trim();
              }
            })
            .catch(() => {});
        })
        .catch(markNativePlayerUnavailable);
    },
    [
      callNativePlayer,
      inactivePlayerService,
      isLive,
      markNativePlayerUnavailable,
      props.muted,
      props.onPlayingChange,
      props.onStateChange,
      props.onPlaybackErrorCodeChange,
      props.onPlaybackErrorMessageChange,
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
      if (
        isLive &&
        !isListenLiveEventForSession(
          data,
          "stream",
          livePlaybackSessionIDRef.current,
        )
      ) {
        return;
      }
      if (data.type === "debug") {
        return;
      }
	  if (data.type === "embedded-video-fullscreen-change") {
		setEmbeddedVideoFullscreen(data.active === true);
		setEmbeddedVideoFullscreenPending(false);
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
	      if (
	        isLive &&
	        eventBelongsToCurrentTrack &&
	        !livePlaybackSessionIDRef.current &&
	        data.sessionId?.trim()
	      ) {
	        livePlaybackSessionIDRef.current = data.sessionId.trim();
	      }
	      if (!isLive && data.type === "lyrics-time") {
	        if (
	          eventBelongsToCurrentTrack &&
	          typeof data.currentTime === "number" &&
	          Number.isFinite(data.currentTime)
	        ) {
	          setLyricsCurrentTimeMs(Math.max(0, data.currentTime * 1000));
	          setLyricsPlaybackRate(
	            normalizeListenLyricsPlaybackRate(data.playbackRate),
	          );
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
          props.onPlaybackErrorCodeChange?.(errorCode);
          props.onPlaybackErrorMessageChange?.(errorMessage);
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
          props.onPlaybackErrorCodeChange?.("");
          props.onPlaybackErrorMessageChange?.("");
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
    props.onPlaybackErrorCodeChange,
    props.onPlaybackErrorMessageChange,
    props.onPlayingChange,
    props.onPrevious,
    props.onProgressChange,
    props.onStateChange,
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
    props.onPlaybackErrorCodeChange?.("");
    props.onPlaybackErrorMessageChange?.("");
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
    props.onPlaybackErrorCodeChange,
    props.onPlaybackErrorMessageChange,
    props.onProgressChange,
    props.onStateChange,
    props.progress.duration,
    props.track.videoId,
  ]);

  const handleRequestEmbeddedVideoFullscreen = React.useCallback(() => {
	if (embeddedVideoFullscreenPending) {
		return;
	}
	setEmbeddedVideoFullscreenPending(true);
    void (async () => {
      if (!isLive) {
		await callNativePlayer(
			embeddedVideoFullscreen
				? "ExitEmbeddedVideoFullscreen"
				: "RequestEmbeddedVideoFullscreen",
		);
        return;
      }
      let sessionId = livePlaybackSessionIDRef.current.trim();
      if (!sessionId) {
        const status = (await callNativePlayer("Status")) as ListenNativePlayerEvent;
        if (
          status.provider === "stream" &&
          status.videoId?.trim() === props.track.videoId &&
          status.sessionId?.trim()
        ) {
          sessionId = status.sessionId.trim();
          livePlaybackSessionIDRef.current = sessionId;
        }
      }
	  await callNativePlayer(
		embeddedVideoFullscreen
			? "ExitEmbeddedVideoFullscreen"
			: "RequestEmbeddedVideoFullscreen",
		{
		  provider: "stream",
		  sessionId,
		},
	  );
    })().catch((error) => {
      const description = getListenErrorMessage(error);
      console.warn("[Listen] embedded video fullscreen unavailable", error);
      messageBus.publishToast({
        id: "listen-video-fullscreen-error",
        intent: "danger",
        title: props.text.completed.previewEnterFullscreen,
        description,
        source: "listen",
      });
	}).finally(() => {
		setEmbeddedVideoFullscreenPending(false);
    });
  }, [
	callNativePlayer,
	embeddedVideoFullscreen,
	embeddedVideoFullscreenPending,
	isLive,
	props.text.completed.previewEnterFullscreen,
	props.track.videoId,
  ]);

  const playbackTimelineProgress = playbackAdvertising
    ? playbackAdvertisingProgress ?? LISTEN_EMPTY_PROGRESS
    : props.progress;
  const lyricsSurfaceTimeMs =
    onlineLyricsCurrentState.data?.kind === "synced"
      ? lyricsCurrentTimeMs
      : Math.max(0, props.progress.currentTime * 1000);
  const lyricsSurfacePlaybackRate =
    onlineLyricsCurrentState.data?.kind === "synced"
      ? lyricsPlaybackRate
      : 1;
  const lyricsTimelineRunning =
    props.playing && playbackActivity.timelineRunning && !playbackAdvertising;
  const renderOnlineLyricsControls = (
    placement: "overlay" | "companion" | "fullscreen",
  ) => (
    <ListenLyricsControls
      key={`${onlineLyricsWorkspaceTrack.videoId}:${placement}`}
      placement={placement}
      text={props.text}
      track={onlineLyricsWorkspaceTrack}
      lyrics={onlineLyricsCurrentState.data}
      currentTimeMs={lyricsSurfaceTimeMs}
      timelineRunning={lyricsTimelineRunning}
      playbackRate={lyricsSurfacePlaybackRate}
      language={props.text.locale}
      synced={props.syncedLyricsEnabled}
      romanized={props.romanizedLyrics}
      pinyin={props.pinyinLyrics}
      onLyricsChange={handleOnlineLyricsChange}
      onRestoreAutomatic={handleOnlineLyricsRestoreAutomatic}
    />
  );
  if (props.companionMode === "queue") {
    return (
      <ListenWorkspaceOnlineQueueCompanion
        queueTitle={props.queueTitle}
        queueItems={props.queueItems}
        selectedQueueId={props.selectedQueueId}
        httpBaseURL={props.httpBaseURL}
        playMode={props.playMode}
        text={props.text}
        onPlayModeChange={props.onPlayModeChange}
        onClearQueue={isLive ? undefined : props.onClearQueue}
        onRemoveQueueItem={isLive ? undefined : props.onRemoveQueueItem}
        onMoveQueueItem={isLive ? undefined : props.onMoveQueueItem}
        onUndoQueueEdit={isLive ? undefined : props.onUndoQueueEdit}
        onRedoQueueEdit={isLive ? undefined : props.onRedoQueueEdit}
        queueCanUndo={!isLive && props.queueCanUndo}
        queueCanRedo={!isLive && props.queueCanRedo}
        onSelectQueueTrack={props.onSelectQueueTrack}
      />
    );
  }

  if (props.companionMode === "lyrics") {
    return (
      <ListenWorkspaceLyricsCompanion
        artworkCandidates={buildListenPosterCandidates(
          props.httpBaseURL,
          props.track,
        )}
        title={props.track.title}
        text={props.text}
        lyricsControls={
          isLive ? undefined : renderOnlineLyricsControls("companion")
        }
      >
        <ListenLyricsWorkspace
          variant="companion"
          surfaceActive={props.active && props.companionMode === "lyrics"}
          text={props.text}
          track={onlineLyricsWorkspaceTrack}
          current={{
            lyrics: onlineLyricsCurrentState.data,
            loading: onlineLyricsCurrentState.loading,
            error: onlineLyricsCurrentState.error,
            errorCode: onlineLyricsCurrentState.errorCode,
            errorRetryable: onlineLyricsCurrentState.errorRetryable,
            onRetry: retryLyrics,
          }}
          currentTimeMs={lyricsSurfaceTimeMs}
          timelineRunning={lyricsTimelineRunning}
          playbackRate={lyricsSurfacePlaybackRate}
          romanized={props.romanizedLyrics}
          pinyin={props.pinyinLyrics}
          onSeek={isLive ? undefined : handleOnlineSeek}
        />
      </ListenWorkspaceLyricsCompanion>
    );
  }

  const onlineLyricsControls = !isLive && mediaMode === "lyrics"
    ? renderOnlineLyricsControls(
        props.presentation === "page"
          ? "overlay"
          : props.presentation === "fullscreen"
            ? "fullscreen"
            : "companion",
      )
    : null;
  const effectivePlaybackErrorCode =
    playbackErrorLabel || props.playbackErrorCode || "";
  const effectivePlaybackErrorMessage =
    playbackErrorMessage.trim() || props.playbackErrorMessage?.trim() || "";
  const playbackErrorActive =
    props.state === "error" ||
    effectivePlaybackErrorCode !== "" ||
    effectivePlaybackErrorMessage !== "";
  const playbackErrorSubtitle =
    effectivePlaybackErrorCode ===
    LISTEN_YOUTUBE_VERIFICATION_REQUIRED_ERROR_CODE
      ? props.text.listen.youtubeVerificationRequired
      : effectivePlaybackErrorCode ===
          LISTEN_YOUTUBE_REGION_UNAVAILABLE_ERROR_CODE
        ? props.text.listen.youtubeRegionUnavailable
        : playbackErrorActive
          ? effectivePlaybackErrorMessage || effectivePlaybackErrorCode
          : "";
  const playbackStatusActive = playbackErrorSubtitle !== "";
  const playbackSubtitle = playbackErrorSubtitle || artistName;

  return (
    <div className={cn(
      "relative h-full min-h-0",
      artworkVisualizerVisible ? "overflow-visible" : "overflow-hidden",
    )}>
      <ListenPlayerChrome
        mediaMode={mediaMode}
        presentation={props.presentation}
        workspaceFullscreen={props.workspaceFullscreen}
        backdropCandidates={buildListenPosterCandidates(
          props.httpBaseURL,
          props.track,
        )}
        listOpen={props.listOpen}
        onToggleList={props.onToggleList}
        reserveWindowControls={props.reserveWindowControls}
        airPlaySupported={props.airPlaySupported}
        sourceBadge={<ListenPlayerSourceBadge source={playbackSource} text={props.text} />}
        sourceLabel={resolveListenPlayerSourceLabel(playbackSource, props.text)}
        onOpenSource={props.onOpenPlaybackSource ? () => props.onOpenPlaybackSource?.(playbackSource) : undefined}
        onRequestFullscreen={props.onRequestPlayerFullscreen}
        onExitFullscreen={props.onExitPlayerFullscreen}
        onRequestVideoFullscreen={handleRequestEmbeddedVideoFullscreen}
        lyricsControls={
          props.presentation === "page" ? undefined : onlineLyricsControls
        }
        queueControls={
          <ListenWorkspaceQueueModeSwitch
            playMode={props.playMode}
            text={props.text}
            onChange={props.onPlayModeChange}
          />
        }
        observedPlaybackAudioQuality={props.observedPlaybackAudioQuality}
        headerCover={
          <ListenCompactCoverSurface
            key={artworkRevisionKey}
            srcCandidates={buildListenPosterCandidates(
              props.httpBaseURL,
              props.track,
            )}
            title={props.track.title}
          />
        }
        cover={
          <ListenOnlineArtwork
            key={artworkRevisionKey}
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
          <ListenLyricsWorkspace
            variant={props.presentation === "companion" || props.workspaceFullscreen ? "companion" : "player"}
            surfaceActive={props.active && mediaMode === "lyrics"}
            text={props.text}
            track={onlineLyricsWorkspaceTrack}
            current={{
              lyrics: onlineLyricsCurrentState.data,
              loading: onlineLyricsCurrentState.loading,
              error: onlineLyricsCurrentState.error,
              errorCode: onlineLyricsCurrentState.errorCode,
              errorRetryable: onlineLyricsCurrentState.errorRetryable,
              onRetry: retryLyrics,
            }}
            currentTimeMs={lyricsSurfaceTimeMs}
            timelineRunning={lyricsTimelineRunning}
            playbackRate={lyricsSurfacePlaybackRate}
            romanized={props.romanizedLyrics}
            pinyin={props.pinyinLyrics}
            controls={
              props.presentation === "page" ? onlineLyricsControls : undefined
            }
            onSeek={isLive ? undefined : handleOnlineSeek}
          />
        }
        hasVideo={hasVideo}
        videoLoading={videoLoading}
        live={isLive}
        fullscreenLive={liveFullscreenActive}
        videoId={props.track.videoId}
        liveVideoModeActive={liveVideoModeActive}
        liveVideoVisible={liveVideoVisible}
		nativeVideoGeometrySuspended={embeddedVideoGeometrySuspended}
        inlineVideoRevealReady={inlineVideoRevealReady}
        inlineVideoVisible={inlineVideoVisible}
        inlineVideoAspectRatio={videoAspectRatio}
        track={props.track}
        httpBaseURL={props.httpBaseURL}
        pet={props.pet}
        petImageURL={props.petImageURL}
        lyricsAvailable={!isLive}
        lyricsKind={onlineLyricsCurrentState.data?.kind}
        lyricsLoading={onlineLyricsCurrentState.loading}
        title={props.track.title}
        subtitle={playbackSubtitle}
        subtitleDanger={playbackStatusActive}
        subtitleArtistParts={
          artistName && !isLive && !playbackStatusActive
            ? artistLabelParts
            : undefined
        }
        onSubtitleClick={
          artistName && !isLive && !playbackStatusActive
            ? () => props.onOpenArtist(props.track)
            : undefined
        }
        onSubtitleArtistClick={
          artistName && !isLive && !playbackStatusActive
            ? handleSubtitleArtistClick
            : undefined
        }
        infoActions={
          <>
            {showFavoriteAction ? (
              <ListenPlayerIconButton
                label={props.text.listen.favorite}
                active={props.favoriteActive}
                disabled={props.favoriteBusy}
                onClick={props.onToggleFavorite}
              >
                <Heart
                  className={cn(
                    "h-4 w-4",
                    props.favoriteActive && "listen-playback-icon--filled",
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
        onSeek={isLive ? undefined : handleOnlineSeek}
        onStopPlayback={handleStopPlayback}
        onFitLiveVideoWindow={handleFitLiveVideoWindow}
        onLiveVideoRectChange={showLiveEmbeddedVideo}
        onInlineVideoRectChange={
          props.presentation === "fullscreen"
            ? showInlineEmbeddedVideo
            : undefined
        }
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
          setQueueOpen((current) => {
            const next = !current;
            if (next) {
              setMediaMode("cover");
              hideInlineEmbeddedVideo();
            }
            return next;
          });
        }}
        visualizerMode={props.visualizerMode}
        visualizerEnabled={props.visualizerEnabled}
        visualizerActive={props.playing}
        queueOpen={queueOpen}
        workspaceQueue={
          <ListenWorkspaceOnlineQueueCompanion
            queueTitle={props.queueTitle}
            queueItems={props.queueItems}
            selectedQueueId={props.selectedQueueId}
            httpBaseURL={props.httpBaseURL}
            playMode={props.playMode}
            text={props.text}
            onPlayModeChange={props.onPlayModeChange}
            onClearQueue={isLive ? undefined : props.onClearQueue}
            onRemoveQueueItem={isLive ? undefined : props.onRemoveQueueItem}
            onMoveQueueItem={isLive ? undefined : props.onMoveQueueItem}
            onUndoQueueEdit={isLive ? undefined : props.onUndoQueueEdit}
            onRedoQueueEdit={isLive ? undefined : props.onRedoQueueEdit}
            queueCanUndo={!isLive && props.queueCanUndo}
            queueCanRedo={!isLive && props.queueCanRedo}
            showFooter={props.presentation !== "companion"}
            onSelectQueueTrack={props.onSelectQueueTrack}
          />
        }
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
  presentation: ListenPlayerPresentation;
  workspaceFullscreen?: boolean;
  backdropCandidates?: string[];
  listOpen?: boolean;
  onToggleList?: () => void;
  reserveWindowControls: boolean;
  airPlaySupported: boolean;
  sourceBadge?: React.ReactNode;
  sourceLabel?: string;
  onOpenSource?: () => void;
  lyricsControls?: React.ReactNode;
  queueControls?: React.ReactNode;
  onRequestFullscreen?: () => void;
  onExitFullscreen?: () => void;
  onRequestVideoFullscreen?: () => void;
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
	nativeVideoGeometrySuspended?: boolean;
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
  subtitleDanger?: boolean;
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
  workspaceQueue?: React.ReactNode;
}) {
  const workspaceCompanion = props.presentation === "companion";
  const renderedMediaMode = props.mediaMode;
  const [videoAppFullscreen, setVideoAppFullscreen] = React.useState(false);
  const inlineVideoActive =
    props.presentation === "fullscreen" &&
    props.hasVideo &&
    renderedMediaMode === "video" &&
    Boolean(props.onInlineVideoRectChange);
  const workspaceQueueActive =
    (props.workspaceFullscreen === true || workspaceCompanion) &&
    props.queueOpen === true &&
    Boolean(props.workspaceQueue);
  const inlineVideoFullscreen =
    props.workspaceFullscreen === true &&
    inlineVideoActive &&
    !workspaceQueueActive &&
    videoAppFullscreen;
  const splitMode =
    workspaceQueueActive || renderedMediaMode === "lyrics" || inlineVideoActive;
  const rootRef = React.useRef<HTMLDivElement | null>(null);
  const playerStackRef = React.useRef<HTMLDivElement | null>(null);
  const layoutKey = [
    props.presentation,
    props.listOpen === true ? "list" : "compact",
    renderedMediaMode,
    workspaceQueueActive ? "queue" : "media",
    inlineVideoFullscreen ? "video-app-fullscreen" : "video-carved",
  ].join(":");
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
    splitMode &&
    (props.workspaceFullscreen ||
      (layoutMeasured && layoutWidth >= LISTEN_MEDIA_SPLIT_MIN_WIDTH));
  const singleColumnContext =
    (workspaceQueueActive || renderedMediaMode === "lyrics") &&
    !inlineVideoActive &&
    !splitEnabled;
  const playLabel = props.loading
    ? resolveListenPlaybackStatusLabel(
        props.playbackState ?? "loading",
        props.text,
      )
    : props.playing
      ? props.text.listen.pause
      : props.text.listen.play;
  const visualizerMode = props.visualizerMode ?? "off";
  const visualizerEnabled = props.visualizerEnabled === true && visualizerMode !== "off";
  const visualizerActive = props.visualizerActive === true && props.playing && !props.loading;
  React.useEffect(() => {
    if (!inlineVideoActive || workspaceQueueActive) {
      setVideoAppFullscreen(false);
    }
  }, [inlineVideoActive, workspaceQueueActive]);
  React.useEffect(() => {
    setVideoAppFullscreen(false);
  }, [props.videoId]);
  const handleToggleVideoAppFullscreen = React.useCallback(() => {
    setVideoAppFullscreen((current) => !current);
  }, []);
  React.useEffect(() => {
    if (!inlineVideoFullscreen) {
      return;
    }
    const handleEscape = (event: KeyboardEvent) => {
      if (event.key !== "Escape" || event.defaultPrevented) {
        return;
      }
      if (document.querySelector('[role="dialog"][data-state="open"]')) {
        return;
      }
      event.preventDefault();
      event.stopPropagation();
      event.stopImmediatePropagation();
      setVideoAppFullscreen(false);
    };
    window.addEventListener("keydown", handleEscape, true);
    return () => window.removeEventListener("keydown", handleEscape, true);
  }, [inlineVideoFullscreen]);
  const inlineVideoSurface = inlineVideoActive ? (
    <ListenInlineVideoSurface
      variant={
        inlineVideoFullscreen
          ? "fullscreen"
          : splitEnabled
            ? "wide"
            : "compact"
      }
      active={
        inlineVideoActive &&
        (inlineVideoFullscreen || layoutMeasured) &&
        props.inlineVideoRevealReady === true
      }
      visible={
        (inlineVideoFullscreen || layoutMeasured) &&
        props.inlineVideoVisible === true
      }
	  geometrySuspended={props.nativeVideoGeometrySuspended === true}
      aspectRatio={props.inlineVideoAspectRatio}
      pet={props.pet ?? null}
      petImageURL={props.petImageURL ?? ""}
      title={props.title}
      text={props.text}
      onRectChange={props.onInlineVideoRectChange}
    />
  ) : null;
  const activeMedia =
    (workspaceQueueActive ? props.workspaceQueue : null) ??
    inlineVideoSurface ??
    (renderedMediaMode === "lyrics"
      ? props.lyrics
      : props.cover);
  const mediaStage =
    workspaceQueueActive ? (
      <div
        className={cn(
          "h-full min-h-0 w-full overflow-hidden",
          props.workspaceFullscreen &&
            "listen-workspace-fullscreen-player__queue",
        )}
      >
        {props.workspaceFullscreen ? (
          <GlassSurface
            className="listen-workspace-fullscreen-player__queue-surface h-full min-h-0 w-full"
            surfaceRole="card"
            elevation="floating"
            shape="card"
          >
            {props.workspaceQueue}
          </GlassSurface>
        ) : (
          props.workspaceQueue
        )}
      </div>
    ) : inlineVideoSurface ? (
      inlineVideoSurface
    ) : renderedMediaMode !== "lyrics" ? (
      props.cover
    ) : (
      <div
        className={cn(
          "listen-player-media-transition w-full",
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
  }, [layoutKey, renderedMediaMode]);

  return (
    <TooltipProvider delayDuration={0}>
      <div
        ref={rootRef}
        data-listen-player-root="true"
        data-player-presentation={props.presentation}
        data-workspace-fullscreen={props.workspaceFullscreen ? "true" : undefined}
        data-fullscreen-media-mode={
          props.workspaceFullscreen ? renderedMediaMode : undefined
        }
        data-workspace-split={splitEnabled ? "true" : "false"}
        data-video-app-fullscreen={inlineVideoFullscreen ? "true" : "false"}
        data-artwork-visualizer-visible={visualizerEnabled ? "true" : "false"}
        data-playback-state={props.playbackState}
        className={cn(
          "relative isolate flex h-full min-h-0 flex-col",
          props.workspaceFullscreen && "listen-workspace-fullscreen-player",
          workspaceCompanion && "listen-workspace-companion-player",
          visualizerEnabled ? "overflow-visible" : "overflow-hidden",
        )}
      >
        {props.workspaceFullscreen && !props.fullscreenLive && !inlineVideoActive ? (
          <ListenWorkspaceFullscreenBackdrop
            candidates={props.backdropCandidates ?? [LISTEN_DEFAULT_COVER_IMAGE_URL]}
            playing={props.playing}
          />
        ) : null}
        {inlineVideoFullscreen ? (
          <div className="listen-workspace-fullscreen-player__video">
            {inlineVideoSurface}
          </div>
        ) : props.fullscreenLive ? (
          <ListenLiveVideoShell
            videoId={props.videoId ?? ""}
            liveVideoModeActive={props.liveVideoModeActive === true}
            liveVideoVisible={props.liveVideoVisible === true}
			geometrySuspended={props.nativeVideoGeometrySuspended === true}
            track={props.track}
            httpBaseURL={props.httpBaseURL ?? ""}
            pet={props.pet ?? null}
            petImageURL={props.petImageURL ?? ""}
            title={props.title}
            subtitle={props.subtitle}
            subtitleDanger={props.subtitleDanger}
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
            "relative z-10 min-h-0 flex-1 px-3 pb-2 pt-1 sm:px-5 sm:pb-4",
            props.workspaceFullscreen && "listen-workspace-fullscreen-player__content",
            workspaceCompanion && "listen-workspace-companion-player__content",
            visualizerEnabled ? "overflow-visible" : "overflow-hidden",
          )}
        >
          <div
            data-split={splitEnabled ? "true" : "false"}
            data-motion={props.workspaceFullscreen || inlineVideoActive ? "off" : "on"}
            className={cn(
              "listen-player-layout-grid mx-auto grid h-full min-h-0 w-full items-center gap-6",
              props.workspaceFullscreen
                ? "listen-workspace-fullscreen-player__grid"
                : workspaceCompanion
                  ? "listen-workspace-companion-player__grid"
                : splitMode
                  ? splitEnabled
                    ? "max-w-7xl grid-cols-[18rem_minmax(0,1fr)] gap-6 lg:gap-8"
                    : "max-w-[18rem] justify-center"
                  : "max-w-[18rem] justify-center",
            )}
          >
            <div
              data-motion={inlineVideoActive ? "off" : "on"}
              className={cn(
                "listen-player-now-playing min-w-0",
                props.workspaceFullscreen &&
                  "listen-workspace-fullscreen-player__now-playing",
                props.workspaceFullscreen || splitEnabled
                  ? "justify-self-start"
                  : workspaceCompanion
                    ? "w-full justify-self-stretch"
                    : "justify-self-center",
              )}
            >
              <div
                ref={playerStackRef}
                className={cn(
                  "mx-auto w-full",
                  props.workspaceFullscreen
                    ? "listen-workspace-fullscreen-player__stack"
                    : workspaceCompanion
                      ? "listen-workspace-companion-player__stack"
                    : LISTEN_PLAYER_SURFACE_WIDTH_CLASS,
                )}
              >
                {singleColumnContext ? (
                  <div
                    data-player-context={workspaceQueueActive ? "queue" : "lyrics"}
                    className="listen-single-lyrics-panel flex h-[min(42rem,calc(100vh-8.5rem))] max-h-full min-h-0 flex-col overflow-hidden"
                  >
                    <div className="listen-single-lyrics-panel__header mb-4 flex shrink-0 items-center gap-3">
                      {props.headerCover}
                      <div className="min-w-0 flex-1">
                        <ListenScrollingText
                          text={props.title}
                          className="listen-single-lyrics-panel__title"
                        />
                        <ListenSubtitleText
                          text={props.subtitle || props.text.listen.nowPlaying}
                          artistParts={props.subtitleArtistParts}
                          className={cn(
                            "listen-single-lyrics-panel__subtitle mt-0.5",
                            props.subtitleDanger &&
                              "listen-playback-status-subtitle",
                          )}
                          onClick={props.subtitle ? props.onSubtitleClick : undefined}
                          onArtistClick={props.onSubtitleArtistClick}
                        />
                      </div>
                      <div className="flex shrink-0 items-center gap-1">
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <Button
                              type="button"
                              variant="default"
                              size="icon"
                              shape="circle"
                              className={cn(
                                LISTEN_PRIMARY_PLAY_BUTTON_CLASS,
                                LISTEN_PRIMARY_PLAY_BUTTON_SIZE_CLASS.small,
                                !props.disabled && LISTEN_PRIMARY_PLAY_BUTTON_HOVER_CLASS,
                              )}
                              disabled={props.disabled}
                              aria-label={playLabel}
                              title={playLabel}
                              onClick={props.onTogglePlayback}
                            >
                              {props.loading ? (
                                <Loader2 className={cn(LISTEN_PRIMARY_PLAY_ICON_SIZE_CLASS.small, "listen-loading-spinner")} />
                              ) : props.playing ? (
                                <Pause className={cn(LISTEN_PRIMARY_PLAY_ICON_SIZE_CLASS.small, "listen-playback-icon--filled")} />
                              ) : (
                                <Play className={cn("listen-playback-icon--filled ml-0.5", LISTEN_PRIMARY_PLAY_ICON_SIZE_CLASS.small)} />
                              )}
                            </Button>
                          </TooltipTrigger>
                          <TooltipContent side="top">{playLabel}</TooltipContent>
                        </Tooltip>
                        <ListenPlayerIconButton
                          label={props.text.listen.next}
                          disabled={props.disabled}
                          onClick={props.onNext}
                        >
                          <SkipForward className="listen-playback-icon--filled h-4 w-4" />
                        </ListenPlayerIconButton>
                      </div>
                    </div>
                    <div className="min-h-0 flex-1 overflow-hidden">
                      {workspaceQueueActive ? mediaStage : props.lyrics}
                    </div>
                  </div>
                ) : (
                  <>
                    <div className={cn(splitMode && splitEnabled && "hidden")}>
                      {splitMode && splitEnabled ? null : mediaStage}
                    </div>
                    {splitMode && splitEnabled ? (
                      <div className={cn("listen-player-split-cover", splitEnabled ? "block" : "hidden")}>
                        {props.cover}
                      </div>
                    ) : null}
                    <ListenTrackInfoRow
                      title={props.title}
                      subtitle={props.subtitle}
                      subtitleArtistParts={props.subtitleArtistParts}
                      subtitleDanger={props.subtitleDanger}
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
                      centerLabel={
                        props.workspaceFullscreen || workspaceCompanion
                          ? resolveListenFullscreenQualityLabel(
                              props.observedPlaybackAudioQuality,
                              props.text,
                            )
                          : ""
                      }
                      live={props.live}
                      playing={props.playing}
                      advertising={props.advertising}
                      advertisingLabel={props.advertisingLabel}
                      loading={props.progressLoading}
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
                    {props.workspaceFullscreen ? null : (
                      <ListenPlayerVolume
                        muted={props.muted}
                        volume={props.volume}
                        text={props.text}
                        onToggleMute={props.onToggleMute}
                        onVolumeChange={props.onVolumeChange}
                      />
                    )}
                  </>
                )}
              </div>
            </div>
            {splitMode ? (
              <div
                data-motion={inlineVideoActive ? "off" : "on"}
                data-visible={splitEnabled ? "true" : "false"}
                className={cn(
                  "listen-player-secondary-media min-h-0",
                  props.workspaceFullscreen && "listen-workspace-fullscreen-player__media",
                  splitEnabled
                    ? "flex h-full translate-x-0 items-center justify-center"
                    : "pointer-events-none hidden translate-x-4",
                )}
                style={
                  !props.workspaceFullscreen && splitEnabled && playerStackHeight > 0
                    ? { height: playerStackHeight }
                    : undefined
                }
              >
                <div
                  className="listen-player-secondary-media__content h-full w-full max-w-[46rem]"
                >
                  {splitEnabled ? mediaStage : null}
                </div>
              </div>
            ) : null}
          </div>
        </div>
        )}
        {props.presentation === "page" ? props.queueOverlay : null}
        {props.fullscreenLive ? null : (
        <ListenPlayerFooter
          mediaMode={renderedMediaMode}
          presentation={props.presentation}
          workspaceFullscreen={props.workspaceFullscreen}
          reserveWindowControls={props.reserveWindowControls}
          airPlaySupported={props.airPlaySupported}
          sourceBadge={props.workspaceFullscreen ? undefined : props.sourceBadge}
          sourceLabel={props.sourceLabel}
          fullscreenTransport={
            inlineVideoFullscreen ? (
              <div className="listen-video-fullscreen-transport flex min-w-0 flex-1 items-center gap-2">
                <ListenPlayerIconButton
                  label={playLabel}
                  disabled={props.disabled}
                  className={cn(
                    LISTEN_PLAYER_FOOTER_ICON_BUTTON_CLASS,
                    "listen-player-footer__transport-toggle shrink-0",
                  )}
                  onClick={props.onTogglePlayback}
                >
                  {props.loading ? (
                    <Loader2 className="h-4 w-4 listen-loading-spinner" />
                  ) : props.playing ? (
                    <Pause className="listen-playback-icon--filled h-4 w-4" />
                  ) : (
                    <Play className="listen-playback-icon--filled ml-0.5 h-4 w-4" />
                  )}
                </ListenPlayerIconButton>
                <ListenPlayerProgress
                  variant="footer"
                  progress={props.progress}
                  text={props.text}
                  live={props.live}
                  playing={props.playing}
                  advertising={props.advertising}
                  advertisingLabel={props.advertisingLabel}
                  loading={props.progressLoading}
                  onSeek={props.onSeek}
                />
              </div>
            ) : undefined
          }
          leading={
            props.workspaceFullscreen ? (
              <div className="listen-workspace-fullscreen-player__volume wails-no-drag">
                <ListenPlayerVolume
                  muted={props.muted}
                  volume={props.volume}
                  text={props.text}
                  onToggleMute={props.onToggleMute}
                  onVolumeChange={props.onVolumeChange}
                />
              </div>
            ) : undefined
          }
          hasVideo={props.hasVideo}
          videoHidden={props.videoHidden}
          videoLoading={props.videoLoading}
          live={props.live}
          lyricsAvailable={props.lyricsAvailable !== false}
          lyricsKind={props.lyricsKind}
          lyricsLoading={props.lyricsLoading}
          queueOpen={props.queueOpen}
          text={props.text}
          muted={props.muted}
          onAirPlay={props.presentation === "page" ? props.onAirPlay : undefined}
          onMediaModeChange={props.onMediaModeChange}
          onToggleQueue={props.onToggleQueue}
          onToggleMute={props.workspaceFullscreen ? undefined : props.onToggleMute}
          onOpenSource={props.onOpenSource}
          lyricsControls={
            renderedMediaMode === "lyrics" ? props.lyricsControls : undefined
          }
          companionControls={
            workspaceCompanion
              ? workspaceQueueActive
                ? props.queueControls
                : renderedMediaMode === "lyrics"
                  ? props.lyricsControls
                  : undefined
              : undefined
          }
          videoAppFullscreen={inlineVideoFullscreen}
          onToggleVideoAppFullscreen={
            inlineVideoActive ? handleToggleVideoAppFullscreen : undefined
          }
          onRequestVideoFullscreen={
            inlineVideoActive && props.inlineVideoVisible === true
              ? props.onRequestVideoFullscreen
              : undefined
          }
          onRequestFullscreen={props.onRequestFullscreen}
        />
        )}
      </div>
    </TooltipProvider>
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
