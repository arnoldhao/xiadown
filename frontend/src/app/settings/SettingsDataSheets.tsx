import {
  Check,
  Database,
  FolderOpen,
  HardDrive,
  Loader2,
  Pencil,
  RefreshCcw,
  RefreshCw,
  ShieldAlert,
  Trash2,
} from "lucide-react";
import * as React from "react";

import { cn } from "@/lib/utils";
import { ManagedProfileAvatar } from "@/shared/browser-source/ManagedProfileIdentity";
import type {
  DataManagementCategoryId,
  DataManagementItem,
} from "@/shared/contracts/dataManagement";
import { useI18n } from "@/shared/i18n";
import {
  clearSniffProfile,
  deleteSniffProfile,
  openSniffProfile,
  renameSniffProfile,
  useBrowserSources,
  useRefreshBrowserSources,
} from "@/shared/query/browserSources";
import {
  settleDataManagementCleanResults,
  useCleanDataManagement,
  useDataManagementSnapshot,
  useResetApplication,
} from "@/shared/query/dataManagement";
import { Button } from "@/shared/ui/button";
import { Input } from "@/shared/ui/input";
import { StatusBadge } from "@/shared/ui/status-badge";
import {
  Sheet,
  SheetBody,
  SheetCloseButton,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetHeading,
  SheetTitle,
} from "@/shared/ui/sheet";
import { formatBytes } from "@/shared/utils/formatBytes";

function translatedOrFallback(
  key: string | undefined,
  fallbackText: string,
  t: (key: string) => string,
) {
  const normalized = key?.trim() ?? "";
  if (normalized) {
    const translated = t(normalized);
    if (translated !== normalized) {
      return translated;
    }
  }
  return fallbackText;
}

function resourceText(item: DataManagementItem, t: (key: string) => string) {
  if (item.id === "legacy.app-sessions") {
    return {
      label: t("dataManagement.resource.legacyAppSessions.label"),
      description: t("dataManagement.resource.legacyAppSessions.description"),
    };
  }
  if (item.id === "legacy.sniff-profiles") {
    return {
      label: t("dataManagement.resource.legacyBrowserProfiles.label"),
      description: t("dataManagement.resource.legacyBrowserProfiles.description"),
    };
  }
  if (item.id === "legacy.database-backups") {
    return {
      label: t("dataManagement.resource.legacyDatabaseBackups.label"),
      description: t("dataManagement.resource.legacyDatabaseBackups.description"),
    };
  }
  const fallback = (() => {
    switch (item.id) {
      case "database":
        return [
          t("dataManagement.resource.database.label"),
          t("dataManagement.resource.database.description"),
        ];
      case "database-backups":
        return [
          t("dataManagement.resource.databaseBackups.label"),
          t("dataManagement.resource.databaseBackups.description"),
        ];
      case "app-sessions":
        return [
          t("dataManagement.resource.currentAppSessions.label"),
          t("dataManagement.resource.currentAppSessions.description"),
        ];
      case "app_sessions":
        return [
          t("dataManagement.resource.appSessions.label"),
          t("dataManagement.resource.appSessions.description"),
        ];
      case "browser-profiles":
        return [
          t("dataManagement.resource.currentSniffProfiles.label"),
          t("dataManagement.resource.currentSniffProfiles.description"),
        ];
      case "browser_profiles":
      case "sniff_profiles":
        return [
          t("dataManagement.resource.browserProfiles.label"),
          t("dataManagement.resource.browserProfiles.description"),
        ];
      case "dependencies":
        return [
          t("dataManagement.resource.dependencies.label"),
          t("dataManagement.resource.dependencies.description"),
        ];
      case "active-logs":
        return [
          t("dataManagement.resource.activeLogs.label"),
          t("dataManagement.resource.activeLogs.description"),
        ];
      case "session-vault-key":
        return [
          t("dataManagement.resource.sessionVaultKey.label"),
          t("dataManagement.resource.sessionVaultKey.description"),
        ];
      case "user-content":
        return [
          t("dataManagement.resource.userContent.label"),
          t("dataManagement.resource.userContent.description"),
        ];
      case "update-stage":
        return [
          t("dataManagement.resource.updateStage.label"),
          t("dataManagement.resource.updateStage.description"),
        ];
      case "logs":
        return [
          t("dataManagement.resource.logs.label"),
          t("dataManagement.resource.logs.description"),
        ];
      case "archived-logs":
        return [
          t("dataManagement.resource.archivedLogs.label"),
          t("dataManagement.resource.archivedLogs.description"),
        ];
      case "image-cache":
        return [
          t("dataManagement.resource.imageCache.label"),
          t("dataManagement.resource.imageCache.description"),
        ];
      case "rss-cache":
        return [
          t("dataManagement.resource.rssCache.label"),
          t("dataManagement.resource.rssCache.description"),
        ];
      case "favicon-cache":
        return [
          t("dataManagement.resource.faviconCache.label"),
          t("dataManagement.resource.faviconCache.description"),
        ];
      case "cache":
      case "caches":
        return [
          t("dataManagement.resource.caches.label"),
          t("dataManagement.resource.caches.description"),
        ];
      case "temporary":
        return [
          t("dataManagement.resource.temporary.label"),
          t("dataManagement.resource.temporary.description"),
        ];
      default:
        if (item.id.startsWith("sniff-profile:")) {
          return [
            t("dataManagement.resource.browserProfiles.label"),
            t("dataManagement.resource.browserProfiles.description"),
          ];
        }
        return [
          t("dataManagement.resource.other.label"),
          t("dataManagement.resource.other.description"),
        ];
    }
  })();
  return {
    label: translatedOrFallback(item.labelKey, fallback[0], t),
    description: translatedOrFallback(item.descriptionKey, fallback[1], t),
  };
}

