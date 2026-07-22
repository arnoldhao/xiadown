export type RSSHostVideoFullscreenOwnership =
  | "none"
  | "owned"
  | "preexisting";

export function resolveRSSHostVideoFullscreenOwnership(
  windowWasFullscreen: boolean,
): RSSHostVideoFullscreenOwnership {
  return windowWasFullscreen ? "preexisting" : "owned";
}

export function shouldRestoreRSSHostWindow(
  ownership: RSSHostVideoFullscreenOwnership,
) {
  return ownership === "owned";
}
