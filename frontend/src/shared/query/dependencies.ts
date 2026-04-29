import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Call } from "@wailsio/runtime";

import type {
  Dependency,
  DependencyInstallState,
  DependencyUpdateInfo,
  InstallDependencyRequest,
  OpenDependencyDirectoryRequest,
  RemoveDependencyRequest,
  VerifyDependencyRequest,
} from "@/shared/contracts/dependencies";

export const DEPENDENCIES_QUERY_KEY = ["dependencies"];
export const DEPENDENCY_UPDATES_QUERY_KEY = ["dependencies-updates"];
export const DEPENDENCY_INSTALL_STATE_QUERY_KEY = ["dependency-install-state"];

type UseDependenciesOptions = {
  refetchInterval?: number | false;
  staleTime?: number;
};

export function useDependencies(options?: UseDependenciesOptions) {
  return useQuery({
    queryKey: DEPENDENCIES_QUERY_KEY,
    queryFn: async (): Promise<Dependency[]> => {
      const result = await Call.ByName("xiadown/internal/presentation/wails.DependenciesHandler.ListDependencies");
      return (result as Dependency[]) ?? [];
    },
    staleTime: options?.staleTime ?? 5_000,
    refetchInterval: options?.refetchInterval,
  });
}

export function useDependencyUpdates() {
  return useQuery({
    queryKey: DEPENDENCY_UPDATES_QUERY_KEY,
    queryFn: async (): Promise<DependencyUpdateInfo[]> => {
      const result = await Call.ByName("xiadown/internal/presentation/wails.DependenciesHandler.ListDependencyUpdates");
      return (result as DependencyUpdateInfo[]) ?? [];
    },
    staleTime: 60 * 60 * 1_000,
  });
}

export function useDependencyInstallState(name: string, enabled = true) {
  return useQuery({
    queryKey: [...DEPENDENCY_INSTALL_STATE_QUERY_KEY, name],
    queryFn: async (): Promise<DependencyInstallState> => {
      const result = await Call.ByName("xiadown/internal/presentation/wails.DependenciesHandler.GetDependencyInstallState", { name });
      return result as DependencyInstallState;
    },
    enabled: enabled && name.trim().length > 0,
    staleTime: 0,
    refetchInterval: enabled ? 500 : false,
  });
}

export function useInstallDependency() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: InstallDependencyRequest): Promise<Dependency> => {
      const result = await Call.ByName("xiadown/internal/presentation/wails.DependenciesHandler.InstallDependency", request);
      return result as Dependency;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: DEPENDENCIES_QUERY_KEY });
      queryClient.invalidateQueries({ queryKey: DEPENDENCY_UPDATES_QUERY_KEY });
      queryClient.invalidateQueries({ queryKey: DEPENDENCY_INSTALL_STATE_QUERY_KEY });
    },
  });
}

export function useRemoveDependency() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: RemoveDependencyRequest): Promise<void> => {
      await Call.ByName("xiadown/internal/presentation/wails.DependenciesHandler.RemoveDependency", request);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: DEPENDENCIES_QUERY_KEY });
      queryClient.invalidateQueries({ queryKey: DEPENDENCY_INSTALL_STATE_QUERY_KEY });
    },
  });
}

export function useVerifyDependency() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: VerifyDependencyRequest): Promise<Dependency> => {
      const result = await Call.ByName("xiadown/internal/presentation/wails.DependenciesHandler.VerifyDependency", request);
      return result as Dependency;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: DEPENDENCIES_QUERY_KEY });
    },
  });
}

export function useOpenDependencyDirectory() {
  return useMutation({
    mutationFn: async (request: OpenDependencyDirectoryRequest): Promise<void> => {
      await Call.ByName("xiadown/internal/presentation/wails.DependenciesHandler.OpenDependencyDirectory", request);
    },
  });
}
