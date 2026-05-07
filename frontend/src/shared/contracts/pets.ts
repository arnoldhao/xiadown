export type PetScope = "builtin" | "imported";
export type PetStatus = "ready" | "invalid";

export interface Pet {
  id: string;
  displayName: string;
  description: string;
  frameCount: number;
  columns: number;
  rows: number;
  cellWidth: number;
  cellHeight: number;
  spritesheetFile: string;
  spritesheetPath: string;
  origin?: string;
  scope: PetScope;
  status: PetStatus;
  validationCode?: string;
  validationMessage?: string;
  imageWidth: number;
  imageHeight: number;
  createdAt: string;
  updatedAt: string;
}

export interface InspectPetRequest {
  path: string;
}

export interface PetImportDraft {
  path: string;
  displayName: string;
  description: string;
  frameCount: number;
  columns: number;
  rows: number;
  cellWidth: number;
  cellHeight: number;
  spritesheetFile: string;
  status: PetStatus;
  validationCode?: string;
  validationMessage?: string;
  imageWidth: number;
  imageHeight: number;
}

export interface ImportPetRequest {
  path: string;
  origin?: string;
}

export interface StartOnlinePetImportRequest {
  siteId: string;
}

export interface GetOnlinePetImportSessionRequest {
  sessionId: string;
}

export interface FinishOnlinePetImportSessionRequest {
  sessionId: string;
}

export interface OnlinePetImportSession {
  sessionId: string;
  siteId: string;
  siteLabel: string;
  url: string;
  state: string;
  browserStatus: string;
  importedPets: Pet[];
  errorCode?: string;
  error?: string;
  updatedAt: string;
}

export interface ExportPetRequest {
  id: string;
  outputPath: string;
}

export interface DeletePetRequest {
  id: string;
}

export interface GetPetManifestRequest {
  id: string;
}

export interface PetManifest {
  id: string;
  displayName: string;
  description: string;
  scope: PetScope;
  spritesheetPath: string;
  imageWidth: number;
  imageHeight: number;
  sheetWidth: number;
  sheetHeight: number;
  columns: number;
  rows: number;
  cellWidth: number;
  cellHeight: number;
  canDelete: boolean;
  updatedAt: string;
}
