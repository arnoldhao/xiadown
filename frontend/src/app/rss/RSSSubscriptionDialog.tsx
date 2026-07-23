import { ArrowLeft, Check, LoaderCircle, Plus, RefreshCcw, Rss, Settings2, X } from "lucide-react";
import * as React from "react";

import { useRSSAddSubscription, useRSSUpdateSubscription } from "./api";
import {
  buildRSSDiscoveryRouteURL,
  initialRSSDiscoveryParameterValues,
  rssFeedAddressSubscribed,
  RSSDiscoveryRouteConfigurationError,
} from "./discovery-utils";
import { RSSDiscoveryRouteIcon } from "./RSSDiscoveryCards";
import {
  cachedPreviewRSSSubscription,
  deleteCachedRSSPreview,
} from "./preview-api-cache";
import type {
  RSSDiscoveryParameter,
  RSSDiscoveryRoute,
  RSSCategory,
  RSSPreviewResult,
  RSSSubscription,
  RSSViewType,
} from "./types";
import {
  buildRSSAddSubscriptionRequest,
  buildRSSSubscriptionUpdateRequest,
  rssPreviewErrorText,
} from "./subscription-dialog-utils";
import { useI18n, type TFunction } from "@/shared/i18n";
import { Button } from "@/shared/ui/button";
import { DreamInlineSwitch } from "@/shared/ui/dream-inline-switch";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/shared/ui/dialog";
import { Input } from "@/shared/ui/input";
import { Select } from "@/shared/ui/select";

export interface RSSSubscriptionDialogTarget {
  kind: "subscribe";
  url: string;
  route?: RSSDiscoveryRoute;
}

export interface RSSSubscriptionEditDialogTarget {
  kind: "edit";
  subscription: RSSSubscription;
}

interface RSSSubscriptionDialogBaseProps {
  subscriptions: readonly RSSSubscription[];
  categories?: readonly RSSCategory[];
  onClose: () => void;
  returnFocusTarget?: HTMLElement | null;
}

interface RSSSubscriptionSubscribeDialogProps
  extends RSSSubscriptionDialogBaseProps {
  target: RSSSubscriptionDialogTarget;
  onAdded: (subscription: RSSSubscription) => void;
  onUpdated?: never;
}

interface RSSSubscriptionEditorDialogProps
  extends RSSSubscriptionDialogBaseProps {
  target: RSSSubscriptionEditDialogTarget;
  onAdded?: never;
  onUpdated?: (subscription: RSSSubscription) => void;
}

export type RSSSubscriptionDialogProps =
  | RSSSubscriptionSubscribeDialogProps
  | RSSSubscriptionEditorDialogProps;

type DialogStage = "configure" | "preview";

export function RSSSubscriptionDialog(props: RSSSubscriptionDialogProps) {
  if (isRSSSubscriptionEditorDialogProps(props)) {
    return (
      <RSSSubscriptionEditorDialog
        subscription={props.target.subscription}
        categories={props.categories ?? []}
        onClose={props.onClose}
        onUpdated={props.onUpdated}
        returnFocusTarget={props.returnFocusTarget}
      />
    );
  }
  return (
    <RSSSubscribeDialog
      subscriptions={props.subscriptions}
      target={props.target}
      onAdded={props.onAdded}
      onClose={props.onClose}
      returnFocusTarget={props.returnFocusTarget}
    />
  );
}

function isRSSSubscriptionEditorDialogProps(
  props: RSSSubscriptionDialogProps,
): props is RSSSubscriptionEditorDialogProps {
  return props.target.kind === "edit";
}

