import {
  listenArtistCountFromLabelParts,
  type ListenArtistLabelPart,
} from "@/app/main/listen/playback-helpers";
import type {
  ListenOnlineItem,
  ListenTrackArtist,
} from "@/app/main/listen/types";

export function listenArtistBrowseTrack(
  track: ListenOnlineItem,
  artist: ListenTrackArtist,
  labelParts: ListenArtistLabelPart[],
): ListenOnlineItem | null {
  const artistName = artist.name.trim();
  const artistBrowseId = artist.browseId?.trim() ?? "";
  if (!artistName) {
    return null;
  }
  const linkedArtist =
    (artistBrowseId
      ? track.artists?.find(
          (candidate) => candidate.browseId?.trim() === artistBrowseId,
        ) ?? null
      : null) ?? listenTrackArtistByName(track.artists, artistName);
  const resolvedArtist =
    linkedArtist || artistBrowseId
      ? {
          name: linkedArtist?.name.trim() || artistName,
          browseId:
            artistBrowseId || linkedArtist?.browseId?.trim() || undefined,
          thumbnailUrl:
            artist.thumbnailUrl?.trim() ||
            linkedArtist?.thumbnailUrl?.trim() ||
            undefined,
        }
      : null;
  if (resolvedArtist) {
    return {
      ...track,
      channel: resolvedArtist.name,
      artists: [resolvedArtist],
      artistBrowseId: resolvedArtist.browseId,
      artistSource: resolvedArtist.browseId ? "api-linked" : undefined,
      thumbnailUrl: resolvedArtist.thumbnailUrl,
    };
  }
  const keepOriginalArtistLink =
    (listenArtistCountFromLabelParts(labelParts) <= 1 &&
      artistName === track.channel.trim()) ||
    (track.artistSource === "api-linked-multiple" &&
      artistName ===
        labelParts.find((part) => part.kind === "artist")?.text.trim());
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
