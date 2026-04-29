import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Call } from "@wailsio/runtime";

import {
  normalizeUpdateInfo,
  normalizeWhatsNewInfo,
  type UpdateInfo,
  type WhatsNewInfo,
} from "@/shared/store/update";

const UPDATE_QUERY_KEY = ["update-state"];
const WHATS_NEW_QUERY_KEY = ["whats-new"];

export function useUpdateState() {
  return useQuery({
    queryKey: UPDATE_QUERY_KEY,
    queryFn: async (): Promise<UpdateInfo> => {
      const result = await Call.ByName("xiadown/internal/presentation/wails.UpdateHandler.GetState");
      return normalizeUpdateInfo(result as Partial<UpdateInfo>);
    },
    staleTime: 30_000,
  });
}

export function useCheckForUpdate() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (currentVersion: string): Promise<UpdateInfo> => {
      const result = await Call.ByName(
        "xiadown/internal/presentation/wails.UpdateHandler.CheckForUpdate",
        currentVersion
      );
      return normalizeUpdateInfo(result as Partial<UpdateInfo>);
    },
    onSuccess: (data) => {
      queryClient.setQueryData(UPDATE_QUERY_KEY, data);
    },
  });
}

export function useDownloadUpdate() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (): Promise<UpdateInfo> => {
      const result = await Call.ByName("xiadown/internal/presentation/wails.UpdateHandler.DownloadUpdate");
      return normalizeUpdateInfo(result as Partial<UpdateInfo>);
    },
    onSuccess: (data) => {
      queryClient.setQueryData(UPDATE_QUERY_KEY, data);
    },
  });
}

export function useRestartToApply() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (): Promise<UpdateInfo> => {
      const result = await Call.ByName("xiadown/internal/presentation/wails.UpdateHandler.RestartToApply");
      return normalizeUpdateInfo(result as Partial<UpdateInfo>);
    },
    onSuccess: (data) => {
      queryClient.setQueryData(UPDATE_QUERY_KEY, data);
    },
  });
}

export function useWhatsNew() {
  return useQuery({
    queryKey: WHATS_NEW_QUERY_KEY,
    queryFn: async (): Promise<WhatsNewInfo | null> => {
      const result = await Call.ByName("xiadown/internal/presentation/wails.UpdateHandler.GetWhatsNew");
      return normalizeWhatsNewInfo(result as Partial<WhatsNewInfo>);
    },
    staleTime: Infinity,
  });
}

export function useDismissWhatsNew() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (version: string): Promise<void> => {
      await Call.ByName(
        "xiadown/internal/presentation/wails.UpdateHandler.DismissWhatsNew",
        version
      );
    },
    onSuccess: () => {
      queryClient.setQueryData(WHATS_NEW_QUERY_KEY, null);
    },
  });
}

export { UPDATE_QUERY_KEY, WHATS_NEW_QUERY_KEY };
