import type { getXiaText } from "@/features/xiadown/shared";
import { getListenErrorCode } from "@/app/main/listen/api";

type LyricsText = ReturnType<typeof getXiaText>;

export type ListenLyricsPublicErrorCode =
  | "lyrics_auth_required"
  | "lyrics_network_unavailable"
  | "lyrics_provider_unavailable"
  | "lyrics_rate_limited"
  | "lyrics_timeout"
  | "lyrics_unavailable";

export type ListenLyricsErrorPresentation = {
  code: ListenLyricsPublicErrorCode;
  message: string;
  retryable: boolean;
};

export function resolveListenLyricsErrorPresentation(
  text: LyricsText,
  error: unknown,
): ListenLyricsErrorPresentation {
  const code = normalizeListenLyricsPublicErrorCode(
    getListenErrorCode(error),
  );
  return {
    code,
    message: listenLyricsErrorMessage(text, code),
    retryable: code !== "lyrics_auth_required",
  };
}

export function normalizeListenLyricsPublicErrorCode(
  value: string,
): ListenLyricsPublicErrorCode {
  switch (value.trim().toLowerCase()) {
    case "lyrics_auth_required":
      return "lyrics_auth_required";
    case "lyrics_network_unavailable":
      return "lyrics_network_unavailable";
    case "lyrics_provider_transient":
    case "lyrics_provider_unavailable":
      return "lyrics_provider_unavailable";
    case "lyrics_rate_limited":
      return "lyrics_rate_limited";
    case "lyrics_timeout":
      return "lyrics_timeout";
    default:
      return "lyrics_unavailable";
  }
}

function listenLyricsErrorMessage(
  text: LyricsText,
  code: ListenLyricsPublicErrorCode,
) {
  switch (code) {
    case "lyrics_network_unavailable":
      return text.listen.lyricsErrorNetworkUnavailable;
    case "lyrics_provider_unavailable":
      return text.listen.lyricsErrorProviderUnavailable;
    case "lyrics_rate_limited":
      return text.listen.lyricsErrorRateLimited;
    case "lyrics_timeout":
      return text.listen.lyricsErrorTimeout;
    case "lyrics_auth_required":
      return text.listen.lyricsErrorAuthRequired;
    case "lyrics_unavailable":
      return text.listen.lyricsErrorUnavailable;
  }
}
