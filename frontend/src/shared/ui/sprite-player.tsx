import * as React from "react";

import type { Sprite } from "@/shared/contracts/sprites";
import { SPRITE_ROW_BY_ANIMATION, type SpriteAnimation } from "@/shared/sprites/animation";
import { cn } from "@/lib/utils";
import { SPRITE_DISPLAY_GLOW_STYLE } from "@/shared/styles/xiadown";

const SPRITE_PLAYBACK_SLOWDOWN = 2;
const FRAME_DURATION_MS = 220 * SPRITE_PLAYBACK_SLOWDOWN;
const MAGENTA_PROCESS_CACHE_VERSION = "frontend-magenta-v1";
const MAGENTA_CHANNEL_TOLERANCE = 120;
const MAGENTA_COLOR_DISTANCE = 232;
const MIN_MAGENTA_STRENGTH = 20;
const MAGENTA_EDGE_SPILL_THRESHOLD = 12;
const MAGENTA_EDGE_ALPHA_SOFTEN = 0.48;
const MAGENTA_EDGE_MATCH_ALPHA_SOFTEN = 0.82;
const MAGENTA_EDGE_CHANNEL_REDUCTION = 0.9;
const MAGENTA_HARD_CLEAR_MATCH = 0.9;
const SAMPLE_POINT_FACTORS = [
  [0, 0],
  [0.5, 0],
  [1, 0],
  [0, 0.5],
  [1, 0.5],
  [0, 1],
  [0.5, 1],
  [1, 1],
] as const;

interface BackgroundColor {
  r: number;
  g: number;
  b: number;
}

const spriteImageCache = new Map<string, Promise<HTMLImageElement | null>>();
const processedSheetCache = new Map<string, Promise<HTMLCanvasElement | null>>();
const previewFrameCache = new Map<string, Promise<HTMLCanvasElement | null>>();
const rowFrameCache = new WeakMap<HTMLCanvasElement, Map<string, HTMLCanvasElement[]>>();
const SPRITE_IMAGE_CACHE_LIMIT = 48;
const PROCESSED_SHEET_CACHE_LIMIT = 24;
const PREVIEW_FRAME_CACHE_LIMIT = 96;

export function SpriteDisplay(props: {
  sprite: Sprite | null;
  imageUrl: string;
  staticImageUrl?: string;
  animation?: SpriteAnimation;
  animate?: boolean;
  alt: string;
  className?: string;
  glowClassName?: string;
  glowStyle?: React.CSSProperties;
  spriteClassName?: string;
  fallbackSrc?: string;
  size?: number;
}) {
  const {
    sprite,
    imageUrl,
    staticImageUrl,
    animation,
    animate,
    alt,
    className,
    glowClassName,
    glowStyle,
    spriteClassName,
    fallbackSrc,
    size,
  } = props;
  const spriteSize = size ?? 64;
  const containerStyle: React.CSSProperties = { userSelect: "none" };
  const imageStyle: React.CSSProperties = { userSelect: "none" };

  if (size !== undefined) {
    containerStyle.width = size;
    containerStyle.height = size;
    imageStyle.width = size;
    imageStyle.height = size;
  }

  return (
    <div
      className={cn(
        "relative flex h-20 w-20 items-center justify-center overflow-visible select-none",
        className,
      )}
      style={containerStyle}
    >
      <div
        aria-hidden="true"
        className={cn(
          "pointer-events-none absolute left-1/2 top-1/2 h-48 w-64 -translate-x-1/2 -translate-y-1/2 blur-xl",
          glowClassName,
        )}
        style={
          glowStyle
            ? { ...SPRITE_DISPLAY_GLOW_STYLE, ...glowStyle }
            : SPRITE_DISPLAY_GLOW_STYLE
        }
      />
      {staticImageUrl ? (
        <img
          src={staticImageUrl}
          alt={alt}
          width={spriteSize}
          height={spriteSize}
          loading="lazy"
          decoding="async"
          draggable={false}
          className={cn(
            "relative z-10 shrink-0 select-none object-contain",
            size === undefined && "h-16 w-16",
            spriteClassName,
          )}
          style={imageStyle}
        />
      ) : (
        <SpritePlayer
          sprite={sprite}
          imageUrl={imageUrl}
          animation={animation}
          animate={animate}
          size={spriteSize}
          alt={alt}
          fallbackSrc={fallbackSrc}
          className={cn("relative z-10 shrink-0 select-none", spriteClassName)}
        />
      )}
    </div>
  );
}

