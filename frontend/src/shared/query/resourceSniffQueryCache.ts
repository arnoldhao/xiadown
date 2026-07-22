import type { QueryClient } from "@tanstack/react-query";

import type { ResourceSniffSession } from "@/shared/contracts/library";

export const LIBRARY_RESOURCE_SNIFF_QUERY_KEY = [
  "library",
  "resource-sniff",
] as const;
export const LIBRARY_RESOURCE_SNIFF_SESSIONS_QUERY_KEY = [
  "library",
  "resource-sniff",
  "sessions",
] as const;
export const LIBRARY_RESOURCE_SNIFF_RESOURCES_QUERY_KEY = [
  "library",
  "resource-sniff",
  "resources",
] as const;

export async function removeCanceledResourceSniffSessionQueries(
  queryClient: QueryClient,
  rawSessionId: string,
) {
  const sessionId = rawSessionId.trim();
  if (!sessionId) {
    return;
  }

  const removeSessionFromCachedList = () => {
    queryClient.setQueryData<ResourceSniffSession[] | undefined>(
      LIBRARY_RESOURCE_SNIFF_SESSIONS_QUERY_KEY,
      (sessions) =>
        sessions?.filter((session) => session.sessionId !== sessionId),
    );
  };
  removeSessionFromCachedList();

  const sessionQueryKey = [...LIBRARY_RESOURCE_SNIFF_QUERY_KEY, sessionId];
  const resourcesQueryKey = [
    ...LIBRARY_RESOURCE_SNIFF_RESOURCES_QUERY_KEY,
    sessionId,
  ];
  await Promise.all([
    queryClient.cancelQueries({
      queryKey: LIBRARY_RESOURCE_SNIFF_SESSIONS_QUERY_KEY,
      exact: true,
    }),
    queryClient.cancelQueries({ queryKey: sessionQueryKey, exact: true }),
    queryClient.cancelQueries({ queryKey: resourcesQueryKey, exact: true }),
  ]);
  // Canceling an in-flight list query may revert its cache to the pre-fetch
  // snapshot, so apply the eviction again before refreshing the list.
  removeSessionFromCachedList();
  queryClient.removeQueries({ queryKey: sessionQueryKey, exact: true });
  queryClient.removeQueries({ queryKey: resourcesQueryKey, exact: true });
}
