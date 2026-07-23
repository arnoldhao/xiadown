export type ListenYouTubeAppSessionTransition =
  | "connected"
  | "disconnected"
  | "refresh"
  | "ignore";

function normalizedEventField(value: unknown) {
  return typeof value === "string" ? value.trim().toLowerCase() : "";
}

export function resolveListenYouTubeAppSessionTransition(
  event: unknown,
): ListenYouTubeAppSessionTransition {
  const envelope =
    event && typeof event === "object"
      ? (event as Record<string, unknown>)
      : null;
  const payloadValue = envelope?.data ?? event;
  const payload =
    payloadValue && typeof payloadValue === "object"
      ? (payloadValue as Record<string, unknown>)
      : {};
  const siteKey = normalizedEventField(payload.siteKey);
  if (siteKey && siteKey !== "youtube") {
    return "ignore";
  }

  const action = normalizedEventField(payload.action);
  const status = normalizedEventField(payload.status);
  if (action === "clear" || status === "disconnected") {
    return "disconnected";
  }
  if (action === "finish" || status === "connected") {
    return "connected";
  }
  return "refresh";
}

export function handleListenYouTubeAppSessionEvent(
  event: unknown,
  handlers: {
    onConnected: () => void;
    onDisconnected: () => void;
    onReload: () => void;
    onRefetch: () => void;
  },
) {
  const transition = resolveListenYouTubeAppSessionTransition(event);
  if (transition === "ignore") {
    return false;
  }
  if (transition === "disconnected") {
    handlers.onDisconnected();
  } else if (transition === "connected") {
    handlers.onConnected();
  }
  handlers.onReload();
  handlers.onRefetch();
  return true;
}
