export type ConnectorStatus = "connected" | "disconnected" | "expired";
export type ConnectorCredentialMode = "cookies" | "profile";
export type ConnectorCredentialState =
  | "connected"
  | "disconnected"
  | "expired"
  | "profile";

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

export interface ConnectorProfileComponent {
  name: string;
  path?: string;
  kind?: string;
  sizeBytes: number;
  fileCount: number;
  directoryCount: number;
}

export interface ConnectorProfile {
  path?: string;
  browser?: string;
  exists: boolean;
  sizeBytes: number;
  fileCount: number;
  directoryCount: number;
  components?: ConnectorProfileComponent[];
  bindings?: ConnectorProfileBinding[];
  truncated?: boolean;
  error?: string;
}

export interface ConnectorProfileBinding {
  browser: string;
  path?: string;
  exists: boolean;
  current?: boolean;
  sizeBytes: number;
  fileCount: number;
  directoryCount: number;
}

export interface ConnectorSite {
  key?: string;
  label?: string;
  url: string;
}

export interface Connector {
  id: string;
  type: string;
  group?: string;
  desc?: string;
  status: ConnectorStatus | string;
  credentialMode?: ConnectorCredentialMode | string;
  credentialState?: ConnectorCredentialState | string;
  cookiesCount?: number;
  cookies?: ConnectorCookie[];
  profileKey?: string;
  profilePath?: string;
  profileBrowser?: string;
  profileInfo?: ConnectorProfile;
  domains?: string[];
  profileSites?: ConnectorSite[];
  policyKey?: string;
  capabilities?: string[];
  lastVerifiedAt?: string;
}

export interface UpsertConnectorRequest {
  id?: string;
  type?: string;
  status?: ConnectorStatus | string;
  credentialMode?: ConnectorCredentialMode | string;
  cookiesPath?: string;
}

export interface ClearConnectorRequest {
  id: string;
}

export interface StartConnectorConnectRequest {
  id: string;
  targetUrl?: string;
}

export interface StartConnectorConnectResult {
  sessionId: string;
  connector: Connector;
  targetUrl?: string;
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
  targetUrl?: string;
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
  targetUrl?: string;
}
