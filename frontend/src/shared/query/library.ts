import { Call } from "@wailsio/runtime";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import * as LibraryBindings from "../../../bindings/xiadown/internal/application/library/dto/models";
import * as LibraryHandler from "../../../bindings/xiadown/internal/presentation/wails/libraryhandler";
import type {
  CDPBrowserStatus,
  CancelOperationRequest,
  CancelResourceSniffRequest,
  ClearResourceSniffResourcesRequest,
  CurrentResourceSniffBrowserStatus,
  ApplyLibraryRelinksRequest,
  ApplyLibraryRelinksResponse,
  CreateTranscodeJobRequest,
  CreateYTDLPBatchJobsRequest,
  CreateYTDLPBatchJobsResponse,
  CreateYTDLPJobRequest,
  DeleteFilesRequest,
  DeletedLibraryItemMutationRequest,
  DeletedLibraryItemMutationResponse,
  DeleteOperationOutputRequest,
  DeleteOperationRequest,
  DeleteOperationsRequest,
  LibraryDTO,
  LibraryOperationDTO,
  ListDeletedLibraryItemsRequest,
  ListDeletedLibraryItemsResponse,
  GetResourceSniffPreviewRequest,
  CurrentResourceSniffBrowserStatusRequest,
  ListMissingLibraryFilesResponse,
  ListResourceSniffResourcesRequest,
  ListResourceSniffResourcesResponse,
  ListOperationsRequest,
  ListTranscodePresetsForDownloadRequest,
  OpenFileLocationRequest,
  OpenPathRequest,
  OperationListItemDTO,
  GetResourceSniffSessionRequest,
  ParseYTDLPDownloadRequest,
  ParseYTDLPDownloadResponse,
  PrepareResourceSniffRawDownloadRequest,
  PrepareResourceSniffRawPreviewRequest,
  PrepareResourceSniffRawPreviewResponse,
  PrepareYTDLPDownloadRequest,
  PrepareYTDLPDownloadResponse,
  ProbeTranscodeInputRequest,
  ProbeTranscodeInputResponse,
  RenameFileRequest,
  RenameOperationRequest,
  ResourceSniffPreviewResponse,
  ResourceSniffSession,
  ResumeOperationRequest,
  ScanMissingLibraryFilesRequest,
  ScanMissingLibraryFilesResponse,
  StartResourceSniffRequest,
  StartResourceSniffResult,
  StopCDPBrowserRuntimeRequest,
  TranscodePreset,
} from "@/shared/contracts/library";
import { collectCompleteOperations } from "@/shared/query/complete-operations";
import { normalizeCurrentResourceSniffBrowserStatus } from "@/shared/query/currentResourceSniffBrowserStatus";
import {
  LIBRARY_RESOURCE_SNIFF_QUERY_KEY,
  LIBRARY_RESOURCE_SNIFF_RESOURCES_QUERY_KEY,
  LIBRARY_RESOURCE_SNIFF_SESSIONS_QUERY_KEY,
  removeCanceledResourceSniffSessionQueries,
} from "@/shared/query/resourceSniffQueryCache";

export { normalizeCurrentResourceSniffBrowserStatus } from "@/shared/query/currentResourceSniffBrowserStatus";
export {
  LIBRARY_RESOURCE_SNIFF_QUERY_KEY,
  LIBRARY_RESOURCE_SNIFF_RESOURCES_QUERY_KEY,
  LIBRARY_RESOURCE_SNIFF_SESSIONS_QUERY_KEY,
  removeCanceledResourceSniffSessionQueries,
} from "@/shared/query/resourceSniffQueryCache";

