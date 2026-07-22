export type DataManagementRisk = "safe" | "caution" | "protected" | string;
export type DataManagementCategoryId = "core" | "reclaimable" | "obsolete";

export interface DataManagementItem {
  id: string;
  labelKey?: string;
  descriptionKey?: string;
  sizeBytes: number;
  itemCount: number;
  state?: string;
  risk: DataManagementRisk;
  clearable: boolean;
  selectedByDefault?: boolean;
}

export interface DataManagementCategory {
  id: DataManagementCategoryId;
  labelKey?: string;
  totalBytes: number;
  items: DataManagementItem[];
}

export interface DataManagementSnapshot {
  totalBytes: number;
  safeReclaimableBytes: number;
  scannedAt?: string;
  categories: DataManagementCategory[];
}

export interface CleanDataManagementRequest {
  resourceIds: string[];
}

export interface CleanDataManagementResult {
  resourceId: string;
  status: string;
  bytesFreed: number;
  message?: string;
}

export interface CleanDataManagementResponse {
  results: CleanDataManagementResult[];
  snapshot: DataManagementSnapshot;
}

export interface ResetApplicationResponse {
  scheduled: boolean;
}
