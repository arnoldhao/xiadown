import {
  ArchiveRestore,
  ChevronLeft,
  ChevronRight,
  File,
  LibraryBig,
  ListTodo,
  LoaderCircle,
  RefreshCw,
  Trash2,
  type LucideIcon,
} from "lucide-react";
import * as React from "react";

import type {
  DeletedLibraryItemDTO,
  DeletedLibraryItemKind,
} from "@/shared/contracts/library";
import type { CatalogItem } from "@/shared/contracts/catalog";
import { useI18n, type TFunction } from "@/shared/i18n";
import {
  useCompleteDeletedLibraryItems,
  useListLibraries,
  useRestoreDeletedLibraryItem,
} from "@/shared/query/library";
import {
  LIBRARY_CATALOG_ACTOR_ID,
  useCompleteCatalogItems,
  useRestoreCatalogItem,
} from "@/shared/query/catalog";
import { Button } from "@/shared/ui/button";
import { StatusBadge } from "@/shared/ui/status-badge";
import { formatBytes } from "@/shared/utils/formatBytes";

import { createLibraryWorkspaceLabels } from "./types";

const DELETED_PAGE_SIZE = 50;

export type LibraryDeletedCompanionItemKind = DeletedLibraryItemKind | "catalog_item";

export interface LibraryDeletedCompanionItem {
  id: string;
  kind: LibraryDeletedCompanionItemKind;
  source: string;
  libraryId: string;
  title: string;
  category: string;
  status: string;
  deletedAt: string;
  canRestore: boolean;
  revision?: number;
  detail: DeletedLibraryItemDTO["detail"] & { catalogItem?: CatalogItem };
}

const KIND_ICONS = {
  task: ListTodo,
  file: File,
  catalog_item: LibraryBig,
} satisfies Record<LibraryDeletedCompanionItemKind, LucideIcon>;

export interface LibraryDeletedCompanionLabels {
  title: string;
  description: string;
  emptyTitle: string;
  emptyDescription: string;
  itemKind: Record<LibraryDeletedCompanionItemKind, string>;
  deletedAt: string;
  source: string;
  category: string;
  status: string;
  library: string;
  location: string;
  size: string;
  format: string;
  revision: string;
  outputCount: string;
  taskAudit: string;
  auditPreserved: string;
  details: string;
  back: string;
  restore: string;
  restoring: string;
  restoreUnavailable: string;
  loading: string;
  loadFailed: string;
  retry: string;
  previousPage: string;
  nextPage: string;
  count: (visible: number, total: number) => string;
}

export function createLibraryDeletedCompanionLabels(
  t: TFunction,
): LibraryDeletedCompanionLabels {
  const label = (key: string) => t(`xiadown.libraryCatalog.${key}`);
  return {
    title: label("deletedItemsTitle"),
    description: label("deletedItemsDescription"),
    emptyTitle: label("deletedItemsEmptyTitle"),
    emptyDescription: label("deletedItemsEmptyDescription"),
    itemKind: {
      task: label("deletedTask"),
      file: label("deletedFile"),
      catalog_item: label("deletedCatalogItem"),
    },
    deletedAt: label("deletedAt"),
    source: label("source"),
    category: label("category"),
    status: label("status"),
    library: label("library"),
    location: label("location"),
    size: label("size"),
    format: label("format"),
    revision: label("revision"),
    outputCount: label("deletedOutputCount"),
    taskAudit: label("deletedTaskAudit"),
    auditPreserved: label("deletedAuditPreserved"),
    details: label("itemDetails"),
    back: label("backToDeletedItems"),
    restore: label("restoreItem"),
    restoring: label("restoringDeletedItem"),
    restoreUnavailable: label("deletedRestoreUnavailable"),
    loading: label("loading"),
    loadFailed: label("deletedItemsLoadFailed"),
    retry: label("retry"),
    previousPage: t("xiadown.completed.previousPage"),
    nextPage: t("xiadown.completed.nextPage"),
    count: (visible, total) => label("deletedItemsCount")
      .replace("{visible}", String(visible))
      .replace("{total}", String(total)),
  };
}

