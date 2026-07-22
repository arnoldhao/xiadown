import {
  ArrowDown,
  ArrowUp,
  Folder,
  ListPlus,
  LoaderCircle,
  Pencil,
  Plus,
  Save,
  Trash2,
  X,
} from "lucide-react";
import * as React from "react";

import {
  useRSSAddCollectionItems,
  useRSSCreateCategory,
  useRSSCreateCollection,
  useRSSDeleteCategory,
  useRSSDeleteCollection,
  useRSSReorderCategories,
  useRSSUpdateCategory,
  useRSSUpdateCollection,
  useRSSUpdateSubscription,
} from "./api";
import { runRSSBulkAction } from "./rss-bulk-actions";
import {
  moveOrganizationID,
  selectionAfterRSSBulkAction,
} from "./rss-organization-utils";
import type {
  RSSCategory,
  RSSCollection,
  RSSSubscription,
} from "./types";
import { useI18n } from "@/shared/i18n";
import { Button } from "@/shared/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/shared/ui/dialog";
import { Input } from "@/shared/ui/input";
import { Select } from "@/shared/ui/select";

import "./rss-organization-manager.css";

export interface RSSOrganizationManagerProps {
  categories: readonly RSSCategory[];
  collections: readonly RSSCollection[];
  subscriptions: readonly RSSSubscription[];
  selectedSubscriptionIDs: ReadonlySet<string>;
  onSelectionChange: (selection: Set<string>) => void;
}

type OrganizationKind = "category" | "collection";

interface RenameTarget {
  kind: OrganizationKind;
  id: string;
  title: string;
}

type DeleteTarget = RenameTarget;

