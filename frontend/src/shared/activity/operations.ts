import type {
  OperationListItemDTO,
  OperationSpeedMetricDTO,
} from "@/shared/contracts/library"
import { buildAssetPreviewURL } from "@/shared/utils/resourceHelpers"

export type OperationActivityKind = "download" | "transcode"
export type OperationActivityStatus = "queued" | "running"
export type OperationActivityProgressMode = "determinate" | "indeterminate"

export type OperationActivitySpeed =
  | { kind: "bytes_per_second"; value: number; label: string }
  | { kind: "frames_per_second"; value: number; label: string }
  | { kind: "factor"; value: number; label: string }
  | { kind: "other"; label: string }

export interface OperationActivitySpeedAggregate {
  bytesPerSecond?: number
  framesPerSecond?: number
  factor?: number
  labels: string[]
}

export interface OperationActivityItem {
  operation: OperationListItemDTO
  operationId: string
  kind: OperationActivityKind
  status: OperationActivityStatus
  stage: string
  progressMode: OperationActivityProgressMode
  percent?: number
  speed?: OperationActivitySpeed
  updatedAt: string
}

export interface OperationKindActivitySnapshot {
  kind: OperationActivityKind
  items: OperationActivityItem[]
  primary?: OperationActivityItem
  activeCount: number
  runningCount: number
  queuedCount: number
  determinateCount: number
  indeterminateCount: number
  hasIndeterminateProgress: boolean
  progressPercent?: number
  speed: OperationActivitySpeedAggregate
}

export interface OperationActivitySnapshot {
  items: OperationActivityItem[]
  primary?: OperationActivityItem
  download: OperationKindActivitySnapshot
  transcode: OperationKindActivitySnapshot
  activeCount: number
  runningCount: number
  queuedCount: number
  hasActivity: boolean
}

export function resolveOperationThumbnailCoverURL(
  baseURL: string,
  operation: Pick<OperationListItemDTO, "thumbnailPreviewPath">,
) {
  const thumbnailPreviewPath = operation.thumbnailPreviewPath?.trim() ?? ""
  if (!thumbnailPreviewPath) {
    return ""
  }
  return buildAssetPreviewURL(baseURL, thumbnailPreviewPath)
}

const ACTIVE_OPERATION_STATUSES = new Set<OperationActivityStatus>(["queued", "running"])

const SPEED_UNIT_MULTIPLIERS: Record<string, number> = {
  b: 1,
  kb: 1024,
  kib: 1024,
  mb: 1024 ** 2,
  mib: 1024 ** 2,
  gb: 1024 ** 3,
  gib: 1024 ** 3,
  tb: 1024 ** 4,
  tib: 1024 ** 4,
}

export function projectOperationActivitySnapshot(
  operations: readonly OperationListItemDTO[] | null | undefined,
): OperationActivitySnapshot {
  const items = (operations ?? [])
    .map(projectOperationActivityItem)
    .filter((item): item is OperationActivityItem => item !== null)
    .sort(compareOperationActivityItems)
  const download = projectOperationKindActivity("download", items)
  const transcode = projectOperationKindActivity("transcode", items)

  return {
    items,
    primary: items[0],
    download,
    transcode,
    activeCount: items.length,
    runningCount: items.filter((item) => item.status === "running").length,
    queuedCount: items.filter((item) => item.status === "queued").length,
    hasActivity: items.length > 0,
  }
}

export function normalizeOperationActivityStage(value?: string) {
  return (value ?? "")
    .trim()
    .toLowerCase()
    .replace(/[\s-]+/g, "_")
}

export function resolveOperationActivitySpeed(
  operation: Pick<OperationListItemDTO, "kind" | "progress">,
): OperationActivitySpeed | undefined {
  return (
    speedFromMetric(operation.progress?.speedMetric) ??
    speedFromText(operation.kind, operation.progress?.speed)
  )
}

function projectOperationActivityItem(
  operation: OperationListItemDTO,
): OperationActivityItem | null {
  const kind = normalizeActivityKind(operation.kind)
  const status = normalizeActivityStatus(operation.status)
  if (!kind || !status) {
    return null
  }

  const rawPercent = operation.progress?.percent
  const percent =
    typeof rawPercent === "number" && Number.isFinite(rawPercent)
      ? Math.min(100, Math.max(0, rawPercent))
      : undefined
  const updatedAt = firstTrimmed(
    operation.progress?.updatedAt,
    operation.startedAt,
    operation.createdAt,
  )

  return {
    operation,
    operationId: operation.operationId.trim(),
    kind,
    status,
    stage: normalizeOperationActivityStage(operation.progress?.stage || status),
    progressMode: percent === undefined ? "indeterminate" : "determinate",
    percent,
    speed: resolveOperationActivitySpeed(operation),
    updatedAt,
  }
}

