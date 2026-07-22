import { afterAll, beforeEach, describe, expect, mock, test } from "bun:test";

type Deferred = {
  promise: Promise<string>;
  resolve: (value: string) => void;
};

function deferred(): Deferred {
  let resolve!: (value: string) => void;
  const promise = new Promise<string>((next) => {
    resolve = next;
  });
  return { promise, resolve };
}

const runtimeState = {
  nextURL: Promise.resolve("ws://127.0.0.1/realtime"),
  statuses: [] as Array<{ status: string; url?: string }>,
};

type SocketListener = () => void;

class FakeWebSocket {
  static readonly OPEN = 1;
  static instances: FakeWebSocket[] = [];

  readyState = 0;
  closed = false;
  private readonly listeners = new Map<string, Set<SocketListener>>();

  constructor(readonly url: string) {
    FakeWebSocket.instances.push(this);
  }

  addEventListener(name: string, listener: SocketListener) {
    const listeners = this.listeners.get(name) ?? new Set<SocketListener>();
    listeners.add(listener);
    this.listeners.set(name, listeners);
  }

  close() {
    if (this.closed) {
      return;
    }
    this.closed = true;
    for (const listener of Array.from(this.listeners.get("close") ?? [])) {
      listener();
    }
  }

  open() {
    this.readyState = FakeWebSocket.OPEN;
    for (const listener of Array.from(this.listeners.get("open") ?? [])) {
      listener();
    }
  }

  send() {}
}

mock.module("@wailsio/runtime", () => ({
  Call: {
    ByName: () => runtimeState.nextURL,
  },
}));

mock.module("./store", () => ({
  useRealtimeStore: {
    getState: () => ({
      setStatus(status: string, url?: string) {
        runtimeState.statuses.push({ status, url });
      },
      pushMessage() {},
      recordMetric() {},
      registerTopic() {},
    }),
  },
}));

mock.module("@/shared/store/update", () => ({
  normalizeUpdateInfo: (value: unknown) => value,
  useUpdateStore: {
    getState: () => ({ setInfo() {} }),
  },
}));

const {
  onRealtimeConnected,
  registerTopic,
  startRealtime,
  stopRealtime,
} = await import("./index");

const originalWindow = (globalThis as { window?: unknown }).window;
const originalWebSocket = globalThis.WebSocket;
let nextTimerID = 1;

(globalThis as { window?: unknown }).window = {
  setTimeout: () => nextTimerID++,
  clearTimeout: () => undefined,
};
globalThis.WebSocket = FakeWebSocket as unknown as typeof WebSocket;

beforeEach(() => {
  stopRealtime();
  FakeWebSocket.instances = [];
  runtimeState.statuses = [];
  runtimeState.nextURL = Promise.resolve("ws://127.0.0.1/realtime");
});

describe("realtime singleton lifecycle", () => {
  test("topic registration waits for the explicit startup gate", async () => {
    const unsubscribe = registerTopic("library.file");
    await Promise.resolve();

    expect(FakeWebSocket.instances).toHaveLength(0);

    await startRealtime();
    expect(FakeWebSocket.instances).toHaveLength(1);

    unsubscribe();
    stopRealtime();
  });

  test("reports readiness only after the socket is actually connected", async () => {
    let connected = 0;
    const unsubscribe = onRealtimeConnected(() => {
      connected += 1;
    });

    await startRealtime();
    expect(connected).toBe(0);

    FakeWebSocket.instances[0]!.open();
    expect(connected).toBe(1);

    unsubscribe();
    stopRealtime();
  });

  test("disconnects the singleton and creates a fresh client after disposal", async () => {
    await startRealtime();
    await Promise.resolve();

    const first = FakeWebSocket.instances[0]!;
    expect(first.closed).toBe(false);

    stopRealtime();

    expect(first.closed).toBe(true);
    expect(runtimeState.statuses.at(-1)).toEqual({ status: "disconnected", url: "" });

    await startRealtime();
    expect(FakeWebSocket.instances).toHaveLength(2);
    expect(FakeWebSocket.instances[1]).not.toBe(first);
    stopRealtime();
  });

  test("does not create a client when disposal wins an in-flight URL lookup", async () => {
    const url = deferred();
    runtimeState.nextURL = url.promise;

    const startup = startRealtime();
    stopRealtime();
    url.resolve("ws://127.0.0.1/stale");
    await startup;

    expect(FakeWebSocket.instances).toHaveLength(0);
  });

  test("reports an unavailable endpoint so the provider can retry", async () => {
    runtimeState.nextURL = Promise.resolve("");

    await expect(startRealtime()).rejects.toThrow(
      "realtime endpoint is not ready",
    );
    expect(FakeWebSocket.instances).toHaveLength(0);

    runtimeState.nextURL = Promise.resolve("ws://127.0.0.1/recovered");
    await expect(startRealtime()).resolves.toBeUndefined();
    expect(FakeWebSocket.instances).toHaveLength(1);
    stopRealtime();
  });
});

afterAll(() => {
  stopRealtime();
  if (originalWindow === undefined) {
    delete (globalThis as { window?: unknown }).window;
  } else {
    (globalThis as { window?: unknown }).window = originalWindow;
  }
  globalThis.WebSocket = originalWebSocket;
});
