import * as React from "react";

export type WindowMaterialMode = "native" | "css" | "solid";

export const REDUCED_TRANSPARENCY_MEDIA_QUERY =
  "(prefers-reduced-transparency: reduce)";

export interface WindowMaterialInputs {
  runtimeOS?: unknown;
  windowType?: string | null;
  surfaceStyle?: unknown;
  explicitReducedTransparency?: boolean;
  prefersReducedTransparency?: boolean;
}

interface WailsRuntimeHost {
  _wails?: {
    environment?: {
      OS?: unknown;
    };
  };
}

/**
 * The Wails runtime package creates `window._wails` in a regular browser too.
 * Only the backend-injected environment is proof that this page is hosted by
 * a real desktop WebView.
 */
export function readWailsRuntimeOS(host: unknown): string {
  const value = (host as WailsRuntimeHost | null | undefined)?._wails
    ?.environment?.OS;
  return typeof value === "string" ? value.trim().toLowerCase() : "";
}

export function resolveWindowMaterialMode({
  runtimeOS,
  windowType,
  surfaceStyle,
  explicitReducedTransparency = false,
  prefersReducedTransparency = false,
}: WindowMaterialInputs): WindowMaterialMode {
  if (explicitReducedTransparency || prefersReducedTransparency) {
    return "solid";
  }

  const normalizedWindowType = (windowType ?? "").trim();
  const normalizedOS =
    typeof runtimeOS === "string" ? runtimeOS.trim().toLowerCase() : "";
  const supportsNativeUnderlay =
    normalizedOS === "darwin" || normalizedOS === "windows";
  const normalizedSurfaceStyle =
    typeof surfaceStyle === "string" ? surfaceStyle.trim().toLowerCase() : "";
  const nativeWindow =
    !normalizedWindowType ||
    (normalizedWindowType === "settings" &&
      normalizedSurfaceStyle !== "contrast");

  return nativeWindow && supportsNativeUnderlay ? "native" : "css";
}

function readWindowType() {
  if (typeof window === "undefined") {
    return "";
  }
  return new URLSearchParams(window.location.search).get("window") ?? "";
}

export function readWindowSurfaceStyleHint(): string {
  if (typeof window === "undefined") {
    return "";
  }
  return (
    new URLSearchParams(window.location.search).get("surfaceStyle") ?? ""
  )
    .trim()
    .toLowerCase();
}

function readReducedTransparencyPreference() {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
    return false;
  }
  try {
    return window.matchMedia(REDUCED_TRANSPARENCY_MEDIA_QUERY).matches;
  } catch {
    return false;
  }
}

export function readWindowMaterialMode(
  surfaceStyle?: unknown,
): WindowMaterialMode {
  const root = typeof document === "undefined" ? null : document.documentElement;
  return resolveWindowMaterialMode({
    runtimeOS: readWailsRuntimeOS(
      typeof window === "undefined" ? undefined : window,
    ),
    windowType: readWindowType(),
    surfaceStyle:
      surfaceStyle ??
      root?.dataset.xiadownSurfaceStyle ??
      readWindowSurfaceStyleHint(),
    explicitReducedTransparency:
      root?.dataset.reduceTransparency === "true",
    prefersReducedTransparency: readReducedTransparencyPreference(),
  });
}

export function applyWindowMaterialMode(
  surfaceStyle?: unknown,
): WindowMaterialMode {
  const mode = readWindowMaterialMode(surfaceStyle);
  if (typeof document !== "undefined") {
    document.documentElement.dataset.windowMaterial = mode;
  }
  return mode;
}

/**
 * Keeps the semantic mode synchronized with both the app override and the OS
 * accessibility preference. The mode is mirrored onto `<html>` so body and
 * root backgrounds can reveal or mask the native underlay before React paints.
 */
export function useWindowMaterialMode(
  surfaceStyle?: unknown,
): WindowMaterialMode {
  const [mode, setMode] = React.useState<WindowMaterialMode>(() =>
    applyWindowMaterialMode(surfaceStyle),
  );

  React.useEffect(() => {
    const reconcile = () => setMode(applyWindowMaterialMode(surfaceStyle));
    reconcile();

    if (typeof window === "undefined" || typeof document === "undefined") {
      return;
    }

    const mediaQuery =
      typeof window.matchMedia === "function"
        ? window.matchMedia(REDUCED_TRANSPARENCY_MEDIA_QUERY)
        : null;
    mediaQuery?.addEventListener?.("change", reconcile);

    const observer =
      typeof MutationObserver === "undefined"
        ? null
        : new MutationObserver(reconcile);
    observer?.observe(document.documentElement, {
      attributes: true,
      attributeFilter: [
        "data-reduce-transparency",
        "data-xiadown-surface-style",
      ],
    });

    return () => {
      observer?.disconnect();
      mediaQuery?.removeEventListener?.("change", reconcile);
    };
  }, [surfaceStyle]);

  return mode;
}