export function RSSOrganizationManager({
  categories,
  collections,
  subscriptions,
  selectedSubscriptionIDs,
  onSelectionChange,
}: RSSOrganizationManagerProps) {
  const { t } = useI18n();
  const createCategory = useRSSCreateCategory();
  const updateCategory = useRSSUpdateCategory();
  const deleteCategory = useRSSDeleteCategory();
  const reorderCategories = useRSSReorderCategories();
  const updateSubscription = useRSSUpdateSubscription();
  const createCollection = useRSSCreateCollection();
  const updateCollection = useRSSUpdateCollection();
  const deleteCollection = useRSSDeleteCollection();
  const addCollectionItems = useRSSAddCollectionItems();

  const [categoryTitle, setCategoryTitle] = React.useState("");
  const [collectionTitle, setCollectionTitle] = React.useState("");
  const [moveCategoryID, setMoveCategoryID] = React.useState("");
  const [renameTarget, setRenameTarget] = React.useState<RenameTarget | null>(null);
  const [deleteTarget, setDeleteTarget] = React.useState<DeleteTarget | null>(null);
  const [pendingRows, setPendingRows] = React.useState<Set<string>>(() => new Set());
  const [rowErrors, setRowErrors] = React.useState<Record<string, string>>({});
  const [movingSelection, setMovingSelection] = React.useState(false);
  const [feedbackKey, setFeedbackKey] = React.useState("");
  const managerSurfaceRef = React.useRef<HTMLElement | null>(null);
  const deleteReturnFocusTargetRef = React.useRef<HTMLButtonElement | null>(null);

  const subscriptionIDs = React.useMemo(
    () => new Set(subscriptions.map((subscription) => subscription.id)),
    [subscriptions],
  );
  const selectedIDs = React.useMemo(
    () => [...selectedSubscriptionIDs].filter((id) => subscriptionIDs.has(id)),
    [selectedSubscriptionIDs, subscriptionIDs],
  );
  const subscriptionCollections = React.useMemo(
    () => collections.filter((collection) => collection.kind === "subscriptions"),
    [collections],
  );

  const runRowAction = async (key: string, action: () => Promise<unknown>) => {
    setPendingRows((current) => new Set(current).add(key));
    setRowErrors((current) => omitRecordKey(current, key));
    try {
      await action();
      return true;
    } catch {
      setRowErrors((current) => ({
        ...current,
        [key]: "xiadown.rss.organizationActionFailed",
      }));
      return false;
    } finally {
      setPendingRows((current) => {
        const next = new Set(current);
        next.delete(key);
        return next;
      });
    }
  };

  const submitCategory = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const title = categoryTitle.trim();
    if (!title || createCategory.isPending) return;
    try {
      await createCategory.mutateAsync({ title });
      setCategoryTitle("");
      setFeedbackKey("xiadown.rss.categoryCreated");
    } catch {
      // The create form renders its bounded error below.
    }
  };

  const submitCollection = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const title = collectionTitle.trim();
    if (!title || createCollection.isPending) return;
    try {
      await createCollection.mutateAsync({
        title,
        kind: "subscriptions",
        viewType: "auto",
      });
      setCollectionTitle("");
      setFeedbackKey("xiadown.rss.customListCreated");
    } catch {
      // The create form renders its bounded error below.
    }
  };

  const submitRename = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!renameTarget) return;
    const title = renameTarget.title.trim();
    if (!title) return;
    const key = organizationRowKey(renameTarget.kind, renameTarget.id);
    const saved = await runRowAction(key, async () => {
      switch (renameTarget.kind) {
        case "category":
          await updateCategory.mutateAsync({ id: renameTarget.id, title });
          break;
        case "collection":
          await updateCollection.mutateAsync({ id: renameTarget.id, title });
          break;
      }
    });
    if (saved) setRenameTarget(null);
  };

  const moveSelectedSubscriptions = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (selectedIDs.length === 0 || movingSelection) return;
    setMovingSelection(true);
    setFeedbackKey("");
    const result = await runRSSBulkAction(
      selectedIDs,
      (id) => updateSubscription.mutateAsync({ id, categoryId: moveCategoryID }),
    );
    onSelectionChange(selectionAfterRSSBulkAction(result));
    setFeedbackKey(result.failures.length > 0
      ? "xiadown.rss.organizationPartialFailure"
      : "xiadown.rss.subscriptionsMoved");
    setMovingSelection(false);
  };

  const addSelectionToCollection = async (collection: RSSCollection) => {
    if (selectedIDs.length === 0) return;
    const key = organizationRowKey("collection", collection.id);
    const saved = await runRowAction(key, () => addCollectionItems.mutateAsync({
      id: collection.id,
      itemIds: selectedIDs,
    }));
    if (saved) {
      onSelectionChange(new Set());
      setFeedbackKey("xiadown.rss.subscriptionsAddedToList");
    }
  };

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    try {
      switch (deleteTarget.kind) {
        case "category":
          await deleteCategory.mutateAsync({ id: deleteTarget.id });
          break;
        case "collection":
          await deleteCollection.mutateAsync({ id: deleteTarget.id });
          break;
      }
      setDeleteTarget(null);
      setFeedbackKey("xiadown.rss.organizationItemDeleted");
    } catch {
      // The confirmation dialog keeps the target and renders the error.
    }
  };

  const deletePending = deleteTarget?.kind === "category"
    ? deleteCategory.isPending
    : deleteTarget?.kind === "collection"
      ? deleteCollection.isPending
      : false;
  const deleteError = deleteTarget?.kind === "category"
    ? deleteCategory.isError
    : deleteTarget?.kind === "collection"
      ? deleteCollection.isError
      : false;

  return (
    <section
      aria-label={t("xiadown.rss.organizationTitle")}
      className="rss-organization-manager"
      ref={managerSurfaceRef}
      tabIndex={-1}
    >
      <div aria-atomic="true" aria-live="polite" className="rss-organization-manager__status" role="status">
        {feedbackKey ? t(feedbackKey) : null}
      </div>

      <div className="rss-organization-manager__grid">
        <OrganizationSection
          description={t("xiadown.rss.categoriesDescription")}
          icon={<Folder aria-hidden="true" />}
          title={t("xiadown.rss.categories")}
        >
          <form className="rss-organization-manager__create" onSubmit={submitCategory}>
            <Input
              aria-label={t("xiadown.rss.categoryName")}
              onChange={(event) => setCategoryTitle(event.currentTarget.value)}
              placeholder={t("xiadown.rss.categoryName")}
              value={categoryTitle}
            />
            <Button disabled={!categoryTitle.trim() || createCategory.isPending} size="compact" type="submit">
              {createCategory.isPending ? <LoaderCircle className="app-motion-spin" /> : <Plus />}
              {t("xiadown.rss.createCategory")}
            </Button>
          </form>
          {createCategory.isError ? <OrganizationError text={t("xiadown.rss.organizationActionFailed")} /> : null}
          <form className="rss-organization-manager__bulk" onSubmit={moveSelectedSubscriptions}>
            <Select
              aria-label={t("xiadown.rss.moveSelectedToCategory")}
              onChange={(event) => setMoveCategoryID(event.currentTarget.value)}
              value={moveCategoryID}
            >
              <option value="">{t("xiadown.rss.uncategorized")}</option>
              {categories.map((category) => (
                <option key={category.id} value={category.id}>{category.title}</option>
              ))}
            </Select>
            <Button disabled={selectedIDs.length === 0 || movingSelection} size="compact" type="submit" variant="outline">
              {movingSelection ? <LoaderCircle className="app-motion-spin" /> : <Folder />}
              {t("xiadown.rss.moveSelected")}
            </Button>
          </form>
          {categories.length === 0 ? (
            <OrganizationEmpty text={t("xiadown.rss.noCategories")} />
          ) : (
            <div className="rss-organization-manager__items" role="list">
              {categories.map((category, index) => {
                const key = organizationRowKey("category", category.id);
                return (
                  <OrganizationRow
                    count={category.subscriptionCount}
                    editing={renameTarget?.kind === "category" && renameTarget.id === category.id}
                    error={rowErrors[key] ? t(rowErrors[key]) : ""}
                    key={category.id}
                    pending={pendingRows.has(key)}
                    title={renameTarget?.kind === "category" && renameTarget.id === category.id
                      ? renameTarget.title
                      : category.title}
                    unreadCount={category.unreadCount}
                    onCancelRename={() => setRenameTarget(null)}
                    onDelete={(returnFocusTarget) => {
                      deleteReturnFocusTargetRef.current = returnFocusTarget;
                      setDeleteTarget({ kind: "category", id: category.id, title: category.title });
                    }}
                    onMoveDown={() => void runRowAction(key, () => reorderCategories.mutateAsync({
                      ids: moveOrganizationID(categories.map((item) => item.id), category.id, 1),
                    }))}
                    onMoveUp={() => void runRowAction(key, () => reorderCategories.mutateAsync({
                      ids: moveOrganizationID(categories.map((item) => item.id), category.id, -1),
                    }))}
                    onRename={() => setRenameTarget({ kind: "category", id: category.id, title: category.title })}
                    onRenameTitle={(title) => setRenameTarget((current) => current ? { ...current, title } : current)}
                    onSubmitRename={submitRename}
                    moveDownDisabled={index === categories.length - 1 || reorderCategories.isPending}
                    moveUpDisabled={index === 0 || reorderCategories.isPending}
                  />
                );
              })}
            </div>
          )}
        </OrganizationSection>

        <OrganizationSection
          description={t("xiadown.rss.customListsDescription")}
          icon={<ListPlus aria-hidden="true" />}
          title={t("xiadown.rss.customLists")}
        >
          <form className="rss-organization-manager__create" onSubmit={submitCollection}>
            <Input
              aria-label={t("xiadown.rss.customListName")}
              onChange={(event) => setCollectionTitle(event.currentTarget.value)}
              placeholder={t("xiadown.rss.customListName")}
              value={collectionTitle}
            />
            <Button disabled={!collectionTitle.trim() || createCollection.isPending} size="compact" type="submit">
              {createCollection.isPending ? <LoaderCircle className="app-motion-spin" /> : <Plus />}
              {t("xiadown.rss.createCustomList")}
            </Button>
          </form>
          {createCollection.isError ? <OrganizationError text={t("xiadown.rss.organizationActionFailed")} /> : null}
          {subscriptionCollections.length === 0 ? (
            <OrganizationEmpty text={t("xiadown.rss.noCustomLists")} />
          ) : (
            <div className="rss-organization-manager__items" role="list">
              {subscriptionCollections.map((collection) => {
                const key = organizationRowKey("collection", collection.id);
                return (
                  <OrganizationRow
                    count={collection.itemCount}
                    editing={renameTarget?.kind === "collection" && renameTarget.id === collection.id}
                    error={rowErrors[key] ? t(rowErrors[key]) : ""}
                    key={collection.id}
                    pending={pendingRows.has(key)}
                    title={renameTarget?.kind === "collection" && renameTarget.id === collection.id
                      ? renameTarget.title
                      : collection.title}
                    unreadCount={collection.unreadCount}
                    trailingAction={(
                      <Button
                        aria-label={`${t("xiadown.rss.addSelectedToList")} ${collection.title}`}
                        disabled={selectedIDs.length === 0 || pendingRows.has(key)}
                        onClick={() => void addSelectionToCollection(collection)}
                        size="icon"
                        title={t("xiadown.rss.addSelectedToList")}
                        type="button"
                        variant="ghost"
                      >
                        <ListPlus />
                      </Button>
                    )}
                    onCancelRename={() => setRenameTarget(null)}
                    onDelete={(returnFocusTarget) => {
                      deleteReturnFocusTargetRef.current = returnFocusTarget;
                      setDeleteTarget({ kind: "collection", id: collection.id, title: collection.title });
                    }}
                    onRename={() => setRenameTarget({ kind: "collection", id: collection.id, title: collection.title })}
                    onRenameTitle={(title) => setRenameTarget((current) => current ? { ...current, title } : current)}
                    onSubmitRename={submitRename}
                  />
                );
              })}
            </div>
          )}
        </OrganizationSection>

      </div>

      <Dialog
        open={Boolean(deleteTarget)}
        onOpenChange={(open) => {
          if (!open && !deletePending) setDeleteTarget(null);
        }}
      >
        <DialogContent
          className="rss-organization-manager__confirm"
          onCloseAutoFocus={(event) => {
            const preferredTarget = deleteReturnFocusTargetRef.current;
            deleteReturnFocusTargetRef.current = null;
            const target = preferredTarget?.isConnected
              ? preferredTarget
              : managerSurfaceRef.current;
            if (!target) return;
            event.preventDefault();
            target.focus();
          }}
          showCloseButton={false}
        >
          <DialogTitle>{deleteTarget ? t(deleteTitleKey(deleteTarget.kind)) : null}</DialogTitle>
          <DialogDescription className="rss-organization-manager__confirm-description">
            <span>{deleteTarget ? t(deleteDescriptionKey(deleteTarget.kind)) : null}</span>
            {deleteTarget ? <strong>{deleteTarget.title}</strong> : null}
          </DialogDescription>
          {deleteError ? <OrganizationError text={t("xiadown.rss.organizationActionFailed")} /> : null}
          <div className="rss-organization-manager__confirm-actions">
            <DialogClose asChild>
              <Button disabled={deletePending} type="button" variant="outline">
                {t("xiadown.rss.cancel")}
              </Button>
            </DialogClose>
            <Button disabled={!deleteTarget || deletePending} onClick={() => void confirmDelete()} type="button" variant="destructive">
              {deletePending ? <LoaderCircle className="app-motion-spin" /> : <Trash2 />}
              {t("xiadown.rss.delete")}
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </section>
  );
}

