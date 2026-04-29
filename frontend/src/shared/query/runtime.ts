import { useQuery } from "@tanstack/react-query";
import { Call } from "@wailsio/runtime";

export const HTTP_BASE_URL_QUERY_KEY = ["runtime", "http-base-url"] as const;

export function useHttpBaseURL() {
  return useQuery({
    queryKey: HTTP_BASE_URL_QUERY_KEY,
    queryFn: async (): Promise<string> => {
      const value = await Call.ByName("xiadown/internal/presentation/wails.RealtimeHandler.HTTPBaseURL");
      return typeof value === "string" ? value : String(value ?? "");
    },
    staleTime: Infinity,
    retry: false,
  });
}
