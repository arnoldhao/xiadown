import {
type AudioSrc
} from "@vidstack/react";
import * as React from "react";

import { LISTEN_DEFAULT_COVER_IMAGE_URL } from "@/shared/assets/default-cover";
import type { LibraryDTO } from "@/shared/contracts/library";
import {
buildAssetPreviewURL,
extractExtensionFromPath,
getPathBaseName,
stripPathExtension,
} from "@/shared/utils/resourceHelpers";

import { AUDIO_MIME_BY_EXTENSION } from "@/app/main/listen/catalog";
import type { ListenLocalItem,ListenOnlineItem } from "@/app/main/listen/types";

type ListenLocalTrackDTO = {
  id?: string;
  fileId?: string;
  libraryId?: string;
  title?: string;
  author?: string;
  localPath?: string;
  coverLocalPath?: string;
  durationMs?: number;
  modTimeUnix?: number;
  availability?: string;
};

type ListenLocalTrackResponseDTO = {
  items?: ListenLocalTrackDTO[];
};

export type ListenLocalTrackIndexState = {
  tracks: ListenLocalItem[];
  loading: boolean;
  refreshing: boolean;
  clearingMissing: boolean;
  refresh: () => Promise<void>;
  clearMissing: () => Promise<number>;
};

export function useListenLocalTracks(
  libraries: LibraryDTO[],
  httpBaseURL: string,
): ListenLocalTrackIndexState {
  const [tracks, setTracks] = React.useState<ListenLocalItem[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [refreshing, setRefreshing] = React.useState(false);
  const [clearingMissing, setClearingMissing] = React.useState(false);
  const libraryVersion = React.useMemo(
    () => libraries.map((library) => `${library.id}:${library.updatedAt}`).join("|"),
    [libraries],
  );

  const loadTracks = React.useCallback(
    async (signal?: AbortSignal) => {
      const baseURL = normalizeListenHTTPBaseURL(httpBaseURL);
      if (!baseURL) {
        setTracks([]);
        setLoading(false);
        return;
      }
      const response = await fetch(`${baseURL}/api/listen/local`, {
        method: "GET",
        signal,
        headers: { Accept: "application/json" },
      });
      if (!response.ok) {
        throw new Error(`listen local failed: ${response.status}`);
      }
      const payload = (await response.json()) as ListenLocalTrackResponseDTO;
      setTracks(
        (payload.items ?? [])
          .filter((item) => (item.availability ?? "available") === "available")
          .map((item) => mapListenLocalTrackDTO(item, baseURL)),
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
    const baseURL = normalizeListenHTTPBaseURL(httpBaseURL);
    if (!baseURL || refreshing) {
      return;
    }
    setRefreshing(true);
    try {
      const response = await fetch(`${baseURL}/api/listen/local/refresh`, {
        method: "POST",
        headers: { Accept: "application/json" },
      });
      if (!response.ok) {
        throw new Error(`listen local refresh failed: ${response.status}`);
      }
      await loadTracks();
    } finally {
      setRefreshing(false);
    }
  }, [httpBaseURL, loadTracks, refreshing]);

  const clearMissing = React.useCallback(async () => {
    const baseURL = normalizeListenHTTPBaseURL(httpBaseURL);
    if (!baseURL || clearingMissing) {
      return 0;
    }
    setClearingMissing(true);
    try {
      const response = await fetch(`${baseURL}/api/listen/local/clear-missing`, {
        method: "POST",
        headers: { Accept: "application/json" },
      });
      if (!response.ok) {
        throw new Error(`listen local clear failed: ${response.status}`);
      }
      const payload = (await response.json()) as { removed?: number };
      await loadTracks();
      return Number.isFinite(payload.removed) ? Number(payload.removed) : 0;
    } finally {
      setClearingMissing(false);
    }
  }, [clearingMissing, httpBaseURL, loadTracks]);

  return { tracks, loading, refreshing, clearingMissing, refresh, clearMissing };
}

function normalizeListenHTTPBaseURL(value: string) {
  return value.trim().replace(/\/+$/, "");
}

function mapListenLocalTrackDTO(item: ListenLocalTrackDTO, baseURL: string): ListenLocalItem {
  const path = item.localPath?.trim() ?? "";
  const fileTitle = stripPathExtension(firstTrimmedValue(getPathBaseName(path), path)).trim();
  const title = cleanListenLocalTrackTitle(
    firstTrimmedValue(item.title, fileTitle, item.fileId, path),
  );
  const author = firstTrimmedValue(item.author);
  const lyricsFields = resolveListenLocalLyricsFields(title, author, fileTitle);
  const coverURL =
    buildAssetPreviewURL(baseURL, item.coverLocalPath?.trim() ?? "") ||
    LISTEN_DEFAULT_COVER_IMAGE_URL;
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
    modTimeUnix:
      typeof item.modTimeUnix === "number" && Number.isFinite(item.modTimeUnix)
        ? Math.max(0, item.modTimeUnix)
        : 0,
  };
}

function cleanListenLocalTrackTitle(value: string) {
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

function resolveListenLocalLyricsFields(
  title: string,
  author: string,
  fileTitle: string,
) {
  const cleanTitle = cleanListenLocalTrackTitle(title || fileTitle);
  const cleanAuthor = author.trim();
  if (cleanAuthor) {
    return { title: cleanTitle, artist: cleanAuthor };
  }
  const split = splitListenLocalArtistTitle(cleanTitle);
  if (split) {
    return split;
  }
  return { title: cleanTitle, artist: "" };
}

function splitListenLocalArtistTitle(value: string) {
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

export function resolveListenLiveSelectionId(
  items: ListenOnlineItem[],
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
