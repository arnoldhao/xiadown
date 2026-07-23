import {
  Check,
  Copy,
  Loader2,
  Plus,
  RefreshCcw,
  Smartphone,
  Wifi,
} from "lucide-react";
import { QRCodeSVG } from "qrcode.react";
import * as React from "react";
import { Clipboard } from "@wailsio/runtime";

import { getXiaText } from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
import { messageBus } from "@/shared/message";
import {
  useLibraryAccessConfig,
  useLibraryAccessStatus,
  usePairedLibraryDevices,
  useStartLibraryPairing,
  useUpdateLibraryAccessConfig,
} from "@/shared/query/library-access";
import { Button } from "@/shared/ui/button";
import { StatusBadge, type DreamStatusTone } from "@/shared/ui/status-badge";
import {
  Sheet,
  SheetBody,
  SheetCloseButton,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetHeading,
  SheetTitle,
} from "@/shared/ui/sheet";
import {
  LIBRARY_PAIRING_QR_OPTIONS,
  isValidLibraryPairingLink,
  resolveLibraryAccessStatusTone,
  safeLibraryAccessBackendErrorMessage,
  type LibraryAccessStatusTone,
} from "./library-access-ui";

function statusBadgeTone(tone: LibraryAccessStatusTone): DreamStatusTone {
  if (tone === "pending") return "busy";
  if (tone === "neutral") return "muted";
  return tone;
}

