import { describe, expect, test } from "bun:test";

import { RSSLatestEntryStateMutationCoordinator } from "./entry-state-mutation-coordinator";
import type { RSSSetEntryStateRequest } from "./types";

describe("RSS entry-state latest-wins coordinator", () => {
  test("serializes rapid true then false writes so the latest intent commits last", async () => {
    const coordinator = new RSSLatestEntryStateMutationCoordinator();
    const first = coordinator.reserve(request("read-true", true));
    const latest = coordinator.reserve(request("read-false", false));
    const firstGate = deferred<void>();
    const events: string[] = [];
    let durableRead = false;

    const firstWrite = coordinator.execute(first, async () => {
      events.push("true:start");
      await firstGate.promise;
      durableRead = true;
      events.push("true:commit");
      return durableRead;
    });
    const latestWrite = coordinator.execute(latest, async () => {
      events.push("false:start");
      durableRead = false;
      events.push("false:commit");
      return durableRead;
    });

    await Promise.resolve();
    expect(events).toEqual(["true:start"]);
    expect(coordinator.isLatest(first)).toBeFalse();
    expect(coordinator.isLatest(latest)).toBeTrue();

    firstGate.resolve();
    expect(await firstWrite).toBeTrue();
    expect(await latestWrite).toBeFalse();
    expect(events).toEqual([
      "true:start",
      "true:commit",
      "false:start",
      "false:commit",
    ]);
    expect(durableRead).toBeFalse();

    coordinator.retire(first);
    coordinator.retire(latest);
  });

  test("does not serialize unrelated fields for the same entry", async () => {
    const coordinator = new RSSLatestEntryStateMutationCoordinator();
    const read = coordinator.reserve(request("read", true));
    const starred = coordinator.reserve({
      ...request("starred", true),
      field: "starred",
      read: undefined,
      starred: true,
    });
    const readGate = deferred<void>();
    const events: string[] = [];
    const readWrite = coordinator.execute(read, async () => {
      events.push("read:start");
      await readGate.promise;
    });
    const starredWrite = coordinator.execute(starred, async () => {
      events.push("starred:start");
    });

    await starredWrite;
    expect(events).toEqual(["read:start", "starred:start"]);
    readGate.resolve();
    await readWrite;
  });

  test("keeps the queue chained when a newer reservation is cancelled", async () => {
    const coordinator = new RSSLatestEntryStateMutationCoordinator();
    const first = coordinator.reserve(request("first", true));
    const cancelled = coordinator.reserve(request("cancelled", false));
    const firstGate = deferred<void>();
    const events: string[] = [];
    const firstWrite = coordinator.execute(first, async () => {
      events.push("first:start");
      await firstGate.promise;
      events.push("first:commit");
    });

    coordinator.cancel(cancelled);
    coordinator.retire(cancelled);
    const latest = coordinator.reserve(request("latest", false));
    const latestWrite = coordinator.execute(latest, async () => {
      events.push("latest:start");
    });
    await Promise.resolve();
    expect(events).toEqual(["first:start"]);

    firstGate.resolve();
    await firstWrite;
    coordinator.retire(first);
    await latestWrite;
    expect(events).toEqual(["first:start", "first:commit", "latest:start"]);
    coordinator.retire(latest);
  });
});

function request(mutationId: string, read: boolean): RSSSetEntryStateRequest {
  return {
    id: "entry-1",
    field: "read",
    read,
    expectedRevision: 0,
    mutationId,
  };
}

function deferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void;
  const promise = new Promise<T>((next) => {
    resolve = next;
  });
  return { promise, resolve };
}
