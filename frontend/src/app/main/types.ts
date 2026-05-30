import type { LibraryDTO,LibraryMediaInfoDTO,OperationListItemDTO } from "@/shared/contracts/library";

export type MainViewId =
  | "running"
  | "completed"
  | "listen"
  | "connections"
  | "petsGallery"
  | "sniffDesk";
export type CompletedViewMode = "tasks" | "files";
export type CompletedContextMenuTarget =
  | { kind: "task"; id: string; x: number; y: number }
  | { kind: "file"; id: string; x: number; y: number };
export type CompletedDeleteConfirmation =
  | { kind: "tasks"; ids: string[]; label: string; count: number }
  | { kind: "files"; ids: string[]; label: string; count: number };

export type CompletedFileEntry = {
  id: string;
  libraryId: string;
  libraryName: string;
  operationId: string;
  latestOperationId: string;
  originOperationId: string;
  rootFileId: string;
  sourceFileId: string;
  sourceFileName: string;
  name: string;
  title: string;
  author: string;
  path: string;
  kind: string;
  format: string;
  sizeBytes: number;
  updatedAt: string;
  previewURL: string;
  coverURL: string;
  canDelete: boolean;
  media: LibraryMediaInfoDTO | null;
};

export type CompletedFileType = "video" | "audio" | "subtitle" | "image" | "other";
export type CompletedPreviewGroupKind = "media" | "subtitle" | "image" | "other";

export type CompletedTaskEntry = {
  operation: OperationListItemDTO;
  library: LibraryDTO | null;
  coverURL: string;
  files: CompletedFileEntry[];
  sourceFileId: string;
  sourceFileName: string;
  counts: {
    media: number;
    subtitle: number;
    image: number;
  };
  updatedAt: string;
};

export type SetupState = {
  completed: boolean;
};

export type NewTaskDialogMode = "download" | "sniff" | "transcode";
export type NewTaskDialogTranscodeSource = {
  fileId?: string;
  inputPath: string;
  title?: string;
  author?: string;
};
export type DownloadDialogStep = "input" | "config";
export type DownloadDialogTab = "quick" | "custom" | "sniff";
export type DownloadQuality = "best" | "audio";
export type SourceMediaType = "video" | "audio";
export type SelectOption = {
  value: string;
  label: string;
};
