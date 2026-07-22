import { CircleHelp, Disc3, Radio } from "lucide-react";
import { siYoutube, siYoutubemusic } from "simple-icons";

import { getXiaText } from "@/features/xiadown/shared";
import { ListenCoverArtwork } from "@/shared/assets/listen-cover-artwork";

import type {
  ListenMode,
  ListenObservedPlaybackAudioQuality,
  ListenPlaybackSource,
} from "@/app/main/listen/types";

export function resolveListenPlayerSurfaceActive(
  workspaceActive: boolean,
  surfaceVisible: boolean | undefined,
) {
  return (
    surfaceVisible === true ||
    (workspaceActive && surfaceVisible !== false)
  );
}

export function openListenArtistFromPlayerSurface(options: {
  workspaceActive: boolean;
  workspaceLayout: boolean;
  openPlaybackSource?: (source: ListenPlaybackSource) => void;
  openArtist: () => void;
  schedule: (openArtist: () => void) => void;
}) {
  if (
    options.workspaceLayout &&
    !options.workspaceActive &&
    options.openPlaybackSource
  ) {
    options.openPlaybackSource("youtube_music");
    options.schedule(options.openArtist);
    return;
  }
  options.openArtist();
}

export function listenPlaybackSourceFromMode(
  mode: ListenMode,
): Extract<ListenPlaybackSource, "youtube_music" | "radio" | "local"> {
  if (mode === "hush") {
    return "radio";
  }
  if (mode === "muse") {
    return "youtube_music";
  }
  return "local";
}

export function resolveListenPlayerSourceLabel(
  source: ListenPlaybackSource,
  text: ReturnType<typeof getXiaText>,
) {
  switch (source) {
    case "youtube_music":
      return text.workspace.youtubeMusic;
    case "radio":
      return text.workspace.radio;
    case "local":
      return text.workspace.local;
    case "youtube":
      return text.workspace.youtube;
    case "library_preview":
      return text.workspace.library;
    default:
      return text.common.unknown;
  }
}

export function ListenPlayerSourceIcon(props: {
  source: ListenPlaybackSource;
  className?: string;
}) {
  if (props.source === "radio") {
    return <Radio aria-hidden="true" className={props.className} />;
  }
  if (props.source === "youtube_music" || props.source === "youtube") {
    const path = props.source === "youtube_music"
      ? siYoutubemusic.path
      : siYoutube.path;
    return (
      <svg
        aria-hidden="true"
        viewBox="0 0 24 24"
        fill="currentColor"
        className={props.className}
      >
        <path d={path} />
      </svg>
    );
  }
  if (props.source === "unknown") {
    return <CircleHelp aria-hidden="true" className={props.className} />;
  }
  return <Disc3 aria-hidden="true" className={props.className} />;
}

export function ListenPlayerSourceBadge(props: {
  source: ListenPlaybackSource;
  text: ReturnType<typeof getXiaText>;
}) {
  const label = resolveListenPlayerSourceLabel(props.source, props.text);
  return (
    <>
      <ListenPlayerSourceIcon
        source={props.source}
        className="listen-player-source-badge__icon"
      />
      <span className="listen-player-source-badge__label">{label}</span>
    </>
  );
}

export function ListenWorkspaceFullscreenBackdrop(props: {
  candidates: string[];
  playing: boolean;
}) {
  return (
    <div
      aria-hidden="true"
      className="listen-workspace-fullscreen-backdrop"
      data-playing={props.playing ? "true" : "false"}
    >
      <ListenCoverArtwork
        alt=""
        candidates={props.candidates}
        draggable={false}
        className="listen-workspace-fullscreen-backdrop__artwork"
      />
      <span className="listen-workspace-fullscreen-backdrop__wash" />
      <span className="listen-workspace-fullscreen-backdrop__vignette" />
    </div>
  );
}

export function resolveListenFullscreenQualityLabel(
  quality: ListenObservedPlaybackAudioQuality | "" | undefined,
  text: ReturnType<typeof getXiaText>,
) {
  switch (quality) {
    case "AUDIO_QUALITY_LOW":
      return text.listen.audioQualityOptions.low;
    case "AUDIO_QUALITY_HIGH":
      return text.listen.audioQualityOptions.high;
    case "AUDIO_QUALITY_MEDIUM":
      return text.listen.audioQualityOptions.medium;
    default:
      return "";
  }
}