function RSSSubscribeDialog({
  subscriptions,
  target,
  onAdded,
  onClose,
  returnFocusTarget: providedReturnFocusTarget,
}: RSSSubscriptionDialogBaseProps & {
  target: RSSSubscriptionDialogTarget;
  onAdded: (subscription: RSSSubscription) => void;
}) {
  const { t } = useI18n();
  const [returnFocusTarget] = React.useState<HTMLElement | null>(() => (
    providedReturnFocusTarget ?? (
      typeof document !== "undefined" && document.activeElement instanceof HTMLElement
        ? document.activeElement
        : null
    )
  ));
  const routeNeedsParameters = Boolean(target.route?.needsParameters);
  const [stage, setStage] = React.useState<DialogStage>(routeNeedsParameters ? "configure" : "preview");
  const [values, setValues] = React.useState<Record<string, string>>(() =>
    target.route ? initialRSSDiscoveryParameterValues(target.route) : {},
  );
  const [fieldErrors, setFieldErrors] = React.useState<Record<string, string>>({});
  const [formError, setFormError] = React.useState("");
  const [configuredURL, setConfiguredURL] = React.useState(routeNeedsParameters ? "" : target.url);
  const firstFieldRef = React.useRef<HTMLInputElement | HTMLSelectElement | null>(null);
  const fieldRefs = React.useRef<Record<string, HTMLInputElement | HTMLSelectElement | null>>({});
  const previewTitleRef = React.useRef<HTMLHeadingElement | null>(null);

  const configure = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!target.route) return;
    setFieldErrors({});
    setFormError("");
    try {
      const url = buildRSSDiscoveryRouteURL(target.route, values);
      setConfiguredURL(url);
      setStage("preview");
    } catch (error) {
      if (!(error instanceof RSSDiscoveryRouteConfigurationError)) {
        setFormError(errorText(error));
        return;
      }
      const message = configurationErrorText(error, t);
      if (error.parameterName) {
        setFieldErrors({ [error.parameterName]: message });
        fieldRefs.current[error.parameterName]?.focus();
      } else {
        setFormError(message);
      }
    }
  };

  React.useEffect(() => {
    if (stage === "configure") {
      firstFieldRef.current?.focus();
      return;
    }
    previewTitleRef.current?.focus();
  }, [stage]);

  const dialogTitle = stage === "configure"
    ? t("xiadown.rss.configureSubscription")
    : t("xiadown.rss.previewSubscription");
  const status = stage === "configure"
    ? formError || Object.values(fieldErrors)[0] || ""
    : "";

  return (
    <Dialog open onOpenChange={(open) => { if (!open) onClose(); }}>
      <DialogContent
        className="rss-discovery-dialog"
        onOpenAutoFocus={(event) => {
          if (stage === "configure" && firstFieldRef.current) {
            event.preventDefault();
            firstFieldRef.current.focus();
          }
        }}
        onCloseAutoFocus={(event) => {
          const target = returnFocusTarget?.isConnected
            ? returnFocusTarget
            : fallbackRSSDialogFocusTarget();
          if (!target) return;
          event.preventDefault();
          target.focus();
        }}
        showCloseButton={false}
      >
        <header>
          <div>
            <span>{dialogTitle}</span>
            <DialogTitle ref={previewTitleRef} tabIndex={-1}>
              {target.route?.title || configuredURL || target.url}
            </DialogTitle>
            <DialogDescription className="sr-only">
              {stage === "configure"
                ? t("xiadown.rss.configureSubscriptionDescription")
                : t("xiadown.rss.previewSubscriptionDescription")}
            </DialogDescription>
          </div>
          <DialogClose asChild>
            <Button aria-label={t("xiadown.rss.closePreview")} size="icon" type="button" variant="ghost"><X /></Button>
          </DialogClose>
        </header>

        <div aria-atomic="true" aria-live="polite" className="sr-only" role="status">{status}</div>

        {stage === "configure" && target.route ? (
          <RSSRouteConfigurationForm
            fieldErrors={fieldErrors}
            fieldRefs={fieldRefs}
            firstFieldRef={firstFieldRef}
            formError={formError}
            route={target.route}
            t={t}
            values={values}
            onChange={(name, value) => {
              setValues((current) => ({ ...current, [name]: value }));
              setFieldErrors((current) => {
                if (!current[name]) return current;
                const next = { ...current };
                delete next[name];
                return next;
              });
              setFormError("");
            }}
            onSubmit={configure}
          />
        ) : (
          <RSSFeedPreview
            configuredURL={configuredURL}
            route={target.route}
            subscriptions={subscriptions}
            t={t}
            onAdded={onAdded}
            onBack={routeNeedsParameters ? () => setStage("configure") : undefined}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

function RSSSubscriptionEditorDialog({
  subscription,
  categories,
  onClose,
  onUpdated,
  returnFocusTarget: providedReturnFocusTarget,
}: {
  subscription: RSSSubscription;
  categories: readonly RSSCategory[];
  onClose: () => void;
  onUpdated?: (subscription: RSSSubscription) => void;
  returnFocusTarget?: HTMLElement | null;
}) {
  const { t } = useI18n();
  const update = useRSSUpdateSubscription();
  const [title, setTitle] = React.useState(subscription.title);
  const [viewType, setViewType] = React.useState<RSSViewType>(subscription.viewType);
  const [enabled, setEnabled] = React.useState(subscription.enabled);
  const [categoryId, setCategoryId] = React.useState(subscription.categoryId || "");
  const [returnFocusTarget] = React.useState<HTMLElement | null>(() => (
    providedReturnFocusTarget ?? (
      typeof document !== "undefined" && document.activeElement instanceof HTMLElement
        ? document.activeElement
        : null
    )
  ));
  const titleRef = React.useRef<HTMLInputElement | null>(null);
  const normalizedTitle = title.trim();
  const changed = normalizedTitle !== subscription.title ||
    viewType !== subscription.viewType ||
    enabled !== subscription.enabled ||
    categoryId !== (subscription.categoryId || "");

  const save = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!normalizedTitle || !changed || update.isPending) return;
    try {
      const updated = await update.mutateAsync(buildRSSSubscriptionUpdateRequest(
        subscription,
        {
          title: normalizedTitle,
          viewType,
          enabled,
          categoryId,
        },
      ));
      onUpdated?.(updated);
      onClose();
    } catch {
      // The bounded mutation error is rendered below the settings.
    }
  };

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !update.isPending && !changed) onClose();
      }}
    >
      <DialogContent
        className="rss-discovery-dialog rss-subscription-editor-dialog"
        onOpenAutoFocus={(event) => {
          event.preventDefault();
          titleRef.current?.focus();
          titleRef.current?.select();
        }}
        onCloseAutoFocus={(event) => {
          const target = returnFocusTarget?.isConnected
            ? returnFocusTarget
            : fallbackRSSDialogFocusTarget();
          if (!target) return;
          event.preventDefault();
          target.focus();
        }}
        showCloseButton={false}
      >
        <header>
          <div>
            <span>{t("xiadown.rss.editSubscription")}</span>
            <DialogTitle
              aria-label={`${t("xiadown.rss.editSubscription")}: ${subscription.title}`}
            >
              {subscription.title}
            </DialogTitle>
            <DialogDescription className="sr-only">
              {t("xiadown.rss.manageSubscriptions")}
            </DialogDescription>
          </div>
          <Button
            aria-label={t("common.close")}
            disabled={update.isPending}
            onClick={onClose}
            size="icon"
            type="button"
            variant="ghost"
          >
            <X />
          </Button>
        </header>
        <form className="rss-subscription-editor-form" onSubmit={save}>
          <div className="rss-discovery-dialog__source">
            <span className="rss-favicon"><Rss /></span>
            <span>
              <strong>{subscription.title}</strong>
              <small>{subscription.feedUrl}</small>
            </span>
          </div>
          <RSSSubscriptionSettingsFields
            categories={categories}
            categoryId={categoryId}
            enabled={enabled}
            inputRef={titleRef}
            title={title}
            viewType={viewType}
            onEnabledChange={setEnabled}
            onCategoryIdChange={setCategoryId}
            onTitleChange={setTitle}
            onViewTypeChange={setViewType}
          />
          {update.error ? (
            <p aria-live="assertive" className="rss-form-error" role="alert">
              {errorText(update.error)}
            </p>
          ) : null}
          <footer>
            <Button
              disabled={update.isPending}
              onClick={onClose}
              type="button"
              variant="outline"
            >
              {t("xiadown.rss.cancel")}
            </Button>
            <Button
              disabled={!normalizedTitle || !changed || update.isPending}
              type="submit"
            >
              {update.isPending ? <LoaderCircle className="app-motion-spin" /> : null}
              {t("xiadown.actions.save")}
            </Button>
          </footer>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function RSSSubscriptionSettingsFields({
  categories,
  categoryId,
  enabled,
  inputRef,
  title,
  viewType,
  onCategoryIdChange,
  onEnabledChange,
  onTitleChange,
  onViewTypeChange,
}: {
  categories?: readonly RSSCategory[];
  categoryId?: string;
  enabled?: boolean;
  inputRef?: React.Ref<HTMLInputElement>;
  title: string;
  viewType: RSSViewType;
  onCategoryIdChange?: (categoryId: string) => void;
  onEnabledChange?: (enabled: boolean) => void;
  onTitleChange: (title: string) => void;
  onViewTypeChange: (viewType: RSSViewType) => void;
}) {
  const { t } = useI18n();
  return (
    <div className="rss-subscription-settings-fields">
      <label className="rss-subscription-settings-field">
        <span>{t("xiadown.rss.subscriptionTitle")}</span>
        <Input
          maxLength={512}
          onChange={(event) => onTitleChange(event.currentTarget.value)}
          ref={inputRef}
          required
          value={title}
        />
      </label>
      {categories && onCategoryIdChange ? (
        <label className="rss-subscription-settings-field">
          <span>{t("xiadown.rss.folder")}</span>
          <Select
            onChange={(event) => onCategoryIdChange(event.currentTarget.value)}
            value={categoryId || ""}
          >
            <option value="">{t("xiadown.rss.uncategorized")}</option>
            {categories.map((category) => (
              <option key={category.id} value={category.id}>{category.title}</option>
            ))}
          </Select>
        </label>
      ) : null}
      <label className="rss-subscription-settings-field">
        <span>{t("xiadown.rss.viewType")}</span>
        <Select
          onChange={(event) => onViewTypeChange(event.currentTarget.value as RSSViewType)}
          value={viewType}
        >
          {viewTypeOptions(t).map((option) => (
            <option key={option.value} value={option.value}>{option.label}</option>
          ))}
        </Select>
      </label>
      {enabled !== undefined && onEnabledChange ? (
        <div className="rss-subscription-settings-field rss-subscription-settings-field--switch">
          <span>
            <strong>{t("xiadown.rss.subscriptionEnabled")}</strong>
            <small>{t("xiadown.rss.subscriptionEnabledDescription")}</small>
          </span>
          <DreamInlineSwitch
            ariaLabel={t("xiadown.rss.subscriptionEnabled")}
            checked={enabled}
            onCheckedChange={onEnabledChange}
          />
        </div>
      ) : null}
    </div>
  );
}

