import { Events } from "@wailsio/runtime";
import * as React from "react";

import {
  LISTEN_NOW_PLAYING_EVENT,
  LISTEN_NOW_PLAYING_STORAGE_KEY,
  LISTEN_TRAY_COMMAND_EVENT,
} from "@/app/main/listen/catalog";
import type { ListenNowPlayingStatus } from "@/app/main/listen/types";
import { ListenNowPlayingHoverPanel } from "@/app/main/sidebar";
import { normalizeTrayNowPlayingStatus } from "@/app/main/tray-now-playing";
import { getXiaText } from "@/features/xiadown/shared";
import { useSettingsStore } from "@/shared/store/settings";

function readStoredNowPlayingStatus() {
  try {
    return normalizeTrayNowPlayingStatus(
      JSON.parse(localStorage.getItem(LISTEN_NOW_PLAYING_STORAGE_KEY) || "null"),
    );
  } catch {
    return null;
  }
}

export function TrayMiniPlayerApp() {
  const settings = useSettingsStore((state) => state.settings);
  const [status, setStatus] = React.useState<ListenNowPlayingStatus | null>(
    () => readStoredNowPlayingStatus(),
  );
  const text = getXiaText(settings?.language);

  React.useEffect(() => {
    const offNowPlaying = Events.On(LISTEN_NOW_PLAYING_EVENT, (event: any) => {
      const payload = event?.data ?? event;
      setStatus(normalizeTrayNowPlayingStatus(payload));
    });
    return () => {
      offNowPlaying();
    };
  }, []);

  const sendTrayCommand = React.useCallback(
    (command: "previous" | "toggle" | "next") => {
      void Events.Emit(LISTEN_TRAY_COMMAND_EVENT, { command });
    },
    [],
  );

  return (
    <div className="tray-mini-player-root">
      <ListenNowPlayingHoverPanel
        status={status}
        text={text}
        surface="tray"
        onControlCommand={sendTrayCommand}
      />
    </div>
  );
}
