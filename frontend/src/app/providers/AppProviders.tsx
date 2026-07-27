import { useEffect, type PropsWithChildren } from "react";
import { QueryClientProvider } from "@tanstack/react-query";
import { Events } from "@wailsio/runtime";

import { createQueryClient } from "./query-client";
import {
  LIBRARY_DETAIL_QUERY_KEY,
  LIBRARY_ENDED_OPERATIONS_QUERY_KEY,
  LIBRARY_FILE_EVENTS_QUERY_KEY,
  LIBRARY_HISTORY_QUERY_KEY,
  LIBRARY_LIST_QUERY_KEY,
  LIBRARY_OPERATIONS_QUERY_KEY,
  LIBRARY_WORKSPACE_PROJECT_QUERY_KEY,
  LIBRARY_WORKSPACE_QUERY_KEY,
} from "@/shared/query/library";
import { shouldRefreshEndedOperations } from "@/shared/query/complete-operations";
import { DEPENDENCIES_QUERY_KEY } from "@/shared/query/dependencies";
import { catalogKeys } from "@/shared/query/catalog";
import { PETS_QUERY_KEY } from "@/shared/query/pets";
import {
  APP_SESSIONS_CHANGED_EVENT,
  APP_SESSIONS_QUERY_KEY,
} from "@/shared/query/appSessions";
import {
  REALTIME_TOPICS,
  onRealtimeConnected,
  registerTopic,
  startRealtime,
} from "@/shared/realtime";
import { messageBus } from "@/shared/message";
import { t } from "@/shared/i18n";
import { installInputModalityTracking } from "@/shared/ui/input-modality";
import { STARTUP_READY_EVENT } from "@/startup-presentation";

const queryClient = createQueryClient();

function useInputModality() {
  useEffect(() => {
    if (typeof document === "undefined") {
      return;
    }
    return installInputModalityTracking(document);
  }, []);
}

function useSuppressNativeTooltips() {
  useEffect(() => {
    if (typeof document === "undefined") {
      return;
    }

    const suppressedTitles = new WeakMap<Element, string>();
    const activeElements = new Set<Element>();

    const isElement = (value: EventTarget | null): value is Element =>
      value instanceof Element;
    const isNode = (value: EventTarget | null): value is Node =>
      typeof Node !== "undefined" && value instanceof Node;

    const suppressElement = (element: Element) => {
      const title = element.getAttribute("title");
      if (!title) {
        return;
      }
      suppressedTitles.set(element, title);
      activeElements.add(element);
      element.removeAttribute("title");
    };

    const suppressFromTarget = (target: EventTarget | null) => {
      if (!isElement(target)) {
        return;
      }
      let current: Element | null = target;
      while (current) {
        suppressElement(current);
        current = current.parentElement;
      }
    };

    const shouldKeepSuppressed = (element: Element, relatedTarget: Node | null) => {
      if (relatedTarget && element.contains(relatedTarget)) {
        return true;
      }
      const activeElement = document.activeElement;
      if (activeElement && element.contains(activeElement)) {
        return true;
      }
      try {
        return element.matches(":hover");
      } catch {
        return false;
      }
    };

    const restoreInactiveTitles = (relatedTarget: EventTarget | null) => {
      const relatedNode = isNode(relatedTarget) ? relatedTarget : null;
      activeElements.forEach((element) => {
        if (shouldKeepSuppressed(element, relatedNode)) {
          return;
        }
        const title = suppressedTitles.get(element);
        if (title) {
          element.setAttribute("title", title);
        }
        suppressedTitles.delete(element);
        activeElements.delete(element);
      });
    };

    const handleEnter = (event: Event) => {
      suppressFromTarget(event.target);
    };
    const handleLeave = (event: Event) => {
      restoreInactiveTitles((event as MouseEvent | FocusEvent).relatedTarget);
    };

    const observer = new MutationObserver((mutations) => {
      mutations.forEach((mutation) => {
        if (
          mutation.type !== "attributes" ||
          mutation.attributeName !== "title" ||
          !activeElements.has(mutation.target as Element)
        ) {
          return;
        }
        suppressElement(mutation.target as Element);
      });
    });

    document.addEventListener("pointerover", handleEnter, true);
    document.addEventListener("pointermove", handleEnter, true);
    document.addEventListener("pointerout", handleLeave, true);
    document.addEventListener("focusin", handleEnter, true);
    document.addEventListener("focusout", handleLeave, true);
    observer.observe(document.documentElement, {
      attributeFilter: ["title"],
      attributes: true,
      subtree: true,
    });

    return () => {
      document.removeEventListener("pointerover", handleEnter, true);
      document.removeEventListener("pointermove", handleEnter, true);
      document.removeEventListener("pointerout", handleLeave, true);
      document.removeEventListener("focusin", handleEnter, true);
      document.removeEventListener("focusout", handleLeave, true);
      observer.disconnect();
      activeElements.forEach((element) => {
        const title = suppressedTitles.get(element);
        if (title) {
          element.setAttribute("title", title);
        }
      });
    };
  }, []);
}

