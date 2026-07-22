import type { CompanionPresentation } from "@/app/workspace";

export const WORKSPACE_WINDOW_EDGE_GAP = 16;

export type CompanionWindowPlan = {
  presentation: CompanionPresentation;
  targetWidth?: number;
};

/**
 * A fullscreen player reuses the CompanionPanel host as a fixed overlay, but
 * it must not reserve the width of a docked companion underneath that overlay.
 */
export function workspaceCompanionAffectsLayout(
  companionOpen: boolean,
  playerFullscreen: boolean,
) {
  return companionOpen && !playerFullscreen;
}

export function resolveCompanionWindowPlan(input: {
  open: boolean;
  currentWidth: number;
  requiredDockedWidth: number;
  workAreaWidth: number;
  fullscreen: boolean;
  maximized: boolean;
}): CompanionWindowPlan {
  if (!input.open) {
    return { presentation: "docked" };
  }
  if (input.fullscreen || input.maximized) {
    // A native fullscreen/maximised window already owns all available width.
    // Keep Companion in the structural Sidebar / Primary / Companion row when
    // that row fits; attempting to resize the native window here is invalid.
    // Only retain the overlay fallback for displays that cannot physically fit
    // the 224 + 800 + 390 workspace contract.
    return {
      presentation:
        input.currentWidth >= input.requiredDockedWidth
          ? "docked"
          : "overlay",
    };
  }
  const maximumWidth =
    Math.max(0, Math.floor(input.workAreaWidth)) - WORKSPACE_WINDOW_EDGE_GAP;
  if (maximumWidth > 0 && input.requiredDockedWidth > maximumWidth) {
    return { presentation: "overlay" };
  }
  if (input.currentWidth + 1 < input.requiredDockedWidth) {
    return {
      presentation: "docked",
      targetWidth: Math.ceil(input.requiredDockedWidth),
    };
  }
  return { presentation: "docked" };
}
