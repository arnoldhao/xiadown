export type BrowserSourceIntent = "app_session" | "network_sniff";

export type BrowserSourceMode =
  | "browser_profile"
  | "current_browser"
  | "xiadown_profile";

export interface BrowserSourceProfile {
  id: string;
  label?: string;
  browserId?: string;
  browserLabel?: string;
  subtitle?: string;
  displayPath?: string;
  path?: string;
  isDefault?: boolean;
  redundant?: boolean;
  available: boolean;
  error?: string;
  sizeBytes?: number;
  state?: string;
  virtual?: boolean;
}

export interface BrowserSourceBrowser {
  id: string;
  label: string;
  available: boolean;
  state?: string;
  error?: string;
  profiles: BrowserSourceProfile[];
}

export interface BrowserSourceCatalog {
  browsers: BrowserSourceBrowser[];
  xiadownProfiles: BrowserSourceProfile[];
}

export interface BrowserSourceSelection {
  mode: BrowserSourceMode;
  browserId: string;
  profileId: string;
}

export interface SniffProfileRequest {
  profileId?: string;
  browser?: string;
  displayName?: string;
}

export type AppSessionBrowserScanStatus =
  | "new"
  | "replace"
  | "unchanged"
  | "unavailable";

export type {
  AppSessionBrowserImportRequest,
  AppSessionBrowserImportResult,
  AppSessionBrowserScanItem,
  AppSessionBrowserScanResult,
} from "@/shared/contracts/appSessions";
