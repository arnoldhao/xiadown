import { Call, Events, System, Window } from "@wailsio/runtime";

import { getLanguage, t } from "@/shared/i18n";

const OS_NOTIFICATION_METHOD =
  "xiadown/internal/presentation/wails.OSNotificationHandler.Send";
const OS_NOTIFICATION_APP_ACTIVE_METHOD =
  "xiadown/internal/presentation/wails.OSNotificationHandler.IsAppActive";
const DEFAULT_AUTO_CLOSE_MS = 8_000;
const ACTIVE_WINDOWS_STORAGE_KEY = "xiadown:active-windows";
const ACTIVE_WINDOW_TTL_MS = 4_000;
const ACTIVE_WINDOW_HEARTBEAT_MS = 1_000;

export interface PublishOSNotificationInput {
  id?: string;
  title: string;
  subtitle?: string;
  body?: string;
  iconUrl?: string;
  imageUrl?: string;
  source?: string;
  data?: Record<string, unknown>;
  autoCloseMs?: number;
  silent?: boolean;
}

type NormalizedOSNotification = Required<
  Pick<PublishOSNotificationInput, "id" | "title">
> &
  Omit<PublishOSNotificationInput, "id" | "title">;

type ActiveWindowRecord = {
  active: boolean;
  ts: number;
};

const notificationWindowId =
  typeof crypto !== "undefined" && "randomUUID" in crypto
    ? crypto.randomUUID()
    : `window_${Date.now()}_${Math.random().toString(16).slice(2, 8)}`;

let localWindowActive = false;
let activeTrackerStarted = false;

function nextOSNotificationId() {
  return `os_${Date.now()}_${Math.random().toString(16).slice(2, 8)}`;
}

function isDocumentActive() {
  if (typeof document === "undefined") {
    return false;
  }
  return document.visibilityState !== "hidden" && document.hasFocus();
}

function readActiveWindowRecords() {
  if (typeof localStorage === "undefined") {
    return {} as Record<string, ActiveWindowRecord>;
  }
  try {
    const raw = JSON.parse(localStorage.getItem(ACTIVE_WINDOWS_STORAGE_KEY) || "{}");
    if (!raw || typeof raw !== "object") {
      return {} as Record<string, ActiveWindowRecord>;
    }
    return raw as Record<string, ActiveWindowRecord>;
  } catch {
    return {} as Record<string, ActiveWindowRecord>;
  }
}

function writeActiveWindowRecords(records: Record<string, ActiveWindowRecord>) {
  if (typeof localStorage === "undefined") {
    return;
  }
  try {
    localStorage.setItem(ACTIVE_WINDOWS_STORAGE_KEY, JSON.stringify(records));
  } catch {
    // noop
  }
}

function pruneActiveWindowRecords(records: Record<string, ActiveWindowRecord>, now: number) {
  return Object.fromEntries(
    Object.entries(records).filter(([, record]) => now - Number(record?.ts ?? 0) <= ACTIVE_WINDOW_TTL_MS)
  ) as Record<string, ActiveWindowRecord>;
}

function updateLocalWindowActive(active = isDocumentActive()) {
  localWindowActive = active;
  const now = Date.now();
  const records = pruneActiveWindowRecords(readActiveWindowRecords(), now);
  records[notificationWindowId] = { active, ts: now };
  writeActiveWindowRecords(records);
}

function clearLocalWindowActive() {
  updateLocalWindowActive(false);
}

function hasAnyActiveAppWindow() {
  if (isDocumentActive() || localWindowActive) {
    return true;
  }
  const now = Date.now();
  const records = pruneActiveWindowRecords(readActiveWindowRecords(), now);
  writeActiveWindowRecords(records);
  return Object.values(records).some((record) => record.active && now - record.ts <= ACTIVE_WINDOW_TTL_MS);
}

function startAppActiveTracker() {
  if (activeTrackerStarted || typeof window === "undefined") {
    return;
  }
  activeTrackerStarted = true;
  updateLocalWindowActive();

  window.addEventListener("focus", () => updateLocalWindowActive(true));
  window.addEventListener("blur", clearLocalWindowActive);
  window.addEventListener("pagehide", clearLocalWindowActive);
  window.addEventListener("beforeunload", clearLocalWindowActive);
  document.addEventListener("visibilitychange", () => updateLocalWindowActive());

  const markFocused = () => updateLocalWindowActive(true);
  const markInactive = () => updateLocalWindowActive(false);
  const recompute = () => updateLocalWindowActive();

  try {
    Events.On("common:WindowFocus", markFocused);
    Events.On("common:WindowLostFocus", markInactive);
    Events.On("common:WindowMinimise", markInactive);
    Events.On("common:WindowHide", markInactive);
    Events.On("common:WindowUnMinimise", recompute);
    Events.On("common:WindowShow", recompute);
    Events.On("mac:ApplicationDidBecomeActive", markFocused);
    Events.On("mac:ApplicationDidResignActive", markInactive);
    Events.On("windows:WindowActive", markFocused);
    Events.On("windows:WindowInactive", markInactive);
    Events.On("windows:WindowSetFocus", markFocused);
    Events.On("windows:WindowKillFocus", markInactive);
    Events.On("linux:WindowFocusIn", markFocused);
    Events.On("linux:WindowFocusOut", markInactive);
  } catch {
    // Browser focus/visibility events still cover the web fallback path.
  }

  window.setInterval(() => {
    if (localWindowActive || isDocumentActive()) {
      updateLocalWindowActive(true);
    }
  }, ACTIVE_WINDOW_HEARTBEAT_MS);
}

