import type { ResourceSniffStatusDTO } from "@/shared/contracts/activity"

export type SniffStatusRuntime = "none" | "managed" | "orphan"
export type SniffStatusState = "idle" | "starting" | "active" | "closing" | "error"

export interface SniffStatusSnapshot {
  runtime: SniffStatusRuntime
  state: SniffStatusState
  sessionId?: string
  runtimeId?: string
  title: string
  url: string
  favicon: string
  resourceCount: number
  downloadableCount: number
  lastCaptureAt?: string
  canClear: boolean
  canStop: boolean
}

export function projectSniffStatusSnapshot(
  input?: ResourceSniffStatusDTO | null,
): SniffStatusSnapshot {
  const runtime = normalizeSniffRuntime(input?.runtime)
  const resourceCount = normalizeCount(input?.resourceCount)
  const downloadableCount = Math.min(
    resourceCount,
    normalizeCount(input?.downloadableCount),
  )
  const url = input?.url?.trim() ?? ""
  const title = input?.title?.trim() || hostnameFromURL(url)
  const lastCaptureAt = input?.lastCaptureAt?.trim() || undefined

  return {
    runtime,
    state: normalizeSniffState(input?.state, runtime),
    sessionId: input?.sessionId?.trim() || undefined,
    runtimeId: input?.runtimeId?.trim() || undefined,
    title,
    url,
    favicon: input?.favicon?.trim() ?? "",
    resourceCount,
    downloadableCount,
    lastCaptureAt,
    canClear: Boolean(input?.canClear && runtime === "managed" && resourceCount > 0),
    canStop: Boolean(input?.canStop && runtime !== "none"),
  }
}

function normalizeSniffRuntime(value?: string): SniffStatusRuntime {
  switch (value?.trim().toLowerCase()) {
    case "managed":
      return "managed"
    case "orphan":
      return "orphan"
    default:
      return "none"
  }
}

function normalizeSniffState(value: string | undefined, runtime: SniffStatusRuntime): SniffStatusState {
  if (runtime === "none") {
    return "idle"
  }
  switch (value?.trim().toLowerCase()) {
    case "idle":
    case "starting":
    case "active":
    case "closing":
    case "error":
      return value.trim().toLowerCase() as SniffStatusState
    default:
      return "error"
  }
}

function normalizeCount(value?: number) {
  if (typeof value !== "number" || !Number.isFinite(value) || value <= 0) {
    return 0
  }
  return Math.floor(value)
}

function hostnameFromURL(value: string) {
  if (!value) {
    return ""
  }
  try {
    return new URL(value).hostname.replace(/^www\./i, "")
  } catch {
    return ""
  }
}
