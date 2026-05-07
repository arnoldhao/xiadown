import { Events,System } from "@wailsio/runtime";
import {
ArrowUpCircle,
CassetteTape,
CheckCircle2,
FolderOpen,
Link2,
Plus,
RefreshCcw,
Settings2,
PawPrint,
Waves,
Wrench
} from "lucide-react";
import * as React from "react";

import {
DreamFMPage,
type DreamFMExternalCommand,
type DreamFMNowPlayingStatus
} from "@/app/main/DreamFM";
import { RunningPage } from "@/app/main/RunningPage";
import {
setPendingSettingsTab,
type XiaSettingsTabId,
} from "@/app/settings/sectionStorage";
import { PetsGalleryPage,type PetsGalleryNavigation } from "@/app/pets-gallery";
import { WindowControls } from "@/components/layout/WindowControls";
import {
ConnectorsSection
} from "@/features/settings/connectors";
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
useOpenLibraryPath
} from "@/shared/query/library";
import { useHttpBaseURL } from "@/shared/query/runtime";
import {
setWelcomeWindowChromeHidden,
useShowSettingsWindow
} from "@/shared/query/settings";
import { usePets } from "@/shared/query/pets";
import { useCurrentUserProfile } from "@/shared/query/system";
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
DREAM_FM_NOW_PLAYING_EVENT,
DREAM_FM_NOW_PLAYING_STORAGE_KEY,
DREAM_FM_TRAY_COMMAND_EVENT,
} from "@/app/main/dreamfm/catalog";
import { formatVersionBadge,normalizeDependencyVersion,resolveCompletedStatusLabel } from "@/app/main/helpers";
import { CORE_DEPENDENCIES,MAIN_SIDEBAR_ACTION_CLASS,MAIN_SIDEBAR_ICON_CLASS,SETUP_STORAGE_KEY,SIDEBAR_DROPDOWN_CONTENT_CLASS_NAME,SIDEBAR_DROPDOWN_ICON_SLOT_CLASS_NAME,SIDEBAR_DROPDOWN_ITEM_CLASS_NAME,useSetupState } from "@/app/main/main-constants";
import { NewTaskDialog } from "@/app/main/NewTaskDialog";
import { DreamFMNowPlayingMiniPlayer,DreamFMSidebarSourceBadge,resolveSidebarSurface,SidebarIconButton } from "@/app/main/sidebar";
import type { MainViewId,NewTaskDialogMode } from "@/app/main/types";
import {
WELCOME_DEBUG_EVENT,
WelcomeScreen,
type WelcomeDebugCommand,
type WelcomeDebugStep,
} from "@/app/main/WelcomeScreen";

const NOTIFIABLE_OPERATION_STATUSES = new Set(["succeeded", "failed"]);
const MAIN_NEW_DOWNLOAD_EVENT = "main:new-download";

