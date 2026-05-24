import * as React from "react";
import { System } from "@wailsio/runtime";
import {
  siBilibili,
  siFacebook,
  siInstagram,
  siNiconico,
  siTiktok,
  siTwitch,
  siVimeo,
  siX,
  siYoutube,
} from "simple-icons";
import {
  CircleOff,
  ExternalLink,
  Eye,
  FolderOpen,
  Globe2,
  Link2,
  Loader2,
  Panda,
  Plug2,
  RefreshCw,
  Search,
  Trash2,
  UserRound,
  X,
} from "lucide-react";

import { Button } from "@/shared/ui/button";
import { WindowControls } from "@/components/layout/WindowControls";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogListCard,
  DialogListCardContent,
  DialogRow,
  DialogScrollArea,
  DialogTitle,
} from "@/shared/ui/dialog";
import { Input } from "@/shared/ui/input";
import { Select } from "@/shared/ui/select";
import {
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/shared/ui/sidebar";
import {
  SETTINGS_ROW_CLASS,
  SETTINGS_ROW_LABEL_CLASS,
  SettingsSeparator,
} from "@/shared/ui/settings-layout";
import { useI18n } from "@/shared/i18n";
import {
  useCancelConnectorConnect,
  useClearConnector,
  useConnectorConnectSession,
  useFinishConnectorConnect,
  useConnectors,
  useOpenConnectorSite,
  useStartConnectorConnect,
} from "@/shared/query/connectors";
import { useOpenLibraryPath } from "@/shared/query/library";
import { useBrowserCandidates } from "@/shared/query/settings";
import { messageBus } from "@/shared/message";
import { formatBytes } from "@/shared/utils/formatBytes";
import type {
  Connector,
  ConnectorConnectSession,
  FinishConnectorConnectResult,
  StartConnectorConnectResult,
} from "@/shared/contracts/connectors";
import { cn } from "@/lib/utils";

const STATUS_META: Record<
  string,
  {
    statusKey: string;
    className: string;
    icon: React.ComponentType<{ className?: string }>;
  }
> = {
  connected: {
    statusKey: "connected",
    className:
      "app-connectors-status-badge app-connectors-status-connected",
    icon: Plug2,
  },
  expired: {
    statusKey: "expired",
    className:
      "app-connectors-status-badge app-connectors-status-expired",
    icon: RefreshCw,
  },
  disconnected: {
    statusKey: "disconnected",
    className: "app-connectors-status-badge app-connectors-status-disconnected",
    icon: CircleOff,
  },
  profile: {
    statusKey: "profile",
    className: "app-connectors-status-badge app-connectors-status-profile",
    icon: UserRound,
  },
};

type ConnectorMeta = {
  labelKey: string;
  fallbackLabel: string;
};

type ConnectorDialogMode = "connect" | "profile" | "open";

const CONNECTOR_META: Record<string, ConnectorMeta> = {
  youtube: {
    labelKey: "settings.connectors.item.youtube",
    fallbackLabel: "YouTube",
  },
  bilibili: {
    labelKey: "settings.connectors.item.bilibili",
    fallbackLabel: "Bilibili",
  },
  tiktok: {
    labelKey: "settings.connectors.item.tiktok",
    fallbackLabel: "TikTok",
  },
  china_private: {
    labelKey: "settings.connectors.item.chinaPrivate",
    fallbackLabel: "China private domain",
  },
  instagram: {
    labelKey: "settings.connectors.item.instagram",
    fallbackLabel: "Instagram",
  },
  x: {
    labelKey: "settings.connectors.item.x",
    fallbackLabel: "X / Twitter",
  },
  facebook: {
    labelKey: "settings.connectors.item.facebook",
    fallbackLabel: "Facebook",
  },
  vimeo: {
    labelKey: "settings.connectors.item.vimeo",
    fallbackLabel: "Vimeo",
  },
  twitch: {
    labelKey: "settings.connectors.item.twitch",
    fallbackLabel: "Twitch",
  },
  niconico: {
    labelKey: "settings.connectors.item.niconico",
    fallbackLabel: "Niconico",
  },
};

function resolveBrowserStatusKey(status: string) {
  switch (status) {
    case "not_open":
      return "notOpen";
    case "tab_closed":
      return "tabClosed";
    case "browser_closed":
      return "browserClosed";
    default:
      return status;
  }
}

const GENERAL_CARD_HEIGHT = "min-h-[240px]";
const CONNECTOR_BRAND_ICONS = {
  youtube: siYoutube,
  bilibili: siBilibili,
  tiktok: siTiktok,
  instagram: siInstagram,
  x: siX,
  facebook: siFacebook,
  vimeo: siVimeo,
  twitch: siTwitch,
  niconico: siNiconico,
} satisfies Record<string, { path: string; title: string }>;

const formatCookieExpires = (expires?: number) => {
  if (!expires || expires <= 0) {
    return "-";
  }
  const date = new Date(expires * 1000);
  if (Number.isNaN(date.getTime())) {
    return "-";
  }
  return date.toLocaleString();
};

const formatConnectorTemplate = (
  template: string,
  params: Record<string, string>,
) =>
  Object.entries(params).reduce(
    (text, [key, value]) => text.split(`{${key}}`).join(value),
    template,
  );

const resolveConnectorErrorMessage = (error: unknown, fallback: string) => {
  if (error instanceof Error && error.message.trim()) {
    return error.message;
  }
  if (typeof error === "string" && error.trim()) {
    return error;
  }
  return fallback;
};

const resolveConnectorMeta = (connectorType: string): ConnectorMeta | null => {
  const normalized = connectorType.trim().toLowerCase();
  if (!normalized) {
    return null;
  }
  return CONNECTOR_META[normalized] ?? null;
};

export function ConnectorBrandIcon(props: {
  connectorType?: string;
  className?: string;
  fallback?: "globe" | "none";
}) {
  const normalized = props.connectorType?.trim().toLowerCase() ?? "";
  if (normalized === "china_private") {
    return (
      <Panda
        className={cn("block shrink-0", props.className)}
        aria-hidden="true"
      />
    );
  }
  const icon = normalized
    ? CONNECTOR_BRAND_ICONS[normalized as keyof typeof CONNECTOR_BRAND_ICONS]
    : undefined;
  if (!icon) {
    if (props.fallback === "none") {
      return null;
    }
    return <Globe2 className={props.className} />;
  }
  return (
    <svg
      viewBox="0 0 24 24"
      fill="currentColor"
      aria-hidden="true"
      className={cn("block shrink-0", props.className)}
    >
      <path d={icon.path} />
    </svg>
  );
}

export function ConnectorsSection() {
  const { t } = useI18n();
  const isWindows = System.IsWindows();
  const connectors = useConnectors();
  const browserCandidates = useBrowserCandidates();
  const startConnectorConnect = useStartConnectorConnect();
  const finishConnectorConnect = useFinishConnectorConnect();
  const cancelConnectorConnect = useCancelConnectorConnect();
  const clearConnector = useClearConnector();
  const openConnectorSite = useOpenConnectorSite();
  const openProfilePath = useOpenLibraryPath();

  const [selectedId, setSelectedId] = React.useState<string | null>(null);
  const [query, setQuery] = React.useState("");
  const [loginDialogOpen, setLoginDialogOpen] = React.useState(false);
  const [loginTarget, setLoginTarget] = React.useState<Connector | null>(null);
  const [loginTargetURL, setLoginTargetURL] = React.useState("");
  const [loginSessionId, setLoginSessionId] = React.useState("");
  const [loginResult, setLoginResult] =
    React.useState<FinishConnectorConnectResult | null>(null);
  const [loginError, setLoginError] = React.useState("");
  const [loginDialogMode, setLoginDialogMode] =
    React.useState<ConnectorDialogMode>("connect");
  const [loginFinalBrowserStatus, setLoginFinalBrowserStatus] =
    React.useState("");
  const [loginClosing, setLoginClosing] = React.useState(false);
  const [cookiesDialogOpen, setCookiesDialogOpen] = React.useState(false);
  const [profileContentDialogOpen, setProfileContentDialogOpen] =
    React.useState(false);
  const [clearConfirmTarget, setClearConfirmTarget] =
    React.useState<Connector | null>(null);
  const [clearConfirmError, setClearConfirmError] = React.useState("");
  const [profileSiteURLByConnectorId, setProfileSiteURLByConnectorId] =
    React.useState<Record<string, string>>({});
  const loginStartTokenRef = React.useRef(0);
  const loginSessionIdRef = React.useRef("");
  const loginStartPromiseRef =
    React.useRef<Promise<StartConnectorConnectResult | null> | null>(null);
  const loginSession = useConnectorConnectSession(
    { sessionId: loginSessionId },
    loginDialogOpen && loginSessionId.trim().length > 0,
  );

  React.useEffect(() => {
    loginSessionIdRef.current = loginSessionId;
  }, [loginSessionId]);

  const items = connectors.data ?? [];
  const browserLabelById = React.useMemo(
    () =>
      new Map(
        (browserCandidates.data ?? []).map((candidate) => [
          candidate.id,
          candidate.label,
        ]),
      ),
    [browserCandidates.data],
  );
  const resolveBrowserLabel = React.useCallback(
    (browser?: string) => {
      const normalized = (browser ?? "").trim();
      if (!normalized) {
        return "-";
      }
      if (normalized === "default") {
        return t("settings.connectors.detail.profileBrowserDefault");
      }
      return (
        browserLabelById.get(normalized) ??
        normalized
          .split(/[-_\s]+/)
          .filter(Boolean)
          .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
          .join(" ")
      );
    },
    [browserLabelById, t],
  );
  const resolveConnectorLabel = React.useCallback(
    (connector: Connector) => {
      const meta = resolveConnectorMeta(connector.type);
      if (!meta) {
        return connector.type;
      }
      return t(meta.labelKey);
    },
    [t],
  );

  const trimmedQuery = query.trim().toLowerCase();
  const connectorUsesProfile = React.useCallback(
    (connector: Connector) => connector.credentialMode === "profile",
    [],
  );
  const resolveConnectorListSubtitle = React.useCallback(
    (connector: Connector) => {
      if (connector.type === "china_private") {
        return t("settings.connectors.item.chinaPrivateSites");
      }
      if (connector.credentialMode !== "profile") {
        return "";
      }
      return (connector.domains ?? []).join(" ");
    },
    [t],
  );
  const filteredItems = React.useMemo(() => {
    if (!trimmedQuery) {
      return items;
    }
    return items.filter((connector) => {
      const label = resolveConnectorLabel(connector).toLowerCase();
      const type = connector.type.toLowerCase();
      return label.includes(trimmedQuery) || type.includes(trimmedQuery);
    });
  }, [items, resolveConnectorLabel, trimmedQuery]);

  const sortedProfileItems = React.useMemo(
    () =>
      filteredItems
        .filter(connectorUsesProfile)
        .sort((left, right) =>
          resolveConnectorLabel(left).localeCompare(resolveConnectorLabel(right)),
        ),
    [connectorUsesProfile, filteredItems, resolveConnectorLabel],
  );
  const sortedCookieItems = React.useMemo(
    () =>
      filteredItems
        .filter((connector) => !connectorUsesProfile(connector))
        .sort((left, right) =>
          resolveConnectorLabel(left).localeCompare(resolveConnectorLabel(right)),
        ),
    [connectorUsesProfile, filteredItems, resolveConnectorLabel],
  );
  const sortedItems = React.useMemo(
    () => [...sortedProfileItems, ...sortedCookieItems],
    [sortedCookieItems, sortedProfileItems],
  );
  const connectorListGroups = React.useMemo(
    () =>
      [
        {
          key: "profile",
          label: t("settings.connectors.status.profile"),
          items: sortedProfileItems,
        },
        {
          key: "cookies",
          label: t("settings.connectors.cookiesTitle"),
          items: sortedCookieItems,
        },
      ].filter((group) => group.items.length > 0),
    [sortedCookieItems, sortedProfileItems, t],
  );

  const renderConnectorListItem = React.useCallback(
    (connector: Connector) => {
      const statusMeta =
        STATUS_META[
          connector.credentialState ??
            connector.status ??
            "disconnected"
        ] ?? STATUS_META.disconnected;
      const isSelected = connector.id === selectedId;
      const usesProfileCredential = connectorUsesProfile(connector);
      const subtitle = resolveConnectorListSubtitle(connector);
      return (
        <SidebarMenuItem key={connector.id}>
          <SidebarMenuButton
            type="button"
            isActive={isSelected}
            className={cn(
              "app-connectors-list-item justify-between",
              usesProfileCredential ? "min-h-[3.25rem]" : "min-h-11",
            )}
            onClick={() => setSelectedId(connector.id)}
          >
            <div className="flex min-w-0 items-center gap-3">
              <div
                className="app-connectors-icon-slot flex h-8 w-8 shrink-0 items-center justify-center"
                data-active={isSelected ? "true" : undefined}
              >
                <ConnectorBrandIcon
                  connectorType={connector.type}
                  className="h-5 w-5"
                />
              </div>
              <span className="flex min-w-0 flex-col gap-0.5">
                <span
                  className="app-connectors-list-label truncate text-sm font-medium leading-5 transition-colors"
                  data-active={isSelected ? "true" : undefined}
                >
                  {resolveConnectorLabel(connector)}
                </span>
                {usesProfileCredential ? (
                  <span
                    className="truncate text-[11px] font-medium leading-4 text-muted-foreground"
                    title={subtitle}
                  >
                    {subtitle}
                  </span>
                ) : null}
              </span>
            </div>
            <div className="shrink-0">
              <span
                className={statusMeta.className}
              >
                {React.createElement(statusMeta.icon, {
                  className: "h-3.5 w-3.5",
                })}
              </span>
            </div>
          </SidebarMenuButton>
        </SidebarMenuItem>
      );
    },
    [
      connectorUsesProfile,
      resolveConnectorLabel,
      resolveConnectorListSubtitle,
      selectedId,
    ],
  );

  React.useEffect(() => {
    if (selectedId && !items.some((item) => item.id === selectedId)) {
      setSelectedId(null);
    }
  }, [items, selectedId]);

  React.useEffect(() => {
    if (selectedId && sortedItems.some((item) => item.id === selectedId)) {
      return;
    }
    if (sortedItems.length > 0) {
      setSelectedId(sortedItems[0].id);
      return;
    }
    setSelectedId(null);
  }, [selectedId, sortedItems]);

  const selected = items.find((item) => item.id === selectedId) ?? null;
  const status =
    STATUS_META[selected?.credentialState ?? selected?.status ?? "disconnected"] ??
    STATUS_META.disconnected;

  const isBusy =
    startConnectorConnect.isPending ||
    finishConnectorConnect.isPending ||
    cancelConnectorConnect.isPending ||
    loginClosing ||
    openConnectorSite.isPending ||
    openProfilePath.isPending ||
    clearConnector.isPending;
  const isLoginStarting =
    startConnectorConnect.isPending || openConnectorSite.isPending;
  const isLoginRunning =
    isLoginStarting ||
    finishConnectorConnect.isPending ||
    cancelConnectorConnect.isPending ||
    loginClosing;
  const isOpenRunning = openConnectorSite.isPending;

  const resolveLoginError = React.useCallback(
    (error: unknown) => {
      const message = error instanceof Error ? error.message : String(error);
      if (message.toLowerCase().includes("no supported browser detected")) {
        return t("settings.connectors.browserMissing");
      }
      if (message.toLowerCase().includes("connector browser session ended")) {
        return t("settings.connectors.browserSessionEnded");
      }
      if (message.toLowerCase().includes("connector session not found")) {
        return t("settings.connectors.loginSessionMissing");
      }
      return error instanceof Error
        ? error.message
        : t("settings.connectors.loginError");
    },
    [t],
  );

  const resolveOpenError = React.useCallback(
    (error: unknown) => {
      const message = error instanceof Error ? error.message : String(error);
      if (message.toLowerCase().includes("no cookies")) {
        return t("settings.connectors.noCookies");
      }
      if (message.toLowerCase().includes("no supported browser detected")) {
        return t("settings.connectors.browserMissing");
      }
      return error instanceof Error
        ? error.message
        : t("settings.connectors.openSiteError");
    },
    [t],
  );

  const toLoginResult = React.useCallback(
    (session: ConnectorConnectSession): FinishConnectorConnectResult => {
      return {
        sessionId: session.sessionId,
        saved: session.saved,
        rawCookiesCount: session.rawCookiesCount,
        filteredCookiesCount: session.filteredCookiesCount,
        domains: session.domains,
        reason: session.reason,
        connector: session.connector,
      };
    },
    [],
  );

  const disposeLoginSession = React.useCallback(
    async (sessionId: string) => {
      const trimmed = sessionId.trim();
      if (!trimmed) {
        return;
      }
      try {
        await cancelConnectorConnect.mutateAsync({ sessionId: trimmed });
      } catch {
        // ignore disposal failures; a fresh connect attempt will replace stale sessions
      }
    },
    [cancelConnectorConnect],
  );

  const resetLoginState = React.useCallback(() => {
    setLoginDialogOpen(false);
    setLoginTarget(null);
    setLoginTargetURL("");
    setLoginSessionId("");
    setLoginResult(null);
    setLoginError("");
    setLoginDialogMode("connect");
    setLoginFinalBrowserStatus("");
    setLoginClosing(false);
  }, []);

  const handleDismissLogin = React.useCallback(async () => {
    if (loginClosing) {
      return;
    }
    loginStartTokenRef.current += 1;
    setLoginClosing(true);
    const pendingStart = loginStartPromiseRef.current;
    let startedSessionId = "";
    if (pendingStart) {
      const result = await pendingStart;
      startedSessionId = result?.sessionId ?? "";
    }
    const sessionId =
      startedSessionId.trim() || loginSessionIdRef.current.trim();
    if (sessionId) {
      await disposeLoginSession(sessionId);
    }
    resetLoginState();
  }, [disposeLoginSession, loginClosing, resetLoginState]);

  const startConnectorBrowserSession = async (
    connector: Connector,
    mode: ConnectorDialogMode,
    targetUrl?: string,
  ): Promise<StartConnectorConnectResult | null> => {
    const startToken = loginStartTokenRef.current + 1;
    const normalizedTargetURL = (targetUrl ?? "").trim();
    loginStartTokenRef.current = startToken;
    setLoginDialogMode(mode);
    setLoginTarget(connector);
    setLoginTargetURL(normalizedTargetURL);
    setLoginDialogOpen(true);
    setLoginSessionId("");
    setLoginResult(null);
    setLoginError("");
    setLoginFinalBrowserStatus("");
    setLoginClosing(false);
    const request = {
      id: connector.id,
      ...(normalizedTargetURL ? { targetUrl: normalizedTargetURL } : {}),
    };
    const startPromise =
      mode === "open"
        ? openConnectorSite.mutateAsync(request)
        : startConnectorConnect.mutateAsync(request);
    let guardedStartPromise: Promise<StartConnectorConnectResult | null>;
    guardedStartPromise = startPromise
      .then((result) => result)
      .catch((error) => {
        if (loginStartTokenRef.current === startToken) {
          setLoginError(
            mode === "open" ? resolveOpenError(error) : resolveLoginError(error),
          );
        }
        return null;
      })
      .finally(() => {
        if (loginStartPromiseRef.current === guardedStartPromise) {
          loginStartPromiseRef.current = null;
        }
      });
    loginStartPromiseRef.current = guardedStartPromise;
    try {
      const result = await guardedStartPromise;
      if (!result) {
        return null;
      }
      if (loginStartTokenRef.current !== startToken) {
        await disposeLoginSession(result.sessionId);
        return result;
      }
      setLoginTargetURL(result.targetUrl || normalizedTargetURL);
      setLoginSessionId(result.sessionId);
      return result;
    } catch (error) {
      if (loginStartTokenRef.current !== startToken) {
        return null;
      }
      setLoginError(resolveLoginError(error));
      return null;
    }
  };

  const handleConnect = async (connector: Connector) => {
    await startConnectorBrowserSession(connector, "connect");
  };

  const handleOpenProfileSite = async (connector: Connector, targetUrl?: string) => {
    await startConnectorBrowserSession(connector, "profile", targetUrl);
  };

  const handleFinishLogin = async () => {
    const finishToken = loginStartTokenRef.current;
    const sessionId = loginSessionId.trim();
    if (!sessionId) {
      setLoginError(t("settings.connectors.loginSessionMissing"));
      return;
    }
    setLoginError("");
    try {
      const result = await finishConnectorConnect.mutateAsync({ sessionId });
      if (loginStartTokenRef.current !== finishToken) {
        return;
      }
      setLoginResult(result);
      await disposeLoginSession(sessionId);
      if (!result.saved && result.connector.credentialMode !== "profile") {
        messageBus.publishToast({
          intent: "danger",
          title: t("settings.connectors.loginTitle"),
          description: t("settings.connectors.noCookiesRead"),
        });
      }
      resetLoginState();
    } catch (error) {
      setLoginError(resolveLoginError(error));
    }
  };

  React.useEffect(() => {
    const session = loginSession.data;
    if (!session || loginSessionId.trim().length === 0 || isLoginRunning) {
      return;
    }
    if (session.state === "running") {
      return;
    }

    const sessionId = session.sessionId;
    if (loginDialogMode === "open") {
      setLoginFinalBrowserStatus(session.browserStatus || session.state);
      if (session.error) {
        setLoginError(session.error);
      }
      return;
    }
    const isProfileSession =
      session.connector.credentialMode === "profile" ||
      loginDialogMode === "profile";
    setLoginResult(toLoginResult(session));
    setLoginFinalBrowserStatus(session.browserStatus || session.state);
    void disposeLoginSession(sessionId);
    setLoginSessionId("");

    if (session.state === "completed" && session.saved) {
      if (isProfileSession) {
        setLoginError("");
        return;
      }
      resetLoginState();
      return;
    }

    if (session.state === "completed" && !isProfileSession) {
      setLoginError(t("settings.connectors.noCookiesRead"));
      return;
    }

    if (session.error) {
      setLoginError(session.error);
      return;
    }

    setLoginError(t("settings.connectors.loginError"));
  }, [
    disposeLoginSession,
    isLoginRunning,
    loginSession.data,
    loginDialogMode,
    loginSessionId,
    resetLoginState,
    t,
    toLoginResult,
  ]);

  const handleOpenSite = async (connector: Connector) => {
    await startConnectorBrowserSession(connector, "open");
  };

  const rowClassName = SETTINGS_ROW_CLASS;
  const loginSessionData = loginSession.data ?? null;
  const loginBrowserStatus = isLoginStarting
    ? "opening"
    : loginSessionData?.browserStatus ||
      loginFinalBrowserStatus ||
      (loginSessionId ? "open" : "not_open");
  const loginConnectionLabel = loginTarget
    ? resolveConnectorLabel(loginTarget)
    : loginSessionData?.connector
      ? resolveConnectorLabel(loginSessionData.connector)
      : "-";
  const loginConnector = loginSessionData?.connector ?? loginTarget;
  const loginUsesProfile = loginConnector?.credentialMode === "profile";
  const loginDisplayTargetURL = (
    loginSessionData?.targetUrl ||
    loginTargetURL
  ).trim();
  const loginStatusRows = [
    {
      label: t("settings.connectors.loginCard.currentConnection"),
      value: loginConnectionLabel,
    },
    ...(loginDisplayTargetURL
      ? [
          {
            label: t("settings.connectors.loginCard.site"),
            value: loginDisplayTargetURL,
          },
        ]
      : []),
    {
      label: t("settings.connectors.loginCard.browserStatus"),
      value: t(
        `settings.connectors.browserStatus.${resolveBrowserStatusKey(loginBrowserStatus)}`,
      ),
    },
    ...(loginDialogMode === "open"
      ? []
      : [
          {
            label: loginUsesProfile
              ? t("settings.connectors.loginCard.profile")
              : t("settings.connectors.loginCard.currentCookiesCount"),
            value: loginUsesProfile
              ? t("settings.connectors.status.profile")
              : String(
                  loginSessionData?.currentCookiesCount ??
                    loginResult?.filteredCookiesCount ??
                    0,
                ),
          },
        ]),
  ];
  const selectedLabel = selected ? resolveConnectorLabel(selected) : "";
  const cookiesCount = selected?.cookiesCount ?? selected?.cookies?.length ?? 0;
  const cookiesList = selected?.cookies ?? [];
  const isConnected = (selected?.status ?? "disconnected") === "connected";
  const usesProfile = selected?.credentialMode === "profile";
  const hasOpenableCredential = usesProfile || cookiesCount > 0;
  const profileInfo = selected?.profileInfo ?? null;
  const profileExists = profileInfo?.exists === true;
  const profileComponents = profileInfo?.components ?? [];
  const profileBindings = profileInfo?.bindings ?? [];
  const profileSites = selected?.profileSites ?? [];
  const selectedProfileSiteURL = React.useMemo(() => {
    if (!selected || profileSites.length === 0) {
      return "";
    }
    const storedURL = (profileSiteURLByConnectorId[selected.id] ?? "").trim();
    if (storedURL && profileSites.some((site) => site.url === storedURL)) {
      return storedURL;
    }
    return profileSites[0]?.url ?? "";
  }, [profileSiteURLByConnectorId, profileSites, selected]);
  const profileScopeValues =
    selected?.domains?.filter(Boolean) ?? [];
  const activeProfileBrowser =
    selected?.profileBrowser || profileInfo?.browser || "";
  const activeProfileBrowserLabel = resolveBrowserLabel(activeProfileBrowser);
  const formatProfileCountLabel = React.useCallback(
    (fileCount?: number, directoryCount?: number) =>
      [
        fileCount
          ? t("settings.connectors.detail.profileFiles").replace(
              "{count}",
              String(fileCount),
            )
          : "",
        directoryCount
          ? t("settings.connectors.detail.profileFolders").replace(
              "{count}",
              String(directoryCount),
            )
          : "",
      ]
        .filter(Boolean)
        .join(" · "),
    [t],
  );
  const profileSizeLabel = formatBytes(profileInfo?.sizeBytes);
  const clearConfirmUsesProfile =
    clearConfirmTarget?.credentialMode === "profile";
  const clearConfirmTargetLabel = clearConfirmTarget
    ? resolveConnectorLabel(clearConfirmTarget)
    : "";
  const clearConfirmTitle = clearConfirmUsesProfile
    ? t("settings.connectors.profileClearConfirmTitle")
    : t("settings.connectors.clearConfirmTitle");
  const clearConfirmDescription = clearConfirmTarget
    ? formatConnectorTemplate(
        t(
          clearConfirmUsesProfile
            ? "settings.connectors.profileClearConfirmDescription"
            : "settings.connectors.clearConfirmDescription",
        ),
        { name: clearConfirmTargetLabel },
      )
    : "";

  const handleProfileSiteChange = (connector: Connector, targetUrl: string) => {
    setProfileSiteURLByConnectorId((current) => ({
      ...current,
      [connector.id]: targetUrl,
    }));
  };

  const handleOpenProfileFolder = async () => {
    const profilePath = (profileInfo?.path || selected?.profilePath || "").trim();
    if (!profilePath) {
      return;
    }
    try {
      await openProfilePath.mutateAsync({ path: profilePath });
    } catch (error) {
      messageBus.publishToast({
        intent: "danger",
        title: t("settings.connectors.detail.profileOpenFolder"),
        description:
          error instanceof Error
            ? error.message
            : t("settings.connectors.openSiteError"),
      });
    }
  };

  const executeClearConnector = async () => {
    const connector = clearConfirmTarget;
    if (!connector || clearConnector.isPending) {
      return;
    }
    setClearConfirmError("");
    try {
      await clearConnector.mutateAsync({ id: connector.id });
      setClearConfirmTarget(null);
    } catch (error) {
      setClearConfirmError(
        resolveConnectorErrorMessage(
          error,
          t("settings.connectors.clearError"),
        ),
      );
    }
  };

  return (
    <div className="app-main-page app-main-connectors-page flex min-h-0 min-w-0 flex-1 overflow-hidden bg-background">
      <aside className="app-main-list-pane app-connectors-list-pane flex min-h-0 w-[320px] shrink-0 flex-col border-r">
        <div
          className={cn(
            "px-4",
            isWindows
              ? "wails-drag flex min-h-[var(--app-page-top-drag-height)] items-center pb-3 pt-4"
              : "pb-4 pt-4",
          )}
        >
          <div
            className={cn(
              "app-dream-search-control app-dream-control-shell h-9 w-full min-w-0 px-3",
              isWindows ? "wails-drag" : "wails-no-drag",
            )}
          >
            <Search className="h-4 w-4 text-muted-foreground" />
            <Input
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder={t("settings.connectors.searchPlaceholder")}
              size="compact"
              className="app-control-input-compact h-auto rounded-none border-0 bg-transparent px-0 shadow-none"
            />
          </div>
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto px-3 py-3">
          {sortedItems.length === 0 ? (
            <div className="px-3 py-2 text-sm text-muted-foreground">
              {t("settings.connectors.searchEmpty")}
            </div>
          ) : (
            <SidebarMenu className="gap-1.5">
              {connectorListGroups.map((group) => (
                <React.Fragment key={group.key}>
                  <SidebarMenuItem>
                    <div className="px-2 pb-1 pt-2 text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground first:pt-0">
                      {group.label}
                    </div>
                  </SidebarMenuItem>
                  {group.items.map((connector) =>
                    renderConnectorListItem(connector),
                  )}
                </React.Fragment>
              ))}
            </SidebarMenu>
          )}
        </div>
      </aside>

      <section className="app-main-detail-pane app-connectors-detail-pane flex min-h-0 min-w-0 flex-1 flex-col">
        <div
          className={cn(
            "app-main-page-header app-connectors-detail-header wails-drag flex shrink-0 items-center justify-between border-b pl-5",
            isWindows
              ? "min-h-[var(--app-page-top-drag-height)] pb-3 pt-4 pr-0"
              : "pr-5",
          )}
        >
          <div
            className={cn(
              "flex min-w-0 flex-1 items-center gap-3 pr-3 text-sm",
              isWindows ? "min-h-9" : "min-h-[56px]",
            )}
          >
            {selected ? (
              <>
                <div className="app-connectors-detail-icon flex h-8 w-8 shrink-0 items-center justify-center">
                  <ConnectorBrandIcon
                    connectorType={selected.type}
                    className="h-5 w-5"
                  />
                </div>
                <span className="truncate text-sm font-semibold text-foreground">
                  {selectedLabel}
                </span>
              </>
            ) : (
              <span className="font-medium text-muted-foreground">
                {t("settings.connectors.headerRoot")}
              </span>
            )}
          </div>
          {isWindows ? <WindowControls platform="windows" /> : null}
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">
          {selected ? (
            <div
              className={cn(
                "flex h-full flex-col space-y-1.5",
                GENERAL_CARD_HEIGHT,
              )}
            >
              <div className={rowClassName}>
                <div className={SETTINGS_ROW_LABEL_CLASS}>
                  {t("settings.connectors.detail.status")}
                </div>
                <span
                  className={status.className}
                >
                  {React.createElement(status.icon, {
                    className: "h-3.5 w-3.5",
                  })}
                  {t(`settings.connectors.status.${status.statusKey}`)}
                </span>
              </div>

              <SettingsSeparator />

              <div className={rowClassName}>
                <div className={SETTINGS_ROW_LABEL_CLASS}>
                  {t("settings.connectors.detail.data")}
                </div>
                <div className="flex min-w-0 items-center justify-end gap-2">
                  {usesProfile && profileExists ? (
                    <>
                      <span className="max-w-[9rem] truncate text-xs font-medium text-muted-foreground">
                        {profileSizeLabel}
                      </span>
                      <Button
                        variant="outline"
                        size="compact"
                        onClick={() => setProfileContentDialogOpen(true)}
                        disabled={
                          profileComponents.length === 0 &&
                          profileBindings.length === 0
                        }
                      >
                        <Eye className="h-4 w-4" />
                        {t("settings.connectors.detail.profileViewContent")}
                      </Button>
                      <Button
                        variant="outline"
                        size="compactIcon"
                        onClick={() => void handleOpenProfileFolder()}
                        disabled={
                          openProfilePath.isPending ||
                          !(profileInfo?.path || selected.profilePath)
                        }
                        aria-label={t(
                          "settings.connectors.detail.profileOpenFolder",
                        )}
                        title={t("settings.connectors.detail.profileOpenFolder")}
                      >
                        {openProfilePath.isPending ? (
                          <Loader2 className="h-4 w-4 animate-spin" />
                        ) : (
                          <FolderOpen className="h-4 w-4" />
                        )}
                      </Button>
                    </>
                  ) : !usesProfile && cookiesCount > 0 ? (
                    <>
                      <span className="app-connectors-cookie-count">
                        {cookiesCount}
                      </span>
                      <Button
                        variant="outline"
                        size="compact"
                        onClick={() => setCookiesDialogOpen(true)}
                      >
                        <Eye className="h-4 w-4" />
                        {t("settings.connectors.viewCookies")}
                      </Button>
                    </>
                  ) : (
                    <span className="text-xs font-medium text-muted-foreground">
                      {usesProfile
                        ? t("settings.connectors.status.profile")
                        : t("settings.connectors.cookiesTitle")}
                    </span>
                  )}
                </div>
              </div>

              <SettingsSeparator />

              {usesProfile ? (
                <>
                  <div className={rowClassName}>
                    <div className={SETTINGS_ROW_LABEL_CLASS}>
                      {t("settings.connectors.detail.profileBrowser")}
                    </div>
                    <div className="flex min-w-0 flex-col items-end gap-0.5 text-right">
                      <span className="max-w-[14rem] truncate text-xs font-medium text-foreground">
                        {activeProfileBrowserLabel}
                      </span>
                      {profileBindings.length > 0 ? (
                        <span className="text-[11px] font-medium text-muted-foreground">
                          {t("settings.connectors.detail.profileBindings").replace(
                            "{count}",
                            String(profileBindings.length),
                          )}
                        </span>
                      ) : null}
                    </div>
                  </div>

                  <SettingsSeparator />
                </>
              ) : null}

              <div className={rowClassName}>
                <div className={SETTINGS_ROW_LABEL_CLASS}>
                  {t("settings.connectors.detail.scope")}
                </div>
                <div className="max-w-[60%] break-words text-right text-xs leading-5 text-muted-foreground [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:3]">
                  {profileScopeValues.length > 0
                    ? profileScopeValues.join(", ")
                    : "-"}
                </div>
              </div>

              <SettingsSeparator />

              <div className={rowClassName}>
                <div className={SETTINGS_ROW_LABEL_CLASS}>
                  {t("settings.connectors.detail.actions")}
                </div>
                <div className="flex flex-wrap items-center justify-end gap-2">
                  {usesProfile ? (
                    <>
                      {profileSites.length > 0 ? (
                        <Select
                          value={selectedProfileSiteURL}
                          onChange={(event) =>
                            handleProfileSiteChange(selected, event.target.value)
                          }
                          disabled={isBusy}
                          className="h-8 w-40 max-w-40 min-w-0 truncate whitespace-nowrap px-2"
                          aria-label={t("settings.connectors.detail.openSiteTarget")}
                          title={t("settings.connectors.detail.openSiteTarget")}
                        >
                          {profileSites.map((site) => (
                            <option key={site.url} value={site.url}>
                              {site.label || site.url}
                            </option>
                          ))}
                        </Select>
                      ) : null}
                      <Button
                        variant="outline"
                        size="compact"
                        onClick={() =>
                          handleOpenProfileSite(selected, selectedProfileSiteURL)
                        }
                        disabled={isBusy || !hasOpenableCredential}
                      >
                        {startConnectorConnect.isPending ? (
                          <Loader2 className="h-4 w-4 animate-spin" />
                        ) : (
                          <ExternalLink className="h-4 w-4" />
                        )}
                        {t("settings.connectors.openSite")}
                      </Button>
                    </>
                  ) : (
                    <>
                      <Button
                        variant="outline"
                        size="compact"
                        onClick={() => handleConnect(selected)}
                        disabled={isBusy}
                      >
                        {isLoginRunning ? (
                          <Loader2 className="h-4 w-4 animate-spin" />
                        ) : (
                          <Link2 className="h-4 w-4" />
                        )}
                        {isConnected
                          ? t("settings.connectors.reconnect")
                          : t("settings.connectors.connect")}
                      </Button>
                      <Button
                        variant="outline"
                        size="compact"
                        onClick={() => handleOpenSite(selected)}
                        disabled={isBusy || !hasOpenableCredential}
                      >
                        {isOpenRunning ? (
                          <Loader2 className="h-4 w-4 animate-spin" />
                        ) : (
                          <ExternalLink className="h-4 w-4" />
                        )}
                        {t("settings.connectors.openSite")}
                      </Button>
                    </>
                  )}
                  <Button
                    variant="outline"
                    size="compact"
                    onClick={() => {
                      setClearConfirmError("");
                      setClearConfirmTarget(selected);
                    }}
                    disabled={isBusy}
                  >
                    <Trash2 className="h-4 w-4" />
                    {usesProfile
                      ? t("settings.connectors.profileClear")
                      : t("settings.connectors.clear")}
                  </Button>
                </div>
              </div>
            </div>
          ) : (
            <div className="p-4 text-sm text-muted-foreground">
              {t("settings.connectors.empty")}
            </div>
          )}
        </div>
      </section>

      <Dialog
        open={loginDialogOpen}
        onOpenChange={(open) => {
          if (open) {
            setLoginDialogOpen(true);
            return;
          }
          void handleDismissLogin();
        }}
      >
        <DialogContent
          className="grid max-h-[min(32rem,calc(100vh-2rem))] w-[min(32rem,calc(100vw-2rem))] max-w-none grid-rows-[auto_minmax(0,1fr)_auto] gap-4 overflow-hidden"
          showCloseButton={false}
          onEscapeKeyDown={(event) => event.preventDefault()}
          onInteractOutside={(event) => event.preventDefault()}
        >
          <DialogHeader className="min-w-0">
            <DialogTitle className="overflow-hidden break-words pr-6 text-left leading-[1.35] [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:2]">
              {loginUsesProfile || loginDialogMode === "open"
                ? t("settings.connectors.openSite")
                : t("settings.connectors.loginTitle")}
            </DialogTitle>
          </DialogHeader>
          <DialogScrollArea className="min-h-0">
            <div className="grid gap-2">
              <DialogListCard className="app-connectors-info-card shadow-none">
                <DialogListCardContent>
                  {loginStatusRows.map((row) => (
                    <DialogRow
                      key={row.label}
                      className="app-connectors-info-row flex items-center justify-between gap-4 px-3 py-2.5 text-sm"
                    >
                      <span className="text-muted-foreground">{row.label}</span>
                      <span className="max-w-[55%] overflow-hidden break-words text-right font-medium leading-5 text-foreground [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:2]">
                        {row.value}
                      </span>
                    </DialogRow>
                  ))}
                </DialogListCardContent>
              </DialogListCard>
              {loginError ? (
                <div className="app-connectors-error p-2 text-xs leading-5 [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:4]">
                  {loginError}
                </div>
              ) : null}
            </div>
          </DialogScrollArea>
          <DialogFooter className="shrink-0">
            <Button
              variant="outline"
              onClick={() => void handleDismissLogin()}
              disabled={loginClosing || finishConnectorConnect.isPending}
            >
              {loginClosing || cancelConnectorConnect.isPending ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <X className="h-4 w-4" />
              )}
              {t("xiadown.actions.closeBrowser")}
            </Button>
            {loginDialogMode !== "open" ? (
              <Button
                onClick={() => void handleFinishLogin()}
                disabled={isLoginRunning || !loginSessionId}
              >
                {finishConnectorConnect.isPending ? (
                  <Loader2 className="h-4 w-4 animate-spin" />
                ) : (
                  <Link2 className="h-4 w-4" />
                )}
                {t("settings.connectors.loginFinish")}
              </Button>
            ) : null}
          </DialogFooter>
          </DialogContent>
      </Dialog>

      <Dialog
        open={Boolean(clearConfirmTarget)}
        onOpenChange={(open) => {
          if (clearConnector.isPending) {
            return;
          }
          if (!open) {
            setClearConfirmTarget(null);
            setClearConfirmError("");
          }
        }}
      >
        <DialogContent className="grid max-h-[calc(100vh-2rem)] w-[min(24rem,calc(100vw-2rem))] max-w-none grid-rows-[auto_minmax(0,1fr)_auto] gap-3 overflow-hidden">
          <DialogHeader className="min-w-0">
            <DialogTitle className="overflow-hidden break-words pr-6 text-left leading-[1.35] [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:2]">
              {clearConfirmTitle}
            </DialogTitle>
            <DialogDescription className="overflow-hidden break-words text-left text-sm leading-5 [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:3]">
              {clearConfirmDescription}
            </DialogDescription>
          </DialogHeader>

          <div className="min-h-0 overflow-hidden">
            {clearConfirmError ? (
              <div
                className="app-dream-status-message overflow-hidden break-words px-3 py-2 text-xs leading-5 [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:2]"
                data-intent="danger"
              >
                {clearConfirmError}
              </div>
            ) : null}
          </div>

          <DialogFooter className="flex-nowrap justify-between">
            <DialogClose asChild>
              <Button variant="outline" disabled={clearConnector.isPending}>
                {t("common.cancel")}
              </Button>
            </DialogClose>
            <Button
              variant="destructive"
              disabled={!clearConfirmTarget || clearConnector.isPending}
              onClick={() => void executeClearConnector()}
            >
              {clearConnector.isPending ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : null}
              {clearConfirmUsesProfile
                ? t("settings.connectors.profileClear")
                : t("settings.connectors.clear")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={profileContentDialogOpen}
        onOpenChange={setProfileContentDialogOpen}
      >
        <DialogContent className="grid max-h-[min(32rem,calc(100vh-2rem))] w-[min(38rem,calc(100vw-2rem))] max-w-none grid-rows-[auto_minmax(0,1fr)_auto] overflow-hidden">
          <DialogHeader className="min-w-0">
            <DialogTitle className="overflow-hidden break-words pr-6 text-left leading-[1.35] [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:2]">
              {selectedLabel
                ? t(
                    "settings.connectors.detail.profileContentDialogTitle",
                  ).replace("{name}", selectedLabel)
                : t("settings.connectors.detail.profileContent")}
            </DialogTitle>
          </DialogHeader>
          <DialogScrollArea className="min-h-0">
            <div className="grid gap-2">
              <DialogListCard className="shadow-none">
                <DialogListCardContent className="p-0">
                  {profileBindings.length === 0 ? (
                    <div className="p-4 text-sm text-muted-foreground">
                      {t("settings.connectors.detail.profileBindingEmpty")}
                    </div>
                  ) : (
                    profileBindings.map((binding) => (
                      <DialogRow
                        key={`${binding.browser}-${binding.path || ""}`}
                        className="grid grid-cols-[minmax(0,1fr)_auto_auto] items-center gap-3 px-3 py-2 text-xs"
                      >
                        <span
                          className="min-w-0 truncate whitespace-nowrap font-medium text-foreground"
                          title={binding.path || binding.browser}
                        >
                          {resolveBrowserLabel(binding.browser)}
                        </span>
                        <span className="shrink-0 whitespace-nowrap text-muted-foreground">
                          {binding.current
                            ? t("settings.connectors.detail.profileCurrent")
                            : binding.exists
                              ? t("settings.connectors.status.profile")
                              : t("settings.connectors.status.disconnected")}
                        </span>
                        <span className="shrink-0 whitespace-nowrap text-muted-foreground">
                          {binding.exists ? formatBytes(binding.sizeBytes) : "-"}
                        </span>
                      </DialogRow>
                    ))
                  )}
                </DialogListCardContent>
              </DialogListCard>
              <DialogListCard className="shadow-none">
                <DialogListCardContent className="p-0">
                  {profileComponents.length === 0 ? (
                    <div className="p-4 text-sm text-muted-foreground">
                      {t("settings.connectors.detail.profileContentEmpty")}
                    </div>
                  ) : (
                    profileComponents.map((component) => (
                      <DialogRow
                        key={`${component.path || component.name}-${component.kind || ""}`}
                        className="grid grid-cols-[minmax(0,1fr)_auto_auto] items-center gap-3 px-3 py-2 text-xs"
                      >
                        <span
                          className="min-w-0 truncate whitespace-nowrap font-medium text-foreground"
                          title={component.path || component.name}
                        >
                          {component.name}
                        </span>
                        <span className="shrink-0 whitespace-nowrap text-muted-foreground">
                          {formatBytes(component.sizeBytes)}
                        </span>
                        <span className="shrink-0 whitespace-nowrap text-muted-foreground">
                          {formatProfileCountLabel(
                            component.fileCount,
                            component.directoryCount,
                          ) || "-"}
                        </span>
                      </DialogRow>
                    ))
                  )}
                </DialogListCardContent>
              </DialogListCard>
            </div>
          </DialogScrollArea>
          <DialogFooter className="shrink-0">
            <Button
              variant="outline"
              onClick={() => setProfileContentDialogOpen(false)}
            >
              {t("common.close")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={cookiesDialogOpen} onOpenChange={setCookiesDialogOpen}>
        <DialogContent className="grid max-h-[min(40rem,calc(100vh-2rem))] w-[min(48rem,calc(100vw-2rem))] max-w-none grid-rows-[auto_minmax(0,1fr)_auto] overflow-hidden">
          <DialogHeader className="min-w-0">
            <DialogTitle className="overflow-hidden break-words pr-6 text-left leading-[1.35] [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:2]">
              {selectedLabel
                ? t("settings.connectors.cookiesDialogTitle").replace(
                    "{name}",
                    selectedLabel,
                  )
                : t("settings.connectors.cookiesTitle")}
            </DialogTitle>
          </DialogHeader>
          <div className="app-connectors-table-shell flex min-h-0 flex-col overflow-hidden">
            {cookiesList.length === 0 ? (
              <div className="p-4 text-sm text-muted-foreground">
                {t("settings.connectors.cookiesEmpty")}
              </div>
            ) : (
              <>
                <div className="app-connectors-table-header">
                  <table className="w-full table-fixed text-xs">
                    <colgroup>
                      <col className="w-[120px]" />
                      <col />
                      <col />
                      <col className="w-[60px]" />
                      <col className="w-[160px]" />
                      <col className="w-[60px]" />
                    </colgroup>
                    <thead>
                      <tr>
                        <th className="w-[120px] px-3 py-2 text-left font-medium text-muted-foreground">
                          {t("settings.connectors.cookieColumns.name")}
                        </th>
                        <th className="px-3 py-2 text-left font-medium text-muted-foreground">
                          {t("settings.connectors.cookieColumns.value")}
                        </th>
                        <th className="px-3 py-2 text-left font-medium text-muted-foreground">
                          {t("settings.connectors.cookieColumns.domain")}
                        </th>
                        <th className="w-[60px] px-3 py-2 text-left font-medium text-muted-foreground">
                          {t("settings.connectors.cookieColumns.path")}
                        </th>
                        <th className="w-[160px] px-3 py-2 text-left font-medium text-muted-foreground">
                          {t("settings.connectors.cookieColumns.expires")}
                        </th>
                        <th className="w-[60px] px-3 py-2 text-left font-medium text-muted-foreground">
                          {t("settings.connectors.cookieColumns.secure")}
                        </th>
                      </tr>
                    </thead>
                  </table>
                </div>
                <div className="min-h-0 flex-1 overflow-y-auto overflow-x-hidden">
                  <table className="w-full table-fixed text-xs">
                    <colgroup>
                      <col className="w-[120px]" />
                      <col />
                      <col />
                      <col className="w-[60px]" />
                      <col className="w-[160px]" />
                      <col className="w-[60px]" />
                    </colgroup>
                    <tbody>
                      {cookiesList.map((cookie, index) => (
                        <tr
                          key={`${cookie.name}-${cookie.domain}-${index}`}
                          className="app-connectors-table-row"
                        >
                          <td className="truncate px-3 py-2 font-medium text-foreground">
                            {cookie.name}
                          </td>
                          <td className="truncate px-3 py-2 text-muted-foreground">
                            {cookie.value}
                          </td>
                          <td className="truncate px-3 py-2 text-muted-foreground">
                            {cookie.domain}
                          </td>
                          <td className="truncate px-3 py-2 text-muted-foreground">
                            {cookie.path}
                          </td>
                          <td className="truncate px-3 py-2 text-muted-foreground">
                            {formatCookieExpires(cookie.expires)}
                          </td>
                          <td className="truncate px-3 py-2 text-muted-foreground">
                            {cookie.secure ? "Yes" : "No"}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </>
            )}
          </div>
          <DialogFooter className="shrink-0">
            <Button
              variant="outline"
              onClick={() => setCookiesDialogOpen(false)}
            >
              {t("common.close")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
