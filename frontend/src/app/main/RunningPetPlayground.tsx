import * as React from "react";

import type { Pet } from "@/shared/contracts/pets";
import type { PetAnimation } from "@/shared/pets/animation";
import { RUNNING_PET_GLOW_STYLE } from "@/shared/styles/xiadown";
import { PetDisplay } from "@/shared/ui/pet-player";

type PlaygroundToy = "none" | "ball";

interface RunningPetPlaygroundState {
  xRatio: number;
  yRatio: number;
  scale: number;
  rotation: number;
  toy: PlaygroundToy;
}

type DragState =
  | {
      mode: "move";
      pointerId: number;
      startClientX: number;
      startClientY: number;
      startState: RunningPetPlaygroundState;
    }
  | {
      mode: "rotate";
      pointerId: number;
      startClientX: number;
      startState: RunningPetPlaygroundState;
    };

const STORAGE_KEY = "xiadown.runningPetPlayground.v1";
const PET_BASE_SIZE = 192;
const DEFAULT_STATE: RunningPetPlaygroundState = {
  xRatio: 0.5,
  yRatio: 0.38,
  scale: 1,
  rotation: 0,
  toy: "none",
};
const MIN_SCALE = 0.65;
const MAX_SCALE = 1.7;
const WALK_SPEED_PX_PER_SECOND = 190;
const SPRINT_MULTIPLIER = 2.05;
const JUMP_ANIMATION_MS = 760;
const WAVE_ANIMATION_MS = 980;
const FAILED_ANIMATION_MS = 1_480;
const ROTATION_DEGREES_PER_PIXEL = 0.42;

const MOVEMENT_BY_CODE: Record<string, readonly [number, number]> = {
  KeyW: [0, -1],
  KeyS: [0, 1],
  KeyA: [-1, 0],
  KeyD: [1, 0],
  ArrowUp: [0, -1],
  ArrowDown: [0, 1],
  ArrowLeft: [-1, 0],
  ArrowRight: [1, 0],
};

const CONTROL_CODES = new Set([
  ...Object.keys(MOVEMENT_BY_CODE),
  "ShiftLeft",
  "ShiftRight",
  "Shift",
  "Space",
  "KeyE",
  "KeyQ",
  "KeyB",
  "KeyT",
  "KeyR",
]);

const KEY_CODE_FALLBACKS: Record<string, string> = {
  " ": "Space",
  arrowdown: "ArrowDown",
  arrowleft: "ArrowLeft",
  arrowright: "ArrowRight",
  arrowup: "ArrowUp",
  down: "ArrowDown",
  left: "ArrowLeft",
  q: "KeyQ",
  r: "KeyR",
  right: "ArrowRight",
  shift: "Shift",
  spacebar: "Space",
  t: "KeyT",
  up: "ArrowUp",
  w: "KeyW",
  a: "KeyA",
  s: "KeyS",
  d: "KeyD",
  e: "KeyE",
  b: "KeyB",
};

