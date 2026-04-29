import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Call, Events } from "@wailsio/runtime";

import type {
  DeleteSpriteRequest,
  ExportSpriteRequest,
  GetSpriteManifestRequest,
  InspectSpriteRequest,
  InstallSpriteFromURLRequest,
  ImportSpriteRequest,
  Sprite,
  SpriteImportDraft,
  SpriteManifest,
  UpdateSpriteRequest,
  UpdateSpriteSlicesRequest,
} from "@/shared/contracts/sprites";

export const SPRITES_QUERY_KEY = ["sprites"] as const;

export function useSprites() {
  return useQuery({
    queryKey: SPRITES_QUERY_KEY,
    queryFn: async (): Promise<Sprite[]> => {
      const result = await Call.ByName("xiadown/internal/presentation/wails.SpritesHandler.ListSprites");
      return normalizeSprites(result);
    },
    staleTime: 5_000,
  });
}

export function useImportSprite() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: ImportSpriteRequest): Promise<Sprite> => {
      const result = await Call.ByName("xiadown/internal/presentation/wails.SpritesHandler.ImportSprite", request);
      return normalizeSprite(result);
    },
    onSuccess: (sprite) => {
      queryClient.setQueryData(SPRITES_QUERY_KEY, (current: Sprite[] | undefined) =>
        upsertSprite(current ?? [], sprite),
      );
      void Events.Emit("sprites:updated");
    },
  });
}

export function useInstallSpriteFromURL() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: InstallSpriteFromURLRequest): Promise<Sprite> => {
      const result = await Call.ByName("xiadown/internal/presentation/wails.SpritesHandler.InstallSpriteFromURL", request);
      return normalizeSprite(result);
    },
    onSuccess: (sprite) => {
      queryClient.setQueryData(SPRITES_QUERY_KEY, (current: Sprite[] | undefined) =>
        upsertSprite(current ?? [], sprite),
      );
      void Events.Emit("sprites:updated");
    },
  });
}

export function useInspectSpriteSource() {
  return useMutation({
    mutationFn: async (request: InspectSpriteRequest): Promise<SpriteImportDraft> => {
      const result = await Call.ByName("xiadown/internal/presentation/wails.SpritesHandler.InspectSpriteSource", request);
      return normalizeSpriteImportDraft(result);
    },
  });
}

export function useSpriteManifest(spriteId: string) {
  return useQuery({
    queryKey: [...SPRITES_QUERY_KEY, "manifest", spriteId],
    queryFn: async (): Promise<SpriteManifest> => {
      const request: GetSpriteManifestRequest = { id: spriteId };
      const result = await Call.ByName("xiadown/internal/presentation/wails.SpritesHandler.GetSpriteManifest", request);
      return normalizeSpriteManifest(result);
    },
    enabled: spriteId.trim().length > 0,
  });
}

export function useUpdateSprite() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: UpdateSpriteRequest): Promise<Sprite> => {
      const result = await Call.ByName("xiadown/internal/presentation/wails.SpritesHandler.UpdateSprite", request);
      return normalizeSprite(result);
    },
    onSuccess: (sprite) => {
      queryClient.setQueryData(SPRITES_QUERY_KEY, (current: Sprite[] | undefined) =>
        upsertSprite(current ?? [], sprite),
      );
      void Events.Emit("sprites:updated");
    },
  });
}

export function useUpdateSpriteSlices() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: UpdateSpriteSlicesRequest): Promise<Sprite> => {
      const result = await Call.ByName("xiadown/internal/presentation/wails.SpritesHandler.UpdateSpriteSlices", request);
      return normalizeSprite(result);
    },
    onSuccess: (sprite) => {
      queryClient.setQueryData(SPRITES_QUERY_KEY, (current: Sprite[] | undefined) =>
        upsertSprite(current ?? [], sprite),
      );
      queryClient.invalidateQueries({ queryKey: [...SPRITES_QUERY_KEY, "manifest", sprite.id] });
      void Events.Emit("sprites:updated");
    },
  });
}

