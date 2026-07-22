import { describe, expect, test } from "bun:test";

import { createInitialRequestCoordinator } from "./initial-request-coordinator";

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise;
    reject = rejectPromise;
  });
  return { promise, resolve, reject };
}

describe("initial request coordinator", () => {
  test("shares a preload that starts before the initial query", async () => {
    const first = deferred<string>();
    let calls = 0;
    const coordinator = createInitialRequestCoordinator(() => {
      calls += 1;
      return first.promise;
    });

    const preload = coordinator.preload();
    const initial = coordinator.requestInitial();
    first.resolve("settings");

    await expect(preload).resolves.toBeUndefined();
    await expect(initial).resolves.toBe("settings");
    expect(calls).toBe(1);
  });

  test("a late preload shares an initial query that is still pending", async () => {
    const first = deferred<string>();
    let calls = 0;
    const coordinator = createInitialRequestCoordinator(() => {
      calls += 1;
      return first.promise;
    });

    const initial = coordinator.requestInitial();
    const preload = coordinator.preload();
    first.resolve("settings");

    await expect(initial).resolves.toBe("settings");
    await expect(preload).resolves.toBeUndefined();
    expect(calls).toBe(1);
  });

  test("a preload arriving after initial completion does not refetch", async () => {
    let calls = 0;
    const coordinator = createInitialRequestCoordinator(async () => {
      calls += 1;
      return `settings-${calls}`;
    });

    await expect(coordinator.requestInitial()).resolves.toBe("settings-1");
    await coordinator.preload();
    expect(calls).toBe(1);

    await expect(coordinator.request()).resolves.toBe("settings-2");
    expect(calls).toBe(2);
  });

  test("a failed startup request can be retried without a stale preload", async () => {
    let calls = 0;
    const coordinator = createInitialRequestCoordinator(async () => {
      calls += 1;
      if (calls === 1) throw new Error("unavailable");
      return "settings";
    });

    await expect(coordinator.requestInitial()).rejects.toThrow("unavailable");
    await coordinator.preload();
    await expect(coordinator.requestInitial()).resolves.toBe("settings");
    expect(calls).toBe(2);
  });
});
