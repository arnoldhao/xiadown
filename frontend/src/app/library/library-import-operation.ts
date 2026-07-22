import type {
  LibraryImportBatch,
  LibraryImportBatchStatus,
} from "@/shared/contracts/library-management";

export type LibraryImportOperationKind =
  | "scan"
  | "select"
  | "commit"
  | "resume"
  | "cancel";

export interface LibraryImportOperationToken {
  id: number;
  kind: LibraryImportOperationKind;
  batchId?: string;
  lane: "primary" | "cancellation";
}

/**
 * Serializes import mutations while still allowing a cancellation request to
 * accompany the currently executing commit/resume for the same batch.
 */
export class LibraryImportOperationController {
  private sequence = 0;
  private primary: LibraryImportOperationToken | null = null;
  private cancellation: LibraryImportOperationToken | null = null;

  begin(
    kind: Exclude<LibraryImportOperationKind, "cancel">,
    batchId?: string,
  ): LibraryImportOperationToken | null {
    if (this.primary || this.cancellation) return null;
    const token = this.createToken(kind, "primary", batchId);
    this.primary = token;
    return token;
  }

  beginCancellation(batchId: string): LibraryImportOperationToken | null {
    if (this.cancellation) return null;
    if (this.primary) {
      const canAccompanyExecution =
        (this.primary.kind === "commit" || this.primary.kind === "resume")
        && this.primary.batchId === batchId;
      if (!canAccompanyExecution) return null;

      const token = this.createToken("cancel", "cancellation", batchId);
      this.cancellation = token;
      return token;
    }

    const token = this.createToken("cancel", "primary", batchId);
    this.primary = token;
    return token;
  }

  isCurrent(token: LibraryImportOperationToken): boolean {
    const active = token.lane === "primary" ? this.primary : this.cancellation;
    return active?.id === token.id;
  }

  canAnnounce(token: LibraryImportOperationToken): boolean {
    return this.isCurrent(token)
      && !(token.lane === "primary" && this.cancellation !== null);
  }

  settle(token: LibraryImportOperationToken): boolean {
    const active = token.lane === "primary" ? this.primary : this.cancellation;
    if (active?.id !== token.id) return false;
    if (token.lane === "primary") {
      this.primary = null;
    } else {
      this.cancellation = null;
    }
    return true;
  }

  isBusy(): boolean {
    return this.primary !== null || this.cancellation !== null;
  }

  invalidate(): void {
    this.sequence += 1;
    this.primary = null;
    this.cancellation = null;
  }

  private createToken(
    kind: LibraryImportOperationKind,
    lane: LibraryImportOperationToken["lane"],
    batchId?: string,
  ): LibraryImportOperationToken {
    this.sequence += 1;
    return {
      id: this.sequence,
      kind,
      lane,
      ...(batchId ? { batchId } : {}),
    };
  }
}

export function reconcileLibraryImportBatch(
  current: LibraryImportBatch | null | undefined,
  incoming: LibraryImportBatch,
): LibraryImportBatch {
  if (!current || current.id !== incoming.id) return incoming;
  const currentUpdatedAt = Date.parse(current.updatedAt);
  const incomingUpdatedAt = Date.parse(incoming.updatedAt);
  if (
    Number.isFinite(currentUpdatedAt)
    && Number.isFinite(incomingUpdatedAt)
    && incomingUpdatedAt < currentUpdatedAt
  ) {
    return current;
  }
  return incoming;
}

export function resolveLibraryImportResultNotice(
  operation: "scan" | "commit" | "resume" | "cancel",
  batch: Pick<LibraryImportBatch, "status" | "cancelRequested">,
  labels: {
    scanReadyNotice: string;
    commitCompletedNotice: string;
    resumeCompletedNotice: string;
    cancelRequestedNotice: string;
  },
): string | null {
  const status: LibraryImportBatchStatus = batch.status;
  if (status === "cancelled" || status === "cancelling") {
    return labels.cancelRequestedNotice;
  }
  if (batch.cancelRequested) return labels.cancelRequestedNotice;
  // A cancellation can race with normal completion. If the backend returns a
  // completed batch without cancelRequested, cancellation was too late and
  // must not be announced as accepted.
  if (operation === "cancel") return null;
  if (operation === "scan") {
    return status === "ready" ? labels.scanReadyNotice : null;
  }
  if (operation === "commit") {
    return status === "completed" ? labels.commitCompletedNotice : null;
  }
  if (operation === "resume") {
    return status === "running" || status === "completed"
      ? labels.resumeCompletedNotice
      : null;
  }
  return null;
}
