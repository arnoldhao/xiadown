/** Shared stamp-edge geometry for Task folders and semantic file placeholders. */
export const LIBRARY_PAPER_GEOMETRY = {
  x: -56,
  y: 0,
  width: 56,
  height: 84.8,
  radius: 1.8,
  inset: 1.4,
} as const;

export const LIBRARY_PAPER_NOTCH_POSITIONS = [
  6, 12, 18, 24, 30, 36, 42, 48, 54, 60, 66, 72, 78,
] as const;

/** Horizontal offsets used only by full placeholder stamps. */
export const PLACEHOLDER_PAPER_HORIZONTAL_NOTCH_OFFSETS = [
  4, 10, 16, 22, 28, 34, 40, 46, 52,
] as const;

export const TASK_FOLDER_PAPER_PLACEMENT = {
  translateX: 99.2,
  translateY: 27,
  rotation: 14,
} as const;

export const TASK_FOLDER_PAPER_TRANSFORM =
  `translate(${TASK_FOLDER_PAPER_PLACEMENT.translateX} ${TASK_FOLDER_PAPER_PLACEMENT.translateY}) rotate(${TASK_FOLDER_PAPER_PLACEMENT.rotation})`;

export const PLACEHOLDER_PAPER_PLACEMENT = {
  translateX: 74.64,
  translateY: 22.688,
  scale: 0.88,
} as const;

export const PLACEHOLDER_PAPER_TRANSFORM =
  `translate(${PLACEHOLDER_PAPER_PLACEMENT.translateX} ${PLACEHOLDER_PAPER_PLACEMENT.translateY}) scale(${PLACEHOLDER_PAPER_PLACEMENT.scale})`;
