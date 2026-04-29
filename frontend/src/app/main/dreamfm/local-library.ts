import {
type AudioSrc
} from "@vidstack/react";
import * as React from "react";

import { DEFAULT_COVER_IMAGE_URL } from "@/shared/assets/default-cover";
import type { LibraryDTO } from "@/shared/contracts/library";
import {
buildAssetPreviewURL,
extractExtensionFromPath,
getPathBaseName,
stripPathExtension,
} from "@/shared/utils/resourceHelpers";

import { AUDIO_MIME_BY_EXTENSION } from "@/app/main/dreamfm/catalog";
import type { DreamFMLocalItem,DreamFMOnlineItem } from "@/app/main/dreamfm/types";

type DreamFMLocalTrackDTO = {
  id?: string;
  fileId?: string;
  libraryId?: string;
  title?: string;
  author?: string;
  localPath?: string;
  coverLocalPath?: string;
  durationMs?: number;
  availability?: string;
};

type DreamFMLocalTrackResponseDTO = {
  items?: DreamFMLocalTrackDTO[];
};

export type DreamFMLocalTrackIndexState = {
  tracks: DreamFMLocalItem[];
  loading: boolean;
  refreshing: boolean;
  clearingMissing: boolean;
  refresh: () => Promise<void>;
  clearMissing: () => Promise<void>;
};

export function useDreamFMLocalTracks(
  libraries: LibraryDTO[],
  httpBaseURL: string,
): DreamFMLocalTrackIndexState {
  const [tracks, setTracks] = React.useState<DreamFMLocalItem[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [refreshing, setRefreshing] = React.useState(false);
  const [clearingMissing, setClearingMissing] = React.useState(false);
  const libraryVersion = React.useMemo(
    () => libraries.map((library) => `${library.id}:${library.updatedAt}`).join("|"),
    [libraries],
  );

  const loadTracks = React.useCallback(
    async (signal?: AbortSignal) => {
      const baseURL = normalizeDreamFMHTTPBaseURL(httpBaseURL);
      if (!baseURL) {
        setTracks([]);
        setLoading(false);
        return;
      }
      const response = await fetch(`${baseURL}/api/dreamfm/local`, {
        method: "GET",
        signal,
        headers: { Accept: "application/json" },
      });
      if (!response.ok) {
        throw new Error(`dreamfm local failed: ${response.status}`);
      }
      const payload = (await response.json()) as DreamFMLocalTrackResponseDTO;
      setTracks(
        (payload.items ?? [])
          .filter((item) => (item.availability ?? "available") === "available")
          .map((item) => mapDreamFMLocalTrackDTO(item, baseURL)),
      );
      setLoading(false);
    },
    [httpBaseURL],
  );

  React.useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    loadTracks(controller.signal).catch(() => {
      if (!controller.signal.aborted) {
        setTracks([]);
        setLoading(false);
      }
    });
    return () => controller.abort();
  }, [libraryVersion, loadTracks]);

  const refresh = React.useCallback(async () => {
    const baseURL = normalizeDreamFMHTTPBaseURL(httpBaseURL);
    if (!baseURL || refreshing) {
      return;
    }
    setRefreshing(true);
    try {
      const response = await fetch(`${baseURL}/api/dreamfm/local/refresh`, {
        method: "POST",
        headers: { Accept: "application/json" },
      });
      if (!response.ok) {
        throw new Error(`dreamfm local refresh failed: ${response.status}`);
      }
      await loadTracks();
    } finally {
      setRefreshing(false);
    }
  }, [httpBaseURL, loadTracks, refreshing]);

  const clearMissing = React.useCallback(async () => {
    const baseURL = normalizeDreamFMHTTPBaseURL(httpBaseURL);
    if (!baseURL || clearingMissing) {
      return;
    }
    setClearingMissing(true);
    try {
      const response = await fetch(`${baseURL}/api/dreamfm/local/clear-missing`, {
        method: "POST",
        headers: { Accept: "application/json" },
      });
      if (!response.ok) {
        throw new Error(`dreamfm local clear failed: ${response.status}`);
      }
      await loadTracks();
    } finally {
      setClearingMissing(false);
    }
  }, [clearingMissing, httpBaseURL, loadTracks]);

  return { tracks, loading, refreshing, clearingMissing, refresh, clearMissing };
}

function normalizeDreamFMHTTPBaseURL(value: string) {
  return value.trim().replace(/\/+$/, "");
}