export function SpritePlayer(props: {
  sprite: Sprite | null;
  imageUrl: string;
  animation?: SpriteAnimation;
  size?: number;
  animate?: boolean;
  alt: string;
  className?: string;
  fallbackSrc?: string;
}) {
  const {
    sprite,
    imageUrl,
    animation = "greeting",
    size = 112,
    animate = true,
    alt,
    className,
    fallbackSrc = "/appicon.png",
  } = props;
  const canvasRef = React.useRef<HTMLCanvasElement | null>(null);
  const [sourceCanvas, setSourceCanvas] = React.useState<HTMLCanvasElement | null>(null);
  const [useCanvasPlayback, setUseCanvasPlayback] = React.useState(true);
  const motionClassName = animate && animation !== "working" ? "sprite-motion-breathe" : undefined;
  const isReady = Boolean(
    sprite &&
      sprite.status === "ready" &&
      sprite.imageWidth > 0 &&
      sprite.imageHeight > 0 &&
      sprite.columns > 0 &&
      sprite.rows > 0 &&
      imageUrl,
  );

  React.useEffect(() => {
    if (!isReady || !sprite) {
      setSourceCanvas(null);
      setUseCanvasPlayback(true);
      return;
    }

    let active = true;
    const rowIndex = Math.min(Math.max(1, sprite.rows || 1) - 1, SPRITE_ROW_BY_ANIMATION[animation] ?? 0);
    const loader = animate
      ? loadProcessedSpriteSheet(imageUrl, sprite)
      : loadProcessedSpritePreviewFrame(imageUrl, sprite, rowIndex, 0);

    void loader.then((nextCanvas) => {
      if (!active) {
        return;
      }
      if (!nextCanvas) {
        setUseCanvasPlayback(false);
        setSourceCanvas(null);
        return;
      }
      setUseCanvasPlayback(true);
      setSourceCanvas(nextCanvas);
    });

    return () => {
      active = false;
    };
  }, [animate, animation, imageUrl, isReady, sprite?.columns, sprite?.id, sprite?.rows]);

  React.useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) {
      return;
    }
    const context = canvas.getContext("2d");
    if (!context) {
      return;
    }

    const dpr = window.devicePixelRatio || 1;
    canvas.width = Math.round(size * dpr);
    canvas.height = Math.round(size * dpr);
    canvas.style.width = `${size}px`;
    canvas.style.height = `${size}px`;

    const clear = () => {
      context.setTransform(dpr, 0, 0, dpr, 0, 0);
      context.clearRect(0, 0, size, size);
    };

    clear();
    if (!isReady || !sourceCanvas || !sprite) {
      return;
    }

    if (!animate) {
      applyHighQualitySmoothing(context);
      context.drawImage(
        sourceCanvas,
        0,
        0,
        sourceCanvas.width,
        sourceCanvas.height,
        0,
        0,
        size,
        size,
      );
      return;
    }

    const totalFrames = Math.max(1, sprite.columns || 1);
    const totalRows = Math.max(1, sprite.rows || 1);
    const rowIndex = Math.min(totalRows - 1, SPRITE_ROW_BY_ANIMATION[animation] ?? 0);
    const frameCanvases = buildRowFrameCanvases(sourceCanvas, rowIndex, totalFrames, totalRows);
    let frameIndex = 0;
    let frameHandle = 0;
    let timeoutHandle = 0;
    let active = true;

    const drawFrame = (nextFrameIndex: number) => {
      clear();
      applyHighQualitySmoothing(context);
      const frameCanvas = frameCanvases[nextFrameIndex] ?? frameCanvases[0];
      if (!frameCanvas) {
        return;
      }

      context.drawImage(
        frameCanvas,
        0,
        0,
        frameCanvas.width,
        frameCanvas.height,
        0,
        0,
        size,
        size,
      );
    };

    drawFrame(0);
    if (totalFrames <= 1) {
      return () => {
        active = false;
      };
    }

    const scheduleNextFrame = () => {
      timeoutHandle = window.setTimeout(() => {
        if (!active) {
          return;
        }
        frameHandle = window.requestAnimationFrame(() => {
          if (!active) {
            return;
          }
          frameIndex = (frameIndex + 1) % totalFrames;
          drawFrame(frameIndex);
          scheduleNextFrame();
        });
      }, FRAME_DURATION_MS);
    };

    scheduleNextFrame();
    return () => {
      active = false;
      window.clearTimeout(timeoutHandle);
      window.cancelAnimationFrame(frameHandle);
    };
  }, [animate, animation, isReady, size, sourceCanvas, sprite?.columns, sprite?.id, sprite?.rows]);

  if (!sprite) {
    return (
      <img
        src={fallbackSrc}
        alt={alt}
        draggable={false}
        className={cn("select-none object-contain", className)}
        style={{ width: size, height: size, userSelect: "none" }}
      />
    );
  }

  if (!imageUrl) {
    return (
      <div
        className={cn("select-none bg-muted", className)}
        style={{ width: size, height: size, userSelect: "none" }}
        aria-hidden="true"
      />
    );
  }

  if (sprite.status !== "ready" || !useCanvasPlayback) {
    return (
      <img
        src={imageUrl}
        alt={alt}
        draggable={false}
        className={cn("select-none object-contain", className)}
        style={{ width: size, height: size, userSelect: "none" }}
      />
    );
  }

  return (
    <canvas
      ref={canvasRef}
      className={cn("select-none", className, motionClassName)}
      style={{ userSelect: "none" }}
      aria-label={alt}
    />
  );
}

