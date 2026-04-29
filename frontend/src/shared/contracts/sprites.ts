export type SpriteScope = "builtin" | "imported";
export type SpriteStatus = "ready" | "invalid";

export interface SpriteAuthor {
  id: string;
  displayName: string;
}

export interface SpriteSliceGrid {
  x: number[];
  y: number[];
}

export interface Sprite {
  id: string;
  name: string;
  description: string;
  frameCount: number;
  columns: number;
  rows: number;
  spriteFile: string;
  spritePath: string;
  sourceType?: string;
  origin?: string;
  scope: SpriteScope;
  status: SpriteStatus;
  validationMessage?: string;
  imageWidth: number;
  imageHeight: number;
  author: SpriteAuthor;
  createdAt: string;
  updatedAt: string;
  version: string;
  coverImageDataUrl?: string;
}

export interface InspectSpriteRequest {
  path: string;
}

export interface SpriteImportDraft {
  path: string;
  previewPath: string;
  sourceType: string;
  name: string;
  description: string;
  authorDisplayName: string;
  version: string;
  frameCount: number;
  columns: number;
  rows: number;
  spriteFile: string;
  status: SpriteStatus;
  validationMessage?: string;
  imageWidth: number;
  imageHeight: number;
}

export interface ImportSpriteRequest {
  path: string;
  name: string;
  description: string;
  authorDisplayName: string;
  version: string;
  origin?: string;
}

export interface InstallSpriteFromURLRequest {
  url: string;
  sha256?: string;
  size?: number;
  name: string;
  description: string;
  authorDisplayName: string;
  version: string;
}

export interface UpdateSpriteRequest {
  id: string;
  name: string;
  description: string;
  authorDisplayName: string;
  version: string;
}

export interface ExportSpriteRequest {
  id: string;
  outputPath: string;
}

export interface DeleteSpriteRequest {
  id: string;
}

export interface GetSpriteManifestRequest {
  id: string;
}

export interface SpriteManifest {
  id: string;
  name: string;
  scope: SpriteScope;
  spritePath: string;
  sourceType?: string;
  imageWidth: number;
  imageHeight: number;
  sheetWidth: number;
  sheetHeight: number;
  columns: number;
  rows: number;
  sliceGrid: SpriteSliceGrid;
  canEdit: boolean;
  updatedAt: string;
}

export interface UpdateSpriteSlicesRequest {
  id: string;
  sliceGrid: SpriteSliceGrid;
}
