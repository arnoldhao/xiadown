import { describe, expect, test } from "bun:test";
import { QueryClient } from "@tanstack/react-query";

import type {
  ListResourceSniffResourcesResponse,
  ResourceSniffSession,
} from "@/shared/contracts/library";
import {
  LIBRARY_RESOURCE_SNIFF_QUERY_KEY,
  LIBRARY_RESOURCE_SNIFF_RESOURCES_QUERY_KEY,
  LIBRARY_RESOURCE_SNIFF_SESSIONS_QUERY_KEY,
  removeCanceledResourceSniffSessionQueries,
} from "./resourceSniffQueryCache";

function session(sessionId: string): ResourceSniffSession {
  return {
    sessionId,
    state: "active",
    browserStatus: "open",
    url: "",
  };
}

describe("resource sniff cancellation query cleanup", () => {
  test("removes only the canceled session and its exact polling queries", async () => {
    const queryClient = new QueryClient();
    const canceledSession = session("session-canceled");
    const remainingSession = session("session-remaining");
    const canceledResources: ListResourceSniffResourcesResponse = {
      session: canceledSession,
      resources: [],
      updatedAt: "2026-07-17T00:00:00Z",
    };
    const remainingResources: ListResourceSniffResourcesResponse = {
      session: remainingSession,
      resources: [],
      updatedAt: "2026-07-17T00:00:01Z",
    };

    queryClient.setQueryData(LIBRARY_RESOURCE_SNIFF_SESSIONS_QUERY_KEY, [
      canceledSession,
      remainingSession,
    ]);
    queryClient.setQueryData(
      [...LIBRARY_RESOURCE_SNIFF_QUERY_KEY, canceledSession.sessionId],
      canceledSession,
    );
    queryClient.setQueryData(
      [...LIBRARY_RESOURCE_SNIFF_QUERY_KEY, remainingSession.sessionId],
      remainingSession,
    );
    queryClient.setQueryData(
      [
        ...LIBRARY_RESOURCE_SNIFF_RESOURCES_QUERY_KEY,
        canceledSession.sessionId,
      ],
      canceledResources,
    );
    queryClient.setQueryData(
      [
        ...LIBRARY_RESOURCE_SNIFF_RESOURCES_QUERY_KEY,
        remainingSession.sessionId,
      ],
      remainingResources,
    );

    await removeCanceledResourceSniffSessionQueries(
      queryClient,
      `  ${canceledSession.sessionId}  `,
    );

    expect(
      queryClient.getQueryData<ResourceSniffSession[]>(
        LIBRARY_RESOURCE_SNIFF_SESSIONS_QUERY_KEY,
      ),
    ).toEqual([remainingSession]);
    expect(
      queryClient.getQueryData([
        ...LIBRARY_RESOURCE_SNIFF_QUERY_KEY,
        canceledSession.sessionId,
      ]),
    ).toBeUndefined();
    expect(
      queryClient.getQueryData([
        ...LIBRARY_RESOURCE_SNIFF_RESOURCES_QUERY_KEY,
        canceledSession.sessionId,
      ]),
    ).toBeUndefined();
    expect(
      queryClient.getQueryData([
        ...LIBRARY_RESOURCE_SNIFF_QUERY_KEY,
        remainingSession.sessionId,
      ]),
    ).toEqual(remainingSession);
    expect(
      queryClient.getQueryData([
        ...LIBRARY_RESOURCE_SNIFF_RESOURCES_QUERY_KEY,
        remainingSession.sessionId,
      ]),
    ).toEqual(remainingResources);

    queryClient.clear();
  });

  test("keeps the canceled session evicted when an in-flight list refresh reverts", async () => {
    const queryClient = new QueryClient();
    const canceledSession = session("session-canceled");
    const remainingSession = session("session-remaining");
    queryClient.setQueryData(LIBRARY_RESOURCE_SNIFF_SESSIONS_QUERY_KEY, [
      canceledSession,
      remainingSession,
    ]);
    let refreshAborted = false;
    const inFlightRefresh = queryClient
      .fetchQuery({
        queryKey: LIBRARY_RESOURCE_SNIFF_SESSIONS_QUERY_KEY,
        queryFn: ({ signal }) =>
          new Promise<ResourceSniffSession[]>((resolve) => {
            signal.addEventListener(
              "abort",
              () => {
                refreshAborted = true;
                resolve([canceledSession, remainingSession]);
              },
              { once: true },
            );
          }),
      })
      .catch(() => undefined);

    await removeCanceledResourceSniffSessionQueries(
      queryClient,
      canceledSession.sessionId,
    );
    await inFlightRefresh;

    expect(refreshAborted).toBe(true);
    expect(
      queryClient.getQueryData<ResourceSniffSession[]>(
        LIBRARY_RESOURCE_SNIFF_SESSIONS_QUERY_KEY,
      ),
    ).toEqual([remainingSession]);

    queryClient.clear();
  });

  test("cleans the canceled cache before refreshing sessions and status", async () => {
    const source = await Bun.file(new URL("./library.ts", import.meta.url)).text();
    const cancelHook = source.slice(
      source.indexOf("export function useCancelResourceSniff"),
      source.indexOf("export function useResourceSniffSession"),
    );

    expect(cancelHook).toContain(
      "await removeCanceledResourceSniffSessionQueries(",
    );
    expect(cancelHook).toContain(
      "queryKey: LIBRARY_RESOURCE_SNIFF_SESSIONS_QUERY_KEY",
    );
    expect(cancelHook).toContain(
      "queryKey: LIBRARY_CURRENT_RESOURCE_SNIFF_BROWSER_STATUS_QUERY_KEY",
    );
    expect(cancelHook).not.toContain(
      "queryKey: LIBRARY_RESOURCE_SNIFF_RESOURCES_QUERY_KEY",
    );
    expect(
      cancelHook.indexOf("removeCanceledResourceSniffSessionQueries"),
    ).toBeLessThan(cancelHook.indexOf("invalidateQueries"));
  });
});
