import type { getXiaText } from "@/features/xiadown/shared";
import {
  getAppErrorCode,
  parseAppErrorMessage,
  resolveUnknownErrorMessage,
} from "@/app/main/helpers";

type XiaText = ReturnType<typeof getXiaText>;

function normalized(value?: string) {
  return (value ?? "").trim().toLowerCase();
}

function resolveResourceSniffResolveError(text: XiaText, message: string) {
  const value = normalized(message);
  const matches: Array<[string, string]> = [
    ["resource sniff session is required", text.sniffDesk.errors.sessionRequired],
    ["resource sniff session not found", text.sniffDesk.errors.sessionNotFound],
    ["resource sniff preview resource is required", text.sniffDesk.errors.previewRequired],
    ["resource sniff preview is required", text.sniffDesk.errors.previewRequired],
    ["resource sniff preview is unavailable", text.sniffDesk.errors.previewUnavailable],
    ["resource sniff resource is required", text.sniffDesk.errors.resourceRequired],
    ["resource sniff raw resource not found", text.sniffDesk.errors.resourceNotFound],
    ["resource sniff raw resource is not previewable", text.sniffDesk.errors.notPreviewable],
    ["resource sniff raw resource url is required", text.sniffDesk.errors.resourceUrlRequired],
    ["resource sniff raw resource url is not fetchable", text.sniffDesk.errors.resourceUrlNotFetchable],
    ["resource sniff raw resource is not downloadable", text.sniffDesk.errors.notDownloadable],
    ["resource sniff media snapshot is unavailable", text.sniffDesk.errors.mediaSnapshotUnavailable],
    ["resource sniff result is unavailable", text.sniffDesk.errors.resultUnavailable],
    ["resource sniff tab is unavailable", text.sniffDesk.errors.tabUnavailable],
  ];
  return matches.find(([needle]) => value.includes(needle))?.[1] ?? "";
}

export function resolveSniffDeskErrorDescription(text: XiaText, error: unknown) {
  const code = getAppErrorCode(error);
  const rawMessage = resolveUnknownErrorMessage(error, text.common.unknown);
  const parsed = parseAppErrorMessage(rawMessage);
  const message = parsed.message || rawMessage;
  const resolvedCode = code || parsed.code;
  const normalizedMessage = normalized(message);

  if (
    normalizedMessage.includes("context canceled") ||
    normalizedMessage.includes("operation canceled")
  ) {
    return text.sniffDesk.errors.operationCanceled;
  }
  if (resolvedCode === "resource_verification_required") {
    return text.dialogs.resourceVerificationRequired;
  }
  if (resolvedCode === "resource_no_media_detected") {
    return text.dialogs.resourceNoMediaDetected;
  }
  if (
    resolvedCode === "resource_browser_unavailable" ||
    normalizedMessage.includes("no supported browser detected") ||
    normalizedMessage.includes("resource sniff browser unavailable")
  ) {
    return text.sniffDesk.errors.browserUnavailable;
  }
  if (resolvedCode === "resource_current_browser_not_running") {
    return text.sniffDesk.errors.currentBrowserNotRunning;
  }
  if (resolvedCode === "resource_current_browser_remote_debugging_required") {
    return text.sniffDesk.errors.currentBrowserRemoteDebugging;
  }
  if (resolvedCode === "resource_current_browser_permission_required") {
    return text.sniffDesk.errors.currentBrowserPermission;
  }
  if (resolvedCode === "resource_current_browser_unsupported") {
    return text.sniffDesk.errors.currentBrowserUnsupported;
  }
  if (resolvedCode === "resource_current_browser_endpoint_unavailable") {
    return text.sniffDesk.errors.currentBrowserEndpoint;
  }
  if (resolvedCode === "resource_browser_launch_failed") {
    if (normalizedMessage.includes("resource sniff browser is closed")) {
      return text.sniffDesk.errors.browserClosed;
    }
    if (normalizedMessage.includes("resource sniff browser is closing")) {
      return text.sniffDesk.errors.browserClosing;
    }
    return text.sniffDesk.errors.browserLaunchFailed;
  }
  if (resolvedCode === "resource_resolve_failed") {
    return (
      resolveResourceSniffResolveError(text, message) ||
      text.sniffDesk.errors.resolveFailed
    );
  }

  return message || text.common.unknown;
}