export const LIBRARY_LIST_QUERY_KEY = ["library", "libraries"] as const;
export const LIBRARY_DETAIL_QUERY_KEY = ["library", "detail"] as const;
export const LIBRARY_OPERATIONS_QUERY_KEY = ["library", "operations"] as const;
export const LIBRARY_ENDED_OPERATIONS_QUERY_KEY = ["library", "ended-operations"] as const;
export const LIBRARY_HISTORY_QUERY_KEY = ["library", "history"] as const;
export const LIBRARY_FILE_EVENTS_QUERY_KEY = ["library", "file-events"] as const;
export const LIBRARY_DELETED_ITEMS_QUERY_KEY = ["library", "deleted-items"] as const;
export const LIBRARY_WORKSPACE_QUERY_KEY = ["library", "workspace"] as const;
export const LIBRARY_WORKSPACE_PROJECT_QUERY_KEY = ["library", "workspace-project"] as const;
export const LIBRARY_TRANSCODE_PRESETS_QUERY_KEY = ["library", "transcode-presets"] as const;
export const LIBRARY_TRANSCODE_PRESETS_FOR_DOWNLOAD_QUERY_KEY = ["library", "transcode-presets-download"] as const;
export const LIBRARY_TRANSCODE_PROBE_QUERY_KEY = ["library", "transcode-probe"] as const;
export const LIBRARY_CDP_BROWSER_STATUS_QUERY_KEY = ["library", "cdp-browser-status"] as const;
export const LIBRARY_CURRENT_RESOURCE_SNIFF_BROWSER_STATUS_QUERY_KEY = [
  "library",
  "resource-sniff",
  "current-browser-status",
] as const;
const LIBRARY_HANDLER_SERVICE = "xiadown/internal/presentation/wails.LibraryHandler";

export function invalidateLibraryQueries(queryClient: ReturnType<typeof useQueryClient>, libraryId?: string) {
  // File lifecycle and task cascades are reflected into the Catalog service by
  // the backend. Invalidate both projections together so Library cards do not
  // retain a stale primary asset after a successful mutation.
  queryClient.invalidateQueries({ queryKey: ["catalog"] });
  queryClient.invalidateQueries({ queryKey: LIBRARY_LIST_QUERY_KEY });
  queryClient.invalidateQueries({ queryKey: LIBRARY_OPERATIONS_QUERY_KEY });
  queryClient.invalidateQueries({ queryKey: LIBRARY_HISTORY_QUERY_KEY });
  queryClient.invalidateQueries({ queryKey: LIBRARY_FILE_EVENTS_QUERY_KEY });
  queryClient.invalidateQueries({ queryKey: LIBRARY_DELETED_ITEMS_QUERY_KEY });
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

export function invalidateOperationQueries(
  queryClient: ReturnType<typeof useQueryClient>,
  libraryId?: string,
) {
  invalidateLibraryQueries(queryClient, libraryId);
  queryClient.invalidateQueries({ queryKey: LIBRARY_ENDED_OPERATIONS_QUERY_KEY });
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

export function useEndedOperations(options: { enabled?: boolean } = {}) {
  return useQuery({
    queryKey: LIBRARY_ENDED_OPERATIONS_QUERY_KEY,
    queryFn: () => collectCompleteOperations({
      status: ["succeeded", "failed", "canceled"],
    }, async (pageRequest) => (
      (await LibraryHandler.ListOperations(
        LibraryBindings.ListOperationsRequest.createFrom(pageRequest),
      )) ?? []
    ) as OperationListItemDTO[]),
    enabled: options.enabled === true,
    staleTime: 5 * 60_000,
    refetchOnWindowFocus: false,
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
    onSuccess: (operation) => invalidateOperationQueries(queryClient, operation.libraryId),
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
    onSuccess: (operation) => invalidateOperationQueries(queryClient, operation.libraryId),
  });
}

export function useRenameOperation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: RenameOperationRequest): Promise<LibraryOperationDTO> => {
      return (await LibraryHandler.RenameOperation(
        LibraryBindings.RenameOperationRequest.createFrom(request),
      )) as LibraryOperationDTO;
    },
    onSuccess: (operation) => invalidateOperationQueries(queryClient, operation.libraryId),
  });
}

