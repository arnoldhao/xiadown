import { describe, expect, test } from "bun:test";

import { workspaceRequiredWidth } from "@/app/workspace";

import {
  resolveCompanionWindowPlan,
  workspaceCompanionAffectsLayout,
} from "./workspace-window-fit";

const CLOSED_WORKSPACE_WIDTH = workspaceRequiredWidth();
const DOCKED_WORKSPACE_WIDTH = workspaceRequiredWidth({
  companionOpen: true,
});

describe("workspace companion window fitting", () => {
  test("does not reserve docked companion width for a fullscreen overlay", () => {
    expect(workspaceCompanionAffectsLayout(true, false)).toBe(true);
    expect(workspaceCompanionAffectsLayout(true, true)).toBe(false);
    expect(workspaceCompanionAffectsLayout(false, true)).toBe(false);
    expect(CLOSED_WORKSPACE_WIDTH).toBe(1024);
    expect(
      workspaceRequiredWidth({
        companionOpen: workspaceCompanionAffectsLayout(true, true),
      }),
    ).toBe(1024);
  });

  test("keeps the 1414px docked workspace on common desktop work areas", () => {
    expect(DOCKED_WORKSPACE_WIDTH).toBe(1414);
    for (const workAreaWidth of [1536, 1440]) {
      expect(
        resolveCompanionWindowPlan({
          open: true,
          currentWidth: CLOSED_WORKSPACE_WIDTH,
          requiredDockedWidth: DOCKED_WORKSPACE_WIDTH,
          workAreaWidth,
          fullscreen: false,
          maximized: false,
        }),
      ).toEqual({
        presentation: "docked",
        targetWidth: DOCKED_WORKSPACE_WIDTH,
      });
    }
  });

  test("keeps constrained native windows overlaid", () => {
    expect(
      resolveCompanionWindowPlan({
        open: true,
        currentWidth: CLOSED_WORKSPACE_WIDTH,
        requiredDockedWidth: DOCKED_WORKSPACE_WIDTH,
        workAreaWidth: 1280,
        fullscreen: false,
        maximized: false,
      }).presentation,
    ).toBe("overlay");
    expect(
      resolveCompanionWindowPlan({
        open: true,
        currentWidth: 1280,
        requiredDockedWidth: DOCKED_WORKSPACE_WIDTH,
        workAreaWidth: 1280,
        fullscreen: false,
        maximized: true,
      }).presentation,
    ).toBe("overlay");
  });

  test("docks the companion beside Primary in wide native fullscreen and maximized windows", () => {
    for (const nativeState of [
      { fullscreen: true, maximized: false },
      { fullscreen: false, maximized: true },
    ]) {
      for (const currentWidth of [1440, 1536, 1920]) {
        expect(
          resolveCompanionWindowPlan({
            open: true,
            currentWidth,
            requiredDockedWidth: DOCKED_WORKSPACE_WIDTH,
            workAreaWidth: currentWidth,
            ...nativeState,
          }),
        ).toEqual({ presentation: "docked" });
      }
    }
  });

  test("honours the exact 1414px native three-pane boundary", () => {
    expect(
      resolveCompanionWindowPlan({
        open: true,
        currentWidth: DOCKED_WORKSPACE_WIDTH - 1,
        requiredDockedWidth: DOCKED_WORKSPACE_WIDTH,
        workAreaWidth: DOCKED_WORKSPACE_WIDTH,
        fullscreen: true,
        maximized: false,
      }).presentation,
    ).toBe("overlay");
    expect(
      resolveCompanionWindowPlan({
        open: true,
        currentWidth: DOCKED_WORKSPACE_WIDTH,
        requiredDockedWidth: DOCKED_WORKSPACE_WIDTH,
        workAreaWidth: DOCKED_WORKSPACE_WIDTH,
        fullscreen: true,
        maximized: false,
      }).presentation,
    ).toBe("docked");
  });

  test("re-evaluates the companion when native window presentation changes", async () => {
    const source = await Bun.file(
      new URL("./useWorkspaceWindowFit.ts", import.meta.url),
    ).text();

    for (const eventName of [
      "WindowFullscreen",
      "WindowUnFullscreen",
      "WindowMaximise",
      "WindowUnMaximise",
      "WindowRestore",
    ]) {
      expect(source).toContain(`Events.Types.Common.${eventName}`);
    }
    expect(source).toContain(
      "removeNativeListeners.forEach((removeListener) => removeListener())",
    );
  });

  test("never auto-shrinks when the companion closes", () => {
    expect(
      resolveCompanionWindowPlan({
        open: false,
        currentWidth: 1300,
        requiredDockedWidth: DOCKED_WORKSPACE_WIDTH,
        workAreaWidth: 1536,
        fullscreen: false,
        maximized: false,
      }),
    ).toEqual({ presentation: "docked" });
  });
});
