import { Events,System } from "@wailsio/runtime";
import {
ArrowUpCircle,
CheckCircle2,
FileText,
FolderOpen,
Link2,
Plus,
Radar,
RefreshCcw,
Settings2,
PawPrint,
Waves,
Wrench
} from "lucide-react";
import * as React from "react";

import {
ListenPage,
type ListenExternalCommand,
type ListenNowPlayingStatus
} from "@/app/main/Listen";
import { RunningPage } from "@/app/main/RunningPage";
import {
setPendingSettingsTab,
type XiaSettingsTabId,
} from "@/app/settings/sectionStorage";
import { PetsGalleryPage,type PetsGalleryNavigation } from "@/app/pets-gallery";
import { SniffDeskPage } from "@/app/sniff-desk";
import {
AppSessionsSection
} from "@/features/settings/app-sessions";
import {
resolveActivePet,
useRunningPetAnimation,
} from "@/features/pets/shared";
import {
getXiaText,
resolveLibraryCoverURL
} from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
import type { LibraryDTO,OperationListItemDTO } from "@/shared/contracts/library";
import { messageBus, publishOSNotification } from "@/shared/message";
import {
useDependencies,
useDependencyUpdates
} from "@/shared/query/dependencies";
import {
useListLibraries,
useListOperations,
useOpenLibraryPath,
useCDPBrowserStatus,
useStopCDPBrowserRuntime
} from "@/shared/query/library";
import { useHttpBaseURL } from "@/shared/query/runtime";
import {
setWelcomeWindowChromeHidden,
useShowSettingsWindow
} from "@/shared/query/settings";
import { usePets } from "@/shared/query/pets";
import { openExternalURL,useCurrentUserProfile } from "@/shared/query/system";
import {
useRestartToApply,
useUpdateState
} from "@/shared/query/update";
import { useSettingsStore } from "@/shared/store/settings";
import {
displayUpdateVersion,
hasPreparedUpdate,
hasRemoteUpdate,
useUpdateStore,
} from "@/shared/store/update";
import {
DropdownMenu,
DropdownMenuContent,
DropdownMenuItem,
DropdownMenuSeparator,
DropdownMenuTrigger
} from "@/shared/ui/dropdown-menu";
import {
resolveUserDisplayName,
resolveUserSubtitle,
UserAvatar,
} from "@/shared/ui/user-avatar";
import {
buildAssetPreviewURL
} from "@/shared/utils/resourceHelpers";
import {
readXiaAppearance,
resolveThemePack,
} from "@/shared/styles/xiadown-theme";

import { CompletedPage } from "@/app/main/completed/CompletedPage";
import { WhatsNewFeatureDialog } from "@/app/main/dialogs";
import {
LISTEN_NOW_PLAYING_EVENT,
LISTEN_NOW_PLAYING_STORAGE_KEY,
LISTEN_TRAY_COMMAND_EVENT,
} from "@/app/main/listen/catalog";
import { formatVersionBadge,normalizeDependencyVersion,resolveCompletedStatusLabel } from "@/app/main/helpers";
import { CORE_DEPENDENCIES,MAIN_SIDEBAR_ACTION_CLASS,MAIN_SIDEBAR_ICON_CLASS,SETUP_STORAGE_KEY,SIDEBAR_DROPDOWN_CONTENT_CLASS_NAME,SIDEBAR_DROPDOWN_ICON_SLOT_CLASS_NAME,SIDEBAR_DROPDOWN_ITEM_CLASS_NAME,useSetupState } from "@/app/main/main-constants";
import { NewTaskDialog } from "@/app/main/NewTaskDialog";
import { CDPBrowserStatusMiniButton,ListenNowPlayingMiniPlayer,resolveSidebarSurface,SidebarIconButton } from "@/app/main/sidebar";
import type { CompletedFileEntry,MainViewId,NewTaskDialogMode,NewTaskDialogTranscodeSource } from "@/app/main/types";
import {
WELCOME_DEBUG_EVENT,
WelcomeScreen,
type WelcomeDebugCommand,
type WelcomeDebugStep,
} from "@/app/main/WelcomeScreen";

