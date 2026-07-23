import * as React from "react";

import {
  playbackCoordinator,
  subscribePlaybackCoordinator,
} from "@/shared/playback/client";
import { usePlaybackCoordinatorStore } from "@/shared/playback/store";

let playbackSyncConsumers = 0;
let stopPlaybackSync: (() => void) | null = null;

function beginPlaybackCoordinatorSync() {
  playbackSyncConsumers += 1;
  if (playbackSyncConsumers === 1) {
    stopPlaybackSync = subscribePlaybackCoordinator();
    void playbackCoordinator.snapshot().catch((error) => {
      usePlaybackCoordinatorStore.getState().setError(error);
    });
  }
  return () => {
    playbackSyncConsumers = Math.max(0, playbackSyncConsumers - 1);
    if (playbackSyncConsumers === 0) {
      stopPlaybackSync?.();
      stopPlaybackSync = null;
    }
  };
}

export function usePlaybackCoordinator(enabled = true) {
  const snapshot = usePlaybackCoordinatorStore((state) => state.snapshot);
  const hydrated = usePlaybackCoordinatorStore((state) => state.hydrated);
  const error = usePlaybackCoordinatorStore((state) => state.error);

  React.useEffect(() => {
    if (!enabled) {
      return;
    }
    return beginPlaybackCoordinatorSync();
  }, [enabled]);

  return { snapshot, hydrated, error, commands: playbackCoordinator };
}
