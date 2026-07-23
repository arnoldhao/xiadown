export const DEFAULT_COVER_IMAGE_URL = "/app_default.png";

/**
 * Semantic Library artwork tokens. These values are deliberately not asset
 * URLs: LibraryArtwork resolves them to the current theme's live SVG/CSS
 * placeholder without issuing a network request.
 */
export const COMPLETED_DEFAULT_COVER_IMAGE_URLS = {
  video: "xiadown-library-default:video",
  audio: "xiadown-library-default:audio",
  media: "xiadown-library-default:media",
  videoSubtitle: "xiadown-library-default:video-subtitle",
  audioSubtitle: "xiadown-library-default:audio-subtitle",
  mediaSubtitle: "xiadown-library-default:media-subtitle",
  subtitle: "xiadown-library-default:subtitle",
  image: "xiadown-library-default:image",
  live: "xiadown-library-default:live",
  manifest: "xiadown-library-default:manifest",
  api: "xiadown-library-default:api",
  document: "xiadown-library-default:document",
  font: "xiadown-library-default:font",
  archive: "xiadown-library-default:archive",
  mixed: "xiadown-library-default:mixed",
  other: "xiadown-library-default:other",
} as const;

/**
 * Player-only music artwork. React player surfaces render a theme-aware live
 * equivalent, while URL-only boundaries (notifications, video posters and
 * ambient images) can safely consume this bundled SVG.
 */
export const LISTEN_DEFAULT_COVER_IMAGE_URL = "/listen_default_cover.svg";

export function isListenDefaultCoverImageURL(value: string | null | undefined) {
  return value?.trim() === LISTEN_DEFAULT_COVER_IMAGE_URL;
}

export type CompletedDefaultCoverImageKey =
  keyof typeof COMPLETED_DEFAULT_COVER_IMAGE_URLS;
