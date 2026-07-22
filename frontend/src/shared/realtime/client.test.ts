import { afterEach, beforeEach, describe, expect, test } from "bun:test";

import { WebSocketClient } from "./client";

type SocketEventName = "open" | "message" | "close" | "error";
type SocketListener = (event: MessageEvent<string>) => void;

class FakeTimers {
  private nextID = 1;
  private readonly callbacks = new Map<number, () => void>();

  readonly setTimeout = (callback: TimerHandler) => {
    const id = this.nextID++;
    if (typeof callback === "function") {
      this.callbacks.set(id, () => callback());
    }
    return id;
  };

  readonly clearTimeout = (id?: number) => {
    if (id !== undefined) {
      this.callbacks.delete(id);
    }
  };

  run(id: number) {
    const callback = this.callbacks.get(id);
    this.callbacks.delete(id);
    callback?.();
  }

  ids() {
    return Array.from(this.callbacks.keys());
  }
}

class FakeWebSocket {
  static readonly OPEN = 1;
  static instances: FakeWebSocket[] = [];

  readyState = 0;
  closed = false;
  private readonly listeners = new Map<SocketEventName, Set<SocketListener>>();

  constructor(readonly url: string) {
    FakeWebSocket.instances.push(this);
  }

  addEventListener(name: SocketEventName, listener: SocketListener) {
    const listeners = this.listeners.get(name) ?? new Set<SocketListener>();
    listeners.add(listener);
    this.listeners.set(name, listeners);
  }

  emit(name: SocketEventName, data = "") {
    const event = { data } as MessageEvent<string>;
    for (const listener of Array.from(this.listeners.get(name) ?? [])) {
      listener(event);
    }
  }

  open() {
    this.readyState = FakeWebSocket.OPEN;
    this.emit("open");
  }

  close() {
    if (this.closed) {
      return;
    }
    this.closed = true;
    this.readyState = 3;
    this.emit("close");
  }

  send() {}
}

const originalWindow = (globalThis as { window?: unknown }).window;
const originalWebSocket = globalThis.WebSocket;
let timers: FakeTimers;

beforeEach(() => {
  timers = new FakeTimers();
  FakeWebSocket.instances = [];
  (globalThis as { window?: unknown }).window = {
    setTimeout: timers.setTimeout,
    clearTimeout: timers.clearTimeout,
  };
  globalThis.WebSocket = FakeWebSocket as unknown as typeof WebSocket;
});

afterEach(() => {
  if (originalWindow === undefined) {
    delete (globalThis as { window?: unknown }).window;
  } else {
    (globalThis as { window?: unknown }).window = originalWindow;
  }
  globalThis.WebSocket = originalWebSocket;
});

describe("WebSocketClient connection lifecycle", () => {
  test("an old connection timeout cannot close a successful replacement", () => {
    const statuses: string[] = [];
    const client = new WebSocketClient("ws://127.0.0.1/realtime", {
      reconnectIntervalMs: 10,
      connectTimeoutMs: 25,
      onStatusChange: (status) => statuses.push(status),
    });

    client.connect();
    const first = FakeWebSocket.instances[0]!;
    first.emit("error");

    expect(timers.ids()).toEqual([2]);
    timers.run(2);
    const second = FakeWebSocket.instances[1]!;
    second.open();

    // Timer 1 belonged to the failed socket. It has been cancelled and must
    // never operate on the replacement stored by the client.
    timers.run(1);

    expect(second.closed).toBe(false);
    expect(statuses.at(-1)).toBe("connected");
    client.disconnect();
  });

  test("late events from a replaced socket do not disconnect the active one", () => {
    const statuses: string[] = [];
    const client = new WebSocketClient("ws://127.0.0.1/realtime", {
      reconnectIntervalMs: 10,
      connectTimeoutMs: 25,
      onStatusChange: (status) => statuses.push(status),
    });

    client.connect();
    const first = FakeWebSocket.instances[0]!;
    first.emit("error");
    timers.run(2);
    const second = FakeWebSocket.instances[1]!;
    second.open();

    first.emit("close");
    first.emit("message", JSON.stringify({ topic: "library.file", seq: 1 }));

    expect(second.closed).toBe(false);
    expect(statuses.at(-1)).toBe("connected");
    expect(timers.ids()).toEqual([]);
    client.disconnect();
  });
});
