import TelemetryDeck from "@telemetrydeck/sdk";
import telemetryDeckPackage from "@telemetrydeck/sdk/package.json";
import { Call, Events } from "@wailsio/runtime";

const TELEMETRY_SIGNAL_EVENT = "telemetry:signal";
const TELEMETRY_HANDLER_SERVICE = "xiadown/internal/presentation/wails.TelemetryHandler";
const TELEMETRY_TARGET = "https://nom.telemetrydeck.com/v2/";
const TELEMETRY_CLIENT_NAME = "JavaScriptSDK";
const TELEMETRY_CLIENT_VERSION =
  typeof telemetryDeckPackage.version === "string" ? telemetryDeckPackage.version.trim() : "";
const TELEMETRY_CLIENT_NAME_AND_VERSION = TELEMETRY_CLIENT_VERSION
  ? `${TELEMETRY_CLIENT_NAME} ${TELEMETRY_CLIENT_VERSION}`
  : TELEMETRY_CLIENT_NAME;
const FORBIDDEN_PAYLOAD_KEYS = new Set([
  "count",
  "type",
  "appID",
  "clientUser",
  "__time",
  "payload",
  "platform",
  "receivedAt",
]);

type TelemetryBootstrap = {
  enabled: boolean;
  appId: string;
  appVersion: string;
  installId: string;
  sessionId: string;
  testMode: boolean;
};

type TelemetrySignal = {
  type: string;
  floatValue?: number;
  payload?: Record<string, unknown>;
};

const resolveTimeZone = () => {
  if (typeof Intl === "undefined" || typeof Intl.DateTimeFormat !== "function") {
    return "";
  }
  return Intl.DateTimeFormat().resolvedOptions().timeZone?.trim() ?? "";
};

const isRecord = (value: unknown): value is Record<string, unknown> =>
  Boolean(value) && typeof value === "object" && !Array.isArray(value);

const stringOrEmpty = (value: unknown) => (typeof value === "string" ? value.trim() : "");
const finiteNumberOrUndefined = (value: unknown) =>
  typeof value === "number" && Number.isFinite(value) ? value : undefined;
const finiteNumber = (value: unknown) => (typeof value === "number" && Number.isFinite(value) ? value : null);

const normalizeLocale = (value: unknown) => stringOrEmpty(value).replace(/_/g, "-");

const primaryLanguage = (locale: string) => {
  const normalized = normalizeLocale(locale);
  return normalized.split("-")[0]?.trim() ?? "";
};

const regionFromLocale = (locale: string) => {
  const normalized = normalizeLocale(locale);
  const parts = normalized.split("-").map((part) => part.trim()).filter(Boolean);
  return parts.length > 1 ? (parts[parts.length - 1] ?? "").toUpperCase() : "";
};

const extractSystemVersion = (userAgent: string) => {
  const trimmed = userAgent.trim();
  const windows = trimmed.match(/Windows NT ([0-9.]+)/);
  if (windows?.[1]) {
    return `Windows ${windows[1]}`;
  }
  const mac = trimmed.match(/Mac OS X ([0-9_]+)/);
  if (mac?.[1]) {
    return `macOS ${mac[1].replace(/_/g, ".")}`;
  }
  const ios = trimmed.match(/(?:iPhone|iPad|iPod).* OS ([0-9_]+)/);
  if (ios?.[1]) {
    return `iOS ${ios[1].replace(/_/g, ".")}`;
  }
  const android = trimmed.match(/Android ([0-9.]+)/);
  if (android?.[1]) {
    return `Android ${android[1]}`;
  }
  if (/\bLinux\b/.test(trimmed)) {
    return "Linux";
  }
  return "";
};

const systemMajorVersion = (systemVersion: string) => {
  const version = systemVersion.match(/[0-9]+(?:\.[0-9]+)?/)?.[0] ?? "";
  return version.split(".")[0] ?? "";
};

const systemMajorMinorVersion = (systemVersion: string) => {
  const version = systemVersion.match(/[0-9]+(?:\.[0-9]+)?/)?.[0] ?? "";
  const [major, minor] = version.split(".");
  return major && minor ? `${major}.${minor}` : major ?? "";
};

