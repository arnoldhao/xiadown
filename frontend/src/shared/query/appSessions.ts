import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import type {
  AppSession,
  AppSessionConnectSession,
  CancelAppSessionConnectRequest,
  ClearAppSessionRequest,
  FinishAppSessionConnectRequest,
  FinishAppSessionConnectResult,
  GetAppSessionConnectSessionRequest,
  OpenAppSessionSiteRequest,
  StartAppSessionConnectRequest,
  StartAppSessionConnectResult,
  VerifyAppSessionRequest,
} from "@/shared/contracts/appSessions";
import {
  AppSession as BindingsAppSession,
  AppSessionConnectSession as BindingsAppSessionConnectSession,
  CancelAppSessionConnectRequest as BindingsCancelAppSessionConnectRequest,
  ClearAppSessionRequest as BindingsClearAppSessionRequest,
  FinishAppSessionConnectRequest as BindingsFinishAppSessionConnectRequest,
  FinishAppSessionConnectResult as BindingsFinishAppSessionConnectResult,
  GetAppSessionConnectSessionRequest as BindingsGetAppSessionConnectSessionRequest,
  OpenAppSessionSiteRequest as BindingsOpenAppSessionSiteRequest,
  StartAppSessionConnectRequest as BindingsStartAppSessionConnectRequest,
  StartAppSessionConnectResult as BindingsStartAppSessionConnectResult,
  VerifyAppSessionRequest as BindingsVerifyAppSessionRequest,
} from "../../../bindings/xiadown/internal/application/appsessions/dto/models";
import { loadAppSessionsHandlerBindings } from "./appSessionsBindings";

export const APP_SESSIONS_QUERY_KEY = ["app-sessions"];
export const APP_SESSION_CONNECT_SESSION_QUERY_KEY = ["app-session-connect-session"];
export const APP_SESSIONS_CHANGED_EVENT = "app-sessions:changed";

export function useAppSessions() {
  return useQuery({
    queryKey: APP_SESSIONS_QUERY_KEY,
    queryFn: async (): Promise<AppSession[]> => {
      const bindings = await loadAppSessionsHandlerBindings();
      return (await bindings.ListAppSessions()).map(toAppSession);
    },
    staleTime: 5_000,
  });
}

export function useClearAppSession() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: ClearAppSessionRequest): Promise<void> => {
      const bindings = await loadAppSessionsHandlerBindings();
      await bindings.ClearAppSession(BindingsClearAppSessionRequest.createFrom(request));
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: APP_SESSIONS_QUERY_KEY });
    },
  });
}

export function useStartAppSessionConnect() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: StartAppSessionConnectRequest): Promise<StartAppSessionConnectResult> => {
      const bindings = await loadAppSessionsHandlerBindings();
      return toStartAppSessionConnectResult(
        await bindings.StartAppSessionConnect(BindingsStartAppSessionConnectRequest.createFrom(request)),
      );
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: APP_SESSIONS_QUERY_KEY });
    },
  });
}

export function useFinishAppSessionConnect() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: FinishAppSessionConnectRequest): Promise<FinishAppSessionConnectResult> => {
      const bindings = await loadAppSessionsHandlerBindings();
      return toFinishAppSessionConnectResult(
        await bindings.FinishAppSessionConnect(BindingsFinishAppSessionConnectRequest.createFrom(request)),
      );
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: APP_SESSIONS_QUERY_KEY });
    },
  });
}

export function useCancelAppSessionConnect() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: CancelAppSessionConnectRequest): Promise<void> => {
      const bindings = await loadAppSessionsHandlerBindings();
      await bindings.CancelAppSessionConnect(BindingsCancelAppSessionConnectRequest.createFrom(request));
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: APP_SESSIONS_QUERY_KEY });
    },
  });
}

export function useOpenAppSessionSite() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: OpenAppSessionSiteRequest): Promise<StartAppSessionConnectResult> => {
      const bindings = await loadAppSessionsHandlerBindings();
      return toStartAppSessionConnectResult(
        await bindings.OpenAppSessionSite(BindingsOpenAppSessionSiteRequest.createFrom(request)),
      );
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: APP_SESSIONS_QUERY_KEY });
    },
  });
}

export function useVerifyAppSession() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: VerifyAppSessionRequest): Promise<AppSession> => {
      const bindings = await loadAppSessionsHandlerBindings();
      return toAppSession(
        await bindings.VerifyAppSession(BindingsVerifyAppSessionRequest.createFrom(request)),
      );
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: APP_SESSIONS_QUERY_KEY });
    },
  });
}

export function useAppSessionConnectSession(request: GetAppSessionConnectSessionRequest, enabled: boolean) {
  return useQuery({
    queryKey: [...APP_SESSION_CONNECT_SESSION_QUERY_KEY, request.sessionId],
    enabled: enabled && request.sessionId.trim().length > 0,
    queryFn: async (): Promise<AppSessionConnectSession> => {
      const bindings = await loadAppSessionsHandlerBindings();
      return toAppSessionConnectSession(
        await bindings.GetAppSessionConnectSession(BindingsGetAppSessionConnectSessionRequest.createFrom(request)),
      );
    },
    refetchInterval: 1000,
    staleTime: 0,
  });
}

function toAppSession(raw: BindingsAppSession): AppSession {
  const extended = raw as BindingsAppSession & {
    source?: AppSession["source"];
    sourceType?: string;
    sourceBrowser?: string;
    sourceProfile?: string;
    lastSyncedAt?: string;
  };
  const source = extended.source ?? (
    extended.sourceType || extended.sourceBrowser || extended.sourceProfile
      ? {
          mode: extended.sourceType === "browser_profile" ? "browser_profile" : "xiadown_profile",
          browserLabel: extended.sourceBrowser,
          profileLabel: extended.sourceProfile,
          syncedAt: extended.lastSyncedAt,
        }
      : undefined
  );
  return {
    ...raw,
    sourceType: extended.sourceType,
    sourceBrowser: extended.sourceBrowser,
    sourceProfile: extended.sourceProfile,
    lastSyncedAt: extended.lastSyncedAt,
    source: source ? { ...source } : undefined,
    cookies: (raw.cookies ?? []).map((item) => ({ ...item })),
    domains: [...(raw.domains ?? [])],
    capabilities: [...(raw.capabilities ?? [])],
    account: raw.account
      ? {
          ...raw.account,
          badges: (raw.account.badges ?? []).map((item) => ({ ...item })),
          metadata: { ...(raw.account.metadata ?? {}) },
        }
      : null,
  };
}

function toStartAppSessionConnectResult(raw: BindingsStartAppSessionConnectResult): StartAppSessionConnectResult {
  return {
    ...raw,
    appSession: toAppSession(raw.appSession),
  };
}

function toFinishAppSessionConnectResult(raw: BindingsFinishAppSessionConnectResult): FinishAppSessionConnectResult {
  return {
    ...raw,
    appSession: toAppSession(raw.appSession),
  };
}

function toAppSessionConnectSession(raw: BindingsAppSessionConnectSession): AppSessionConnectSession {
  return {
    ...raw,
    appSession: toAppSession(raw.appSession),
  };
}
