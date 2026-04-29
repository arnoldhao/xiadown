import type { ColorScheme } from "@/shared/contracts/settings";

export const DEFAULT_COLOR_SCHEME: ColorScheme = "default";

export interface ColorSchemePreview {
  shell: string;
  sidebar: string;
  panel: string;
  accent: string;
}

export interface ColorSchemeDefinition {
  id: ColorScheme;
  preview: ColorSchemePreview;
}

export const COLOR_SCHEME_OPTIONS: ColorSchemeDefinition[] = [
  {
    id: "default",
    preview: {
      shell: "#f7fafc",
      sidebar: "#fafafa",
      panel: "#ffffff",
      accent: "#4f46e5",
    },
  },
  {
    id: "contrast",
    preview: {
      shell: "#f4f7fb",
      sidebar: "#e8edf5",
      panel: "#ffffff",
      accent: "#2563eb",
    },
  },
  {
    id: "slate",
    preview: {
      shell: "#f4f7fb",
      sidebar: "#eef3f8",
      panel: "#fbfdff",
      accent: "#0f766e",
    },
  },
  {
    id: "warm",
    preview: {
      shell: "#fbf7f1",
      sidebar: "#f2eadf",
      panel: "#fffdf9",
      accent: "#c2410c",
    },
  },
];

export function normalizeColorScheme(value: string | undefined): ColorScheme {
  switch ((value ?? "").trim()) {
    case "contrast":
    case "slate":
    case "warm":
      return value as ColorScheme;
    case "default":
    default:
      return DEFAULT_COLOR_SCHEME;
  }
}