async function isAppActiveForNotifications() {
  if (hasAnyActiveAppWindow()) {
    return true;
  }
  try {
    if ((await Call.ByName(OS_NOTIFICATION_APP_ACTIVE_METHOD)) === true) {
      return true;
    }
  } catch {
    // Older/runtime-less builds fall back to the current frontend window check.
  }
  try {
    return await Window.IsFocused();
  } catch {
    return false;
  }
}

function normalizeOSNotification(input: PublishOSNotificationInput): NormalizedOSNotification | null {
  const title = input.title.trim();
  if (!title) {
    return null;
  }
  return {
    ...input,
    id: input.id?.trim() || nextOSNotificationId(),
    title,
    subtitle: input.subtitle?.trim(),
    body: input.body?.trim(),
    iconUrl: input.iconUrl?.trim(),
    imageUrl: input.imageUrl?.trim(),
    source: input.source?.trim(),
    autoCloseMs:
      input.autoCloseMs === undefined
        ? DEFAULT_AUTO_CLOSE_MS
        : Math.max(0, input.autoCloseMs),
  };
}

async function requestWebNotificationPermission() {
  if (typeof window === "undefined" || !("Notification" in window)) {
    return "unsupported";
  }
  if (Notification.permission === "default") {
    return Notification.requestPermission();
  }
  return Notification.permission;
}

async function publishWebNotification(input: NormalizedOSNotification) {
  const permission = await requestWebNotificationPermission();
  if (permission !== "granted") {
    return false;
  }

  const imageUrl = input.imageUrl || input.iconUrl || "";
  const iconUrl = input.iconUrl || imageUrl;
  const title = resolveWebNotificationTitle(input);
  const body = resolveWebNotificationBody(input, title);
  const options: NotificationOptions & { image?: string } = {
    body,
    data: input.data,
    icon: iconUrl || undefined,
    silent: input.silent,
    tag: input.id,
  };

  if (imageUrl) {
    options.image = imageUrl;
  }

  const notification = new Notification(title, options);
  if (input.autoCloseMs && input.autoCloseMs > 0) {
    window.setTimeout(() => notification.close(), input.autoCloseMs);
  }
  return true;
}

function isWindowsPlatform() {
  try {
    return System.IsWindows();
  } catch {
    const platform =
      typeof navigator === "undefined"
        ? ""
        : `${navigator.platform} ${navigator.userAgent}`.toLowerCase();
    return platform.includes("win");
  }
}

function resolveAppNotificationTitle() {
  const language = getLanguage();
  return t("xiadown.appName", language);
}

function resolveWebNotificationTitle(input: NormalizedOSNotification) {
  return isWindowsPlatform() ? resolveAppNotificationTitle() : input.title;
}

function resolveWebNotificationBody(input: NormalizedOSNotification, title: string) {
  const detail = input.body || input.subtitle || "";
  if (!isWindowsPlatform()) {
    return detail;
  }

  return [input.title === title ? "" : input.title, detail]
    .map((value) => value.trim())
    .filter(Boolean)
    .join("\n");
}

async function publishNativeNotification(input: NormalizedOSNotification) {
  await Call.ByName(OS_NOTIFICATION_METHOD, {
    id: input.id,
    title: input.title,
    subtitle: input.subtitle || "",
    body: input.body || "",
    iconUrl: input.iconUrl || "",
    imageUrl: input.imageUrl || "",
    source: input.source || "",
    data: input.data ?? {},
  });
  return true;
}

function shouldPreferNativeNotification(input: NormalizedOSNotification) {
  try {
    if (System.IsWindows()) {
      return false;
    }
    if (System.IsMac()) {
      return Boolean(input.imageUrl || input.iconUrl);
    }
  } catch {
    // Runtime platform detection can fail in browser-only builds.
  }
  return false;
}

export async function publishOSNotification(input: PublishOSNotificationInput) {
  const normalized = normalizeOSNotification(input);
  if (!normalized) {
    return false;
  }
  if (await isAppActiveForNotifications()) {
    return false;
  }

  let nativeAttempted = false;
  if (shouldPreferNativeNotification(normalized)) {
    nativeAttempted = true;
    try {
      if (await publishNativeNotification(normalized)) {
        return true;
      }
    } catch (error) {
      console.warn("[notification] native notification failed", error);
    }
  }

  try {
    if (await publishWebNotification(normalized)) {
      return true;
    }
  } catch (error) {
    console.warn("[notification] web notification failed", error);
  }

  if (nativeAttempted || isWindowsPlatform()) {
    return false;
  }

  try {
    return await publishNativeNotification(normalized);
  } catch (error) {
    console.warn("[notification] native notification failed", error);
    return false;
  }
}

startAppActiveTracker();