export function RunningPetPlayground(props: {
  pet: Pet | null;
  imageUrl: string;
  animation: PetAnimation;
  alt: string;
  hint: string;
  children: React.ReactNode;
}) {
  const stageRef = React.useRef<HTMLDivElement | null>(null);
  const stateRef = React.useRef<RunningPetPlaygroundState>(DEFAULT_STATE);
  const pressedKeysRef = React.useRef<Set<string>>(new Set());
  const dragStateRef = React.useRef<DragState | null>(null);
  const transientTimeoutRef = React.useRef<number | null>(null);
  const movementAnimationRef = React.useRef<PetAnimation | null>(null);
  const [state, setState] = React.useState<RunningPetPlaygroundState>(() =>
    readStoredState(),
  );
  const [dragging, setDragging] = React.useState(false);
  const [movingAnimation, setMovingAnimation] =
    React.useState<PetAnimation | null>(null);
  const [transientAnimation, setTransientAnimation] =
    React.useState<PetAnimation | null>(null);
  const [actionAnimation, setActionAnimation] =
    React.useState<PetAnimation | null>(null);
  const [idleOverride, setIdleOverride] = React.useState(false);

  const updateMovementAnimation = React.useCallback(
    (nextAnimation: PetAnimation | null) => {
      if (movementAnimationRef.current === nextAnimation) {
        return;
      }
      movementAnimationRef.current = nextAnimation;
      setMovingAnimation(nextAnimation);
    },
    [],
  );

  const clearTransientAnimation = React.useCallback(() => {
    if (transientTimeoutRef.current !== null) {
      window.clearTimeout(transientTimeoutRef.current);
      transientTimeoutRef.current = null;
    }
    setTransientAnimation(null);
    setActionAnimation(null);
  }, []);

  const triggerTransientAnimation = React.useCallback(
    (animation: PetAnimation, durationMs: number) => {
      clearTransientAnimation();
      setIdleOverride(false);
      setTransientAnimation(animation);
      setActionAnimation(animation);
      transientTimeoutRef.current = window.setTimeout(() => {
        transientTimeoutRef.current = null;
        setTransientAnimation(null);
        setActionAnimation(null);
      }, durationMs);
    },
    [clearTransientAnimation],
  );

  const toggleToy = React.useCallback((toy: Exclude<PlaygroundToy, "none">) => {
    setState((current) => ({
      ...current,
      toy: current.toy === toy ? "none" : toy,
    }));
  }, []);

  const resetPlayground = React.useCallback(() => {
    pressedKeysRef.current.clear();
    clearTransientAnimation();
    updateMovementAnimation(null);
    setIdleOverride(false);
    setState(DEFAULT_STATE);
  }, [clearTransientAnimation, updateMovementAnimation]);

  React.useEffect(() => {
    stateRef.current = state;
  }, [state]);

  React.useEffect(() => {
    const timeoutId = window.setTimeout(() => writeStoredState(state), 140);
    return () => window.clearTimeout(timeoutId);
  }, [state]);

  React.useEffect(() => {
    return () => {
      writeStoredState(stateRef.current);
    };
  }, []);

  React.useEffect(() => {
    stageRef.current?.focus({ preventScroll: true });
  }, []);

  React.useEffect(() => {
    let frameId = 0;
    let previousTime = 0;

    const tick = (time: number) => {
      frameId = window.requestAnimationFrame(tick);
      const deltaSeconds = previousTime
        ? Math.min((time - previousTime) / 1000, 0.05)
        : 0;
      previousTime = time;
      if (deltaSeconds <= 0) {
        return;
      }

      const pressedKeys = pressedKeysRef.current;
      let dx = 0;
      let dy = 0;
      Object.entries(MOVEMENT_BY_CODE).forEach(([code, vector]) => {
        if (!pressedKeys.has(code)) {
          return;
        }
        dx += vector[0];
        dy += vector[1];
      });

      if (dx === 0 && dy === 0) {
        updateMovementAnimation(null);
        return;
      }

      setIdleOverride(false);
      clearTransientAnimation();

      const length = Math.hypot(dx, dy) || 1;
      dx /= length;
      dy /= length;

      const rect = stageRef.current?.getBoundingClientRect();
      if (!rect || rect.width <= 0 || rect.height <= 0) {
        return;
      }
      const sprinting =
        pressedKeys.has("ShiftLeft") ||
        pressedKeys.has("ShiftRight") ||
        pressedKeys.has("Shift");
      const speed =
        WALK_SPEED_PX_PER_SECOND * (sprinting ? SPRINT_MULTIPLIER : 1);
      const nextAnimation =
        dx > 0 ? "running-right" : dx < 0 ? "running-left" : "running";
      updateMovementAnimation(nextAnimation);

      setState((current) =>
        clampState(
          {
            ...current,
            xRatio: current.xRatio + (dx * speed * deltaSeconds) / rect.width,
            yRatio: current.yRatio + (dy * speed * deltaSeconds) / rect.height,
          },
          rect.width,
          rect.height,
        ),
      );
    };

    frameId = window.requestAnimationFrame(tick);
    return () => window.cancelAnimationFrame(frameId);
  }, [clearTransientAnimation, updateMovementAnimation]);

  const endDrag = React.useCallback(() => {
    dragStateRef.current = null;
    setDragging(false);
    updateMovementAnimation(null);
    window.removeEventListener("pointermove", handleWindowPointerMove);
    window.removeEventListener("pointerup", endDrag);
    window.removeEventListener("pointercancel", endDrag);
  }, [updateMovementAnimation]);

  const handleWindowPointerMove = React.useCallback(
    (event: PointerEvent) => {
      const dragState = dragStateRef.current;
      const rect = stageRef.current?.getBoundingClientRect();
      if (!dragState || !rect || rect.width <= 0 || rect.height <= 0) {
        return;
      }
      event.preventDefault();
      if (dragState.mode === "rotate") {
        setState((current) => ({
          ...current,
          rotation: normalizeRotation(
            dragState.startState.rotation +
              (event.clientX - dragState.startClientX) *
                ROTATION_DEGREES_PER_PIXEL,
          ),
        }));
        return;
      }

      setIdleOverride(false);
      const dx = event.clientX - dragState.startClientX;
      const dy = event.clientY - dragState.startClientY;
      updateMovementAnimation(
        Math.abs(dx) > Math.abs(dy)
          ? dx >= 0
            ? "running-right"
            : "running-left"
          : "running",
      );
      setState((current) =>
        clampState(
          {
            ...current,
            xRatio: dragState.startState.xRatio + dx / rect.width,
            yRatio: dragState.startState.yRatio + dy / rect.height,
          },
          rect.width,
          rect.height,
        ),
      );
    },
    [updateMovementAnimation],
  );

  const beginDrag = React.useCallback(
    (nextDragState: DragState) => {
      dragStateRef.current = nextDragState;
      setDragging(true);
      stageRef.current?.focus({ preventScroll: true });
      window.addEventListener("pointermove", handleWindowPointerMove, {
        passive: false,
      });
      window.addEventListener("pointerup", endDrag);
      window.addEventListener("pointercancel", endDrag);
    },
    [endDrag, handleWindowPointerMove],
  );

  React.useEffect(() => {
    return () => {
      if (transientTimeoutRef.current !== null) {
        window.clearTimeout(transientTimeoutRef.current);
      }
      endDrag();
    };
  }, [endDrag]);

  React.useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      const code = normalizeKeyboardCode(event);
      if (!CONTROL_CODES.has(code) || isEditableTarget(event.target)) {
        return;
      }
      if (event.metaKey || event.ctrlKey || event.altKey) {
        return;
      }
      event.preventDefault();
      if (code in MOVEMENT_BY_CODE || code.startsWith("Shift")) {
        pressedKeysRef.current.add(code);
        return;
      }
      if (event.repeat) {
        return;
      }
      switch (code) {
        case "Space":
          triggerTransientAnimation("jumping", JUMP_ANIMATION_MS);
          break;
        case "KeyE":
          triggerTransientAnimation("waving", WAVE_ANIMATION_MS);
          break;
        case "KeyQ":
          clearTransientAnimation();
          setIdleOverride((current) => !current);
          break;
        case "KeyB":
          toggleToy("ball");
          break;
        case "KeyT":
          triggerTransientAnimation("failed", FAILED_ANIMATION_MS);
          break;
        case "KeyR":
          resetPlayground();
          break;
        default:
          break;
      }
    };

    const handleKeyUp = (event: KeyboardEvent) => {
      pressedKeysRef.current.delete(normalizeKeyboardCode(event));
    };

    const clearPressedKeys = () => pressedKeysRef.current.clear();

    window.addEventListener("keydown", handleKeyDown);
    window.addEventListener("keyup", handleKeyUp);
    window.addEventListener("blur", clearPressedKeys);
    return () => {
      window.removeEventListener("keydown", handleKeyDown);
      window.removeEventListener("keyup", handleKeyUp);
      window.removeEventListener("blur", clearPressedKeys);
      pressedKeysRef.current.clear();
    };
  }, [
    clearTransientAnimation,
    resetPlayground,
    toggleToy,
    triggerTransientAnimation,
  ]);

  const handlePetPointerDown = React.useCallback(
    (event: React.PointerEvent<HTMLDivElement>) => {
      if (event.button !== 0 && event.button !== 2) {
        return;
      }
      event.preventDefault();
      event.stopPropagation();
      if (event.button === 2) {
        beginDrag({
          mode: "rotate",
          pointerId: event.pointerId,
          startClientX: event.clientX,
          startState: stateRef.current,
        });
        return;
      }
      beginDrag({
        mode: "move",
        pointerId: event.pointerId,
        startClientX: event.clientX,
        startClientY: event.clientY,
        startState: stateRef.current,
      });
    },
    [beginDrag],
  );

  const handleStagePointerDown = React.useCallback(
    (event: React.PointerEvent<HTMLDivElement>) => {
      if (event.button !== 0 || event.target !== event.currentTarget) {
        return;
      }
      event.preventDefault();
      beginDrag({
        mode: "rotate",
        pointerId: event.pointerId,
        startClientX: event.clientX,
        startState: stateRef.current,
      });
    },
    [beginDrag],
  );

  const handleWheel = React.useCallback(
    (event: React.WheelEvent<HTMLDivElement>) => {
      if (event.ctrlKey || event.metaKey) {
        return;
      }
      event.preventDefault();
      const rect = stageRef.current?.getBoundingClientRect();
      setState((current) =>
        clampState(
          {
            ...current,
            scale: clamp(current.scale - event.deltaY * 0.0012, MIN_SCALE, MAX_SCALE),
          },
          rect?.width,
          rect?.height,
        ),
      );
    },
    [],
  );

  const resolvedAnimation =
    transientAnimation ?? movingAnimation ?? (idleOverride ? "idle" : props.animation);

  const petStyle: React.CSSProperties = {
    left: `${state.xRatio * 100}%`,
    top: `${state.yRatio * 100}%`,
    transform: `translate(-50%, -50%) scale(${state.scale})`,
  };
  const petCoreStyle = {
    "--running-pet-rotation": `${state.rotation}deg`,
  } as React.CSSProperties;
  const showHint = hasLeftDefaultState(state);

  return (
    <div
      ref={stageRef}
      className="app-running-pet-playground wails-no-drag relative h-full min-h-0 w-full overflow-hidden outline-none"
      tabIndex={0}
      onPointerDown={handleStagePointerDown}
      onWheel={handleWheel}
      aria-label={props.alt}
    >
      <div className="app-running-playground-action pointer-events-none absolute left-1/2 top-[67%] z-20 -translate-x-1/2 -translate-y-1/2">
        <div className="pointer-events-auto">{props.children}</div>
      </div>

      <div
        className="app-running-playground-hint pointer-events-none absolute bottom-5 left-1/2 z-40 max-w-[min(42rem,calc(100%-2rem))] -translate-x-1/2"
        data-visible={showHint ? "true" : undefined}
        role="status"
        aria-hidden={!showHint}
      >
        {props.hint}
      </div>

      <div
        className="app-running-playground-pet absolute z-30"
        data-dragging={dragging ? "true" : undefined}
        data-toy={state.toy !== "none" ? state.toy : undefined}
        style={petStyle}
        onPointerDown={handlePetPointerDown}
        onContextMenu={(event) => event.preventDefault()}
      >
        <span className="app-running-playground-shadow" aria-hidden="true" />
        {state.toy === "ball" ? (
          <span className="app-running-playground-ball" aria-hidden="true" />
        ) : null}
        <div
          className="app-running-playground-pet-core"
          data-action={actionAnimation ?? undefined}
          style={petCoreStyle}
        >
          <PetDisplay
            pet={props.pet}
            imageUrl={props.imageUrl}
            animation={resolvedAnimation}
            alt={props.alt}
            className="app-running-playground-pet-display"
            glowClassName="h-[18rem] w-[18rem] blur-2xl"
            glowStyle={RUNNING_PET_GLOW_STYLE}
            size={PET_BASE_SIZE}
          />
        </div>
      </div>
    </div>
  );
}

