import { Loader2, RefreshCcw, ShieldCheck } from "lucide-react";
import * as React from "react";

import { getXiaText } from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
import type {
  LibraryAccessConfig,
  UpdateLibraryAccessConfig,
} from "@/shared/contracts/library-access";
import {
  useLibraryAccessConfig,
  useLibraryAccessStatus,
  usePairedLibraryDevices,
  useUpdateLibraryAccessConfig,
} from "@/shared/query/library-access";
import { Button } from "@/shared/ui/button";
import { Input } from "@/shared/ui/input";
import { StatusBadge, type DreamStatusTone } from "@/shared/ui/status-badge";
import {
  SettingsCompactListCard,
  SettingsCompactRow,
  SettingsCompactSeparator,
} from "@/shared/ui/settings-layout";
import { InlineSwitch } from "./settings-helpers";
import { LibraryPairedDeviceRow } from "./LibraryDeviceDetailsContent";
import { LibraryDeviceDetailsDialog } from "./LibraryDeviceDetailsDialog";
import { LibraryPairingSheet } from "./LibraryPairingSheet";
import {
  normalizeLibraryAccessPath,
  normalizeLibraryAccessPort,
  resolveLibraryAccessStatusTone,
  resolveLibraryAccessTransportTone,
  safeLibraryAccessBackendErrorMessage,
  type LibraryAccessStatusTone,
} from "./library-access-ui";

function statusBadgeTone(tone: LibraryAccessStatusTone): DreamStatusTone {
  if (tone === "pending") return "busy";
  if (tone === "neutral") return "muted";
  return tone;
}

function LibraryAccessStatusValue(props: {
  busy?: boolean;
  label: string;
  tone: LibraryAccessStatusTone;
}) {
  return (
    <StatusBadge
      className="min-w-0"
      tone={statusBadgeTone(props.tone)}
      marker={!props.busy}
      icon={props.busy ? <RefreshCcw className="app-motion-spin" /> : undefined}
    >
      {props.label}
    </StatusBadge>
  );
}