export function useRenameFile() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: RenameFileRequest): Promise<LibraryDTO["files"][number]> => {
      return (await LibraryHandler.RenameFile(
        LibraryBindings.RenameFileRequest.createFrom(request),
      )) as LibraryDTO["files"][number];
    },
    onSuccess: (file) => invalidateLibraryQueries(queryClient, file.libraryId),
  });
}

export function useDeleteOperation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: DeleteOperationRequest): Promise<void> => {
      await LibraryHandler.DeleteOperation(LibraryBindings.DeleteOperationRequest.createFrom(request));
    },
    onSuccess: () => {
      invalidateOperationQueries(queryClient);
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
      invalidateOperationQueries(queryClient);
    },
  });
}

export function useDeleteOperationOutput() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: DeleteOperationOutputRequest): Promise<LibraryOperationDTO> => {
      return (await LibraryHandler.DeleteOperationOutput(
        LibraryBindings.DeleteOperationOutputRequest.createFrom(request),
      )) as LibraryOperationDTO;
    },
    // Reconcile every projection after both success and failure. In particular,
    // an OS/file-store failure remains visible in the confirmation dialog while
    // a refetch prevents partially completed lower-level work from going stale.
    onSettled: (operation) => {
      invalidateOperationQueries(queryClient, operation?.libraryId);
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

export async function listDeletedLibraryItems(
  request: ListDeletedLibraryItemsRequest = {},
): Promise<ListDeletedLibraryItemsResponse> {
  const response = await Call.ByName(
    `${LIBRARY_HANDLER_SERVICE}.ListDeletedLibraryItems`,
    request,
  ) as Partial<ListDeletedLibraryItemsResponse> | undefined;
  const items = Array.isArray(response?.items) ? response.items : [];
  return {
    items,
    total: Number.isFinite(response?.total) ? Number(response?.total) : items.length,
    limit: Number.isFinite(response?.limit)
      ? Number(response?.limit)
      : request.limit ?? items.length,
    offset: Number.isFinite(response?.offset)
      ? Number(response?.offset)
      : request.offset ?? 0,
  };
}

export function useDeletedLibraryItems(
  request: ListDeletedLibraryItemsRequest = {},
  enabled = true,
) {
  return useQuery({
    queryKey: [...LIBRARY_DELETED_ITEMS_QUERY_KEY, request],
    queryFn: () => listDeletedLibraryItems(request),
    enabled,
    staleTime: 3_000,
  });
}

export async function listCompleteDeletedLibraryItems(
  request: Omit<ListDeletedLibraryItemsRequest, "limit" | "offset"> = {},
): Promise<ListDeletedLibraryItemsResponse> {
  const pageSize = 500;
  const items: ListDeletedLibraryItemsResponse["items"] = [];
  let offset = 0;
  let total = 0;
  for (;;) {
    const page = await listDeletedLibraryItems({ ...request, limit: pageSize, offset });
    total = page.total;
    items.push(...page.items);
    if (page.items.length === 0 || items.length >= total) {
      return { items, total, limit: items.length, offset: 0 };
    }
    offset += page.items.length;
  }
}

export function useCompleteDeletedLibraryItems(
  request: Omit<ListDeletedLibraryItemsRequest, "limit" | "offset"> = {},
  enabled = true,
) {
  return useQuery({
    queryKey: [...LIBRARY_DELETED_ITEMS_QUERY_KEY, "complete", request],
    queryFn: () => listCompleteDeletedLibraryItems(request),
    enabled,
    staleTime: 3_000,
  });
}

export function useRestoreDeletedLibraryItem() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (
      request: DeletedLibraryItemMutationRequest,
    ): Promise<DeletedLibraryItemMutationResponse> => (
      await Call.ByName(
        `${LIBRARY_HANDLER_SERVICE}.RestoreDeletedLibraryItem`,
        request,
      )
    ) as DeletedLibraryItemMutationResponse,
    // A lower layer may commit the restore and still surface a warning
    // while completing follow-up work. Reconcile every projection after either
    // outcome so Deleted and the normal Library/Catalog views cannot diverge.
    onSettled: () => invalidateOperationQueries(queryClient),
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

export function useListMissingLibraryFiles() {
  return useMutation({
    mutationFn: async (): Promise<ListMissingLibraryFilesResponse> => {
      return (await Call.ByName(`${LIBRARY_HANDLER_SERVICE}.ListMissingLibraryFiles`)) as ListMissingLibraryFilesResponse;
    },
  });
}

export function useScanMissingLibraryFiles() {
  return useMutation({
    mutationFn: async (request: ScanMissingLibraryFilesRequest): Promise<ScanMissingLibraryFilesResponse> => {
      return (await Call.ByName(`${LIBRARY_HANDLER_SERVICE}.ScanMissingLibraryFiles`, request)) as ScanMissingLibraryFilesResponse;
    },
  });
}

export function useApplyLibraryRelinks() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: ApplyLibraryRelinksRequest): Promise<ApplyLibraryRelinksResponse> => {
      return (await Call.ByName(`${LIBRARY_HANDLER_SERVICE}.ApplyLibraryRelinks`, request)) as ApplyLibraryRelinksResponse;
    },
    onSuccess: () => {
      invalidateLibraryQueries(queryClient);
    },
  });
}

