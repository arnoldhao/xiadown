import type {
  LibraryAccessStatus,
  LibraryAccessTransportStatus,
  LibraryDeviceScope,
} from "@/shared/contracts/library-access";

export type LibraryAccessStatusTone = "neutral" | "pending" | "success" | "danger";

export type LibraryAccessBackendErrorText = {
  tailscaleServeNotEnabled: string;
  tailscaleCLIUnavailable: string;
  requestTimedOut: string;
  networkUnavailable: string;
  requestFailed: string;
};

const READY_STATES = new Set(["active", "configured", "listening", "ready", "running"]);
const PENDING_STATES = new Set(["connecting", "starting"]);
const ERROR_STATES = new Set(["disconnected", "error", "unavailable"]);

export const LIBRARY_PAIRING_QR_OPTIONS = {
  size: 184,
  level: "M",
  bgColor: "#FFFFFF",
  fgColor: "#111827",
  marginSize: 4,
} as const;

export const LIBRARY_DEVICE_SCOPE_ORDER: readonly LibraryDeviceScope[] = [
  "library.read",
  "music.read",
  "music.state",
  "music.manage",
  "rss.read",
  "rss.state",
  "rss.manage",
  "rss.fetch",
  "tasks.read",
  "tasks.create",
  "tasks.control",
];

export const DEFAULT_LIBRARY_DEVICE_SCOPES: readonly LibraryDeviceScope[] = [
  "library.read",
  "tasks.read",
];

export function normalizeLibraryAccessPath(value: string): string {
  const trimmed = value.trim();
  if (!trimmed || trimmed === "/") {
    return "/xiadown";
  }
  return `/${trimmed.replace(/^\/+|\/+$/g, "")}`;
}

export function normalizeLibraryAccessPort(value: string, fallback: number): number {
  const parsed = Number.parseInt(value, 10);
  return Number.isInteger(parsed) && parsed >= 1 && parsed <= 65_535 ? parsed : fallback;
}

export function isValidLibraryPairingLink(value: string, version: number): boolean {
  const trimmed = value.trim();
  if (version !== 1 || !trimmed || trimmed.length > 2_048) {
    return false;
  }
  let parsed: URL;
  try {
    parsed = new URL(trimmed);
  } catch {
    return false;
  }
  const query = parsed.searchParams;
  if (parsed.protocol !== "xiadown:" || parsed.hostname !== "pair" || parsed.pathname ||
    parsed.username || parsed.password || parsed.hash || query.get("v") !== String(version) ||
    !/^[A-Za-z0-9_-]{16,}$/.test(query.get("nonce") ?? "") ||
    !/^\d{6}$/.test(query.get("code") ?? "") ||
    !Number.isFinite(Date.parse(query.get("expires") ?? "")) ||
    !/^[0-9A-Fa-f]{64}$/.test(query.get("fingerprint") ?? "")) {
    return false;
  }
  const endpoints = [...query.getAll("lan"), ...query.getAll("remote")];
  return endpoints.length > 0 && endpoints.every((endpoint) => {
    try {
      const transport = new URL(endpoint);
      return transport.protocol === "https:" && Boolean(transport.hostname) &&
        !transport.username && !transport.password && !transport.search && !transport.hash &&
        transport.pathname.endsWith("/");
    } catch {
      return false;
    }
  });
}

export function resolveLibraryAccessTransportTone(
  transport?: LibraryAccessTransportStatus | null,
): LibraryAccessStatusTone {
  if (!transport?.desiredEnabled) {
    return "neutral";
  }
  const state = (transport.state ?? "").trim().toLowerCase();
  if (transport.lastError?.trim() || ERROR_STATES.has(state)) {
    return "danger";
  }
  if (READY_STATES.has(state)) {
    return "success";
  }
  return PENDING_STATES.has(state) || state ? "pending" : "neutral";
}

export function resolveLibraryAccessStatusTone(
  status?: LibraryAccessStatus | null,
): LibraryAccessStatusTone {
  if (!status?.desiredEnabled) {
    return "neutral";
  }
  const tones = [
    resolveLibraryAccessTransportTone(status.lan),
    resolveLibraryAccessTransportTone(status.tailscale),
  ];
  if (tones.includes("success")) {
    return "success";
  }
  if (tones.includes("danger")) {
    return "danger";
  }
  return tones.includes("pending") ? "pending" : "neutral";
}

export function libraryAccessErrorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message.trim();
  }
  return typeof error === "string" ? error.trim() : "";
}

export function safeLibraryAccessBackendErrorMessage(
  error: unknown,
  text: LibraryAccessBackendErrorText,
): string {
  const message = libraryAccessErrorMessage(error).toLowerCase();
  if (!message) {
    return "";
  }
  if (
    /serve (?:is )?(?:not enabled|disabled)/.test(message) ||
    message.includes("login.tailscale.com/f/serve")
  ) {
    return text.tailscaleServeNotEnabled;
  }
  if (
    /tailscale(?:\.exe)?[^\n]*(?:not installed|not found|unavailable)/.test(message) ||
    message.includes("executable file not found") ||
    message.includes("command not found") ||
    message.includes("cannot find the file specified")
  ) {
    return text.tailscaleCLIUnavailable;
  }
  if (
    message.includes("timed out") ||
    message.includes("timeout") ||
    message.includes("deadline exceeded")
  ) {
    return text.requestTimedOut;
  }
  if (
    /network (?:is )?(?:unavailable|unreachable|offline)/.test(message) ||
    message.includes("no route to host") ||
    message.includes("connection refused") ||
    message.includes("connection reset") ||
    message.includes("failed to connect") ||
    message.includes("temporary failure in name resolution") ||
    message.includes("tailscale is disconnected") ||
    message.includes("dial tcp")
  ) {
    return text.networkUnavailable;
  }
  return text.requestFailed;
}

export function toggleLibraryDeviceScope(
  current: readonly LibraryDeviceScope[],
  scope: LibraryDeviceScope,
): LibraryDeviceScope[] {
  const next = new Set(current);
  if (next.has(scope)) {
    next.delete(scope);
  } else {
    next.add(scope);
  }
  if (next.size === 0) {
    return [...current];
  }
  return LIBRARY_DEVICE_SCOPE_ORDER.filter((candidate) => next.has(candidate));
}

export function isLibraryAccessRevisionConflict(error: unknown): boolean {
  return /revision\s+conflict/i.test(libraryAccessErrorMessage(error));
}
