import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Call, Events } from "@wailsio/runtime";

import type {
  DeletePetRequest,
  ExportPetRequest,
  FinishOnlinePetImportSessionRequest,
  GetOnlinePetImportSessionRequest,
  GetPetManifestRequest,
  ImportPetRequest,
  InspectPetRequest,
  OnlinePetImportSession,
  Pet,
  PetImportDraft,
  PetManifest,
  StartOnlinePetImportRequest,
} from "@/shared/contracts/pets";

export const PETS_QUERY_KEY = ["pets"] as const;

export function usePets() {
  return useQuery({
    queryKey: PETS_QUERY_KEY,
    queryFn: async (): Promise<Pet[]> => {
      const result = await Call.ByName("xiadown/internal/presentation/wails.PetsHandler.ListPets");
      return normalizePets(result);
    },
    staleTime: 5_000,
  });
}

export function useImportPet() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: ImportPetRequest): Promise<Pet> => {
      const result = await Call.ByName("xiadown/internal/presentation/wails.PetsHandler.ImportPet", request);
      return normalizePet(result);
    },
    onSuccess: (pet) => {
      queryClient.setQueryData(PETS_QUERY_KEY, (current: Pet[] | undefined) =>
        upsertPet(current ?? [], pet),
      );
      void Events.Emit("pets:updated");
    },
  });
}

export function useStartOnlinePetImport() {
  return useMutation({
    mutationFn: async (request: StartOnlinePetImportRequest): Promise<OnlinePetImportSession> => {
      const result = await Call.ByName("xiadown/internal/presentation/wails.PetsHandler.StartOnlinePetImport", request);
      return normalizeOnlinePetImportSession(result);
    },
  });
}

export function useOnlinePetImportSession(request: GetOnlinePetImportSessionRequest, enabled: boolean) {
  return useQuery({
    queryKey: [...PETS_QUERY_KEY, "online-import", request.sessionId],
    enabled: enabled && request.sessionId.trim().length > 0,
    queryFn: async (): Promise<OnlinePetImportSession> => {
      const result = await Call.ByName("xiadown/internal/presentation/wails.PetsHandler.GetOnlinePetImportSession", request);
      return normalizeOnlinePetImportSession(result);
    },
    refetchInterval: 1000,
    staleTime: 0,
  });
}

export function useFinishOnlinePetImportSession() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: FinishOnlinePetImportSessionRequest): Promise<OnlinePetImportSession> => {
      const result = await Call.ByName("xiadown/internal/presentation/wails.PetsHandler.FinishOnlinePetImportSession", request);
      return normalizeOnlinePetImportSession(result);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: PETS_QUERY_KEY });
      void Events.Emit("pets:updated");
    },
  });
}

export function useInspectPetSource() {
  return useMutation({
    mutationFn: async (request: InspectPetRequest): Promise<PetImportDraft> => {
      const result = await Call.ByName("xiadown/internal/presentation/wails.PetsHandler.InspectPetSource", request);
      return normalizePetImportDraft(result);
    },
  });
}

export function usePetManifest(petId: string) {
  return useQuery({
    queryKey: [...PETS_QUERY_KEY, "manifest", petId],
    queryFn: async (): Promise<PetManifest> => {
      const request: GetPetManifestRequest = { id: petId };
      const result = await Call.ByName("xiadown/internal/presentation/wails.PetsHandler.GetPetManifest", request);
      return normalizePetManifest(result);
    },
    enabled: petId.trim().length > 0,
  });
}

export function useExportPet() {
  return useMutation({
    mutationFn: async (request: ExportPetRequest): Promise<void> => {
      await Call.ByName("xiadown/internal/presentation/wails.PetsHandler.ExportPet", request);
    },
  });
}

