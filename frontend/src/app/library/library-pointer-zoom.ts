export interface PointerZoomAnchor {
  contentRatioX: number;
  contentRatioY: number;
  viewportX: number;
  viewportY: number;
}

const WHEEL_LINE_HEIGHT_PX = 16;
const MAX_WHEEL_DELTA_PX = 120;
const WHEEL_ZOOM_SENSITIVITY = 0.0025;

export function normalizeWheelDeltaY(
  deltaY: number,
  deltaMode: number,
  viewportHeight: number,
) {
  if (!Number.isFinite(deltaY)) return 0;
  if (deltaMode === 1) return deltaY * WHEEL_LINE_HEIGHT_PX;
  if (deltaMode === 2) {
    return deltaY * Math.max(1, Number.isFinite(viewportHeight)
      ? viewportHeight
      : 1);
  }
  return deltaY;
}

export function zoomAfterWheel(
  current: number,
  deltaY: number,
  deltaMode: number,
  viewportHeight: number,
  clamp: (value: number) => number,
) {
  const normalized = normalizeWheelDeltaY(deltaY, deltaMode, viewportHeight);
  if (normalized === 0) return clamp(current);
  const boundedDelta = Math.min(
    MAX_WHEEL_DELTA_PX,
    Math.max(-MAX_WHEEL_DELTA_PX, normalized),
  );
  return clamp(
    current * Math.exp(-boundedDelta * WHEEL_ZOOM_SENSITIVITY),
  );
}

export function capturePointerZoomAnchor(
  stage: HTMLElement,
  clientX: number,
  clientY: number,
): PointerZoomAnchor {
  const rect = stage.getBoundingClientRect();
  const viewportX = Math.min(
    stage.clientWidth,
    Math.max(0, clientX - rect.left),
  );
  const viewportY = Math.min(
    stage.clientHeight,
    Math.max(0, clientY - rect.top),
  );
  return {
    contentRatioX: (stage.scrollLeft + viewportX) /
      Math.max(1, stage.scrollWidth),
    contentRatioY: (stage.scrollTop + viewportY) /
      Math.max(1, stage.scrollHeight),
    viewportX,
    viewportY,
  };
}

export function restorePointerZoomAnchor(
  stage: HTMLElement,
  anchor: PointerZoomAnchor,
) {
  stage.scrollLeft = Math.max(
    0,
    anchor.contentRatioX * stage.scrollWidth - anchor.viewportX,
  );
  stage.scrollTop = Math.max(
    0,
    anchor.contentRatioY * stage.scrollHeight - anchor.viewportY,
  );
}