function buildProcessedSpriteSheet(image: HTMLImageElement): HTMLCanvasElement | null {
  const canvas = document.createElement("canvas");
  canvas.width = image.naturalWidth;
  canvas.height = image.naturalHeight;
  const context = canvas.getContext("2d", { willReadFrequently: true });
  if (!context) {
    return null;
  }

  applyHighQualitySmoothing(context);
  context.clearRect(0, 0, canvas.width, canvas.height);
  context.drawImage(
    image,
    0,
    0,
    image.naturalWidth,
    image.naturalHeight,
    0,
    0,
    image.naturalWidth,
    image.naturalHeight,
  );
  try {
    const imageData = context.getImageData(0, 0, canvas.width, canvas.height);
    removeMagentaBackground(imageData.data, canvas.width, canvas.height);
    context.putImageData(imageData, 0, 0);
  } catch {
    // Keep animating with the raw sheet if the browser blocks pixel reads.
  }
  return canvas;
}

function applyHighQualitySmoothing(context: CanvasRenderingContext2D) {
  context.imageSmoothingEnabled = true;
  context.imageSmoothingQuality = "high";
}

function buildProcessedSpritePreviewFrame(
  image: HTMLImageElement,
  sprite: Sprite,
  rowIndex: number,
  frameIndex: number,
): HTMLCanvasElement | null {
  const totalFrames = Math.max(1, sprite.columns || 1);
  const totalRows = Math.max(1, sprite.rows || 1);
  const [sourceLeft, sourceRight, sourceTop, sourceBottom] = getFrameSliceBounds(
    image.naturalWidth,
    image.naturalHeight,
    totalFrames,
    totalRows,
    rowIndex,
    frameIndex,
  );
  const frameCanvas = document.createElement("canvas");
  frameCanvas.width = Math.max(1, sourceRight - sourceLeft);
  frameCanvas.height = Math.max(1, sourceBottom - sourceTop);
  const context = frameCanvas.getContext("2d", { willReadFrequently: true });
  if (!context) {
    return null;
  }

  applyHighQualitySmoothing(context);
  context.clearRect(0, 0, frameCanvas.width, frameCanvas.height);
  const drawTop = Math.min(sourceTop, image.naturalHeight);
  const drawHeight = Math.max(0, Math.min(image.naturalHeight, sourceBottom) - drawTop);
  if (drawHeight > 0) {
    context.drawImage(
      image,
      sourceLeft,
      drawTop,
      frameCanvas.width,
      drawHeight,
      0,
      0,
      frameCanvas.width,
      drawHeight,
    );
  }

  try {
    const imageData = context.getImageData(0, 0, frameCanvas.width, frameCanvas.height);
    removeMagentaBackground(imageData.data, frameCanvas.width, frameCanvas.height);
    context.putImageData(imageData, 0, 0);
  } catch {
    // Fall back to the raw frame if pixel reads are unavailable.
  }

  return frameCanvas;
}

