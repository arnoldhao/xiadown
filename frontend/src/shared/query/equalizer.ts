import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Call, Events } from "@wailsio/runtime";
import * as React from "react";

import {
  type EqualizerSnapshot,
  type EqualizerVisualizerFrame,
  type EqualizerVisualizerMode,
} from "@/shared/contracts/equalizer";
import {
  emptyEqualizerVisualizerFrame,
  normalizeEqualizerSnapshot,
  normalizeEqualizerVisualizerFrame,
  resolveEqualizerVisualizerPollDelay,
  shouldCommitEqualizerVisualizerFrame,
} from "@/shared/query/equalizer-normalize";

export const EQUALIZER_QUERY_KEY = ["equalizer"];
const EQUALIZER_SNAPSHOT_EVENT = "equalizer:snapshot";

const SERVICE = "xiadown/internal/presentation/wails.EqualizerHandler";

export function useEqualizerSnapshot(enabled = true) {
  const queryClient = useQueryClient();
  React.useEffect(() => {
    if (!enabled || !isWailsRuntimeReady()) {
      return;
    }
    const off = Events.On(EQUALIZER_SNAPSHOT_EVENT, (event: unknown) => {
      const payload = (event as { data?: unknown })?.data ?? event;
      queryClient.setQueryData(EQUALIZER_QUERY_KEY, normalizeEqualizerSnapshot(payload));
    });
    return () => off();
  }, [enabled, queryClient]);

  return useQuery({
    queryKey: EQUALIZER_QUERY_KEY,
    queryFn: fetchEqualizerSnapshot,
    enabled,
    refetchInterval: enabled ? 1_000 : false,
  });
}

export function useSetEqualizerEnabled() {
  return useEqualizerMutation((enabled: boolean) => callEqualizer("SetEnabled", enabled));
}

export function useApplyEqualizerPreset() {
  return useEqualizerMutation((presetID: string) => callEqualizer("ApplyPreset", presetID));
}

export function useSetEqualizerBandGain() {
  return useEqualizerMutation((input: { index: number; gainDb: number }) =>
    callEqualizer("SetBandGain", input.index, input.gainDb),
  );
}

export function useSetEqualizerPreamp() {
  return useEqualizerMutation((gainDb: number) => callEqualizer("SetPreamp", gainDb));
}

export function useSetEqualizerVisualizerMode() {
  return useEqualizerMutation((mode: EqualizerVisualizerMode) => callEqualizer("SetVisualizerMode", mode));
}

export function useResetEqualizer() {
  return useEqualizerMutation(() => callEqualizer("Reset"));
}

export function useRetryEqualizer() {
  return useEqualizerMutation(() => callEqualizer("Retry"));
}

export function useOpenEqualizerPermissionGuide() {
  return useMutation({
    mutationFn: (input: { permissionName: string; hint: string }) =>
      callEqualizer("OpenPermissionGuide", input.permissionName, input.hint),
  });
}

async function fetchEqualizerSnapshot(): Promise<EqualizerSnapshot> {
  return normalizeEqualizerSnapshot(await callEqualizer("Snapshot"));
}

export function useEqualizerVisualizerFrame(enabled = true) {
  const [frame, setFrame] = React.useState<EqualizerVisualizerFrame>(() => emptyEqualizerVisualizerFrame());

  React.useEffect(() => {
    if (!enabled) {
      setFrame(emptyEqualizerVisualizerFrame());
      return;
    }
    let disposed = false;
    let timer = 0;
    let inFlight = false;
    let lastRunning = false;
    let nextPollDelayMs = resolveEqualizerVisualizerPollDelay(false, isDocumentHidden());
    let poll = () => {};
    const schedulePoll = (delayMs: number) => {
      window.clearTimeout(timer);
      timer = window.setTimeout(poll, delayMs);
    };
    poll = () => {
      if (disposed) {
        return;
      }
      if (inFlight) {
        schedulePoll(nextPollDelayMs);
        return;
      }
      inFlight = true;
      void fetchEqualizerVisualizerFrame()
        .then((nextFrame) => {
          if (!disposed) {
            setFrame((currentFrame) =>
              shouldCommitEqualizerVisualizerFrame(currentFrame, nextFrame) ? nextFrame : currentFrame,
            );
            lastRunning = nextFrame.running;
            nextPollDelayMs = resolveEqualizerVisualizerPollDelay(nextFrame.running, isDocumentHidden());
          }
        })
        .catch(() => {
          if (!disposed) {
            setFrame(emptyEqualizerVisualizerFrame());
            lastRunning = false;
            nextPollDelayMs = resolveEqualizerVisualizerPollDelay(false, isDocumentHidden());
          }
        })
        .finally(() => {
          inFlight = false;
          if (!disposed) {
            schedulePoll(nextPollDelayMs);
          }
        });
    };
    const handleVisibilityChange = () => {
      nextPollDelayMs = resolveEqualizerVisualizerPollDelay(lastRunning, isDocumentHidden());
      if (!isDocumentHidden()) {
        schedulePoll(0);
      }
    };
    document.addEventListener("visibilitychange", handleVisibilityChange);
    poll();
    return () => {
      disposed = true;
      window.clearTimeout(timer);
      document.removeEventListener("visibilitychange", handleVisibilityChange);
    };
  }, [enabled]);

  return frame;
}

async function fetchEqualizerVisualizerFrame(): Promise<EqualizerVisualizerFrame> {
  const requestAtMs = nowMs();
  const rawFrame = await callEqualizer("VisualizerFrame");
  const responseAtMs = nowMs();
  return normalizeEqualizerVisualizerFrame(rawFrame, requestAtMs + (responseAtMs - requestAtMs) / 2);
}

function useEqualizerMutation<T>(mutationFn: (input: T) => Promise<EqualizerSnapshot>) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn,
    onSuccess: (snapshot) => {
      const normalizedSnapshot = normalizeEqualizerSnapshot(snapshot);
      queryClient.setQueryData(EQUALIZER_QUERY_KEY, normalizedSnapshot);
      if (isWailsRuntimeReady()) {
        void Events.Emit(EQUALIZER_SNAPSHOT_EVENT, normalizedSnapshot);
      }
    },
  });
}

async function callEqualizer(method: string, ...args: unknown[]) {
  return Call.ByName(`${SERVICE}.${method}`, ...args);
}

function isWailsRuntimeReady() {
  return typeof window !== "undefined" && typeof (window as any)._wails?.dispatchWailsEvent === "function";
}

function nowMs() {
  return typeof performance !== "undefined" ? performance.now() : Date.now();
}

function isDocumentHidden() {
  return typeof document !== "undefined" && document.visibilityState === "hidden";
}
