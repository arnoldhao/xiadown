import { Call } from "@wailsio/runtime";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import type {
  LibraryAccessConfig,
  LibraryAccessStatus,
  LibraryAccessUpdate,
  LibraryPairingSession,
  PairedLibraryDevice,
  RevokePairedLibraryDevice,
  UpdateLibraryAccessConfig,
  UpdatePairedLibraryDeviceScopes,
} from "@/shared/contracts/library-access";

const ACCESS_HANDLER = "xiadown/internal/presentation/wails.LibraryAccessHandler";
const PAIRING_HANDLER = "xiadown/internal/presentation/wails.LibraryPairingHandler";
const accessKeys = {
  config: ["library-access", "config"] as const,
  status: ["library-access", "status"] as const,
  devices: ["library-access", "devices"] as const,
};

export function useLibraryAccessConfig() {
  return useQuery({
    queryKey: accessKeys.config,
    queryFn: () => Call.ByName(`${ACCESS_HANDLER}.GetLibraryAccessConfig`) as Promise<LibraryAccessConfig>,
  });
}

export function useLibraryAccessStatus(enabled = true) {
  return useQuery({
    queryKey: accessKeys.status,
    enabled,
    queryFn: () => Call.ByName(`${ACCESS_HANDLER}.GetLibraryAccessStatus`) as Promise<LibraryAccessStatus>,
    refetchInterval: enabled ? 5_000 : false,
  });
}

export function useUpdateLibraryAccessConfig() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (request: UpdateLibraryAccessConfig) =>
      Call.ByName(`${ACCESS_HANDLER}.UpdateLibraryAccessConfig`, request) as Promise<LibraryAccessUpdate>,
    onSuccess: (result) => {
      client.setQueryData(accessKeys.config, result.config);
      client.setQueryData(accessKeys.status, result.status);
    },
  });
}

export function useStartLibraryPairing() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: () => Call.ByName(`${PAIRING_HANDLER}.StartLibraryPairing`) as Promise<LibraryPairingSession>,
    onSuccess: () => client.invalidateQueries({ queryKey: accessKeys.devices }),
  });
}

export function usePairedLibraryDevices(enabled = true) {
  return useQuery({
    queryKey: accessKeys.devices,
    enabled,
    queryFn: () => Call.ByName(`${PAIRING_HANDLER}.ListPairedLibraryDevices`) as Promise<PairedLibraryDevice[]>,
    refetchInterval: enabled ? 5_000 : false,
  });
}

export function useUpdatePairedLibraryDeviceScopes() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (request: UpdatePairedLibraryDeviceScopes) =>
      Call.ByName(`${PAIRING_HANDLER}.UpdatePairedLibraryDeviceScopes`, request) as Promise<PairedLibraryDevice>,
    onSuccess: (updated) => {
      client.setQueryData<PairedLibraryDevice[]>(accessKeys.devices, (current) =>
        current?.map((device) => device.grantId === updated.grantId ? updated : device) ?? [updated],
      );
    },
    onError: () => client.invalidateQueries({ queryKey: accessKeys.devices }),
  });
}

export function useRevokePairedLibraryDevice() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (request: RevokePairedLibraryDevice) =>
      Call.ByName(`${PAIRING_HANDLER}.RevokePairedLibraryDevice`, request) as Promise<PairedLibraryDevice>,
    onSuccess: (updated) => {
      client.setQueryData<PairedLibraryDevice[]>(accessKeys.devices, (current) =>
        current?.map((device) => device.grantId === updated.grantId ? updated : device) ?? [updated],
      );
    },
    onError: () => client.invalidateQueries({ queryKey: accessKeys.devices }),
  });
}
