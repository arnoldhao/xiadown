import { create } from "zustand";

export type UpdateKind = "app" | "dependency" | "plugin";
export type UpdateStatus =
  | "idle"
  | "checking"
  | "no_update"
  | "available"
  | "downloading"
  | "installing"
  | "ready_to_restart"
  | "error";

export interface UpdateInfo {
  kind: UpdateKind;
  currentVersion: string;
  latestVersion: string;
  changelog: string;
  preparedVersion: string;
  preparedChangelog: string;
  downloadURL: string;
  checkedAt?: string;
  status: UpdateStatus;
  progress: number;
  message?: string;
}

export interface WhatsNewInfo {
  version: string;
  currentVersion: string;
  changelog: string;
}

export interface WhatsNewPreview {
  notice: WhatsNewInfo;
  targetWindow: "main" | "settings";
}

export interface UpdateStore {
  info: UpdateInfo;
  whatsNewPreview: WhatsNewPreview | null;
  setInfo: (info: UpdateInfo) => void;
  openWhatsNewPreview: (notice: WhatsNewInfo, targetWindow: WhatsNewPreview["targetWindow"]) => void;
  clearWhatsNewPreview: () => void;
}

const defaultInfo: UpdateInfo = {
  kind: "app",
  currentVersion: "",
  latestVersion: "",
  changelog: "",
  preparedVersion: "",
  preparedChangelog: "",
  downloadURL: "",
  status: "idle",
  progress: 0,
  message: "",
};

export const useUpdateStore = create<UpdateStore>((set) => ({
  info: defaultInfo,
  whatsNewPreview: null,
  setInfo: (info) => set({ info }),
  openWhatsNewPreview: (notice, targetWindow) => set({ whatsNewPreview: { notice, targetWindow } }),
  clearWhatsNewPreview: () => set({ whatsNewPreview: null }),
}));

export function normalizeUpdateInfo(raw: Partial<UpdateInfo> | null | undefined): UpdateInfo {
  if (!raw) {
    return defaultInfo;
  }
  const anyRaw = raw as any;
  return {
    kind: (raw.kind as UpdateKind) ?? (anyRaw.Kind as UpdateKind) ?? "app",
    currentVersion: raw.currentVersion ?? anyRaw.CurrentVersion ?? "",
    latestVersion: raw.latestVersion ?? anyRaw.LatestVersion ?? "",
    changelog: raw.changelog ?? anyRaw.Changelog ?? "",
    preparedVersion: raw.preparedVersion ?? anyRaw.PreparedVersion ?? "",
    preparedChangelog: raw.preparedChangelog ?? anyRaw.PreparedChangelog ?? "",
    downloadURL: raw.downloadURL ?? anyRaw.DownloadURL ?? "",
    checkedAt: raw.checkedAt ?? anyRaw.CheckedAt,
    status: (raw.status as UpdateStatus) ?? (anyRaw.Status as UpdateStatus) ?? "idle",
    progress: typeof raw.progress === "number" ? raw.progress : typeof anyRaw.Progress === "number" ? anyRaw.Progress : 0,
    message: raw.message ?? anyRaw.Message ?? "",
  };
}

export function normalizeWhatsNewInfo(
  raw: Partial<WhatsNewInfo> | null | undefined
): WhatsNewInfo | null {
  if (!raw) {
    return null;
  }
  const anyRaw = raw as any;
  const version = (raw.version ?? anyRaw.Version ?? "").trim();
  if (!version) {
    return null;
  }
  return {
    version,
    currentVersion: (raw.currentVersion ?? anyRaw.CurrentVersion ?? version).trim(),
    changelog: raw.changelog ?? anyRaw.Changelog ?? "",
  };
}

export function compareUpdateVersion(left: string, right: string): number {
  const leftParts = normalizeVersionParts(left);
  const rightParts = normalizeVersionParts(right);
  const maxLength = Math.max(leftParts.length, rightParts.length);
  for (let index = 0; index < maxLength; index += 1) {
    const leftValue = leftParts[index] ?? 0;
    const rightValue = rightParts[index] ?? 0;
    if (leftValue < rightValue) {
      return -1;
    }
    if (leftValue > rightValue) {
      return 1;
    }
  }
  return 0;
}

export function hasPreparedUpdate(info: UpdateInfo): boolean {
  const preparedVersion = info.preparedVersion.trim();
  if (!preparedVersion) {
    return false;
  }
  return compareUpdateVersion(info.currentVersion, preparedVersion) < 0;
}

export function hasRemoteUpdate(info: UpdateInfo): boolean {
  const latestVersion = info.latestVersion.trim();
  if (!latestVersion) {
    return false;
  }
  return compareUpdateVersion(info.currentVersion, latestVersion) < 0;
}

export function displayUpdateVersion(info: UpdateInfo): string {
  if (hasPreparedUpdate(info)) {
    return info.preparedVersion.trim();
  }
  return info.latestVersion.trim();
}

function normalizeVersionParts(version: string): number[] {
  return version
    .trim()
    .replace(/^v/i, "")
    .split(".")
    .map((part) => Number.parseInt(part, 10))
    .filter((part) => Number.isFinite(part));
}
