import type {
  PlaybackProvider,
  PlaybackSessionSnapshot,
} from "@/shared/playback";
import type {
  ListenNowPlayingStatus,
  ListenPlaybackSource,
} from "@/app/main/listen/types";
import type { YouTubeWorkspacePlaybackState } from "@/app/youtube";
import { APP_WORKSPACE_IDS, type AppWorkspaceId } from "@/app/workspace/types";
import type { MusicWorkspaceScope } from "@/app/workspace/MusicWorkspaceSidebar";

const MUSIC_WORKSPACE_PLAYBACK_PROVIDERS: ReadonlySet<PlaybackProvider> =
  new Set(["youtube_music", "stream", "local"]);

const LISTEN_SOURCE_BY_PROVIDER: Partial<
  Record<PlaybackProvider, ListenPlaybackSource>
> = {
  local: "local",
  stream: "radio",
  youtube_music: "youtube_music",
};

type WorkspaceTransportSession = Pick<PlaybackSessionSnapshot, "focus"> & {
  item: {
    source: {
      provider: PlaybackProvider;
    };
  };
};

export type GlobalPlaybackCommand =
  | "previous"
  | "toggle"
  | "play"
  | "pause"
  | "next";

export type GlobalPlaybackCommandRoute =
  | { target: "listen"; command: GlobalPlaybackCommand }
  | { target: "youtube-queue"; command: "previous" | "next" }
  | {
      target: "coordinator";
      command: "previous" | "play" | "pause" | "next";
    }
  | { target: "none" };

export function resolveListenFallbackPlaybackCommand(
  status: Pick<ListenNowPlayingStatus, "state"> | null,
  command: GlobalPlaybackCommand,
): GlobalPlaybackCommand {
  return command === "toggle" && status?.state === "loading"
    ? "pause"
    : command;
}

function normalizedIdentity(value?: string) {
  return value?.trim() ?? "";
}

function normalizedCount(value?: string) {
  const count = Number(value);
  return Number.isFinite(count) && count >= 0 ? count : undefined;
}

function collectMediaIdentityTokens(value?: string) {
  const candidate = normalizedIdentity(value);
  if (!candidate) {
    return [];
  }
  const tokens = [candidate];
  try {
    const url = new URL(candidate);
    const videoID = url.searchParams.get("v")?.trim();
    if (videoID) {
      tokens.push(videoID);
    }
  } catch {
    // IDs and local paths are valid identities without being URLs.
  }
  return tokens;
}

function listenStatusMatchesSession(
  session: PlaybackSessionSnapshot,
  status?: ListenNowPlayingStatus | null,
) {
  if (!status || session.focus !== "persistent") {
    return false;
  }
  const expectedSource = LISTEN_SOURCE_BY_PROVIDER[session.item.source.provider];
  if (!expectedSource || status.playbackSource !== expectedSource) {
    return false;
  }

  const sessionTokens = new Set(
    [
      session.item.id,
      session.item.source.id,
      session.item.source.uri,
      session.item.canonicalUrl,
    ].flatMap(collectMediaIdentityTokens),
  );
  const statusTokens = [status.mediaId, status.sourceURL].flatMap(
    collectMediaIdentityTokens,
  );
  if (sessionTokens.size > 0 && statusTokens.length > 0) {
    return statusTokens.some((token) => sessionTokens.has(token));
  }

  // Older persisted statuses do not carry mediaId. A strict metadata match is
  // safe enough for that one-version migration without allowing an unrelated
  // provider session to inherit stale Listen state.
  const sessionArtist = normalizedIdentity(
    session.item.artist || session.item.artists?.join(", "),
  );
  return (
    normalizedIdentity(status.title) !== "" &&
    normalizedIdentity(status.title) === normalizedIdentity(session.item.title) &&
    sessionArtist !== "" &&
    normalizedIdentity(status.subtitle) === sessionArtist
  );
}

function youtubeStatusMatchesSession(
  session: PlaybackSessionSnapshot,
  status?: YouTubeWorkspacePlaybackState | null,
) {
  if (!status || session.item.source.provider !== "youtube") {
    return false;
  }
  return (
    status.descriptor.sessionId === session.id ||
    status.status.sessionId === session.id ||
    status.descriptor.videoId === session.item.source.id ||
    status.descriptor.videoId === session.item.id
  );
}