function readStoredState(): RunningPetPlaygroundState {
  if (typeof window === "undefined") {
    return DEFAULT_STATE;
  }
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) {
      return DEFAULT_STATE;
    }
    const parsed = JSON.parse(raw) as Partial<RunningPetPlaygroundState>;
    return {
      xRatio: clampNumber(parsed.xRatio, DEFAULT_STATE.xRatio, 0, 1),
      yRatio: clampNumber(parsed.yRatio, DEFAULT_STATE.yRatio, 0, 1),
      scale: clampNumber(parsed.scale, DEFAULT_STATE.scale, MIN_SCALE, MAX_SCALE),
      rotation: normalizeRotation(
        clampNumber(parsed.rotation, DEFAULT_STATE.rotation, -180, 180),
      ),
      toy: normalizeToy(parsed.toy),
    };
  } catch {
    return DEFAULT_STATE;
  }
}

function writeStoredState(state: RunningPetPlaygroundState) {
  if (typeof window === "undefined") {
    return;
  }
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
  } catch {
    // Front-end persistence is best-effort.
  }
}

function clampState(
  state: RunningPetPlaygroundState,
  width?: number,
  height?: number,
): RunningPetPlaygroundState {
  const scale = clamp(state.scale, MIN_SCALE, MAX_SCALE);
  const halfX =
    width && width > 0 ? Math.min(0.42, (PET_BASE_SIZE * scale * 0.45) / width) : 0;
  const halfY =
    height && height > 0 ? Math.min(0.42, (PET_BASE_SIZE * scale * 0.45) / height) : 0;
  return {
    xRatio: clamp(state.xRatio, halfX, 1 - halfX),
    yRatio: clamp(state.yRatio, halfY, 1 - halfY),
    scale,
    rotation: normalizeRotation(state.rotation),
    toy: normalizeToy(state.toy),
  };
}

