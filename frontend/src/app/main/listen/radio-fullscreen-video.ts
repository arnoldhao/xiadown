import * as React from "react";

import { resolveListenRadioFullscreenVideoDecision } from "@/app/main/listen/playback-helpers";
import type { ListenMediaMode } from "@/app/main/listen/playback-ui";
import type { ListenPlayerPresentation } from "@/app/main/listen/types";

export function useListenRadioFullscreenVideoDefault(options: {
  presentation: ListenPlayerPresentation;
  workspaceFullscreen: boolean;
  active: boolean;
  enabled: boolean;
  live: boolean;
  trackKey: string;
  hasVideo: boolean;
  nativeVideoAvailable: boolean;
  queueOpen: boolean;
  mediaMode: ListenMediaMode;
  setQueueOpen: React.Dispatch<React.SetStateAction<boolean>>;
  setMediaMode: React.Dispatch<React.SetStateAction<ListenMediaMode>>;
}) {
  const attemptedTrackKeyRef = React.useRef("");
  React.useEffect(() => {
    const radioFullscreen =
      options.presentation === "fullscreen" &&
      options.workspaceFullscreen &&
      options.live;
    if (!radioFullscreen) {
      attemptedTrackKeyRef.current = "";
      return;
    }
    const decision = resolveListenRadioFullscreenVideoDecision({
      ...options,
      attemptedTrackKey: attemptedTrackKeyRef.current,
    });
    if (decision === "fallback") {
      options.setMediaMode("cover");
    } else if (decision === "open") {
      attemptedTrackKeyRef.current = options.trackKey;
      options.setQueueOpen(false);
      options.setMediaMode("video");
    }
  }, [
    options.active,
    options.enabled,
    options.hasVideo,
    options.live,
    options.mediaMode,
    options.nativeVideoAvailable,
    options.presentation,
    options.queueOpen,
    options.setMediaMode,
    options.setQueueOpen,
    options.trackKey,
    options.workspaceFullscreen,
  ]);
}
