export interface LibraryAccessConfig {
  remoteEnabled: boolean;
  lanEnabled: boolean;
  lanPort: number;
  tailscaleEnabled: boolean;
  tailscaleHTTPSPort: number;
  tailscalePath: string;
  deviceName: string;
}

export interface LibraryAccessTransportStatus {
  desiredEnabled: boolean;
  state: string;
  port?: number;
  lastError?: string;
}

export interface LibraryAccessStatus {
  desiredEnabled: boolean;
  lan: LibraryAccessTransportStatus;
  tailscale: LibraryAccessTransportStatus & {
    installed: boolean;
    version?: string;
    tailnet?: string;
    device?: string;
    serveURL?: string;
  };
}

export interface UpdateLibraryAccessConfig {
  remoteEnabled?: boolean;
  lanEnabled?: boolean;
  lanPort?: number;
  tailscaleEnabled?: boolean;
  tailscaleHTTPSPort?: number;
  tailscalePath?: string;
  deviceName?: string;
}

export interface LibraryAccessUpdate {
  config: LibraryAccessConfig;
  status: LibraryAccessStatus;
}

export interface LibraryPairingSession {
  pairingVersion: number;
  pairingLink: string;
  nonce: string;
  code: string;
  expiresAt: string;
  tlsFingerprint: string;
  lanAddress?: string;
  /** Directory transport bases. Resolve relative API hrefs below these URLs. */
  lanEndpoints?: string[];
  /** Directory transport base. Its trailing slash preserves the Tailscale Serve path. */
  tailscaleURL?: string;
}

export type LibraryDeviceScope =
  | "library.read"
  | "music.read"
  | "music.state"
  | "music.manage"
  | "rss.read"
  | "rss.state"
  | "rss.manage"
  | "rss.fetch"
  | "tasks.read"
  | "tasks.create"
  | "tasks.control";

export interface PairedLibraryDevice {
  grantId: string;
  deviceId: string;
  deviceName: string;
  scopes: LibraryDeviceScope[];
  status: "active" | "revoked" | string;
  expiresAt?: string;
  lastSeenAt?: string;
  revokedAt?: string;
  revision: number;
  createdAt: string;
  updatedAt: string;
}

export interface UpdatePairedLibraryDeviceScopes {
  grantId: string;
  expectedRevision: number;
  scopes: LibraryDeviceScope[];
}

export interface RevokePairedLibraryDevice {
  grantId: string;
  expectedRevision: number;
}
