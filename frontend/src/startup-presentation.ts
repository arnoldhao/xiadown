export const STARTUP_READY_EVENT = "xiadown:startup-ready";
export const STARTUP_ERROR_EVENT = "xiadown:startup-error";
export const STARTUP_SURFACE_READY_EVENT = "xiadown:startup-surface-ready";

const MAIN_BOOT_READY_METHOD =
  "xiadown/internal/presentation/wails.SettingsHandler.MarkMainWindowBootReady";
const MAIN_BOOT_FAILED_METHOD =
  "xiadown/internal/presentation/wails.SettingsHandler.MarkMainWindowBootFailed";
const STARTUP_SHELL_ID = "xiadown-startup";
const STARTUP_TRANSITION_FALLBACK_MS = 420;
const NATIVE_HOST_WAIT_MS = 8_000;
const NATIVE_HOST_POLL_MS = 25;
const NATIVE_CALL_RETRY_DELAYS_MS = [0, 150, 400, 800, 1_600] as const;

interface WailsStartupHost {
  _wails?: {
    environment?: {
      OS?: unknown;
    };
  };
  chrome?: {
    webview?: {
      postMessage?: unknown;
    };
  };
  webkit?: {
    messageHandlers?: {
      external?: {
        postMessage?: unknown;
      };
    };
  };
}

export function readStartupWindowType(search?: string): string {
  const source =
    search ??
    (typeof window === "undefined" ? "" : window.location.search);
  return new URLSearchParams(source).get("window")?.trim() ?? "";
}

export function isMainStartupWindow(search?: string): boolean {
  return readStartupWindowType(search) === "";
}

export function hasNativeWailsHost(host?: Window): boolean {
  const runtimeOS = (host as WailsStartupHost | undefined)?._wails
    ?.environment?.OS;
  return typeof runtimeOS === "string" && runtimeOS.trim().length > 0;
}

export function hasNativeWailsTransport(host?: Window): boolean {
  const candidate = host as WailsStartupHost | undefined;
  return (
    hasNativeWailsHost(host) ||
    typeof candidate?.chrome?.webview?.postMessage === "function" ||
    typeof candidate?.webkit?.messageHandlers?.external?.postMessage ===
      "function"
  );
}

export async function waitForNativeWailsHost(
  host: Window,
  timeoutMs = NATIVE_HOST_WAIT_MS,
): Promise<boolean> {
  if (hasNativeWailsHost(host)) {
    return true;
  }
  if (!hasNativeWailsTransport(host)) {
    return false;
  }

  // Wails injects `_wails.environment` from its navigation-finished callback.
  // The entry module can run slightly earlier, especially in dev mode, so an
  // immediate host check would permanently miss the native handshake and make
  // the window wait for the five-second Go fallback.
  const deadline = Date.now() + Math.max(0, timeoutMs);
  while (Date.now() < deadline) {
    await new Promise<void>((resolve) => {
      host.setTimeout(resolve, NATIVE_HOST_POLL_MS);
    });
    if (hasNativeWailsHost(host)) {
      return true;
    }
  }
  return false;
}

/**
 * Tells the native host that React has committed its stable first frame. On
 * macOS this removes the already-visible NSWindow startup overlay; it never
 * controls the first appearance of that window.
 */
export async function markMainWindowBootReady(
  host = window,
): Promise<boolean> {
  if (!isMainStartupWindow(host.location.search)) {
    return false;
  }

  const nativeHostReady = await waitForNativeWailsHost(host);
  if (!nativeHostReady) {
    return false;
  }

  await callNativeStartupMethod(MAIN_BOOT_READY_METHOD, host);
  return true;
}

async function callNativeStartupMethod(
  method: string,
  host: Window,
): Promise<void> {
  const { Call } = await import("@wailsio/runtime");
  let lastError: unknown;
  for (const delay of NATIVE_CALL_RETRY_DELAYS_MS) {
    if (delay > 0) {
      await new Promise<void>((resolve) => host.setTimeout(resolve, delay));
    }
    try {
      await Call.ByName(method);
      return;
    } catch (error) {
      lastError = error;
    }
  }
  throw lastError ?? new Error(`Native startup call failed: ${method}`);
}

async function releaseNativeStartupFailure(host: Window): Promise<void> {
  if (
    !isMainStartupWindow(host.location.search) ||
    !(await waitForNativeWailsHost(host))
  ) {
    return;
  }
  await callNativeStartupMethod(MAIN_BOOT_FAILED_METHOD, host);
}