function OrganizationSection({
  children,
  description,
  icon,
  title,
}: {
  children: React.ReactNode;
  description: string;
  icon: React.ReactNode;
  title: string;
}) {
  return (
    <section className="rss-organization-manager__section app-dream-card app-motion-surface">
      <header>
        <span className="rss-organization-manager__section-icon">{icon}</span>
        <span>
          <h3>{title}</h3>
          <p>{description}</p>
        </span>
      </header>
      {children}
    </section>
  );
}

function OrganizationRow({
  count,
  editing,
  error,
  icon,
  moveDownDisabled,
  moveUpDisabled,
  pending,
  subtitle,
  title,
  trailingAction,
  unreadCount,
  onCancelRename,
  onDelete,
  onMoveDown,
  onMoveUp,
  onRename,
  onRenameTitle,
  onSubmitRename,
}: {
  count: number;
  editing: boolean;
  error: string;
  icon?: React.ReactNode;
  moveDownDisabled?: boolean;
  moveUpDisabled?: boolean;
  pending: boolean;
  subtitle?: string;
  title: string;
  trailingAction?: React.ReactNode;
  unreadCount: number;
  onCancelRename: () => void;
  onDelete: (returnFocusTarget: HTMLButtonElement) => void;
  onMoveDown?: () => void;
  onMoveUp?: () => void;
  onRename: () => void;
  onRenameTitle: (title: string) => void;
  onSubmitRename: (event: React.FormEvent<HTMLFormElement>) => void;
}) {
  const { t } = useI18n();
  return (
    <div className="rss-organization-manager__item" role="listitem">
      <div className="rss-organization-manager__item-main">
        {icon ? <span className="rss-organization-manager__item-icon" aria-hidden="true">{icon}</span> : null}
        {editing ? (
          <form className="rss-organization-manager__rename" onSubmit={onSubmitRename}>
            <Input
              aria-label={t("xiadown.rss.rename")}
              autoFocus
              onChange={(event) => onRenameTitle(event.currentTarget.value)}
              value={title}
            />
            <Button aria-label={t("xiadown.rss.save")} disabled={!title.trim() || pending} size="icon" type="submit" variant="ghost">
              {pending ? <LoaderCircle className="app-motion-spin" /> : <Save />}
            </Button>
            <Button aria-label={t("xiadown.rss.cancel")} disabled={pending} onClick={onCancelRename} size="icon" type="button" variant="ghost">
              <X />
            </Button>
          </form>
        ) : (
          <span className="rss-organization-manager__identity">
            <strong>{title}</strong>
            <small>
              {subtitle ?? `${count} ${t("xiadown.rss.items")} · ${unreadCount} ${t("xiadown.rss.unread")}`}
            </small>
          </span>
        )}
      </div>
      {!editing ? (
        <div className="rss-organization-manager__item-actions">
          {trailingAction}
          {onMoveUp ? (
            <Button aria-label={`${t("xiadown.rss.moveUp")} ${title}`} disabled={moveUpDisabled || pending} onClick={onMoveUp} size="icon" title={t("xiadown.rss.moveUp")} type="button" variant="ghost">
              <ArrowUp />
            </Button>
          ) : null}
          {onMoveDown ? (
            <Button aria-label={`${t("xiadown.rss.moveDown")} ${title}`} disabled={moveDownDisabled || pending} onClick={onMoveDown} size="icon" title={t("xiadown.rss.moveDown")} type="button" variant="ghost">
              <ArrowDown />
            </Button>
          ) : null}
          <Button aria-label={`${t("xiadown.rss.rename")} ${title}`} disabled={pending} onClick={onRename} size="icon" title={t("xiadown.rss.rename")} type="button" variant="ghost">
            <Pencil />
          </Button>
          <Button aria-label={`${t("xiadown.rss.delete")} ${title}`} disabled={pending} onClick={(event) => onDelete(event.currentTarget)} size="icon" title={t("xiadown.rss.delete")} type="button" variant="ghost">
            {pending ? <LoaderCircle className="app-motion-spin" /> : <Trash2 />}
          </Button>
        </div>
      ) : null}
      {error ? <OrganizationError text={error} /> : null}
    </div>
  );
}

function OrganizationEmpty({ text }: { text: string }) {
  return <p className="rss-organization-manager__empty">{text}</p>;
}

function OrganizationError({ text }: { text: string }) {
  return <p className="rss-organization-manager__error" role="alert">{text}</p>;
}

function organizationRowKey(kind: OrganizationKind, id: string) {
  return `${kind}:${id}`;
}

function omitRecordKey(current: Record<string, string>, key: string) {
  if (!(key in current)) return current;
  const next = { ...current };
  delete next[key];
  return next;
}

function deleteTitleKey(kind: OrganizationKind) {
  switch (kind) {
    case "category": return "xiadown.rss.deleteCategory";
    case "collection": return "xiadown.rss.deleteCustomList";
  }
}

function deleteDescriptionKey(kind: OrganizationKind) {
  switch (kind) {
    case "category": return "xiadown.rss.deleteCategoryDescription";
    case "collection": return "xiadown.rss.deleteCustomListDescription";
  }
}
