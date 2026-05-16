export const LISTEN_MUSIC_VIDEO_TYPE_OMV = "MUSIC_VIDEO_TYPE_OMV";
export const LISTEN_MUSIC_VIDEO_TYPE_ATV = "MUSIC_VIDEO_TYPE_ATV";

export function hasListenMusicVideoContent(musicVideoType: string | undefined) {
  return musicVideoType?.trim() === LISTEN_MUSIC_VIDEO_TYPE_OMV;
}

export function isListenMusicVideoKnownNoVideo(
  musicVideoType: string | undefined,
) {
  return musicVideoType?.trim() === LISTEN_MUSIC_VIDEO_TYPE_ATV;
}

export function doesListenThumbnailSuggestVideoContent(
  videoId: string | undefined,
  thumbnailUrl: string | undefined,
) {
  const id = videoId?.trim().toLowerCase();
  const url = thumbnailUrl?.trim().toLowerCase();
  if (!id || !url) {
    return false;
  }
  return (
    url.includes(`i.ytimg.com/vi/${id}/`) ||
    url.includes(`img.youtube.com/vi/${id}/`) ||
    url.includes(`i.ytimg.com/vi_webp/${id}/`) ||
    url.includes(`img.youtube.com/vi_webp/${id}/`)
  );
}
