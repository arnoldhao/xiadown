import { Call } from "@wailsio/runtime";

import { WebSocketClient } from "./client";
import type { RealtimeEvent } from "./types";
import type { RealtimeTopic } from "./topics";
import { DEFAULT_DEBUG_TOPICS } from "./topics";
import { useRealtimeStore } from "./store";
import { normalizeUpdateInfo, useUpdateStore } from "@/shared/store/update";

let client: WebSocketClient | null = null;
let startPromise: Promise<void> | null = null;
let lifecycleGeneration = 0;
let internalUnsubscribers: Array<() => void> = [];
const connectedListeners = new Set<() => void>();

type TopicRegistration = {
  topic: RealtimeTopic;
  handler?: (event: RealtimeEvent) => void;
  unsubscribe?: () => void;
  stopped: boolean;
};

const topicRegistrations = new Set<TopicRegistration>();

function attachTopicRegistration(registration: TopicRegistration) {
  if (registration.stopped || registration.unsubscribe || !client) {
    return;
  }
  registration.unsubscribe = client.subscribe(registration.topic, (event) => {
    registration.handler?.(event);
  });
}

function attachRegisteredTopics() {
  topicRegistrations.forEach(attachTopicRegistration);
}

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
    onStatusChange: (status) => {
      useRealtimeStore.getState().setStatus(status, url);
      if (status === "connected") {
        connectedListeners.forEach((listener) => {
          try {
            listener();
          } catch {}
        });
      }
    },
    onMessage: (event: RealtimeEvent) => useRealtimeStore.getState().pushMessage(event),
    onMetric: (kind) => useRealtimeStore.getState().recordMetric(kind),
  });

  return client;
}

export async function startRealtime() {
  if (startPromise) {
    return startPromise;
  }

  const generation = lifecycleGeneration;
  startPromise = (async () => {
    const url = await resolveWebSocketURL();
    if (generation !== lifecycleGeneration) {
      return;
    }
    if (!url) {
      useRealtimeStore.getState().setStatus("disconnected", "");
      startPromise = null;
      throw new Error("realtime endpoint is not ready");
    }

    const runtimeClient = ensureClient(url);
    useRealtimeStore.getState().setStatus("connecting", url);
    runtimeClient.connect();

    // Components may register their topics while the startup presentation is
    // still visible. Keep those registrations inert until the explicit
    // post-startup startRealtime call, then attach all of them to this client.
    attachRegisteredTopics();

    internalUnsubscribers = [
      ...DEFAULT_DEBUG_TOPICS.map((topic) => registerTopic(topic)),
      registerTopic("update.status", (event) => {
        const info = normalizeUpdateInfo(event.payload as any);
        useUpdateStore.getState().setInfo(info);
      }),
    ];
  })();

  return startPromise;
}

export function stopRealtime() {
  lifecycleGeneration += 1;
  internalUnsubscribers.forEach((unsubscribe) => unsubscribe());
  internalUnsubscribers = [];

  // Preserve live component registrations across a deliberate transport
  // restart, but detach them from the old client so the next start can attach
  // each handler exactly once.
  topicRegistrations.forEach((registration) => {
    registration.unsubscribe?.();
    registration.unsubscribe = undefined;
  });
  client?.disconnect();
  client = null;
  startPromise = null;
  useRealtimeStore.getState().setStatus("disconnected", "");
}

export function registerTopic(topic: RealtimeTopic, handler?: (event: RealtimeEvent) => void): () => void {
  useRealtimeStore.getState().registerTopic(topic);

  const registration: TopicRegistration = {
    topic,
    handler,
    stopped: false,
  };
  topicRegistrations.add(registration);
  attachTopicRegistration(registration);

  return () => {
    registration.stopped = true;
    registration.unsubscribe?.();
    registration.unsubscribe = undefined;
    topicRegistrations.delete(registration);
  };
}

export function onRealtimeConnected(listener: () => void): () => void {
  connectedListeners.add(listener);
  return () => connectedListeners.delete(listener);
}

export { REALTIME_TOPICS, DEFAULT_DEBUG_TOPICS } from "./topics";
export { useRealtimeStore } from "./store";
export type { RealtimeEvent } from "./types";

if (import.meta.hot) {
  import.meta.hot.dispose(stopRealtime);
}