function buildRowFrameCanvases(
  sheetCanvas: HTMLCanvasElement,
  rowIndex: number,
  totalFrames: number,
  totalRows: number,
): HTMLCanvasElement[] {
  const cacheKey = `${rowIndex}:${totalFrames}:${totalRows}`;
  const cachedRows = rowFrameCache.get(sheetCanvas);
  const cachedFrames = cachedRows?.get(cacheKey);
  if (cachedFrames) {
    return cachedFrames;
  }

  const sheetContext = sheetCanvas.getContext("2d", { willReadFrequently: true });
  const canReadPixels = Boolean(sheetContext);
  const [sourceTop, sourceBottom] = getEqualSliceBounds(sheetCanvas.height, totalRows, rowIndex);
  const sourceHeight = sourceBottom - sourceTop;

  const frames = Array.from({ length: totalFrames }, (_, frameIndex) => {
    const [sourceLeft, sourceRight] = getEqualSliceBounds(sheetCanvas.width, totalFrames, frameIndex);
    const sourceWidth = sourceRight - sourceLeft;
    const frameCanvas = document.createElement("canvas");
    frameCanvas.width = Math.max(1, sourceWidth);
    frameCanvas.height = Math.max(1, sourceHeight);

    const frameContext = frameCanvas.getContext("2d");
    if (!frameContext) {
      return frameCanvas;
    }

    if (canReadPixels && sheetContext) {
      try {
        const imageData = sheetContext.getImageData(
          sourceLeft,
          sourceTop,
          frameCanvas.width,
          frameCanvas.height,
        );
        frameContext.putImageData(imageData, 0, 0);
        return frameCanvas;
      } catch {
        // Fall back to drawImage when direct pixel reads are unavailable.
      }
    }

    frameContext.drawImage(
      sheetCanvas,
      sourceLeft,
      sourceTop,
      frameCanvas.width,
      frameCanvas.height,
      0,
      0,
      frameCanvas.width,
      frameCanvas.height,
    );
    return frameCanvas;
  });

  if (cachedRows) {
    cachedRows.set(cacheKey, frames);
  } else {
    rowFrameCache.set(sheetCanvas, new Map([[cacheKey, frames]]));
  }

  return frames;
}

function getEqualSliceBounds(totalSize: number, segments: number, index: number): [number, number] {
  const safeSegments = Math.max(1, segments);
  const safeIndex = Math.max(0, Math.min(safeSegments - 1, index));
  const start = Math.floor((totalSize * safeIndex) / safeSegments);
  const end = Math.floor((totalSize * (safeIndex + 1)) / safeSegments);
  return [start, Math.max(start + 1, end)];
}

