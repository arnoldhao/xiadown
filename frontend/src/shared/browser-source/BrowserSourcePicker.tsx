import { Check, Globe2, Loader2, RefreshCw, ShieldCheck } from "lucide-react";
import * as React from "react";

import { cn } from "@/lib/utils";
import type {
  BrowserSourceCatalog,
  BrowserSourceIntent,
  BrowserSourceSelection,
} from "@/shared/contracts/browserSources";
import { useI18n } from "@/shared/i18n";
import {
  useBrowserSources,
  useRefreshBrowserSources,
} from "@/shared/query/browserSources";
import { Button } from "@/shared/ui/button";
import { Select } from "@/shared/ui/select";
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

export const EMPTY_BROWSER_SOURCE_SELECTION: BrowserSourceSelection = {
  mode: "browser_profile",
  browserId: "",
  profileId: "",
};

export function resolveBrowserSourceSelection(
  catalog: BrowserSourceCatalog | undefined,
  current: BrowserSourceSelection,
  allowXiaDownProfiles: boolean,
  allowBrowserProfiles = true,
): BrowserSourceSelection {
  if (!catalog) {
    return current;
  }
  if (
    allowXiaDownProfiles &&
    (current.mode === "xiadown_profile" || !allowBrowserProfiles)
  ) {
    const availableProfiles = catalog.xiadownProfiles.filter(
      (item) => item.available,
    );
    const preferredBrowserId = current.browserId.trim();
    const browserProfiles = preferredBrowserId
      ? availableProfiles.filter(
          (item) => item.browserId === preferredBrowserId,
        )
      : availableProfiles;
    const profile =
      browserProfiles.find(
        (item) => item.available && item.id === current.profileId,
      ) ??
      browserProfiles.find((item) => item.isDefault) ??
      browserProfiles[0] ??
      availableProfiles.find((item) => item.id === current.profileId) ??
      availableProfiles.find((item) => item.isDefault) ??
      availableProfiles[0];
    if (profile) {
      return {
        mode: "xiadown_profile",
        browserId: profile.browserId ?? "",
        profileId: profile.id,
      };
    }
    if (!allowBrowserProfiles) {
      return {
        mode: "xiadown_profile",
        browserId: "",
        profileId: "",
      };
    }
  }
  const browser =
    catalog.browsers.find(
      (item) => item.available && item.id === current.browserId,
    ) ?? catalog.browsers.find((item) => item.available);
  if (!browser) {
    return { ...EMPTY_BROWSER_SOURCE_SELECTION };
  }
  const profile =
    browser.profiles.find(
      (item) => item.available && item.id === current.profileId,
    ) ??
    browser.profiles.find((item) => item.available && item.isDefault) ??
    browser.profiles.find((item) => item.available);
  return {
    mode: "browser_profile",
    browserId: browser.id,
    profileId: profile?.id ?? "",
  };
}