function invalidateLibraryQueries(libraryId?: string) {
	queryClient.invalidateQueries({ queryKey: catalogKeys.all, refetchType: "active" });
  queryClient.invalidateQueries({ queryKey: LIBRARY_LIST_QUERY_KEY, refetchType: "active" });
  queryClient.invalidateQueries({ queryKey: LIBRARY_OPERATIONS_QUERY_KEY, refetchType: "active" });
  queryClient.invalidateQueries({ queryKey: LIBRARY_HISTORY_QUERY_KEY, refetchType: "active" });
  queryClient.invalidateQueries({ queryKey: LIBRARY_FILE_EVENTS_QUERY_KEY, refetchType: "active" });
  if (libraryId) {
    queryClient.invalidateQueries({ queryKey: [...LIBRARY_DETAIL_QUERY_KEY, libraryId], refetchType: "active" });
    queryClient.invalidateQueries({ queryKey: [...LIBRARY_WORKSPACE_QUERY_KEY, libraryId], refetchType: "active" });
    queryClient.invalidateQueries({ queryKey: [...LIBRARY_WORKSPACE_PROJECT_QUERY_KEY, libraryId], refetchType: "active" });
    return;
  }
  queryClient.invalidateQueries({ queryKey: LIBRARY_DETAIL_QUERY_KEY, refetchType: "active" });
  queryClient.invalidateQueries({ queryKey: LIBRARY_WORKSPACE_QUERY_KEY, refetchType: "active" });
  queryClient.invalidateQueries({ queryKey: LIBRARY_WORKSPACE_PROJECT_QUERY_KEY, refetchType: "active" });
}

function resolveLibraryID(payload: unknown) {
  if (!payload || typeof payload !== "object") {
    return "";
  }
  const record = payload as Record<string, unknown>;
  return typeof record.libraryId === "string" ? record.libraryId.trim() : "";
}

