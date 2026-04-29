import * as React from "react";

import type { OperationListItemDTO } from "@/shared/contracts/library";
import type { Settings } from "@/shared/contracts/settings";
import type { Sprite } from "@/shared/contracts/sprites";
import type { SpriteAnimation } from "@/shared/sprites/animation";
export {
  resolveSpriteCardLighting,
  type SpriteCardLighting,
} from "@/shared/styles/xiadown";

export interface SpritePreferences {
  activeSpriteId: string;
}

const SPRITE_EVENT_CYCLE_MS = 1_280;
const SPRITE_IDLE_TO_SLEEPY_DELAY_MS = 3_200;

const defaultSpritePreferences: SpritePreferences = {
  activeSpriteId: "",
};

type SpriteOutcome = "success" | "failure";
type SpriteTimerKind = "ambient" | "event";

export type { SpriteAnimation } from "@/shared/sprites/animation";

export function readSpritePreferences(settings?: Settings | null): SpritePreferences {
  const root = readAppConfig(settings);
  const sprite = root.sprite;
  if (!sprite || typeof sprite !== "object" || Array.isArray(sprite)) {
    return defaultSpritePreferences;
  }
  const record = sprite as Record<string, unknown>;
  return {
    activeSpriteId: typeof record.activeSpriteId === "string" ? record.activeSpriteId.trim() : "",
  };
}

export function mergeSpritePreferences(
  settings: Settings | null | undefined,
  patch: Partial<SpritePreferences>,
): Record<string, unknown> {
  const current = readSpritePreferences(settings);
  return {
    ...readAppConfig(settings),
    sprite: {
      ...current,
      ...patch,
    },
  };
}

export function resolveActiveSprite(sprites: Sprite[], settings?: Settings | null): Sprite | null {
  const currentID = readSpritePreferences(settings).activeSpriteId;
  const readySprites = sprites.filter((sprite) => sprite.status === "ready");
  if (currentID) {
    const matched = readySprites.find((sprite) => sprite.id === currentID);
    if (matched) {
      return matched;
    }
  }
  return readySprites[0] ?? null;
}

export function useRunningSpriteAnimation(
  runningOperations: OperationListItemDTO[],
  terminalOperations: OperationListItemDTO[],
  terminalSnapshotReady = true,
): SpriteAnimation {
  const runningCount = runningOperations.length;
  const runningCountRef = React.useRef(runningCount);
  const queueRef = React.useRef<SpriteOutcome[]>([]);
  const timeoutRef = React.useRef<number | null>(null);
  const timerKindRef = React.useRef<SpriteTimerKind | null>(null);
  const seenTerminalIDsRef = React.useRef<Set<string> | null>(null);
  const hasSeededTerminalSnapshotRef = React.useRef(false);
  const processQueueRef = React.useRef<() => void>(() => {});
  const [animation, setAnimation] = React.useState<SpriteAnimation>(runningCount > 0 ? "working" : "greeting");

  const clearTimer = React.useCallback(() => {
    if (timeoutRef.current !== null) {
      window.clearTimeout(timeoutRef.current);
      timeoutRef.current = null;
    }
    timerKindRef.current = null;
  }, []);

  const scheduleAmbientMood = React.useCallback(() => {
    clearTimer();

    if (runningCountRef.current > 0) {
      setAnimation("working");
      return;
    }

    setAnimation("greeting");
    timerKindRef.current = "ambient";
    timeoutRef.current = window.setTimeout(() => {
      timeoutRef.current = null;
      timerKindRef.current = null;

      if (queueRef.current.length > 0) {
        processQueueRef.current();
        return;
      }
      if (runningCountRef.current > 0) {
        setAnimation("working");
        return;
      }

      setAnimation("snoring");
    }, SPRITE_IDLE_TO_SLEEPY_DELAY_MS);
  }, [clearTimer]);

  const processQueue = React.useCallback(() => {
    clearTimer();

    const nextOutcome = queueRef.current.shift();
    if (!nextOutcome) {
      scheduleAmbientMood();
      return;
    }

    setAnimation(nextOutcome === "success" ? "celebrate" : "upset");
    timerKindRef.current = "event";
    timeoutRef.current = window.setTimeout(() => {
      timeoutRef.current = null;
      timerKindRef.current = null;
      if (runningCountRef.current > 0) {
        if (queueRef.current.length > 0) {
          processQueueRef.current();
          return;
        }
        setAnimation("working");
        return;
      }

      setAnimation("greeting");
      timerKindRef.current = "event";
      timeoutRef.current = window.setTimeout(() => {
        timeoutRef.current = null;
        timerKindRef.current = null;
        if (queueRef.current.length > 0) {
          processQueueRef.current();
          return;
        }
        scheduleAmbientMood();
      }, SPRITE_EVENT_CYCLE_MS);
    }, SPRITE_EVENT_CYCLE_MS);
  }, [clearTimer, scheduleAmbientMood]);

  React.useEffect(() => {
    processQueueRef.current = processQueue;
  }, [processQueue]);

  React.useEffect(() => {
    runningCountRef.current = runningCount;

    if (runningCount > 0) {
      if (timerKindRef.current === "ambient") {
        clearTimer();
      }
      if (queueRef.current.length === 0 && timeoutRef.current === null) {
        setAnimation("working");
      }
      return;
    }

    if (queueRef.current.length === 0 && timeoutRef.current === null) {
      scheduleAmbientMood();
    }
  }, [clearTimer, runningCount, scheduleAmbientMood]);

  React.useEffect(() => {
    if (!terminalSnapshotReady) {
      return;
    }

    const ordered = [...terminalOperations].sort((left, right) => resolveTerminalTime(left) - resolveTerminalTime(right));
    if (!hasSeededTerminalSnapshotRef.current) {
      seenTerminalIDsRef.current = new Set(ordered.map((operation) => operation.operationId));
      hasSeededTerminalSnapshotRef.current = true;
      return;
    }

    const seen = seenTerminalIDsRef.current ?? new Set<string>();
    seenTerminalIDsRef.current = seen;
    const outcomes: SpriteOutcome[] = [];
    ordered.forEach((operation) => {
      if (seen.has(operation.operationId)) {
        return;
      }
      seen.add(operation.operationId);
      if (operation.status === "succeeded") {
        outcomes.push("success");
      } else if (operation.status === "failed") {
        outcomes.push("failure");
      }
    });

    if (outcomes.length === 0) {
      return;
    }

    queueRef.current.push(...outcomes);
    if (timeoutRef.current === null || timerKindRef.current === "ambient") {
      processQueueRef.current();
    }
  }, [terminalOperations, terminalSnapshotReady]);

  React.useEffect(
    () => () => {
      clearTimer();
    },
    [clearTimer],
  );

  return animation;
}

function readAppConfig(settings?: Settings | null): Record<string, unknown> {
  const raw =
    settings?.appearanceConfig && typeof settings.appearanceConfig === "object" && settings.appearanceConfig !== null
      ? settings.appearanceConfig
      : {};
  return Array.isArray(raw) ? {} : { ...(raw as Record<string, unknown>) };
}

function resolveTerminalTime(operation: OperationListItemDTO): number {
  const raw = operation.finishedAt || operation.progress?.updatedAt || operation.createdAt;
  const parsed = raw ? Date.parse(raw) : Number.NaN;
  return Number.isFinite(parsed) ? parsed : 0;
}