export function useListMissingListenLocalFiles() {
  return useMutation({
    mutationFn: async (): Promise<ListMissingLibraryFilesResponse> => {
      return (await Call.ByName(`${LIBRARY_HANDLER_SERVICE}.ListMissingListenLocalFiles`)) as ListMissingLibraryFilesResponse;
    },
  });
}

export function useScanMissingListenLocalFiles() {
  return useMutation({
    mutationFn: async (request: ScanMissingLibraryFilesRequest): Promise<ScanMissingLibraryFilesResponse> => {
      return (await Call.ByName(`${LIBRARY_HANDLER_SERVICE}.ScanMissingListenLocalFiles`, request)) as ScanMissingLibraryFilesResponse;
    },
  });
}

export function useApplyListenLocalRelinks() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: ApplyLibraryRelinksRequest): Promise<ApplyLibraryRelinksResponse> => {
      return (await Call.ByName(`${LIBRARY_HANDLER_SERVICE}.ApplyListenLocalRelinks`, request)) as ApplyLibraryRelinksResponse;
    },
    onSuccess: () => {
      invalidateLibraryQueries(queryClient);
    },
  });
}

export async function selectLibraryDirectory(title: string, initialPath: string): Promise<string> {
  return String((await LibraryHandler.SelectLibraryDirectory(title, initialPath)) ?? "").trim();
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

export function useStartResourceSniff() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: StartResourceSniffRequest): Promise<StartResourceSniffResult> => {
      return (await Call.ByName(
        `${LIBRARY_HANDLER_SERVICE}.StartResourceSniff`,
        request,
      )) as StartResourceSniffResult;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: LIBRARY_RESOURCE_SNIFF_SESSIONS_QUERY_KEY });
      queryClient.invalidateQueries({ queryKey: LIBRARY_CDP_BROWSER_STATUS_QUERY_KEY });
      queryClient.invalidateQueries({
        queryKey: LIBRARY_CURRENT_RESOURCE_SNIFF_BROWSER_STATUS_QUERY_KEY,
      });
    },
  });
}

export function useCurrentResourceSniffBrowserStatus(
  request: CurrentResourceSniffBrowserStatusRequest,
  enabled: boolean,
) {
  const browserId = request.browserId.trim().toLowerCase();
  return useQuery({
    queryKey: [
      ...LIBRARY_CURRENT_RESOURCE_SNIFF_BROWSER_STATUS_QUERY_KEY,
      browserId,
    ],
    enabled: enabled && browserId === "chrome",
    queryFn: async (): Promise<CurrentResourceSniffBrowserStatus> => {
      const result = await Call.ByName(
        `${LIBRARY_HANDLER_SERVICE}.GetCurrentResourceSniffBrowserStatus`,
        { browserId },
      );
      return normalizeCurrentResourceSniffBrowserStatus(result, browserId);
    },
    refetchInterval: enabled ? 1_500 : false,
    refetchIntervalInBackground: false,
    refetchOnWindowFocus: "always",
    staleTime: 0,
  });
}