export function useDeletePet() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: DeletePetRequest): Promise<void> => {
      await Call.ByName("xiadown/internal/presentation/wails.PetsHandler.DeletePet", request);
    },
    onSuccess: (_, request) => {
      queryClient.setQueryData(PETS_QUERY_KEY, (current: Pet[] | undefined) =>
        (current ?? []).filter((pet) => pet.id !== request.id),
      );
      void Events.Emit("pets:updated");
    },
  });
}

function upsertPet(current: Pet[], next: Pet): Pet[] {
  const withoutCurrent = current.filter((pet) => pet.id !== next.id);
  return sortPets([...withoutCurrent, next]);
}

function sortPets(items: Pet[]): Pet[] {
  return [...items].sort(comparePets);
}

function comparePets(left: Pet, right: Pet): number {
  if (left.scope !== right.scope) {
    return left.scope === "builtin" ? -1 : 1;
  }
  if (left.status !== right.status) {
    return left.status === "ready" ? -1 : 1;
  }
  return left.displayName.localeCompare(right.displayName, undefined, { sensitivity: "base" });
}

function normalizePets(raw: unknown): Pet[] {
  if (!Array.isArray(raw)) {
    return [];
  }
  return raw.map((item) => normalizePet(item));
}

function normalizePet(raw: unknown): Pet {
  const item = (raw ?? {}) as Partial<Pet> & Record<string, unknown>;
  return {
    id: stringOrEmpty(item.id) || stringOrEmpty(item.ID),
    displayName: stringOrEmpty(item.displayName) || stringOrEmpty(item.DisplayName),
    description: stringOrEmpty(item.description) || stringOrEmpty(item.Description),
    frameCount: numberOrZero(item.frameCount ?? item.FrameCount),
    columns: numberOrZero(item.columns ?? item.Columns),
    rows: numberOrZero(item.rows ?? item.Rows),
    cellWidth: numberOrZero(item.cellWidth ?? item.CellWidth),
    cellHeight: numberOrZero(item.cellHeight ?? item.CellHeight),
    spritesheetFile: stringOrEmpty(item.spritesheetFile) || stringOrEmpty(item.SpritesheetFile),
    spritesheetPath: stringOrEmpty(item.spritesheetPath) || stringOrEmpty(item.SpritesheetPath),
    origin: stringOrEmpty(item.origin) || stringOrEmpty(item.Origin) || undefined,
    scope: normalizeScope(stringOrEmpty(item.scope) || stringOrEmpty(item.Scope)),
    status: normalizeStatus(stringOrEmpty(item.status) || stringOrEmpty(item.Status)),
    validationCode: stringOrEmpty(item.validationCode) || stringOrEmpty(item.ValidationCode) || undefined,
    validationMessage:
      stringOrEmpty(item.validationMessage) || stringOrEmpty(item.ValidationMessage) || undefined,
    imageWidth: numberOrZero(item.imageWidth ?? item.ImageWidth),
    imageHeight: numberOrZero(item.imageHeight ?? item.ImageHeight),
    createdAt: stringOrEmpty(item.createdAt) || stringOrEmpty(item.CreatedAt),
    updatedAt: stringOrEmpty(item.updatedAt) || stringOrEmpty(item.UpdatedAt),
  };
}

function normalizePetImportDraft(raw: unknown): PetImportDraft {
  const item = (raw ?? {}) as Partial<PetImportDraft> & Record<string, unknown>;
  return {
    path: stringOrEmpty(item.path) || stringOrEmpty(item.Path),
    displayName: stringOrEmpty(item.displayName) || stringOrEmpty(item.DisplayName),
    description: stringOrEmpty(item.description) || stringOrEmpty(item.Description),
    frameCount: numberOrZero(item.frameCount ?? item.FrameCount),
    columns: numberOrZero(item.columns ?? item.Columns),
    rows: numberOrZero(item.rows ?? item.Rows),
    cellWidth: numberOrZero(item.cellWidth ?? item.CellWidth),
    cellHeight: numberOrZero(item.cellHeight ?? item.CellHeight),
    spritesheetFile: stringOrEmpty(item.spritesheetFile) || stringOrEmpty(item.SpritesheetFile),
    status: normalizeStatus(stringOrEmpty(item.status) || stringOrEmpty(item.Status)),
    validationCode: stringOrEmpty(item.validationCode) || stringOrEmpty(item.ValidationCode) || undefined,
    validationMessage:
      stringOrEmpty(item.validationMessage) || stringOrEmpty(item.ValidationMessage) || undefined,
    imageWidth: numberOrZero(item.imageWidth ?? item.ImageWidth),
    imageHeight: numberOrZero(item.imageHeight ?? item.ImageHeight),
  };
}

