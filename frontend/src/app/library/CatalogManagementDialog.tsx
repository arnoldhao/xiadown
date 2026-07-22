import {
  AlertTriangle,
  DatabaseBackup,
  FolderCog,
  FolderPlus,
  HeartPulse,
  RefreshCw,
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
import { StatusBadge } from "@/shared/ui/status-badge";
import {
  catalogKeys,
  useCatalogOverview,
  useCatalogStorageRoots,
  useCheckCatalogStorageRoot,
  useSelectCatalogStorageRoot,
} from "@/shared/query/catalog";
import { formatBytes } from "@/shared/utils/formatBytes";

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

function formatDate(value: string, never: string) {
  const timestamp = Date.parse(value);
  return Number.isFinite(timestamp)
    ? new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" }).format(timestamp)
    : never;
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
  const checkRoot = useCheckCatalogStorageRoot();
  const selectRoot = useSelectCatalogStorageRoot();
  const [rootName, setRootName] = React.useState("");
  const [rootMode, setRootMode] = React.useState<"referenced" | "managed">("referenced");
  const refreshLibrarySurfaces = React.useCallback(async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: catalogKeys.all }),
      queryClient.invalidateQueries({ queryKey: ["library"] }),
    ]);
  }, [queryClient]);

  React.useEffect(() => {
    if (props.open) setSection(props.initialSection);
  }, [props.initialSection, props.open]);

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
  const categoryCards: ReadonlyArray<readonly [string, React.ReactNode]> = overview.data ? [
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
  const statusCards: ReadonlyArray<readonly [string, number]> = overview.data ? [
    [labels.activeItems, overview.data.statuses.active],
    [labels.needsReviewItems, overview.data.statuses.needsReview],
    [labels.missingItems, overview.data.statuses.missing],
    [labels.trashedItems, overview.data.statuses.trashed],
  ] : [];
  const healthCards: ReadonlyArray<readonly [string, number]> = overview.data ? [
    [labels.itemsWithoutAssets, overview.data.health.itemsWithoutAssets],
    [labels.unavailableFiles, overview.data.health.unavailableAssetFiles],
    [labels.offlineRoots, overview.data.health.offlineStorageRoots],
    [labels.rootErrors, overview.data.health.storageRootsWithErrors],
  ] : [];
  const catalogDisplayName = overview.data?.catalog.isDefault
    && overview.data.catalog.name.trim().toLocaleLowerCase() === "library"
    ? labels.library
    : overview.data?.catalog.name ?? "";
  const catalogStatusLabel = overview.data
    ? labels.statusLabel(overview.data.catalog.status)
    : "";

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent
        className={cn(
          "app-catalog-management",
          section === "data" && "app-catalog-management--data",
        )}
        aria-describedby="catalog-management-description"
        data-active-section={section}
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
            overview.isError ? (
              <QueryError label={labels.loadFailed} retry={labels.retry} onRetry={() => void overview.refetch()} />
            ) : overview.isLoading || !overview.data ? (
              <div className="app-catalog-management__loading">{labels.loading}</div>
            ) : (
              <div className="app-catalog-management__sections">
                <section>
                  <div className="app-catalog-management__section-title">
                    <div><strong>{catalogDisplayName}</strong><span>{overview.data.catalog.description}</span></div>
                    <StatusBadge className="app-catalog-management__status" tone="neutral">
                      {catalogStatusLabel}
                    </StatusBadge>
                  </div>
                  <div className="app-catalog-management__metrics app-catalog-management__metrics--categories">
                    {categoryCards.map(([label, count]) => <div key={label}><strong>{count}</strong><span>{label}</span></div>)}
                  </div>
                </section>
                <section>
                  <h3>{labels.itemStatuses}</h3>
                  <div className="app-catalog-management__metrics">
                    {statusCards.map(([label, count]) => <div key={label}><strong>{count}</strong><span>{label}</span></div>)}
                  </div>
                </section>
                <section>
                  <h3>{labels.catalogHealth}</h3>
                  <div className="app-catalog-management__metrics">
                    {healthCards.map(([label, count]) => (
                      <div key={label} data-warning={count > 0}><strong>{count}</strong><span>{label}</span></div>
                    ))}
                  </div>
                </section>
              </div>
            )
          ) : null}

          {section === "storage" ? (
            roots.isError ? (
              <QueryError label={labels.loadFailed} retry={labels.retry} onRetry={() => void roots.refetch()} />
            ) : roots.isLoading ? (
              <div className="app-catalog-management__loading">{labels.loading}</div>
            ) : (
              <div className="app-catalog-roots-layout">
                <form
                  className="app-catalog-roots__add"
                  onSubmit={(event) => {
                    event.preventDefault();
                    if (!rootName.trim() || selectRoot.isPending) return;
                    selectRoot.mutate(
                      { name: rootName.trim(), mode: rootMode },
                      { onSuccess: () => setRootName("") },
                    );
                  }}
                >
                  <input
                    value={rootName}
                    onChange={(event) => setRootName(event.currentTarget.value)}
                    placeholder={labels.storageRootName}
                    aria-label={labels.storageRootName}
                  />
                  <select
                    value={rootMode}
                    onChange={(event) => setRootMode(event.currentTarget.value as "referenced" | "managed")}
                    aria-label={labels.rootMode}
                  >
                    <option value="referenced">{labels.referencedMode}</option>
                    <option value="managed">{labels.managedMode}</option>
                  </select>
                  <button type="submit" disabled={!rootName.trim() || selectRoot.isPending}>
                    <FolderPlus size={14} />
                    {selectRoot.isPending ? labels.selectingFolder : labels.addStorageRoot}
                  </button>
                </form>
                {selectRoot.error ? <p className="app-catalog-roots__error" role="alert">{String(selectRoot.error)}</p> : null}
                {(roots.data ?? []).length === 0 ? (
                  <div className="app-catalog-management__empty"><FolderCog size={20} /><span>{labels.noStorageRoots}</span></div>
                ) : (
                  <div className="app-catalog-roots">
                    {(roots.data ?? []).map((root) => (
                      <article key={root.id}>
                        <div className="app-catalog-roots__head">
                          <div><strong>{root.name}</strong><span>{root.status} · {labels.rootMode}: {root.mode}</span></div>
                          <button
                            type="button"
                            disabled={checkRoot.isPending}
                            onClick={() => checkRoot.mutate(root.id)}
                          >
                            <RefreshCw size={14} className={checkRoot.isPending ? "app-motion-spin" : undefined} />
                            {checkRoot.isPending ? labels.checkingRoot : labels.checkRoot}
                          </button>
                        </div>
                        <code title={root.path}>{root.path}</code>
                        <p>{labels.lastChecked}: {formatDate(root.lastCheckedAt ?? "", labels.never)}</p>
                        {root.lastError ? <p className="app-catalog-roots__error">{root.lastError}</p> : null}
                      </article>
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
  );
}
