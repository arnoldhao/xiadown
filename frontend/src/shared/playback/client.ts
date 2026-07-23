import { Events } from "@wailsio/runtime";

import * as PlaybackCoordinatorBinding from "../../../bindings/xiadown/internal/presentation/wails/playbackcoordinatorhandler";
import { PlaybackSessionRequest as GeneratedPlaybackSessionRequest } from "../../../bindings/xiadown/internal/application/listenplayback/models";

import { usePlaybackCoordinatorStore } from "@/shared/playback/store";
import type {
  PlaybackSessionRequest,
  PlaybackSnapshot,
} from "@/shared/playback/types";

export const PLAYBACK_COORDINATOR_SNAPSHOT_EVENT = "playback:snapshot";
export const PLAYBACK_COORDINATOR_SERVICE =
  "xiadown/internal/presentation/wails.PlaybackCoordinatorHandler";

async function commitSnapshot(request: PromiseLike<unknown>) {
  const raw = await request;
  return usePlaybackCoordinatorStore.getState().applySnapshot(raw);
}

function generatedSessionRequest(
  request: PlaybackSessionRequest,
  focus: "persistent" | "transient_preview",
) {
  return GeneratedPlaybackSessionRequest.createFrom({
    ...request,
    focus,
  });
}

export const playbackCoordinator = {
  snapshot: () => commitSnapshot(PlaybackCoordinatorBinding.Snapshot()),
  startPersistent: (request: PlaybackSessionRequest) =>
    commitSnapshot(
      PlaybackCoordinatorBinding.StartPersistent(
        generatedSessionRequest(request, "persistent"),
      ),
    ),
  startTransientPreview: (request: PlaybackSessionRequest) =>
    commitSnapshot(
      PlaybackCoordinatorBinding.StartTransientPreview(
        generatedSessionRequest(request, "transient_preview"),
      ),
    ),
  closeSession: (sessionId: string) =>
    commitSnapshot(PlaybackCoordinatorBinding.CloseSession({ sessionId })),
  play: () => commitSnapshot(PlaybackCoordinatorBinding.Play()),
  pause: () => commitSnapshot(PlaybackCoordinatorBinding.Pause()),
  stop: () => commitSnapshot(PlaybackCoordinatorBinding.Stop()),
  seek: (seconds: number) =>
    commitSnapshot(PlaybackCoordinatorBinding.Seek({ seconds })),
  setVolume: (volume: number, muted: boolean) =>
    commitSnapshot(PlaybackCoordinatorBinding.SetVolume({ volume, muted })),
  previous: () => commitSnapshot(PlaybackCoordinatorBinding.Previous()),
  next: () => commitSnapshot(PlaybackCoordinatorBinding.Next()),
};

export function subscribePlaybackCoordinator(
  listener?: (snapshot: PlaybackSnapshot) => void,
) {
  return Events.On(PLAYBACK_COORDINATOR_SNAPSHOT_EVENT, (event: unknown) => {
    const snapshot = usePlaybackCoordinatorStore
      .getState()
      .applySnapshot(event);
    listener?.(snapshot);
  });
}
