import * as React from "react";

const LISTEN_NATIVE_VIDEO_HOLE_PROPS = [
  "--listen-native-video-hole-x",
  "--listen-native-video-hole-y",
  "--listen-native-video-hole-w",
  "--listen-native-video-hole-h",
  "--listen-native-video-hole-r",
] as const;

const activeOwners = new Set<symbol>();
let currentHoleOwner: symbol | null = null;

function clearNativeVideoHole(documentElement: HTMLElement) {
  LISTEN_NATIVE_VIDEO_HOLE_PROPS.forEach((property) => {
    documentElement.style.removeProperty(property);
  });
}

function activateNativeVideoUnderlay(owner: symbol) {
  if (typeof document === "undefined") {
    return null;
  }
  activeOwners.add(owner);
  document.documentElement.dataset.listenNativeVideoUnderlay = "true";
  return document.documentElement;
}

function deactivateNativeVideoUnderlay(owner: symbol) {
  if (typeof document === "undefined") {
    return;
  }
  const documentElement = document.documentElement;
  activeOwners.delete(owner);
  if (currentHoleOwner === owner) {
    currentHoleOwner = null;
    clearNativeVideoHole(documentElement);
  }
  if (activeOwners.size === 0) {
    delete documentElement.dataset.listenNativeVideoUnderlay;
    clearNativeVideoHole(documentElement);
  }
}

export function readListenNativeVideoRadius(element: Element) {
  const style = window.getComputedStyle(element);
  const values = [
    style.borderTopLeftRadius,
    style.borderTopRightRadius,
    style.borderBottomRightRadius,
    style.borderBottomLeftRadius,
  ]
    .map((value) => Number.parseFloat(value))
    .filter((value) => Number.isFinite(value) && value > 0);
  return values.length > 0 ? Math.max(...values) : 0;
}

export function useListenNativeVideoUnderlay(active: boolean) {
  const ownerRef = React.useRef<symbol>();
  if (!ownerRef.current) {
    ownerRef.current = Symbol("listen-native-video-underlay");
  }

  React.useLayoutEffect(() => {
    if (!active) {
      return;
    }
    const owner = ownerRef.current;
    if (!owner) {
      return;
    }
    activateNativeVideoUnderlay(owner);
    return () => deactivateNativeVideoUnderlay(owner);
  }, [active]);

  const setHole = React.useCallback((rect: DOMRect, radius: number) => {
    const owner = ownerRef.current;
    if (!owner) {
      return;
    }
    const documentElement = activateNativeVideoUnderlay(owner);
    if (!documentElement) {
      return;
    }
    currentHoleOwner = owner;
    documentElement.style.setProperty(
      "--listen-native-video-hole-x",
      `${Math.max(0, rect.left)}px`,
    );
    documentElement.style.setProperty(
      "--listen-native-video-hole-y",
      `${Math.max(0, rect.top)}px`,
    );
    documentElement.style.setProperty(
      "--listen-native-video-hole-w",
      `${Math.max(1, rect.width)}px`,
    );
    documentElement.style.setProperty(
      "--listen-native-video-hole-h",
      `${Math.max(1, rect.height)}px`,
    );
    documentElement.style.setProperty(
      "--listen-native-video-hole-r",
      `${Math.max(0, radius)}px`,
    );
  }, []);

  const resetHole = React.useCallback(() => {
    const owner = ownerRef.current;
    if (!owner || typeof document === "undefined" || currentHoleOwner !== owner) {
      return;
    }
    currentHoleOwner = null;
    clearNativeVideoHole(document.documentElement);
  }, []);

  return React.useMemo(
    () => ({
      resetHole,
      setHole,
    }),
    [resetHole, setHole],
  );
}
