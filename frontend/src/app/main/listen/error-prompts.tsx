import { Link2, RefreshCw } from "lucide-react";
import type { ReactNode } from "react";

import type { ListenPageProps } from "@/app/main/listen/types";

export type ListenLibraryErrorPrompt = {
  message: string;
  actionLabel?: string;
  action?: "connections" | "refresh";
  icon?: ReactNode;
};

export function resolveListenLibraryErrorPrompt(
  code: string,
  text: ListenPageProps["text"],
  retryable = false,
): ListenLibraryErrorPrompt {
  switch (code.trim()) {
    case "youtube_cookies_missing":
    case "youtube_not_authenticated":
      return {
        message: text.listen.onlineAuthRequired,
        actionLabel: text.listen.openConnections,
        action: "connections" as const,
        icon: <Link2 className="h-5 w-5" />,
      };
    case "youtube_auth_expired":
      return {
        message: text.listen.onlineAuthExpired,
        actionLabel: text.listen.openConnections,
        action: "connections" as const,
        icon: <Link2 className="h-5 w-5" />,
      };
    case "youtube_timeout":
    case "youtube_network_unavailable":
    case "youtube_tls_unavailable":
      return {
        message: text.listen.onlineNetworkUnavailable,
        actionLabel: text.listen.refresh,
        action: "refresh" as const,
        icon: <RefreshCw className="h-5 w-5" />,
      };
    case "youtube_region_unavailable":
      return {
        message: text.listen.onlineRegionUnavailable,
      };
    case "youtube_transient_unavailable":
      return {
        message: text.listen.onlineTransientUnavailable,
        actionLabel: text.listen.refresh,
        action: "refresh" as const,
        icon: <RefreshCw className="h-5 w-5" />,
      };
    default:
      return {
        message: text.listen.onlineServiceUnavailable,
        ...(retryable
          ? {
              actionLabel: text.listen.refresh,
              action: "refresh" as const,
              icon: <RefreshCw className="h-5 w-5" />,
            }
          : {}),
      };
  }
}