function normalizeToy(value: unknown): PlaygroundToy {
  return value === "ball" ? value : "none";
}

function hasLeftDefaultState(state: RunningPetPlaygroundState) {
  return (
    Math.abs(state.xRatio - DEFAULT_STATE.xRatio) > 0.015 ||
    Math.abs(state.yRatio - DEFAULT_STATE.yRatio) > 0.015 ||
    Math.abs(state.scale - DEFAULT_STATE.scale) > 0.02 ||
    Math.abs(normalizeRotation(state.rotation - DEFAULT_STATE.rotation)) > 1 ||
    state.toy !== DEFAULT_STATE.toy
  );
}

function normalizeRotation(value: number) {
  if (!Number.isFinite(value)) {
    return DEFAULT_STATE.rotation;
  }
  let next = value % 360;
  if (next > 180) {
    next -= 360;
  }
  if (next < -180) {
    next += 360;
  }
  return next;
}

function clampNumber(
  value: unknown,
  fallback: number,
  minimum: number,
  maximum: number,
) {
  const numeric = typeof value === "number" ? value : Number(value);
  return Number.isFinite(numeric) ? clamp(numeric, minimum, maximum) : fallback;
}

function clamp(value: number, minimum: number, maximum: number) {
  return Math.max(minimum, Math.min(maximum, value));
}

function normalizeKeyboardCode(event: KeyboardEvent) {
  const code = event.code?.trim() ?? "";
  if (code === "Space" || event.key === " ") {
    return "Space";
  }
  if (event.key === "Shift") {
    return code || "Shift";
  }
  if (code && CONTROL_CODES.has(code)) {
    return code;
  }
  const key = event.key?.trim().toLowerCase() ?? "";
  return KEY_CODE_FALLBACKS[key] ?? (code || event.key);
}

function isEditableTarget(target: EventTarget | null) {
  if (!(target instanceof HTMLElement)) {
    return false;
  }
  const tagName = target.tagName.toLowerCase();
  return (
    target.isContentEditable ||
    tagName === "input" ||
    tagName === "textarea" ||
    tagName === "select" ||
    tagName === "button"
  );
}