export function useExportSprite() {
  return useMutation({
    mutationFn: async (request: ExportSpriteRequest): Promise<void> => {
      await Call.ByName("xiadown/internal/presentation/wails.SpritesHandler.ExportSprite", request);
    },
  });
}

export function useDeleteSprite() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: DeleteSpriteRequest): Promise<void> => {
      await Call.ByName("xiadown/internal/presentation/wails.SpritesHandler.DeleteSprite", request);
    },
    onSuccess: (_, request) => {
      queryClient.setQueryData(SPRITES_QUERY_KEY, (current: Sprite[] | undefined) =>
        (current ?? []).filter((sprite) => sprite.id !== request.id),
      );
      void Events.Emit("sprites:updated");
    },
  });
}

function upsertSprite(current: Sprite[], next: Sprite): Sprite[] {
  const withoutCurrent = current.filter((sprite) => sprite.id !== next.id);
  return sortSprites([...withoutCurrent, next]);
}

function sortSprites(items: Sprite[]): Sprite[] {
  return [...items].sort(compareSprites);
}

function compareSprites(left: Sprite, right: Sprite): number {
  if (left.scope !== right.scope) {
    return left.scope === "builtin" ? -1 : 1;
  }
  if (left.status !== right.status) {
    return left.status === "ready" ? -1 : 1;
  }
  return left.name.localeCompare(right.name, undefined, { sensitivity: "base" });
}

function normalizeSprites(raw: unknown): Sprite[] {
  if (!Array.isArray(raw)) {
    return [];
  }
  return raw.map((item) => normalizeSprite(item));
}

function normalizeSprite(raw: unknown): Sprite {
  const item = (raw ?? {}) as Partial<Sprite> & Record<string, unknown>;
  return {
    id: stringOrEmpty(item.id) || stringOrEmpty(item.ID),
    name: stringOrEmpty(item.name) || stringOrEmpty(item.Name),
    description: stringOrEmpty(item.description) || stringOrEmpty(item.Description),
    frameCount: numberOrZero(item.frameCount ?? item.FrameCount),
    columns: numberOrZero(item.columns ?? item.Columns),
    rows: numberOrZero(item.rows ?? item.Rows),
    spriteFile: stringOrEmpty(item.spriteFile) || stringOrEmpty(item.SpriteFile),
    spritePath: stringOrEmpty(item.spritePath) || stringOrEmpty(item.SpritePath),
    sourceType: stringOrEmpty(item.sourceType) || stringOrEmpty(item.SourceType) || undefined,
    origin: stringOrEmpty(item.origin) || stringOrEmpty(item.Origin) || undefined,
    scope: normalizeScope(stringOrEmpty(item.scope) || stringOrEmpty(item.Scope)),
    status: normalizeStatus(stringOrEmpty(item.status) || stringOrEmpty(item.Status)),
    validationMessage:
      stringOrEmpty(item.validationMessage) || stringOrEmpty(item.ValidationMessage) || undefined,
    imageWidth: numberOrZero(item.imageWidth ?? item.ImageWidth),
    imageHeight: numberOrZero(item.imageHeight ?? item.ImageHeight),
    author: normalizeAuthor(item.author ?? item.Author),
    createdAt: stringOrEmpty(item.createdAt) || stringOrEmpty(item.CreatedAt),
    updatedAt: stringOrEmpty(item.updatedAt) || stringOrEmpty(item.UpdatedAt),
    version: stringOrEmpty(item.version) || stringOrEmpty(item.Version),
    coverImageDataUrl:
      stringOrEmpty(item.coverImageDataUrl) || stringOrEmpty(item.CoverImageDataUrl) || undefined,
  };
}

