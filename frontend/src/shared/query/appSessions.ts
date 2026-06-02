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
} from "@/shared/contracts/appSessions";
import {
  CancelAppSessionConnect as CancelAppSessionConnectBinding,
  ClearAppSession as ClearAppSessionBinding,
  FinishAppSessionConnect as FinishAppSessionConnectBinding,
  GetAppSessionConnectSession as GetAppSessionConnectSessionBinding,
  ListAppSessions,
  OpenAppSessionSite as OpenAppSessionSiteBinding,
  StartAppSessionConnect as StartAppSessionConnectBinding,
} from "../../../bindings/xiadown/internal/presentation/wails/appsessionshandler";
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
} from "../../../bindings/xiadown/internal/application/appsessions/dto/models";

export const APP_SESSIONS_QUERY_KEY = ["app-sessions"];
export const APP_SESSION_CONNECT_SESSION_QUERY_KEY = ["app-session-connect-session"];
export const APP_SESSIONS_CHANGED_EVENT = "app-sessions:changed";

export function useAppSessions() {
  return useQuery({
    queryKey: APP_SESSIONS_QUERY_KEY,
    queryFn: async (): Promise<AppSession[]> => {
      return (await ListAppSessions()).map(toAppSession);
    },
    staleTime: 5_000,
  });
}

export function useClearAppSession() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: ClearAppSessionRequest): Promise<void> => {
      await ClearAppSessionBinding(BindingsClearAppSessionRequest.createFrom(request));
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
      return toStartAppSessionConnectResult(
        await StartAppSessionConnectBinding(BindingsStartAppSessionConnectRequest.createFrom(request)),
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
      return toFinishAppSessionConnectResult(
        await FinishAppSessionConnectBinding(BindingsFinishAppSessionConnectRequest.createFrom(request)),
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
      await CancelAppSessionConnectBinding(BindingsCancelAppSessionConnectRequest.createFrom(request));
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
      return toStartAppSessionConnectResult(
        await OpenAppSessionSiteBinding(BindingsOpenAppSessionSiteRequest.createFrom(request)),
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
      return toAppSessionConnectSession(
        await GetAppSessionConnectSessionBinding(BindingsGetAppSessionConnectSessionRequest.createFrom(request)),
      );
    },
    refetchInterval: 1000,
    staleTime: 0,
  });
}

function toAppSession(raw: BindingsAppSession): AppSession {
  return {
    ...raw,
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
