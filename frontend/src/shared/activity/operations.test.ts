import { describe, expect, it } from "bun:test"

import {
  normalizeOperationActivityStage,
  projectOperationActivitySnapshot,
  resolveOperationActivitySpeed,
  resolveOperationThumbnailCoverURL,
} from "@/shared/activity/operations"
import type { OperationListItemDTO } from "@/shared/contracts/library"

function operation(
  input: Partial<OperationListItemDTO> &
    Pick<OperationListItemDTO, "operationId" | "kind" | "status">,
): OperationListItemDTO {
  return {
    operationId: input.operationId,
    libraryId: input.libraryId ?? "library-1",
    name: input.name ?? input.operationId,
    kind: input.kind,
    status: input.status,
    correlation: input.correlation ?? {},
    metrics: input.metrics ?? { fileCount: 0 },
    createdAt: input.createdAt ?? "2026-07-10T08:00:00Z",
    ...input,
  }
}

describe("projectOperationActivitySnapshot", () => {
  it("returns empty, typed kind summaries when there is no activity", () => {
    const snapshot = projectOperationActivitySnapshot(undefined)

    expect(snapshot.hasActivity).toBe(false)
    expect(snapshot.activeCount).toBe(0)
    expect(snapshot.primary).toBeUndefined()
    expect(snapshot.download.kind).toBe("download")
    expect(snapshot.transcode.kind).toBe("transcode")
    expect(snapshot.download.hasIndeterminateProgress).toBe(false)
  })

  it("separates kinds, distinguishes indeterminate progress, and selects a running primary", () => {
    const input = [
      operation({
        operationId: "download-running",
        kind: "download",
        status: "running",
        progress: {
          stage: "Downloading Video",
          percent: 40,
          updatedAt: "2026-07-10T08:03:00Z",
        },
      }),
      operation({
        operationId: "transcode-queued",
        kind: "transcode",
        status: "queued",
        progress: { stage: "preparing" },
        createdAt: "2026-07-10T08:04:00Z",
      }),
      operation({
        operationId: "completed",
        kind: "download",
        status: "completed",
        progress: { percent: 100 },
      }),
      operation({
        operationId: "other-kind",
        kind: "scan",
        status: "running",
      }),
    ]

    const snapshot = projectOperationActivitySnapshot(input)

    expect(snapshot.items.map((item) => item.operationId)).toEqual([
      "download-running",
      "transcode-queued",
    ])
    expect(snapshot.primary?.operationId).toBe("download-running")
    expect(snapshot.runningCount).toBe(1)
    expect(snapshot.queuedCount).toBe(1)
    expect(snapshot.download.progressPercent).toBe(40)
    expect(snapshot.download.hasIndeterminateProgress).toBe(false)
    expect(snapshot.download.primary?.stage).toBe("downloading_video")
    expect(snapshot.transcode.progressPercent).toBeUndefined()
    expect(snapshot.transcode.hasIndeterminateProgress).toBe(true)
    expect(snapshot.transcode.indeterminateCount).toBe(1)
    expect(input[0].operationId).toBe("download-running")
  })

  it("chooses the most recently updated running operation and clamps percentages", () => {
    const snapshot = projectOperationActivitySnapshot([
      operation({
        operationId: "older",
        kind: "download",
        status: "running",
        progress: { percent: -10, updatedAt: "2026-07-10T08:01:00Z" },
      }),
      operation({
        operationId: "newer",
        kind: "download",
        status: "running",
        progress: { percent: 140, updatedAt: "2026-07-10T08:02:00Z" },
      }),
    ])

    expect(snapshot.primary?.operationId).toBe("newer")
    expect(snapshot.download.items.map((item) => item.percent)).toEqual([100, 0])
    expect(snapshot.download.progressPercent).toBe(50)
  })

  it("aggregates typed and fallback speeds per operation kind", () => {
    const snapshot = projectOperationActivitySnapshot([
      operation({
        operationId: "download-1",
        kind: "download",
        status: "running",
        progress: {
          speedMetric: {
            kind: "bytes_per_second",
            label: "2 MiB/s",
            bytesPerSecond: 2 * 1024 ** 2,
          },
        },
      }),
      operation({
        operationId: "download-2",
        kind: "download",
        status: "running",
        progress: { speed: "512 KiB/s" },
      }),
      operation({
        operationId: "transcode-fps",
        kind: "transcode",
        status: "running",
        progress: { speed: "60 FPS" },
      }),
      operation({
        operationId: "transcode-factor",
        kind: "transcode",
        status: "running",
        progress: {
          speedMetric: { kind: "factor", label: "1.5x", factor: 1.5 },
        },
      }),
    ])

    expect(snapshot.download.speed.bytesPerSecond).toBe(2.5 * 1024 ** 2)
    expect(snapshot.transcode.speed.framesPerSecond).toBe(60)
    expect(snapshot.transcode.speed.factor).toBe(1.5)
  })
})

describe("operation activity helpers", () => {
  it("uses the canonical asset preview route for operation thumbnails", () => {
    expect(
      resolveOperationThumbnailCoverURL("http://127.0.0.1:34115/", {
        thumbnailPreviewPath: " C:\\cache\\cover art.jpg ",
      }),
    ).toBe(
      "http://127.0.0.1:34115/api/library/asset/cover%20art.jpg?path=C%3A%5Ccache%5Ccover+art.jpg",
    )
    expect(
      resolveOperationThumbnailCoverURL("http://127.0.0.1:34115", {}),
    ).toBe("")
  })

  it("normalizes stage codes", () => {
    expect(normalizeOperationActivityStage(" Post-Processing ")).toBe("post_processing")
  })

  it("prefers typed speed metrics over the legacy speed string", () => {
    expect(
      resolveOperationActivitySpeed({
        kind: "download",
        progress: {
          speed: "99 MiB/s",
          speedMetric: {
            kind: "bytes_per_second",
            label: "3 MiB/s",
            bytesPerSecond: 3 * 1024 ** 2,
          },
        },
      }),
    ).toEqual({
      kind: "bytes_per_second",
      value: 3 * 1024 ** 2,
      label: "3 MiB/s",
    })
  })
})