export function recoverYouTubeWorkspacePlayback(
  session: PlaybackSessionSnapshot | null,
): YouTubeWorkspacePlaybackState | null {
  if (
    !session ||
    session.focus !== "persistent" ||
    session.item.source.provider !== "youtube"
  ) {
    return null;
  }
  const currentVideoID =
    normalizedIdentity(session.item.source.id) ||
    normalizedIdentity(session.item.id);
  if (!currentVideoID) {
    return null;
  }
  const toVideo = (item: PlaybackSessionSnapshot["item"]) => {
    const videoId = normalizedIdentity(item.source.id) || normalizedIdentity(item.id);
    return {
      itemKind: "video" as const,
      videoId,
      title: item.title,
      channel: item.artist || item.artists?.join(", ") || "",
      channelId: item.metadata?.channelId,
      thumbnailUrl: item.artworkUrl,
      durationSeconds: item.duration,
      viewCount: normalizedCount(item.metadata?.viewCount),
      publishedLabel: item.metadata?.publishedLabel,
      webUrl:
        item.canonicalUrl ||
        (videoId ? `https://www.youtube.com/watch?v=${videoId}` : ""),
      live: item.source.live,
    };
  };
  const queue = session.queue
    .filter(
      (item) =>
        item.source.provider === "youtube" &&
        Boolean(normalizedIdentity(item.source.id) || normalizedIdentity(item.id)),
    )
    .map(toVideo);
  if (!queue.some((item) => item.videoId === currentVideoID)) {
    queue.unshift(toVideo(session.item));
  }
  const matchedIndex = queue.findIndex((item) => item.videoId === currentVideoID);
  const currentIndex = matchedIndex >= 0 ? matchedIndex : 0;
  const duration = Math.max(0, session.duration || session.item.duration || 0);
  const controls = {
    like: session.capabilities.like,
    dislike: session.capabilities.dislike,
    captions: session.capabilities.captions,
    audioTrack: session.capabilities.audioTracks,
    quality: session.capabilities.quality,
    volume: session.capabilities.volume,
  };
  return {
    descriptor: {
      source: "youtube",
      mediaKind: "video",
      sessionId: session.id,
      videoId: currentVideoID,
      title: session.item.title,
      artist: session.item.artist || session.item.artists?.join(", ") || undefined,
      channelId: session.item.metadata?.channelId,
      thumbnailUrl: session.item.artworkUrl,
      durationSeconds: duration,
      viewCount: normalizedCount(session.item.metadata?.viewCount),
      publishedLabel: session.item.metadata?.publishedLabel,
      webUrl:
        session.item.canonicalUrl ||
        `https://www.youtube.com/watch?v=${currentVideoID}`,
    },
    status: {
      provider: "youtube",
      sessionId: session.id,
      available: session.capabilities.available,
      videoId: currentVideoID,
      state: session.state,
      title: session.item.title,
      artist: session.item.artist || session.item.artists?.join(", ") || undefined,
      thumbnailUrl: session.item.artworkUrl,
      currentTime: Math.max(0, session.position),
      duration,
      volume: session.volume,
      muted: session.muted,
      controls,
    },
    currentIndex,
    queue,
    muted: session.muted,
    volume: session.volume,
    capabilities: {
      previous: currentIndex > 0 || session.capabilities.previous,
      next: currentIndex + 1 < queue.length || session.capabilities.next,
      playPause: session.capabilities.playPause,
      like: session.capabilities.like,
      dislike: session.capabilities.dislike,
      fullscreen: session.capabilities.fullscreen,
      captions: session.capabilities.captions,
      audioTrack: session.capabilities.audioTracks,
      quality: session.capabilities.quality,
      volume: session.capabilities.volume,
    },
  };
}

