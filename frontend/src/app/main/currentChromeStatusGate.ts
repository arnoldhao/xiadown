import type { CurrentResourceSniffBrowserStatus } from "@/shared/contracts/library";

export interface CurrentChromeStatusGateSnapshot {
  data?: CurrentResourceSniffBrowserStatus;
  dataUpdatedAt: number;
  entryDataUpdatedAt: number | null;
  isFetching: boolean;
  isError: boolean;
  isRefetchError: boolean;
}

/**
 * Cached readiness is never enough to attach to a user-owned Chrome process.
 * Each visit to the Chrome source step must observe a newer successful probe,
 * and any in-flight or failed refresh closes the gate again.
 */
export function canUseCurrentChrome(
  snapshot: CurrentChromeStatusGateSnapshot,
) {
  return Boolean(
    snapshot.entryDataUpdatedAt !== null &&
      snapshot.data?.ready === true &&
      snapshot.dataUpdatedAt > snapshot.entryDataUpdatedAt &&
      !snapshot.isFetching &&
      !snapshot.isError &&
      !snapshot.isRefetchError,
  );
}