function normalizeSpriteImportDraft(raw: unknown): SpriteImportDraft {
  const item = (raw ?? {}) as Partial<SpriteImportDraft> & Record<string, unknown>;
  return {
    path: stringOrEmpty(item.path) || stringOrEmpty(item.Path),
    previewPath: stringOrEmpty(item.previewPath) || stringOrEmpty(item.PreviewPath),
    sourceType: stringOrEmpty(item.sourceType) || stringOrEmpty(item.SourceType),
    name: stringOrEmpty(item.name) || stringOrEmpty(item.Name),
    description: stringOrEmpty(item.description) || stringOrEmpty(item.Description),
    authorDisplayName:
      stringOrEmpty(item.authorDisplayName) || stringOrEmpty(item.AuthorDisplayName),
    version: stringOrEmpty(item.version) || stringOrEmpty(item.Version),
    frameCount: numberOrZero(item.frameCount ?? item.FrameCount),
    columns: numberOrZero(item.columns ?? item.Columns),
    rows: numberOrZero(item.rows ?? item.Rows),
    spriteFile: stringOrEmpty(item.spriteFile) || stringOrEmpty(item.SpriteFile),
    status: normalizeStatus(stringOrEmpty(item.status) || stringOrEmpty(item.Status)),
    validationMessage:
      stringOrEmpty(item.validationMessage) || stringOrEmpty(item.ValidationMessage) || undefined,
    imageWidth: numberOrZero(item.imageWidth ?? item.ImageWidth),
    imageHeight: numberOrZero(item.imageHeight ?? item.ImageHeight),
  };
}

function normalizeSpriteManifest(raw: unknown): SpriteManifest {
  const item = (raw ?? {}) as Partial<SpriteManifest> & Record<string, unknown>;
  return {
    id: stringOrEmpty(item.id) || stringOrEmpty(item.ID),
    name: stringOrEmpty(item.name) || stringOrEmpty(item.Name),
    scope: normalizeScope(stringOrEmpty(item.scope) || stringOrEmpty(item.Scope)),
    spritePath: stringOrEmpty(item.spritePath) || stringOrEmpty(item.SpritePath),
    sourceType: stringOrEmpty(item.sourceType) || stringOrEmpty(item.SourceType) || undefined,
    imageWidth: numberOrZero(item.imageWidth ?? item.ImageWidth),
    imageHeight: numberOrZero(item.imageHeight ?? item.ImageHeight),
    sheetWidth: numberOrZero(item.sheetWidth ?? item.SheetWidth),
    sheetHeight: numberOrZero(item.sheetHeight ?? item.SheetHeight),
    columns: numberOrZero(item.columns ?? item.Columns),
    rows: numberOrZero(item.rows ?? item.Rows),
    sliceGrid: normalizeSliceGrid(item.sliceGrid ?? item.SliceGrid),
    canEdit: Boolean(item.canEdit ?? item.CanEdit),
    updatedAt: stringOrEmpty(item.updatedAt) || stringOrEmpty(item.UpdatedAt),
  };
}

function normalizeSliceGrid(raw: unknown): SpriteManifest["sliceGrid"] {
  const item = (raw ?? {}) as Partial<SpriteManifest["sliceGrid"]> & Record<string, unknown>;
  return {
    x: numberArray(item.x ?? item.X),
    y: numberArray(item.y ?? item.Y),
  };
}

function normalizeAuthor(raw: unknown): Sprite["author"] {
  const item = (raw ?? {}) as Partial<Sprite["author"]> & Record<string, unknown>;
  return {
    id: stringOrEmpty(item.id) || stringOrEmpty(item.ID),
    displayName: stringOrEmpty(item.displayName) || stringOrEmpty(item.DisplayName),
  };
}

function normalizeScope(value: string): Sprite["scope"] {
  return value === "imported" ? "imported" : "builtin";
}

function normalizeStatus(value: string): Sprite["status"] {
  return value === "invalid" ? "invalid" : "ready";
}

function stringOrEmpty(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}

function numberOrZero(value: unknown): number {
  return typeof value === "number" && Number.isFinite(value) ? value : 0;
}

function numberArray(value: unknown): number[] {
  if (!Array.isArray(value)) {
    return [];
  }
  return value
    .map((item) => (typeof item === "number" && Number.isFinite(item) ? Math.round(item) : Number.NaN))
    .filter((item) => Number.isFinite(item));
}
