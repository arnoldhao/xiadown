import type {
  BrowserSourceProfile,
  BrowserSourceSelection,
} from "@/shared/contracts/browserSources";

export type AppSessionImportFlowStep =
  | "browser"
  | "method"
  | "prerequisite"
  | "review"
  | "complete";

export type AppSessionImportOperation = "" | "scan" | "import";

type AppSessionImportOperationLatch = {
  current: AppSessionImportOperation;
};

export function browserProfileCanEnterPrerequisite(
  profile: Pick<BrowserSourceProfile, "available" | "state">,
) {
  if (profile.available) {
    return true;
  }
  switch (profile.state?.trim().toLowerCase()) {
    case "permission_required":
    case "permission_denied":
    case "browser_running":
      return true;
    default:
      return false;
  }
}

export function resolveBrowserProfilePrerequisite(
  profiles: BrowserSourceProfile[],
  profileId: string,
  snapshot: BrowserSourceProfile | null,
) {
  const normalizedProfileId = profileId.trim();
  if (!normalizedProfileId) {
    return {
      profile: null,
      presentInDiscovery: false,
      ready: false,
    };
  }

  const discoveredProfile = profiles.find(
    (profile) => profile.id === normalizedProfileId,
  );
  const fallbackProfile = snapshot?.id === normalizedProfileId ? snapshot : null;

  return {
    profile: discoveredProfile ?? fallbackProfile,
    presentInDiscovery: Boolean(discoveredProfile),
    ready: discoveredProfile?.available === true,
  };
}

export function hasActiveAppSessionImportOperation(
  latch: AppSessionImportOperationLatch,
) {
  return latch.current !== "";
}

export function tryBeginAppSessionImportOperation(
  latch: AppSessionImportOperationLatch,
  operation: Exclude<AppSessionImportOperation, "">,
) {
  if (hasActiveAppSessionImportOperation(latch)) {
    return false;
  }
  latch.current = operation;
  return true;
}

export type AppSessionImportFlowSnapshot = {
  open: boolean;
  step: AppSessionImportFlowStep;
  discoveryEpoch: number;
  operationEpoch: number;
  selectedBrowserId: string;
  selection: BrowserSourceSelection;
  snapshotToken: string;
};

export function isCurrentBrowserDiscovery(
  current: AppSessionImportFlowSnapshot,
  request: {
    epoch: number;
    browserId: string;
    step: "method" | "prerequisite";
  },
) {
  return Boolean(
    current.open &&
      current.step === request.step &&
      current.discoveryEpoch === request.epoch &&
      current.selectedBrowserId === request.browserId,
  );
}

export function isCurrentImportOperation(
  current: AppSessionImportFlowSnapshot,
  request: {
    epoch: number;
    step: "prerequisite" | "review";
    selection: BrowserSourceSelection;
    snapshotToken?: string;
  },
) {
  return Boolean(
    current.open &&
      current.step === request.step &&
      current.operationEpoch === request.epoch &&
      current.selection.mode === request.selection.mode &&
      current.selection.browserId === request.selection.browserId &&
      current.selection.profileId === request.selection.profileId &&
      (request.snapshotToken === undefined ||
        current.snapshotToken === request.snapshotToken),
  );
}
