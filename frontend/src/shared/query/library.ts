import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import * as LibraryBindings from "../../../bindings/xiadown/internal/application/library/dto/models";
import * as LibraryHandler from "../../../bindings/xiadown/internal/presentation/wails/libraryhandler";
import type {
  CancelOperationRequest,
  CreateTranscodeJobRequest,
  CreateYTDLPJobRequest,
  DeleteFilesRequest,
  DeleteOperationRequest,
  DeleteOperationsRequest,
  LibraryDTO,
  LibraryOperationDTO,
  ListOperationsRequest,
  ListTranscodePresetsForDownloadRequest,
  OpenFileLocationRequest,
  OpenPathRequest,
  OperationListItemDTO,
  ParseYTDLPDownloadRequest,
  ParseYTDLPDownloadResponse,
  PrepareYTDLPDownloadRequest,
  PrepareYTDLPDownloadResponse,
  ResumeOperationRequest,
  TranscodePreset,
} from "@/shared/contracts/library";

export const LIBRARY_LIST_QUERY_KEY = ["library", "libraries"] as const;
export const LIBRARY_DETAIL_QUERY_KEY = ["library", "detail"] as const;
export const LIBRARY_OPERATIONS_QUERY_KEY = ["library", "operations"] as const;
export const LIBRARY_HISTORY_QUERY_KEY = ["library", "history"] as const;
export const LIBRARY_FILE_EVENTS_QUERY_KEY = ["library", "file-events"] as const;
export const LIBRARY_WORKSPACE_QUERY_KEY = ["library", "workspace"] as const;
export const LIBRARY_WORKSPACE_PROJECT_QUERY_KEY = ["library", "workspace-project"] as const;
export const LIBRARY_TRANSCODE_PRESETS_QUERY_KEY = ["library", "transcode-presets"] as const;
export const LIBRARY_TRANSCODE_PRESETS_FOR_DOWNLOAD_QUERY_KEY = ["library", "transcode-presets-download"] as const;

export function invalidateLibraryQueries(queryClient: ReturnType<typeof useQueryClient>, libraryId?: string) {
  queryClient.invalidateQueries({ queryKey: LIBRARY_LIST_QUERY_KEY });
  queryClient.invalidateQueries({ queryKey: LIBRARY_OPERATIONS_QUERY_KEY });
  queryClient.invalidateQueries({ queryKey: LIBRARY_HISTORY_QUERY_KEY });
  queryClient.invalidateQueries({ queryKey: LIBRARY_FILE_EVENTS_QUERY_KEY });
  if (libraryId) {
    queryClient.invalidateQueries({ queryKey: [...LIBRARY_DETAIL_QUERY_KEY, libraryId] });
    queryClient.invalidateQueries({ queryKey: [...LIBRARY_WORKSPACE_QUERY_KEY, libraryId] });
    queryClient.invalidateQueries({ queryKey: [...LIBRARY_WORKSPACE_PROJECT_QUERY_KEY, libraryId] });
    return;
  }
  queryClient.invalidateQueries({ queryKey: LIBRARY_DETAIL_QUERY_KEY });
  queryClient.invalidateQueries({ queryKey: LIBRARY_WORKSPACE_QUERY_KEY });
  queryClient.invalidateQueries({ queryKey: LIBRARY_WORKSPACE_PROJECT_QUERY_KEY });
}

export function useListLibraries() {
  return useQuery({
    queryKey: LIBRARY_LIST_QUERY_KEY,
    queryFn: async (): Promise<LibraryDTO[]> => {
      return ((await LibraryHandler.ListLibraries()) ?? []) as LibraryDTO[];
    },
    staleTime: 5_000,
  });
}

export function useListOperations(request: ListOperationsRequest) {
  return useQuery({
    queryKey: [...LIBRARY_OPERATIONS_QUERY_KEY, request],
    queryFn: async (): Promise<OperationListItemDTO[]> => {
      return (
        (await LibraryHandler.ListOperations(LibraryBindings.ListOperationsRequest.createFrom(request))) ?? []
      ) as OperationListItemDTO[];
    },
    staleTime: 3_000,
  });
}

export function useCancelOperation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: CancelOperationRequest): Promise<LibraryOperationDTO> => {
      return (await LibraryHandler.CancelOperation(
        LibraryBindings.CancelOperationRequest.createFrom(request),
      )) as LibraryOperationDTO;
    },
    onSuccess: (operation) => invalidateLibraryQueries(queryClient, operation.libraryId),
  });
}

