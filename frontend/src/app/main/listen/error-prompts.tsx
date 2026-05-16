import { Link2, RefreshCw } from "lucide-react";

import type { ListenPageProps } from "@/app/main/listen/types";

export function resolveListenLibraryErrorPrompt(
  code: string,
  text: ListenPageProps["text"],
) {
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
      return {
        message: text.listen.onlineNetworkUnavailable,
        actionLabel: text.listen.refresh,
        action: "refresh" as const,
        icon: <RefreshCw className="h-5 w-5" />,
      };
    default:
      return {
        message: text.listen.onlineServiceUnavailable,
        actionLabel: text.listen.refresh,
        action: "refresh" as const,
        icon: <RefreshCw className="h-5 w-5" />,
      };
  }
}
