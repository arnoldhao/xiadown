import { useQuery } from "@tanstack/react-query";

import type {
  OnlineSpriteCatalog,
  OnlineSpriteCatalogCategory,
  OnlineSpriteCatalogItem,
} from "@/shared/contracts/online-sprites";
import type { SupportedLanguage } from "@/shared/i18n";

export const ONLINE_SPRITE_CATALOG_QUERY_KEY = ["online-sprite-catalog"] as const;

const defaultCatalogURL = "https://sprites.dreamapp.cc/index.json";
const configuredCatalogURL = String(import.meta.env.VITE_XIADOWN_SPRITE_CATALOG_URL ?? defaultCatalogURL).trim() || defaultCatalogURL;

export function useOnlineSpriteCatalog(language: SupportedLanguage) {
  return useQuery({
    queryKey: [...ONLINE_SPRITE_CATALOG_QUERY_KEY, configuredCatalogURL, language],
    queryFn: async (): Promise<OnlineSpriteCatalog> => {
      const response = await fetch(configuredCatalogURL, {
        headers: { Accept: "application/json" },
        cache: "no-cache",
      });
      if (!response.ok) {
        throw new Error(`catalog ${response.status}`);
      }
      const raw = await response.json();
      return normalizeOnlineSpriteCatalog(raw);
    },
    staleTime: 5 * 60 * 1_000,
  });
}

function normalizeOnlineSpriteCatalog(raw: unknown): OnlineSpriteCatalog {
  const source = (raw ?? {}) as Partial<OnlineSpriteCatalog> & Record<string, unknown>;
  const categories = normalizeCategories(source.categories);
  const flatItems = normalizeItems(source.items ?? source.sprites);
  const categoryItems = normalizeCategoryItems(source.categories);
  const items = flatItems.length > 0 ? flatItems : categoryItems;
  return {
    schemaVersion: numberOrDefault(source.schemaVersion, 1),
    updatedAt: stringOrDefault(source.updatedAt, new Date().toISOString()),
    source: "remote",
    categories: categories.length > 0 ? categories : deriveCategories(items),
    items,
  };
}

function normalizeCategories(raw: unknown): OnlineSpriteCatalogCategory[] {
  if (!Array.isArray(raw)) {
    return [];
  }
  return raw
    .map((item) => {
      const record = (item ?? {}) as Partial<OnlineSpriteCatalogCategory> & Record<string, unknown>;
      const id = stringOrEmpty(record.id);
      return {
        id,
        label: stringOrEmpty(record.label) || id,
      };
    })
    .filter((item) => item.id && item.label);
}

function normalizeCategoryItems(raw: unknown): OnlineSpriteCatalogItem[] {
  if (!Array.isArray(raw)) {
    return [];
  }
  return raw.flatMap((category) => {
    const record = (category ?? {}) as Record<string, unknown>;
    const categoryId = stringOrEmpty(record.id) || "featured";
    return normalizeItems(record.items ?? record.sprites, categoryId);
  });
}

function normalizeItems(raw: unknown, fallbackCategory = "featured"): OnlineSpriteCatalogItem[] {
  if (!Array.isArray(raw)) {
    return [];
  }
  return raw
    .map((item) => {
      const record = (item ?? {}) as Partial<OnlineSpriteCatalogItem> & Record<string, unknown>;
      const id = stringOrEmpty(record.id);
      const name = stringOrEmpty(record.name);
      const authorDisplayName = stringOrEmpty(record.authorDisplayName) || stringOrEmpty(record.author);
      const downloadUrl = stringOrEmpty(record.downloadUrl);
      const sha256 = stringOrEmpty(record.sha256).toLowerCase();
      return {
        id,
        slug: stringOrEmpty(record.slug) || id,
        name,
        description: stringOrEmpty(record.description),
        authorDisplayName,
        version: stringOrEmpty(record.version) || "1.0.0",
        assetVersion: stringOrEmpty(record.assetVersion) || stringOrEmpty(record.packageVersion) || stringOrEmpty(record.resourceVersion) || stringOrEmpty(record.version) || "1.0.0",
        category: stringOrEmpty(record.category) || stringOrEmpty(record.categoryId) || fallbackCategory,
        tags: stringArray(record.tags),
        previewUrl: stringOrEmpty(record.previewUrl),
        downloadUrl,
        size: numberOrDefault(record.size, 0),
        sha256,
        updatedAt: stringOrDefault(record.updatedAt, ""),
        downloadCount: numberOrDefault(record.downloadCount, 0),
        featured: Boolean(record.featured),
      };
    })
    .filter((item) => item.id && item.name && item.previewUrl && item.downloadUrl && isSHA256(item.sha256));
}

function deriveCategories(items: OnlineSpriteCatalogItem[]): OnlineSpriteCatalogCategory[] {
  const ids = Array.from(new Set(items.map((item) => item.category).filter(Boolean)));
  return ids.map((id) => ({ id, label: id }));
}

function stringArray(value: unknown): string[] {
  if (!Array.isArray(value)) {
    return [];
  }
  return value.map((item) => stringOrEmpty(item)).filter(Boolean);
}

function stringOrEmpty(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}

function stringOrDefault(value: unknown, fallback: string): string {
  return stringOrEmpty(value) || fallback;
}

function numberOrDefault(value: unknown, fallback: number): number {
  return typeof value === "number" && Number.isFinite(value) ? value : fallback;
}

function isSHA256(value: string): boolean {
  return /^[a-f0-9]{64}$/.test(value);
}