export function useResumeOperation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: ResumeOperationRequest): Promise<LibraryOperationDTO> => {
      return (await LibraryHandler.ResumeOperation(
        LibraryBindings.ResumeOperationRequest.createFrom(request),
      )) as LibraryOperationDTO;
    },
    onSuccess: (operation) => invalidateLibraryQueries(queryClient, operation.libraryId),
  });
}

export function useDeleteOperation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: DeleteOperationRequest): Promise<void> => {
      await LibraryHandler.DeleteOperation(LibraryBindings.DeleteOperationRequest.createFrom(request));
    },
    onSuccess: () => {
      invalidateLibraryQueries(queryClient);
    },
  });
}

export function useDeleteOperations() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: DeleteOperationsRequest): Promise<void> => {
      await LibraryHandler.DeleteOperations(LibraryBindings.DeleteOperationsRequest.createFrom(request));
    },
    onSuccess: () => {
      invalidateLibraryQueries(queryClient);
    },
  });
}

export function useDeleteFiles() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: DeleteFilesRequest): Promise<void> => {
      await LibraryHandler.DeleteFiles(LibraryBindings.DeleteFilesRequest.createFrom(request));
    },
    onSuccess: () => {
      invalidateLibraryQueries(queryClient);
    },
  });
}

export function useOpenLibraryPath() {
  return useMutation({
    mutationFn: async (request: OpenPathRequest): Promise<void> => {
      await LibraryHandler.OpenPath(LibraryBindings.OpenPathRequest.createFrom(request));
    },
  });
}

export function useOpenLibraryFileLocation() {
  return useMutation({
    mutationFn: async (request: OpenFileLocationRequest): Promise<void> => {
      await LibraryHandler.OpenFileLocation(LibraryBindings.OpenFileLocationRequest.createFrom(request));
    },
  });
}

export function usePrepareYTDLPDownload() {
  return useMutation({
    mutationFn: async (request: PrepareYTDLPDownloadRequest): Promise<PrepareYTDLPDownloadResponse> => {
      return (await LibraryHandler.PrepareYTDLPDownload(
        LibraryBindings.PrepareYTDLPDownloadRequest.createFrom(request),
      )) as PrepareYTDLPDownloadResponse;
    },
  });
}

export function useParseYTDLPDownload() {
  return useMutation({
    mutationFn: async (request: ParseYTDLPDownloadRequest): Promise<ParseYTDLPDownloadResponse> => {
      return (await LibraryHandler.ParseYTDLPDownload(
        LibraryBindings.ParseYTDLPDownloadRequest.createFrom(request),
      )) as ParseYTDLPDownloadResponse;
    },
  });
}

export function useCreateYTDLPJob() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: CreateYTDLPJobRequest): Promise<LibraryOperationDTO> => {
      return (await LibraryHandler.CreateYTDLPJob(
        LibraryBindings.CreateYTDLPJobRequest.createFrom(request),
      )) as LibraryOperationDTO;
    },
    onSuccess: (operation) => invalidateLibraryQueries(queryClient, operation.libraryId),
  });
}

export function useCreateTranscodeJob() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: CreateTranscodeJobRequest): Promise<LibraryOperationDTO> => {
      return (await LibraryHandler.CreateTranscodeJob(
        LibraryBindings.CreateTranscodeJobRequest.createFrom(request),
      )) as LibraryOperationDTO;
    },
    onSuccess: (operation) => invalidateLibraryQueries(queryClient, operation.libraryId),
  });
}

export function useTranscodePresets() {
  return useQuery({
    queryKey: LIBRARY_TRANSCODE_PRESETS_QUERY_KEY,
    queryFn: async (): Promise<TranscodePreset[]> => {
      return ((await LibraryHandler.ListTranscodePresets()) ?? []) as TranscodePreset[];
    },
    staleTime: 30_000,
  });
}

export function useTranscodePresetsForDownload(request: ListTranscodePresetsForDownloadRequest | null) {
  return useQuery({
    queryKey: [...LIBRARY_TRANSCODE_PRESETS_FOR_DOWNLOAD_QUERY_KEY, request],
    enabled: request !== null && request.mediaType.trim().length > 0,
    queryFn: async (): Promise<TranscodePreset[]> => {
      if (!request) {
        return [];
      }
      return (
        (await LibraryHandler.ListTranscodePresetsForDownload(
          LibraryBindings.ListTranscodePresetsForDownloadRequest.createFrom(request),
        )) ?? []
      ) as TranscodePreset[];
    },
    staleTime: 30_000,
  });
}
