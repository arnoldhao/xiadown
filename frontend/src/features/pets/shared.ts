import * as React from "react";

import type { OperationListItemDTO } from "@/shared/contracts/library";
import type { Settings } from "@/shared/contracts/settings";
import type { Pet } from "@/shared/contracts/pets";
import type { PetAnimation } from "@/shared/pets/animation";
export {
  resolvePetCardLighting,
  type PetCardLighting,
} from "@/shared/styles/xiadown";

export interface PetPreferences {
  activePetId: string;
}

const PET_EVENT_CYCLE_MS = 1_280;
const PET_IDLE_TO_WAITING_DELAY_MS = 3_200;

const defaultPetPreferences: PetPreferences = {
  activePetId: "",
};

type PetOutcome = "success" | "failure";
type PetTimerKind = "ambient" | "event";

export type { PetAnimation } from "@/shared/pets/animation";

export function readPetPreferences(settings?: Settings | null): PetPreferences {
  const root = readAppConfig(settings);
  const pet = root.pet;
  if (!pet || typeof pet !== "object" || Array.isArray(pet)) {
    return defaultPetPreferences;
  }
  const record = pet as Record<string, unknown>;
  return {
    activePetId: typeof record.activePetId === "string" ? record.activePetId.trim() : "",
  };
}

export function mergePetPreferences(
  settings: Settings | null | undefined,
  patch: Partial<PetPreferences>,
): Record<string, unknown> {
  const current = readPetPreferences(settings);
  return {
    ...readAppConfig(settings),
    pet: {
      ...current,
      ...patch,
    },
  };
}

export function resolveActivePet(pets: Pet[], settings?: Settings | null): Pet | null {
  const currentID = readPetPreferences(settings).activePetId;
  const readyPets = pets.filter((pet) => pet.status === "ready");
  if (currentID) {
    const matched = readyPets.find((pet) => pet.id === currentID);
    if (matched) {
      return matched;
    }
  }
  return readyPets[0] ?? null;
}

export function useRunningPetAnimation(
  runningOperations: OperationListItemDTO[],
  terminalOperations: OperationListItemDTO[],
  terminalSnapshotReady = true,
): PetAnimation {
  const runningCount = runningOperations.length;
  const runningCountRef = React.useRef(runningCount);
  const queueRef = React.useRef<PetOutcome[]>([]);
  const timeoutRef = React.useRef<number | null>(null);
  const timerKindRef = React.useRef<PetTimerKind | null>(null);
  const seenTerminalIDsRef = React.useRef<Set<string> | null>(null);
  const hasSeededTerminalSnapshotRef = React.useRef(false);
  const processQueueRef = React.useRef<() => void>(() => {});
  const [animation, setAnimation] = React.useState<PetAnimation>(runningCount > 0 ? "running" : "idle");

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
      setAnimation("running");
      return;
    }

    setAnimation("idle");
    timerKindRef.current = "ambient";
    timeoutRef.current = window.setTimeout(() => {
      timeoutRef.current = null;
      timerKindRef.current = null;

      if (queueRef.current.length > 0) {
        processQueueRef.current();
        return;
      }
      if (runningCountRef.current > 0) {
        setAnimation("running");
        return;
      }

      setAnimation("waiting");
    }, PET_IDLE_TO_WAITING_DELAY_MS);
  }, [clearTimer]);

  const processQueue = React.useCallback(() => {
    clearTimer();

    const nextOutcome = queueRef.current.shift();
    if (!nextOutcome) {
      scheduleAmbientMood();
      return;
    }

    setAnimation(nextOutcome === "success" ? "jumping" : "failed");
    timerKindRef.current = "event";
    timeoutRef.current = window.setTimeout(() => {
      timeoutRef.current = null;
      timerKindRef.current = null;
      if (runningCountRef.current > 0) {
        if (queueRef.current.length > 0) {
          processQueueRef.current();
          return;
        }
        setAnimation("running");
        return;
      }

      setAnimation("idle");
      timerKindRef.current = "event";
      timeoutRef.current = window.setTimeout(() => {
        timeoutRef.current = null;
        timerKindRef.current = null;
        if (queueRef.current.length > 0) {
          processQueueRef.current();
          return;
        }
        scheduleAmbientMood();
      }, PET_EVENT_CYCLE_MS);
    }, PET_EVENT_CYCLE_MS);
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
        setAnimation("running");
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
    const outcomes: PetOutcome[] = [];
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
