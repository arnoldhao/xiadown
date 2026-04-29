export type ConnectorStatus = "connected" | "disconnected" | "expired";

export interface ConnectorCookie {
  name: string;
  value: string;
  domain: string;
  path: string;
  expires: number;
  httpOnly: boolean;
  secure: boolean;
  sameSite?: string;
}

export interface Connector {
  id: string;
  type: string;
  group?: string;
  desc?: string;
  status: ConnectorStatus | string;
  cookiesCount?: number;
  cookies?: ConnectorCookie[];
  domains?: string[];
  policyKey?: string;
  capabilities?: string[];
  lastVerifiedAt?: string;
}

export interface UpsertConnectorRequest {
  id?: string;
  type?: string;
  status?: ConnectorStatus | string;
  cookiesPath?: string;
}

export interface ClearConnectorRequest {
  id: string;
}

export interface StartConnectorConnectRequest {
  id: string;
}

export interface StartConnectorConnectResult {
  sessionId: string;
  connector: Connector;
}

export interface FinishConnectorConnectRequest {
  sessionId: string;
}

export interface FinishConnectorConnectResult {
  sessionId: string;
  saved: boolean;
  rawCookiesCount: number;
  filteredCookiesCount: number;
  domains?: string[];
  reason?: string;
  connector: Connector;
}

export interface CancelConnectorConnectRequest {
  sessionId: string;
}

export interface ConnectorConnectSession {
  sessionId: string;
  connectorId: string;
  state: string;
  browserStatus: string;
  currentCookiesCount: number;
  saved: boolean;
  rawCookiesCount: number;
  filteredCookiesCount: number;
  domains?: string[];
  reason?: string;
  error?: string;
  lastCookiesAt?: string;
  connector: Connector;
}

export interface GetConnectorConnectSessionRequest {
  sessionId: string;
}

export interface OpenConnectorSiteRequest {
  id: string;
}