function RSSRouteConfigurationForm({
  fieldErrors,
  fieldRefs,
  firstFieldRef,
  formError,
  route,
  t,
  values,
  onChange,
  onSubmit,
}: {
  fieldErrors: Readonly<Record<string, string>>;
  fieldRefs: React.MutableRefObject<Record<string, HTMLInputElement | HTMLSelectElement | null>>;
  firstFieldRef: React.MutableRefObject<HTMLInputElement | HTMLSelectElement | null>;
  formError: string;
  route: RSSDiscoveryRoute;
  t: TFunction;
  values: Readonly<Record<string, string>>;
  onChange: (name: string, value: string) => void;
  onSubmit: (event: React.FormEvent<HTMLFormElement>) => void;
}) {
  const fieldPrefix = React.useId();
  return (
    <form className="rss-discovery-parameter-form" noValidate onSubmit={onSubmit}>
      <div className="rss-discovery-dialog__source">
        <RSSDiscoveryRouteIcon route={route} />
        <span><strong>{route.sourceName}</strong><small>rsshub://{route.routePath}</small></span>
      </div>
      <RSSRouteRequirementsNotice route={route} t={t} />
      <div className="rss-discovery-parameter-intro">
        <strong>{t("xiadown.rss.routeParameters")}</strong>
        <span>{t("xiadown.rss.routeParametersDescription")}</span>
      </div>
      <div className="rss-discovery-parameter-fields">
        {route.parameters.map((parameter, index) => (
          <RSSRouteParameterField
            error={fieldErrors[parameter.name]}
            fieldPrefix={fieldPrefix}
            inputRef={(node) => {
              fieldRefs.current[parameter.name] = node;
              if (index === 0) firstFieldRef.current = node;
            }}
            key={parameter.name}
            parameter={parameter}
            t={t}
            value={values[parameter.name] ?? ""}
            onChange={(value) => onChange(parameter.name, value)}
          />
        ))}
      </div>
      {formError ? <p aria-live="assertive" className="rss-form-error" role="alert">{formError}</p> : null}
      <footer>
        <DialogClose asChild><Button type="button" variant="outline">{t("xiadown.rss.cancel")}</Button></DialogClose>
        <Button type="submit">{t("xiadown.rss.continueToPreview")}</Button>
      </footer>
    </form>
  );
}