function normalizeOperationStatus(status?: string) {
  return (status ?? "").trim().toLowerCase();
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
  const [setupState, setSetupState] = useSetupState();
  const [debugWelcomeOpen, setDebugWelcomeOpen] = React.useState(false);
  const [activeView, setActiveView] = React.useState<MainViewId>("running");
  const [petsGalleryNavigation, setPetsGalleryNavigation] =
    React.useState<PetsGalleryNavigation | null>(null);
  const [newTaskDialogOpen, setNewTaskDialogOpen] = React.useState(false);
  const [newTaskDialogMode, setNewTaskDialogMode] =
    React.useState<NewTaskDialogMode>("download");
  const [prefilledDownloadURL, setPrefilledDownloadURL] = React.useState("");
  const [dreamFMNowPlaying, setDreamFMNowPlaying] =
    React.useState<DreamFMNowPlayingStatus | null>(null);
  const [dreamFMControlCommand, setDreamFMControlCommand] =
    React.useState<DreamFMExternalCommand | null>(null);
  const dreamFMCommandIdRef = React.useRef(0);
  const dreamFMNotificationKeyRef = React.useRef("");
  const activeOperationSnapshotRef = React.useRef<Map<string, OperationListItemDTO>>(new Map());
  const notifiedOperationIdsRef = React.useRef<Set<string>>(new Set());

  const text = getXiaText(settings?.language);
  const appearance = readXiaAppearance(settings);
  const theme = resolveThemePack(appearance.themePackId);
  const isWindows = System.IsWindows();
  const welcomeOpen = !setupState.completed || debugWelcomeOpen;
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
  const visibleRunningOperations = runningOperations;
  const runningPetAnimation = useRunningPetAnimation(
    visibleRunningOperations,
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
  const shellTheme = activeView === "dreamfm" ? "dream" : "default";

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
    if (!dreamFMNowPlaying) {
      return;
    }
    try {
      localStorage.setItem(
        DREAM_FM_NOW_PLAYING_STORAGE_KEY,
        JSON.stringify(dreamFMNowPlaying),
      );
    } catch {
      // noop
    }
    void Events.Emit(DREAM_FM_NOW_PLAYING_EVENT, dreamFMNowPlaying);
  }, [dreamFMNowPlaying]);

  React.useEffect(() => {
    if (!dreamFMNowPlaying || dreamFMNowPlaying.state !== "playing") {
      return;
    }
    const title = dreamFMNowPlaying.title.trim();
    if (!title) {
      return;
    }
    const artist = dreamFMNowPlaying.subtitle.trim();
    const artworkURL = dreamFMNowPlaying.artworkURL.trim();
    const notificationKey = [
      dreamFMNowPlaying.mode,
      title,
      artist,
      artworkURL,
    ].join("::");
    if (dreamFMNotificationKeyRef.current === notificationKey) {
      return;
    }
    dreamFMNotificationKeyRef.current = notificationKey;

    void publishOSNotification({
      id: `dreamfm_${Date.now()}`,
      title,
      body: artist || text.dreamFm.nowPlaying,
      iconUrl: artworkURL,
      imageUrl: artworkURL,
      source: "Dream.fm",
      data: {
        source: "dreamfm",
        mode: dreamFMNowPlaying.mode,
        title,
        artist,
        artworkURL,
      },
    });
  }, [text.dreamFm.nowPlaying, dreamFMNowPlaying]);

  const openNewTaskDialog = React.useCallback((mode: NewTaskDialogMode, url = "") => {
    setNewTaskDialogMode(mode);
    setPrefilledDownloadURL(mode === "download" ? url : "");
    setNewTaskDialogOpen(true);
  }, []);

  const openDownloadDialog = React.useCallback((url = "") => {
    openNewTaskDialog("download", url);
  }, [openNewTaskDialog]);

  const sendDreamFMCommand = React.useCallback(
    (command: DreamFMExternalCommand["command"]) => {
      dreamFMCommandIdRef.current += 1;
      setDreamFMControlCommand({ id: dreamFMCommandIdRef.current, command });
    },
    [],
  );

  React.useEffect(() => {
    const offTrayCommand = Events.On(DREAM_FM_TRAY_COMMAND_EVENT, (event: any) => {
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
        sendDreamFMCommand(command);
      }
    });
    return () => {
      offTrayCommand();
    };
  }, [sendDreamFMCommand]);

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
              onSelect: () => openSettingsTab("dependencies"),
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
      className={cn(
        "app-main-shell relative flex h-screen overflow-hidden bg-background text-foreground",
        shellTheme === "default" && "app-dream-frame app-dream-window",
      )}
    >
      <aside
        className={cn(
          "app-main-sidebar relative z-40 flex w-[var(--app-main-sidebar-width)] shrink-0 flex-col items-center justify-between border-sidebar-border/70 px-3 pb-4 pt-3 text-sidebar-foreground",
          shellTheme === "default" && "app-main-default-sidebar",
          resolveSidebarSurface(theme.id, shellTheme),
        )}
      >
        <div className="flex flex-col items-center gap-5">
          <div className="min-h-[34px]" aria-hidden="true" />

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

        <div className="pointer-events-none absolute left-1/2 top-1/2 z-50 -translate-x-1/2 -translate-y-1/2">
          <div className="pointer-events-auto">
            <DreamFMNowPlayingMiniPlayer
              status={dreamFMNowPlaying}
              text={text}
              onOpen={() => setActiveView("dreamfm")}
              onToggle={() =>
                sendDreamFMCommand(
                  dreamFMNowPlaying?.state === "playing" ? "pause" : "play",
                )
              }
              onControlCommand={sendDreamFMCommand}
            />
          </div>
        </div>

        <div className="flex flex-col items-center gap-3">
          <SidebarIconButton
            label={text.views.dreamFM}
            active={activeView === "dreamfm"}
            onClick={() => setActiveView("dreamfm")}
          >
            <CassetteTape className={MAIN_SIDEBAR_ICON_CLASS} />
            <DreamFMSidebarSourceBadge status={dreamFMNowPlaying} />
          </SidebarIconButton>
          <SidebarIconButton
            label={text.views.connections}
            active={activeView === "connections"}
            onClick={() => setActiveView("connections")}
          >
            <Link2 className={MAIN_SIDEBAR_ICON_CLASS} />
          </SidebarIconButton>

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
              <div className="p-1">
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
                  onSelect={() => showSettingsWindow.mutate()}
                >
                  <div className={SIDEBAR_DROPDOWN_ICON_SLOT_CLASS_NAME}>
                    <Settings2 className="h-4 w-4 text-muted-foreground" />
                  </div>
                  <span className="truncate font-medium text-foreground">
                    {text.actions.openSettings}
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
        {activeView === "running" ? (
          <div className="app-main-page-header wails-drag flex min-h-[3.75rem] items-center justify-between gap-4 px-5 pb-3 pt-4">
            <div />
            <div
              className={cn(
                "flex items-center justify-end gap-3",
                isWindows && "min-w-[var(--app-windows-caption-control-width)]",
              )}
            >
              {isWindows ? <WindowControls platform="windows" /> : null}
            </div>
          </div>
        ) : null}

        <div
          className={cn(
            "app-main-view-viewport min-h-0 flex-1",
            activeView === "connections" ||
              activeView === "completed" ||
              activeView === "dreamfm" ||
              activeView === "petsGallery"
              ? "flex overflow-hidden"
              : activeView === "running"
                ? "overflow-hidden px-5 pb-5"
                : "overflow-auto px-5 pb-5",
          )}
        >
          {activeView === "running" ? (
            <RunningPage
              text={text}
              operations={visibleRunningOperations}
              filesById={filesById}
              httpBaseURL={httpBaseURL}
              pet={activePet}
              petImageURL={activePetImageURL}
              petAnimation={runningPetAnimation}
              loading={
                visibleRunningOperations.length === 0 &&
                !runningQuery.isFetched &&
                runningQuery.isFetching
              }
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
            />
          ) : activeView === "connections" ? (
            <ConnectorsSection />
          ) : activeView === "petsGallery" ? (
            <PetsGalleryPage
              text={text}
              settings={settings}
              navigation={petsGalleryNavigation}
            />
          ) : null}
          <DreamFMPage
            active={activeView === "dreamfm"}
            text={text}
            libraries={libraries}
            httpBaseURL={httpBaseURL}
            pet={activePet}
            petImageURL={activePetImageURL}
            controlCommand={dreamFMControlCommand}
            onNowPlayingChange={setDreamFMNowPlaying}
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
        settings={settings}
      />
    </div>
  );
}