function getFrameSliceBounds(
  totalWidth: number,
  totalHeight: number,
  totalFrames: number,
  totalRows: number,
  rowIndex: number,
  frameIndex: number,
) {
  const [sourceLeft, sourceRight] = getEqualSliceBounds(totalWidth, totalFrames, frameIndex);
  const [sourceTop, sourceBottom] = getEqualSliceBounds(totalHeight, totalRows, rowIndex);
  return [sourceLeft, sourceRight, sourceTop, sourceBottom] as const;
}

function loadProcessedSpriteSheet(imageUrl: string, sprite: Sprite) {
  const cacheKey = JSON.stringify([
    "sheet",
    MAGENTA_PROCESS_CACHE_VERSION,
    imageUrl,
    sprite.id,
    sprite.columns,
    sprite.rows,
    sprite.imageWidth,
    sprite.imageHeight,
  ]);
  const cached = getCachedPromise(processedSheetCache, cacheKey);
  if (cached) {
    return cached;
  }

  const next = loadSpriteImage(imageUrl).then((image) => {
    if (!image) {
      return null;
    }
    return buildProcessedSpriteSheet(image);
  });
  setCachedPromise(processedSheetCache, cacheKey, next, PROCESSED_SHEET_CACHE_LIMIT);
  return next;
}

function loadProcessedSpritePreviewFrame(
  imageUrl: string,
  sprite: Sprite,
  rowIndex: number,
  frameIndex: number,
) {
  const cacheKey = JSON.stringify([
    "frame",
    MAGENTA_PROCESS_CACHE_VERSION,
    imageUrl,
    sprite.id,
    sprite.columns,
    sprite.rows,
    sprite.imageWidth,
    sprite.imageHeight,
    rowIndex,
    frameIndex,
  ]);
  const cached = getCachedPromise(previewFrameCache, cacheKey);
  if (cached) {
    return cached;
  }

  const next = loadSpriteImage(imageUrl).then((image) => {
    if (!image) {
      return null;
    }
    return buildProcessedSpritePreviewFrame(image, sprite, rowIndex, frameIndex);
  });
  setCachedPromise(previewFrameCache, cacheKey, next, PREVIEW_FRAME_CACHE_LIMIT);
  return next;
}

function loadSpriteImage(imageUrl: string) {
  const cached = getCachedPromise(spriteImageCache, imageUrl);
  if (cached) {
    return cached;
  }

  const next = new Promise<HTMLImageElement | null>((resolve) => {
    const image = new Image();
    image.crossOrigin = "anonymous";
    image.decoding = "async";
    image.onload = () => {
      if (typeof image.decode !== "function") {
        resolve(image);
        return;
      }
      void image
        .decode()
        .then(() => resolve(image))
        .catch(() => resolve(image));
    };
    image.onerror = () => resolve(null);
    image.src = imageUrl;
  });
  setCachedPromise(spriteImageCache, imageUrl, next, SPRITE_IMAGE_CACHE_LIMIT);
  return next;
}

function getCachedPromise<T>(cache: Map<string, Promise<T>>, key: string) {
  const cached = cache.get(key);
  if (!cached) {
    return undefined;
  }
  cache.delete(key);
  cache.set(key, cached);
  return cached;
}

function setCachedPromise<T>(
  cache: Map<string, Promise<T>>,
  key: string,
  value: Promise<T>,
  limit: number,
) {
  cache.set(key, value);
  while (cache.size > limit) {
    const oldestKey = cache.keys().next().value;
    if (!oldestKey) {
      break;
    }
    cache.delete(oldestKey);
  }
}

