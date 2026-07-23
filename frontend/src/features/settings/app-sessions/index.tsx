import { Events } from "@wailsio/runtime";
import {
  AlertCircle,
  CircleOff,
  CloudSync,
  Loader2,
  LogIn,
  LogOut,
  RefreshCw,
  ShieldCheck,
  Trash2,
} from "lucide-react";
import * as React from "react";

import type {
  AppSession,
  AppSessionAccount,
  AppSessionBadge,
  StartAppSessionConnectResult,
} from "@/shared/contracts/appSessions";
import { type SupportedLanguage, type TFunction, useI18n } from "@/shared/i18n";
import {
  APP_SESSIONS_CHANGED_EVENT,
  useAppSessionConnectSession,
  useAppSessions,
  useClearAppSession,
  useStartAppSessionConnect,
  useVerifyAppSession,
} from "@/shared/query/appSessions";
import { Button } from "@/shared/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/shared/ui/dialog";
import {
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/shared/ui/sidebar";
import {
  SiteBrandIcon,
  siteBrandKey,
  siteBrandSurfaceStyle,
} from "@/shared/ui/site-brand-icon";
import {
  StatusBadge as DreamStatusBadge,
  type DreamStatusTone,
} from "@/shared/ui/status-badge";
import {
  WorkspacePage,
  WorkspacePageContent,
  WorkspacePageTopBar,
  defineWorkspacePageContract,
} from "@/shared/ui/workspace-page";
import { WorkspacePrimaryHeaderAction } from "@/shared/ui/workspace-primary-header-action";
import { AppSessionImportSheet } from "./AppSessionImportSheet";

type ActiveAppSessionBrowser = {
  sessionId: string;
  appSessionId: string;
};

const APP_SESSION_LABEL_KEYS = {
  searchPlaceholder: "settings.appSessions.searchPlaceholder",
  searchEmpty: "settings.appSessions.searchEmpty",
  empty: "settings.appSessions.empty",
  headerRoot: "settings.appSessions.headerRoot",
  connect: "settings.appSessions.connect",
  reconnect: "settings.appSessions.reconnect",
  openSite: "settings.appSessions.openSite",
  clear: "settings.appSessions.clear",
  finish: "settings.appSessions.finish",
  cancel: "settings.appSessions.cancel",
  status: "settings.appSessions.status",
  account: "settings.appSessions.account",
  handle: "settings.appSessions.handle",
  membership: "settings.appSessions.membership",
  badges: "settings.appSessions.badges",
  cookies: "settings.appSessions.cookies",
  domains: "settings.appSessions.domains",
  lastVerifiedAt: "settings.appSessions.lastVerifiedAt",
  actions: "settings.appSessions.actions",
  capabilities: "settings.appSessions.capabilities",
  providerUnsupported: "settings.appSessions.providerUnsupported",
  connectTitle: "settings.appSessions.connectTitle",
  openTitle: "settings.appSessions.openTitle",
  browserStatus: "settings.appSessions.browserStatus",
  currentCookies: "settings.appSessions.currentCookies",
  savedCookies: "settings.appSessions.savedCookies",
  clearConfirmTitle: "settings.appSessions.clearConfirmTitle",
  clearConfirmDescription: "settings.appSessions.clearConfirmDescription",
  clearError: "settings.appSessions.clearError",
  loginError: "settings.appSessions.loginError",
  noCookiesRead: "settings.appSessions.noCookiesRead",
  browserMissing: "settings.appSessions.browserMissing",
  browserSessionEnded: "settings.appSessions.browserSessionEnded",
  sessionMissing: "settings.appSessions.sessionMissing",
  connected: "settings.appSessions.connected",
  expired: "settings.appSessions.expired",
  disconnected: "settings.appSessions.disconnected",
  unsupported: "settings.appSessions.unsupported",
  siteDetailTitle: "settings.appSessions.siteDetailTitle",
  youtubeIdentity: "settings.appSessions.youtubeIdentity",
  youtubeMembership: "settings.appSessions.youtubeMembership",
  youtubeDomains: "settings.appSessions.youtubeDomains",
  bilibiliIdentity: "settings.appSessions.bilibiliIdentity",
  bilibiliMembership: "settings.appSessions.bilibiliMembership",
  bilibiliVip: "settings.appSessions.bilibiliVip",
  bilibiliAnnualVip: "settings.appSessions.bilibiliAnnualVip",
  bilibiliActiveVip: "settings.appSessions.bilibiliActiveVip",
  socialIdentity: "settings.appSessions.socialIdentity",
  genericIdentity: "settings.appSessions.genericIdentity",
  accountFallbackName: "settings.appSessions.accountFallbackName",
  disconnectedName: "settings.appSessions.disconnectedName",
  manualSignIn: "settings.appSessions.manualSignIn",
  signOut: "settings.appSessions.signOut",
  verifyLogin: "settings.appSessions.verifyLogin",
  verifyStatus: "settings.appSessions.verifyStatus",
  verificationVerified: "settings.appSessions.verificationVerified",
  verificationVerifying: "settings.appSessions.verificationVerifying",
  verificationUnverified: "settings.appSessions.verificationUnverified",
  verificationUnsupported: "settings.appSessions.verificationUnsupported",
  lastLoginAt: "settings.appSessions.lastLoginAt",
  expiresAt: "settings.appSessions.expiresAt",
  youtubeAccountFallbackName: "settings.appSessions.youtubeAccountFallbackName",
  youtubeDisconnectedName: "settings.appSessions.youtubeDisconnectedName",
  youtubeSignOut: "settings.appSessions.youtubeSignOut",
  youtubeVerifyLogin: "settings.appSessions.youtubeVerifyLogin",
  youtubeVerifyStatus: "settings.appSessions.youtubeVerifyStatus",
  youtubeLastLoginAt: "settings.appSessions.youtubeLastLoginAt",
  youtubeExpiresAt: "settings.appSessions.youtubeExpiresAt",
  source: "browserSource.source",
  sourceXiaDown: "browserSource.sourceXiaDown",
  browserSync: "browserSource.browserSync",
} as const;

type AppSessionLabels = Record<keyof typeof APP_SESSION_LABEL_KEYS, string>;

function useAppSessionLabels(t: TFunction): AppSessionLabels {
  return React.useMemo(() => {
    return Object.fromEntries(
      Object.entries(APP_SESSION_LABEL_KEYS).map(([key, value]) => [key, t(value)]),
    ) as AppSessionLabels;
  }, [t]);
}

const SITE_LABEL_KEYS: Record<string, string> = {
  youtube: "settings.appSessions.item.youtube",
  bilibili: "settings.appSessions.item.bilibili",
  douyin: "settings.appSessions.item.douyin",
  xiaohongshu: "settings.appSessions.item.xiaohongshu",
  tiktok: "settings.appSessions.item.tiktok",
  instagram: "settings.appSessions.item.instagram",
  x: "settings.appSessions.item.x",
  facebook: "settings.appSessions.item.facebook",
  vimeo: "settings.appSessions.item.vimeo",
  twitch: "settings.appSessions.item.twitch",
  niconico: "settings.appSessions.item.niconico",
};

const ACCOUNT_VERIFIABLE_SITE_KEYS = new Set([
  "youtube",
  "bilibili",
  "douyin",
  "xiaohongshu",
  "tiktok",
  "instagram",
  "x",
  "facebook",
  "vimeo",
  "twitch",
  "niconico",
]);

const STATUS_META: Record<
  string,
  {
    label: keyof Pick<AppSessionLabels, "connected" | "expired" | "disconnected" | "unsupported">;
    tone: DreamStatusTone;
    icon: React.ComponentType<{ className?: string }>;
  }
> = {
  connected: {
    label: "connected",
    tone: "success",
    icon: ShieldCheck,
  },
  expired: {
    label: "expired",
    tone: "warning",
    icon: RefreshCw,
  },
  disconnected: {
    label: "disconnected",
    tone: "muted",
    icon: CircleOff,
  },
  unsupported: {
    label: "unsupported",
    tone: "danger",
    icon: AlertCircle,
  },
};

function formatTemplate(template: string, params: Record<string, string>) {
  return Object.entries(params).reduce(
    (output, [key, value]) => output.split(`{${key}}`).join(value),
    template,
  );
}

function formatRelativeTime(value: string | undefined, language: SupportedLanguage) {
  const trimmed = value?.trim();
  if (!trimmed) {
    return "-";
  }
  const date = new Date(trimmed);
  if (Number.isNaN(date.getTime())) {
    return "-";
  }
  const deltaMs = date.getTime() - Date.now();
  const absoluteDeltaMs = Math.abs(deltaMs);
  const units = [
    ["year", 365 * 24 * 60 * 60 * 1000],
    ["month", 30 * 24 * 60 * 60 * 1000],
    ["week", 7 * 24 * 60 * 60 * 1000],
    ["day", 24 * 60 * 60 * 1000],
    ["hour", 60 * 60 * 1000],
    ["minute", 60 * 1000],
  ] as const satisfies readonly [Intl.RelativeTimeFormatUnit, number][];
  const formatter = new Intl.RelativeTimeFormat(language, { numeric: "auto" });
  if (absoluteDeltaMs < 60 * 1000) {
    return formatter.format(0, "second");
  }
  const [unit, unitMs] =
    units.find(([, ms]) => absoluteDeltaMs >= ms) ??
    units[units.length - 1];
  return formatter.format(Math.round(deltaMs / unitMs), unit);
}

function normalizeStatus(session?: AppSession | null) {
  if (!session?.providerSupported) {
    return "unsupported";
  }
  const status = (session.status || session.credentialState || "disconnected")
    .trim()
    .toLowerCase();
  return STATUS_META[status] ? status : "disconnected";
}

function normalizeAccountVerificationStatus(session?: AppSession | null) {
  const value = session?.accountVerificationStatus?.trim().toLowerCase() ?? "";
  switch (value) {
    case "verified":
    case "verifying":
    case "unsupported":
      return value;
    default:
      return "unverified";
  }
}

function accountVerificationStatusLabel(
  session: AppSession,
  labels: AppSessionLabels,
) {
  switch (normalizeAccountVerificationStatus(session)) {
    case "verified":
      return labels.verificationVerified;
    case "verifying":
      return labels.verificationVerifying;
    case "unsupported":
      return labels.verificationUnsupported;
    default:
      return labels.verificationUnverified;
  }
}

function resolveAccountName(siteKey: string, account?: AppSessionAccount | null) {
  return (
    normalizeDisplayName(siteKey, account?.displayName) ||
    normalizeHandle(siteKey, account?.handle) ||
    ""
  );
}

function normalizeDisplayName(siteKey: string, displayName?: string) {
  const trimmed = displayName?.trim() ?? "";
  if (!trimmed) {
    return "";
  }
  const normalized = trimmed.toLowerCase().replace(/[.!。！]+$/g, "").trim();
  if (siteKey === "facebook") {
    if (
      normalized === "facebook" ||
      normalized === "meta" ||
      normalized === "redirecting" ||
      normalized.includes("redirecting") ||
      normalized.includes("log in to facebook") ||
      normalized.includes("log into facebook")
    ) {
      return "";
    }
  }
  return trimmed;
}

function normalizeHandle(siteKey: string, handle?: string) {
  const trimmed = handle?.trim() ?? "";
  if (!trimmed) {
    return "";
  }
  if ((siteKey === "facebook" || siteKey === "niconico") && /^\d+$/.test(trimmed)) {
    return "";
  }
  if (
    siteKey === "facebook" &&
    /^(me|profile\.php|login|login\.php|checkpoint)$/i.test(trimmed)
  ) {
    return "";
  }
  return trimmed.startsWith("@") ? trimmed : `@${trimmed}`;
}

function resolveBilibiliTierLabel(tierKey: string, labels: AppSessionLabels) {
  switch (tierKey.trim()) {
    case "vip":
      return labels.bilibiliVip;
    case "vip_annual":
      return labels.bilibiliAnnualVip;
    case "vip_active":
      return labels.bilibiliActiveVip;
    default:
      return "";
  }
}

function resolveAccountTierLabel(
  account: AppSessionAccount | null | undefined,
  siteKey: string,
  labels: AppSessionLabels,
) {
  const tierKey = account?.tierKey?.trim() ?? "";
  const tierLabel = account?.tierLabel?.trim() ?? "";
  if (siteKey === "bilibili") {
    return resolveBilibiliTierLabel(tierKey, labels) || tierLabel || tierKey;
  }
  return tierLabel || tierKey;
}

function resolveAccountBadgeLabel(
  badge: AppSessionBadge,
  siteKey: string,
  labels: AppSessionLabels,
) {
  const badgeKey = badge.key?.trim() ?? "";
  if (siteKey === "bilibili") {
    if (resolveBilibiliTierLabel(badgeKey, labels)) {
      return "";
    }
  }
  return badge.label?.trim() || badgeKey;
}

function resolveDialogError(error: unknown, labels: AppSessionLabels) {
  const message = error instanceof Error ? error.message : String(error ?? "");
  const normalized = message.toLowerCase();
  if (normalized.includes("no supported browser detected")) {
    return labels.browserMissing;
  }
  if (normalized.includes("browser session ended") || normalized.includes("window closed")) {
    return labels.browserSessionEnded;
  }
  if (normalized.includes("session not found")) {
    return labels.sessionMissing;
  }
  if (normalized.includes("no cookies")) {
    return labels.noCookiesRead;
  }
  return message.trim() || labels.loginError;
}

function AppSessionStatusBadge(props: {
  session?: AppSession | null;
  labels: AppSessionLabels;
}) {
  const key = normalizeStatus(props.session);
  const meta = STATUS_META[key] ?? STATUS_META.disconnected;
  const label = props.labels[meta.label];
  const Icon = meta.icon;
  return (
    <DreamStatusBadge
      aria-label={label}
      icon={<Icon />}
      iconOnly
      title={label}
      tone={meta.tone}
    />
  );
}

function AppSessionInfoRow(props: {
  label: React.ReactNode;
  value: React.ReactNode;
}) {
  return (
    <div className="app-sessions-info-row grid min-w-0 items-center gap-3">
      <span className="app-sessions-info-label min-w-0 truncate">
        {props.label}
      </span>
      <span className="app-sessions-info-value min-w-0 truncate">
        {props.value}
      </span>
    </div>
  );
}

function AppSessionAccountDetail(props: {
  session: AppSession;
  siteLabel: string;
  isConnected: boolean;
  isBusy: boolean;
  isLoginRunning: boolean;
  isVerifyRunning: boolean;
  canVerify: boolean;
  actionError: string;
  labels: AppSessionLabels;
  language: SupportedLanguage;
  onConnect: () => void;
  onBrowserSync: () => void;
  onVerify: () => void;
  onClear: () => void;
}) {
  const [avatarFailed, setAvatarFailed] = React.useState(false);
  const account = props.session.account;
  const siteKey = props.session.siteKey.trim().toLowerCase();
  const isYouTube = siteKey === "youtube";
  const canResolveAccountInfo = ACCOUNT_VERIFIABLE_SITE_KEYS.has(siteKey);
  const tierLabel = resolveAccountTierLabel(account, siteKey, props.labels);
  const badgeLabels = props.isConnected
    ? (account?.badges ?? [])
        .map((badge) => resolveAccountBadgeLabel(badge, siteKey, props.labels))
        .filter((value): value is string => Boolean(value))
    : [];
  const accountDisplayName = props.isConnected
    ? normalizeDisplayName(siteKey, account?.displayName)
    : "";
  const displayName = props.isConnected
    ? accountDisplayName ||
      (isYouTube ? props.labels.youtubeAccountFallbackName : props.siteLabel || props.labels.accountFallbackName)
    : isYouTube
      ? props.labels.youtubeDisconnectedName
      : canResolveAccountInfo
        ? props.labels.disconnectedName
        : props.siteLabel || props.labels.disconnectedName;
  const avatarURL = props.isConnected ? account?.avatarURL?.trim() ?? "" : "";
  const accountHandle = props.isConnected ? account?.handle?.trim() ?? "" : "";
  const normalizedHandle = normalizeHandle(siteKey, accountHandle);
  const signOutLabel = isYouTube ? props.labels.youtubeSignOut : props.labels.signOut;
  const verifyLoginLabel = isYouTube ? props.labels.youtubeVerifyLogin : props.labels.verifyLogin;
  const verifyLabel = isYouTube ? props.labels.youtubeVerifyStatus : props.labels.verifyStatus;
  const verificationStatus = normalizeAccountVerificationStatus(props.session);
  const verificationStatusText = accountVerificationStatusLabel(props.session, props.labels);
  const verificationError =
    props.isConnected && verificationStatus === "unverified"
      ? props.session.accountVerificationError?.trim() ?? ""
      : "";
  const lastLoginLabelKey = isYouTube ? props.labels.youtubeLastLoginAt : props.labels.lastLoginAt;
  const expiresAtLabelKey = isYouTube ? props.labels.youtubeExpiresAt : props.labels.expiresAt;
  const lastLoginLabel = formatRelativeTime(
    props.session.lastVerifiedAt,
    props.language,
  );
  const expiresAtLabel = formatRelativeTime(
    account?.expiresAt,
    props.language,
  );
  const sourceProfileLabel = props.session.source?.profileLabel?.trim() ?? "";
  const explicitSourceLabel = [
    props.session.source?.browserLabel?.trim(),
    /^profile-[0-9a-f]{16,}$/i.test(sourceProfileLabel) ? "" : sourceProfileLabel,
  ]
    .filter((value): value is string => Boolean(value))
    .join(" / ");
  const sourceLabel = explicitSourceLabel ||
    (props.isConnected || normalizeStatus(props.session) === "expired"
      ? props.labels.sourceXiaDown
      : "");

  React.useEffect(() => {
    setAvatarFailed(false);
  }, [avatarURL]);

  return (
    <div className="flex min-h-full min-w-0 items-center justify-center px-5 py-8">
      <div className="app-sessions-account-detail-content flex w-full flex-col items-center">
        <div
          className="app-session-account-avatar app-site-brand-surface mb-5 flex h-24 w-24 items-center justify-center overflow-hidden"
          data-site={siteBrandKey(props.session.siteKey)}
          style={siteBrandSurfaceStyle(props.session.siteKey)}
        >
          {avatarURL && !avatarFailed ? (
            <img
              src={avatarURL}
              alt={displayName}
              className="h-full w-full object-cover"
              referrerPolicy="no-referrer"
              onError={() => setAvatarFailed(true)}
            />
          ) : (
            <SiteBrandIcon siteKey={props.session.siteKey} className="h-12 w-12" />
          )}
        </div>
        <h2 className="app-sessions-account-title max-w-full truncate">
          {displayName}
        </h2>
        {normalizedHandle ? (
          <p className="app-sessions-account-handle mt-1 max-w-full truncate">
            {normalizedHandle}
          </p>
        ) : null}
        {sourceLabel ? (
          <p className="app-sessions-account-source mt-2 max-w-full truncate px-2.5 py-1">
            {props.labels.source}: {sourceLabel}
          </p>
        ) : null}

        {!props.session.providerSupported ? (
          <div className="app-sessions-error mt-5 w-full p-2">
            {props.labels.providerUnsupported}
          </div>
        ) : null}
        {props.actionError ? (
          <div className="app-sessions-error mt-5 w-full p-2">
            {props.actionError}
          </div>
        ) : null}

        <div className="mt-8 flex w-full flex-col gap-2">
          {props.isConnected ? (
            <div className="grid w-full grid-cols-2 gap-2">
              <Button
                type="button"
                onClick={props.onVerify}
                disabled={props.isBusy || verificationStatus === "verifying" || !props.canVerify}
                className="app-sessions-account-action min-w-0"
              >
                {props.isVerifyRunning || verificationStatus === "verifying" ? (
                  <Loader2 className="h-4 w-4 app-motion-spin" />
                ) : (
                  <ShieldCheck className="h-4 w-4" />
                )}
                <span className="truncate">
                  {verifyLoginLabel}
                </span>
              </Button>
              <Button
                type="button"
                variant="destructive"
                onClick={props.onClear}
                disabled={props.isBusy || !props.session.providerSupported}
                className="app-sessions-account-action min-w-0"
              >
                <LogOut className="h-4 w-4" />
                <span className="truncate">
                  {signOutLabel}
                </span>
              </Button>
            </div>
          ) : (
            <div className="grid w-full grid-cols-2 gap-2">
              <Button
                type="button"
                variant="outline"
                onClick={props.onConnect}
                disabled={props.isBusy || !props.session.providerSupported}
                className="app-sessions-account-action min-w-0"
              >
                {props.isLoginRunning ? (
                  <Loader2 className="h-4 w-4 app-motion-spin" />
                ) : (
                  <LogIn className="h-4 w-4" />
                )}
                <span className="truncate">{props.labels.manualSignIn}</span>
              </Button>
              <Button
                type="button"
                onClick={props.onBrowserSync}
                disabled={props.isBusy || !props.session.providerSupported}
                className="app-sessions-account-action min-w-0"
              >
                <CloudSync className="h-4 w-4" />
                <span className="truncate">{props.labels.browserSync}</span>
              </Button>
            </div>
          )}
          {props.isConnected ? (
            <div className="app-sessions-account-info mt-3 w-full p-3">
              <div className="app-sessions-account-info-grid grid gap-2">
                {canResolveAccountInfo ? (
                  <AppSessionInfoRow
                    label={verifyLabel}
                    value={
                      <span className="inline-flex min-w-0 items-center justify-end gap-1.5">
                        {verificationStatus === "verifying" ? (
                          <Loader2 className="h-3.5 w-3.5 shrink-0 app-motion-spin" />
                        ) : null}
                        <span className="truncate">{verificationStatusText}</span>
                      </span>
                    }
                  />
                ) : null}
                {canResolveAccountInfo && verificationError ? (
                  <div className="app-dream-status-message min-w-0 px-2 py-1.5 break-words" data-intent="danger">
                    {verificationError}
                  </div>
                ) : null}
                <AppSessionInfoRow label={lastLoginLabelKey} value={lastLoginLabel} />
                <AppSessionInfoRow label={expiresAtLabelKey} value={expiresAtLabel} />
                {tierLabel ? (
                  <AppSessionInfoRow label={props.labels.membership} value={tierLabel} />
                ) : null}
                {badgeLabels.length > 0 ? (
                  <AppSessionInfoRow label={props.labels.badges} value={badgeLabels.join(" / ")} />
                ) : null}
              </div>
            </div>
          ) : null}
        </div>
      </div>
    </div>
  );
}

function ClearConfirmDialog(props: {
  target: AppSession | null;
  label: string;
  labels: AppSessionLabels;
  error: string;
  isClearing: boolean;
  onCancel: () => void;
  onConfirm: () => void;
}) {
  return (
    <Dialog open={props.target !== null} onOpenChange={(open) => {
      if (!open) {
        props.onCancel();
      }
    }}>
      <DialogContent className="max-w-sm">
        <DialogHeader>
          <DialogTitle>{props.labels.clearConfirmTitle}</DialogTitle>
          <DialogDescription>
            {formatTemplate(props.labels.clearConfirmDescription, {
              name: props.label,
            })}
          </DialogDescription>
        </DialogHeader>
        {props.error ? (
          <div className="app-sessions-error p-2">
            {props.error}
          </div>
        ) : null}
        <DialogFooter>
          <DialogClose asChild>
            <Button type="button" variant="outline" disabled={props.isClearing}>
              {props.labels.cancel}
            </Button>
          </DialogClose>
          <Button
            type="button"
            variant="destructive"
            disabled={props.isClearing}
            onClick={props.onConfirm}
          >
            {props.isClearing ? <Loader2 className="h-4 w-4 app-motion-spin" /> : <Trash2 className="h-4 w-4" />}
            {props.labels.clear}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export function AppSessionsSection({
  reserveWindowControls = false,
}: {
  reserveWindowControls?: boolean;
}) {
  const { t, language } = useI18n();
  const labels = useAppSessionLabels(t);
  const sessionsQuery = useAppSessions();
  const startConnect = useStartAppSessionConnect();
  const verifySession = useVerifyAppSession();
  const clearSession = useClearAppSession();

  const [selectedId, setSelectedId] = React.useState<string | null>(null);
  const [importSheetOpen, setImportSheetOpen] = React.useState(false);
  const [activeBrowser, setActiveBrowser] =
    React.useState<ActiveAppSessionBrowser | null>(null);
  const [actionError, setActionError] = React.useState("");
  const [clearTarget, setClearTarget] = React.useState<AppSession | null>(null);
  const [clearError, setClearError] = React.useState("");

  const connectSession = useAppSessionConnectSession(
    { sessionId: activeBrowser?.sessionId ?? "" },
    activeBrowser !== null,
  );

  React.useEffect(() => {
    const offAppSessionsChanged = Events.On(APP_SESSIONS_CHANGED_EVENT, () => {
      void sessionsQuery.refetch();
    });
    return () => offAppSessionsChanged();
  }, [sessionsQuery.refetch]);

  const items = sessionsQuery.data ?? [];
  const resolveLabel = React.useCallback(
    (session: AppSession) => {
      const siteKey = session.siteKey.trim().toLowerCase();
      const i18nKey = SITE_LABEL_KEYS[siteKey];
      return session.label?.trim() || (i18nKey ? t(i18nKey) : "") || siteKey || session.id;
    },
    [t],
  );

  const sortedItems = React.useMemo(() => {
    return [...items].sort((left, right) => {
      const labelOrder = resolveLabel(left).localeCompare(
        resolveLabel(right),
        language,
      );
      return labelOrder || left.id.localeCompare(right.id);
    });
  }, [items, language, resolveLabel]);

  React.useEffect(() => {
    if (selectedId && sortedItems.some((item) => item.id === selectedId)) {
      return;
    }
    setSelectedId(sortedItems[0]?.id ?? null);
  }, [sortedItems, selectedId]);

  React.useEffect(() => {
    setActionError("");
  }, [selectedId]);

  const selected = items.find((item) => item.id === selectedId) ?? null;
  const selectedLabel = selected ? resolveLabel(selected) : "";
  const selectedIsConnected = normalizeStatus(selected) === "connected";
  const hasSelectedProvider = selected?.providerSupported === true;
  const selectedActiveBrowser =
    selected && activeBrowser?.appSessionId === selected.id ? activeBrowser : null;
  const isBusy =
    startConnect.isPending ||
    verifySession.isPending ||
    activeBrowser !== null ||
    clearSession.isPending;

  const beginConnect = React.useCallback(
    async (session: AppSession) => {
      setActionError("");
      try {
        const result: StartAppSessionConnectResult = await startConnect.mutateAsync({ id: session.id });
        setActiveBrowser({
          sessionId: result.sessionId,
          appSessionId: result.appSession.id || session.id,
        });
      } catch (error) {
        setActionError(resolveDialogError(error, labels));
      }
    },
    [labels, startConnect],
  );

  const verifyAccount = React.useCallback(
    async (session: AppSession) => {
      setActionError("");
      try {
        await verifySession.mutateAsync({ id: session.id });
      } catch (error) {
        setActionError(resolveDialogError(error, labels));
      }
    },
    [labels, verifySession],
  );

  React.useEffect(() => {
    if (!activeBrowser) {
      return;
    }
    if (connectSession.isError) {
      setActiveBrowser(null);
      void sessionsQuery.refetch();
      return;
    }
    const browserStatus = connectSession.data?.browserStatus?.trim().toLowerCase();
    const state = connectSession.data?.state?.trim().toLowerCase();
    if (!browserStatus && !state) {
      return;
    }
    if (browserStatus !== "open" || state === "completed" || state === "failed") {
      setActiveBrowser(null);
      void sessionsQuery.refetch();
    }
  }, [
    activeBrowser,
    connectSession.data?.browserStatus,
    connectSession.data?.state,
    connectSession.isError,
    sessionsQuery.refetch,
  ]);

  const confirmClear = React.useCallback(async () => {
    if (!clearTarget) {
      return;
    }
    setClearError("");
    try {
      await clearSession.mutateAsync({ id: clearTarget.id });
      setClearTarget(null);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error ?? "");
      setClearError(message.trim() || labels.clearError);
    }
  }, [clearSession, clearTarget, labels.clearError]);

  const renderListItem = React.useCallback(
    (session: AppSession) => {
      const isSelected = session.id === selectedId;
      const label = resolveLabel(session);
      const siteKey = session.siteKey.trim().toLowerCase();
      const isConnected = normalizeStatus(session) === "connected";
      const accountName = isConnected ? resolveAccountName(siteKey, session.account) : "";
      const canResolveAccountInfo = ACCOUNT_VERIFIABLE_SITE_KEYS.has(siteKey);
      const secondaryLabel = accountName && accountName !== label
        ? accountName
        : isConnected && canResolveAccountInfo
          ? accountVerificationStatusLabel(session, labels)
          : "";
      return (
        <SidebarMenuItem key={session.id}>
          <SidebarMenuButton
            type="button"
            isActive={isSelected}
            className="app-sessions-list-item min-h-[3.25rem] justify-between"
            onClick={() => setSelectedId(session.id)}
          >
            <div className="flex min-w-0 items-center gap-3">
              <div
                className="app-sessions-icon-slot app-site-brand-surface flex h-8 w-8 shrink-0 items-center justify-center"
                data-active={isSelected ? "true" : undefined}
                data-site={siteBrandKey(session.siteKey)}
                style={siteBrandSurfaceStyle(session.siteKey)}
              >
                <SiteBrandIcon siteKey={session.siteKey} className="h-5 w-5" />
              </div>
              <span className="flex min-w-0 flex-col gap-0.5">
                <span
                  className="app-sessions-list-label truncate"
                  data-active={isSelected ? "true" : undefined}
                >
                  {label}
                </span>
                {secondaryLabel ? (
                  <span className="app-sessions-list-secondary truncate">
                    {secondaryLabel}
                  </span>
                ) : null}
              </span>
            </div>
            <div className="shrink-0">
              <AppSessionStatusBadge session={session} labels={labels} />
            </div>
          </SidebarMenuButton>
        </SidebarMenuItem>
      );
    },
    [labels, resolveLabel, selectedId],
  );

  const pageContract = defineWorkspacePageContract({
    presentation: "primary",
    recipe: "operational",
    routeLabel: labels.headerRoot,
    topBar: "actions",
    heading: "assistive",
    contentLayout: "split",
    footer: "none",
    scroll: "panes",
    density: "compact",
    immersion: "edge-to-edge",
  });

  return (
    <WorkspacePage
      contract={pageContract}
      className="app-main-page app-main-app-sessions-page"
    >
      <WorkspacePageTopBar
        actionsLabel={labels.headerRoot}
        className="app-sessions-page-topbar"
        reserveWindowControls={reserveWindowControls}
      >
        <div className="app-sessions-page-topbar-leading app-workspace-primary-subpane app-workspace-primary-subpane--leading justify-start gap-3">
          <WorkspacePrimaryHeaderAction
            className="app-sessions-sync-title-action"
            label={t("browserSource.syncAction")}
            onClick={() => setImportSheetOpen(true)}
          >
            <CloudSync className="h-4 w-4" aria-hidden="true" />
          </WorkspacePrimaryHeaderAction>
        </div>
      </WorkspacePageTopBar>

      <WorkspacePageContent className="app-sessions-split flex min-h-0 min-w-0 overflow-hidden p-0">
        <aside
          aria-label={labels.headerRoot}
          className="app-main-list-pane app-sessions-list-pane app-workspace-primary-subpane app-workspace-primary-subpane--leading flex min-h-0 shrink-0 flex-col"
        >
          <div
            className="min-h-0 flex-1 overflow-y-auto px-3 py-3"
            data-scroll-owner="true"
            data-scroll-pane="app-sessions-list"
          >
            {sessionsQuery.isLoading ? (
              <div
                aria-label={labels.headerRoot}
                className="app-sessions-list-feedback flex items-center px-3 py-2"
                role="status"
              >
                <Loader2 className="h-4 w-4 app-motion-spin" />
              </div>
            ) : sortedItems.length === 0 ? (
              <div className="app-sessions-list-feedback px-3 py-2">
                {labels.empty}
              </div>
            ) : (
              <SidebarMenu className="gap-1.5">
                {sortedItems.map((session) => renderListItem(session))}
              </SidebarMenu>
            )}
          </div>
        </aside>

        <section
          aria-label={selected ? selectedLabel : labels.headerRoot}
          className="app-main-detail-pane app-sessions-detail-pane app-workspace-primary-subpane flex min-h-0 min-w-0 flex-1 flex-col"
        >
          <div
            className="min-h-0 flex-1 overflow-y-auto px-5 py-4"
            data-scroll-owner="true"
            data-scroll-pane="app-sessions-detail"
          >
            {selected ? (
              <AppSessionAccountDetail
                session={selected}
                siteLabel={selectedLabel}
                isConnected={selectedIsConnected}
                isBusy={isBusy}
                isLoginRunning={
                  startConnect.isPending ||
                  selectedActiveBrowser !== null
                }
                isVerifyRunning={verifySession.isPending}
                canVerify={hasSelectedProvider && ACCOUNT_VERIFIABLE_SITE_KEYS.has(selected.siteKey.trim().toLowerCase())}
                actionError={actionError}
                labels={labels}
                language={language}
                onConnect={() => void beginConnect(selected)}
                onBrowserSync={() => setImportSheetOpen(true)}
                onVerify={() => void verifyAccount(selected)}
                onClear={() => {
                  setClearError("");
                  setClearTarget(selected);
                }}
              />
            ) : (
              <div className="app-sessions-empty-detail flex h-full items-center justify-center">
                {labels.empty}
              </div>
            )}
          </div>
        </section>
      </WorkspacePageContent>

      <ClearConfirmDialog
        target={clearTarget}
        label={clearTarget ? resolveLabel(clearTarget) : selectedLabel}
        labels={labels}
        error={clearError}
        isClearing={clearSession.isPending}
        onCancel={() => {
          setClearTarget(null);
          setClearError("");
        }}
        onConfirm={() => void confirmClear()}
      />
      <AppSessionImportSheet
        open={importSheetOpen}
        onOpenChange={setImportSheetOpen}
        sessions={items}
        resolveLabel={resolveLabel}
        onImported={() => void sessionsQuery.refetch()}
      />
    </WorkspacePage>
  );
}
