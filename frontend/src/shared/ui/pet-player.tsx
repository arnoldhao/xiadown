import * as React from "react";

import type { Pet } from "@/shared/contracts/pets";
import {
  PET_ANIMATION_DEFINITIONS,
  type PetAnimation,
} from "@/shared/pets/animation";
import { cn } from "@/lib/utils";
import { PET_DISPLAY_GLOW_STYLE } from "@/shared/styles/xiadown";

const PET_PLAYBACK_SLOWDOWN = 2;
const imageCache = new Map<string, Promise<HTMLImageElement | null>>();
const IMAGE_CACHE_LIMIT = 48;

export function PetDisplay(props: {
  pet: Pet | null;
  imageUrl: string;
  animation?: PetAnimation;
  animate?: boolean;
  alt: string;
  className?: string;
  glowClassName?: string;
  glowStyle?: React.CSSProperties;
  petClassName?: string;
  fallbackSrc?: string;
  size?: number;
}) {
  const {
    pet,
    imageUrl,
    animation,
    animate,
    alt,
    className,
    glowClassName,
    glowStyle,
    petClassName,
    fallbackSrc,
    size,
  } = props;
  const petSize = size ?? 64;
  const containerStyle: React.CSSProperties = { userSelect: "none" };
  if (size !== undefined) {
    containerStyle.width = size;
    containerStyle.height = size;
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
        style={glowStyle ? { ...PET_DISPLAY_GLOW_STYLE, ...glowStyle } : PET_DISPLAY_GLOW_STYLE}
      />
      <PetPlayer
        pet={pet}
        imageUrl={imageUrl}
        animation={animation}
        animate={animate}
        size={petSize}
        alt={alt}
        fallbackSrc={fallbackSrc}
        className={cn("relative z-10 shrink-0 select-none", petClassName)}
      />
    </div>
  );
}

export function PetPlayer(props: {
  pet: Pet | null;
  imageUrl: string;
  animation?: PetAnimation;
  size?: number;
  animate?: boolean;
  alt: string;
  className?: string;
  fallbackSrc?: string;
}) {
  const {
    pet,
    imageUrl,
    animation = "idle",
    size = 112,
    animate = true,
    alt,
    className,
    fallbackSrc = "/appicon.png",
  } = props;
  const canvasRef = React.useRef<HTMLCanvasElement | null>(null);
  const [image, setImage] = React.useState<HTMLImageElement | null>(null);
  const [failed, setFailed] = React.useState(false);
  const isReady = Boolean(
    pet &&
      pet.status === "ready" &&
      pet.imageWidth > 0 &&
      pet.imageHeight > 0 &&
      pet.columns > 0 &&
      pet.rows > 0 &&
      pet.cellWidth > 0 &&
      pet.cellHeight > 0 &&
      imageUrl,
  );

  React.useEffect(() => {
    if (!isReady) {
      setImage(null);
      setFailed(false);
      return;
    }

    let active = true;
    void loadPetImage(imageUrl).then((nextImage) => {
      if (!active) {
        return;
      }
      setImage(nextImage);
      setFailed(!nextImage);
    });
    return () => {
      active = false;
    };
  }, [imageUrl, isReady]);

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
      context.imageSmoothingEnabled = false;
    };

    clear();
    if (!isReady || !pet || !image) {
      return;
    }

    const definition = PET_ANIMATION_DEFINITIONS[animation] ?? PET_ANIMATION_DEFINITIONS.idle;
    const totalFrames = Math.max(1, Math.min(definition.frames, pet.columns || 1));
    const row = Math.max(0, Math.min(definition.row, (pet.rows || 1) - 1));
    let frameIndex = 0;
    let rafHandle = 0;
    let timeoutHandle = 0;
    let active = true;

    const drawFrame = (nextFrameIndex: number) => {
      clear();
      const cellWidth = pet.cellWidth || Math.floor(pet.imageWidth / pet.columns);
      const cellHeight = pet.cellHeight || Math.floor(pet.imageHeight / pet.rows);
      const scale = Math.min(size / cellWidth, size / cellHeight);
      const targetWidth = cellWidth * scale;
      const targetHeight = cellHeight * scale;
      const targetX = (size - targetWidth) / 2;
      const targetY = (size - targetHeight) / 2;
      context.drawImage(
        image,
        nextFrameIndex * cellWidth,
        row * cellHeight,
        cellWidth,
        cellHeight,
        targetX,
        targetY,
        targetWidth,
        targetHeight,
      );
    };

    const tick = () => {
      if (!active) {
        return;
      }
      drawFrame(frameIndex);
      if (!animate) {
        return;
      }
      const duration =
        (definition.durations[frameIndex] ?? definition.durations[0] ?? 160) * PET_PLAYBACK_SLOWDOWN;
      frameIndex = (frameIndex + 1) % totalFrames;
      timeoutHandle = window.setTimeout(() => {
        rafHandle = window.requestAnimationFrame(tick);
      }, duration);
    };

    tick();
    return () => {
      active = false;
      if (timeoutHandle) {
        window.clearTimeout(timeoutHandle);
      }
      if (rafHandle) {
        window.cancelAnimationFrame(rafHandle);
      }
    };
  }, [animate, animation, image, isReady, pet, size]);

  if (failed || !isReady) {
    return (
      <img
        src={fallbackSrc}
        alt={alt}
        width={size}
        height={size}
        loading="lazy"
        decoding="async"
        draggable={false}
        className={cn("h-16 w-16 object-contain opacity-90", className)}
        style={{ width: size, height: size }}
      />
    );
  }

  return (
    <canvas
      ref={canvasRef}
      aria-label={alt}
      role="img"
      className={cn("h-16 w-16", className)}
      style={{ width: size, height: size }}
    />
  );
}

function loadPetImage(imageUrl: string): Promise<HTMLImageElement | null> {
  const key = imageUrl.trim();
  if (!key) {
    return Promise.resolve(null);
  }
  const cached = imageCache.get(key);
  if (cached) {
    return cached;
  }
  const promise = new Promise<HTMLImageElement | null>((resolve) => {
    const image = new Image();
    image.crossOrigin = "anonymous";
    image.onload = () => resolve(image);
    image.onerror = () => resolve(null);
    image.src = key;
  });
  imageCache.set(key, promise);
  trimCache(imageCache, IMAGE_CACHE_LIMIT);
  return promise;
}

function trimCache<K, V>(cache: Map<K, V>, limit: number) {
  while (cache.size > limit) {
    const first = cache.keys().next();
    if (first.done) {
      break;
    }
    cache.delete(first.value);
  }
}
