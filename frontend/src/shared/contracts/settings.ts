import type { Candidate as BrowserCDPCandidate } from "../../../bindings/xiadown/internal/application/browsercdp/models";

export type AppearanceMode = "light" | "dark" | "auto";
export type ThemeColor = string;
export type ColorScheme = "default" | "contrast" | "slate" | "warm";
export type ProxyMode = "none" | "system" | "manual";
export type ProxyScheme = "http" | "https" | "socks5";
export type SystemProxySource = "system" | "vpn";
export type MenuBarVisibility = "always" | "whenRunning" | "never";
export type ResourceSniffScope = "default" | "advanced" | "all";
export type PlaybackAudioQualityPreference =
  | "AUDIO_QUALITY_AUTO"
  | "AUDIO_QUALITY_LOW"
  | "AUDIO_QUALITY_MEDIUM"
  | "AUDIO_QUALITY_HIGH";

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

export type BrowserCandidate = Omit<BrowserCDPCandidate, "id"> & { id: string };

export interface SystemProxyInfo {
  address: string;
  source?: SystemProxySource;
  name?: string;
}

export interface SniffProfileInfo {
  browser: string;
  exists: boolean;
  sizeBytes: number;
  fileCount: number;
  directoryCount: number;
  truncated?: boolean;
  error?: string;
}

export interface SniffProfileRequest {
  browser?: string;
}

export interface Settings {
  appearance: AppearanceMode;
  effectiveAppearance: string;
  fontFamily: string;
  fontSize: number;
  language: string;
  sniffBrowser: string;
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
  syncedLyricsEnabled: boolean;
  romanizedLyrics: boolean;
  pinyinLyrics: boolean;
  playbackAudioQuality: PlaybackAudioQualityPreference;
  resourceSniffScope: ResourceSniffScope;
  resourceSniffMinBytes: number;
  resourceSniffRetain: number;
  ytdlpConcurrentDownloads: number;
  ytdlpConcurrentFragments: number;
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
  sniffBrowser?: string;
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
  syncedLyricsEnabled?: boolean;
  romanizedLyrics?: boolean;
  pinyinLyrics?: boolean;
  playbackAudioQuality?: PlaybackAudioQualityPreference;
  resourceSniffScope?: ResourceSniffScope;
  resourceSniffMinBytes?: number;
  resourceSniffRetain?: number;
  ytdlpConcurrentDownloads?: number;
  ytdlpConcurrentFragments?: number;
  mainBounds?: WindowBounds;
  settingsBounds?: WindowBounds;
  proxy?: Proxy;
  appearanceConfig?: Record<string, unknown>;
}
