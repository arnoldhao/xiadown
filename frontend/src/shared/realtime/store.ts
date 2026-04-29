import { create } from "zustand";
import { persist } from "zustand/middleware";

import type { ConnectionStatus, RealtimeEvent } from "./types";

const MAX_PER_TOPIC = 100;
const MAX_TOTAL_MESSAGES = 400;

function trimMessageBuckets(messages: Record<string, RealtimeEvent[]>) {
  const entries = Object.entries(messages);
  let total = entries.reduce((sum, [, events]) => sum + events.length, 0);
  if (total <= MAX_TOTAL_MESSAGES) {
    return messages;
  }

  const flattened = entries
    .flatMap(([topic, events]) => events.map((event) => ({ topic, ts: event.ts })))
    .sort((left, right) => left.ts - right.ts);
  const dropCounts = new Map<string, number>();

  for (let index = 0; index < flattened.length && total > MAX_TOTAL_MESSAGES; index += 1) {
    const topic = flattened[index]?.topic ?? "";
    if (!topic) {
      continue;
    }
    dropCounts.set(topic, (dropCounts.get(topic) ?? 0) + 1);
    total -= 1;
  }

  const nextMessages: Record<string, RealtimeEvent[]> = {};
  entries.forEach(([topic, events]) => {
    const dropCount = dropCounts.get(topic) ?? 0;
    const nextEvents = dropCount > 0 ? events.slice(dropCount) : events;
    if (nextEvents.length > 0) {
      nextMessages[topic] = nextEvents;
    }
  });
  return nextMessages;
}

export interface RealtimeState {
  status: ConnectionStatus;
  url: string;
  topics: string[];
  messages: Record<string, RealtimeEvent[]>;
  metrics: {
    reconnects: number;
    replayEvents: number;
    resyncRequired: number;
    duplicateDrops: number;
    lastStatusChangeAt: number;
  };
  setStatus: (status: ConnectionStatus, url?: string) => void;
  registerTopic: (topic: string) => void;
  pushMessage: (event: RealtimeEvent) => void;
  recordMetric: (kind: "reconnect" | "replay" | "resync-required" | "duplicate-drop") => void;
  clearMessages: (topic?: string) => void;
}

const defaultRealtimeMetrics = () => ({
  reconnects: 0,
  replayEvents: 0,
  resyncRequired: 0,
  duplicateDrops: 0,
  lastStatusChangeAt: 0,
});

export const useRealtimeStore = create<RealtimeState>()(
  persist(
    (set) => ({
      status: "disconnected",
      url: "",
      topics: [],
      messages: {},
      metrics: defaultRealtimeMetrics(),
      setStatus: (status, url) =>
        set((state) => ({
          status,
          url: url ?? state.url,
          metrics: {
            ...state.metrics,
            lastStatusChangeAt: Date.now(),
          },
        })),
      registerTopic: (topic) =>
        set((state) =>
          state.topics.includes(topic)
            ? state
            : {
                topics: [...state.topics, topic],
              }
        ),
      pushMessage: (event) =>
        set((state) => {
          const existing = state.messages[event.topic] ?? [];
          const next = [...existing, event].slice(-MAX_PER_TOPIC);
          return {
            messages: trimMessageBuckets({
              ...state.messages,
              [event.topic]: next,
            }),
          };
        }),
      recordMetric: (kind) =>
        set((state) => {
          const metrics = { ...state.metrics };
          if (kind === "reconnect") {
            metrics.reconnects += 1;
          } else if (kind === "replay") {
            metrics.replayEvents += 1;
          } else if (kind === "resync-required") {
            metrics.resyncRequired += 1;
          } else if (kind === "duplicate-drop") {
            metrics.duplicateDrops += 1;
          }
          return { metrics };
        }),
      clearMessages: (topic) =>
        set((state) => {
          if (!topic) {
            return { messages: {} };
          }
          const { [topic]: _removed, ...rest } = state.messages;
          return { messages: rest };
        }),
    }),
    {
      name: "realtime-messages",
      version: 2,
      partialize: (state) => ({
        topics: state.topics,
        url: state.url,
        metrics: state.metrics,
      }),
      migrate: (persistedState) => {
        const state = (persistedState ?? {}) as Partial<RealtimeState>;
        return {
          status: "disconnected" as const,
          url: typeof state.url === "string" ? state.url : "",
          topics: Array.isArray(state.topics) ? state.topics.filter((topic): topic is string => typeof topic === "string") : [],
          messages: {},
          metrics: {
            ...defaultRealtimeMetrics(),
            ...(state.metrics ?? {}),
          },
        };
      },
    }
  )
);