function removeStartupShell(
  documentRef: Document,
  host: Window,
  immediately = false,
) {
  const shell = documentRef.getElementById(STARTUP_SHELL_ID);
  if (!shell) {
    return;
  }

  if (immediately) {
    shell.remove();
    return;
  }

  let removed = false;
  let fallbackID: number | undefined;
  const remove = () => {
    if (removed) {
      return;
    }
    removed = true;
    if (fallbackID !== undefined) {
      host.clearTimeout(fallbackID);
    }
    shell.remove();
  };

  const reducedMotion =
    typeof host.matchMedia === "function" &&
    host.matchMedia("(prefers-reduced-motion: reduce)").matches;
  if (reducedMotion) {
    host.queueMicrotask(remove);
    return;
  }

  shell.addEventListener(
    "transitionend",
    (event) => {
      if (event.target === shell && event.propertyName === "opacity") {
        remove();
      }
    },
    { once: true },
  );
  fallbackID = host.setTimeout(remove, STARTUP_TRANSITION_FALLBACK_MS);
}

export function completeStartupPresentation(
  documentRef = document,
  host = window,
): void {
  const root = documentRef.documentElement;
  if (root.dataset.startupState === "ready") {
    return;
  }

  const applicationRoot = documentRef.getElementById("root");
  applicationRoot?.setAttribute("aria-busy", "false");
  applicationRoot?.removeAttribute("aria-hidden");
  applicationRoot?.removeAttribute("inert");
  documentRef.getElementById(STARTUP_SHELL_ID)?.setAttribute("aria-hidden", "true");
  root.dataset.startupState = "ready";
  host.dispatchEvent(new Event(STARTUP_READY_EVENT));
  // The HTML shell is only a browser/non-macOS fallback. When a native overlay
  // owns the visible transition, remove the hidden duplicate synchronously so
  // two icons cannot cross-fade through each other.
  removeStartupShell(
    documentRef,
    host,
    isMainStartupWindow(host.location.search) && hasNativeWailsTransport(host),
  );
}

/**
 * Lets the browser commit the real application DOM before starting the
 * cross-fade. A short timer keeps hidden/throttled auxiliary windows from
 * waiting forever for requestAnimationFrame.
 */
export function scheduleStartupCompletion(
  documentRef = document,
  host = window,
): () => void {
  let disposed = false;
  let contentCommitted = false;
  let contentFrameID: number | undefined;
  let handshakeFrameID: number | undefined;
  let contentFallbackID: number | undefined;
  let handshakeFallbackID: number | undefined;
  let handshakeStarted = false;

  const notifyNativeHost = () => {
    if (disposed || handshakeStarted) {
      return;
    }
    handshakeStarted = true;
    if (handshakeFallbackID !== undefined) {
      host.clearTimeout(handshakeFallbackID);
    }
    void markMainWindowBootReady(host).catch((error) => {
      // Keep the native surface in place. The Go timeout path deliberately does
      // not remove it when HTML/bridge startup has failed.
      console.warn("XiaDown native startup transition failed", error);
    });
  };

  const commitContent = () => {
    if (disposed || contentCommitted) {
      return;
    }
    contentCommitted = true;
    if (contentFallbackID !== undefined) {
      host.clearTimeout(contentFallbackID);
    }
    completeStartupPresentation(documentRef, host);
    // A second frame ensures the real application has been composited below
    // the native overlay before AppKit starts its scale/fade transition.
    handshakeFrameID = host.requestAnimationFrame(notifyNativeHost);
    handshakeFallbackID = host.setTimeout(notifyNativeHost, 100);
  };

  contentFrameID = host.requestAnimationFrame(commitContent);
  contentFallbackID = host.setTimeout(commitContent, 100);

  return () => {
    disposed = true;
    if (contentFrameID !== undefined) {
      host.cancelAnimationFrame(contentFrameID);
    }
    if (handshakeFrameID !== undefined) {
      host.cancelAnimationFrame(handshakeFrameID);
    }
    if (contentFallbackID !== undefined) {
      host.clearTimeout(contentFallbackID);
    }
    if (handshakeFallbackID !== undefined) {
      host.clearTimeout(handshakeFallbackID);
    }
  };
}

export function reportStartupFailure(
  error: unknown,
  documentRef = document,
  host = window,
): void {
  console.error("XiaDown frontend startup failed", error);
  if (documentRef.documentElement.dataset.startupState !== "ready") {
    documentRef.documentElement.dataset.startupState = "error";
  }
  host.dispatchEvent(new Event(STARTUP_ERROR_EVENT));
  if (hasNativeWailsTransport(host)) {
    void releaseNativeStartupFailure(host).catch((nativeError) => {
      console.warn("XiaDown native startup fallback failed", nativeError);
    });
  }
}
