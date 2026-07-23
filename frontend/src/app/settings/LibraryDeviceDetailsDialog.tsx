import * as React from "react";

import { getXiaText } from "@/features/xiadown/shared";
import type {
  LibraryDeviceScope,
  PairedLibraryDevice,
} from "@/shared/contracts/library-access";
import {
  useRevokePairedLibraryDevice,
  useUpdatePairedLibraryDeviceScopes,
} from "@/shared/query/library-access";
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
import { LibraryDeviceDetailsContent } from "./LibraryDeviceDetailsContent";
import {
  isLibraryAccessRevisionConflict,
  safeLibraryAccessBackendErrorMessage,
  toggleLibraryDeviceScope,
} from "./library-access-ui";

export function LibraryDeviceDetailsDialog(props: {
  device: PairedLibraryDevice | null;
  language?: string;
  onOpenChange: (open: boolean) => void;
  onRefresh: () => void | Promise<unknown>;
}) {
  const text = getXiaText(props.language);
  const accessText = text.settings.libraryAccess;
  const updateDeviceScopes = useUpdatePairedLibraryDeviceScopes();
  const revokeDevice = useRevokePairedLibraryDevice();
  const [revokeConfirmGrantId, setRevokeConfirmGrantId] = React.useState("");
  const device = props.device;
  const mutating = updateDeviceScopes.isPending || revokeDevice.isPending;
  const mutationError = updateDeviceScopes.error || revokeDevice.error;
  const errorLabel = isLibraryAccessRevisionConflict(mutationError)
    ? accessText.revisionConflict
    : safeLibraryAccessBackendErrorMessage(mutationError, accessText);

  React.useEffect(() => {
    setRevokeConfirmGrantId("");
  }, [device?.grantId]);

  const refreshDevices = () => {
    void props.onRefresh();
  };

  return (
    <Dialog
      open={device !== null}
      onOpenChange={(open) => {
        if (!open) {
          setRevokeConfirmGrantId("");
        }
        props.onOpenChange(open);
      }}
    >
      <DialogContent className="app-library-device-dialog">
        {device ? (
          <>
            <DialogHeader>
              <DialogTitle>{device.deviceName || device.deviceId}</DialogTitle>
              <DialogDescription>{accessText.pairedDevicesDescription}</DialogDescription>
            </DialogHeader>

            <LibraryDeviceDetailsContent
              device={device}
              language={props.language}
              mutating={mutating}
              revokeConfirming={revokeConfirmGrantId === device.grantId}
              revokePending={
                revokeDevice.isPending && revokeConfirmGrantId === device.grantId
              }
              errorLabel={errorLabel}
              onToggleScope={(scope: LibraryDeviceScope) => {
                const scopes = toggleLibraryDeviceScope(device.scopes, scope);
                if (
                  scopes.length === device.scopes.length
                  && scopes.every((candidate) => device.scopes.includes(candidate))
                ) {
                  return;
                }
                void updateDeviceScopes.mutateAsync({
                  grantId: device.grantId,
                  expectedRevision: device.revision,
                  scopes,
                }).catch(refreshDevices);
              }}
              onCancelRevoke={() => setRevokeConfirmGrantId("")}
              onRevoke={() => {
                if (revokeConfirmGrantId !== device.grantId) {
                  setRevokeConfirmGrantId(device.grantId);
                  return;
                }
                void revokeDevice.mutateAsync({
                  grantId: device.grantId,
                  expectedRevision: device.revision,
                }).then(() => setRevokeConfirmGrantId(""))
                  .catch(refreshDevices);
              }}
            />

            <DialogFooter>
              <DialogClose asChild>
                <Button type="button" variant="outline">
                  {text.actions.close}
                </Button>
              </DialogClose>
            </DialogFooter>
          </>
        ) : null}
      </DialogContent>
    </Dialog>
  );
}