function projectOperationKindActivity(
  kind: OperationActivityKind,
  allItems: readonly OperationActivityItem[],
): OperationKindActivitySnapshot {
  const items = allItems.filter((item) => item.kind === kind)
  const determinateItems = items.filter((item) => item.percent !== undefined)
  const indeterminateCount = items.length - determinateItems.length
  const progressPercent =
    determinateItems.length > 0
      ? determinateItems.reduce((total, item) => total + (item.percent ?? 0), 0) /
        determinateItems.length
      : undefined

  return {
    kind,
    items,
    primary: items[0],
    activeCount: items.length,
    runningCount: items.filter((item) => item.status === "running").length,
    queuedCount: items.filter((item) => item.status === "queued").length,
    determinateCount: determinateItems.length,
    indeterminateCount,
    hasIndeterminateProgress: indeterminateCount > 0,
    progressPercent,
    speed: aggregateOperationActivitySpeed(items),
  }
}

function aggregateOperationActivitySpeed(
  items: readonly OperationActivityItem[],
): OperationActivitySpeedAggregate {
  let bytesPerSecond = 0
  let framesPerSecond = 0
  let factor = 0
  const labels = new Set<string>()

  items.forEach((item) => {
    if (!item.speed) {
      return
    }
    switch (item.speed.kind) {
      case "bytes_per_second":
        bytesPerSecond += item.speed.value
        break
      case "frames_per_second":
        framesPerSecond += item.speed.value
        break
      case "factor":
        factor += item.speed.value
        break
      case "other":
        labels.add(item.speed.label)
        break
    }
  })

  return {
    bytesPerSecond: bytesPerSecond > 0 ? bytesPerSecond : undefined,
    framesPerSecond: framesPerSecond > 0 ? framesPerSecond : undefined,
    factor: factor > 0 ? factor : undefined,
    labels: [...labels],
  }
}

function speedFromMetric(metric?: OperationSpeedMetricDTO): OperationActivitySpeed | undefined {
  if (!metric) {
    return undefined
  }
  const label = metric.label?.trim() ?? ""
  if (isPositiveFinite(metric.bytesPerSecond)) {
    return { kind: "bytes_per_second", value: metric.bytesPerSecond, label }
  }
  if (isPositiveFinite(metric.framesPerSecond)) {
    return { kind: "frames_per_second", value: metric.framesPerSecond, label }
  }
  if (isPositiveFinite(metric.factor)) {
    return { kind: "factor", value: metric.factor, label }
  }
  return label ? { kind: "other", label } : undefined
}

function speedFromText(kind: string, raw?: string): OperationActivitySpeed | undefined {
  const value = raw?.trim() ?? ""
  if (!value) {
    return undefined
  }

  const bytesMatch = value.match(/([\d.]+)\s*([kmgt]?i?b)\s*\/\s*s/i)
  if (bytesMatch) {
    const amount = Number.parseFloat(bytesMatch[1])
    const multiplier = SPEED_UNIT_MULTIPLIERS[bytesMatch[2].toLowerCase()]
    if (Number.isFinite(amount) && amount > 0 && multiplier) {
      return { kind: "bytes_per_second", value: amount * multiplier, label: value }
    }
  }

  const framesMatch = value.match(/([\d.]+)\s*fps\b/i)
  if (framesMatch) {
    const amount = Number.parseFloat(framesMatch[1])
    if (Number.isFinite(amount) && amount > 0) {
      return { kind: "frames_per_second", value: amount, label: value }
    }
  }

  const factorMatch = value.match(/([\d.]+)\s*x\b/i)
  if (normalizeActivityKind(kind) === "transcode" && factorMatch) {
    const amount = Number.parseFloat(factorMatch[1])
    if (Number.isFinite(amount) && amount > 0) {
      return { kind: "factor", value: amount, label: value }
    }
  }

  return { kind: "other", label: value }
}

function normalizeActivityKind(value?: string): OperationActivityKind | null {
  switch (value?.trim().toLowerCase()) {
    case "download":
      return "download"
    case "transcode":
      return "transcode"
    default:
      return null
  }
}

function normalizeActivityStatus(value?: string): OperationActivityStatus | null {
  const normalized = value?.trim().toLowerCase() as OperationActivityStatus | undefined
  return normalized && ACTIVE_OPERATION_STATUSES.has(normalized) ? normalized : null
}

function compareOperationActivityItems(
  left: OperationActivityItem,
  right: OperationActivityItem,
) {
  const statusDifference = statusPriority(right.status) - statusPriority(left.status)
  if (statusDifference !== 0) {
    return statusDifference
  }
  const timeDifference = parseTimestamp(right.updatedAt) - parseTimestamp(left.updatedAt)
  if (timeDifference !== 0) {
    return timeDifference
  }
  return left.operationId.localeCompare(right.operationId)
}

function statusPriority(status: OperationActivityStatus) {
  return status === "running" ? 2 : 1
}

function parseTimestamp(value: string) {
  const parsed = Date.parse(value)
  return Number.isFinite(parsed) ? parsed : 0
}

function isPositiveFinite(value: number | undefined): value is number {
  return typeof value === "number" && Number.isFinite(value) && value > 0
}

function firstTrimmed(...values: Array<string | undefined>) {
  for (const value of values) {
    const trimmed = value?.trim()
    if (trimmed) {
      return trimmed
    }
  }
  return ""
}
