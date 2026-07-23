type Listener = () => void;

export interface ActiveWindowTrackerOptions {
  addWindowListener: (name: string, listener: Listener) => void;
  removeWindowListener: (name: string, listener: Listener) => void;
  addDocumentListener: (name: string, listener: Listener) => void;
  removeDocumentListener: (name: string, listener: Listener) => void;
  subscribeRuntimeEvent: (name: string, listener: Listener) => () => void;
  setHeartbeat: (listener: Listener, intervalMs: number) => number;
  clearHeartbeat: (handle: number) => void;
  updateActive: (active?: boolean) => void;
  shouldHeartbeat: () => boolean;
  heartbeatMs: number;
}

const RUNTIME_EVENT_STATES = [
  ["common:WindowFocus", "focused"],
  ["common:WindowLostFocus", "inactive"],
  ["common:WindowMinimise", "inactive"],
  ["common:WindowHide", "inactive"],
  ["common:WindowUnMinimise", "recompute"],
  ["common:WindowShow", "recompute"],
  ["mac:ApplicationDidBecomeActive", "focused"],
  ["mac:ApplicationDidResignActive", "inactive"],
  ["windows:WindowActive", "focused"],
  ["windows:WindowInactive", "inactive"],
  ["windows:WindowSetFocus", "focused"],
  ["windows:WindowKillFocus", "inactive"],
  ["linux:WindowFocusIn", "focused"],
  ["linux:WindowFocusOut", "inactive"],
] as const;

export function installActiveWindowTracker(options: ActiveWindowTrackerOptions) {
  const markFocused = () => options.updateActive(true);
  const markInactive = () => options.updateActive(false);
  const recompute = () => options.updateActive();

  options.addWindowListener("focus", markFocused);
  options.addWindowListener("blur", markInactive);
  options.addWindowListener("pagehide", markInactive);
  options.addWindowListener("beforeunload", markInactive);
  options.addDocumentListener("visibilitychange", recompute);

  const runtimeUnsubscribers: Array<() => void> = [];
  RUNTIME_EVENT_STATES.forEach(([name, state]) => {
    const listener = state === "focused" ? markFocused : state === "inactive" ? markInactive : recompute;
    try {
      runtimeUnsubscribers.push(options.subscribeRuntimeEvent(name, listener));
    } catch {
      // Browser focus and visibility events remain as the fallback path.
    }
  });

  const heartbeat = options.setHeartbeat(() => {
    if (options.shouldHeartbeat()) {
      options.updateActive(true);
    }
  }, options.heartbeatMs);

  let disposed = false;
  return () => {
    if (disposed) {
      return;
    }
    disposed = true;
    options.removeWindowListener("focus", markFocused);
    options.removeWindowListener("blur", markInactive);
    options.removeWindowListener("pagehide", markInactive);
    options.removeWindowListener("beforeunload", markInactive);
    options.removeDocumentListener("visibilitychange", recompute);
    options.clearHeartbeat(heartbeat);
    runtimeUnsubscribers.forEach((unsubscribe) => unsubscribe());
    options.updateActive(false);
  };
}
