import { System } from "@wailsio/runtime";
import {
ChevronRight,
Loader2,
WandSparkles
} from "lucide-react";
import * as React from "react";

import { OnlineSpriteGallery,SegmentedTabs,SpriteDetailHeader,SpriteGallery,SpriteMetricSegments,SpriteSheetEditor,SpriteStudioPreview,buildSpriteArchivePath,cloneGrid,defaultGrid,gridKey,normalizeGrid,resolveSpriteError,resolveSpriteImportFailure,type OnlineSpriteInstallState } from "@/app/sprite-studio/components";
import { WindowControls } from "@/components/layout/WindowControls";
import type { SpriteAnimation } from "@/shared/sprites/animation";
import {
mergeSpritePreferences,
readSpritePreferences
} from "@/features/sprites/shared";
import { getXiaText } from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
import type { OnlineSpriteCatalogItem } from "@/shared/contracts/online-sprites";
import type { Settings } from "@/shared/contracts/settings";
import type { Sprite,SpriteSliceGrid } from "@/shared/contracts/sprites";
import { messageBus } from "@/shared/message/store";
import { useOnlineSpriteCatalog } from "@/shared/query/online-sprites";
import { useHttpBaseURL } from "@/shared/query/runtime";
import { useUpdateSettings } from "@/shared/query/settings";
import {
useDeleteSprite,
useExportSprite,
useInstallSpriteFromURL,
useImportSprite,
useInspectSpriteSource,
useSpriteManifest,
useSprites,
useUpdateSprite,
useUpdateSpriteSlices,
} from "@/shared/query/sprites";
import { useCurrentUserProfile } from "@/shared/query/system";
import { openFileDialog } from "@/shared/utils/dialogHelpers";
import {
buildAssetPreviewURL,
getPathBaseName,
resolveDialogPath,
stripPathExtension,
} from "@/shared/utils/resourceHelpers";

type XiaText = ReturnType<typeof getXiaText>;

export type SpriteStudioNavigation = {
  action: "gallery" | "edit" | "import";
  spriteId?: string;
  nonce: number;
};

type StudioTab = "edit" | "preview";
type StudioMode = "home" | "local" | "editor";
type SaveState = "idle" | "saving" | "saved" | "error";
type SpriteMetadataDraft = {
  name: string;
  authorDisplayName: string;
  version: string;
  description: string;
};

