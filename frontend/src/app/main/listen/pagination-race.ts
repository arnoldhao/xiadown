export type ListenPaginationKind =
  | "artist"
  | "library"
  | "playlist"
  | "search";

export type ListenPaginationRequest = {
  context: string;
  controller: AbortController;
  key: string;
  kind: ListenPaginationKind;
};

export type ListenPaginationRequestRegistry = Map<
  string,
  ListenPaginationRequest
>;

export function createListenPaginationContextKey(
  parts: ReadonlyArray<boolean | number | string>,
) {
  return JSON.stringify(parts);
}

export function createListenPaginationRequestKey(
  kind: ListenPaginationKind,
  context: string,
  continuation: string,
) {
  return JSON.stringify([kind, context, continuation.trim()]);
}

export function beginListenPaginationRequest(
  requests: ListenPaginationRequestRegistry,
  kind: ListenPaginationKind,
  context: string,
  continuation: string,
): ListenPaginationRequest | null {
  const key = createListenPaginationRequestKey(
    kind,
    context,
    continuation,
  );
  if (requests.has(key)) {
    return null;
  }
  const controller = new AbortController();
  const request = { context, controller, key, kind };
  requests.set(key, request);
  return request;
}

export function finishListenPaginationRequest(
  requests: ListenPaginationRequestRegistry,
  request: ListenPaginationRequest,
) {
  if (requests.get(request.key) === request) {
    requests.delete(request.key);
  }
}

export function abortStaleListenPaginationRequests(
  requests: ListenPaginationRequestRegistry,
  kind: ListenPaginationKind,
  currentContext: string,
) {
  requests.forEach((request, key) => {
    if (request.kind !== kind || request.context === currentContext) {
      return;
    }
    request.controller.abort();
    requests.delete(key);
  });
}

export function abortListenPaginationRequests(
  requests: ListenPaginationRequestRegistry,
) {
  requests.forEach((request) => request.controller.abort());
  requests.clear();
}

export function isListenPaginationContextCurrent(
  expected: string,
  current: string,
) {
  return expected === current;
}

export function resolveListenNextContinuation(
  requestedContinuation: string,
  returnedContinuation: string,
) {
  const requested = requestedContinuation.trim();
  const returned = returnedContinuation.trim();
  return returned && returned !== requested ? returned : "";
}
