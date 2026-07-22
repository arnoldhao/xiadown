// 14rem leaves a stable 200px content lane after the sidebar's 12px insets.
export const WORKSPACE_SIDEBAR_WIDTH = 224;
export const COMPANION_PANEL_WIDTH = 390;
export const PRIMARY_PANE_DEFAULT_MIN_WIDTH = 800;

export type CompanionPresentation = "docked" | "overlay";

export interface WorkspaceWidthOptions {
  primaryMinWidth?: number;
  companionOpen?: boolean;
  companionPresentation?: CompanionPresentation;
}

export function workspaceStageRequiredWidth({
  primaryMinWidth = PRIMARY_PANE_DEFAULT_MIN_WIDTH,
  companionOpen = false,
  companionPresentation = "docked",
}: WorkspaceWidthOptions = {}) {
  return normalizeMinimumWidth(primaryMinWidth) +
    (companionOpen && companionPresentation === "docked"
      ? COMPANION_PANEL_WIDTH
      : 0);
}

export function workspaceRequiredWidth({
  primaryMinWidth = PRIMARY_PANE_DEFAULT_MIN_WIDTH,
  companionOpen = false,
  companionPresentation = "docked",
}: WorkspaceWidthOptions = {}) {
  return (
    WORKSPACE_SIDEBAR_WIDTH +
    workspaceStageRequiredWidth({
      primaryMinWidth,
      companionOpen,
      companionPresentation,
    })
  );
}

export function normalizeMinimumWidth(width: number) {
  return Number.isFinite(width) && width > 0
    ? Math.round(width)
    : PRIMARY_PANE_DEFAULT_MIN_WIDTH;
}

export function toPixelValue(width: number) {
  return `${Math.round(width)}px`;
}
