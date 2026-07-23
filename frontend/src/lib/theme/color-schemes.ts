import type { ColorScheme } from "@/shared/contracts/settings";

export const DEFAULT_COLOR_SCHEME: ColorScheme = "default";

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
