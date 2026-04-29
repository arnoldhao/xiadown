export interface OnlineSpriteCatalogCategory {
  id: string;
  label: string;
}

export interface OnlineSpriteCatalogItem {
  id: string;
  slug: string;
  name: string;
  description: string;
  authorDisplayName: string;
  version: string;
  assetVersion: string;
  category: string;
  tags: string[];
  previewUrl: string;
  downloadUrl: string;
  size: number;
  sha256: string;
  updatedAt: string;
  downloadCount: number;
  featured?: boolean;
}

export interface OnlineSpriteCatalog {
  schemaVersion: number;
  updatedAt: string;
  source: "remote";
  categories: OnlineSpriteCatalogCategory[];
  items: OnlineSpriteCatalogItem[];
}
