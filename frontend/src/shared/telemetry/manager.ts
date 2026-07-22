import TelemetryDeck from "@telemetrydeck/sdk";
import telemetryDeckPackage from "@telemetrydeck/sdk/package.json";
import { Call, Events } from "@wailsio/runtime";

const TELEMETRY_SIGNAL_EVENT = "telemetry:signal";
const TELEMETRY_HANDLER_SERVICE =
  "xiadown/internal/presentation/wails.TelemetryHandler";
const TELEMETRY_TARGET = "https://nom.telemetrydeck.com/v2/";
const TELEMETRY_CLIENT_NAME = "JavaScriptSDK";
const TELEMETRY_CLIENT_VERSION =
  typeof telemetryDeckPackage.version === "string"
    ? telemetryDeckPackage.version.trim()
    : "";
const TELEMETRY_CLIENT_NAME_AND_VERSION = TELEMETRY_CLIENT_VERSION
  ? `${TELEMETRY_CLIENT_NAME} ${TELEMETRY_CLIENT_VERSION}`
  : TELEMETRY_CLIENT_NAME;
const BRIDGE_RETRY_DELAYS = [0, 100, 250, 500, 1_000, 2_000] as const;
const ALLOWED_SIGNAL_TYPES = new Set([
  "TelemetryDeck.Session.started",
  "TelemetryDeck.Acquisition.newInstallDetected",
  "XiaDown.Station.opened",
]);
const ALLOWED_PAYLOAD_KEYS = new Set([
  "TelemetryDeck.AppInfo.version",
  "TelemetryDeck.AppInfo.buildNumber",
  "TelemetryDeck.AppInfo.versionAndBuildNumber",
  "TelemetryDeck.Device.architecture",
  "TelemetryDeck.Device.modelName",
  "TelemetryDeck.Device.operatingSystem",
  "TelemetryDeck.Device.platform",
  "TelemetryDeck.Device.timeZone",
  "TelemetryDeck.RunContext.isDebug",
  "TelemetryDeck.RunContext.targetEnvironment",
  "TelemetryDeck.RunContext.locale",
  "TelemetryDeck.RunContext.language",
  "TelemetryDeck.Acquisition.firstSessionDate",
  "TelemetryDeck.Retention.distinctDaysUsed",
  "TelemetryDeck.Retention.distinctDaysUsedLastMonth",
  "TelemetryDeck.Retention.totalSessionsCount",
  "TelemetryDeck.UserPreference.language",
  "TelemetryDeck.UserPreference.region",
  "XiaDown.Station.name",
]);

type TelemetryBootstrap = {
  enabled: boolean;
  appId: string;
  installId: string;
  sessionId: string;
  testMode: boolean;
};

type TelemetrySignal = {
  type: string;
  payload?: Record<string, unknown>;
};

type RetryResult =
  | { ok: true; value: unknown }
  | { ok: false; value?: never };

const isRecord = (value: unknown): value is Record<string, unknown> =>
  Boolean(value) && typeof value === "object" && !Array.isArray(value);

const stringOrEmpty = (value: unknown) =>
  typeof value === "string" ? value.trim() : "";

const sanitizedPayload = (payload: Record<string, unknown>) => {
  const result: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(payload)) {
    const trimmedKey = key.trim();
    if (!trimmedKey || !ALLOWED_PAYLOAD_KEYS.has(trimmedKey)) {
      continue;
    }
    result[trimmedKey] = value;
  }
  return result;
};

const telemetryPayloadValue = (value: unknown) => {
  if (value instanceof Date) {
    return value.toISOString();
  }
  if (typeof value === "string") {
    return value;
  }
  if (value !== null && typeof value === "object") {
    return JSON.stringify(value);
  }
  return `${value}`;
};

const buildTelemetryPayload = (payload: Record<string, unknown>) => {
  const result: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(payload)) {
    result[key] = telemetryPayloadValue(value);
  }
  return result;
};

const restoreTypedTelemetryDeckPayload = (
  bodyPayload: Record<string, unknown>,
  sourcePayload: Record<string, unknown>,
) => {
  for (const [key, value] of Object.entries(sourcePayload)) {
    if (!key.startsWith("TelemetryDeck.")) {
      continue;
    }
    if (
      typeof value === "boolean" ||
      (typeof value === "number" && Number.isFinite(value))
    ) {
      bodyPayload[key] = value;
    }
  }
};

const appendSdkPayload = (
  body: Record<string, unknown>,
  bodyPayload: Record<string, unknown>,
) => {
  const nameAndVersion = stringOrEmpty(body.telemetryClientVersion);
  if (!nameAndVersion) {
    return;
  }
  bodyPayload["TelemetryDeck.SDK.nameAndVersion"] = nameAndVersion;
  const [name, version] = nameAndVersion.split(/\s+/, 2);
  if (name) {
    bodyPayload["TelemetryDeck.SDK.name"] = name;
  }
  if (version) {
    bodyPayload["TelemetryDeck.SDK.version"] = version;
  }
};

