import * as React from "react";

const LISTEN_NATIVE_VIDEO_HOLE_PROPS = [
  "--listen-native-video-hole-x",
  "--listen-native-video-hole-y",
  "--listen-native-video-hole-w",
  "--listen-native-video-hole-h",
  "--listen-native-video-hole-r",
] as const;

const LISTEN_NATIVE_VIDEO_PRIMARY_HOLE_PROPS = [
  "--listen-native-video-primary-hole-x",
  "--listen-native-video-primary-hole-y",
  "--listen-native-video-primary-hole-w",
  "--listen-native-video-primary-hole-h",
  "--listen-native-video-primary-hole-r",
] as const;

const activeOwners = new Set<symbol>();
let currentHoleOwner: symbol | null = null;

type ListenNativeVideoHole = {
  left: number;
  top: number;
  width: number;
  height: number;
  radius: number;
};

type ListenNativeVideoRect = Pick<
  DOMRect,
  "left" | "top" | "width" | "height"
>;

export function resolveListenNativeVideoPrimaryHole(
  hole: ListenNativeVideoHole,
  primaryPane: ListenNativeVideoRect,
): ListenNativeVideoHole {
  const paneWidth = Math.max(0, primaryPane.width);
  const paneHeight = Math.max(0, primaryPane.height);
  const left = clampNativeVideoCoordinate(
    hole.left - primaryPane.left,
    paneWidth,
  );
  const top = clampNativeVideoCoordinate(
    hole.top - primaryPane.top,
    paneHeight,
  );
  const right = clampNativeVideoCoordinate(
    hole.left + hole.width - primaryPane.left,
    paneWidth,
  );
  const bottom = clampNativeVideoCoordinate(
    hole.top + hole.height - primaryPane.top,
    paneHeight,
  );
  const width = Math.max(1, right - left);
  const height = Math.max(1, bottom - top);

  return {
    left,
    top,
    width,
    height,
    radius: Math.min(
      Math.max(0, hole.radius),
      width / 2,
      height / 2,
    ),
  };
}

function clampNativeVideoCoordinate(value: number, maximum: number) {
  return Math.min(maximum, Math.max(0, Number.isFinite(value) ? value : 0));
}

function clearNativeVideoHole(documentElement: HTMLElement) {
  [
    ...LISTEN_NATIVE_VIDEO_HOLE_PROPS,
    ...LISTEN_NATIVE_VIDEO_PRIMARY_HOLE_PROPS,
  ].forEach((property) => {
    documentElement.style.removeProperty(property);
  });
}

function writeNativeVideoHole(
  documentElement: HTMLElement,
  hole: ListenNativeVideoHole,
) {
  documentElement.style.setProperty(
    "--listen-native-video-hole-x",
    `${Math.max(0, hole.left)}px`,
  );
  documentElement.style.setProperty(
    "--listen-native-video-hole-y",
    `${Math.max(0, hole.top)}px`,
  );
  documentElement.style.setProperty(
    "--listen-native-video-hole-w",
    `${Math.max(1, hole.width)}px`,
  );
  documentElement.style.setProperty(
    "--listen-native-video-hole-h",
    `${Math.max(1, hole.height)}px`,
  );
  documentElement.style.setProperty(
    "--listen-native-video-hole-r",
    `${Math.max(0, hole.radius)}px`,
  );

  const primaryPaneRect = findListenNativeVideoPrimaryPaneRect(hole);
  if (!primaryPaneRect) {
    LISTEN_NATIVE_VIDEO_PRIMARY_HOLE_PROPS.forEach((property) => {
      documentElement.style.removeProperty(property);
    });
    return;
  }
  const primaryHole = resolveListenNativeVideoPrimaryHole(
    hole,
    primaryPaneRect,
  );
  documentElement.style.setProperty(
    "--listen-native-video-primary-hole-x",
    `${primaryHole.left}px`,
  );
  documentElement.style.setProperty(
    "--listen-native-video-primary-hole-y",
    `${primaryHole.top}px`,
  );
  documentElement.style.setProperty(
    "--listen-native-video-primary-hole-w",
    `${primaryHole.width}px`,
  );
  documentElement.style.setProperty(
    "--listen-native-video-primary-hole-h",
    `${primaryHole.height}px`,
  );
  documentElement.style.setProperty(
    "--listen-native-video-primary-hole-r",
    `${primaryHole.radius}px`,
  );
}

function findListenNativeVideoPrimaryPaneRect(
  hole: ListenNativeVideoHole,
): ListenNativeVideoRect | null {
  const centerX = hole.left + hole.width / 2;
  const centerY = hole.top + hole.height / 2;
  for (const pane of document.querySelectorAll<HTMLElement>(
    ".app-workspace-primary-pane",
  )) {
    const rect = pane.getBoundingClientRect();
    if (
      rect.width > 0 &&
      rect.height > 0 &&
      centerX >= rect.left &&
      centerX <= rect.right &&
      centerY >= rect.top &&
      centerY <= rect.bottom
    ) {
      return rect;
    }
  }
  return null;
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
  const activeRef = React.useRef(active);
  const holeRef = React.useRef<ListenNativeVideoHole | null>(null);
  if (!ownerRef.current) {
    ownerRef.current = Symbol("listen-native-video-underlay");
  }
  activeRef.current = active;

  React.useLayoutEffect(() => {
    const owner = ownerRef.current;
    if (!owner) {
      return;
    }
    if (!active) {
      holeRef.current = null;
      deactivateNativeVideoUnderlay(owner);
      return;
    }
    const hole = holeRef.current;
    const documentElement = hole ? activateNativeVideoUnderlay(owner) : null;
    if (documentElement && hole) {
      currentHoleOwner = owner;
      writeNativeVideoHole(documentElement, hole);
    }
    return () => deactivateNativeVideoUnderlay(owner);
  }, [active]);

  const setHole = React.useCallback((rect: DOMRect, radius: number) => {
    const hole = {
      left: rect.left,
      top: rect.top,
      width: rect.width,
      height: rect.height,
      radius,
    };
    holeRef.current = hole;
    if (!activeRef.current) {
      return;
    }
    const owner = ownerRef.current;
    if (!owner) {
      return;
    }
    const documentElement = activateNativeVideoUnderlay(owner);
    if (!documentElement) {
      return;
    }
    currentHoleOwner = owner;
    writeNativeVideoHole(documentElement, hole);
  }, []);

  const resetHole = React.useCallback(() => {
    holeRef.current = null;
    const owner = ownerRef.current;
    if (!owner || typeof document === "undefined") {
      return;
    }
    deactivateNativeVideoUnderlay(owner);
  }, []);

  return React.useMemo(
    () => ({
      resetHole,
      setHole,
    }),
    [resetHole, setHole],
  );
}
