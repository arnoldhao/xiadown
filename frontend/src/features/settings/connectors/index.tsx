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
  Globe2,
  Link2,
  Loader2,
  Plug2,
  RefreshCw,
  Search,
  Trash2,
} from "lucide-react";

import { Button } from "@/shared/ui/button";
import { Card, CardContent } from "@/shared/ui/card";
import { WindowControls } from "@/components/layout/WindowControls";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/shared/ui/dialog";
import { Input } from "@/shared/ui/input";
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
import { messageBus } from "@/shared/message";
import type {
  Connector,
  ConnectorConnectSession,
  FinishConnectorConnectResult,
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
      "bg-emerald-100 text-emerald-800 dark:bg-emerald-900/60 dark:text-emerald-100",
    icon: Plug2,
  },
  expired: {
    statusKey: "expired",
    className:
      "bg-amber-100 text-amber-800 dark:bg-amber-900/60 dark:text-amber-100",
    icon: RefreshCw,
  },
  disconnected: {
    statusKey: "disconnected",
    className: "bg-muted text-muted-foreground",
    icon: CircleOff,
  },
};

type ConnectorMeta = {
  labelKey: string;
  fallbackLabel: string;
};

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
  douyin: {
    labelKey: "settings.connectors.item.douyin",
    fallbackLabel: "Douyin",
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

const GENERAL_CARD_HEIGHT = "min-h-[240px]";
const CONNECTOR_BRAND_ICONS = {
  youtube: siYoutube,
  bilibili: siBilibili,
  tiktok: siTiktok,
  douyin: siTiktok,
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
  const startConnectorConnect = useStartConnectorConnect();
  const finishConnectorConnect = useFinishConnectorConnect();
  const cancelConnectorConnect = useCancelConnectorConnect();
  const clearConnector = useClearConnector();
  const openConnectorSite = useOpenConnectorSite();

  const [selectedId, setSelectedId] = React.useState<string | null>(null);
  const [query, setQuery] = React.useState("");
  const [loginDialogOpen, setLoginDialogOpen] = React.useState(false);
  const [loginTarget, setLoginTarget] = React.useState<Connector | null>(null);
  const [loginSessionId, setLoginSessionId] = React.useState("");
  const [loginResult, setLoginResult] =
    React.useState<FinishConnectorConnectResult | null>(null);
  const [loginError, setLoginError] = React.useState("");
  const [cookiesDialogOpen, setCookiesDialogOpen] = React.useState(false);
  const loginStartTokenRef = React.useRef(0);
  const loginSession = useConnectorConnectSession(
    { sessionId: loginSessionId },
    loginDialogOpen && loginSessionId.trim().length > 0,
  );

  const items = connectors.data ?? [];
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

  const sortedItems = React.useMemo(
    () =>
      [...filteredItems].sort((left, right) =>
        resolveConnectorLabel(left).localeCompare(resolveConnectorLabel(right)),
      ),
    [filteredItems, resolveConnectorLabel],
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
    STATUS_META[selected?.status ?? "disconnected"] ?? STATUS_META.disconnected;

  const isBusy =
    startConnectorConnect.isPending ||
    finishConnectorConnect.isPending ||
    cancelConnectorConnect.isPending ||
    openConnectorSite.isPending ||
    clearConnector.isPending;
  const isLoginRunning =
    startConnectorConnect.isPending ||
    finishConnectorConnect.isPending ||
    cancelConnectorConnect.isPending;
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
    setLoginSessionId("");
    setLoginResult(null);
    setLoginError("");
  }, []);

  const handleDismissLogin = React.useCallback(async () => {
    loginStartTokenRef.current += 1;
    const sessionId = loginSessionId.trim();
    resetLoginState();
    if (sessionId) {
      await disposeLoginSession(sessionId);
    }
  }, [disposeLoginSession, loginSessionId, resetLoginState]);

  const handleConnect = async (connector: Connector) => {
    const startToken = loginStartTokenRef.current + 1;
    loginStartTokenRef.current = startToken;
    setLoginTarget(connector);
    setLoginDialogOpen(true);
    setLoginSessionId("");
    setLoginResult(null);
    setLoginError("");
    try {
      const result = await startConnectorConnect.mutateAsync({
        id: connector.id,
      });
      if (loginStartTokenRef.current !== startToken) {
        await disposeLoginSession(result.sessionId);
        return;
      }
      setLoginSessionId(result.sessionId);
    } catch (error) {
      if (loginStartTokenRef.current !== startToken) {
        return;
      }
      setLoginError(resolveLoginError(error));
    }
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
      if (!result.saved) {
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
    setLoginResult(toLoginResult(session));
    void disposeLoginSession(sessionId);
    setLoginSessionId("");

    if (session.state === "completed" && session.saved) {
      resetLoginState();
      return;
    }

    if (session.state === "completed") {
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
    loginSessionId,
    resetLoginState,
    t,
    toLoginResult,
  ]);

  const resolveOpenError = (error: unknown) => {
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
  };

  const handleOpenSite = async (connector: Connector) => {
    try {
      await openConnectorSite.mutateAsync({ id: connector.id });
    } catch (error) {
      messageBus.publishToast({
        intent: "danger",
        title: t("settings.connectors.openSite"),
        description: resolveOpenError(error),
      });
    }
  };

  const rowClassName = SETTINGS_ROW_CLASS;
  const loginSessionData = loginSession.data ?? null;
  const loginBrowserStatus = startConnectorConnect.isPending
    ? "opening"
    : loginSessionData?.browserStatus || (loginSessionId ? "open" : "not_open");
  const loginConnectionLabel = loginTarget
    ? resolveConnectorLabel(loginTarget)
    : loginSessionData?.connector
      ? resolveConnectorLabel(loginSessionData.connector)
      : "-";
  const loginStatusRows = [
    {
      label: t("settings.connectors.loginCard.currentConnection"),
      value: loginConnectionLabel,
    },
    {
      label: t("settings.connectors.loginCard.browserStatus"),
      value: t(`settings.connectors.browserStatus.${loginBrowserStatus}`),
    },
    {
      label: t("settings.connectors.loginCard.currentCookiesCount"),
      value: String(
        loginSessionData?.currentCookiesCount ??
          loginResult?.filteredCookiesCount ??
          0,
      ),
    },
  ];
  const selectedLabel = selected ? resolveConnectorLabel(selected) : "";
  const cookiesCount = selected?.cookiesCount ?? selected?.cookies?.length ?? 0;
  const cookiesList = selected?.cookies ?? [];
  const isConnected = (selected?.status ?? "disconnected") === "connected";

  return (
    <div className="flex min-h-0 min-w-0 flex-1 overflow-hidden bg-background">
      <aside className="flex min-h-0 w-[320px] shrink-0 flex-col border-r border-sidebar-border/70 bg-sidebar-background/40">
        <div className="px-4 py-4">
          <div className="app-control-shell-compact h-9 rounded-xl border-border/70 bg-background/92 px-3 shadow-sm">
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
              {sortedItems.map((connector) => {
                const statusMeta =
                  STATUS_META[connector.status ?? "disconnected"] ??
                  STATUS_META.disconnected;
                const isSelected = connector.id === selectedId;
                return (
                  <SidebarMenuItem key={connector.id}>
                    <SidebarMenuButton
                      type="button"
                      isActive={isSelected}
                      className="min-h-11 justify-between rounded-xl border border-transparent hover:bg-accent/68 hover:text-accent-foreground data-[active=true]:bg-accent data-[active=true]:text-accent-foreground data-[active=true]:shadow-sm"
                      onClick={() => setSelectedId(connector.id)}
                    >
                      <div className="flex min-w-0 items-center gap-3">
                        <div
                          className={cn(
                            "flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border shadow-sm transition-colors",
                            isSelected
                              ? "border-[hsl(var(--primary)/0.22)] bg-primary/[0.08] text-primary/85 shadow-[inset_0_1px_0_hsl(var(--background)/0.44)]"
                              : "border-border/70 bg-background/92 text-muted-foreground",
                          )}
                        >
                          <ConnectorBrandIcon
                            connectorType={connector.type}
                            className="h-5 w-5"
                          />
                        </div>
                        <span
                          className={cn(
                            "truncate text-sm font-medium transition-colors",
                            isSelected
                              ? "text-accent-foreground/86"
                              : "text-muted-foreground",
                          )}
                        >
                          {resolveConnectorLabel(connector)}
                        </span>
                      </div>
                      <div className="shrink-0">
                        <span
                          className={cn(
                            "inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium",
                            statusMeta.className,
                          )}
                        >
                          {React.createElement(statusMeta.icon, {
                            className: "h-3.5 w-3.5",
                          })}
                        </span>
                      </div>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                );
              })}
            </SidebarMenu>
          )}
        </div>
      </aside>

      <section className="flex min-h-0 min-w-0 flex-1 flex-col bg-card">
        <div
          className={cn(
            "wails-drag flex shrink-0 items-center justify-between border-b border-border/70 bg-card pl-5",
            isWindows ? "pr-0" : "pr-5",
          )}
        >
          <div className="flex min-h-[56px] min-w-0 flex-1 items-center gap-3 pr-3 text-sm">
            {selected ? (
              <>
                <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-border/70 bg-background/92 text-muted-foreground shadow-sm">
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
                  className={cn(
                    "inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium",
                    status.className,
                  )}
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
                  <span className="inline-flex items-center rounded-full bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground">
                    {cookiesCount}
                  </span>
                  <Button
                    variant="outline"
                    size="compact"
                    onClick={() => setCookiesDialogOpen(true)}
                    disabled={cookiesCount === 0}
                  >
                    <Eye className="h-4 w-4" />
                    {t("settings.connectors.viewCookies")}
                  </Button>
                </div>
              </div>

              <SettingsSeparator />

              <div className={rowClassName}>
                <div className={SETTINGS_ROW_LABEL_CLASS}>
                  {t("settings.connectors.detail.scope")}
                </div>
                <div className="max-w-[60%] text-right text-xs text-muted-foreground">
                  {selected.domains && selected.domains.length > 0
                    ? selected.domains.join(", ")
                    : "-"}
                </div>
              </div>

              <SettingsSeparator />

              <div className={rowClassName}>
                <div className={SETTINGS_ROW_LABEL_CLASS}>
                  {t("settings.connectors.detail.actions")}
                </div>
                <div className="flex flex-wrap items-center justify-end gap-2">
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
                    disabled={isBusy || cookiesCount === 0}
                  >
                    {isOpenRunning ? (
                      <Loader2 className="h-4 w-4 animate-spin" />
                    ) : (
                      <ExternalLink className="h-4 w-4" />
                    )}
                    {t("settings.connectors.openSite")}
                  </Button>
                  <Button
                    variant="outline"
                    size="compact"
                    onClick={() => clearConnector.mutate({ id: selected.id })}
                    disabled={isBusy}
                  >
                    <Trash2 className="h-4 w-4" />
                    {t("settings.connectors.clear")}
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
        <DialogContent className="grid max-h-[min(32rem,calc(100vh-2rem))] w-[min(32rem,calc(100vw-2rem))] max-w-none grid-rows-[auto_minmax(0,1fr)_auto] gap-4 overflow-hidden">
          <DialogHeader className="min-w-0">
            <DialogTitle className="overflow-hidden break-words pr-6 text-left leading-[1.35] [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:2]">
              {t("settings.connectors.loginTitle")}
            </DialogTitle>
          </DialogHeader>
          <div className="min-h-0 overflow-y-auto pr-1">
            <div className="grid gap-2">
            <Card className="border-border/70 bg-muted/20 shadow-none">
              <CardContent className="p-0">
                {loginStatusRows.map((row, index) => (
                  <div
                    key={row.label}
                    className={cn(
                      "flex items-center justify-between gap-4 px-3 py-2.5 text-sm",
                      index > 0 && "border-t border-border/70",
                    )}
                  >
                    <span className="text-muted-foreground">{row.label}</span>
                    <span className="max-w-[55%] overflow-hidden break-words text-right font-medium leading-5 text-foreground [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:2]">
                      {row.value}
                    </span>
                  </div>
                ))}
              </CardContent>
            </Card>
            {loginError ? (
              <div className="overflow-hidden rounded-md border border-destructive/30 bg-destructive/10 p-2 text-xs leading-5 text-destructive [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:4]">
                {loginError}
              </div>
            ) : null}
            </div>
          </div>
          <DialogFooter className="shrink-0">
            <Button
              variant="outline"
              className="h-7"
              onClick={() => void handleDismissLogin()}
              disabled={isLoginRunning}
            >
              {t("common.cancel")}
            </Button>
            <Button
              className="h-7"
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
          <div className="flex min-h-0 flex-col overflow-hidden rounded-md border">
            {cookiesList.length === 0 ? (
              <div className="p-4 text-sm text-muted-foreground">
                {t("settings.connectors.cookiesEmpty")}
              </div>
            ) : (
              <>
                <div className="bg-card">
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
                      <tr className="border-b">
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
                          className="border-b last:border-b-0"
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
              className="h-7"
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