function normalizePetManifest(raw: unknown): PetManifest {
  const item = (raw ?? {}) as Partial<PetManifest> & Record<string, unknown>;
  return {
    id: stringOrEmpty(item.id) || stringOrEmpty(item.ID),
    displayName: stringOrEmpty(item.displayName) || stringOrEmpty(item.DisplayName),
    description: stringOrEmpty(item.description) || stringOrEmpty(item.Description),
    scope: normalizeScope(stringOrEmpty(item.scope) || stringOrEmpty(item.Scope)),
    spritesheetPath: stringOrEmpty(item.spritesheetPath) || stringOrEmpty(item.SpritesheetPath),
    imageWidth: numberOrZero(item.imageWidth ?? item.ImageWidth),
    imageHeight: numberOrZero(item.imageHeight ?? item.ImageHeight),
    sheetWidth: numberOrZero(item.sheetWidth ?? item.SheetWidth),
    sheetHeight: numberOrZero(item.sheetHeight ?? item.SheetHeight),
    columns: numberOrZero(item.columns ?? item.Columns),
    rows: numberOrZero(item.rows ?? item.Rows),
    cellWidth: numberOrZero(item.cellWidth ?? item.CellWidth),
    cellHeight: numberOrZero(item.cellHeight ?? item.CellHeight),
    canDelete: Boolean(item.canDelete ?? item.CanDelete),
    updatedAt: stringOrEmpty(item.updatedAt) || stringOrEmpty(item.UpdatedAt),
  };
}

function normalizeOnlinePetImportSession(raw: unknown): OnlinePetImportSession {
  const item = (raw ?? {}) as Partial<OnlinePetImportSession> & Record<string, unknown>;
  const importedRaw = item.importedPets ?? item.ImportedPets;
  return {
    sessionId: stringOrEmpty(item.sessionId) || stringOrEmpty(item.SessionID),
    siteId: stringOrEmpty(item.siteId) || stringOrEmpty(item.SiteID),
    siteLabel: stringOrEmpty(item.siteLabel) || stringOrEmpty(item.SiteLabel),
    url: stringOrEmpty(item.url) || stringOrEmpty(item.URL),
    state: stringOrEmpty(item.state) || stringOrEmpty(item.State),
    browserStatus: stringOrEmpty(item.browserStatus) || stringOrEmpty(item.BrowserStatus),
    importedPets: Array.isArray(importedRaw) ? importedRaw.map((pet) => normalizePet(pet)) : [],
    errorCode: stringOrEmpty(item.errorCode) || stringOrEmpty(item.ErrorCode) || undefined,
    error: stringOrEmpty(item.error) || stringOrEmpty(item.Error) || undefined,
    updatedAt: stringOrEmpty(item.updatedAt) || stringOrEmpty(item.UpdatedAt),
  };
}

function normalizeScope(value: string): Pet["scope"] {
  return value === "imported" ? "imported" : "builtin";
}

function normalizeStatus(value: string): Pet["status"] {
  return value === "invalid" ? "invalid" : "ready";
}

function stringOrEmpty(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}

function numberOrZero(value: unknown): number {
  return typeof value === "number" && Number.isFinite(value) ? value : 0;
}
