import type { getXiaText } from "@/features/xiadown/shared";
import type { ResourceSniffFailure } from "@/shared/contracts/library";
import {
  getAppErrorCode,
  parseAppErrorMessage,
  resolveUnknownErrorMessage,
} from "@/app/main/helpers";

type XiaText = ReturnType<typeof getXiaText>;

function formatTemplate(template: string, params: Record<string, string>) {
  return Object.entries(params).reduce(
    (output, [key, value]) => output.split(`{${key}}`).join(value),
    template,
  );
}

function normalized(value?: string) {
  return (value ?? "").trim().toLowerCase();
}

function extractUnsupportedSniffDomain(message: string) {
  const match = message.match(/resource sniff does not support\s+([^\s,]+)/i);
  return (match?.[1]?.trim() ?? "").replace(/[.;:]+$/, "");
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

export function resolveStartSniffFailureDescription(
  text: XiaText,
  failure: ResourceSniffFailure,
) {
  const descriptions: Partial<Record<ResourceSniffFailure["code"], string>> = {
    profile_connection_required: text.dialogs.profileConnectionRequired,
    verification_required: text.dialogs.resourceVerificationRequired,
    no_media_detected: text.dialogs.resourceNoMediaDetected,
    unsupported_douyin_lvdetail: text.dialogs.resourceDouyinLVDetail,
    douyin_recommend_login_required:
      text.dialogs.resourceDouyinRecommendLoginRequired,
  };
  return descriptions[failure.code] || failure.detail || text.common.unknown;
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
  if (normalizedMessage.includes("url is required")) {
    return text.sniffDesk.urlRequired;
  }
  if (
    normalizedMessage.includes("invalid url") ||
    normalizedMessage.includes("unsupported video path")
  ) {
    return text.sniffDesk.urlInvalid;
  }
  if (
    resolvedCode === "resource_unsupported_domain" ||
    normalizedMessage.includes("resource sniff does not support")
  ) {
    const domain = extractUnsupportedSniffDomain(message);
    return domain
      ? formatTemplate(text.sniffDesk.urlUnsupportedDomain, { domain })
      : text.sniffDesk.urlUnsupported;
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