function RSSRouteParameterField({
  error,
  fieldPrefix,
  inputRef,
  parameter,
  t,
  value,
  onChange,
}: {
  error?: string;
  fieldPrefix: string;
  inputRef: (node: HTMLInputElement | HTMLSelectElement | null) => void;
  parameter: RSSDiscoveryParameter;
  t: TFunction;
  value: string;
  onChange: (value: string) => void;
}) {
  const inputID = `${fieldPrefix}-${parameter.name.replace(/[^A-Za-z0-9_-]/g, "-")}`;
  const descriptionID = `${inputID}-description`;
  const errorID = `${inputID}-error`;
  const describedBy = [parameter.description || parameter.defaultValue ? descriptionID : "", error ? errorID : ""].filter(Boolean).join(" ") || undefined;
  const label = humanizeParameterName(parameter.name);
  const placeholder = parameter.exampleValue ? `${t("xiadown.rss.examplePrefix")} ${parameter.exampleValue}` : undefined;
  const commonProps = {
    "aria-describedby": describedBy,
    "aria-invalid": Boolean(error) || undefined,
    id: inputID,
    name: parameter.name,
    required: !parameter.optional,
    value,
  };
  return (
    <label className="rss-discovery-parameter-field" htmlFor={inputID}>
      <span className="rss-discovery-parameter-field__label">
        <strong>{label}</strong>
        {!parameter.optional ? <sup aria-label={t("xiadown.rss.required")}>*</sup> : <em>{t("xiadown.rss.optional")}</em>}
      </span>
      {parameter.options.length > 0 ? (
        <Select
          {...commonProps}
          ref={inputRef}
          onChange={(event) => onChange(event.currentTarget.value)}
        >
          <option value="">{t("xiadown.rss.selectParameterValue")}</option>
          {parameter.options.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
        </Select>
      ) : (
        <Input
          {...commonProps}
          inputMode={parameter.type === "number" ? "numeric" : undefined}
          placeholder={placeholder}
          ref={inputRef}
          type={parameter.type === "number" ? "number" : "text"}
          onChange={(event) => onChange(event.currentTarget.value)}
        />
      )}
      {parameter.description || parameter.defaultValue ? (
        <small id={descriptionID}>
          {parameter.description}
          {parameter.defaultValue ? <span>{t("xiadown.rss.defaultValue")} <code>{parameter.defaultValue}</code></span> : null}
        </small>
      ) : null}
      {error ? <small className="rss-form-error" id={errorID} role="alert">{error}</small> : null}
    </label>
  );
}

function RSSFeedPreview({
  configuredURL,
  route,
  subscriptions,
  t,
  onAdded,
  onBack,
}: {
  configuredURL: string;
  route?: RSSDiscoveryRoute;
  subscriptions: readonly RSSSubscription[];
  t: TFunction;
  onAdded: (subscription: RSSSubscription) => void;
  onBack?: () => void;
}) {
  const add = useRSSAddSubscription();
  const [viewType, setViewType] = React.useState<RSSViewType>(route?.viewType ?? "auto");
  const [title, setTitle] = React.useState(
    () => route?.sourceName || subscriptionTitleFromURL(configuredURL),
  );
  const [titleEdited, setTitleEdited] = React.useState(false);
  const [preview, setPreview] = React.useState<RSSPreviewResult | null>(null);
  const [previewError, setPreviewError] = React.useState("");
  const [previewing, setPreviewing] = React.useState(true);
  const [previewAttempt, setPreviewAttempt] = React.useState(0);
  const subscribed = rssFeedAddressSubscribed(configuredURL, subscriptions);

  React.useEffect(() => {
    if (!titleEdited && preview?.subscription.title) {
      setTitle(preview.subscription.title);
    }
  }, [preview?.subscription.title, titleEdited]);

  React.useEffect(() => {
    let current = true;
    setPreview(null);
    setPreviewError("");
    setPreviewing(true);
    void cachedPreviewRSSSubscription(
      { url: configuredURL, viewType: route?.viewType ?? "auto" },
      { force: previewAttempt > 0 },
    )
      .then((result) => { if (current) setPreview(result); })
      .catch((error) => {
        if (current) {
          setPreviewError(rssPreviewErrorText(
            error,
            t("xiadown.rss.previewFailureHint"),
          ));
        }
      })
      .finally(() => { if (current) setPreviewing(false); });
    return () => { current = false; };
  }, [configuredURL, previewAttempt, route?.viewType]);

  const subscribe = async () => {
    try {
      const item = await add.mutateAsync(buildRSSAddSubscriptionRequest(
        configuredURL,
        viewType,
        preview,
        title,
      ));
      deleteCachedRSSPreview({
        url: configuredURL,
        viewType: route?.viewType ?? "auto",
      });
      onAdded(item);
    } catch {
      // The mutation error is rendered below the actions.
    }
  };

  return (
    <>
      <div className="rss-discovery-dialog__source">
        {route ? <RSSDiscoveryRouteIcon route={route} /> : <span className="rss-favicon"><Rss /></span>}
        <span><strong>{preview?.subscription.title || route?.sourceName || t("xiadown.rss.loadingPreview")}</strong><small>{configuredURL}</small></span>
      </div>
      {route ? <RSSRouteRequirementsNotice route={route} t={t} /> : null}
      <div aria-busy={previewing || undefined} aria-live="polite" className="rss-discovery-dialog__preview">
        {previewing ? (
          <div className="rss-state-surface" role="status"><LoaderCircle className="app-motion-spin" /><span>{t("xiadown.rss.loadingPreview")}</span></div>
        ) : previewError ? (
          <div className="rss-state-surface rss-state-surface--error" role="alert">
            <Rss />
            <strong>{t("xiadown.rss.previewFailed")}</strong>
            <span>{previewError}</span>
            <Button
              disabled={add.isPending}
              onClick={() => {
                add.reset();
                setPreviewAttempt((current) => current + 1);
              }}
              type="button"
              variant="outline"
            >
              <RefreshCcw />
              {t("xiadown.rss.tryAgain")} · {t("xiadown.rss.previewSubscription")}
            </Button>
          </div>
        ) : preview ? (
          <>
            <p>{preview.subscription.description}</p>
            <div className="rss-discovery-preview-entries">
              {preview.entries.map((entry) => <article key={entry.id}><span /><div><strong>{entry.title}</strong><small>{entry.summary}</small></div></article>)}
            </div>
          </>
        ) : null}
      </div>
      <RSSSubscriptionSettingsFields
        title={title}
        viewType={viewType}
        onTitleChange={(value) => {
          setTitleEdited(true);
          setTitle(value);
        }}
        onViewTypeChange={setViewType}
      />
      <footer>
        {onBack ? <Button onClick={onBack} type="button" variant="ghost"><ArrowLeft />{t("xiadown.rss.editParameters")}</Button> : null}
        <span />
        <DialogClose asChild><Button type="button" variant="outline">{t("xiadown.rss.cancel")}</Button></DialogClose>
        <Button
          disabled={!title.trim() || add.isPending || subscribed}
          onClick={() => void subscribe()}
          type="button"
        >
          {add.isPending ? <LoaderCircle className="app-motion-spin" /> : subscribed ? <Check /> : <Plus />}
          {subscribed
            ? t("xiadown.rss.subscribed")
            : previewError
              ? <>{t("xiadown.rss.subscribe")} · {t("xiadown.rss.previewFailed")}</>
              : t("xiadown.rss.subscribe")}
        </Button>
      </footer>
      {add.error ? <p aria-live="assertive" className="rss-form-error" role="alert">{errorText(add.error)}</p> : null}
    </>
  );
}

function configurationErrorText(error: RSSDiscoveryRouteConfigurationError, t: TFunction) {
  switch (error.code) {
    case "required": return t("xiadown.rss.parameterRequired");
    case "optionalOrder": return t("xiadown.rss.parameterOptionalOrder");
    case "invalidOption": return t("xiadown.rss.parameterInvalidOption");
    case "invalidValue": return t("xiadown.rss.parameterInvalidValue");
    default: return t("xiadown.rss.parameterInvalidTemplate");
  }
}

function RSSRouteRequirementsNotice({ route, t }: { route: RSSDiscoveryRoute; t: TFunction }) {
  const notices = [
    route.requiresConfig ? t("xiadown.rss.configPreviewHint") : "",
    route.requiresPuppeteer ? t("xiadown.rss.puppeteerPreviewHint") : "",
  ].filter(Boolean);
  if (notices.length === 0) return null;
  return (
    <div className="rss-discovery-dialog__notice">
      <Settings2 />
      <span>{notices.map((notice) => <span key={notice}>{notice}</span>)}</span>
    </div>
  );
}

function humanizeParameterName(value: string) {
  return value
    .replace(/([a-z0-9])([A-Z])/g, "$1 $2")
    .replace(/[-_]+/g, " ")
    .trim()
    .replace(/^./, (character) => character.toUpperCase());
}

function subscriptionTitleFromURL(rawURL: string) {
  try {
    return new URL(rawURL).hostname.replace(/^www\./, "") || rawURL;
  } catch {
    return rawURL.trim();
  }
}

function viewTypeOptions(t: TFunction): Array<{ value: RSSViewType; label: string }> {
  return [
    { value: "auto", label: t("xiadown.rss.viewAuto") },
    { value: "article", label: t("xiadown.rss.articles") },
    { value: "social", label: t("xiadown.rss.socialMedia") },
    { value: "image", label: t("xiadown.workspace.images") },
    { value: "video", label: t("xiadown.rss.videos") },
  ];
}

function errorText(error: unknown) {
  return error instanceof Error ? error.message : String(error ?? "");
}

function fallbackRSSDialogFocusTarget() {
  return typeof document === "undefined"
    ? null
    : document.querySelector<HTMLButtonElement>('button[data-route-id="all"]');
}
