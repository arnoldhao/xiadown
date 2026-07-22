import { describe, expect, it } from "bun:test"

import { projectSniffStatusSnapshot } from "@/shared/activity/sniff"
import type { ResourceSniffStatusDTO } from "@/shared/contracts/activity"

describe("projectSniffStatusSnapshot", () => {
  it("returns a stable idle snapshot for missing input", () => {
    expect(projectSniffStatusSnapshot()).toEqual({
      runtime: "none",
      state: "idle",
      sessionId: undefined,
      runtimeId: undefined,
      title: "",
      url: "",
      favicon: "",
      resourceCount: 0,
      downloadableCount: 0,
      lastCaptureAt: undefined,
      canClear: false,
      canStop: false,
    })
  })

  it("normalizes a managed status without changing its activity meaning", () => {
    const input: ResourceSniffStatusDTO = {
      runtime: " MANAGED ",
      state: "ACTIVE",
      sessionId: " session-1 ",
      title: " Example Page ",
      url: " https://www.example.test/watch/1 ",
      favicon: " data:image/png;base64,icon ",
      resourceCount: 4.9,
      downloadableCount: 9,
      lastCaptureAt: " 2026-07-10T08:00:00Z ",
      canClear: true,
      canStop: true,
    }

    expect(projectSniffStatusSnapshot(input)).toEqual({
      runtime: "managed",
      state: "active",
      sessionId: "session-1",
      runtimeId: undefined,
      title: "Example Page",
      url: "https://www.example.test/watch/1",
      favicon: "data:image/png;base64,icon",
      resourceCount: 4,
      downloadableCount: 4,
      lastCaptureAt: "2026-07-10T08:00:00Z",
      canClear: true,
      canStop: true,
    })
  })

  it("uses the hostname as the title fallback and constrains capabilities", () => {
    const snapshot = projectSniffStatusSnapshot({
      runtime: "managed",
      state: "mystery",
      url: "https://www.example.test/path",
      resourceCount: -1,
      downloadableCount: 3,
      canClear: true,
      canStop: false,
    })

    expect(snapshot.title).toBe("example.test")
    expect(snapshot.state).toBe("error")
    expect(snapshot.resourceCount).toBe(0)
    expect(snapshot.downloadableCount).toBe(0)
    expect(snapshot.canClear).toBe(false)
  })

  it("preserves orphan stop identity without allowing resource clearing", () => {
    const snapshot = projectSniffStatusSnapshot({
      runtime: "orphan",
      state: "active",
      runtimeId: "runtime-1",
      resourceCount: 5,
      downloadableCount: 2,
      canClear: true,
      canStop: true,
    })

    expect(snapshot.runtimeId).toBe("runtime-1")
    expect(snapshot.canStop).toBe(true)
    expect(snapshot.canClear).toBe(false)
  })
})
