import { describe, expect, test } from "bun:test";

import { installActiveWindowTracker } from "./active-window-tracker";

type Listener = () => void;

function listenerHarness() {
  const windowListeners = new Map<string, Set<Listener>>();
  const documentListeners = new Map<string, Set<Listener>>();
  const runtimeListeners = new Map<string, Set<Listener>>();
  const intervals = new Map<number, Listener>();
  const updates: Array<boolean | undefined> = [];
  let nextIntervalID = 1;

  const add = (target: Map<string, Set<Listener>>, name: string, listener: Listener) => {
    const listeners = target.get(name) ?? new Set<Listener>();
    listeners.add(listener);
    target.set(name, listeners);
  };
  const remove = (target: Map<string, Set<Listener>>, name: string, listener: Listener) => {
    const listeners = target.get(name);
    listeners?.delete(listener);
    if (listeners?.size === 0) {
      target.delete(name);
    }
  };

  const cleanup = installActiveWindowTracker({
    addWindowListener: (name, listener) => add(windowListeners, name, listener),
    removeWindowListener: (name, listener) => remove(windowListeners, name, listener),
    addDocumentListener: (name, listener) => add(documentListeners, name, listener),
    removeDocumentListener: (name, listener) => remove(documentListeners, name, listener),
    subscribeRuntimeEvent: (name, listener) => {
      add(runtimeListeners, name, listener);
      return () => remove(runtimeListeners, name, listener);
    },
    setHeartbeat: (listener) => {
      const id = nextIntervalID++;
      intervals.set(id, listener);
      return id;
    },
    clearHeartbeat: (id) => intervals.delete(id),
    updateActive: (active) => updates.push(active),
    shouldHeartbeat: () => true,
    heartbeatMs: 1_000,
  });

  return {
    cleanup,
    windowListeners,
    documentListeners,
    runtimeListeners,
    intervals,
    updates,
  };
}

describe("active-window tracker lifecycle", () => {
  test("installs the complete DOM, runtime-event and heartbeat contract", () => {
    const harness = listenerHarness();

    expect(Array.from(harness.windowListeners.values()).reduce((sum, listeners) => sum + listeners.size, 0)).toBe(4);
    expect(Array.from(harness.documentListeners.values()).reduce((sum, listeners) => sum + listeners.size, 0)).toBe(1);
    expect(Array.from(harness.runtimeListeners.values()).reduce((sum, listeners) => sum + listeners.size, 0)).toBe(14);
    expect(harness.intervals.size).toBe(1);

    harness.windowListeners.get("focus")?.forEach((listener) => listener());
    harness.windowListeners.get("blur")?.forEach((listener) => listener());
    harness.documentListeners.get("visibilitychange")?.forEach((listener) => listener());
    harness.intervals.values().next().value?.();
    expect(harness.updates).toEqual([true, false, undefined, true]);

    harness.cleanup();
  });

  test("removes every resource exactly once during HMR disposal", () => {
    const harness = listenerHarness();

    harness.cleanup();
    harness.cleanup();

    expect(harness.windowListeners.size).toBe(0);
    expect(harness.documentListeners.size).toBe(0);
    expect(harness.runtimeListeners.size).toBe(0);
    expect(harness.intervals.size).toBe(0);
    expect(harness.updates).toEqual([false]);
  });
});