export function LibraryAccessSettingsCard(props: { language?: string }) {
  const text = getXiaText(props.language);
  const accessText = text.settings.libraryAccess;
  const configQuery = useLibraryAccessConfig();
  const statusQuery = useLibraryAccessStatus(configQuery.data?.remoteEnabled === true);
  const updateConfig = useUpdateLibraryAccessConfig();
  const pairedDevices = usePairedLibraryDevices(configQuery.data?.remoteEnabled === true);
  const [draft, setDraft] = React.useState<LibraryAccessConfig | null>(null);
  const [pairingSheetOpen, setPairingSheetOpen] = React.useState(false);
  const [deviceDetailsGrantId, setDeviceDetailsGrantId] = React.useState("");

  React.useEffect(() => {
    if (configQuery.data) {
      setDraft(configQuery.data);
    }
  }, [configQuery.data]);

  const persist = React.useCallback(
    async (patch: UpdateLibraryAccessConfig) => {
      const result = await updateConfig.mutateAsync(patch);
      setDraft(result.config);
      return result;
    },
    [updateConfig],
  );

  const updateDraft = <K extends keyof LibraryAccessConfig>(key: K, value: LibraryAccessConfig[K]) => {
    setDraft((current) => current ? { ...current, [key]: value } : current);
  };

  const saveFields = async () => {
    if (!draft || !configQuery.data) {
      return;
    }
    await persist({
      deviceName: draft.deviceName.trim(),
      lanPort: normalizeLibraryAccessPort(String(draft.lanPort), configQuery.data.lanPort),
      tailscaleHTTPSPort: normalizeLibraryAccessPort(String(draft.tailscaleHTTPSPort), configQuery.data.tailscaleHTTPSPort),
      tailscalePath: normalizeLibraryAccessPath(draft.tailscalePath),
    }).catch(() => undefined);
  };

  const hasFieldChanges = Boolean(draft && configQuery.data && (
    draft.deviceName.trim() !== configQuery.data.deviceName ||
    draft.lanPort !== configQuery.data.lanPort ||
    draft.tailscaleHTTPSPort !== configQuery.data.tailscaleHTTPSPort ||
    normalizeLibraryAccessPath(draft.tailscalePath) !== normalizeLibraryAccessPath(configQuery.data.tailscalePath)
  ));
  const status = statusQuery.data;
  const aggregateTone = resolveLibraryAccessStatusTone(status);
  const statusLabel = aggregateTone === "success"
    ? accessText.available
    : aggregateTone === "danger"
      ? accessText.error
      : aggregateTone === "pending"
        ? accessText.starting
        : accessText.disabled;
  const configurationError = safeLibraryAccessBackendErrorMessage(updateConfig.error, accessText)
    || safeLibraryAccessBackendErrorMessage(configQuery.error, accessText)
    || safeLibraryAccessBackendErrorMessage(statusQuery.error, accessText)
    || safeLibraryAccessBackendErrorMessage(status?.lan.lastError, accessText)
    || safeLibraryAccessBackendErrorMessage(status?.tailscale.lastError, accessText);
  const deviceErrorLabel = safeLibraryAccessBackendErrorMessage(pairedDevices.error, accessText);
  const sortedDevices = [...(pairedDevices.data ?? [])].sort((left, right) => {
    if (left.status === "active" && right.status !== "active") return -1;
    if (left.status !== "active" && right.status === "active") return 1;
    return Date.parse(right.updatedAt) - Date.parse(left.updatedAt);
  });
  const selectedDevice = sortedDevices.find(
    (device) => device.grantId === deviceDetailsGrantId,
  ) ?? null;
  const transportLabel = (tone: LibraryAccessStatusTone) => tone === "success"
    ? accessText.available
    : tone === "danger"
      ? accessText.error
      : tone === "pending"
        ? accessText.starting
        : accessText.disabled;

  const remoteEnabled = draft?.remoteEnabled === true;
  const lanEnabled = remoteEnabled && draft?.lanEnabled === true;
  const tailscaleEnabled = remoteEnabled && draft?.tailscaleEnabled === true;
  const statusBusy = updateConfig.isPending || (remoteEnabled && statusQuery.isLoading);
  const displayedAggregateTone: LibraryAccessStatusTone = configurationError
    ? "danger"
    : statusBusy
      ? "pending"
      : aggregateTone;
  const displayedStatusLabel = configurationError
    ? accessText.error
    : statusBusy
      ? accessText.starting
      : statusLabel;
  const lanTone = resolveLibraryAccessTransportTone(status?.lan);
  const lanBusy = lanEnabled && statusQuery.isLoading;
  const displayedLanTone: LibraryAccessStatusTone = lanBusy ? "pending" : lanTone;
  const displayedLanLabel = !lanEnabled
    ? accessText.disabled
    : lanBusy
      ? accessText.starting
      : transportLabel(lanTone);
  const tailscaleTone = resolveLibraryAccessTransportTone(status?.tailscale);
  const tailscaleBusy = tailscaleEnabled && statusQuery.isLoading;
  const displayedTailscaleTone: LibraryAccessStatusTone = tailscaleBusy ? "pending" : tailscaleTone;
  const displayedTailscaleLabel = !tailscaleEnabled
    ? accessText.disabled
    : tailscaleBusy
      ? accessText.starting
      : status && !status.tailscale.installed
        ? accessText.notInstalled
        : transportLabel(tailscaleTone);

  if (!draft) {
    return (
      <SettingsCompactListCard>
        <SettingsCompactRow label={accessText.title}>
          {configQuery.isLoading ? <Loader2 className="app-settings-muted-icon h-4 w-4 app-motion-spin" /> : <span className="app-settings-error-label">{accessText.unavailable}</span>}
        </SettingsCompactRow>
      </SettingsCompactListCard>
    );
  }

  return (
    <>
      <SettingsCompactListCard data-library-access-settings>
        <SettingsCompactRow
          label={<span title={accessText.description}>{accessText.title}</span>}
          contentClassName="min-w-0"
        >
          <div className="flex min-w-0 items-center gap-3">
            <LibraryAccessStatusValue
              busy={statusBusy || statusQuery.isFetching}
              label={displayedStatusLabel}
              tone={displayedAggregateTone}
            />
            <InlineSwitch
              checked={draft.remoteEnabled}
              disabled={updateConfig.isPending}
              onChange={(remoteEnabled) => void persist({ remoteEnabled }).catch(() => undefined)}
              ariaLabel={accessText.remoteAccess}
            />
          </div>
        </SettingsCompactRow>

        <SettingsCompactSeparator />

        <SettingsCompactRow
          label={<span title={accessText.localNetworkDescription}>{accessText.localNetwork}</span>}
          contentClassName="min-w-0"
        >
          <div className="flex min-w-0 items-center gap-3">
            <LibraryAccessStatusValue
              busy={lanBusy}
              label={displayedLanLabel}
              tone={displayedLanTone}
            />
            <InlineSwitch
              checked={draft.lanEnabled}
              disabled={!draft.remoteEnabled || updateConfig.isPending}
              onChange={(lanEnabled) => void persist({ lanEnabled }).catch(() => undefined)}
              ariaLabel={accessText.localNetwork}
            />
          </div>
        </SettingsCompactRow>

        {draft.lanEnabled ? (
          <>
            <SettingsCompactSeparator />
            <SettingsCompactRow label={accessText.lanPort}>
              <Input
                aria-label={accessText.lanPort}
                type="number"
                min={1}
                max={65535}
                value={draft.lanPort}
                onChange={(event) => updateDraft("lanPort", Number(event.target.value))}
                className="app-settings-monospace-input w-28"
              />
            </SettingsCompactRow>
          </>
        ) : null}

        <SettingsCompactSeparator />

        <SettingsCompactRow
          label={<span title={accessText.tailscaleDescription}>{accessText.tailscale}</span>}
          contentClassName="min-w-0"
        >
          <div className="flex min-w-0 items-center gap-3">
            <LibraryAccessStatusValue
              busy={tailscaleBusy}
              label={displayedTailscaleLabel}
              tone={displayedTailscaleTone}
            />
            <InlineSwitch
              checked={draft.tailscaleEnabled}
              disabled={!draft.remoteEnabled || updateConfig.isPending}
              onChange={(tailscaleEnabled) => void persist({ tailscaleEnabled }).catch(() => undefined)}
              ariaLabel={accessText.tailscale}
            />
          </div>
        </SettingsCompactRow>

        {draft.tailscaleEnabled ? (
          <>
            <SettingsCompactSeparator />
            <SettingsCompactRow label={accessText.tailscalePort}>
              <Input
                aria-label={accessText.tailscalePort}
                type="number"
                min={1}
                max={65535}
                value={draft.tailscaleHTTPSPort}
                onChange={(event) => updateDraft("tailscaleHTTPSPort", Number(event.target.value))}
                className="app-settings-monospace-input w-28"
              />
            </SettingsCompactRow>
            <SettingsCompactSeparator />
            <SettingsCompactRow label={accessText.tailscalePath}>
              <Input
                aria-label={accessText.tailscalePath}
                value={draft.tailscalePath}
                onChange={(event) => updateDraft("tailscalePath", event.target.value)}
                placeholder="/xiadown"
                className="app-settings-monospace-input w-48"
              />
            </SettingsCompactRow>
          </>
        ) : null}

        <SettingsCompactSeparator />

        <SettingsCompactRow label={accessText.deviceName}>
          <div className="flex items-center gap-2">
            <Input
              aria-label={accessText.deviceName}
              value={draft.deviceName}
              onChange={(event) => updateDraft("deviceName", event.target.value)}
              className="app-settings-field-input w-48"
            />
            <Button
              type="button"
              variant="outline"
              size="compact"
              disabled={!hasFieldChanges || updateConfig.isPending}
              onClick={() => void saveFields()}
            >
              {updateConfig.isPending ? <Loader2 className="h-4 w-4 app-motion-spin" /> : null}
              {accessText.saveConfiguration}
            </Button>
          </div>
        </SettingsCompactRow>

        {configurationError ? (
          <div className="app-dream-status-message mx-3 mb-3 overflow-hidden break-words px-3 py-2" data-intent="danger">
            {configurationError}
          </div>
        ) : null}
      </SettingsCompactListCard>

      {draft.remoteEnabled ? (
        <SettingsCompactListCard data-library-pairing-settings>
          <SettingsCompactRow
            label={<span title={accessText.pairDescription}>{accessText.pairDevice}</span>}
          >
            <Button
              type="button"
              variant="outline"
              size="compact"
              disabled={aggregateTone !== "success"}
              onClick={() => setPairingSheetOpen(true)}
            >
              <ShieldCheck className="h-4 w-4" />
              {accessText.pairDevice}
            </Button>
          </SettingsCompactRow>
        </SettingsCompactListCard>
      ) : null}

      {draft.remoteEnabled ? (
        <SettingsCompactListCard data-library-paired-devices>
          <SettingsCompactRow
            label={<span title={accessText.pairedDevicesDescription}>{accessText.pairedDevices}</span>}
          >
            <Button
              type="button"
              variant="outline"
              size="compactIcon"
              disabled={pairedDevices.isFetching}
              onClick={() => void pairedDevices.refetch()}
              aria-label={accessText.refreshDevices}
              title={accessText.refreshDevices}
            >
              <RefreshCcw className={cn("h-4 w-4", pairedDevices.isFetching && "app-motion-spin")} />
            </Button>
          </SettingsCompactRow>

          {pairedDevices.isLoading ? (
            <>
              <SettingsCompactSeparator />
              <SettingsCompactRow label={accessText.starting}>
                <Loader2 className="app-settings-muted-icon h-4 w-4 app-motion-spin" />
              </SettingsCompactRow>
            </>
          ) : sortedDevices.length === 0 ? (
            <>
              <SettingsCompactSeparator />
              <SettingsCompactRow label={accessText.noPairedDevices}>
                <span className="app-settings-muted-text">—</span>
              </SettingsCompactRow>
            </>
          ) : sortedDevices.map((device) => {
            return (
              <React.Fragment key={device.grantId}>
                <SettingsCompactSeparator />
                <LibraryPairedDeviceRow
                  device={device}
                  language={props.language}
                  onView={() => setDeviceDetailsGrantId(device.grantId)}
                />
              </React.Fragment>
            );
          })}

          {deviceErrorLabel ? (
            <div className="app-dream-status-message mx-3 mb-3 overflow-hidden break-words px-3 py-2" data-intent="danger">
              {deviceErrorLabel}
            </div>
          ) : null}
        </SettingsCompactListCard>
      ) : null}

      <LibraryDeviceDetailsDialog
        device={selectedDevice}
        language={props.language}
        onOpenChange={(open) => {
          if (!open) {
            setDeviceDetailsGrantId("");
          }
        }}
        onRefresh={() => pairedDevices.refetch()}
      />

      <LibraryPairingSheet
        language={props.language}
        open={pairingSheetOpen}
        onOpenChange={setPairingSheetOpen}
      />
    </>
  );
}
