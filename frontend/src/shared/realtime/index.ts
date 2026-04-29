import { Call } from "@wailsio/runtime";

import { WebSocketClient } from "./client";
import type { RealtimeEvent } from "./types";
import type { RealtimeTopic } from "./topics";
import { DEFAULT_DEBUG_TOPICS } from "./topics";
import { useRealtimeStore } from "./store";
import { normalizeUpdateInfo, useUpdateStore } from "@/shared/store/update";

let client: WebSocketClient | null = null;
let startPromise: Promise<void> | null = null;

async function resolveWebSocketURL(): Promise<string> {
  try {
    const url = await Call.ByName("xiadown/internal/presentation/wails.RealtimeHandler.WebSocketURL");
    return typeof url === "string" ? url : String(url ?? "");
  } catch {
    return "";
  }
}

function ensureClient(url: string) {
  if (client) {
    return client;
  }

  client = new WebSocketClient(url, {
    reconnectIntervalMs: 3_000,
    onStatusChange: (status) => useRealtimeStore.getState().setStatus(status, url),
    onMessage: (event: RealtimeEvent) => useRealtimeStore.getState().pushMessage(event),
    onMetric: (kind) => useRealtimeStore.getState().recordMetric(kind),
  });

  return client;
}

export async function startRealtime() {
  if (startPromise) {
    return startPromise;
  }

  startPromise = (async () => {
    const url = await resolveWebSocketURL();
    if (!url) {
      useRealtimeStore.getState().setStatus("disconnected", "");
      startPromise = null;
      return;
    }

    const runtimeClient = ensureClient(url);
    useRealtimeStore.getState().setStatus("connecting", url);
    runtimeClient.connect();

    DEFAULT_DEBUG_TOPICS.forEach((topic) => registerTopic(topic));
    registerTopic("update.status", (event) => {
      const info = normalizeUpdateInfo(event.payload as any);
      useUpdateStore.getState().setInfo(info);
    });
  })();

  return startPromise;
}

export function registerTopic(topic: RealtimeTopic, handler?: (event: RealtimeEvent) => void): () => void {
  useRealtimeStore.getState().registerTopic(topic);

  let unsub: (() => void) | null = null;
  let stopped = false;

  startRealtime().then(() => {
    if (stopped || !client) {
      return;
    }
    unsub = client.subscribe(topic, (event) => {
      handler?.(event);
    });
  });

  return () => {
    stopped = true;
    unsub?.();
  };
}

export { REALTIME_TOPICS, DEFAULT_DEBUG_TOPICS } from "./topics";
export { useRealtimeStore } from "./store";
export type { RealtimeEvent } from "./types";