function mapDreamFMLocalTrackDTO(item: DreamFMLocalTrackDTO, baseURL: string): DreamFMLocalItem {
  const path = item.localPath?.trim() ?? "";
  const fileTitle = stripPathExtension(firstTrimmedValue(getPathBaseName(path), path)).trim();
  const title = cleanDreamFMLocalTrackTitle(
    firstTrimmedValue(item.title, fileTitle, item.fileId, path),
  );
  const author = firstTrimmedValue(item.author);
  const lyricsFields = resolveDreamFMLocalLyricsFields(title, author, fileTitle);
  const coverURL =
    buildAssetPreviewURL(baseURL, item.coverLocalPath?.trim() ?? "") ||
    DEFAULT_COVER_IMAGE_URL;
  return {
    id: firstTrimmedValue(item.id, item.fileId, path),
    title,
    author,
    lyricsTitle: lyricsFields.title,
    lyricsArtist: lyricsFields.artist,
    path,
    previewURL: buildAssetPreviewURL(baseURL, path),
    durationLabel: formatDurationMs(item.durationMs),
    coverURL,
  };
}

function cleanDreamFMLocalTrackTitle(value: string) {
  const trimmed = value.trim();
  if (!trimmed) {
    return "";
  }
  const extension = extractExtensionFromPath(trimmed);
  if (extension && AUDIO_MIME_BY_EXTENSION[extension]) {
    return stripPathExtension(trimmed).trim() || trimmed;
  }
  return trimmed;
}

function resolveDreamFMLocalLyricsFields(
  title: string,
  author: string,
  fileTitle: string,
) {
  const cleanTitle = cleanDreamFMLocalTrackTitle(title || fileTitle);
  const cleanAuthor = author.trim();
  if (cleanAuthor) {
    return { title: cleanTitle, artist: cleanAuthor };
  }
  const split = splitDreamFMLocalArtistTitle(cleanTitle);
  if (split) {
    return split;
  }
  return { title: cleanTitle, artist: "" };
}

function splitDreamFMLocalArtistTitle(value: string) {
  const separators = [" - ", " – ", " — "];
  for (const separator of separators) {
    const index = value.indexOf(separator);
    if (index <= 0) {
      continue;
    }
    const artist = value.slice(0, index).trim();
    const title = value.slice(index + separator.length).trim();
    if (artist && title) {
      return { title, artist };
    }
  }
  return null;
}

export function normalizeSearch(value: string) {
  return value.trim().toLowerCase();
}

export function matchesQuery(query: string, values: string[]) {
  if (!query) {
    return true;
  }
  return values.some((value) => value.toLowerCase().includes(query));
}

export function firstTrimmedValue(...values: Array<string | undefined | null>) {
  for (const value of values) {
    const trimmedValue = value?.trim() ?? "";
    if (trimmedValue) {
      return trimmedValue;
    }
  }
  return "";
}

export function formatDurationMs(durationMs?: number) {
  if (!durationMs || durationMs <= 0) {
    return "";
  }
  const totalSeconds = Math.max(0, Math.round(durationMs / 1000));
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  if (hours > 0) {
    return `${hours}:${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`;
  }
  return `${minutes}:${String(seconds).padStart(2, "0")}`;
}

export function formatProgressSeconds(seconds?: number) {
  return formatDurationMs((seconds || 0) * 1000) || "0:00";
}

export function clampVolume(value: number) {
  if (!Number.isFinite(value)) {
    return 1;
  }
  return Math.min(1, Math.max(0, value));
}

export function resolveAudioSource(mediaUrl: string, path: string): string | AudioSrc {
  const extension = extractExtensionFromPath(path || mediaUrl).toLowerCase();
  const type = AUDIO_MIME_BY_EXTENSION[extension];
  return type ? { src: mediaUrl, type: type as AudioSrc["type"] } : mediaUrl;
}

export function resolveDreamFMLiveSelectionId(
  items: DreamFMOnlineItem[],
  id: string,
) {
  const trimmedId = id.trim();
  if (trimmedId) {
    const match = items.find(
      (item) => item.id === trimmedId || item.videoId === trimmedId,
    );
    if (match) {
      return match.id;
    }
  }
  return items[0]?.id ?? "";
}

export function resolveQueueIndex<T extends { id: string }>(items: T[], id: string) {
  return Math.max(
    0,
    items.findIndex((item) => item.id === id),
  );
}

export function resolveAdjacentIndex(
  length: number,
  currentIndex: number,
  direction: -1 | 1,
) {
  if (length <= 0) {
    return -1;
  }
  return (currentIndex + direction + length) % length;
}

export function resolveRandomIndex(length: number, currentIndex: number) {
  if (length <= 1) {
    return 0;
  }
  const randomIndex = Math.floor(Math.random() * (length - 1));
  return randomIndex >= currentIndex ? randomIndex + 1 : randomIndex;
}