export function useCancelResourceSniff() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: CancelResourceSniffRequest): Promise<void> => {
      await LibraryHandler.CancelResourceSniff(
        LibraryBindings.CancelResourceSniffRequest.createFrom(request),
      );
    },
    onSuccess: async (_result, request) => {
      await removeCanceledResourceSniffSessionQueries(
        queryClient,
        request.sessionId,
      );
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: LIBRARY_RESOURCE_SNIFF_SESSIONS_QUERY_KEY,
        }),
        queryClient.invalidateQueries({
          queryKey: LIBRARY_CDP_BROWSER_STATUS_QUERY_KEY,
        }),
        queryClient.invalidateQueries({
          queryKey: LIBRARY_CURRENT_RESOURCE_SNIFF_BROWSER_STATUS_QUERY_KEY,
        }),
      ]);
    },
  });
}

export function useResourceSniffSession(request: GetResourceSniffSessionRequest | null, enabled: boolean) {
  const sessionId = request?.sessionId.trim() ?? "";
  return useQuery({
    queryKey: [...LIBRARY_RESOURCE_SNIFF_QUERY_KEY, sessionId],
    enabled: enabled && sessionId.length > 0,
    queryFn: async (): Promise<ResourceSniffSession> => {
      if (!sessionId) {
        throw new Error("resource sniff session request is required");
      }
      return (await LibraryHandler.GetResourceSniffSession(
        LibraryBindings.GetResourceSniffSessionRequest.createFrom({ sessionId }),
      )) as ResourceSniffSession;
    },
    refetchInterval: 250,
    refetchIntervalInBackground: true,
    refetchOnWindowFocus: "always",
    staleTime: 0,
  });
}

export function useResourceSniffSessions(enabled = true) {
  return useQuery({
    queryKey: LIBRARY_RESOURCE_SNIFF_SESSIONS_QUERY_KEY,
    enabled,
    queryFn: async (): Promise<ResourceSniffSession[]> => {
      return ((await LibraryHandler.ListResourceSniffSessions()) ?? []) as ResourceSniffSession[];
    },
    refetchInterval: 750,
    refetchIntervalInBackground: true,
    refetchOnWindowFocus: "always",
    staleTime: 0,
  });
}

export function useResourceSniffResources(
  request: ListResourceSniffResourcesRequest | null,
  enabled = true,
) {
  const sessionId = request?.sessionId.trim() ?? "";
  return useQuery({
    queryKey: [...LIBRARY_RESOURCE_SNIFF_RESOURCES_QUERY_KEY, sessionId],
    enabled: enabled && sessionId.length > 0,
    queryFn: async (): Promise<ListResourceSniffResourcesResponse> => {
      if (!sessionId) {
        throw new Error("resource sniff session request is required");
      }
      return (await LibraryHandler.ListResourceSniffResources(
        LibraryBindings.ListResourceSniffResourcesRequest.createFrom({ sessionId }),
      )) as ListResourceSniffResourcesResponse;
    },
    refetchInterval: 750,
    refetchIntervalInBackground: true,
    refetchOnWindowFocus: "always",
    staleTime: 0,
  });
}

export function useClearResourceSniffResources() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: ClearResourceSniffResourcesRequest): Promise<void> => {
      await Call.ByName(`${LIBRARY_HANDLER_SERVICE}.ClearResourceSniffResources`, request);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: LIBRARY_RESOURCE_SNIFF_RESOURCES_QUERY_KEY });
    },
  });
}

