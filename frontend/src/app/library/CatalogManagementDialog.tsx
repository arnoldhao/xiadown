import {
  AlertTriangle,
  DatabaseBackup,
  FolderCog,
  FolderPlus,
  HeartPulse,
} from "lucide-react";
import { useQueryClient } from "@tanstack/react-query";
import * as React from "react";

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogScrollArea,
  DialogTitle,
} from "@/shared/ui/dialog";
import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n";
import { Button } from "@/shared/ui/button";
import { useRovingTabs } from "@/shared/ui/roving-tabs";
import {
  catalogKeys,
  openCatalogStorageRoot,
  useCancelCatalogStorageRootScan,
  useCatalogOverview,
  useCatalogStorageRootSyncStates,
  useCatalogStorageRoots,
  useCatalogStorageVolumes,
  useCheckCatalogStorageRoot,
  useRelocateCatalogStorageRoot,
  useRemoveCatalogStorageRoot,
  useSelectCatalogStorageRoot,
  useStartCatalogStorageRootScan,
  useUpdateCatalogStorageRoot,
} from "@/shared/query/catalog";
import type { CatalogStorageRoot } from "@/shared/contracts/catalog";
import { messageBus } from "@/shared/message";
import { formatBytes } from "@/shared/utils/formatBytes";

import {
  CatalogStorageOverview,
  CatalogStorageRootCard,
  isCatalogStorageRootMounted,
} from "./CatalogStorageSurfaces";
import type { LibraryWorkspaceLabels } from "./types";
import { LibraryDataManagement } from "./LibraryDataManagement";
import { createLibraryDataManagementLabels } from "./library-data-labels";

type ManagementSection = "summary" | "storage" | "data";

export interface CatalogManagementDialogProps {
  open: boolean;
  initialSection: ManagementSection;
  labels: LibraryWorkspaceLabels;
  onOpenChange: (open: boolean) => void;
}

function QueryError(props: { label: string; onRetry: () => void; retry: string }) {
  return (
    <div className="app-catalog-management__empty" role="alert">
      <AlertTriangle size={20} aria-hidden="true" />
      <span>{props.label}</span>
      <button type="button" onClick={props.onRetry}>{props.retry}</button>
    </div>
  );
}