const sha256Hex = async (value: string) => {
  const subtleCrypto = globalThis.crypto?.subtle;
  if (!subtleCrypto) {
    throw new Error("SubtleCrypto is unavailable");
  }
  const hashBuffer = await subtleCrypto.digest(
    "SHA-256",
    new TextEncoder().encode(value),
  );
  return Array.from(new Uint8Array(hashBuffer))
    .map((byte) => byte.toString(16).padStart(2, "0"))
    .join("");
};

const normalizeBootstrap = (value: unknown): TelemetryBootstrap => {
  const raw = isRecord(value) ? value : {};
  return {
    enabled: raw.enabled === true,
    appId: stringOrEmpty(raw.appId),
    installId: stringOrEmpty(raw.installId),
    sessionId: stringOrEmpty(raw.sessionId),
    testMode: raw.testMode === true,
  };
};

const normalizeSignal = (value: unknown): TelemetrySignal | null => {
  const raw = isRecord(value) ? value : {};
  const type = stringOrEmpty(raw.type);
  if (!ALLOWED_SIGNAL_TYPES.has(type)) {
    return null;
  }
  const payload = isRecord(raw.payload) ? raw.payload : undefined;
  return { type, payload };
};

export class TelemetryManager {
  private client: TelemetryDeck | null = null;
  private stopFns: Array<() => void> = [];
  private stopped = false;
  private started = false;
  private clientUserHashKey = "";
  private clientUserHash = "";

  async start() {
    if (typeof window === "undefined" || this.started) {
      return;
    }
    this.started = true;
    this.stopped = false;

    const bootstrapResult = await this.callBridgeWithRetry(
      `${TELEMETRY_HANDLER_SERVICE}.Bootstrap`,
    );
    if (!bootstrapResult.ok || this.stopped) {
      return;
    }
    const bootstrap = normalizeBootstrap(bootstrapResult.value);
    if (!bootstrap.enabled || !bootstrap.appId || !bootstrap.installId) {
      return;
    }

    try {
      this.client = new TelemetryDeck({
        appID: bootstrap.appId,
        clientUser: bootstrap.installId,
        sessionID: bootstrap.sessionId || undefined,
        testMode: bootstrap.testMode,
      });
    } catch {
      return;
    }
    if (this.stopped) {
      this.client = null;
      return;
    }

    this.stopFns.push(
      Events.On(TELEMETRY_SIGNAL_EVENT, (event: unknown) => {
        const signal = normalizeSignal(
          (event as { data?: unknown } | null)?.data ?? event,
        );
        if (signal) {
          void this.sendSignal(signal);
        }
      }),
    );

    await this.callBridgeWithRetry(`${TELEMETRY_HANDLER_SERVICE}.TrackAppLaunch`);
  }

  stop() {
    this.stopped = true;
    for (const stop of this.stopFns.splice(0)) {
      stop();
    }
    this.client = null;
  }

  private async callBridgeWithRetry(
    name: string,
    ...args: unknown[]
  ): Promise<RetryResult> {
    for (const delayMs of BRIDGE_RETRY_DELAYS) {
      if (delayMs > 0) {
        await new Promise((resolve) => window.setTimeout(resolve, delayMs));
      }
      if (this.stopped) {
        return { ok: false };
      }
      try {
        return { ok: true, value: await Call.ByName(name, ...args) };
      } catch {
        // The Wails bridge can become available just after the first paint.
      }
    }
    return { ok: false };
  }

  private async sendSignal(signal: TelemetrySignal) {
    if (!this.client || this.stopped) {
      return;
    }
    try {
      const body = await this.buildSignalBody(signal);
      if (body && !this.stopped) {
        await Call.ByName(`${TELEMETRY_HANDLER_SERVICE}.PostSignal`, {
          target: stringOrEmpty(this.client.target) || TELEMETRY_TARGET,
          body: [body],
        });
      }
    } catch {
      // Telemetry stays best-effort and cannot affect product behavior.
    }
  }

  private async buildSignalBody(signal: TelemetrySignal) {
    if (!this.client) {
      return null;
    }
    const cleanPayload = sanitizedPayload(signal.payload ?? {});
    const bodyPayload = buildTelemetryPayload(cleanPayload);
    const body: Record<string, unknown> = {
      clientUser: await this.hashedClientUser(),
      sessionID: this.client.sessionID,
      appID: this.client.appID,
      type: signal.type,
      telemetryClientVersion: TELEMETRY_CLIENT_NAME_AND_VERSION,
      payload: bodyPayload,
    };
    if (this.client.testMode) {
      body.isTestMode = true;
    }
    restoreTypedTelemetryDeckPayload(bodyPayload, cleanPayload);
    appendSdkPayload(body, bodyPayload);
    return body;
  }

  private async hashedClientUser() {
    if (!this.client) {
      return "";
    }
    const clientUser = stringOrEmpty(this.client.clientUser);
    if (!clientUser) {
      throw new Error("TelemetryDeck clientUser is not set");
    }
    const salt = stringOrEmpty(this.client.salt);
    const cacheKey = `${clientUser}\u0000${salt}`;
    if (this.clientUserHashKey !== cacheKey) {
      this.clientUserHash = await sha256Hex(`${clientUser}${salt}`);
      this.clientUserHashKey = cacheKey;
    }
    return this.clientUserHash;
  }
}
