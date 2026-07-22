import { afterEach, beforeEach, describe, expect, it, mock } from "bun:test";

const runtimeState = {
  bootstrap: {
    enabled: true,
    appId: "app-123",
    installId: "install-1",
    sessionId: "session-1",
    testMode: true,
  },
  calls: [] as Array<{ name: string; args: unknown[] }>,
  eventHandlers: new Map<string, (event: unknown) => void>(),
  bootstrapFailures: 0,
  launchFailures: 0,
};

mock.module("@wailsio/runtime", () => ({
  Call: {
    ByName(name: string, ...args: unknown[]) {
      runtimeState.calls.push({ name, args });
      if (name.endsWith(".Bootstrap")) {
        if (runtimeState.bootstrapFailures > 0) {
          runtimeState.bootstrapFailures -= 1;
          return Promise.reject(new Error("bridge not ready"));
        }
        return Promise.resolve(runtimeState.bootstrap);
      }
      if (name.endsWith(".TrackAppLaunch")) {
        if (runtimeState.launchFailures > 0) {
          runtimeState.launchFailures -= 1;
          return Promise.reject(new Error("launch not ready"));
        }
        return Promise.resolve(1);
      }
      return Promise.resolve(undefined);
    },
  },
  Events: {
    On(name: string, callback: (event: unknown) => void) {
      runtimeState.eventHandlers.set(name, callback);
      return () => {
        if (runtimeState.eventHandlers.get(name) === callback) {
          runtimeState.eventHandlers.delete(name);
        }
      };
    },
  },
}));

const { TelemetryManager } = await import("./manager");

const originalWindow = (globalThis as { window?: unknown }).window;

const waitFor = async (predicate: () => boolean) => {
  for (let attempt = 0; attempt < 80; attempt += 1) {
    if (predicate()) {
      return;
    }
    await new Promise((resolve) => setTimeout(resolve, 0));
  }
  throw new Error("timed out waiting for telemetry call");
};

beforeEach(() => {
  runtimeState.calls = [];
  runtimeState.eventHandlers.clear();
  runtimeState.bootstrapFailures = 0;
  runtimeState.launchFailures = 0;
  (globalThis as { window?: unknown }).window = {
    setTimeout(callback: () => void) {
      return setTimeout(callback, 0);
    },
  };
});

afterEach(() => {
  if (originalWindow === undefined) {
    delete (globalThis as { window?: unknown }).window;
  } else {
    (globalThis as { window?: unknown }).window = originalWindow;
  }
});

describe("TelemetryManager", () => {
  it("retries bridge bootstrap and launch before subscribing once", async () => {
    runtimeState.bootstrapFailures = 2;
    runtimeState.launchFailures = 1;
    const manager = new TelemetryManager();

    await manager.start();

    expect(
      runtimeState.calls.filter((call) => call.name.endsWith(".Bootstrap")),
    ).toHaveLength(3);
    expect(
      runtimeState.calls.filter((call) =>
        call.name.endsWith(".TrackAppLaunch"),
      ),
    ).toHaveLength(2);
    expect(runtimeState.eventHandlers.has("telemetry:signal")).toBe(true);

    manager.stop();
    expect(runtimeState.eventHandlers.has("telemetry:signal")).toBe(false);
  });

  it("sends only allowed signals and anonymous common/station properties", async () => {
    const manager = new TelemetryManager();
    await manager.start();
    const emit = runtimeState.eventHandlers.get("telemetry:signal");
    expect(emit).toBeDefined();

    emit?.({
      data: {
        type: "XiaDown.Library.deleted",
        payload: { "XiaDown.Library.path": "/private/file.mp4" },
      },
    });
    emit?.({
      data: {
        type: "XiaDown.Station.opened",
        payload: {
          "TelemetryDeck.AppInfo.version": "1.2.3",
          "TelemetryDeck.Calendar.hourOfDay": 10,
          "XiaDown.Station.name": "library",
          "XiaDown.Library.path": "/private/file.mp4",
        },
      },
    });

    await waitFor(() =>
      runtimeState.calls.some((call) => call.name.endsWith(".PostSignal")),
    );
    const posts = runtimeState.calls.filter((call) =>
      call.name.endsWith(".PostSignal"),
    );
    expect(posts).toHaveLength(1);
    const request = posts[0]?.args[0] as {
      body: Array<Record<string, unknown>>;
    };
    const body = request.body[0] ?? {};
    const payload = body.payload as Record<string, unknown>;

    expect(body.type).toBe("XiaDown.Station.opened");
    expect(body.clientUser).toBe(
      "dbdacfba94e13158e2a06038e42c25809d989956e96a08bc836e0d164b420eae",
    );
    expect(body.sessionID).toBe("session-1");
    expect(body.isTestMode).toBe(true);
    expect(payload["XiaDown.Station.name"]).toBe("library");
    expect(payload["TelemetryDeck.AppInfo.version"]).toBe("1.2.3");
    expect(payload["TelemetryDeck.SDK.name"]).toBe("JavaScriptSDK");
    expect(payload["TelemetryDeck.Calendar.hourOfDay"]).toBeUndefined();
    expect(payload["XiaDown.Library.path"]).toBeUndefined();

    manager.stop();
  });

  it("has no unload summary or telemetry console logging path", async () => {
    const source = await Bun.file(new URL("./manager.ts", import.meta.url)).text();
    expect(source).not.toContain("FlushSessionSummary");
    expect(source).not.toContain("pagehide");
    expect(source).not.toContain("beforeunload");
    expect(source).not.toContain("console.");
  });
});