const NOTIFIABLE_OPERATION_STATUSES = new Set(["succeeded", "failed"]);
const MAIN_NEW_DOWNLOAD_EVENT = "main:new-download";
const DOCUMENTATION_ORIGIN = "https://xiadown.app";
const DOCUMENTATION_PATH_BY_LANGUAGE: Record<string, string> = {
  "zh-CN": "/docs/",
  "zh-TW": "/zh-tw/docs/",
  en: "/en/docs/",
  "ja-JP": "/ja-jp/docs/",
  "ko-KR": "/ko-kr/docs/",
  "es-419": "/es-419/docs/",
  "pt-BR": "/pt-br/docs/",
  "id-ID": "/id-id/docs/",
  "vi-VN": "/vi-vn/docs/",
};

function normalizeOperationStatus(status?: string) {
  return (status ?? "").trim().toLowerCase();
}

function resolveDocumentationLanguage(language?: string | null) {
  const normalized = language?.trim() || "zh-CN";
  return DOCUMENTATION_PATH_BY_LANGUAGE[normalized] ? normalized : "zh-CN";
}

function normalizeDocumentationPagePath(path?: string) {
  const normalized = (path ?? "").trim().replace(/^\/+|\/+$/g, "");
  if (!normalized || normalized === "docs") {
    return "";
  }
  return normalized
    .replace(/^(?:zh-tw|en|ja-jp|ko-kr|es-419|pt-br|id-id|vi-vn)\/docs\/?/i, "")
    .replace(/^docs\/?/i, "")
    .replace(/\/+$/g, "");
}

function buildDocumentationURL(language?: string | null, path?: string) {
  const url = new URL(DOCUMENTATION_ORIGIN);
  const languageRoot = DOCUMENTATION_PATH_BY_LANGUAGE[resolveDocumentationLanguage(language)];
  const pagePath = normalizeDocumentationPagePath(path);
  url.pathname = pagePath ? `${languageRoot}${pagePath}/` : languageRoot;
  return url.toString();
}

function resolveEffectiveDownloadDirectory(directory?: string) {
  const trimmed = directory?.trim() ?? "";
  if (!trimmed) {
    return "";
  }
  const normalized = trimmed.replace(/[\\/]+$/, "") || trimmed;
  const baseName = normalized.split(/[\\/]+/).pop()?.trim().toLowerCase() ?? "";
  if (baseName === "xiadown") {
    return normalized;
  }
  const separator = normalized.includes("\\") && !normalized.includes("/")
    ? "\\"
    : "/";
  return `${normalized}${separator}xiadown`;
}

function resolveOperationNotificationCoverURL(
  baseURL: string,
  operation: OperationListItemDTO,
  filesById: Map<string, LibraryDTO["files"][number]>,
  librariesById: Map<string, LibraryDTO>,
) {
  const outputCoverURL = (operation.outputFiles ?? [])
    .map((output) => {
      const kind = normalizeOperationStatus(output.kind);
      if (kind !== "thumbnail" && kind !== "image") {
        return "";
      }
      const path = filesById.get(output.fileId)?.storage.localPath?.trim() ?? "";
      return path ? buildAssetPreviewURL(baseURL, path) : "";
    })
    .find(Boolean);
  if (outputCoverURL) {
    return outputCoverURL;
  }

  const operationCoverURL = [...filesById.values()]
    .filter((file) => file.libraryId === operation.libraryId)
    .filter((file) => {
      const kind = normalizeOperationStatus(file.kind);
      return (kind === "thumbnail" || kind === "image") && !file.state.deleted;
    })
    .filter((file) => {
      const operationId = operation.operationId.trim();
      return file.latestOperationId === operationId || file.origin.operationId === operationId;
    })
    .map((file) => {
      const path = file.storage.localPath?.trim() ?? "";
      return path ? buildAssetPreviewURL(baseURL, path) : "";
    })
    .find(Boolean);
  if (operationCoverURL) {
    return operationCoverURL;
  }

  return resolveLibraryCoverURL(baseURL, librariesById.get(operation.libraryId));
}

function resolveOperationNotificationTitle(operation: OperationListItemDTO) {
  return operation.name.trim() || operation.operationId.trim();
}

