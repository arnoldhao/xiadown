import { useEffect, type PropsWithChildren } from "react";
import { QueryClientProvider } from "@tanstack/react-query";
import { Call, Events } from "@wailsio/runtime";

import { createQueryClient } from "./query-client";
import {
  LIBRARY_DETAIL_QUERY_KEY,
  LIBRARY_FILE_EVENTS_QUERY_KEY,
  LIBRARY_HISTORY_QUERY_KEY,
  LIBRARY_LIST_QUERY_KEY,
  LIBRARY_OPERATIONS_QUERY_KEY,
  LIBRARY_WORKSPACE_PROJECT_QUERY_KEY,
  LIBRARY_WORKSPACE_QUERY_KEY,
} from "@/shared/query/library";
import { DEPENDENCIES_QUERY_KEY } from "@/shared/query/dependencies";
import { PETS_QUERY_KEY } from "@/shared/query/pets";
import { REALTIME_TOPICS, registerTopic, startRealtime } from "@/shared/realtime";
import { messageBus } from "@/shared/message";
import { TelemetryManager } from "@/shared/telemetry/manager";
import { normalizeUpdateInfo, type UpdateInfo, useUpdateStore } from "@/shared/store/update";
import { t } from "@/shared/i18n";

const queryClient = createQueryClient();

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

export function AppProviders({ children }: PropsWithChildren) {
  const setUpdateInfo = useUpdateStore((state) => state.setInfo);

  useSuppressNativeTooltips();

  useEffect(() => {
    const telemetry = new TelemetryManager();
    void telemetry.start();
    return () => {
      telemetry.stop();
    };
  }, []);

  useEffect(() => {
    startRealtime().catch((error) => {
      console.warn("[realtime] failed to start", error);
      messageBus.publishToast({
        intent: "warning",
        title: t("common.realtimeUnavailableTitle"),
        description: t("common.realtimeUnavailableDescription"),
      });
    });

    Call.ByName("xiadown/internal/presentation/wails.UpdateHandler.GetState")
      .then((result) => {
        setUpdateInfo(normalizeUpdateInfo(result as Partial<UpdateInfo>));
      })
      .catch((error) => {
        console.warn("[update] get state failed", error);
      });

    const offDependenciesUpdated = Events.On("dependencies:updated", () => {
      queryClient.invalidateQueries({ queryKey: DEPENDENCIES_QUERY_KEY, refetchType: "all" });
      queryClient.invalidateQueries({ queryKey: ["dependencies-updates"], refetchType: "all" });
    });
    const offPetsUpdated = Events.On("pets:updated", () => {
      queryClient.invalidateQueries({ queryKey: PETS_QUERY_KEY, refetchType: "all" });
    });

    const unsubscribeLibraryOperation = registerTopic(REALTIME_TOPICS.library.operation, (event) => {
      invalidateLibraryQueries(resolveLibraryID(event?.payload));
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
      offDependenciesUpdated();
      offPetsUpdated();
      unsubscribeLibraryOperation();
      unsubscribeLibraryFile();
      unsubscribeLibraryHistory();
      unsubscribeLibraryWorkspace();
      unsubscribeLibraryWorkspaceProject();
    };
  }, [setUpdateInfo]);

  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}