function removeMagentaBackground(pixels: Uint8ClampedArray, width: number, height: number) {
  const background = sampleBackgroundColor(pixels, width, height);
  if (!background) {
    removeNearPureMagentaPixels(pixels);
    cleanupMagentaFringe(pixels, width, height, null);
    return;
  }

  const visited = new Uint8Array(width * height);
  const queue: number[] = [];
  const enqueue = (x: number, y: number) => {
    if (x < 0 || y < 0 || x >= width || y >= height) {
      return;
    }
    const index = y * width + x;
    if (visited[index]) {
      return;
    }
    const offset = index * 4;
    if (
      pixels[offset + 3] === 0 ||
      !matchesBackgroundColor(pixels[offset], pixels[offset + 1], pixels[offset + 2], background)
    ) {
      return;
    }
    visited[index] = 1;
    queue.push(index);
  };

  for (let x = 0; x < width; x += 1) {
    enqueue(x, 0);
    enqueue(x, height - 1);
  }
  for (let y = 0; y < height; y += 1) {
    enqueue(0, y);
    enqueue(width - 1, y);
  }

  for (let cursor = 0; cursor < queue.length; cursor += 1) {
    const index = queue[cursor];
    const offset = index * 4;
    pixels[offset + 3] = 0;

    const x = index % width;
    const y = (index - x) / width;
    enqueue(x - 1, y);
    enqueue(x + 1, y);
    enqueue(x, y - 1);
    enqueue(x, y + 1);
  }

  cleanupMagentaFringe(pixels, width, height, background);
}

function sampleBackgroundColor(
  pixels: Uint8ClampedArray,
  width: number,
  height: number,
): BackgroundColor | null {
  const samples: BackgroundColor[] = [];
  SAMPLE_POINT_FACTORS.forEach(([xFactor, yFactor]) => {
    const x = Math.max(0, Math.min(width - 1, Math.round((width - 1) * xFactor)));
    const y = Math.max(0, Math.min(height - 1, Math.round((height - 1) * yFactor)));
    const offset = (y * width + x) * 4;
    const alpha = pixels[offset + 3];
    if (alpha === 0) {
      return;
    }
    const r = pixels[offset];
    const g = pixels[offset + 1];
    const b = pixels[offset + 2];
    if (!isMagentaLike(r, g, b)) {
      return;
    }
    samples.push({ r, g, b });
  });

  if (samples.length === 0) {
    return null;
  }

  const totals = samples.reduce(
    (result, sample) => ({
      r: result.r + sample.r,
      g: result.g + sample.g,
      b: result.b + sample.b,
    }),
    { r: 0, g: 0, b: 0 },
  );

  return {
    r: Math.round(totals.r / samples.length),
    g: Math.round(totals.g / samples.length),
    b: Math.round(totals.b / samples.length),
  };
}

function matchesBackgroundColor(r: number, g: number, b: number, background: BackgroundColor) {
  if (!isMagentaLike(r, g, b)) {
    return false;
  }

  const totalDistance = Math.abs(r - background.r) + Math.abs(g - background.g) + Math.abs(b - background.b);
  return (
    Math.abs(r - background.r) <= MAGENTA_CHANNEL_TOLERANCE &&
    Math.abs(g - background.g) <= MAGENTA_CHANNEL_TOLERANCE &&
    Math.abs(b - background.b) <= MAGENTA_CHANNEL_TOLERANCE &&
    totalDistance <= MAGENTA_COLOR_DISTANCE
  );
}

function resolveBackgroundMatchStrength(r: number, g: number, b: number, background: BackgroundColor) {
  if (!isMagentaLike(r, g, b)) {
    return 0;
  }

  const channelDistance = Math.max(
    Math.abs(r - background.r),
    Math.abs(g - background.g),
    Math.abs(b - background.b),
  );
  const totalDistance = Math.abs(r - background.r) + Math.abs(g - background.g) + Math.abs(b - background.b);
  const channelScore = 1 - channelDistance / MAGENTA_CHANNEL_TOLERANCE;
  const totalScore = 1 - totalDistance / MAGENTA_COLOR_DISTANCE;
  return clampUnit(Math.min(channelScore, totalScore));
}

