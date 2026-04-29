import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import type {
  CancelConnectorConnectRequest,
  ClearConnectorRequest,
  ConnectorConnectSession,
  Connector,
  FinishConnectorConnectRequest,
  FinishConnectorConnectResult,
  GetConnectorConnectSessionRequest,
  OpenConnectorSiteRequest,
  StartConnectorConnectRequest,
  StartConnectorConnectResult,
  UpsertConnectorRequest,
} from "@/shared/contracts/connectors";
import {
  CancelConnectorConnect as CancelConnectorConnectBinding,
  ClearConnector as ClearConnectorBinding,
  FinishConnectorConnect as FinishConnectorConnectBinding,
  GetConnectorConnectSession as GetConnectorConnectSessionBinding,
  ListConnectors,
  OpenConnectorSite as OpenConnectorSiteBinding,
  StartConnectorConnect as StartConnectorConnectBinding,
  UpsertConnector as UpsertConnectorBinding,
} from "../../../bindings/xiadown/internal/presentation/wails/connectorshandler";
import {
  CancelConnectorConnectRequest as BindingsCancelConnectorConnectRequest,
  ClearConnectorRequest as BindingsClearConnectorRequest,
  ConnectorConnectSession as BindingsConnectorConnectSession,
  Connector as BindingsConnector,
  FinishConnectorConnectRequest as BindingsFinishConnectorConnectRequest,
  FinishConnectorConnectResult as BindingsFinishConnectorConnectResult,
  GetConnectorConnectSessionRequest as BindingsGetConnectorConnectSessionRequest,
  OpenConnectorSiteRequest as BindingsOpenConnectorSiteRequest,
  StartConnectorConnectRequest as BindingsStartConnectorConnectRequest,
  StartConnectorConnectResult as BindingsStartConnectorConnectResult,
  UpsertConnectorRequest as BindingsUpsertConnectorRequest,
} from "../../../bindings/xiadown/internal/application/connectors/dto/models";

export const CONNECTORS_QUERY_KEY = ["connectors"];
export const CONNECTOR_CONNECT_SESSION_QUERY_KEY = ["connector-connect-session"];

export function useConnectors() {
  return useQuery({
    queryKey: CONNECTORS_QUERY_KEY,
    queryFn: async (): Promise<Connector[]> => {
      return (await ListConnectors()).map(toConnector);
    },
    staleTime: 5_000,
  });
}

export function useUpsertConnector() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: UpsertConnectorRequest): Promise<Connector> => {
      return toConnector(await UpsertConnectorBinding(BindingsUpsertConnectorRequest.createFrom(request)));
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: CONNECTORS_QUERY_KEY });
    },
  });
}

export function useClearConnector() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: ClearConnectorRequest): Promise<void> => {
      await ClearConnectorBinding(BindingsClearConnectorRequest.createFrom(request));
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: CONNECTORS_QUERY_KEY });
    },
  });
}

export function useStartConnectorConnect() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: StartConnectorConnectRequest): Promise<StartConnectorConnectResult> => {
      return toStartConnectorConnectResult(
        await StartConnectorConnectBinding(BindingsStartConnectorConnectRequest.createFrom(request))
      );
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: CONNECTORS_QUERY_KEY });
    },
  });
}

export function useFinishConnectorConnect() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: FinishConnectorConnectRequest): Promise<FinishConnectorConnectResult> => {
      return toFinishConnectorConnectResult(
        await FinishConnectorConnectBinding(BindingsFinishConnectorConnectRequest.createFrom(request))
      );
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: CONNECTORS_QUERY_KEY });
    },
  });
}

export function useCancelConnectorConnect() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: CancelConnectorConnectRequest): Promise<void> => {
      await CancelConnectorConnectBinding(BindingsCancelConnectorConnectRequest.createFrom(request));
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: CONNECTORS_QUERY_KEY });
    },
  });
}

export function useOpenConnectorSite() {
  return useMutation({
    mutationFn: async (request: OpenConnectorSiteRequest): Promise<void> => {
      await OpenConnectorSiteBinding(BindingsOpenConnectorSiteRequest.createFrom(request));
    },
  });
}

export function useConnectorConnectSession(request: GetConnectorConnectSessionRequest, enabled: boolean) {
  return useQuery({
    queryKey: [...CONNECTOR_CONNECT_SESSION_QUERY_KEY, request.sessionId],
    enabled: enabled && request.sessionId.trim().length > 0,
    queryFn: async (): Promise<ConnectorConnectSession> => {
      return toConnectorConnectSession(
        await GetConnectorConnectSessionBinding(BindingsGetConnectorConnectSessionRequest.createFrom(request))
      );
    },
    refetchInterval: 1000,
    staleTime: 0,
  });
}

function toConnector(raw: BindingsConnector): Connector {
  return {
    ...raw,
    cookies: raw.cookies.map((item) => ({ ...item })),
  };
}

function toStartConnectorConnectResult(raw: BindingsStartConnectorConnectResult): StartConnectorConnectResult {
  return {
    ...raw,
    connector: toConnector(raw.connector),
  };
}

function toFinishConnectorConnectResult(raw: BindingsFinishConnectorConnectResult): FinishConnectorConnectResult {
  return {
    ...raw,
    connector: toConnector(raw.connector),
  };
}

function toConnectorConnectSession(raw: BindingsConnectorConnectSession): ConnectorConnectSession {
  return {
    ...raw,
    connector: toConnector(raw.connector),
  };
}