function categoryText(
  id: DataManagementCategoryId,
  t: (key: string) => string,
) {
  return {
    label: t(`dataManagement.category.${id}.label`),
    description: t(`dataManagement.category.${id}.description`),
  };
}

function itemStateLabel(
  categoryId: DataManagementCategoryId,
  item: DataManagementItem,
  t: (key: string) => string,
) {
  if (categoryId === "core") {
    return t("dataManagement.coreRetained");
  }
  if (!item.clearable) {
    return t("dataManagement.unavailableToClean");
  }
  return categoryId === "obsolete"
    ? t("dataManagement.obsolete")
    : t("dataManagement.safe");
}

export function DataManagementSheet(props: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { t, language } = useI18n();
  const snapshot = useDataManagementSnapshot(props.open);
  const clean = useCleanDataManagement();
  const reset = useResetApplication();
  const [selectedIds, setSelectedIds] = React.useState<Set<string>>(new Set());
  const [actionError, setActionError] = React.useState("");
  const [confirmingReset, setConfirmingReset] = React.useState(false);
  const [resetScheduled, setResetScheduled] = React.useState(false);
  const appliedDefaultsRef = React.useRef(false);

  React.useEffect(() => {
    if (props.open) {
      appliedDefaultsRef.current = false;
      setSelectedIds(new Set());
      setActionError("");
      setConfirmingReset(false);
      setResetScheduled(false);
    }
  }, [props.open]);

  React.useEffect(() => {
    if (!props.open || !snapshot.data || appliedDefaultsRef.current) {
      return;
    }
    appliedDefaultsRef.current = true;
    setSelectedIds(
      new Set(
        snapshot.data.categories
          .filter((category) => category.id !== "core")
          .flatMap((category) => category.items)
          .filter((item) => item.clearable && item.selectedByDefault === true)
          .map((item) => item.id),
      ),
    );
  }, [props.open, snapshot.data]);

  const clearableItems =
    snapshot.data?.categories
      .filter((category) => category.id !== "core")
      .flatMap((category) => category.items)
      .filter((item) => item.clearable) ?? [];
  const selectedItems = clearableItems.filter((item) => selectedIds.has(item.id));
  const selectedBytes = selectedItems.reduce((total, item) => total + item.sizeBytes, 0);
  const categories = snapshot.data?.categories ?? [];

  const handleClean = async () => {
    if (selectedItems.length === 0 || clean.isPending) {
      return;
    }
    setActionError("");
    try {
      const requestedIds = selectedItems.map((item) => item.id);
      const response = await clean.mutateAsync({ resourceIds: requestedIds });
      const settlement = settleDataManagementCleanResults(
        requestedIds,
        response.results,
      );
      setSelectedIds((current) => {
        const next = new Set(current);
        settlement.succeededIds.forEach((resourceId) => next.delete(resourceId));
        return next;
      });
      if (settlement.failedIds.length > 0) {
        setActionError(t("dataManagement.cleanPartialFailed"));
      }
    } catch {
      setActionError(t("dataManagement.cleanFailed"));
    }
  };

  const handleReset = async () => {
    if (reset.isPending || resetScheduled) {
      return;
    }
    setActionError("");
    try {
      const result = await reset.mutateAsync();
      if (!result.scheduled) {
        throw new Error("application reset was not scheduled");
      }
      setResetScheduled(true);
    } catch {
      setActionError(t("dataManagement.resetFailed"));
    }
  };

  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent centered size="lg" windowChromeSafeArea>
        <SheetHeader>
          <SheetHeading>
            <SheetTitle>{t("dataManagement.title")}</SheetTitle>
            <SheetDescription>{t("dataManagement.description")}</SheetDescription>
          </SheetHeading>
          <SheetCloseButton aria-label={t("common.close")} />
        </SheetHeader>
        <SheetBody>
          {snapshot.isLoading ? (
            <div className="app-settings-feedback flex min-h-64 items-center justify-center gap-2">
              <Loader2 className="h-4 w-4 app-motion-spin" />
              {t("dataManagement.scanning")}
            </div>
          ) : snapshot.isError ? (
            <div className="app-settings-empty-state px-4 py-10">
              <Database className="app-settings-muted-icon mx-auto h-7 w-7" />
              <p className="app-settings-empty-title mt-3">{t("dataManagement.unavailable")}</p>
              <p className="app-settings-empty-description mt-1">{t("dataManagement.unavailableDescription")}</p>
              <Button type="button" variant="outline" size="compact" className="mt-4" onClick={() => void snapshot.refetch()}>
                <RefreshCw className="h-4 w-4" />
                {t("dataManagement.refresh")}
              </Button>
            </div>
          ) : (
            <div className="space-y-5">
              <div className="grid gap-2 sm:grid-cols-2">
                <div className="app-settings-summary-card p-3">
                  <span className="app-settings-summary-label block">{t("dataManagement.totalSize")}</span>
                  <span className="app-settings-summary-value mt-1 block">{formatBytes(snapshot.data?.totalBytes ?? 0)}</span>
                </div>
                <div className="app-settings-summary-card p-3" data-tone="success">
                  <span className="app-settings-summary-label block">{t("dataManagement.safeReclaimable")}</span>
                  <span className="app-settings-summary-value mt-1 block">
                    {formatBytes(snapshot.data?.safeReclaimableBytes ?? 0)}
                  </span>
                </div>
              </div>

              {categories.map((category) => (
                <section key={category.id} className="space-y-2">
                  <div className="flex items-end justify-between gap-3 px-1">
                    <div className="min-w-0">
                      <h3 className="app-settings-section-heading">
                        {categoryText(category.id, t).label}
                      </h3>
                      <p className="app-settings-secondary-label mt-0.5">
                        {categoryText(category.id, t).description}
                      </p>
                    </div>
                    <span className="app-settings-secondary-label">{formatBytes(category.totalBytes)}</span>
                  </div>
                  <div className="app-settings-list-card overflow-hidden">
                    {category.items.length === 0 ? (
                      <div className="app-settings-secondary-label app-settings-centered-text px-3 py-6" role="status">
                        {t("dataManagement.empty")}
                      </div>
                    ) : category.items.map((item, index) => {
                      const displayText = resourceText(item, t);
                      const selected = selectedIds.has(item.id);
                      const selectable = category.id !== "core" && item.clearable;
                      const rowClassName = "app-settings-data-row flex min-w-0 items-center gap-3 px-3 py-3";
                      const rowContent = (
                        <>
                          {selectable ? (
                          <input
                            type="checkbox"
                            checked={selected}
                            onChange={(event) => {
                              setSelectedIds((current) => {
                                const next = new Set(current);
                                if (event.target.checked) {
                                  next.add(item.id);
                                } else {
                                  next.delete(item.id);
                                }
                                return next;
                              });
                            }}
                            className="app-settings-checkbox h-4 w-4 shrink-0"
                          />
                          ) : null}
                          <span className="min-w-0 flex-1">
                            <span className="app-settings-item-title block truncate">{displayText.label}</span>
                            <span className="app-settings-item-description mt-0.5 block">{displayText.description}</span>
                          </span>
                          <span className="app-settings-data-value-column shrink-0">
                            <span className="app-settings-item-size block">{formatBytes(item.sizeBytes)}</span>
                            {item.itemCount > 0 || item.id === "session-vault-key" ? (
                              <span className="app-settings-item-count mt-0.5 block">
                                {t(
                                  item.itemCount === 1
                                    ? "dataManagement.itemCountOne"
                                    : "dataManagement.itemCount",
                                ).replace("{count}", String(item.itemCount))}
                              </span>
                            ) : null}
                            <StatusBadge
                              className="mt-1"
                              tone={category.id === "reclaimable" && item.clearable ? "success" : category.id === "obsolete" && item.clearable ? "warning" : "muted"}
                            >
                              {itemStateLabel(category.id, item, t)}
                            </StatusBadge>
                          </span>
                        </>
                      );
                      return selectable ? (
                        <label key={item.id} className={rowClassName} data-divider={index > 0 || undefined} data-selectable="true">
                          {rowContent}
                        </label>
                      ) : (
                        <div key={item.id} className={rowClassName} data-divider={index > 0 || undefined} data-dimmed={category.id !== "core" || undefined}>
                          {rowContent}
                        </div>
                      );
                    })}
                  </div>
                </section>
              ))}

              <section className="app-settings-danger-panel p-4" aria-labelledby="data-management-reset-title">
                <div className="flex items-start gap-3">
                  <span className="app-settings-danger-icon mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center">
                    <ShieldAlert className="h-4 w-4" aria-hidden="true" />
                  </span>
                  <div className="min-w-0 flex-1">
                    <h3 id="data-management-reset-title" className="app-settings-section-heading">
                      {t("dataManagement.resetTitle")}
                    </h3>
                    <p className="app-settings-secondary-label mt-1">
                      {t("dataManagement.resetDescription")}
                    </p>
                    {resetScheduled ? (
                      <p className="app-settings-danger-text mt-3" role="status">
                        {t("dataManagement.resetScheduled")}
                      </p>
                    ) : confirmingReset ? (
                      <div className="app-settings-danger-confirm mt-3 p-3">
                        <p className="app-settings-confirm-text">
                          {t("dataManagement.resetConfirmation")}
                        </p>
                        <div className="mt-3 flex flex-wrap justify-end gap-2">
                          <Button type="button" size="compact" variant="outline" onClick={() => setConfirmingReset(false)} disabled={reset.isPending}>
                            {t("common.cancel")}
                          </Button>
                          <Button type="button" size="compact" variant="destructive" onClick={() => void handleReset()} disabled={reset.isPending}>
                            {reset.isPending ? <Loader2 className="h-4 w-4 app-motion-spin" /> : <ShieldAlert className="h-4 w-4" />}
                            {t("dataManagement.confirmReset")}
                          </Button>
                        </div>
                      </div>
                    ) : (
                      <Button type="button" size="compact" variant="destructive" className="mt-3" onClick={() => setConfirmingReset(true)}>
                        {t("dataManagement.resetAction")}
                      </Button>
                    )}
                  </div>
                </div>
              </section>

              {snapshot.data?.scannedAt ? (
                <p className="app-settings-timestamp">
                  {t("dataManagement.scannedAt")} {new Date(snapshot.data.scannedAt).toLocaleString(language)}
                </p>
              ) : null}
            </div>
          )}
          {actionError ? (
            <div className="app-dream-status-message mt-3 px-3 py-2" data-intent="danger">{actionError}</div>
          ) : null}
        </SheetBody>
        <SheetFooter className="justify-between">
          <span className="app-settings-secondary-label mr-auto">
            {selectedItems.length > 0
              ? `${t("dataManagement.selected")}: ${formatBytes(selectedBytes)}`
              : t("dataManagement.selectHint")}
          </span>
          <Button type="button" variant="outline" onClick={() => props.onOpenChange(false)}>
            {t("common.close")}
          </Button>
          <Button type="button" disabled={selectedItems.length === 0 || clean.isPending} onClick={() => void handleClean()}>
            {clean.isPending ? <Loader2 className="h-4 w-4 app-motion-spin" /> : <Trash2 className="h-4 w-4" />}
            {t("dataManagement.cleanSelected")}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}

