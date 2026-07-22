import { Events, Window } from "@wailsio/runtime";
import * as React from "react";

import {
  resolveRSSHostVideoFullscreenOwnership,
  shouldRestoreRSSHostWindow,
  type RSSHostVideoFullscreenOwnership,
} from "./host-video-fullscreen-state";

/**
 * Keeps the complete React playback surface (video, transport, popovers) in
 * the main Wails window while the App owns fullscreen presentation. This is
 * intentionally different from asking a provider document to fullscreen its
 * own video element, which would detach the native WebView from React controls.
 */
export function useRSSHostVideoFullscreen(active: boolean) {
  const [fullscreen, setFullscreen] = React.useState(false);
  const activeRef = React.useRef(active);
  const mountedRef = React.useRef(true);
  const fullscreenRef = React.useRef(false);
  const ownershipRef = React.useRef<RSSHostVideoFullscreenOwnership>("none");
  const operationRef = React.useRef<Promise<void>>(Promise.resolve());
  activeRef.current = active;

  const updateFullscreen = React.useCallback((next: boolean) => {
    fullscreenRef.current = next;
    if (mountedRef.current) setFullscreen(next);
  }, []);

  const enter = React.useCallback(async () => {
    if (!activeRef.current || fullscreenRef.current) return;
    updateFullscreen(true);
    let ownership: RSSHostVideoFullscreenOwnership = "none";
    try {
      const windowWasFullscreen = await Window.IsFullscreen();
      if (!activeRef.current) {
        updateFullscreen(false);
        return;
      }
      ownership = resolveRSSHostVideoFullscreenOwnership(windowWasFullscreen);
      ownershipRef.current = ownership;
      if (ownership === "owned") {
        await Window.Fullscreen();
      }
      if (!activeRef.current) {
        ownershipRef.current = "none";
        updateFullscreen(false);
        if (ownership === "owned") {
          await Window.UnFullscreen().catch(() => undefined);
        }
      }
    } catch (reason) {
      ownershipRef.current = "none";
      updateFullscreen(false);
      throw reason;
    }
  }, [updateFullscreen]);

  const exit = React.useCallback(async () => {
    if (!fullscreenRef.current && ownershipRef.current === "none") return;
    const ownership = ownershipRef.current;
    ownershipRef.current = "none";
    updateFullscreen(false);
    if (!shouldRestoreRSSHostWindow(ownership)) return;
    if (await Window.IsFullscreen().catch(() => false)) {
      await Window.UnFullscreen();
    }
  }, [updateFullscreen]);

  const enqueue = React.useCallback((operation: () => Promise<void>) => {
    const next = operationRef.current.then(operation, operation);
    operationRef.current = next.catch(() => undefined);
    return next;
  }, []);

  const toggle = React.useCallback(
    () => enqueue(() => (fullscreenRef.current ? exit() : enter())),
    [enqueue, enter, exit],
  );

  React.useLayoutEffect(() => {
    if (!fullscreen || typeof document === "undefined") return;
    const root = document.documentElement;
    root.dataset.rssHostVideoFullscreen = "true";
    return () => {
      delete root.dataset.rssHostVideoFullscreen;
    };
  }, [fullscreen]);

  React.useEffect(() => {
    const offWindowUnFullscreen = Events.On(
      Events.Types.Common.WindowUnFullscreen,
      () => {
        if (!fullscreenRef.current) return;
        ownershipRef.current = "none";
        updateFullscreen(false);
      },
    );
    return offWindowUnFullscreen;
  }, [updateFullscreen]);

  React.useEffect(() => {
    if (active || !fullscreenRef.current) return;
    void enqueue(exit);
  }, [active, enqueue, exit]);

  React.useEffect(() => {
    if (!fullscreen) return;
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      event.preventDefault();
      event.stopPropagation();
      void enqueue(exit);
    };
    window.addEventListener("keydown", handleKeyDown, true);
    return () => window.removeEventListener("keydown", handleKeyDown, true);
  }, [enqueue, exit, fullscreen]);

  React.useEffect(() => {
    // React StrictMode replays effect setup after its development cleanup.
    mountedRef.current = true;
    activeRef.current = active;
    return () => {
      mountedRef.current = false;
      activeRef.current = false;
      fullscreenRef.current = false;
      const ownership = ownershipRef.current;
      ownershipRef.current = "none";
      if (shouldRestoreRSSHostWindow(ownership)) {
        void Window.UnFullscreen().catch(() => undefined);
      }
    };
  // Mount ownership is deliberately stable; activeRef is refreshed in render.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return { fullscreen, toggle };
}