export function MainApp() {
  const settings = useSettingsStore((state) => state.settings);
  const profile = useCurrentUserProfile().data;
  const showSettingsWindow = useShowSettingsWindow();
  const { data: httpBaseURL = "" } = useHttpBaseURL();
  const petsQuery = usePets();
  const toolsQuery = useDependencies({ refetchInterval: 3_000 });
  const dependencyUpdatesQuery = useDependencyUpdates();
  const updateStateQuery = useUpdateState();
  const updateInfo = useUpdateStore((state) => state.info);
  const setUpdateInfo = useUpdateStore((state) => state.setInfo);
  const restartToApply = useRestartToApply();
  const librariesQuery = useListLibraries();
  const runningQuery = useListOperations({
    status: ["queued", "running"],
    limit: 200,
  });
  const terminalQuery = useListOperations({
    status: ["succeeded", "failed", "canceled"],
    limit: 300,
  });
  const openPath = useOpenLibraryPath();
  const cdpStatusQuery = useCDPBrowserStatus(true);
  const stopCDPBrowserRuntime = useStopCDPBrowserRuntime();
  const [setupState, setSetupState] = useSetupState();
  const [debugWelcomeOpen, setDebugWelcomeOpen] = React.useState(false);
  const [activeView, setActiveView] = React.useState<MainViewId>("running");
  const [petsGalleryNavigation, setPetsGalleryNavigation] =
    React.useState<PetsGalleryNavigation | null>(null);
  const [newTaskDialogOpen, setNewTaskDialogOpen] = React.useState(false);
  const [newTaskDialogMode, setNewTaskDialogMode] =
    React.useState<NewTaskDialogMode>("download");
  const [prefilledDownloadURL, setPrefilledDownloadURL] = React.useState("");
  const [prefilledTranscodeSource, setPrefilledTranscodeSource] =
    React.useState<NewTaskDialogTranscodeSource | null>(null);
  const [listenNowPlaying, setListenNowPlaying] =
    React.useState<ListenNowPlayingStatus | null>(null);
  const [listenControlCommand, setListenControlCommand] =
    React.useState<ListenExternalCommand | null>(null);
  const listenCommandIdRef = React.useRef(0);
  const listenNotificationKeyRef = React.useRef("");
  const activeOperationSnapshotRef = React.useRef<Map<string, OperationListItemDTO>>(new Map());
  const notifiedOperationIdsRef = React.useRef<Set<string>>(new Set());

  const text = getXiaText(settings?.language);
  const cdpStatus = cdpStatusQuery.data ?? null;
  const appearance = readXiaAppearance(settings);
  const theme = resolveThemePack(appearance.themePackId);
  const isWindows = System.IsWindows();
  const welcomeOpen = !setupState.completed || debugWelcomeOpen;
  const closeOrphanCDPBrowser = React.useCallback(
    async (runtimeId: string) => {
      try {
        await stopCDPBrowserRuntime.mutateAsync({ runtimeId });
      } catch (error) {
        messageBus.publishToast({
          intent: "danger",
          title: text.sniffDesk.cdpClose,
          description: error instanceof Error ? error.message : String(error),
        });
      }
    },
    [stopCDPBrowserRuntime, text.sniffDesk],
  );
  const runningOperations = runningQuery.data ?? [];
  const terminalOperations = terminalQuery.data ?? [];
  const libraries = librariesQuery.data ?? [];
  const librariesById = React.useMemo(
    () => new Map(libraries.map((item) => [item.id, item])),
    [libraries],
  );
  const filesById = React.useMemo(
    () =>
      new Map(
        libraries.flatMap((library) =>
          library.files.map((file) => [file.id, file] as const),
        ),
      ),
    [libraries],
  );
  const runningPetAnimation = useRunningPetAnimation(
    runningOperations,
    terminalOperations,
    terminalQuery.isFetched,
  );
  const dependencyItems = React.useMemo(
    () =>
      (toolsQuery.data ?? []).filter((item) =>
        CORE_DEPENDENCIES.includes(
          item.name as (typeof CORE_DEPENDENCIES)[number],
        ),
      ),
    [toolsQuery.data],
  );
  const dependencyUpdatesByName = React.useMemo(
    () =>
      new Map(
        (dependencyUpdatesQuery.data ?? []).map((item) => [item.name, item]),
      ),
    [dependencyUpdatesQuery.data],
  );
  const dependencyUpdateCount = React.useMemo(
    () =>
      dependencyItems.filter((item) => {
        if ((item.status ?? "").trim().toLowerCase() !== "installed") {
          return false;
        }
        const latest = normalizeDependencyVersion(
          dependencyUpdatesByName.get(item.name)?.latestVersion,
          item.name,
        );
        const current = normalizeDependencyVersion(item.version, item.name);
        return Boolean(current && latest && current !== latest);
      }).length,
    [dependencyItems, dependencyUpdatesByName],
  );
  const hasDependencyUpdate = dependencyUpdateCount > 0;
  const hasPreparedAppUpdate =
    updateInfo.status === "ready_to_restart" && hasPreparedUpdate(updateInfo);
  const hasAppUpdateMenu =
    hasPreparedAppUpdate ||
    hasRemoteUpdate(updateInfo) ||
    updateInfo.status === "downloading" ||
    updateInfo.status === "installing";
  const hasUserMenuUpdate = hasAppUpdateMenu || hasDependencyUpdate;
  const shellTheme = "dream";

  const localDownloadDirectory = resolveEffectiveDownloadDirectory(
    settings?.downloadDirectory,
  );
  const activePet = React.useMemo(
    () => resolveActivePet(petsQuery.data ?? [], settings),
    [settings, petsQuery.data],
  );
  const activePetImageURL = React.useMemo(
    () =>
      activePet
        ? buildAssetPreviewURL(httpBaseURL, activePet.spritesheetPath, activePet.updatedAt)
        : "",
    [activePet, httpBaseURL],
  );

  React.useEffect(() => {
    if (updateStateQuery.data) {
      setUpdateInfo(updateStateQuery.data);
    }
  }, [setUpdateInfo, updateStateQuery.data]);

  React.useEffect(() => {
    void setWelcomeWindowChromeHidden(welcomeOpen).catch(() => {
      // Browser preview and older runtimes can ignore the native chrome bridge.
    });
  }, [welcomeOpen]);

  React.useEffect(() => {
    const emitWelcomeCommand = (detail: WelcomeDebugCommand) => {
      window.dispatchEvent(new CustomEvent(WELCOME_DEBUG_EVENT, { detail }));
    };
    const openWelcomeAndEmit = (detail: WelcomeDebugCommand) => {
      setDebugWelcomeOpen(true);
      window.setTimeout(() => emitWelcomeCommand(detail), 40);
    };
    const api = {
      show: () => openWelcomeAndEmit({ type: "show" }),
      hide: () => {
        emitWelcomeCommand({ type: "hide" });
        setDebugWelcomeOpen(false);
      },
      reset: () => {
        window.localStorage.removeItem(SETUP_STORAGE_KEY);
        setSetupState({ completed: false });
        setDebugWelcomeOpen(false);
        window.setTimeout(
          () => emitWelcomeCommand({ type: "step", step: "proxy" }),
          40,
        );
      },
      step: (step: WelcomeDebugStep) => {
        openWelcomeAndEmit({ type: "step", step });
      },
      proxy: (mode: "none" | "system") => {
        openWelcomeAndEmit({ type: "proxy", mode });
      },
    };

    window.xiadownWelcome = api;
    return () => {
      if (window.xiadownWelcome === api) {
        delete window.xiadownWelcome;
      }
    };
  }, [setSetupState]);

  React.useEffect(() => {
    const snapshots = activeOperationSnapshotRef.current;
    runningOperations.forEach((operation) => {
      const id = operation.operationId.trim();
      if (!id) {
        return;
      }
      snapshots.set(id, operation);
    });
  }, [runningOperations]);

  React.useEffect(() => {
    const snapshots = activeOperationSnapshotRef.current;
    const notified = notifiedOperationIdsRef.current;
    terminalOperations.forEach((operation) => {
      const operationId = operation.operationId.trim();
      if (!operationId) {
        return;
      }
      const status = normalizeOperationStatus(operation.status);
      if (status === "canceled") {
        snapshots.delete(operationId);
        return;
      }
      if (!NOTIFIABLE_OPERATION_STATUSES.has(status)) {
        return;
      }
      if (notified.has(operationId) || !snapshots.has(operationId)) {
        return;
      }
      notified.add(operationId);
      snapshots.delete(operationId);

      const title = resolveOperationNotificationTitle(operation);
      const statusLabel = resolveCompletedStatusLabel(text, status);
      const coverURL = resolveOperationNotificationCoverURL(
        httpBaseURL,
        operation,
        filesById,
        librariesById,
      );
      void publishOSNotification({
        id: `operation_${operationId}_${status}`,
        title,
        body: statusLabel,
        iconUrl: coverURL,
        imageUrl: coverURL,
        source: "XiaDown",
        data: {
          source: "operation",
          operationId,
          status,
          title,
          libraryId: operation.libraryId,
          libraryName: operation.libraryName ?? "",
        },
      });
    });
  }, [text, filesById, httpBaseURL, librariesById, terminalOperations]);

  React.useEffect(() => {
    if (!listenNowPlaying) {
      return;
    }
    try {
      localStorage.setItem(
        LISTEN_NOW_PLAYING_STORAGE_KEY,
        JSON.stringify(listenNowPlaying),
      );
    } catch {
      // noop
    }
    void Events.Emit(LISTEN_NOW_PLAYING_EVENT, listenNowPlaying);
  }, [listenNowPlaying]);

  React.useEffect(() => {
    if (!listenNowPlaying || listenNowPlaying.state !== "playing") {
      return;
    }
    const title = listenNowPlaying.title.trim();
    if (!title) {
      return;
    }
    const artist = listenNowPlaying.subtitle.trim();
    const artworkURL = listenNowPlaying.artworkURL.trim();
    const notificationKey = [
      listenNowPlaying.mode,
      title,
      artist,
      artworkURL,
    ].join("::");
    if (listenNotificationKeyRef.current === notificationKey) {
      return;
    }
    listenNotificationKeyRef.current = notificationKey;

    void publishOSNotification({
      id: `listen_${Date.now()}`,
      title,
      body: artist || text.listen.nowPlaying,
      iconUrl: artworkURL,
      imageUrl: artworkURL,
      source: "Listen",
      data: {
        source: "listen",
        mode: listenNowPlaying.mode,
        title,
        artist,
        artworkURL,
      },
    });
  }, [text.listen.nowPlaying, listenNowPlaying]);

  const openNewTaskDialog = React.useCallback((mode: NewTaskDialogMode, url = "") => {
    setNewTaskDialogMode(mode);
    setPrefilledDownloadURL(mode === "download" ? url : "");
    setPrefilledTranscodeSource(null);
    setNewTaskDialogOpen(true);
  }, []);

  const openDownloadDialog = React.useCallback((url = "") => {
    openNewTaskDialog("download", url);
  }, [openNewTaskDialog]);

  const openTranscodeDialog = React.useCallback((file: CompletedFileEntry) => {
    const inputPath = file.path.trim();
    if (!inputPath) {
      return;
    }
    setNewTaskDialogMode("transcode");
    setPrefilledDownloadURL("");
    setPrefilledTranscodeSource({
      fileId: file.canDelete ? file.id : undefined,
      inputPath,
      title: file.title || file.name,
      author: file.author || undefined,
    });
    setNewTaskDialogOpen(true);
  }, []);

  const sendListenCommand = React.useCallback(
    (command: ListenExternalCommand["command"]) => {
      listenCommandIdRef.current += 1;
      setListenControlCommand({ id: listenCommandIdRef.current, command });
    },
    [],
  );

  React.useEffect(() => {
    const offTrayCommand = Events.On(LISTEN_TRAY_COMMAND_EVENT, (event: any) => {
      const payload = event?.data ?? event;
      const command =
        typeof payload === "string"
          ? payload
          : payload && typeof payload === "object" && typeof payload.command === "string"
            ? payload.command
            : "";
      if (
        command === "previous" ||
        command === "toggle" ||
        command === "play" ||
        command === "pause" ||
        command === "next"
      ) {
        sendListenCommand(command);
      }
    });
    return () => {
      offTrayCommand();
    };
  }, [sendListenCommand]);

  React.useEffect(() => {
    const offNewDownload = Events.On(MAIN_NEW_DOWNLOAD_EVENT, () => {
      openDownloadDialog();
    });
    return () => {
      offNewDownload();
    };
  }, [openDownloadDialog]);

  const openPetsGallery = React.useCallback((navigation?: Omit<PetsGalleryNavigation, "nonce">) => {
    setActiveView("petsGallery");
    setPetsGalleryNavigation({
      action: navigation?.action ?? "gallery",
      petId: navigation?.petId,
      nonce: Date.now(),
    });
  }, []);

  const openDocumentation = React.useCallback((path = "") => {
    const url = buildDocumentationURL(settings?.language, path);
    void openExternalURL(url).catch((error) => {
      console.warn("[Main] open documentation unavailable", { url, error });
    });
  }, [settings?.language]);

  const openSettingsTab = React.useCallback(
    (tab: XiaSettingsTabId) => {
      setPendingSettingsTab(tab);
      void showSettingsWindow.mutateAsync().finally(() => {
        void Events.Emit("settings:navigate", tab);
      });
    },
    [showSettingsWindow],
  );

  React.useEffect(() => {
    const offNavigate = Events.On("pets:gallery:navigate", (event: any) => {
      const payload = event?.data ?? event;
      const record =
        payload && typeof payload === "object"
          ? (payload as Record<string, unknown>)
          : {};
      const action = record.action === "detail" ? record.action : "gallery";
      const petId = typeof record.petId === "string" ? record.petId.trim() : "";
      openPetsGallery({ action, petId });
    });
    return () => {
      offNavigate();
    };
  }, [openPetsGallery]);

  const handleRestartPreparedUpdate = React.useCallback(async () => {
    try {
      const next = await restartToApply.mutateAsync();
      setUpdateInfo(next);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      messageBus.publishToast({
        intent: "warning",
        title: text.about.restartAfterUpdate,
        description: message,
      });
    }
  }, [text.about.restartAfterUpdate, restartToApply, setUpdateInfo]);

  const userMenuUpdateItems = React.useMemo(
    () =>
      [
        hasAppUpdateMenu
          ? {
              key: "app-update",
              label: hasPreparedAppUpdate
                ? text.about.restartAfterUpdate
                : text.sidebar.appUpdate,
              meta: formatVersionBadge(displayUpdateVersion(updateInfo)),
              Icon: hasPreparedAppUpdate ? RefreshCcw : ArrowUpCircle,
              onSelect: () => {
                if (hasPreparedAppUpdate) {
                  void handleRestartPreparedUpdate();
                  return;
                }
                openSettingsTab("about");
              },
              disabled: restartToApply.isPending,
            }
          : null,
        hasDependencyUpdate
          ? {
              key: "dependency-update",
              label: text.sidebar.dependencyUpdate,
              meta: String(dependencyUpdateCount),
              Icon: Wrench,
              onSelect: () => openSettingsTab("download"),
              disabled: false,
            }
          : null,
      ].filter((item): item is NonNullable<typeof item> => Boolean(item)),
    [
      text.about.restartAfterUpdate,
      text.sidebar.appUpdate,
      text.sidebar.dependencyUpdate,
      dependencyUpdateCount,
      handleRestartPreparedUpdate,
      hasAppUpdateMenu,
      hasDependencyUpdate,
      hasPreparedAppUpdate,
      openSettingsTab,
      restartToApply.isPending,
      updateInfo,
    ],
  );

  return (
    <div
      data-shell-theme={shellTheme}
      data-sidebar-style={appearance.sidebarStyle}
      className={cn(
        "app-main-shell relative flex h-screen overflow-hidden bg-background text-foreground",
        "app-dream-frame app-dream-window",
      )}
    >
      <aside
        className={cn(
          "app-main-sidebar relative z-40 flex w-[var(--app-main-sidebar-width)] shrink-0 flex-col items-center justify-between border-sidebar-border/70 px-3 pb-4 pt-3 text-sidebar-foreground",
          resolveSidebarSurface(theme.id, appearance.sidebarStyle, shellTheme),
        )}
      >
        <div className="app-main-sidebar-nav flex flex-col items-center gap-5">
          <div
            className={cn("min-h-[34px]", isWindows && "wails-drag w-full")}
            aria-hidden="true"
          />

          <div className="flex flex-col items-center gap-3">
            <SidebarIconButton
              label={text.views.running}
              active={activeView === "running"}
              onClick={() => setActiveView("running")}
            >
              <Waves className={MAIN_SIDEBAR_ICON_CLASS} />
            </SidebarIconButton>
            <SidebarIconButton
              label={text.views.completed}
              active={activeView === "completed"}
              onClick={() => setActiveView("completed")}
            >
              <CheckCircle2 className={MAIN_SIDEBAR_ICON_CLASS} />
            </SidebarIconButton>

            <SidebarIconButton
              label={text.sidebar.newTask}
              onClick={() => openDownloadDialog()}
            >
              <Plus className={MAIN_SIDEBAR_ICON_CLASS} />
            </SidebarIconButton>
          </div>
        </div>

        <div className="app-main-sidebar-dock flex flex-col items-center gap-3">
          <CDPBrowserStatusMiniButton
            status={cdpStatus}
            text={text}
            active={activeView === "sniffDesk"}
            stopping={stopCDPBrowserRuntime.isPending}
            onOpenSniffDesk={() => setActiveView("sniffDesk")}
            onCloseOrphan={(runtimeId) => void closeOrphanCDPBrowser(runtimeId)}
          />

          <ListenNowPlayingMiniPlayer
            status={listenNowPlaying}
            text={text}
            active={activeView === "listen"}
            surface={settings?.effectiveAppearance === "dark" ? "dark" : "white"}
            onOpen={() => setActiveView("listen")}
            onToggle={() =>
              sendListenCommand(
                listenNowPlaying?.state === "playing" ? "pause" : "play",
              )
            }
            onControlCommand={sendListenCommand}
          />

          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <button
                type="button"
                aria-label={resolveUserDisplayName(profile)}
                className={cn(
                  MAIN_SIDEBAR_ACTION_CLASS,
                  "app-main-user-button wails-no-drag relative flex items-center justify-center rounded-2xl bg-transparent p-0 outline-none",
                  "hover:bg-card/45",
                )}
              >
                <UserAvatar
                  profile={profile}
                  tone="theme"
                  className="h-11 w-11 rounded-2xl"
                  fallbackClassName="text-xs tracking-[0.08em]"
                />
                {hasUserMenuUpdate ? (
                  <span className="absolute right-0.5 top-0.5 h-2.5 w-2.5 rounded-full border border-background bg-primary" />
                ) : null}
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent
              side="top"
              align="start"
              sideOffset={8}
              className={SIDEBAR_DROPDOWN_CONTENT_CLASS_NAME}
            >
              <div className="flex items-center gap-3 rounded-lg px-3 py-2">
                <UserAvatar
                  profile={profile}
                  tone="theme"
                  className="h-8 w-8 rounded-xl"
                  fallbackClassName="text-[10px]"
                />
                <div className="min-w-[8rem] max-w-[18rem]">
                  <div className="truncate text-sm font-medium text-foreground">
                    {resolveUserDisplayName(profile)}
                  </div>
                  <div className="truncate text-xs text-muted-foreground">
                    {resolveUserSubtitle(profile) || text.sidebar.profileHint}
                  </div>
                </div>
              </div>
              <DropdownMenuSeparator />
              <div className="grid">
                <DropdownMenuItem
                  className={SIDEBAR_DROPDOWN_ITEM_CLASS_NAME}
                  onSelect={() => openPetsGallery({ action: "gallery" })}
                >
                  <div className={SIDEBAR_DROPDOWN_ICON_SLOT_CLASS_NAME}>
                    <PawPrint className="h-4 w-4 text-muted-foreground" />
                  </div>
                  <span className="truncate font-medium text-foreground">
                    {text.petGallery.title}
                  </span>
                </DropdownMenuItem>
                <DropdownMenuItem
                  className={SIDEBAR_DROPDOWN_ITEM_CLASS_NAME}
                  onSelect={() => setActiveView("sniffDesk")}
                >
                  <div className={SIDEBAR_DROPDOWN_ICON_SLOT_CLASS_NAME}>
                    <Radar className="h-4 w-4 text-muted-foreground" />
                  </div>
                  <span className="truncate font-medium text-foreground">
                    {text.sniffDesk.title}
                  </span>
                </DropdownMenuItem>
                <DropdownMenuItem
                  className={SIDEBAR_DROPDOWN_ITEM_CLASS_NAME}
                  onSelect={() => setActiveView("connections")}
                >
                  <div className={SIDEBAR_DROPDOWN_ICON_SLOT_CLASS_NAME}>
                    <Link2 className="h-4 w-4 text-muted-foreground" />
                  </div>
                  <span className="truncate font-medium text-foreground">
                    {text.views.connections}
                  </span>
                </DropdownMenuItem>
                <DropdownMenuItem
                  className={SIDEBAR_DROPDOWN_ITEM_CLASS_NAME}
                  onSelect={() => openDocumentation()}
                >
                  <div className={SIDEBAR_DROPDOWN_ICON_SLOT_CLASS_NAME}>
                    <FileText className="h-4 w-4 text-muted-foreground" />
                  </div>
                  <span className="truncate font-medium text-foreground">
                    {text.sidebar.documentation}
                  </span>
                </DropdownMenuItem>
                <DropdownMenuItem
                  className={SIDEBAR_DROPDOWN_ITEM_CLASS_NAME}
                  onSelect={() => showSettingsWindow.mutate()}
                >
                  <div className={SIDEBAR_DROPDOWN_ICON_SLOT_CLASS_NAME}>
                    <Settings2 className="h-4 w-4 text-muted-foreground" />
                  </div>
                  <span className="truncate font-medium text-foreground">
                    {text.actions.settings}
                  </span>
                </DropdownMenuItem>
                {localDownloadDirectory ? (
                  <DropdownMenuItem
                    className={SIDEBAR_DROPDOWN_ITEM_CLASS_NAME}
                    onSelect={() =>
                      void openPath.mutateAsync({
                        path: localDownloadDirectory,
                      })
                    }
                  >
                    <div className={SIDEBAR_DROPDOWN_ICON_SLOT_CLASS_NAME}>
                      <FolderOpen className="h-4 w-4 text-muted-foreground" />
                    </div>
                    <span className="truncate font-medium text-foreground">
                      {text.sidebar.openDownloads}
                    </span>
                  </DropdownMenuItem>
                ) : null}
                {userMenuUpdateItems.length > 0 ? (
                  <>
                    <DropdownMenuSeparator className="my-1" />
                    {userMenuUpdateItems.map((item) => (
                      <DropdownMenuItem
                        key={item.key}
                        className={SIDEBAR_DROPDOWN_ITEM_CLASS_NAME}
                        onSelect={item.onSelect}
                        disabled={item.disabled}
                      >
                        <div className={SIDEBAR_DROPDOWN_ICON_SLOT_CLASS_NAME}>
                          <item.Icon className="h-4 w-4 text-primary" />
                        </div>
                        <span className="min-w-0 flex-1 truncate font-medium text-foreground">
                          {item.label}
                        </span>
                        {item.meta ? (
                          <span className="max-w-[5rem] shrink-0 truncate rounded-md border border-border/70 bg-background/80 px-1.5 py-0.5 text-[11px] font-medium text-muted-foreground">
                            {item.meta}
                          </span>
                        ) : null}
                      </DropdownMenuItem>
                    ))}
                  </>
                ) : null}
              </div>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </aside>

      <main className="app-main-content relative flex min-w-0 flex-1 flex-col">
        <div
          className={cn(
            "app-main-view-viewport min-h-0 flex-1",
            activeView === "running" ||
            activeView === "connections" ||
              activeView === "completed" ||
              activeView === "listen" ||
              activeView === "petsGallery" ||
              activeView === "sniffDesk"
              ? "flex overflow-hidden"
              : "overflow-auto px-5 pb-5",
          )}
        >
          {activeView === "running" ? (
            <RunningPage
              text={text}
              operations={runningOperations}
              filesById={filesById}
              httpBaseURL={httpBaseURL}
              pet={activePet}
              petImageURL={activePetImageURL}
              petAnimation={runningPetAnimation}
              loading={
                runningOperations.length === 0 &&
                !runningQuery.isFetched &&
                runningQuery.isFetching
              }
              isWindows={isWindows}
              onNewDownload={() => openDownloadDialog()}
            />
          ) : activeView === "completed" ? (
            <CompletedPage
              text={text}
              libraries={libraries}
              terminalOperations={terminalOperations}
              httpBaseURL={httpBaseURL}
              pet={activePet}
              petImageURL={activePetImageURL}
              onTranscodeFile={openTranscodeDialog}
            />
          ) : activeView === "connections" ? (
            <AppSessionsSection />
          ) : activeView === "petsGallery" ? (
            <PetsGalleryPage
              text={text}
              settings={settings}
              navigation={petsGalleryNavigation}
              onOpenDocumentation={openDocumentation}
            />
          ) : activeView === "sniffDesk" ? (
            <SniffDeskPage
              text={text}
              active={activeView === "sniffDesk"}
              pet={activePet}
              petImageURL={activePetImageURL}
              httpBaseURL={httpBaseURL}
            />
          ) : null}
          <ListenPage
            active={activeView === "listen"}
            text={text}
            libraries={libraries}
            httpBaseURL={httpBaseURL}
            pet={activePet}
            petImageURL={activePetImageURL}
            controlCommand={listenControlCommand}
            onNowPlayingChange={setListenNowPlaying}
            onOpenConnections={() => setActiveView("connections")}
            onDownloadTrack={openDownloadDialog}
          />
        </div>
      </main>

      {welcomeOpen ? (
        <WelcomeScreen
          open={welcomeOpen}
          settings={settings}
          onComplete={() => {
            setSetupState({ completed: true });
            setDebugWelcomeOpen(false);
          }}
        />
      ) : null}
      <WhatsNewFeatureDialog
        blocked={welcomeOpen}
        language={settings?.language}
      />
      <NewTaskDialog
        open={newTaskDialogOpen}
        onOpenChange={setNewTaskDialogOpen}
        initialMode={newTaskDialogMode}
        initialUrl={prefilledDownloadURL}
        initialTranscodeSource={prefilledTranscodeSource}
        settings={settings}
        onOpenConnections={() => setActiveView("connections")}
        onOpenSniffDesk={() => setActiveView("sniffDesk")}
      />
    </div>
  );
}
