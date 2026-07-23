import {
  callListenLyricsCandidates,
  type ListenLyricsCandidateTrack,
} from "@/app/main/listen/lyrics-api";
import {
  mapListenLocalTrackDTO,
  normalizeListenHTTPBaseURL,
  type ListenLocalTrackDTO,
} from "@/app/main/listen/local-library";
import type {
  ListenLocalItem,
  ListenLyricsCandidate,
} from "@/app/main/listen/types";
import { forgetListenLyricsCacheVariants } from "@/app/main/listen/playback-helpers";

export type ListenLocalMetadataDraft = {
  title: string;
  author: string;
  album: string;
  albumArtist: string;
  genre: string;
  trackNumber: number;
  discNumber: number;
  year: number;
};

export const LISTEN_LOCAL_METADATA_INDEX_STALE_CODE =
  "metadata_committed_index_stale";

export class ListenLocalMetadataUpdateError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(
    message: string,
    options: {
      status: number;
      code: string;
    },
  ) {
    super(message);
    this.name = "ListenLocalMetadataUpdateError";
    this.status = options.status;
    this.code = options.code;
  }
}

export function isListenLocalMetadataCommittedIndexStale(
  error: unknown,
): error is ListenLocalMetadataUpdateError {
  return (
    error instanceof ListenLocalMetadataUpdateError &&
    error.code === LISTEN_LOCAL_METADATA_INDEX_STALE_CODE
  );
}

export function listenLocalMetadataDraft(
  track: ListenLocalItem,
): ListenLocalMetadataDraft {
  return {
    title: track.title.trim(),
    author: track.author.trim(),
    album: track.album.trim(),
    albumArtist: track.albumArtist.trim(),
    genre: track.genre.trim(),
    trackNumber: positiveMetadataInteger(track.trackNumber),
    discNumber: positiveMetadataInteger(track.discNumber),
    year: positiveMetadataInteger(track.year),
  };
}

export function localMetadataCandidateTrack(
  track: ListenLocalItem,
  draft: ListenLocalMetadataDraft,
): ListenLyricsCandidateTrack {
  const draftTitle = draft.title.trim();
  const draftArtist = draft.author.trim();
  const useFilenameIdentity =
    !track.author.trim() &&
    !draftArtist &&
    Boolean(track.lyricsArtist.trim());
  const title =
    useFilenameIdentity &&
    (!draftTitle || draftTitle === track.title.trim())
      ? track.lyricsTitle.trim()
      : draftTitle;
  return {
    lyricsId: `local:${track.id}`,
    title: title || track.lyricsTitle || track.title,
    artist:
      draftArtist || (useFilenameIdentity ? track.lyricsArtist : track.author),
    album: draft.album.trim(),
    localPath: track.path,
    durationSeconds: track.durationSeconds,
  };
}

export async function searchListenLocalMetadataCandidate(options: {
  track: ListenLocalItem;
  draft: ListenLocalMetadataDraft;
  language?: string;
}) {
  const candidates = await callListenLyricsCandidates({
    track: localMetadataCandidateTrack(options.track, options.draft),
    language: options.language,
  });
  return selectListenLocalMetadataCandidate(candidates);
}

export function selectListenLocalMetadataCandidate(
  candidates: ListenLyricsCandidate[],
) {
  return (
    candidates.find((candidate) => {
      if (!candidate.accepted || candidate.confidence < 78) {
        return false;
      }
      const strongTitle = candidate.titleScore >= 85;
      const corroborated =
        candidate.artistScore >= 55 ||
        candidate.albumScore >= 55 ||
        candidate.durationScore >= 80;
      return strongTitle && corroborated;
    }) ?? null
  );
}

export function applyListenLocalMetadataCandidate(
  draft: ListenLocalMetadataDraft,
  candidate: ListenLyricsCandidate,
): ListenLocalMetadataDraft {
  const candidateArtist = candidate.artist.trim();
  const shouldUpdateAlbumArtist =
    !draft.albumArtist.trim() ||
    equivalentListenLocalArtist(draft.albumArtist, draft.author);
  return {
    ...draft,
    title: candidate.title.trim() || draft.title,
    author: candidateArtist || draft.author,
    album: candidate.album?.trim() || draft.album,
    albumArtist:
      shouldUpdateAlbumArtist && candidateArtist
        ? candidateArtist
        : draft.albumArtist,
  };
}

function equivalentListenLocalArtist(left: string, right: string) {
  const normalize = (value: string) =>
    value
      .trim()
      .replace(/\s*[-–—]\s*topic\s*$/i, "")
      .trim()
      .toLocaleLowerCase();
  const normalizedLeft = normalize(left);
  return normalizedLeft !== "" && normalizedLeft === normalize(right);
}

export async function updateListenLocalTrackMetadata(options: {
  httpBaseURL: string;
  track: ListenLocalItem;
  draft: ListenLocalMetadataDraft;
}) {
  const baseURL = normalizeListenHTTPBaseURL(options.httpBaseURL);
  if (!baseURL) {
    throw new Error("Local music service is unavailable.");
  }
  const response = await fetch(`${baseURL}/api/listen/local/metadata`, {
    method: "PATCH",
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      fileId: options.track.id,
      title: options.draft.title.trim(),
      author: options.draft.author.trim(),
      album: options.draft.album.trim(),
      albumArtist: options.draft.albumArtist.trim(),
      genre: options.draft.genre.trim(),
      trackNumber: positiveMetadataInteger(options.draft.trackNumber),
      discNumber: positiveMetadataInteger(options.draft.discNumber),
      year: positiveMetadataInteger(options.draft.year),
    }),
  });
  if (!response.ok) {
    const details = await listenLocalMetadataErrorDetails(response);
    const code = details.code || "local_metadata_update_failed";
    if (code === LISTEN_LOCAL_METADATA_INDEX_STALE_CODE) {
      forgetListenLyricsCacheVariants(`local:${options.track.id}`);
    }
    throw new ListenLocalMetadataUpdateError(
      details.message || `Local metadata update failed: ${response.status}`,
      {
        status: response.status,
        code,
      },
    );
  }
  const updated = mapListenLocalTrackDTO(
    (await response.json()) as ListenLocalTrackDTO,
    baseURL,
  );
  forgetListenLyricsCacheVariants(`local:${options.track.id}`);
  return updated;
}

export function parseListenLocalMetadataNumber(value: string) {
  const parsed = Number.parseInt(value.trim(), 10);
  return Number.isFinite(parsed) && parsed > 0 ? Math.min(9999, parsed) : 0;
}

function positiveMetadataInteger(value: number) {
  return Number.isFinite(value) && value > 0
    ? Math.min(9999, Math.floor(value))
    : 0;
}

async function listenLocalMetadataErrorDetails(response: Response) {
  try {
    const payload = (await response.json()) as {
      error?: { code?: string; message?: string };
    };
    return {
      code: payload.error?.code?.trim() ?? "",
      message: payload.error?.message?.trim() ?? "",
    };
  } catch {
    return { code: "", message: "" };
  }
}
