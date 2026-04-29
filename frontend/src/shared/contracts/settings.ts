export type AppearanceMode = "light" | "dark" | "auto";
export type ThemeColor = string;
export type ColorScheme = "default" | "contrast" | "slate" | "warm";
export type ProxyMode = "none" | "system" | "manual";
export type ProxyScheme = "http" | "https" | "socks5";
export type SystemProxySource = "system" | "vpn";
export type MenuBarVisibility = "always" | "whenRunning" | "never";

export interface WindowBounds {
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface Proxy {
  mode: ProxyMode;
  scheme: ProxyScheme;
  host: string;
  port: number;
  username: string;
  password: string;
  noProxy: string[];
  timeoutSeconds: number;
  testedAt: string;
  testSuccess: boolean;
  testMessage: string;
}

export type ProxySettings = Proxy;

export interface SystemProxyInfo {
  address: string;
  source?: SystemProxySource;
  name?: string;
}

export interface Settings {
  appearance: AppearanceMode;
  effectiveAppearance: string;
  fontFamily: string;
  fontSize: number;
  language: string;
  themeColor: ThemeColor;
  colorScheme: ColorScheme;
  systemThemeColor?: string;
  logLevel: string;
  logMaxSizeMB: number;
  logMaxBackups: number;
  logMaxAgeDays: number;
  logCompress: boolean;
  downloadDirectory: string;
  menuBarVisibility: MenuBarVisibility;
  autoStart: boolean;
  minimizeToTrayOnStart: boolean;
  mainBounds: WindowBounds;
  settingsBounds: WindowBounds;
  proxy: Proxy;
  version: number;
  appearanceConfig?: Record<string, unknown>;
}

export interface UpdateSettingsRequest {
  appearance?: AppearanceMode;
  fontFamily?: string;
  fontSize?: number;
  language?: string;
  themeColor?: ThemeColor;
  colorScheme?: ColorScheme;
  logLevel?: string;
  logMaxSizeMB?: number;
  logMaxBackups?: number;
  logMaxAgeDays?: number;
  logCompress?: boolean;
  downloadDirectory?: string;
  menuBarVisibility?: MenuBarVisibility;
  autoStart?: boolean;
  minimizeToTrayOnStart?: boolean;
  mainBounds?: WindowBounds;
  settingsBounds?: WindowBounds;
  proxy?: Proxy;
  appearanceConfig?: Record<string, unknown>;
}
