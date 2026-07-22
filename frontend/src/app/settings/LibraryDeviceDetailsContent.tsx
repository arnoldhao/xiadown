import { Laptop, Loader2, Settings2, Trash2 } from "lucide-react";

import { getXiaText } from "@/features/xiadown/shared";
import type {
  LibraryDeviceScope,
  PairedLibraryDevice,
} from "@/shared/contracts/library-access";
import { Button } from "@/shared/ui/button";
import {
  DialogListCard,
  DialogListCardContent,
  DialogRow,
  DialogScrollArea,
} from "@/shared/ui/dialog";
import {
  SettingsCompactRow,
} from "@/shared/ui/settings-layout";
import { StatusBadge } from "@/shared/ui/status-badge";

import { InlineSwitch } from "./settings-helpers";

export function LibraryPairedDeviceRow(props: {
  device: PairedLibraryDevice;
  language?: string;
  onView: () => void;
}) {
  const text = getXiaText(props.language);
  const accessText = text.settings.libraryAccess;
  const active = props.device.status === "active";
  const name = props.device.deviceName || props.device.deviceId;

  return (
    <SettingsCompactRow
      data-library-device={props.device.grantId}
      label={(
        <span
          className="inline-flex min-w-0 items-center gap-2"
          title={props.device.deviceId}
        >
          <Laptop className="h-4 w-4 shrink-0" />
          <span className="truncate">{name}</span>
        </span>
      )}
    >
      <div className="app-library-device-list-actions">
        <StatusBadge tone={active ? "success" : "muted"} marker>
          {active ? accessText.active : accessText.revoked}
        </StatusBadge>
        <Button
          type="button"
          variant="ghost"
          size="compact"
          onClick={props.onView}
          aria-label={`${text.actions.view}: ${name}`}
        >
          <Settings2 className="h-4 w-4" />
          {text.actions.view}
        </Button>
      </div>
    </SettingsCompactRow>
  );
}

export function LibraryDeviceDetailsContent(props: {
  device: PairedLibraryDevice;
  language?: string;
  mutating?: boolean;
  revokeConfirming?: boolean;
  revokePending?: boolean;
  errorLabel?: string;
  onToggleScope: (scope: LibraryDeviceScope) => void;
  onCancelRevoke: () => void;
  onRevoke: () => void;
}) {
  const text = getXiaText(props.language);
  const accessText = text.settings.libraryAccess;
  const deviceScopes: Array<{ scope: LibraryDeviceScope; label: string }> = [
    { scope: "library.read", label: accessText.scopeLibraryRead },
    { scope: "music.read", label: accessText.scopeMusicRead },
    { scope: "music.state", label: accessText.scopeMusicState },
    { scope: "music.manage", label: accessText.scopeMusicManage },
    { scope: "rss.read", label: accessText.scopeRSSRead },
    { scope: "rss.state", label: accessText.scopeRSSState },
    { scope: "rss.manage", label: accessText.scopeRSSManage },
    { scope: "rss.fetch", label: accessText.scopeRSSFetch },
    { scope: "tasks.read", label: accessText.scopeTasksRead },
    { scope: "tasks.create", label: accessText.scopeTasksCreate },
    { scope: "tasks.control", label: accessText.scopeTasksControl },
  ];

  return (
    <div data-library-device-details-presentation="true">
      <DialogScrollArea className="app-library-device-dialog-scroll">
        <DialogListCard data-library-device-details={props.device.grantId}>
          <DialogListCardContent>
            {deviceScopes.map((permission) => (
              <DialogRow
                key={permission.scope}
                className="app-library-device-dialog-row"
              >
                <span>{permission.label}</span>
                <InlineSwitch
                  checked={props.device.scopes.includes(permission.scope)}
                  disabled={props.device.status !== "active" || props.mutating}
                  onChange={() => props.onToggleScope(permission.scope)}
                  ariaLabel={permission.label}
                />
              </DialogRow>
            ))}

            <DialogRow className="app-library-device-dialog-row">
              <span>{accessText.lastSeen}</span>
              <div className="app-library-device-last-seen">
                <span className="app-settings-secondary-label min-w-0 truncate">
                  {props.device.lastSeenAt
                    ? new Date(props.device.lastSeenAt).toLocaleString(
                        text.locale,
                        { dateStyle: "medium", timeStyle: "short" },
                      )
                    : accessText.never}
                </span>
                {props.device.status === "active" ? (
                  <div className="app-library-device-revoke-actions">
                    {props.revokeConfirming ? (
                      <Button
                        type="button"
                        variant="ghost"
                        size="compact"
                        disabled={props.mutating}
                        onClick={props.onCancelRevoke}
                      >
                        {text.actions.cancelDialog}
                      </Button>
                    ) : null}
                    <Button
                      type="button"
                      variant={props.revokeConfirming ? "destructive" : "ghost"}
                      size="compact"
                      disabled={props.mutating}
                      onClick={props.onRevoke}
                    >
                      {props.revokePending ? (
                        <Loader2 className="h-3.5 w-3.5 app-motion-spin" />
                      ) : (
                        <Trash2 className="h-3.5 w-3.5" />
                      )}
                      {props.revokeConfirming
                        ? accessText.confirmRevoke
                        : accessText.revoke}
                    </Button>
                  </div>
                ) : null}
              </div>
            </DialogRow>
          </DialogListCardContent>
        </DialogListCard>
      </DialogScrollArea>

      {props.errorLabel ? (
        <div
          className="app-dream-status-message overflow-hidden break-words px-3 py-2"
          data-intent="danger"
        >
          {props.errorLabel}
        </div>
      ) : null}
    </div>
  );
}
