export type AppSessionStatus = "connected" | "disconnected" | "expired";
export type AppSessionCredentialState =
  | "app_session"
  | "connected"
  | "disconnected"
  | "expired";
export type AppSessionAccountVerificationStatus =
  | "verifying"
  | "verified"
  | "unverified"
  | "unsupported";

export interface AppSessionCookie {
  name: string;
  value: string;
  domain: string;
  path: string;
  expires: number;
  httpOnly: boolean;
  secure: boolean;
  sameSite?: string;
}

export interface AppSessionBadge {
  key?: string;
  label?: string;
}

export interface AppSessionAccount {
  displayName?: string;
  handle?: string;
  avatarURL?: string;
  tierKey?: string;
  tierLabel?: string;
  badges?: AppSessionBadge[];
  metadata?: Record<string, unknown>;
  expiresAt?: string;
}

export interface AppSessionSource {
  mode?: "browser_profile" | "xiadown_profile" | string;
  browserId?: string;
  browserLabel?: string;
  profileId?: string;
  profileLabel?: string;
  syncedAt?: string;
}

export interface AppSession {
  id: string;
  siteKey: string;
  group?: string;
  label?: string;
  desc?: string;
  status: AppSessionStatus | string;
  credentialState?: AppSessionCredentialState | string;
  cookiesCount?: number;
  cookies?: AppSessionCookie[];
  domains?: string[];
  account?: AppSessionAccount | null;
  policyKey?: string;
  capabilities?: string[];
  providerSupported: boolean;
  accountVerificationStatus?: AppSessionAccountVerificationStatus | string;
  accountVerificationError?: string;
  accountVerificationStartedAt?: string;
  lastVerifiedAt?: string;
  sourceType?: string;
  sourceBrowser?: string;
  sourceProfile?: string;
  lastSyncedAt?: string;
  source?: AppSessionSource | null;
}

export interface ClearAppSessionRequest {
  id: string;
}

export interface StartAppSessionConnectRequest {
  id: string;
  targetUrl?: string;
}

export interface StartAppSessionConnectResult {
  sessionId: string;
  appSession: AppSession;
  targetUrl?: string;
}

export interface FinishAppSessionConnectRequest {
  sessionId: string;
}

export interface FinishAppSessionConnectResult {
  sessionId: string;
  saved: boolean;
  rawCookiesCount: number;
  filteredCookiesCount: number;
  domains?: string[];
  reason?: string;
  appSession: AppSession;
}

export interface CancelAppSessionConnectRequest {
  sessionId: string;
}

export interface AppSessionConnectSession {
  sessionId: string;
  appSessionId: string;
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
  appSession: AppSession;
}

export interface GetAppSessionConnectSessionRequest {
  sessionId: string;
}

export interface OpenAppSessionSiteRequest {
  id: string;
  targetUrl?: string;
}

export interface VerifyAppSessionRequest {
  id: string;
}

export interface BrowserProfileSelection {
  mode?: string;
  browserId: string;
  profileId: string;
}

export interface BrowserProfileDiscoveryRequest {
  browserId: string;
}

export interface AppSessionBrowserScanItem {
  appSessionId: string;
  siteKey: string;
  label: string;
  accountLabel?: string;
  status: string;
  selectable: boolean;
  reason?: string;
}

export interface AppSessionBrowserScanResult {
  browserId: string;
  profileId: string;
  snapshotToken: string;
  items: AppSessionBrowserScanItem[];
}

export interface AppSessionBrowserImportRequest {
  mode?: string;
  browserId: string;
  profileId: string;
  snapshotToken: string;
  appSessionIds: string[];
}

export interface AppSessionBrowserImportResult {
  importedIds: string[];
  skippedIds: string[];
}
