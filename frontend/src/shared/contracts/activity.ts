export type ResourceSniffRuntimeDTO = "none" | "managed" | "orphan" | string
export type ResourceSniffActivityStateDTO =
  | "idle"
  | "starting"
  | "active"
  | "closing"
  | "error"
  | string

/** Lightweight DTO returned by LibraryHandler.GetResourceSniffStatus. */
export interface ResourceSniffStatusDTO {
  runtime: ResourceSniffRuntimeDTO
  state: ResourceSniffActivityStateDTO
  sessionId?: string
  runtimeId?: string
  title?: string
  url?: string
  favicon?: string
  resourceCount: number
  downloadableCount: number
  lastCaptureAt?: string
  canClear: boolean
  canStop: boolean
}