function mergeDeletedLabels(
  base: LibraryDeletedCompanionLabels,
  overrides?: Partial<LibraryDeletedCompanionLabels>,
): LibraryDeletedCompanionLabels {
  return {
    ...base,
    ...overrides,
    itemKind: { ...base.itemKind, ...overrides?.itemKind },
  };
}

function validDateTime(value: string) {
  const timestamp = Date.parse(value);
  return Number.isFinite(timestamp) ? new Date(timestamp) : null;
}

function formatDeletedDate(value: string, locale: string) {
  const date = validDateTime(value);
  if (!date) return value || "–";
  return new Intl.DateTimeFormat(locale, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(date);
}

export function libraryDeletedCompanionItemKey(
  item: Pick<LibraryDeletedCompanionItem, "kind" | "id">,
) {
  return `${item.kind}:${item.id}`;
}

export function mergeDeletedLibraryItems(
  libraryItems: readonly DeletedLibraryItemDTO[],
  catalogItems: readonly CatalogItem[],
  deletedLegacyProjectionFileIds: ReadonlySet<string> = new Set(),
): LibraryDeletedCompanionItem[] {
  const deletedLegacyFileIds = new Set(
    libraryItems
      .filter((item) => item.kind === "file")
      .map((item) => item.id.trim())
      .filter(Boolean),
  );
  deletedLegacyProjectionFileIds.forEach((fileId) => {
    const normalized = fileId.trim();
    if (normalized) deletedLegacyFileIds.add(normalized);
  });
  const projectedCatalogItems: LibraryDeletedCompanionItem[] = catalogItems
    // A soft-deleted legacy file can also have a trashed Catalog projection.
    // Keep the file tombstone because its restore path reconciles both models.
    .filter((item) => {
      const primaryFileId = item.primaryFileId?.trim();
      return !primaryFileId || !deletedLegacyFileIds.has(primaryFileId);
    })
    .map((item) => ({
      id: item.id,
      kind: "catalog_item",
      source: "catalog_trash",
      libraryId: item.catalogId,
      title: item.title,
      category: item.category,
      status: item.status,
      deletedAt: item.trashedAt || item.updatedAt,
      canRestore: true,
      revision: item.revision,
      detail: { catalogItem: item },
    }));
  return [
    ...libraryItems.map((item): LibraryDeletedCompanionItem => ({ ...item })),
    ...projectedCatalogItems,
  ].sort((left, right) => {
    const timeDifference = (Date.parse(right.deletedAt) || 0) - (Date.parse(left.deletedAt) || 0);
    if (timeDifference !== 0) return timeDifference;
    return libraryDeletedCompanionItemKey(left).localeCompare(
      libraryDeletedCompanionItemKey(right),
    );
  });
}

function deletedItemLocation(item: LibraryDeletedCompanionItem) {
  return item.detail.file?.storage.localPath ?? "";
}

function deletedItemFormat(item: LibraryDeletedCompanionItem) {
  return item.detail.file?.media?.format ?? item.detail.catalogItem?.format ?? "";
}

function deletedItemSize(item: LibraryDeletedCompanionItem) {
  return item.detail.file?.media?.sizeBytes ?? item.detail.catalogItem?.sizeBytes;
}

interface DeletedFact {
  label: string;
  value: React.ReactNode;
  title?: string;
}

function DeletedFacts(props: {
  item: LibraryDeletedCompanionItem;
  labels: LibraryDeletedCompanionLabels;
  locale: string;
  categoryValue: string;
  statusValue: string;
}) {
  const item = props.item;
  const taskHistory = item.detail.taskHistory;
  const location = deletedItemLocation(item);
  const format = deletedItemFormat(item);
  const size = deletedItemSize(item);
  const revision = item.revision ?? item.detail.catalogItem?.revision;
  const possibleFacts: Array<DeletedFact | null> = [
    { label: props.labels.status, value: props.statusValue },
    { label: props.labels.category, value: props.categoryValue },
    { label: props.labels.deletedAt, value: formatDeletedDate(item.deletedAt, props.locale) },
    {
      label: props.labels.source,
      value: item.kind === "task" ? props.labels.taskAudit : props.labels.itemKind[item.kind],
    },
    item.libraryId ? { label: props.labels.library, value: item.libraryId, title: item.libraryId } : null,
    location ? { label: props.labels.location, value: location, title: location } : null,
    format ? { label: props.labels.format, value: format } : null,
    typeof size === "number" && size >= 0
      ? { label: props.labels.size, value: formatBytes(size) }
      : null,
    typeof revision === "number"
      ? { label: props.labels.revision, value: revision }
      : null,
    taskHistory
      ? { label: props.labels.outputCount, value: taskHistory.metrics?.fileCount ?? 0 }
      : null,
  ];
  const facts = possibleFacts.filter((fact): fact is DeletedFact => fact !== null);

  return (
    <dl className="app-library-deleted__facts">
      {facts.map((fact) => (
        <div className="app-library-deleted__fact" key={fact.label}>
          <dt>{fact.label}</dt>
          <dd title={fact.title}>{fact.value}</dd>
        </div>
      ))}
    </dl>
  );
}

function DeletedItemIcon(props: { kind: LibraryDeletedCompanionItemKind; large?: boolean }) {
  const Icon = KIND_ICONS[props.kind];
  return (
    <span
      className="app-library-deleted__icon"
      data-kind={props.kind}
      data-size={props.large ? "large" : "small"}
      aria-hidden="true"
    >
      <Icon />
    </span>
  );
}

export interface LibraryDeletedCompanionViewProps {
  items: readonly LibraryDeletedCompanionItem[];
  total: number;
  offset?: number;
  pageSize?: number;
  loading?: boolean;
  loadError?: boolean;
  labels?: Partial<LibraryDeletedCompanionLabels>;
  restoringKey?: string;
  initialSelectedKey?: string;
  onRetry?: () => void;
  onPageChange?: (offset: number) => void;
  onRestore?: (item: LibraryDeletedCompanionItem) => Promise<unknown> | unknown;
}

export function LibraryDeletedCompanionView(
  props: LibraryDeletedCompanionViewProps,
) {
  const { t, language } = useI18n();
  const labels = React.useMemo(
    () => mergeDeletedLabels(createLibraryDeletedCompanionLabels(t), props.labels),
    [props.labels, t],
  );
  const workspaceLabels = React.useMemo(
    () => createLibraryWorkspaceLabels(t, language),
    [language, t],
  );
  const [selectedKey, setSelectedKey] = React.useState(props.initialSelectedKey ?? "");
  const [actionError, setActionError] = React.useState("");
  // Never expose stale detail actions while one of the archive sources failed:
  // the merged view is only safe to mutate when all three sources agree.
  const selectedItem = props.loadError
    ? null
    : props.items.find(
        (item) => libraryDeletedCompanionItemKey(item) === selectedKey,
      ) ?? null;
  const pageSize = props.pageSize ?? DELETED_PAGE_SIZE;
  const offset = props.offset ?? 0;
  const previousOffset = Math.max(0, offset - pageSize);
  const nextOffset = offset + pageSize;
  const hasPrevious = offset > 0;
  const hasNext = nextOffset < props.total;

  React.useEffect(() => {
    if (selectedKey && !selectedItem && !props.loading) setSelectedKey("");
  }, [props.loading, selectedItem, selectedKey]);

  const restore = async (item: LibraryDeletedCompanionItem) => {
    setActionError("");
    try {
      await props.onRestore?.(item);
      setSelectedKey("");
    } catch (error) {
      setActionError(error instanceof Error ? error.message : String(error));
    }
  };

  return (
    <section
      className="app-library-deleted"
      data-companion-scroll-owner="library-deleted"
      data-view={selectedItem ? "detail" : "list"}
      aria-label={labels.title}
    >
      {selectedItem ? (
        <>
          <div className="app-library-deleted__detail-nav">
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={() => {
                setActionError("");
                setSelectedKey("");
              }}
            >
              <ChevronLeft aria-hidden="true" />
              {labels.back}
            </Button>
          </div>
          <div className="app-library-deleted__detail">
            <div className="app-library-deleted__hero">
              <DeletedItemIcon kind={selectedItem.kind} large />
              <div>
                <p>{labels.itemKind[selectedItem.kind]}</p>
                <h2>{selectedItem.title}</h2>
                <StatusBadge tone="muted" icon={<Trash2 aria-hidden="true" />}>
                  {workspaceLabels.statusLabel(selectedItem.status || "deleted")}
                </StatusBadge>
              </div>
            </div>
            <section className="app-library-deleted__detail-section" aria-label={labels.details}>
              <DeletedFacts
                item={selectedItem}
                labels={labels}
                locale={language}
                statusValue={workspaceLabels.statusLabel(selectedItem.status || "deleted")}
                categoryValue={selectedItem.kind === "task"
                  ? workspaceLabels.operationKindLabel(selectedItem.category)
                  : workspaceLabels.catalogValueLabel(selectedItem.category)}
              />
            </section>
            {selectedItem.kind === "task" ? (
              <p className="app-library-deleted__audit-note">
                {labels.auditPreserved}
              </p>
            ) : null}
            {!selectedItem.canRestore ? (
              <p className="app-library-deleted__restore-note">
                {labels.restoreUnavailable}
              </p>
            ) : null}
            {actionError ? (
              <p className="app-library-deleted__action-error" role="alert">{actionError}</p>
            ) : null}
            {selectedItem.canRestore ? (
              <div className="app-library-deleted__detail-actions">
                <Button
                  type="button"
                  variant="outline"
                  disabled={props.restoringKey === libraryDeletedCompanionItemKey(selectedItem)}
                  onClick={() => void restore(selectedItem)}
                >
                  {props.restoringKey === libraryDeletedCompanionItemKey(selectedItem)
                    ? <LoaderCircle className="app-motion-spin" aria-hidden="true" />
                    : <ArchiveRestore aria-hidden="true" />}
                  {props.restoringKey === libraryDeletedCompanionItemKey(selectedItem) ? labels.restoring : labels.restore}
                </Button>
              </div>
            ) : null}
          </div>
        </>
      ) : (
        <>
          <header className="app-library-deleted__intro">
            <div>
              <h2>{labels.title}</h2>
              <p>{labels.description}</p>
            </div>
            <Button
              type="button"
              variant="ghost"
              size="icon"
              aria-label={labels.retry}
              disabled={props.loading}
              onClick={props.onRetry}
            >
              <RefreshCw className={props.loading ? "app-motion-spin" : undefined} />
            </Button>
          </header>
          {props.loading && props.items.length === 0 ? (
            <div className="app-library-deleted__state" role="status">
              <LoaderCircle className="app-motion-spin" aria-hidden="true" />
              <p>{labels.loading}</p>
            </div>
          ) : props.loadError ? (
            <div className="app-library-deleted__state" role="alert">
              <Trash2 aria-hidden="true" />
              <p>{labels.loadFailed}</p>
              <Button type="button" variant="outline" size="sm" onClick={props.onRetry}>
                {labels.retry}
              </Button>
            </div>
          ) : props.items.length === 0 ? (
            <div className="app-library-deleted__state app-library-deleted__state--empty">
              <Trash2 aria-hidden="true" />
              <h2>{labels.emptyTitle}</h2>
              <p>{labels.emptyDescription}</p>
            </div>
          ) : (
            <>
              <div className="app-library-deleted__list">
                {props.items.map((item) => (
                  <button
                    type="button"
                    className="app-library-deleted__item"
                    key={`${item.kind}:${item.id}`}
                    onClick={() => {
                      setActionError("");
                      setSelectedKey(libraryDeletedCompanionItemKey(item));
                    }}
                  >
                    <DeletedItemIcon kind={item.kind} />
                    <span className="app-library-deleted__item-copy">
                      <strong>{item.title}</strong>
                      <span>
                        {labels.itemKind[item.kind]}
                        <span aria-hidden="true"> · </span>
                        <time dateTime={item.deletedAt}>
                          {formatDeletedDate(item.deletedAt, language)}
                        </time>
                      </span>
                    </span>
                    <ChevronRight className="app-library-deleted__item-chevron" aria-hidden="true" />
                  </button>
                ))}
              </div>
              <footer className="app-library-deleted__pagination">
                <span>{labels.count(props.items.length, props.total)}</span>
                <div>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    aria-label={labels.previousPage}
                    disabled={!hasPrevious || props.loading}
                    onClick={() => props.onPageChange?.(previousOffset)}
                  >
                    <ChevronLeft aria-hidden="true" />
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    aria-label={labels.nextPage}
                    disabled={!hasNext || props.loading}
                    onClick={() => props.onPageChange?.(nextOffset)}
                  >
                    <ChevronRight aria-hidden="true" />
                  </Button>
                </div>
              </footer>
            </>
          )}
        </>
      )}
    </section>
  );
}

export interface LibraryDeletedCompanionProps {
  labels?: Partial<LibraryDeletedCompanionLabels>;
}

export function LibraryDeletedCompanion(props: LibraryDeletedCompanionProps) {
  const [offset, setOffset] = React.useState(0);
  const libraryQuery = useCompleteDeletedLibraryItems();
  const librariesQuery = useListLibraries();
  const catalogQuery = useCompleteCatalogItems({
    status: "trashed",
    excludeTrashed: false,
  });
  const restoreItem = useRestoreDeletedLibraryItem();
  const restoreCatalogItem = useRestoreCatalogItem();
  const deletedLegacyProjectionFileIds = React.useMemo(
    () => new Set(
      (librariesQuery.data ?? []).flatMap((library) => library.files)
        .filter((file) => file.state.deleted || ["deleted", "trashed", "purged"].includes(
          file.state.status.trim().toLocaleLowerCase(),
        ))
        .map((file) => file.id),
    ),
    [librariesQuery.data],
  );
  const items = React.useMemo(
    () => mergeDeletedLibraryItems(
      libraryQuery.data?.items ?? [],
      catalogQuery.data?.items ?? [],
      deletedLegacyProjectionFileIds,
    ),
    [catalogQuery.data?.items, deletedLegacyProjectionFileIds, libraryQuery.data?.items],
  );
  const visibleItems = React.useMemo(
    () => items.slice(offset, offset + DELETED_PAGE_SIZE),
    [items, offset],
  );

  React.useEffect(() => {
    if (offset === 0 || offset < items.length) return;
    setOffset(Math.max(0, Math.floor(Math.max(0, items.length - 1) / DELETED_PAGE_SIZE) * DELETED_PAGE_SIZE));
  }, [items.length, offset]);

  const restoringKey = restoreCatalogItem.isPending
    ? `catalog_item:${restoreCatalogItem.variables?.id}`
    : restoreItem.isPending && restoreItem.variables
      ? libraryDeletedCompanionItemKey(restoreItem.variables)
      : undefined;
  return (
    <LibraryDeletedCompanionView
      items={visibleItems}
      total={items.length}
      offset={offset}
      pageSize={DELETED_PAGE_SIZE}
      loading={libraryQuery.isFetching || catalogQuery.isFetching || librariesQuery.isFetching}
      loadError={libraryQuery.isError || catalogQuery.isError || librariesQuery.isError}
      labels={props.labels}
      restoringKey={restoringKey}
      onRetry={() => void Promise.all([
        libraryQuery.refetch(),
        catalogQuery.refetch(),
        librariesQuery.refetch(),
      ])}
      onPageChange={setOffset}
      onRestore={(item) => item.kind === "catalog_item"
        ? restoreCatalogItem.mutateAsync({
            id: item.id,
            expectedRevision: item.revision ?? 0,
            actorId: LIBRARY_CATALOG_ACTOR_ID,
          })
        : restoreItem.mutateAsync({ kind: item.kind, id: item.id })}
    />
  );
}