export function CatalogManagementDialog(props: CatalogManagementDialogProps) {
  const { t } = useI18n();
  const queryClient = useQueryClient();
  const dataLabels = React.useMemo(() => createLibraryDataManagementLabels(t), [t]);
  const [section, setSection] = React.useState<ManagementSection>(props.initialSection);
  const panelId = React.useId();
  const overview = useCatalogOverview(props.open);
  const roots = useCatalogStorageRoots(props.open);
  const volumes = useCatalogStorageVolumes(props.open);
  const rootSyncStates = useCatalogStorageRootSyncStates(props.open);
  const checkRoot = useCheckCatalogStorageRoot();
  const startRootScan = useStartCatalogStorageRootScan();
  const cancelRootScan = useCancelCatalogStorageRootScan();
  const selectRoot = useSelectCatalogStorageRoot();
  const removeRoot = useRemoveCatalogStorageRoot();
  const relocateRoot = useRelocateCatalogStorageRoot();
  const updateRoot = useUpdateCatalogStorageRoot();
  const [rootActionError, setRootActionError] = React.useState("");
  const [removeCandidate, setRemoveCandidate] =
    React.useState<CatalogStorageRoot | null>(null);
  const [emojiPickerRootID, setEmojiPickerRootID] = React.useState("");
  const completedScansRef = React.useRef(new Set<string>());
  const refreshLibrarySurfaces = React.useCallback(async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: catalogKeys.all }),
      queryClient.invalidateQueries({ queryKey: ["library"] }),
    ]);
  }, [queryClient]);

  React.useEffect(() => {
    if (props.open) setSection(props.initialSection);
  }, [props.initialSection, props.open]);

  React.useEffect(() => {
    for (const state of rootSyncStates.data ?? []) {
      if (state.status !== "watching" || !state.finishedAt) continue;
      const key = `${state.rootId}:${state.generation}:${state.finishedAt}`;
      if (completedScansRef.current.has(key)) continue;
      completedScansRef.current.add(key);
      void Promise.all([
        queryClient.invalidateQueries({ queryKey: catalogKeys.storageRoots }),
        queryClient.invalidateQueries({ queryKey: catalogKeys.overview }),
        queryClient.invalidateQueries({ queryKey: ["catalog", "items"] }),
        queryClient.invalidateQueries({ queryKey: ["library"] }),
      ]);
    }
  }, [queryClient, rootSyncStates.data]);

  const labels = props.labels;
  const dataSectionLabel = t("xiadown.libraryData.managementTab");
  const sectionLabels: Record<ManagementSection, string> = {
    summary: labels.summary,
    storage: labels.storageRoots,
    data: dataSectionLabel,
  };
  const managementTabs = [
    {
      value: "summary" as const,
      label: labels.summary,
      icon: <HeartPulse size={15} aria-hidden="true" />,
    },
    {
      value: "storage" as const,
      label: labels.storageRoots,
      icon: <FolderCog size={15} aria-hidden="true" />,
    },
    {
      value: "data" as const,
      label: dataSectionLabel,
      icon: <DatabaseBackup size={15} aria-hidden="true" />,
    },
  ];
  const sectionTabs = useRovingTabs({
    items: managementTabs,
    value: section,
    onValueChange: setSection,
  });
  const activeTabId = `${panelId}-${section}`;
  const categoryCards: ReadonlyArray<readonly [string, number | string]> = overview.data ? [
    [
      labels.librarySize,
      overview.data.totalSizeBytes > 0
        ? formatBytes(overview.data.totalSizeBytes)
        : "0 B",
    ],
    [labels.all, overview.data.categories.all],
    [labels.video, overview.data.categories.video],
    [labels.audio, overview.data.categories.audio],
    [labels.books, overview.data.categories.books],
    [labels.images, overview.data.categories.images],
    [labels.others, overview.data.categories.others],
  ] : [];
  const catalogDisplayName = overview.data?.catalog.isDefault
    && overview.data.catalog.name.trim().toLocaleLowerCase() === "library"
    ? labels.library
    : overview.data?.catalog.name ?? "";
  const catalogStatusLabel = overview.data
    ? labels.statusLabel(overview.data.catalog.status)
    : "";

  const syncStatesByRoot = new Map(
    (rootSyncStates.data ?? []).map((state) => [state.rootId, state]),
  );
  const publishRootScanError = React.useCallback((error: unknown) => {
    messageBus.publishToast({
      intent: "danger",
      title: labels.rootScanFailed,
      description: String(error).replace(/^RuntimeError:\s*/i, ""),
      source: "library-storage-root",
    });
  }, [labels.rootScanFailed]);

  return (
    <>
      <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent
        className={cn(
          "app-catalog-management",
          section === "data" && "app-catalog-management--data",
        )}
        aria-describedby="catalog-management-description"
        data-active-section={section}
        onKeyDownCapture={(event) => {
          if (event.key !== "Escape" || !emojiPickerRootID) return;
          event.preventDefault();
          event.stopPropagation();
          setEmojiPickerRootID("");
        }}
      >
        <DialogHeader className="min-w-0 pr-8">
          <DialogTitle>{labels.managementTitle}</DialogTitle>
          <DialogDescription id="catalog-management-description">
            {section === "data" ? dataLabels.description : labels.managementDescription}
          </DialogDescription>
        </DialogHeader>

        <nav
          className="app-catalog-management__tabs"
          aria-label={labels.manage}
          role="tablist"
          aria-orientation="horizontal"
        >
          {managementTabs.map((tab, index) => (
            <button
              key={tab.value}
              ref={(node) => sectionTabs.setTabRef(index, node)}
              id={`${panelId}-${tab.value}`}
              type="button"
              role="tab"
              aria-controls={`${panelId}-${tab.value}-panel`}
              aria-selected={section === tab.value}
              tabIndex={sectionTabs.focusableIndex === index ? 0 : -1}
              data-active={section === tab.value}
              onClick={() => setSection(tab.value)}
              onKeyDown={(event) => sectionTabs.onKeyDown(event, index)}
            >
              {tab.icon}{tab.label}
            </button>
          ))}
        </nav>

        <DialogScrollArea
          className="app-catalog-management__body"
          data-management-section={section}
          id={`${panelId}-${section}-panel`}
          role="tabpanel"
          aria-label={sectionLabels[section]}
          aria-labelledby={activeTabId}
        >
          {section === "summary" ? (
            overview.isError || roots.isError || volumes.isError ? (
              <QueryError
                label={labels.loadFailed}
                retry={labels.retry}
                onRetry={() => {
                  void overview.refetch();
                  void roots.refetch();
                  void volumes.refetch();
                }}
              />
            ) : overview.isLoading || roots.isLoading || volumes.isLoading ||
              !overview.data ? (
              <div className="app-catalog-management__loading">{labels.loading}</div>
            ) : (
              <CatalogStorageOverview
                catalog={{
                  description: overview.data.catalog.description ?? "",
                  name: catalogDisplayName,
                  statusLabel: catalogStatusLabel,
                }}
                labels={labels}
                metrics={categoryCards}
                roots={roots.data ?? []}
                volumes={volumes.data ?? []}
              />
            )
          ) : null}

          {section === "storage" ? (
            roots.isError ? (
              <QueryError label={labels.loadFailed} retry={labels.retry} onRetry={() => void roots.refetch()} />
            ) : roots.isLoading ? (
              <div className="app-catalog-management__loading">{labels.loading}</div>
            ) : (
              <div className="app-catalog-roots-layout">
                <div className="app-dream-storage-root-heading">
                  <div className="app-dream-storage-root-heading__copy">
                    <h3>{labels.storageRoots}</h3>
                    <p>
                      {labels.storageRootsSummary(
                        (roots.data ?? []).length,
                        (roots.data ?? []).filter(
                          (root) => !isCatalogStorageRootMounted(root),
                        ).length,
                      )}
                    </p>
                  </div>
                  <Button
                    disabled={selectRoot.isPending}
                    onClick={() => selectRoot.mutate({})}
                    size="compact"
                    type="button"
                  >
                    <FolderPlus size={14} aria-hidden="true" />
                    {selectRoot.isPending
                      ? labels.selectingFolder
                      : labels.addStorageRoot}
                  </Button>
                </div>
                {selectRoot.error || removeRoot.error || relocateRoot.error ||
                rootActionError ? (
                  <p className="app-catalog-roots__error" role="alert">
                    {String(selectRoot.error || removeRoot.error ||
                      relocateRoot.error || rootActionError)}
                  </p>
                ) : null}
                {(roots.data ?? []).length === 0 ? (
                  <div className="app-catalog-management__empty"><FolderCog size={20} /><span>{labels.noStorageRoots}</span></div>
                ) : (
                  <div className="app-dream-storage-root-grid app-catalog-roots">
                    {(roots.data ?? []).map((root) => (
                      <CatalogStorageRootCard
                        busy={{
                          check:
                            checkRoot.isPending &&
                            checkRoot.variables === root.id,
                          relocate:
                            relocateRoot.isPending &&
                            relocateRoot.variables === root.id,
                          remove:
                            removeRoot.isPending &&
                            removeRoot.variables === root.id,
                          scan:
                            (startRootScan.isPending &&
                              startRootScan.variables === root.id) ||
                            (cancelRootScan.isPending &&
                              cancelRootScan.variables === root.id),
                          emoji:
                            updateRoot.isPending &&
                            updateRoot.variables?.id === root.id,
                        }}
                        emojiPickerOpen={emojiPickerRootID === root.id}
                        key={root.id}
                        labels={labels}
                        onCheck={() => checkRoot.mutate(root.id)}
                        onCancelScan={() =>
                          cancelRootScan.mutate(root.id, {
                            onError: publishRootScanError,
                          })}
                        onEmojiChange={(emoji) =>
                          updateRoot.mutate(
                            {
                              emoji,
                              id: root.id,
                              mode: root.mode,
                              name: root.name,
                            },
                            {
                              onError: (error) => {
                                messageBus.publishToast({
                                  intent: "danger",
                                  title: labels.editRoot,
                                  description: String(error).replace(
                                    /^RuntimeError:\s*/i,
                                    "",
                                  ),
                                  source: "library-storage-root",
                                });
                              },
                            },
                          )}
                        onEmojiPickerOpenChange={(open) =>
                          setEmojiPickerRootID((current) =>
                            open
                              ? root.id
                              : current === root.id
                                ? ""
                                : current,
                          )}
                        onOpen={() => {
                          setRootActionError("");
                          void openCatalogStorageRoot(root.id).catch((error) =>
                            setRootActionError(String(error)),
                          );
                        }}
                        onRelocate={() => relocateRoot.mutate(root.id)}
                        onRemove={() => setRemoveCandidate(root)}
                        onScan={() =>
                          startRootScan.mutate(root.id, {
                            onError: publishRootScanError,
                          })}
                        root={root}
                        syncState={syncStatesByRoot.get(root.id)}
                      />
                    ))}
                  </div>
                )}
              </div>
            )
          ) : null}

          {section === "data" ? (
            <LibraryDataManagement
              embedded
              labels={dataLabels}
              categoryLabel={labels.operationKindLabel}
              onMaintenanceChanged={refreshLibrarySurfaces}
            />
          ) : null}
        </DialogScrollArea>

        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={() => props.onOpenChange(false)}
          >
            {t("common.close")}
          </Button>
        </DialogFooter>
      </DialogContent>
      </Dialog>

      <Dialog
        open={Boolean(removeCandidate)}
        onOpenChange={(open) => {
          if (!open && !removeRoot.isPending) setRemoveCandidate(null);
        }}
      >
        <DialogContent
          className="app-catalog-root-remove-dialog"
          aria-describedby="catalog-root-remove-description"
        >
          <DialogHeader>
            <DialogTitle>{labels.removeRoot}</DialogTitle>
            <DialogDescription id="catalog-root-remove-description">
              {labels.removeRootConfirm}
            </DialogDescription>
          </DialogHeader>
          {removeCandidate ? (
            <code className="app-catalog-root-remove-dialog__path">
              {removeCandidate.locationPath || removeCandidate.path}
            </code>
          ) : null}
          {removeRoot.error ? (
            <p className="app-catalog-roots__error" role="alert">
              <AlertTriangle aria-hidden="true" />
              {String(removeRoot.error)}
            </p>
          ) : null}
          <DialogFooter>
            <Button
              disabled={removeRoot.isPending}
              onClick={() => setRemoveCandidate(null)}
              type="button"
              variant="outline"
            >
              {t("common.cancel")}
            </Button>
            <Button
              disabled={removeRoot.isPending || !removeCandidate}
              onClick={() => {
                if (!removeCandidate) return;
                removeRoot.mutate(removeCandidate.id, {
                  onSuccess: () => setRemoveCandidate(null),
                });
              }}
              type="button"
              variant="destructive"
            >
              {labels.removeRoot}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
