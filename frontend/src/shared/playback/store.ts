import { create } from "zustand";

import { normalizePlaybackSnapshot } from "@/shared/playback/normalize";
import {
  EMPTY_PLAYBACK_SNAPSHOT,
  type PlaybackSnapshot,
} from "@/shared/playback/types";

type PlaybackCoordinatorStore = {
  snapshot: PlaybackSnapshot;
  hydrated: boolean;
  error: string;
  applySnapshot: (value: unknown) => PlaybackSnapshot;
  setError: (error: unknown) => void;
  reset: () => void;
};

function errorMessage(error: unknown) {
  if (error instanceof Error) {
    return error.message;
  }
  return typeof error === "string" ? error : "Playback coordinator unavailable";
}

export const usePlaybackCoordinatorStore = create<PlaybackCoordinatorStore>(
  (set, get) => ({
    snapshot: EMPTY_PLAYBACK_SNAPSHOT,
    hydrated: false,
    error: "",
    applySnapshot: (value) => {
      const snapshot = normalizePlaybackSnapshot(value);
      const current = get().snapshot;
      if (snapshot.version >= current.version) {
        set({ snapshot, hydrated: true, error: "" });
        return snapshot;
      }
      return current;
    },
    setError: (error) => set({ error: errorMessage(error), hydrated: true }),
    reset: () =>
      set({ snapshot: EMPTY_PLAYBACK_SNAPSHOT, hydrated: false, error: "" }),
  }),
);