export function useResourceSniffPreview() {
  return useMutation({
    mutationFn: async (
      request: GetResourceSniffPreviewRequest,
    ): Promise<ResourceSniffPreviewResponse> => {
      return (await Call.ByName(
        `${LIBRARY_HANDLER_SERVICE}.GetResourceSniffPreview`,
        request,
      )) as ResourceSniffPreviewResponse;
    },
  });
}

export function usePrepareResourceSniffRawPreview() {
  return useMutation({
    mutationFn: async (
      request: PrepareResourceSniffRawPreviewRequest,
    ): Promise<PrepareResourceSniffRawPreviewResponse> => {
      return (await Call.ByName(
        `${LIBRARY_HANDLER_SERVICE}.PrepareResourceSniffRawPreview`,
        request,
      )) as PrepareResourceSniffRawPreviewResponse;
    },
  });
}

export function usePrepareResourceSniffRawDownload() {
  return useMutation({
    mutationFn: async (
      request: PrepareResourceSniffRawDownloadRequest,
    ): Promise<ParseYTDLPDownloadResponse> => {
      return (await LibraryHandler.PrepareResourceSniffRawDownload(
        LibraryBindings.PrepareResourceSniffRawDownloadRequest.createFrom(request),
      )) as ParseYTDLPDownloadResponse;
    },
  });
}

export function useCDPBrowserStatus(enabled = true) {
  return useQuery({
    queryKey: LIBRARY_CDP_BROWSER_STATUS_QUERY_KEY,
    enabled,
    queryFn: async (): Promise<CDPBrowserStatus> => {
      return (await LibraryHandler.GetCDPBrowserStatus()) as CDPBrowserStatus;
    },
    refetchInterval: 1_000,
    refetchIntervalInBackground: true,
    refetchOnWindowFocus: "always",
    staleTime: 0,
  });
}

export function useStopCDPBrowserRuntime() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: StopCDPBrowserRuntimeRequest): Promise<void> => {
      await LibraryHandler.StopCDPBrowserRuntime(
        LibraryBindings.StopCDPBrowserRuntimeRequest.createFrom(request),
      );
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: LIBRARY_CDP_BROWSER_STATUS_QUERY_KEY });
      queryClient.invalidateQueries({ queryKey: LIBRARY_RESOURCE_SNIFF_SESSIONS_QUERY_KEY });
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
    onSuccess: (operation) => invalidateOperationQueries(queryClient, operation.libraryId),
  });
}

export function useCreateYTDLPBatchJobs() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (
      request: CreateYTDLPBatchJobsRequest,
    ): Promise<CreateYTDLPBatchJobsResponse> => {
      return (await Call.ByName(
        `${LIBRARY_HANDLER_SERVICE}.CreateYTDLPBatchJobs`,
        request,
      )) as CreateYTDLPBatchJobsResponse;
    },
    onSuccess: (response) => {
      const libraryIds = new Set(
        response.operations
          .map((operation) => operation.libraryId)
          .filter(Boolean),
      );
      if (libraryIds.size === 0) {
        invalidateOperationQueries(queryClient);
        return;
      }
      for (const libraryId of libraryIds) {
        invalidateOperationQueries(queryClient, libraryId);
      }
    },
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
    onSuccess: (operation) => invalidateOperationQueries(queryClient, operation.libraryId),
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

export function useProbeTranscodeInput(request: ProbeTranscodeInputRequest | null) {
  return useQuery({
    queryKey: [...LIBRARY_TRANSCODE_PROBE_QUERY_KEY, request],
    enabled:
      request !== null &&
      (Boolean(request.fileId?.trim()) || Boolean(request.inputPath?.trim())),
    queryFn: async (): Promise<ProbeTranscodeInputResponse> => {
      if (!request) {
        throw new Error("probe request is required");
      }
      return (await LibraryHandler.ProbeTranscodeInput(
        LibraryBindings.ProbeTranscodeInputRequest.createFrom(request),
      )) as ProbeTranscodeInputResponse;
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