export function BrowserSourcePicker(props: {
  catalog?: BrowserSourceCatalog;
  selection: BrowserSourceSelection;
  onSelectionChange: (selection: BrowserSourceSelection) => void;
  allowXiaDownProfiles: boolean;
  allowBrowserProfiles?: boolean;
  loading?: boolean;
  error?: string;
  refreshing?: boolean;
  onRefresh?: () => void;
}) {
  const { t } = useI18n();
  const availableBrowsers =
    props.catalog?.browsers.filter((browser) => browser.available) ?? [];
  const selectedBrowser = availableBrowsers.find(
    (browser) => browser.id === props.selection.browserId,
  );
  const availableBrowserProfiles =
    selectedBrowser?.profiles.filter((profile) => profile.available) ?? [];
  const availableXiaDownProfiles =
    props.catalog?.xiadownProfiles.filter((profile) => profile.available) ?? [];
  const xiaDownBrowserIds = new Set(
    availableXiaDownProfiles
      .map((profile) => profile.browserId)
      .filter((browserId): browserId is string => Boolean(browserId)),
  );
  const availableXiaDownBrowsers = availableBrowsers.filter((browser) =>
    xiaDownBrowserIds.has(browser.id),
  );
  const selectedXiaDownProfiles = availableXiaDownProfiles.filter(
    (profile) => profile.browserId === props.selection.browserId,
  );

  React.useEffect(() => {
    const next = resolveBrowserSourceSelection(
      props.catalog,
      props.selection,
      props.allowXiaDownProfiles,
      props.allowBrowserProfiles !== false,
    );
    if (
      next.mode !== props.selection.mode ||
      next.browserId !== props.selection.browserId ||
      next.profileId !== props.selection.profileId
    ) {
      props.onSelectionChange(next);
    }
  }, [
    props.allowXiaDownProfiles,
    props.allowBrowserProfiles,
    props.catalog,
    props.onSelectionChange,
    props.selection,
  ]);

  const selectMode = (mode: BrowserSourceSelection["mode"]) => {
    props.onSelectionChange(
      resolveBrowserSourceSelection(
        props.catalog,
        { ...props.selection, mode, browserId: "", profileId: "" },
        props.allowXiaDownProfiles,
        props.allowBrowserProfiles !== false,
      ),
    );
  };

  return (
    <div className="space-y-4">
      {props.allowXiaDownProfiles && props.allowBrowserProfiles !== false ? (
        <div className="grid gap-2 sm:grid-cols-2" role="radiogroup" aria-label={t("browserSource.chooseMode")}>
          {([
            {
              mode: "browser_profile" as const,
              icon: Globe2,
              title: t("browserSource.browserProfile"),
              description: t("browserSource.browserProfileDescription"),
              disabled: availableBrowsers.length === 0,
            },
            {
              mode: "xiadown_profile" as const,
              icon: ShieldCheck,
              title: t("browserSource.xiadownProfile"),
              description: t("browserSource.xiadownProfileDescription"),
              disabled: availableXiaDownProfiles.length === 0,
            },
          ]).map((option) => {
            const selected = props.selection.mode === option.mode;
            return (
              <button
                key={option.mode}
                type="button"
                role="radio"
                aria-checked={selected}
                disabled={option.disabled}
                onClick={() => selectMode(option.mode)}
                className="app-browser-source-option relative flex min-w-0 items-start gap-3 p-3"
                data-selected={selected || undefined}
              >
                <option.icon className="app-browser-source-option-icon mt-0.5 h-4 w-4 shrink-0" />
                <span className="min-w-0 flex-1">
                  <span className="app-browser-source-option-title block">{option.title}</span>
                  <span className="app-browser-source-option-description mt-1 block">{option.description}</span>
                </span>
                {selected ? <Check className="app-browser-source-option-check h-4 w-4 shrink-0" /> : null}
              </button>
            );
          })}
        </div>
      ) : (
        <div className="app-browser-source-notice p-3">
          {props.allowXiaDownProfiles
            ? t("browserSource.xiadownProfileDescription")
            : t("browserSource.readOnlyNotice")}
        </div>
      )}

      <div className="app-browser-source-card p-4">
        <div className="mb-3 flex items-center justify-between gap-3">
          <span className="app-browser-source-heading">
            {props.selection.mode === "xiadown_profile"
              ? t("browserSource.xiadownProfile")
              : t("browserSource.browserProfile")}
          </span>
          {props.onRefresh ? (
            <Button
              type="button"
              variant="ghost"
              size="compactIcon"
              onClick={props.onRefresh}
              disabled={props.refreshing}
              aria-label={t("browserSource.refresh")}
            >
              <RefreshCw className={cn("h-4 w-4", props.refreshing && "app-motion-spin")} />
            </Button>
          ) : null}
        </div>

        {props.loading ? (
          <div className="app-browser-source-feedback flex items-center gap-2 py-5">
            <Loader2 className="h-4 w-4 app-motion-spin" />
            {t("browserSource.loading")}
          </div>
        ) : props.error ? (
          <div className="app-dream-status-message px-3 py-2" data-intent="danger">
            {props.error}
          </div>
        ) : props.selection.mode === "browser_profile" ? (
          <div className="grid gap-3 sm:grid-cols-2">
            <label className="app-browser-source-field-label grid gap-1.5">
              {t("browserSource.browser")}
              <Select
                value={props.selection.browserId}
                disabled={availableBrowsers.length === 0}
                onChange={(event) => {
                  props.onSelectionChange(
                    resolveBrowserSourceSelection(
                      props.catalog,
                      {
                        mode: "browser_profile",
                        browserId: event.target.value,
                        profileId: "",
                      },
                      props.allowXiaDownProfiles,
                      props.allowBrowserProfiles !== false,
                    ),
                  );
                }}
              >
                {availableBrowsers.length === 0 ? (
                  <option value="">{t("browserSource.noBrowsers")}</option>
                ) : null}
                {availableBrowsers.map((browser) => (
                  <option key={browser.id} value={browser.id}>{browser.label}</option>
                ))}
              </Select>
            </label>
            <label className="app-browser-source-field-label grid gap-1.5">
              {t("browserSource.profile")}
              <Select
                value={props.selection.profileId}
                disabled={availableBrowserProfiles.length === 0}
                onChange={(event) =>
                  props.onSelectionChange({
                    ...props.selection,
                    profileId: event.target.value,
                  })
                }
              >
                {availableBrowserProfiles.length === 0 ? (
                  <option value="">{t("browserSource.noProfiles")}</option>
                ) : null}
                {availableBrowserProfiles.map((profile) => (
                  <option key={profile.id} value={profile.id}>
                    {profile.label || (profile.isDefault ? t("browserSource.defaultProfile") : profile.id)}
                  </option>
                ))}
              </Select>
            </label>
          </div>
        ) : (
          <div className="grid gap-3 sm:grid-cols-2">
            <label className="app-browser-source-field-label grid gap-1.5">
              {t("browserSource.browser")}
              <Select
                value={props.selection.browserId}
                disabled={availableXiaDownBrowsers.length === 0}
                onChange={(event) => {
                  props.onSelectionChange(
                    resolveBrowserSourceSelection(
                      props.catalog,
                      {
                        mode: "xiadown_profile",
                        browserId: event.target.value,
                        profileId: "",
                      },
                      props.allowXiaDownProfiles,
                      false,
                    ),
                  );
                }}
              >
                {availableXiaDownBrowsers.length === 0 ? (
                  <option value="">{t("browserSource.noBrowsers")}</option>
                ) : null}
                {availableXiaDownBrowsers.map((browser) => (
                  <option key={browser.id} value={browser.id}>{browser.label}</option>
                ))}
              </Select>
            </label>
            <label className="app-browser-source-field-label grid gap-1.5">
              {t("browserSource.profile")}
              <Select
                value={props.selection.profileId}
                disabled={selectedXiaDownProfiles.length === 0}
                onChange={(event) => {
                  const profile = selectedXiaDownProfiles.find(
                    (item) => item.id === event.target.value,
                  );
                  props.onSelectionChange({
                    mode: "xiadown_profile",
                    browserId: profile?.browserId ?? props.selection.browserId,
                    profileId: event.target.value,
                  });
                }}
              >
                {selectedXiaDownProfiles.length === 0 ? (
                  <option value="">{t("browserSource.noProfiles")}</option>
                ) : null}
                {selectedXiaDownProfiles.map((profile) => (
                  <option key={profile.id || `default:${profile.browserId}`} value={profile.id}>
                    {profile.label || (profile.isDefault ? t("browserSource.defaultProfile") : profile.id)}
                  </option>
                ))}
              </Select>
            </label>
          </div>
        )}
      </div>
    </div>
  );
}

