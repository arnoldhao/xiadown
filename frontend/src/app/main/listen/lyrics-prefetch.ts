import * as React from "react";

import { callListenTrackLyricsCached } from "@/app/main/listen/lyrics-api";
import type { ListenOnlineItem } from "@/app/main/listen/types";

const LISTEN_LYRICS_PREFETCH_DELAY_MS = 1400;

export function canPrefetchListenTrackLyrics(
  enabled: boolean,
  track: Pick<ListenOnlineItem, "videoId" | "title">,
) {
  return enabled && Boolean(track.videoId.trim() && track.title.trim());
}

export function useListenTrackLyricsPrefetch(options: {
  enabled: boolean;
  track: ListenOnlineItem;
  durationSeconds: number;
  language?: string;
  synced?: boolean;
}) {
  React.useEffect(() => {
    if (!canPrefetchListenTrackLyrics(options.enabled, options.track)) {
      return;
    }
    const timer = window.setTimeout(() => {
      void callListenTrackLyricsCached({
        track: options.track,
        durationSeconds: options.durationSeconds,
        language: options.language,
        synced: options.synced,
      }).catch(() => {});
    }, LISTEN_LYRICS_PREFETCH_DELAY_MS);
    return () => window.clearTimeout(timer);
  }, [
    options.durationSeconds,
    options.enabled,
    options.language,
    options.synced,
    options.track,
  ]);
}