function isMagentaLike(r: number, g: number, b: number) {
  return (
    r >= 140 &&
    b >= 140 &&
    Math.min(r, b) - g >= MIN_MAGENTA_STRENGTH &&
    Math.abs(r - b) <= 120
  );
}

function removeNearPureMagentaPixels(pixels: Uint8ClampedArray) {
  for (let index = 0; index < pixels.length; index += 4) {
    if (pixels[index] >= 236 && pixels[index + 1] <= 24 && pixels[index + 2] >= 236) {
      pixels[index + 3] = 0;
    }
  }
}

function cleanupMagentaFringe(
  pixels: Uint8ClampedArray,
  width: number,
  height: number,
  background: BackgroundColor | null,
) {
  const alphaMask = new Uint8Array(width * height);
  for (let index = 0; index < alphaMask.length; index += 1) {
    alphaMask[index] = pixels[index * 4 + 3];
  }

  for (let y = 0; y < height; y += 1) {
    for (let x = 0; x < width; x += 1) {
      const offset = (y * width + x) * 4;
      const alpha = alphaMask[y * width + x];
      if (alpha === 0 || !touchesTransparentEdge(alphaMask, width, height, x, y)) {
        continue;
      }

      const r = pixels[offset];
      const g = pixels[offset + 1];
      const b = pixels[offset + 2];
      const spill = Math.max(0, Math.min(r, b) - g);
      const matchStrength = background
        ? resolveBackgroundMatchStrength(r, g, b, background)
        : resolvePureMagentaStrength(r, g, b);

      if (spill < MAGENTA_EDGE_SPILL_THRESHOLD && matchStrength <= 0) {
        continue;
      }

      const reduction = Math.round(
        spill * (MAGENTA_EDGE_CHANNEL_REDUCTION + matchStrength * 0.35),
      );
      pixels[offset] = Math.max(0, r - reduction);
      pixels[offset + 2] = Math.max(0, b - reduction);

      const normalizedSpill = clampUnit((spill - MAGENTA_EDGE_SPILL_THRESHOLD) / 96);
      const alphaScale = 1 - Math.max(
        normalizedSpill * MAGENTA_EDGE_ALPHA_SOFTEN,
        matchStrength * MAGENTA_EDGE_MATCH_ALPHA_SOFTEN,
      );
      let nextAlpha = Math.max(0, Math.round(alpha * clampUnit(alphaScale)));
      if (matchStrength >= MAGENTA_HARD_CLEAR_MATCH && spill >= MAGENTA_EDGE_SPILL_THRESHOLD) {
        nextAlpha = 0;
      }
      pixels[offset + 3] = nextAlpha;
    }
  }
}

function touchesTransparentEdge(
  alphaMask: Uint8Array,
  width: number,
  height: number,
  x: number,
  y: number,
) {
  for (let deltaY = -1; deltaY <= 1; deltaY += 1) {
    for (let deltaX = -1; deltaX <= 1; deltaX += 1) {
      if (deltaX === 0 && deltaY === 0) {
        continue;
      }
      const nextX = x + deltaX;
      const nextY = y + deltaY;
      if (nextX < 0 || nextY < 0 || nextX >= width || nextY >= height) {
        return true;
      }
      if (alphaMask[nextY * width + nextX] === 0) {
        return true;
      }
    }
  }
  return false;
}

function resolvePureMagentaStrength(r: number, g: number, b: number) {
  if (!isMagentaLike(r, g, b)) {
    return 0;
  }

  const distance = Math.abs(255 - r) + g + Math.abs(255 - b);
  return clampUnit(1 - distance / 180);
}

function clampUnit(value: number) {
  if (!Number.isFinite(value)) {
    return 0;
  }
  return Math.min(1, Math.max(0, value));
}
