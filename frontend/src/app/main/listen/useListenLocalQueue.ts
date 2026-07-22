import * as React from "react";

import { resolveListenLocalPlayableQueue } from "@/app/main/listen/local-format";
import {
  clearListenLocalQueueKeepingSelected,
  moveListenLocalQueueItem,
  pruneListenLocalQueueIds,
  removeListenLocalQueueItem,
  shouldClearListenLocalSelection,
  type ListenLocalQueueEditResult,
  type ListenLocalQueueSnapshot,
} from "@/app/main/listen/local-queue";
import {
  buildListenLocalPlaybackQueueIds,
  resolveListenLocalPlaybackQueue,
} from "@/app/main/listen/local-workspace";
import type {
  ListenLocalItem,
  ListenMode,
  ListenPlayerCommand,
} from "@/app/main/listen/types";

type UseListenLocalQueueOptions = {
  tracks: readonly ListenLocalItem[];
  initialQueueIds: string[] | null;
  initialSelectedId: string;
  loading: boolean;
  error: string;
  playing: boolean;
  setPlaying: React.Dispatch<React.SetStateAction<boolean>>;
  setPlaybackMode: React.Dispatch<React.SetStateAction<ListenMode>>;
  setPlayerCommand: React.Dispatch<
    React.SetStateAction<ListenPlayerCommand | null>
  >;
  clearForwardSkipNavigationStack: () => void;
};

