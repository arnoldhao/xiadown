export type AppearanceLabPlatform = "macos" | "windows";
export type AppearanceLabNativeVideoPreview = "off" | "youtube" | "rss";

export interface DatasetRoot {
  dataset: Record<string, string | undefined>;
}

/**
 * Applies a root dataset value as a reversible transaction. Effects can use
 * the returned cleanup without losing a host value that existed before Lab.
 */
export function applyRootDatasetValue(
  root: DatasetRoot,
  key: string,
  value: string,
) {
  const owned = Object.prototype.hasOwnProperty.call(root.dataset, key);
  const previous = root.dataset[key];
  root.dataset[key] = value;

  return () => {
    if (owned) root.dataset[key] = previous;
    else delete root.dataset[key];
  };
}

export function applyAppearanceLabPlatform(
  root: DatasetRoot,
  platform: AppearanceLabPlatform,
) {
  return applyRootDatasetValue(root, "platform", platform);
}

/**
 * Replays the production native-video root state without copying any of its
 * paint or aperture CSS into the Lab. Explicit false values isolate each QA
 * mode from stale playback flags, while cleanup restores the host verbatim.
 */
export function applyAppearanceLabNativeVideoPreview(
  root: DatasetRoot,
  preview: AppearanceLabNativeVideoPreview,
) {
  const cleanups = [
    applyRootDatasetValue(
      root,
      "listenNativeVideoUnderlay",
      preview === "off" ? "false" : "true",
    ),
    applyRootDatasetValue(
      root,
      "youtubeWorkspaceVideoActive",
      preview === "youtube" ? "true" : "false",
    ),
    applyRootDatasetValue(root, "rssBilibiliVideoActive", "false"),
    applyRootDatasetValue(
      root,
      "rssSiteVideoActive",
      preview === "rss" ? "true" : "false",
    ),
  ];

  return () => {
    for (const cleanup of cleanups.reverse()) cleanup();
  };
}