const browserDefaultPayload = () => {
  const payload: Record<string, unknown> = {};
  const screen = typeof window !== "undefined" ? window.screen : undefined;
  if (screen) {
    const width = finiteNumber(screen.width);
    const height = finiteNumber(screen.height);
    if (width !== null) {
      payload["TelemetryDeck.Device.screenResolutionWidth"] = width;
    }
    if (height !== null) {
      payload["TelemetryDeck.Device.screenResolutionHeight"] = height;
    }
    if (width !== null && height !== null && width > 0 && height > 0) {
      payload["TelemetryDeck.Device.orientation"] = height >= width ? "Portrait" : "Landscape";
    }
  }
  if (typeof window !== "undefined") {
    const scale = finiteNumber(window.devicePixelRatio);
    if (scale !== null && scale > 0) {
      payload["TelemetryDeck.Device.screenScaleFactor"] = scale;
      payload["TelemetryDeck.Device.screenDensity"] = scale;
    }
    const colorScheme = window.matchMedia?.("(prefers-color-scheme: dark)")?.matches ? "Dark" : "Light";
    payload["TelemetryDeck.UserPreference.colorScheme"] = colorScheme;
  }
  const navigatorRef = typeof navigator === "undefined" ? null : navigator;
  const userLocale = normalizeLocale(navigatorRef?.language);
  if (userLocale) {
    const language = primaryLanguage(userLocale);
    payload["TelemetryDeck.RunContext.locale"] = userLocale;
    if (language) {
      payload["TelemetryDeck.RunContext.language"] = language;
      payload["TelemetryDeck.UserPreference.language"] = language;
    }
    const region = regionFromLocale(userLocale);
    if (region) {
      payload["TelemetryDeck.UserPreference.region"] = region;
    }
  }
  const systemVersion = extractSystemVersion(stringOrEmpty(navigatorRef?.userAgent));
  if (systemVersion) {
    payload["TelemetryDeck.Device.systemVersion"] = systemVersion;
    const major = systemMajorVersion(systemVersion);
    if (major) {
      payload["TelemetryDeck.Device.systemMajorVersion"] = major;
    }
    const majorMinor = systemMajorMinorVersion(systemVersion);
    if (majorMinor) {
      payload["TelemetryDeck.Device.systemMajorMinorVersion"] = majorMinor;
    }
  }
  return payload;
};

const sanitizedPayload = (payload: Record<string, unknown>) => {
  const result: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(payload)) {
    const trimmedKey = key.trim();
    if (!trimmedKey || FORBIDDEN_PAYLOAD_KEYS.has(trimmedKey)) {
      continue;
    }
    result[trimmedKey] = value;
  }
  return result;
};

const telemetryPayloadValue = (key: string, value: unknown) => {
  if (key === "floatValue") {
    return Number.parseFloat(String(value));
  }
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
    result[key] = telemetryPayloadValue(key, value);
  }
  return result;
};

const restoreTypedTelemetryDeckPayload = (
  bodyPayload: Record<string, unknown>,
  sourcePayload: Record<string, unknown>
) => {
  for (const [key, value] of Object.entries(sourcePayload)) {
    if (!key.startsWith("TelemetryDeck.")) {
      continue;
    }
    if (typeof value === "boolean") {
      bodyPayload[key] = value;
    } else if (typeof value === "number" && Number.isFinite(value)) {
      bodyPayload[key] = value;
    }
  }
};

const appendSdkPayload = (body: Record<string, unknown>, bodyPayload: Record<string, unknown>) => {
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
  const hashBuffer = await subtleCrypto.digest("SHA-256", new TextEncoder().encode(value));
  return Array.from(new Uint8Array(hashBuffer))
    .map((byte) => byte.toString(16).padStart(2, "0"))
    .join("");
};

const normalizeBootstrap = (value: unknown): TelemetryBootstrap => {
  const raw = isRecord(value) ? value : {};
  return {
    enabled: raw.enabled === true,
    appId: stringOrEmpty(raw.appId),
    appVersion: stringOrEmpty(raw.appVersion),
    installId: stringOrEmpty(raw.installId),
    sessionId: stringOrEmpty(raw.sessionId),
    testMode: raw.testMode === true,
  };
};

const normalizeSignal = (value: unknown): TelemetrySignal | null => {
  const raw = isRecord(value) ? value : {};
  const type = stringOrEmpty(raw.type);
  if (!type) {
    return null;
  }
  const payload = isRecord(raw.payload) ? raw.payload : undefined;
  const floatValue = finiteNumberOrUndefined(raw.floatValue);
  return { type, floatValue, payload };
};

export class TelemetryManager {
  private client: TelemetryDeck | null = null;
  private stopFns: Array<() => void> = [];
  private pendingSignals = new Set<Promise<unknown>>();
  private sessionSummaryRequested = false;
  private readonly timeZone = resolveTimeZone();
  private readonly defaultPayload = browserDefaultPayload();
  private unloading = false;
  private clientUserHashKey = "";
  private clientUserHash = "";