export function LibraryPairingSheet(props: {
  language?: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const text = getXiaText(props.language);
  const accessText = text.settings.libraryAccess;
  const configQuery = useLibraryAccessConfig();
  const remoteEnabled = configQuery.data?.remoteEnabled === true;
  const statusQuery = useLibraryAccessStatus(props.open && remoteEnabled);
  const pairedDevices = usePairedLibraryDevices(props.open);
  const updateConfig = useUpdateLibraryAccessConfig();
  const startPairing = useStartLibraryPairing();
  const [copied, setCopied] = React.useState(false);
  const [now, setNow] = React.useState(() => Date.now());

  const tone = configQuery.isLoading || (remoteEnabled && statusQuery.isLoading)
    ? "pending"
    : resolveLibraryAccessStatusTone(statusQuery.data);
  const displayedTone: LibraryAccessStatusTone = remoteEnabled ? tone : "neutral";
  const statusRefreshing = configQuery.isFetching
    || statusQuery.isFetching
    || updateConfig.isPending;
  const pairingReady = remoteEnabled && tone === "success";
  const pairing = startPairing.data;
  const pairingLinkValid = Boolean(
    pairing
      && Date.parse(pairing.expiresAt) > now
      && isValidLibraryPairingLink(pairing.pairingLink, pairing.pairingVersion),
  );
  const errorMessage = safeLibraryAccessBackendErrorMessage(startPairing.error, accessText)
    || safeLibraryAccessBackendErrorMessage(updateConfig.error, accessText)
    || safeLibraryAccessBackendErrorMessage(configQuery.error, accessText)
    || safeLibraryAccessBackendErrorMessage(statusQuery.error, accessText)
    || safeLibraryAccessBackendErrorMessage(pairedDevices.error, accessText)
    || safeLibraryAccessBackendErrorMessage(statusQuery.data?.lan.lastError, accessText)
    || safeLibraryAccessBackendErrorMessage(statusQuery.data?.tailscale.lastError, accessText);
  const activeDevices = React.useMemo(
    () => [...(pairedDevices.data ?? [])]
      .filter((device) => device.status === "active")
      .sort((left, right) => Date.parse(right.lastSeenAt ?? right.updatedAt)
        - Date.parse(left.lastSeenAt ?? left.updatedAt)),
    [pairedDevices.data],
  );

  React.useEffect(() => {
    if (!props.open) {
      setCopied(false);
      startPairing.reset();
    }
  }, [props.open, startPairing.reset]);

  React.useEffect(() => {
    if (!props.open) {
      return;
    }
    setNow(Date.now());
    const timer = window.setInterval(() => setNow(Date.now()), 15_000);
    return () => window.clearInterval(timer);
  }, [props.open]);

  const requestFreshPairing = React.useCallback(() => {
    setCopied(false);
    startPairing.mutate();
  }, [startPairing.mutate]);

  const copyPairingLink = React.useCallback(async () => {
    if (!pairingLinkValid || !pairing) {
      return;
    }
    try {
      try {
        await Clipboard.SetText(pairing.pairingLink);
      } catch {
        if (!navigator.clipboard?.writeText) {
          throw new Error("clipboard unavailable");
        }
        await navigator.clipboard.writeText(pairing.pairingLink);
      }
      setCopied(true);
    } catch {
      setCopied(false);
      messageBus.publishToast({
        intent: "danger",
        title: accessText.copyPairingLink,
        description: text.completed.copyFailed,
      });
    }
  }, [accessText.copyPairingLink, pairing, pairingLinkValid, text.completed.copyFailed]);

  const statusLabel = !remoteEnabled
    ? accessText.localOnly
    : tone === "success"
      ? accessText.remoteReady
      : tone === "pending"
        ? accessText.starting
        : accessText.remoteUnavailable;

  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent centered size="sm">
        <SheetHeader className="items-center">
          <div className="app-library-pairing-header-icon grid h-10 w-10 shrink-0 place-items-center">
            <Smartphone className="h-5 w-5" aria-hidden="true" />
          </div>
          <SheetHeading>
            <SheetTitle>{accessText.mobileTitle}</SheetTitle>
            <SheetDescription className="sr-only">
              {accessText.mobileDescription}
            </SheetDescription>
          </SheetHeading>
          <SheetCloseButton aria-label={text.actions.close} />
        </SheetHeader>

        <SheetBody className="space-y-4">
          <StatusBadge
            tone={statusBadgeTone(displayedTone)}
            icon={statusRefreshing
              ? <Loader2 className="h-4 w-4 app-motion-spin" aria-hidden="true" />
              : undefined}
            marker={!statusRefreshing}
            className="w-full"
          >
            {statusLabel}
          </StatusBadge>

          <section className="space-y-2.5">
            <div className="flex items-center justify-between gap-3">
              <h3 className="app-settings-section-heading">
                {accessText.currentConnections}
              </h3>
              <Button
                type="button"
                variant="ghost"
                size="compactIcon"
                disabled={pairedDevices.isFetching}
                onClick={() => void pairedDevices.refetch()}
                aria-label={accessText.refreshDevices}
                title={accessText.refreshDevices}
              >
                <RefreshCcw className={cn("h-4 w-4", pairedDevices.isFetching && "app-motion-spin")} />
              </Button>
            </div>
            <div className="app-library-pairing-list overflow-hidden">
              {pairedDevices.isLoading ? (
                <div className="app-library-pairing-feedback flex items-center gap-2 px-3 py-3">
                  <Loader2 className="h-4 w-4 app-motion-spin" />
                  {accessText.starting}
                </div>
              ) : activeDevices.length === 0 ? (
                <div className="app-library-pairing-feedback px-3 py-3">
                  {accessText.noPairedDevices}
                </div>
              ) : activeDevices.map((device, index) => (
                <div
                  className="app-library-pairing-device flex items-center gap-3 px-3 py-2.5"
                  data-divider={index > 0 || undefined}
                  key={device.grantId}
                >
                  <Smartphone className="app-settings-muted-icon h-4 w-4 shrink-0" aria-hidden="true" />
                  <div className="min-w-0 flex-1">
                    <div className="app-library-pairing-device-name truncate">
                      {device.deviceName || device.deviceId}
                    </div>
                    <div className="app-library-pairing-device-meta truncate">
                      {accessText.lastSeen}: {device.lastSeenAt
                        ? new Date(device.lastSeenAt).toLocaleString(text.locale, {
                            dateStyle: "medium",
                            timeStyle: "short",
                          })
                        : accessText.never}
                    </div>
                  </div>
                  <StatusBadge tone="success" iconOnly marker aria-label={accessText.available} />
                </div>
              ))}
            </div>
          </section>

          <section className="app-library-pairing-panel space-y-3 p-3">
            <h3 className="app-settings-section-heading">{accessText.pairDevice}</h3>

            {!remoteEnabled ? (
              <Button
                type="button"
                variant="outline"
                size="compact"
                disabled={updateConfig.isPending}
                onClick={() => updateConfig.mutate({ remoteEnabled: true })}
              >
                {updateConfig.isPending ? <Loader2 className="h-4 w-4 app-motion-spin" /> : <Wifi className="h-4 w-4" />}
                {accessText.enableRemoteAccess}
              </Button>
            ) : tone === "pending" || startPairing.isPending ? (
              <div className="app-library-pairing-feedback flex min-h-32 items-center justify-center gap-2">
                <Loader2 className="h-4 w-4 app-motion-spin" />
                {accessText.starting}
              </div>
            ) : !pairingReady ? (
              <div className="app-library-pairing-unavailable flex items-center justify-between gap-3 px-3 py-2.5">
                <span>{accessText.remoteUnavailable}</span>
                <Button
                  type="button"
                  variant="ghost"
                  size="compactIcon"
                  disabled={statusQuery.isFetching}
                  onClick={() => void statusQuery.refetch()}
                  aria-label={accessText.refreshDevices}
                  title={accessText.refreshDevices}
                >
                  <RefreshCcw className={cn("h-4 w-4", statusQuery.isFetching && "app-motion-spin")} />
                </Button>
              </div>
            ) : pairingLinkValid && pairing ? (
              <div className="grid justify-items-center gap-3">
                <div className="app-library-pairing-qr p-2" data-library-pairing-qr>
                  <QRCodeSVG
                    value={pairing.pairingLink}
                    {...LIBRARY_PAIRING_QR_OPTIONS}
                    className="app-library-pairing-qr-image h-44 w-44"
                    title={accessText.pairingQRCodeAlt}
                  />
                </div>
                <div className="app-library-pairing-expiry">
                  {accessText.expiresAt}: {new Date(pairing.expiresAt).toLocaleTimeString(text.locale, {
                    hour: "2-digit",
                    minute: "2-digit",
                  })}
                </div>
                <div className="grid w-full max-w-xs grid-cols-2 gap-2">
                  <Button
                    type="button"
                    variant="outline"
                    tone={copied ? "success" : "neutral"}
                    size="compact"
                    className="min-w-0"
                    onClick={() => void copyPairingLink()}
                  >
                    {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                    <span className="truncate">{accessText.copyPairingLink}</span>
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    size="compact"
                    className="min-w-0"
                    disabled={startPairing.isPending}
                    onClick={requestFreshPairing}
                  >
                    <RefreshCcw className="h-4 w-4" />
                    <span className="truncate">{accessText.refreshPairingCode}</span>
                  </Button>
                </div>
              </div>
            ) : (
              <Button
                type="button"
                variant="outline"
                size="compact"
                className="w-full"
                disabled={startPairing.isPending}
                onClick={requestFreshPairing}
              >
                {startPairing.isPending ? <Loader2 className="h-4 w-4 app-motion-spin" /> : <Plus className="h-4 w-4" />}
                {accessText.createPairingCode}
              </Button>
            )}

            {errorMessage ? (
              <div className="app-dream-status-message overflow-hidden break-words px-3 py-2" data-intent="danger">
                {errorMessage}
              </div>
            ) : null}
          </section>

          <section className="app-library-pairing-download p-3">
            <div className="flex items-start gap-3">
              <div className="app-library-pairing-download-icon grid h-9 w-9 shrink-0 place-items-center">
                <Smartphone className="h-4 w-4" aria-hidden="true" />
              </div>
              <div className="min-w-0 flex-1">
                <h3 className="app-settings-section-heading">{accessText.downloadMobile}</h3>
                <StatusBadge tone="muted" className="mt-2">
                  {accessText.comingSoon}
                </StatusBadge>
              </div>
            </div>
          </section>
        </SheetBody>
      </SheetContent>
    </Sheet>
  );
}