export function BrowserProfilesSheet(props: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { t } = useI18n();
  const sources = useBrowserSources("network_sniff", props.open);
  const refresh = useRefreshBrowserSources("network_sniff");
  const [editingId, setEditingId] = React.useState("");
  const [editingName, setEditingName] = React.useState("");
  const [confirmAction, setConfirmAction] = React.useState<{ id: string; action: "clear" | "delete" } | null>(null);
  const [pendingId, setPendingId] = React.useState("");
  const [actionError, setActionError] = React.useState("");

  const browsers = sources.data?.browsers.filter((browser) => browser.available) ?? [];
  const profiles = sources.data?.xiadownProfiles.filter((profile) => !profile.virtual) ?? [];

  React.useEffect(() => {
    if (!props.open) {
      return;
    }
    setEditingId("");
    setEditingName("");
    setConfirmAction(null);
    setActionError("");
  }, [props.open]);

  const runProfileAction = async (
    id: string,
    action: "open" | "clear" | "delete" | "rename",
    name?: string,
  ) => {
    const profile = profiles.find((item) => item.id === id);
    if (!profile) {
      return;
    }
    setPendingId(id);
    setActionError("");
    try {
      const request = {
        profileId: id,
        browser: profile.browserId,
        displayName: name,
      };
      if (action === "open") {
        await openSniffProfile(request);
      } else if (action === "clear") {
        await clearSniffProfile(request);
      } else if (action === "delete") {
        await deleteSniffProfile(request);
      } else {
        await renameSniffProfile(request);
      }
      setConfirmAction(null);
      setEditingId("");
      await sources.refetch();
    } catch {
      setActionError(t("dataManagement.profileActionFailed"));
    } finally {
      setPendingId("");
    }
  };

  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent centered size="lg" windowChromeSafeArea>
        <SheetHeader>
          <SheetHeading>
            <SheetTitle>{t("dataManagement.profilesTitle")}</SheetTitle>
            <SheetDescription>{t("dataManagement.profilesDescription")}</SheetDescription>
          </SheetHeading>
          <SheetCloseButton aria-label={t("common.close")} />
        </SheetHeader>
        <SheetBody>
          {sources.isLoading ? (
            <div className="app-settings-feedback flex min-h-48 items-center justify-center gap-2">
              <Loader2 className="h-4 w-4 app-motion-spin" />
              {t("browserSource.loading")}
            </div>
          ) : sources.isError ? (
            <div className="app-settings-empty-state px-4 py-10">
              {t("browserSource.loadFailed")}
            </div>
          ) : profiles.length === 0 ? (
            <div className="app-settings-empty-state px-4 py-10">
              {t("dataManagement.noProfiles")}
            </div>
          ) : (
            <div className="grid gap-2">
              {profiles.map((profile) => {
                const browser = browsers.find((item) => item.id === profile.browserId);
                const profileLabel =
                  profile.label?.trim() || t("browserSource.defaultProfile");
                const profileRole = profile.isDefault
                  ? t("browserSource.defaultProfile")
                  : t("dataManagement.obsolete");
                const pending = pendingId === profile.id;
                const confirming = confirmAction?.id === profile.id;
                return (
                  <div key={profile.id} className="app-settings-profile-row flex min-w-0 items-center gap-3 px-3 py-3">
                    <ManagedProfileAvatar
                      className="h-9 w-9"
                      profile={profile}
                      badge={<HardDrive className="app-settings-muted-icon h-3 w-3" />}
                    />
                    <span className="min-w-0 flex-1">
                      {editingId === profile.id ? (
                        <Input value={editingName} onChange={(event) => setEditingName(event.target.value)} size="compact" />
                      ) : (
                        <>
                          <span className="app-settings-profile-title block truncate">{profileLabel}</span>
                          <span className="app-settings-profile-meta mt-0.5 block truncate">
                            {profileRole} · {browser?.label || profile.browserId || t("browserSource.xiadownProfile")}
                            {profile.sizeBytes !== undefined ? ` · ${formatBytes(profile.sizeBytes)}` : ""}
                          </span>
                        </>
                      )}
                    </span>
                    <div className="flex shrink-0 items-center gap-1">
                      {editingId === profile.id ? (
                        <>
                          <Button type="button" size="compact" variant="outline" onClick={() => setEditingId("")}>{t("common.cancel")}</Button>
                          <Button type="button" size="compact" disabled={!editingName.trim() || pending} onClick={() => void runProfileAction(profile.id, "rename", editingName.trim())}>
                            {pending ? <Loader2 className="h-4 w-4 app-motion-spin" /> : <Check className="h-4 w-4" />}
                            {t("dataManagement.saveProfile")}
                          </Button>
                        </>
                      ) : confirming ? (
                        <>
                          <Button type="button" size="compact" variant="outline" onClick={() => setConfirmAction(null)}>{t("common.cancel")}</Button>
                          <Button type="button" size="compact" variant="destructive" disabled={pending} onClick={() => void runProfileAction(profile.id, confirmAction.action)}>
                            {pending ? <Loader2 className="h-4 w-4 app-motion-spin" /> : <ShieldAlert className="h-4 w-4" />}
                            {confirmAction.action === "delete" ? t("dataManagement.deleteProfile") : t("dataManagement.clearProfile")}
                          </Button>
                        </>
                      ) : (
                        <>
                          <Button type="button" size="compactIcon" variant="ghost" onClick={() => void runProfileAction(profile.id, "open")} disabled={pending} aria-label={t("dataManagement.openProfile")}>
                            <FolderOpen className="h-4 w-4" />
                          </Button>
                          <Button type="button" size="compactIcon" variant="ghost" onClick={() => { setEditingId(profile.id); setEditingName(profile.label || profile.id); }} disabled={pending} aria-label={t("dataManagement.renameProfile")}>
                            <Pencil className="h-4 w-4" />
                          </Button>
                          <Button type="button" size="compactIcon" variant="ghost" onClick={() => setConfirmAction({ id: profile.id, action: "clear" })} disabled={pending} aria-label={t("dataManagement.clearProfile")}>
                            <RefreshCcw className="h-4 w-4" />
                          </Button>
                          <Button type="button" size="compactIcon" variant="ghost" tone="destructive" onClick={() => setConfirmAction({ id: profile.id, action: "delete" })} disabled={pending} aria-label={t("dataManagement.deleteProfile")}>
                            <Trash2 className="h-4 w-4" />
                          </Button>
                        </>
                      )}
                    </div>
                  </div>
                );
              })}
            </div>
          )}
          {actionError ? <div className="app-dream-status-message mt-3 px-3 py-2" data-intent="danger">{actionError}</div> : null}
        </SheetBody>
        <SheetFooter>
          <Button type="button" variant="outline" onClick={() => void refresh.mutateAsync()} disabled={refresh.isPending}>
            <RefreshCw className={cn("h-4 w-4", refresh.isPending && "app-motion-spin")} />
            {t("browserSource.refresh")}
          </Button>
          <Button type="button" onClick={() => props.onOpenChange(false)}>{t("common.close")}</Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}
