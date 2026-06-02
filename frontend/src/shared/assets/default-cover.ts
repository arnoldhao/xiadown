export const DEFAULT_COVER_IMAGE_URL = "/app_default.png";

export const COMPLETED_DEFAULT_COVER_IMAGE_URLS = {
  video: "/completed-defaults/video.jpg",
  audio: "/completed-defaults/audio.jpg",
  media: "/completed-defaults/media.jpg",
  videoSubtitle: "/completed-defaults/video-subtitle.jpg",
  audioSubtitle: "/completed-defaults/audio-subtitle.jpg",
  mediaSubtitle: "/completed-defaults/media-subtitle.jpg",
  subtitle: "/completed-defaults/subtitle.jpg",
  image: "/completed-defaults/image.jpg",
  live: "/completed-defaults/live.jpg",
  manifest: "/completed-defaults/manifest.jpg",
  api: "/completed-defaults/api.jpg",
  document: "/completed-defaults/document.jpg",
  font: "/completed-defaults/font.jpg",
  archive: "/completed-defaults/archive.jpg",
  mixed: "/completed-defaults/mixed.jpg",
  other: "/completed-defaults/other.jpg",
} as const;

export const LISTEN_DEFAULT_COVER_IMAGE_URL: string =
  COMPLETED_DEFAULT_COVER_IMAGE_URLS.audio;

export type CompletedDefaultCoverImageKey =
  keyof typeof COMPLETED_DEFAULT_COVER_IMAGE_URLS;
