import { Call } from "@wailsio/runtime"
import { useQuery } from "@tanstack/react-query"
import * as React from "react"

import {
  projectOperationActivitySnapshot,
  type OperationActivitySnapshot,
} from "@/shared/activity/operations"
import {
  projectSniffStatusSnapshot,
  type SniffStatusSnapshot,
} from "@/shared/activity/sniff"
import type { ResourceSniffStatusDTO } from "@/shared/contracts/activity"
import type { ListOperationsRequest } from "@/shared/contracts/library"
import { useListOperations } from "@/shared/query/library"

const LIBRARY_HANDLER_SERVICE = "xiadown/internal/presentation/wails.LibraryHandler"

export const RESOURCE_SNIFF_STATUS_QUERY_KEY = ["library", "resource-sniff", "status"] as const
export const DEFAULT_OPERATION_ACTIVITY_REQUEST: ListOperationsRequest = {
  status: ["queued", "running"],
  kinds: ["download", "transcode"],
}

export async function fetchResourceSniffStatus(): Promise<ResourceSniffStatusDTO> {
  return (await Call.ByName(
    `${LIBRARY_HANDLER_SERVICE}.GetResourceSniffStatus`,
  )) as ResourceSniffStatusDTO
}

export function useResourceSniffStatus(enabled = true) {
  return useQuery<ResourceSniffStatusDTO, Error, SniffStatusSnapshot>({
    queryKey: RESOURCE_SNIFF_STATUS_QUERY_KEY,
    queryFn: fetchResourceSniffStatus,
    select: projectSniffStatusSnapshot,
    enabled,
    refetchInterval: enabled ? 1_000 : false,
    refetchIntervalInBackground: false,
    staleTime: 500,
  })
}

/** Reuses the existing operations query/cache and only projects its data. */
export function useOperationActivityQuery(
  request: ListOperationsRequest = DEFAULT_OPERATION_ACTIVITY_REQUEST,
) {
  const query = useListOperations(request)
  const data = React.useMemo<OperationActivitySnapshot>(
    () => projectOperationActivitySnapshot(query.data),
    [query.data],
  )
  return { ...query, data, rawData: query.data }
}
