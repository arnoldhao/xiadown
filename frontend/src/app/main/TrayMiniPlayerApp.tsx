import { Events } from "@wailsio/runtime";
import * as React from "react";

import {
  DREAM_FM_NOW_PLAYING_EVENT,
  DREAM_FM_NOW_PLAYING_STORAGE_KEY,
  DREAM_FM_TRAY_COMMAND_EVENT,
} from "@/app/main/dreamfm/catalog";
import type {
  DreamFMMode,
  DreamFMNowPlayingState,
  DreamFMNowPlayingStatus,
} from "@/app/main/dreamfm/types";
import { DreamFMNowPlayingHoverPanel } from "@/app/main/sidebar";
import { getXiaText } from "@/features/xiadown/shared";
import { createDreamFMTrayControlStyle } from "@/shared/styles/theme-runtime";
import { useSettingsStore } from "@/shared/store/settings";

const DREAM_FM_STATES: DreamFMNowPlayingState[] = [
  "idle",
  "loading",
  "playing",
  "paused",
  "error",
];

const DREAM_FM_MODES: DreamFMMode[] = ["local", "online", "live"];

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

function normalizeNowPlayingStatus(value: unknown): DreamFMNowPlayingStatus | null {
  const record = asRecord(value);
  if (!record) {
    return null;
  }

  const state = stringValue(record.state) as DreamFMNowPlayingState;
  if (!DREAM_FM_STATES.includes(state)) {
    return null;
  }

  const mode = stringValue(record.mode) as DreamFMMode;
  const progress = asRecord(record.progress);

  return {
    state,
    title: stringValue(record.title),
    subtitle: stringValue(record.subtitle),
    artworkURL: stringValue(record.artworkURL),
    mode: DREAM_FM_MODES.includes(mode) ? mode : "local",
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
      JSON.parse(localStorage.getItem(DREAM_FM_NOW_PLAYING_STORAGE_KEY) || "null"),
    );
  } catch {
    return null;
  }
}

export function TrayMiniPlayerApp() {
  const settings = useSettingsStore((state) => state.settings);
  const [status, setStatus] = React.useState<DreamFMNowPlayingStatus | null>(
    () => readStoredNowPlayingStatus(),
  );
  const text = getXiaText(settings?.language);
  const controlStyle = React.useMemo(
    () => createDreamFMTrayControlStyle(settings),
    [settings],
  );

  React.useEffect(() => {
    const offNowPlaying = Events.On(DREAM_FM_NOW_PLAYING_EVENT, (event: any) => {
      const payload = event?.data ?? event;
      setStatus(normalizeNowPlayingStatus(payload));
    });
    return () => {
      offNowPlaying();
    };
  }, []);

  const sendTrayCommand = React.useCallback(
    (command: "previous" | "toggle" | "next") => {
      void Events.Emit(DREAM_FM_TRAY_COMMAND_EVENT, { command });
    },
    [],
  );

  return (
    <div
      className="flex h-screen w-screen items-center justify-center overflow-hidden bg-transparent text-foreground"
      style={controlStyle}
    >
      <DreamFMNowPlayingHoverPanel
        status={status}
        text={text}
        onControlCommand={sendTrayCommand}
      />
    </div>
  );
}