export function projectCoordinatorPlaybackStatus(
  session: PlaybackSessionSnapshot | null,
  listenStatus?: ListenNowPlayingStatus | null,
  youtubeStatus?: YouTubeWorkspacePlaybackState | null,
): ListenNowPlayingStatus | null {
  if (!session) {
    return null;
  }
  const state: ListenNowPlayingStatus["state"] =
    session.state === "playing"
      ? "playing"
      : session.state === "loading" || session.state === "buffering"
        ? "loading"
        : session.state === "error"
          ? "error"
          : "paused";
  const provider = session.item.source.provider;
  const persistent = session.focus === "persistent";
  const matchingListenStatus = listenStatusMatchesSession(session, listenStatus)
    ? listenStatus
    : null;
  const matchingYouTubeStatus = youtubeStatusMatchesSession(
    session,
    youtubeStatus,
  )
    ? youtubeStatus
    : null;
  const projectedLive =
    provider === "stream" ||
    session.item.source.live === true ||
    matchingListenStatus?.live === true;
  const playbackSource: ListenNowPlayingStatus["playbackSource"] =
    provider === "youtube"
      ? "youtube"
      : provider === "stream"
        ? "radio"
        : provider === "local"
          ? session.focus === "transient_preview"
            ? "library_preview"
            : "local"
          : provider === "youtube_music"
            ? "youtube_music"
            : "unknown";
  const artworkCandidates = Array.from(
    new Set(
      [
        matchingListenStatus?.artworkURL,
        ...(matchingListenStatus?.artworkCandidates ?? []),
        session.item.artworkUrl,
      ].filter((value): value is string => Boolean(value?.trim())),
    ),
  );
  const canPrevious = persistent
    ? provider === "youtube"
      ? matchingYouTubeStatus?.capabilities.previous === true ||
        session.capabilities.previous
      : matchingListenStatus?.canPrevious ??
        matchingListenStatus?.canControl ??
        session.capabilities.previous
    : false;
  const canNext = persistent
    ? provider === "youtube"
      ? matchingYouTubeStatus?.capabilities.next === true ||
        session.capabilities.next
      : matchingListenStatus?.canNext ??
        matchingListenStatus?.canControl ??
        session.capabilities.next
    : false;
  const currentTime = Math.max(0, session.position);
  const duration = projectedLive
    ? currentTime
    : Math.max(0, session.duration || session.item.duration || 0);

  return {
    state,
    live: projectedLive,
    mediaId: session.item.source.id || session.item.id,
    playbackSource,
    playbackSourceLabel: matchingListenStatus?.playbackSourceLabel,
    title: session.item.title,
    subtitle:
      session.item.artist || session.item.artists?.join(", ") || provider,
    artists: matchingListenStatus?.artists,
    artworkURL: artworkCandidates[0] || "",
    artworkCandidates,
    mode:
      provider === "stream"
        ? "hush"
        : provider === "local"
          ? "linger"
          : "muse",
    canControl: session.capabilities.playPause,
    canPrevious,
    canNext,
    progress: {
      currentTime,
      duration,
      bufferedTime: projectedLive ? currentTime : 0,
    },
    muted: session.muted,
    volume: session.volume,
    sourceURL:
      matchingListenStatus?.sourceURL || session.item.canonicalUrl || undefined,
    favoriteActive: matchingListenStatus?.favoriteActive,
    canFavorite: matchingListenStatus?.canFavorite,
  };
}

export function resolveGlobalPlaybackCommandRoute(
  session: PlaybackSessionSnapshot | null,
  command: GlobalPlaybackCommand,
): GlobalPlaybackCommandRoute {
  if (!session) {
    return { target: "listen", command };
  }

  if (session.focus === "persistent") {
    if (
      session.item.source.provider === "youtube" &&
      (command === "previous" || command === "next")
    ) {
      return { target: "youtube-queue", command };
    }
    if (
      (command === "previous" || command === "next") &&
      MUSIC_WORKSPACE_PLAYBACK_PROVIDERS.has(session.item.source.provider)
    ) {
      return { target: "listen", command };
    }
  }

  const playbackMayBeAudible =
    session.state === "playing" ||
    session.state === "loading" ||
    session.state === "buffering";
  if (command === "toggle") {
    return {
      target: "coordinator",
      command: playbackMayBeAudible ? "pause" : "play",
    };
  }
  if (command === "play") {
    return playbackMayBeAudible
      ? { target: "none" }
      : { target: "coordinator", command: "play" };
  }
  if (command === "pause") {
    return !playbackMayBeAudible
      ? { target: "none" }
      : { target: "coordinator", command: "pause" };
  }
  if (command === "previous" && session.capabilities.previous) {
    return { target: "coordinator", command: "previous" };
  }
  if (command === "next" && session.capabilities.next) {
    return { target: "coordinator", command: "next" };
  }
  return { target: "none" };
}

export function shouldPresentMusicWorkspaceTransport(
  session: WorkspaceTransportSession | null,
) {
  if (!session) {
    return true;
  }
  return (
    session.focus === "persistent" &&
    MUSIC_WORKSPACE_PLAYBACK_PROVIDERS.has(session.item.source.provider)
  );
}

export function shouldShowWorkspacePlaybackActivity(
  source: ListenPlaybackSource | undefined,
  activeWorkspaceId: AppWorkspaceId,
  musicScope: MusicWorkspaceScope,
  youtubeWatchVisible = false,
) {
  // YouTube video playback owns a persistent watch surface instead of an
  // in-workspace transport bar. The Watch page already exposes the complete
  // transport, so repeat the sidebar card only while browsing or after the
  // user leaves the YouTube station.
  if (source === "youtube") {
    return !(
      activeWorkspaceId === APP_WORKSPACE_IDS.youtube && youtubeWatchVisible
    );
  }
  if (activeWorkspaceId === APP_WORKSPACE_IDS.music) {
    const belongsToMusicScope =
      musicScope === "online"
        ? source === "youtube_music" || source === "radio"
        : source === "local";
    return !belongsToMusicScope;
  }
  if (activeWorkspaceId === APP_WORKSPACE_IDS.youtube) {
    return true;
  }
  return true;
}
