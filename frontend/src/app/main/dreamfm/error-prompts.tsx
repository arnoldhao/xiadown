import { Link2, RefreshCw } from "lucide-react";

import type { DreamFMPageProps } from "@/app/main/dreamfm/types";

export function resolveDreamFMLibraryErrorPrompt(
  code: string,
  text: DreamFMPageProps["text"],
) {
  switch (code.trim()) {
    case "youtube_cookies_missing":
    case "youtube_not_authenticated":
      return {
        message: text.dreamFm.onlineAuthRequired,
        actionLabel: text.dreamFm.openConnections,
        action: "connections" as const,
        icon: <Link2 className="h-5 w-5" />,
      };
    case "youtube_auth_expired":
      return {
        message: text.dreamFm.onlineAuthExpired,
        actionLabel: text.dreamFm.openConnections,
        action: "connections" as const,
        icon: <Link2 className="h-5 w-5" />,
      };
    case "youtube_timeout":
    case "youtube_network_unavailable":
      return {
        message: text.dreamFm.onlineNetworkUnavailable,
        actionLabel: text.dreamFm.refresh,
        action: "refresh" as const,
        icon: <RefreshCw className="h-5 w-5" />,
      };
    default:
      return {
        message: text.dreamFm.onlineServiceUnavailable,
        actionLabel: text.dreamFm.refresh,
        action: "refresh" as const,
        icon: <RefreshCw className="h-5 w-5" />,
      };
  }
}
