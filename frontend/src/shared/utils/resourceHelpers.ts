import type { YTDLPSubtitleOption } from "@/shared/contracts/library";

export function formatDomainLabel(domain?: string, url?: string) {
  const raw = (domain ?? "").trim();
  let host = raw;
  if (!host && url) {
    try {
      host = new URL(url).hostname;
    } catch {
      host = "";
    }
  }
  host = host.replace(/^www\./i, "");
  if (!host) {
    return "";
  }
  const parts = host.split(".");
  const label = parts.length > 1 ? parts.slice(0, -1).join(".") : host;
  return label.toUpperCase();
}

export function formatSubtitleLabel(subtitle: YTDLPSubtitleOption, t: (key: string) => string) {
  const name = subtitle.name?.trim() || subtitle.language?.trim() || subtitle.id;
  const parts = [name];
  if (subtitle.ext) {
    parts.push(subtitle.ext.toUpperCase());
  }
  if (subtitle.isAuto) {
    parts.push(t("library.download.subtitle.auto"));
  }
  return parts.filter(Boolean).join(" · ");
}

export function resolveDialogPath(selection: unknown) {
  if (typeof selection === "string") {
    return selection.trim();
  }
  if (Array.isArray(selection) && typeof selection[0] === "string") {
    return selection[0].trim();
  }
  return "";
}

export function getPathBaseName(path: string) {
  if (!path) {
    return "";
  }
  const normalized = path.replace(/\\/g, "/");
  return normalized.split("/").pop()?.trim() ?? "";
}

export function extractExtensionFromPath(path: string) {
  const baseName = getPathBaseName(path);
  if (!baseName) {
    return "";
  }
  const dotIndex = baseName.lastIndexOf(".");
  if (dotIndex <= 0 || dotIndex >= baseName.length - 1) {
    return "";
  }
  return baseName.slice(dotIndex + 1).trim().toLowerCase();
}

export function stripPathExtension(fileName: string) {
  if (!fileName) {
    return "";
  }
  const dotIndex = fileName.lastIndexOf(".");
  if (dotIndex <= 0) {
    return fileName;
  }
  return fileName.slice(0, dotIndex);
}

export function dedupeStrings(values: string[]) {
  const result: string[] = [];
  const seen = new Set<string>();
  values.forEach((value) => {
    const trimmed = value.trim();
    if (!trimmed || seen.has(trimmed)) {
      return;
    }
    seen.add(trimmed);
    result.push(trimmed);
  });
  return result;
}

export function toggleMultiFilterValue<T extends string>(current: T[], value: T, checked: boolean): T[] {
  if (checked) {
    if (current.includes(value)) {
      return current;
    }
    return [...current, value];
  }
  return current.filter((item) => item !== value);
}

export function buildAssetPreviewURL(baseURL: string, path: string, cacheKey?: string) {
  if (!baseURL || !path) {
    return "";
  }
  const trimmed = baseURL.replace(/\/+$/, "");
  const previewName = path.replace(/\\/g, "/").split("/").pop()?.trim() || "asset";
  const params = new URLSearchParams({ path });
  const normalizedCacheKey = (cacheKey ?? "").trim();
  if (normalizedCacheKey) {
    params.set("v", normalizedCacheKey);
  }
  return `${trimmed}/api/library/asset/${encodeURIComponent(previewName)}?${params.toString()}`;
}
