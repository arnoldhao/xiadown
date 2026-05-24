import { Events } from "@wailsio/runtime";
import * as React from "react";

import {
  LISTEN_NOW_PLAYING_EVENT,
  LISTEN_NOW_PLAYING_STORAGE_KEY,
  LISTEN_TRAY_COMMAND_EVENT,
} from "@/app/main/listen/catalog";
import type {
  ListenMode,
  ListenNowPlayingState,
  ListenNowPlayingStatus,
} from "@/app/main/listen/types";
import { ListenNowPlayingHoverPanel } from "@/app/main/sidebar";
import { getXiaText } from "@/features/xiadown/shared";
import { createListenTrayControlStyle } from "@/shared/styles/theme-runtime";
import { useSettingsStore } from "@/shared/store/settings";

const LISTEN_STATES: ListenNowPlayingState[] = [
  "idle",
  "loading",
  "playing",
  "paused",
  "error",
];

const LISTEN_MODES: ListenMode[] = ["linger", "muse", "hush"];

function asRecord(value: unknown): Record<string, unknown> | null {
  return value && typeof value === "object"
    ? (value as Record<string, unknown>)
    : null;
}

function stringValue(value: unknown) {
  return typeof value === "string" ? value : "";
}

function finiteNumber(value: unknown) {
  return typeof value === "number" && Number.isFinite(value) ? value : 0;
}

function normalizeNowPlayingStatus(value: unknown): ListenNowPlayingStatus | null {
  const record = asRecord(value);
  if (!record) {
    return null;
  }

  const state = stringValue(record.state) as ListenNowPlayingState;
  if (!LISTEN_STATES.includes(state)) {
    return null;
  }

  const mode = stringValue(record.mode) as ListenMode;
  const progress = asRecord(record.progress);

  return {
    state,
    title: stringValue(record.title),
    subtitle: stringValue(record.subtitle),
    artworkURL: stringValue(record.artworkURL),
    mode: LISTEN_MODES.includes(mode) ? mode : "linger",
    canControl: record.canControl === true,
    progress: {
      currentTime: finiteNumber(progress?.currentTime),
      duration: finiteNumber(progress?.duration),
      bufferedTime: finiteNumber(progress?.bufferedTime),
    },
  };
}

function readStoredNowPlayingStatus() {
  try {
    return normalizeNowPlayingStatus(
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
  const controlStyle = React.useMemo(
    () => createListenTrayControlStyle(settings),
    [settings],
  );

  React.useEffect(() => {
    const offNowPlaying = Events.On(LISTEN_NOW_PLAYING_EVENT, (event: any) => {
      const payload = event?.data ?? event;
      setStatus(normalizeNowPlayingStatus(payload));
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
    <div
      className="tray-mini-player-root flex h-screen w-screen items-center justify-center overflow-hidden rounded-[22px] bg-transparent text-foreground [clip-path:inset(0_round_22px)]"
      style={controlStyle}
    >
      <ListenNowPlayingHoverPanel
        status={status}
        text={text}
        surface="tray"
        onControlCommand={sendTrayCommand}
      />
    </div>
  );
}