export function SpriteStudioPage(props: {
  text: XiaText;
  settings: Settings | null;
  navigation: SpriteStudioNavigation | null;
}) {
  const { text, navigation, settings } = props;
  const { data: sprites = [], isLoading } = useSprites();
  const onlineCatalogQuery = useOnlineSpriteCatalog(text.locale);
  const { data: httpBaseURL = "" } = useHttpBaseURL();
  const { data: currentUserProfile } = useCurrentUserProfile();
  const inspectSprite = useInspectSpriteSource();
  const exportSprite = useExportSprite();
  const importSprite = useImportSprite();
  const installSpriteFromURL = useInstallSpriteFromURL();
  const updateSprite = useUpdateSprite();
  const deleteSprite = useDeleteSprite();
  const updateSettings = useUpdateSettings();
  const updateSpriteSlices = useUpdateSpriteSlices();

  const readySprites = React.useMemo(
    () => sprites.filter((sprite) => sprite.status === "ready"),
    [sprites],
  );
  const defaultSpriteId = React.useMemo(
    () => readSpritePreferences(settings).activeSpriteId,
    [settings],
  );
  const [mode, setMode] = React.useState<StudioMode>("home");
  const [selectedSpriteId, setSelectedSpriteId] = React.useState("");
  const [activeTab, setActiveTab] = React.useState<StudioTab>("edit");
  const [previewAnimation, setPreviewAnimation] = React.useState<SpriteAnimation>("greeting");
  const [sliceGrid, setSliceGrid] = React.useState<SpriteSliceGrid | null>(null);
  const [saveState, setSaveState] = React.useState<SaveState>("idle");
  const [onlineInstallStates, setOnlineInstallStates] = React.useState<Record<string, OnlineSpriteInstallState | undefined>>({});
  const handledNavigationRef = React.useRef(0);
  const saveTimerRef = React.useRef<number | null>(null);
  const lastSavedGridKeyRef = React.useRef("");
  const savingRef = React.useRef(false);
  const queuedGridRef = React.useRef<SpriteSliceGrid | null>(null);
  const performSaveRef = React.useRef<(grid: SpriteSliceGrid) => void>(() => undefined);

  const selectedSprite = React.useMemo(
    () => readySprites.find((sprite) => sprite.id === selectedSpriteId) ?? null,
    [readySprites, selectedSpriteId],
  );
  const installedSpriteIds = React.useMemo(
    () => new Set(sprites.map((sprite) => sprite.id)),
    [sprites],
  );
  const onlineCatalog = onlineCatalogQuery.data ?? {
    schemaVersion: 1,
    updatedAt: "",
    source: "remote" as const,
    categories: [],
    items: [],
  };
  const manifestQuery = useSpriteManifest(mode === "editor" ? selectedSpriteId : "");
  const manifest = manifestQuery.data ?? null;
  const selectedManifestKey = manifest ? `${manifest.id}:${manifest.updatedAt}` : "";
  const defaultImportAuthorDisplayName =
    currentUserProfile?.displayName || currentUserProfile?.username || "";
  const isWindows = System.IsWindows();

  const openEditor = React.useCallback((spriteId: string) => {
    setSelectedSpriteId(spriteId);
    setMode("editor");
    setActiveTab("edit");
    setPreviewAnimation("greeting");
  }, []);

  const openHome = React.useCallback(() => {
    setMode("home");
    setSelectedSpriteId("");
    setActiveTab("edit");
    setPreviewAnimation("greeting");
    setSliceGrid(null);
    setSaveState("idle");
  }, []);

  const openLocalGallery = React.useCallback(() => {
    setMode("local");
    setSelectedSpriteId("");
    setActiveTab("edit");
    setPreviewAnimation("greeting");
    setSliceGrid(null);
    setSaveState("idle");
  }, []);

  const performSave = React.useCallback(
    (grid: SpriteSliceGrid) => {
      if (!selectedSpriteId || !manifest?.canEdit) {
        return;
      }

      const key = gridKey(grid);
      if (key === lastSavedGridKeyRef.current) {
        setSaveState("saved");
        return;
      }

      if (savingRef.current) {
        queuedGridRef.current = cloneGrid(grid);
        setSaveState("saving");
        return;
      }

      savingRef.current = true;
      setSaveState("saving");
      void updateSpriteSlices
        .mutateAsync({ id: selectedSpriteId, sliceGrid: cloneGrid(grid) })
        .then(() => {
          lastSavedGridKeyRef.current = key;
          setSaveState("saved");
        })
        .catch((error) => {
          setSaveState("error");
          messageBus.publishToast({
            intent: "danger",
            title: text.spriteStudio.saveFailed,
            description: resolveSpriteError(error),
          });
        })
        .finally(() => {
          savingRef.current = false;
          const queued = queuedGridRef.current;
          queuedGridRef.current = null;
          if (queued && gridKey(queued) !== lastSavedGridKeyRef.current) {
            performSaveRef.current(queued);
          }
        });
    },
    [text.spriteStudio.saveFailed, manifest?.canEdit, selectedSpriteId, updateSpriteSlices],
  );

  React.useEffect(() => {
    performSaveRef.current = performSave;
  }, [performSave]);

  const scheduleSave = React.useCallback(
    (grid: SpriteSliceGrid, immediate: boolean) => {
      if (!manifest?.canEdit) {
        return;
      }
      if (saveTimerRef.current !== null) {
        window.clearTimeout(saveTimerRef.current);
        saveTimerRef.current = null;
      }
      if (immediate) {
        performSave(grid);
        return;
      }
      setSaveState("saving");
      saveTimerRef.current = window.setTimeout(() => {
        saveTimerRef.current = null;
        performSave(grid);
      }, 360);
    },
    [manifest?.canEdit, performSave],
  );

  const handleGridChange = React.useCallback(
    (nextGrid: SpriteSliceGrid, immediate = false) => {
      const cloned = cloneGrid(nextGrid);
      setSliceGrid(cloned);
      scheduleSave(cloned, immediate);
    },
    [scheduleSave],
  );

  const handleImportSprite = React.useCallback(async () => {
    setMode("local");
    const selection = await openFileDialog({
      Title: text.spriteStudio.importTitle,
      AllowsOtherFiletypes: false,
      CanChooseDirectories: false,
      CanChooseFiles: true,
      Filters: [{ DisplayName: "Sprite files", Pattern: "*.png;*.zip" }],
    });
    const path = resolveDialogPath(selection);
    if (!path) {
      return;
    }

    try {
      const draft = await inspectSprite.mutateAsync({ path });
      if (draft.status !== "ready") {
        const failure = resolveSpriteImportFailure(draft.validationMessage || text.settings.sprites.invalid, text);
        messageBus.publishToast({
          intent: "danger",
          title: failure.title,
          description: failure.description,
        });
        return;
      }

      const sprite = await importSprite.mutateAsync({
        path,
        name: draft.name || stripPathExtension(getPathBaseName(path)),
        description: draft.description,
        authorDisplayName: draft.authorDisplayName || defaultImportAuthorDisplayName,
        version: draft.version || "1.0",
      });

      await updateSettings.mutateAsync({
        appearanceConfig: mergeSpritePreferences(settings, {
          activeSpriteId: sprite.id,
        }),
      });
      openEditor(sprite.id);
    } catch (error) {
      const failure = resolveSpriteImportFailure(error, text);
      messageBus.publishToast({
        intent: "danger",
        title: failure.title,
        description: failure.description,
      });
    }
  }, [
    text.settings.sprites.invalid,
    text.spriteStudio.importTitle,
    defaultImportAuthorDisplayName,
    importSprite,
    inspectSprite,
    openEditor,
    settings,
    updateSettings,
  ]);

  const handleExportSprite = React.useCallback(
    async (sprite: Sprite) => {
      const selection = await openFileDialog({
        Title: text.settings.sprites.exportTitle,
        CanChooseDirectories: true,
        CanChooseFiles: false,
        CanCreateDirectories: true,
        AllowsOtherFiletypes: true,
        ButtonText: text.settings.sprites.exportAction,
      });
      const directoryPath = resolveDialogPath(selection);
      if (!directoryPath) {
        return;
      }

      try {
        await exportSprite.mutateAsync({
          id: sprite.id,
          outputPath: buildSpriteArchivePath(directoryPath, sprite.name),
        });
        messageBus.publishToast({
          intent: "success",
          title: text.settings.sprites.exportSucceeded,
        });
      } catch (error) {
        messageBus.publishToast({
          intent: "danger",
          title: text.settings.sprites.exportTitle,
          description: resolveSpriteError(error),
        });
      }
    },
    [
      text.settings.sprites.exportAction,
      text.settings.sprites.exportSucceeded,
      text.settings.sprites.exportTitle,
      exportSprite,
    ],
  );

  const handleSetDefaultSprite = React.useCallback(
    async (sprite: Sprite) => {
      await updateSettings.mutateAsync({
        appearanceConfig: mergeSpritePreferences(settings, {
          activeSpriteId: sprite.id,
        }),
      });
      messageBus.publishToast({
        intent: "success",
        title: text.settings.sprites.setDefaultSucceeded,
      });
    },
    [text.settings.sprites.setDefaultSucceeded, settings, updateSettings],
  );

  const handleSaveSpriteMetadata = React.useCallback(
    async (sprite: Sprite, draft: SpriteMetadataDraft) => {
      const updated = await updateSprite.mutateAsync({
        id: sprite.id,
        name: draft.name,
        description: draft.description,
        authorDisplayName: sprite.sourceType === "zip" ? sprite.author.displayName : draft.authorDisplayName,
        version: draft.version,
      });
      setSelectedSpriteId(updated.id);
      await manifestQuery.refetch();
    },
    [manifestQuery, updateSprite],
  );

  const handleDeleteSprite = React.useCallback(
    async (sprite: Sprite) => {
      await deleteSprite.mutateAsync({ id: sprite.id });

      const nextActiveSpriteId = readySprites.find((item) => item.id !== sprite.id)?.id ?? "";
      let settingsError = "";
      try {
        await updateSettings.mutateAsync({
          appearanceConfig: mergeSpritePreferences(settings, {
            activeSpriteId: nextActiveSpriteId,
          }),
        });
      } catch (error) {
        settingsError = resolveSpriteError(error);
      }

      messageBus.publishToast({
        intent: settingsError ? "warning" : "success",
        title: text.settings.sprites.deleteSucceeded,
        description: settingsError || undefined,
      });
      openLocalGallery();
    },
    [
      text.settings.sprites.deleteSucceeded,
      deleteSprite,
      openLocalGallery,
      readySprites,
      settings,
      updateSettings,
    ],
  );

  React.useEffect(
    () => () => {
      if (saveTimerRef.current !== null) {
        window.clearTimeout(saveTimerRef.current);
      }
    },
    [],
  );

  React.useEffect(() => {
    if (!manifest) {
      return;
    }
    const nextGrid = normalizeGrid(manifest.sliceGrid, manifest.columns, manifest.rows, manifest.sheetWidth, manifest.sheetHeight);
    setSliceGrid(nextGrid);
    lastSavedGridKeyRef.current = gridKey(nextGrid);
    setSaveState("idle");
  }, [selectedManifestKey, manifest]);

  React.useEffect(() => {
    if (!navigation || navigation.nonce === handledNavigationRef.current) {
      return;
    }
    handledNavigationRef.current = navigation.nonce;
    if (navigation.action === "import") {
      setMode("local");
      void handleImportSprite();
      return;
    }
    if (navigation.action === "edit" && navigation.spriteId) {
      openEditor(navigation.spriteId);
      return;
    }
    openHome();
  }, [handleImportSprite, navigation, openEditor, openHome]);

  const handleInstallOnlineSprite = React.useCallback(
    async (item: OnlineSpriteCatalogItem) => {
      const setInstallState = (state: OnlineSpriteInstallState) => {
        setOnlineInstallStates((current) => ({ ...current, [item.id]: state }));
      };

      try {
        setInstallState({ status: "downloading", progress: 8 });
        const progressTimer = window.setInterval(() => {
          setOnlineInstallStates((current) => {
            const currentState = current[item.id];
            if (!currentState || currentState.status !== "downloading") {
              return current;
            }
            return {
              ...current,
              [item.id]: {
                ...currentState,
                progress: Math.min(72, currentState.progress + 8),
              },
            };
          });
        }, 320);
        try {
          await installSpriteFromURL.mutateAsync({
            url: item.downloadUrl,
            sha256: item.sha256 || undefined,
            size: item.size || undefined,
            name: item.name,
            description: item.description,
            authorDisplayName: item.authorDisplayName,
            version: item.version,
          });
        } finally {
          window.clearInterval(progressTimer);
        }
        setInstallState({ status: "importing", progress: 86 });
        await wait(180);
        setInstallState({ status: "installed", progress: 100 });
        messageBus.publishToast({
          intent: "success",
          title: text.spriteStudio.installSucceeded,
          description: item.name,
        });
      } catch (error) {
        setInstallState({ status: "error", progress: 0, error: resolveSpriteError(error) });
        messageBus.publishToast({
          intent: "danger",
          title: text.spriteStudio.installFailed,
          description: resolveSpriteError(error),
        });
      }
    },
    [installSpriteFromURL, text.spriteStudio.installFailed, text.spriteStudio.installSucceeded],
  );

  const editorReady = Boolean(mode === "editor" && selectedSprite && manifest && sliceGrid);
  const spriteImageUrl =
    manifest && httpBaseURL
      ? buildAssetPreviewURL(httpBaseURL, manifest.spritePath, manifest.updatedAt)
      : "";
  const displayImageUrl =
    selectedSprite && httpBaseURL
      ? buildAssetPreviewURL(httpBaseURL, selectedSprite.spritePath, selectedSprite.updatedAt)
      : "";

  return (
    <div className="flex h-full w-full min-w-0 flex-1 basis-full flex-col overflow-hidden bg-[radial-gradient(circle_at_50%_-10%,hsl(var(--primary)/0.08),transparent_38%),linear-gradient(180deg,hsl(var(--background)/0.96),hsl(var(--muted)/0.30))]">
      <header
        className={cn(
          "wails-drag grid min-h-[3.75rem] shrink-0 items-center gap-4 border-b border-white/20 bg-background/66 px-5 py-3 shadow-[inset_0_-1px_0_hsl(var(--foreground)/0.05)] backdrop-blur-2xl dark:border-white/10",
          isWindows
            ? "grid-cols-[minmax(var(--app-windows-caption-control-width),1fr)_auto_minmax(var(--app-windows-caption-control-width),1fr)]"
            : "grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)]",
        )}
      >
        <nav className="flex min-w-0 items-center gap-1.5 overflow-hidden text-sm" aria-label={text.spriteStudio.breadcrumb}>
          <button
            type="button"
            className={cn(
              "wails-no-drag min-w-0 truncate rounded-full px-2.5 py-1 font-semibold text-foreground outline-none transition hover:bg-background/72",
              mode === "home" && "pointer-events-none",
            )}
            onClick={openHome}
          >
            <WandSparkles className="mr-1.5 inline h-4 w-4 align-[-0.15em] text-primary" />
            {text.spriteStudio.title}
          </button>
          {mode === "local" || mode === "editor" ? (
            <>
              <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground" />
              <button
                type="button"
                className={cn(
                  "wails-no-drag min-w-0 truncate rounded-full px-2.5 py-1 font-medium text-muted-foreground outline-none transition hover:bg-background/72 hover:text-foreground",
                  mode === "local" && "pointer-events-none",
                )}
                onClick={openLocalGallery}
              >
                {text.spriteStudio.localGallery}
              </button>
            </>
          ) : null}
          {selectedSprite ? (
            <>
              <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground" />
              <span className="min-w-0 truncate rounded-full px-2.5 py-1 font-medium text-muted-foreground">
                {selectedSprite.name}
              </span>
            </>
          ) : null}
        </nav>

        <div className="wails-no-drag flex min-w-0 items-center justify-center">
          {mode === "editor" ? <SegmentedTabs activeTab={activeTab} text={text} onChange={setActiveTab} /> : null}
        </div>

        <div className="wails-no-drag flex min-w-0 items-center justify-end">
          {isWindows ? <WindowControls platform="windows" /> : null}
        </div>
      </header>

      <div className="min-h-0 flex-1 overflow-auto px-5 py-5">
        {mode === "home" ? (
          <div className="flex min-w-0 flex-col gap-8">
            <SpriteGallery
              text={text}
              sprites={readySprites}
              httpBaseURL={httpBaseURL}
              isLoading={isLoading}
              importPending={inspectSprite.isPending || importSprite.isPending || updateSettings.isPending}
              defaultSpriteId={defaultSpriteId}
              defaultPending={updateSettings.isPending}
              onImportSprite={handleImportSprite}
              onOpenSprite={openEditor}
              onSetDefaultSprite={handleSetDefaultSprite}
              exportPending={exportSprite.isPending}
              deletePending={deleteSprite.isPending || updateSettings.isPending}
              onExportSprite={handleExportSprite}
              onDeleteSprite={handleDeleteSprite}
            />
            <div className="h-px bg-foreground/10" />
            <OnlineSpriteGallery
              text={text}
              items={onlineCatalog.items}
              categories={onlineCatalog.categories}
              installedSpriteIds={installedSpriteIds}
              defaultSpriteId={defaultSpriteId}
              installStates={onlineInstallStates}
              isLoading={onlineCatalogQuery.isLoading}
              isFetching={onlineCatalogQuery.isFetching}
              hasError={onlineCatalogQuery.isError && !onlineCatalogQuery.data}
              onRefresh={() => void onlineCatalogQuery.refetch()}
              onInstallSprite={handleInstallOnlineSprite}
            />
          </div>
        ) : mode === "local" ? (
          <SpriteGallery
            text={text}
            sprites={readySprites}
            httpBaseURL={httpBaseURL}
            isLoading={isLoading}
            importPending={inspectSprite.isPending || importSprite.isPending || updateSettings.isPending}
            defaultSpriteId={defaultSpriteId}
            defaultPending={updateSettings.isPending}
            onImportSprite={handleImportSprite}
            onOpenSprite={openEditor}
            onSetDefaultSprite={handleSetDefaultSprite}
            exportPending={exportSprite.isPending}
            deletePending={deleteSprite.isPending || updateSettings.isPending}
            onExportSprite={handleExportSprite}
            onDeleteSprite={handleDeleteSprite}
          />
        ) : editorReady && manifest && sliceGrid && selectedSprite ? (
          <div className="flex min-h-full w-full min-w-0 flex-col gap-4">
            <SpriteDetailHeader
              text={text}
              sprite={selectedSprite}
              manifest={manifest}
              activeTab={activeTab}
              saveState={saveState}
              exportPending={exportSprite.isPending}
              updatePending={updateSprite.isPending}
              deletePending={deleteSprite.isPending || updateSettings.isPending}
              previewAnimation={previewAnimation}
              onPreviewAnimationChange={setPreviewAnimation}
              onExportSprite={() => void handleExportSprite(selectedSprite)}
              onSaveSpriteMetadata={(draft) => handleSaveSpriteMetadata(selectedSprite, draft)}
              onDeleteSprite={() => handleDeleteSprite(selectedSprite)}
              onResetGrid={() =>
                handleGridChange(defaultGrid(manifest.columns, manifest.rows, manifest.sheetWidth, manifest.sheetHeight), true)
              }
            />

            {activeTab === "edit" ? (
              <SpriteSheetEditor
                text={text}
                manifest={manifest}
                imageUrl={spriteImageUrl}
                grid={sliceGrid}
                onGridChange={handleGridChange}
              />
            ) : (
              <SpriteStudioPreview
                text={text}
                manifest={manifest}
                imageUrl={spriteImageUrl}
                displayImageUrl={displayImageUrl}
                sprite={selectedSprite}
                grid={sliceGrid}
                animation={previewAnimation}
              />
            )}

            <div className="mt-auto flex justify-center pt-1">
              <SpriteMetricSegments text={text} manifest={manifest} grid={sliceGrid} />
            </div>
          </div>
        ) : (
          <div className="flex h-full min-h-[22rem] items-center justify-center text-muted-foreground">
            {manifestQuery.isFetching ? <Loader2 className="h-5 w-5 animate-spin" /> : text.spriteStudio.empty}
          </div>
        )}
      </div>
    </div>
  );
}

function wait(ms: number) {
  return new Promise((resolve) => window.setTimeout(resolve, ms));
}