export function useListenLocalQueue(options: UseListenLocalQueueOptions) {
  const {
    tracks,
    initialQueueIds,
    initialSelectedId,
    loading,
    error,
    playing,
    setPlaying,
    setPlaybackMode,
    setPlayerCommand,
    clearForwardSkipNavigationStack,
  } = options;
  const [selectedLocalId, setSelectedLocalId] = React.useState(
    initialSelectedId,
  );
  const [localPlaybackQueueIds, setLocalPlaybackQueueIds] = React.useState<
    string[] | null
  >(() => initialQueueIds);
  const [undoHistory, setUndoHistory] = React.useState<
    ListenLocalQueueSnapshot[]
  >([]);
  const [redoHistory, setRedoHistory] = React.useState<
    ListenLocalQueueSnapshot[]
  >([]);
  const localPlayableTracks = React.useMemo(
    () => resolveListenLocalPlayableQueue(tracks),
    [tracks],
  );
  const localPlaybackQueue = React.useMemo(
    () =>
      resolveListenLocalPlaybackQueue(
        localPlayableTracks,
        localPlaybackQueueIds,
      ),
    [localPlaybackQueueIds, localPlayableTracks],
  );

  React.useEffect(() => {
    if (loading || error) {
      return;
    }
    const pruned = pruneListenLocalQueueIds(
      localPlaybackQueueIds,
      new Set(localPlayableTracks.map((track) => track.id)),
    );
    const changed =
      localPlaybackQueueIds === null || pruned === null
        ? localPlaybackQueueIds !== pruned
        : localPlaybackQueueIds.length !== pruned.length ||
          localPlaybackQueueIds.some((id, index) => id !== pruned[index]);
    if (!changed) {
      return;
    }
    setLocalPlaybackQueueIds(pruned);
    setUndoHistory([]);
    setRedoHistory([]);
  }, [error, loading, localPlayableTracks, localPlaybackQueueIds]);

  React.useEffect(() => {
    if (
      shouldClearListenLocalSelection({
        selectedId: selectedLocalId,
        loading,
        error,
        playableIds: new Set(localPlaybackQueue.map((item) => item.id)),
      })
    ) {
      setSelectedLocalId("");
    }
  }, [error, loading, localPlaybackQueue, selectedLocalId]);

  const selectLocalQueueTrack = React.useCallback(
    (
      item: { id: string },
      selectOptions: {
        forcePlay?: boolean;
        preserveForwardSkipStack?: boolean;
      } = { forcePlay: true },
    ) => {
      if (!item.id) {
        return;
      }
      const playableItem = localPlayableTracks.find(
        (track) => track.id === item.id,
      );
      if (!playableItem) {
        return;
      }
      if (
        selectOptions.forcePlay !== false &&
        selectOptions.preserveForwardSkipStack !== true
      ) {
        clearForwardSkipNavigationStack();
      }
      setSelectedLocalId(item.id);
      setPlaybackMode("linger");
      if (playing || selectOptions.forcePlay) {
        setPlayerCommand({
          id: Date.now(),
          command: "play",
        });
      }
    },
    [
      clearForwardSkipNavigationStack,
      localPlayableTracks,
      playing,
      setPlaybackMode,
      setPlayerCommand,
    ],
  );

  const applyLocalQueueSelection = React.useCallback(
    (nextSelectedId: string) => {
      const changed = nextSelectedId !== selectedLocalId;
      setSelectedLocalId(nextSelectedId);
      if (!nextSelectedId) {
        setPlaying(false);
        return;
      }
      if (changed && playing) {
        setPlayerCommand({
          id: Date.now(),
          command: "play",
        });
      }
    },
    [playing, selectedLocalId, setPlayerCommand, setPlaying],
  );

  const commitLocalQueueEdit = React.useCallback(
    (result: ListenLocalQueueEditResult) => {
      if (!result.changed) {
        return;
      }
      setUndoHistory((history) =>
        [
          ...history,
          {
            queueIds:
              localPlaybackQueueIds === null
                ? null
                : [...localPlaybackQueueIds],
            selectedId: selectedLocalId,
          },
        ].slice(-20),
      );
      setRedoHistory([]);
      setLocalPlaybackQueueIds(result.queueIds);
      applyLocalQueueSelection(result.selectedId);
      clearForwardSkipNavigationStack();
    },
    [
      applyLocalQueueSelection,
      clearForwardSkipNavigationStack,
      localPlaybackQueueIds,
      selectedLocalId,
    ],
  );

  const clearLocalQueue = React.useCallback(() => {
    commitLocalQueueEdit(
      clearListenLocalQueueKeepingSelected(
        localPlaybackQueue,
        selectedLocalId,
      ),
    );
  }, [commitLocalQueueEdit, localPlaybackQueue, selectedLocalId]);

  const removeLocalQueueItem = React.useCallback(
    (item: { id: string }) => {
      commitLocalQueueEdit(
        removeListenLocalQueueItem(
          localPlaybackQueue,
          selectedLocalId,
          item.id,
        ),
      );
    },
    [commitLocalQueueEdit, localPlaybackQueue, selectedLocalId],
  );

  const moveLocalQueueItem = React.useCallback(
    (item: { id: string }, direction: -1 | 1) => {
      commitLocalQueueEdit(
        moveListenLocalQueueItem(
          localPlaybackQueue,
          selectedLocalId,
          item.id,
          direction,
        ),
      );
    },
    [commitLocalQueueEdit, localPlaybackQueue, selectedLocalId],
  );

  const restoreLocalQueueSnapshot = React.useCallback(
    (snapshot: ListenLocalQueueSnapshot) => {
      const validIds = new Set(localPlayableTracks.map((track) => track.id));
      const queueIds = pruneListenLocalQueueIds(snapshot.queueIds, validIds);
      const restoredQueue = resolveListenLocalPlaybackQueue(
        localPlayableTracks,
        queueIds,
      );
      const selectedId = restoredQueue.some(
        (track) => track.id === snapshot.selectedId,
      )
        ? snapshot.selectedId
        : snapshot.selectedId
          ? restoredQueue[0]?.id ?? ""
          : "";
      setLocalPlaybackQueueIds(queueIds);
      applyLocalQueueSelection(selectedId);
      clearForwardSkipNavigationStack();
    },
    [
      applyLocalQueueSelection,
      clearForwardSkipNavigationStack,
      localPlayableTracks,
    ],
  );

  const undoLocalQueueEdit = React.useCallback(() => {
    const target = undoHistory[undoHistory.length - 1];
    if (!target) {
      return;
    }
    setUndoHistory((history) => history.slice(0, -1));
    setRedoHistory((history) =>
      [
        ...history,
        {
          queueIds:
            localPlaybackQueueIds === null
              ? null
              : [...localPlaybackQueueIds],
          selectedId: selectedLocalId,
        },
      ].slice(-20),
    );
    restoreLocalQueueSnapshot(target);
  }, [
    localPlaybackQueueIds,
    restoreLocalQueueSnapshot,
    selectedLocalId,
    undoHistory,
  ]);

  const redoLocalQueueEdit = React.useCallback(() => {
    const target = redoHistory[redoHistory.length - 1];
    if (!target) {
      return;
    }
    setRedoHistory((history) => history.slice(0, -1));
    setUndoHistory((history) =>
      [
        ...history,
        {
          queueIds:
            localPlaybackQueueIds === null
              ? null
              : [...localPlaybackQueueIds],
          selectedId: selectedLocalId,
        },
      ].slice(-20),
    );
    restoreLocalQueueSnapshot(target);
  }, [
    localPlaybackQueueIds,
    redoHistory,
    restoreLocalQueueSnapshot,
    selectedLocalId,
  ]);

  const playLocalBrowseTrack = React.useCallback(
    (item: { id: string }, queue: Array<{ id: string }>) => {
      const playableQueueIds = new Set(
        localPlayableTracks.map((track) => track.id),
      );
      if (!playableQueueIds.has(item.id)) {
        return;
      }
      setUndoHistory([]);
      setRedoHistory([]);
      setLocalPlaybackQueueIds(
        buildListenLocalPlaybackQueueIds(
          item.id,
          queue.filter((track) => playableQueueIds.has(track.id)),
        ),
      );
      selectLocalQueueTrack(item, { forcePlay: true });
    },
    [localPlayableTracks, selectLocalQueueTrack],
  );

  return {
    selectedLocalId,
    setSelectedLocalId,
    localPlaybackQueueIds,
    localPlaybackQueue,
    localQueueCanUndo: undoHistory.length > 0,
    localQueueCanRedo: redoHistory.length > 0,
    selectLocalQueueTrack,
    clearLocalQueue,
    removeLocalQueueItem,
    moveLocalQueueItem,
    undoLocalQueueEdit,
    redoLocalQueueEdit,
    playLocalBrowseTrack,
  };
}