export function AppProviders({
  children,
  runtimeEnabled = true,
  telemetryEnabled = false,
}: PropsWithChildren<{
  runtimeEnabled?: boolean;
  telemetryEnabled?: boolean;
}>) {
  useInputModality();
  useSuppressNativeTooltips();

  useEffect(() => {
    if (!telemetryEnabled) {
      return;
    }
    let disposed = false;
    let startTimer: number | undefined;
    let telemetry: { start: () => Promise<void>; stop: () => void } | undefined;
    const loadTelemetry = () => {
      void import("@/shared/telemetry/manager")
        .then(({ TelemetryManager }) => {
          if (disposed) return;
          telemetry = new TelemetryManager();
          return telemetry.start();
        })
        .catch(() => undefined);
    };
    const scheduleTelemetry = () => {
      if (disposed || startTimer !== undefined) return;
      startTimer = window.setTimeout(loadTelemetry, 250);
    };
    if (document.documentElement.dataset.startupState === "ready") {
      scheduleTelemetry();
    } else {
      window.addEventListener("xiadown:startup-ready", scheduleTelemetry, {
        once: true,
      });
    }
    return () => {
      disposed = true;
      window.removeEventListener("xiadown:startup-ready", scheduleTelemetry);
      if (startTimer !== undefined) window.clearTimeout(startTimer);
      telemetry?.stop();
    };
  }, [telemetryEnabled]);

  useEffect(() => {
    if (!runtimeEnabled) {
      return;
    }
    let disposed = false;
    let endedOperationsRefreshTimer: number | undefined;
    let endedOperationsRefreshScheduledAt = 0;
    let realtimeStartTimer: number | undefined;
    let realtimeRetryTimer: number | undefined;
    let realtimeStartAttempts = 0;
    const realtimeRetryDelays = [500, 1_500, 4_000] as const;
    const scheduleEndedOperationsRefresh = () => {
      endedOperationsRefreshScheduledAt = Date.now();
      if (endedOperationsRefreshTimer !== undefined) {
        window.clearTimeout(endedOperationsRefreshTimer);
      }
      endedOperationsRefreshTimer = window.setTimeout(() => {
        const state = queryClient.getQueryState(LIBRARY_ENDED_OPERATIONS_QUERY_KEY);
        if (state?.fetchStatus === "fetching") {
          scheduleEndedOperationsRefresh();
          return;
        }
        if ((state?.dataUpdatedAt ?? 0) > endedOperationsRefreshScheduledAt) {
          return;
        }
        void queryClient.invalidateQueries({
          queryKey: LIBRARY_ENDED_OPERATIONS_QUERY_KEY,
          refetchType: "active",
        });
      }, 750);
    };

    const startRealtimeAfterStartup = () => {
      if (realtimeStartTimer !== undefined) return;
      realtimeStartTimer = window.setTimeout(() => {
        const connect = () => {
          if (disposed) return;
          realtimeStartAttempts += 1;
          startRealtime().catch((error) => {
            if (disposed) return;
            const retryDelay = realtimeRetryDelays[realtimeStartAttempts - 1];
            if (retryDelay !== undefined) {
              realtimeRetryTimer = window.setTimeout(connect, retryDelay);
              return;
            }
            console.warn("[realtime] failed to start", error);
            messageBus.publishToast({
              intent: "warning",
              title: t("common.realtimeUnavailableTitle"),
              description: t("common.realtimeUnavailableDescription"),
            });
          });
        };
        connect();
      }, 300);
    };
    if (document.documentElement.dataset.startupState === "ready") {
      startRealtimeAfterStartup();
    } else {
      window.addEventListener(STARTUP_READY_EVENT, startRealtimeAfterStartup, {
        once: true,
      });
    }

    const offDependenciesUpdated = Events.On("dependencies:updated", () => {
      queryClient.invalidateQueries({ queryKey: DEPENDENCIES_QUERY_KEY, refetchType: "all" });
      queryClient.invalidateQueries({ queryKey: ["dependencies-updates"], refetchType: "all" });
    });
    const offPetsUpdated = Events.On("pets:updated", () => {
      queryClient.invalidateQueries({ queryKey: PETS_QUERY_KEY, refetchType: "all" });
    });
    const offAppSessionsChanged = Events.On(APP_SESSIONS_CHANGED_EVENT, () => {
      queryClient.invalidateQueries({ queryKey: APP_SESSIONS_QUERY_KEY, refetchType: "all" });
    });
    const offRealtimeConnected = onRealtimeConnected(() => {
      // The server intentionally does not replay from sequence zero. Reconcile
      // after the socket is actually open so events emitted during the short
      // post-paint delay cannot leave initial Library queries stale.
      invalidateLibraryQueries();
      void queryClient.invalidateQueries({
        queryKey: LIBRARY_ENDED_OPERATIONS_QUERY_KEY,
        refetchType: "active",
      });
    });

    const unsubscribeLibraryOperation = registerTopic(REALTIME_TOPICS.library.operation, (event) => {
      invalidateLibraryQueries(resolveLibraryID(event?.payload));
      if (shouldRefreshEndedOperations(event)) {
        scheduleEndedOperationsRefresh();
      }
    });
    const unsubscribeLibraryFile = registerTopic(REALTIME_TOPICS.library.file, (event) => {
      invalidateLibraryQueries(resolveLibraryID(event?.payload));
    });
    const unsubscribeLibraryHistory = registerTopic(REALTIME_TOPICS.library.history, (event) => {
      invalidateLibraryQueries(resolveLibraryID(event?.payload));
    });
    const unsubscribeLibraryWorkspace = registerTopic(REALTIME_TOPICS.library.workspace, (event) => {
      invalidateLibraryQueries(resolveLibraryID(event?.payload));
    });
    const unsubscribeLibraryWorkspaceProject = registerTopic(REALTIME_TOPICS.library.workspaceProject, (event) => {
      invalidateLibraryQueries(resolveLibraryID(event?.payload));
    });

    return () => {
      disposed = true;
      window.removeEventListener(STARTUP_READY_EVENT, startRealtimeAfterStartup);
      offDependenciesUpdated();
      offPetsUpdated();
      offAppSessionsChanged();
      offRealtimeConnected();
      unsubscribeLibraryOperation();
      unsubscribeLibraryFile();
      unsubscribeLibraryHistory();
      unsubscribeLibraryWorkspace();
      unsubscribeLibraryWorkspaceProject();
      if (endedOperationsRefreshTimer !== undefined) {
        window.clearTimeout(endedOperationsRefreshTimer);
      }
      if (realtimeStartTimer !== undefined) {
        window.clearTimeout(realtimeStartTimer);
      }
      if (realtimeRetryTimer !== undefined) {
        window.clearTimeout(realtimeRetryTimer);
      }
    };
  }, [runtimeEnabled]);

  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}