export function BrowserSourceSheet(props: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  intent: BrowserSourceIntent;
  allowXiaDownProfiles: boolean;
  managedOnly?: boolean;
  title: string;
  description: string;
  confirmLabel: string;
  confirming?: boolean;
  onConfirm: (selection: BrowserSourceSelection) => void | Promise<void>;
}) {
  const { t } = useI18n();
  const sources = useBrowserSources(props.intent, props.open);
  const refresh = useRefreshBrowserSources(props.intent);
  const [selection, setSelection] = React.useState<BrowserSourceSelection>(
    EMPTY_BROWSER_SOURCE_SELECTION,
  );
  const resolvedSelection = resolveBrowserSourceSelection(
    sources.data,
    props.managedOnly ? { ...selection, mode: "xiadown_profile" } : selection,
    props.allowXiaDownProfiles,
    !props.managedOnly,
  );
  const canConfirm = Boolean(
    resolvedSelection.mode === "xiadown_profile"
      ? resolvedSelection.browserId
      : resolvedSelection.browserId && resolvedSelection.profileId,
  );

  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent centered size="md" windowChromeSafeArea>
        <SheetHeader>
          <SheetHeading>
            <SheetTitle>{props.title}</SheetTitle>
            <SheetDescription>{props.description}</SheetDescription>
          </SheetHeading>
          <SheetCloseButton aria-label={t("common.close")} />
        </SheetHeader>
        <SheetBody>
          <BrowserSourcePicker
            catalog={sources.data}
            selection={props.managedOnly ? resolvedSelection : selection}
            onSelectionChange={setSelection}
            allowXiaDownProfiles={props.allowXiaDownProfiles}
            allowBrowserProfiles={!props.managedOnly}
            loading={sources.isLoading}
            error={sources.isError ? t("browserSource.loadFailed") : ""}
            refreshing={refresh.isPending}
            onRefresh={() => void refresh.mutateAsync()}
          />
        </SheetBody>
        <SheetFooter>
          <Button type="button" variant="outline" onClick={() => props.onOpenChange(false)}>
            {t("common.cancel")}
          </Button>
          <Button
            type="button"
            disabled={!canConfirm || props.confirming}
            onClick={() => void props.onConfirm(resolvedSelection)}
          >
            {props.confirming ? <Loader2 className="h-4 w-4 app-motion-spin" /> : null}
            {props.confirmLabel}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}