  async start() {
    if (typeof window === "undefined") {
      return;
    }

    const bootstrap = normalizeBootstrap(
      await Call.ByName("xiadown/internal/presentation/wails.TelemetryHandler.Bootstrap").catch(
        (error) => {
          console.warn("[telemetry] bootstrap failed", error);
          return null;
        }
      )
    );
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
    } catch (error) {
      console.warn("[telemetry] sdk init failed", error);
      return;
    }

    const offSignal = Events.On(TELEMETRY_SIGNAL_EVENT, (event: unknown) => {
      const signal = normalizeSignal((event as { data?: unknown } | null)?.data ?? event);
      if (signal) {
        void this.sendSignal(signal);
      }
    });
    this.stopFns.push(offSignal);

    window.addEventListener("pagehide", this.handlePageHide);
    window.addEventListener("beforeunload", this.handleBeforeUnload);

    const emittedLaunchSignals = await Call.ByName("xiadown/internal/presentation/wails.TelemetryHandler.TrackAppLaunch").catch(
      (error) => {
        console.warn("[telemetry] app launch tracking failed", error);
        return 0;
      }
    );
    void emittedLaunchSignals;
  }

  stop() {
    for (const stop of this.stopFns.splice(0)) {
      stop();
    }
    window.removeEventListener("pagehide", this.handlePageHide);
    window.removeEventListener("beforeunload", this.handleBeforeUnload);
  }

  private handlePageHide = () => {
    void this.requestSessionSummary();
  };

  private handleBeforeUnload = () => {
    void this.requestSessionSummary();
  };

  private async requestSessionSummary() {
    if (this.sessionSummaryRequested) {
      return;
    }
    this.sessionSummaryRequested = true;
    this.unloading = true;
    await Call.ByName("xiadown/internal/presentation/wails.TelemetryHandler.FlushSessionSummary").catch(
        (error) => {
          console.warn("[telemetry] session summary failed", error);
        }
    );
    await this.waitForPendingSignals(750);
  }

  private async sendSignal(signal: TelemetrySignal) {
    if (!this.client) {
      return;
    }
    let body: Record<string, unknown> | null = null;
    try {
      body = await this.buildSignalBody(signal);
    } catch (error) {
      console.warn("[telemetry] signal build failed", signal.type, error);
      return;
    }
    if (!body) {
      return;
    }
    let pending: Promise<unknown> | null = null;
    pending = this.postSignalBody(body, this.unloading)
      .then((response) => response)
      .catch((error) => {
        console.warn("[telemetry] signal failed", signal.type, error);
      })
      .finally(() => {
        if (pending) {
          this.pendingSignals.delete(pending);
        }
      });
    this.pendingSignals.add(pending);
    await pending;
  }

  private async buildSignalBody(signal: TelemetrySignal) {
    if (!this.client) {
      return null;
    }
    const payload = signal.payload ? { ...this.defaultPayload, ...signal.payload } : { ...this.defaultPayload };
    if (this.timeZone) {
      payload["XiaDown.Locale.timeZone"] = this.timeZone;
    }
    const cleanPayload = sanitizedPayload(payload);
    const bodyPayload = buildTelemetryPayload(cleanPayload);
    const body: Record<string, unknown> = {
      clientUser: await this.hashedClientUser(),
      sessionID: this.client.sessionID,
      appID: this.client.appID,
      type: signal.type,
      telemetryClientVersion: TELEMETRY_CLIENT_NAME_AND_VERSION,
    };
    if (this.client.testMode) {
      body.isTestMode = true;
    }
    body.payload = bodyPayload;
    restoreTypedTelemetryDeckPayload(bodyPayload, cleanPayload);
    appendSdkPayload(body, bodyPayload);
    if (signal.floatValue !== undefined) {
      body["floatValue"] = signal.floatValue;
      delete bodyPayload["floatValue"];
    }
    return body;
  }

  private postSignalBody(body: Record<string, unknown>, keepalive: boolean) {
    if (!this.client) {
      return Promise.resolve(undefined);
    }
    return Call.ByName(`${TELEMETRY_HANDLER_SERVICE}.PostSignal`, {
      target: stringOrEmpty(this.client.target) || TELEMETRY_TARGET,
      body: [body],
      keepalive,
    });
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

  private async waitForPendingSignals(timeoutMs: number) {
    if (this.pendingSignals.size === 0) {
      return;
    }
    await Promise.race([
      Promise.allSettled(Array.from(this.pendingSignals)),
      new Promise((resolve) => window.setTimeout(resolve, timeoutMs)),
    ]);
  }
}
