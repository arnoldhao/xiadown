import type {
  CompanionDestination,
  CompanionDestinationId,
  CompanionScope,
  CompanionState,
  PersistedRecord,
} from "./types";

/**
 * Declares the serializable identity shared by a Primary selection and the
 * Companion surface that owns it. The open Companion destination is the
 * authority; feature-local payloads are only caches used to render it.
 */
export interface CompanionSelectionContract {
  readonly destinationId: CompanionDestinationId;
  readonly contextKey: string;
}

export function defineCompanionSelectionContract(
  contract: CompanionSelectionContract,
): CompanionSelectionContract {
  const destinationId = contract.destinationId.trim();
  const contextKey = contract.contextKey.trim();
  if (!destinationId || !contextKey) {
    throw new Error(
      "A companion selection contract requires a destinationId and contextKey.",
    );
  }
  return Object.freeze({ destinationId, contextKey });
}

export function createCompanionSelectionDestination(
  contract: CompanionSelectionContract,
  scope: CompanionScope,
  selectionId: string,
  context?: PersistedRecord,
): CompanionDestination {
  const normalizedSelectionId = selectionId.trim();
  if (!normalizedSelectionId) {
    throw new Error("A companion selection destination requires a selection id.");
  }
  return {
    id: contract.destinationId,
    scope,
    context: {
      ...context,
      [contract.contextKey]: normalizedSelectionId,
    },
  };
}

/**
 * Resolves an active selection only while its exact Companion destination is
 * open. Closing, navigating out of scope, replacing the destination, or
 * clearing its context therefore releases Primary selection immediately.
 */
export function resolveActiveCompanionSelectionId(
  companion: CompanionState,
  contract: CompanionSelectionContract,
): string | null {
  if (!companion.open || companion.destination?.id !== contract.destinationId) {
    return null;
  }
  const value = companion.destination.context?.[contract.contextKey];
  if (typeof value !== "string") {
    return null;
  }
  return value.trim() || null;
}

export function resolveActiveCompanionSelection<T>(
  companion: CompanionState,
  contract: CompanionSelectionContract,
  candidate: T | null | undefined,
  getSelectionId: (candidate: T) => string,
): T | null {
  if (!candidate) {
    return null;
  }
  const activeId = resolveActiveCompanionSelectionId(companion, contract);
  return activeId && getSelectionId(candidate).trim() === activeId
    ? candidate
    : null;
}

export type CompanionSelectionResolutionStatus =
  | "inactive"
  | "resolved"
  | "loading"
  | "missing"
  | "unavailable";

export interface CompanionSelectionResolution<T> {
  readonly id: string | null;
  readonly item: T | null;
  readonly status: CompanionSelectionResolutionStatus;
}

export interface ResolveCompanionSelectionOptions<T> {
  /** A click-time cache that may be shown only while canonical data is loading. */
  readonly loadingSnapshot?: T | null;
  readonly loading?: boolean;
  /** Whether an absent id means that the latest canonical result confirmed deletion. */
  readonly authoritative?: boolean;
}

/**
 * Resolves a Companion selection from the latest feature-owned entity map.
 * Cached payloads are deliberately restricted to a loading window: once an
 * authoritative result arrives, a missing id is reported instead of allowing
 * a stale object to live indefinitely in Primary or Companion UI.
 */
export function resolveActiveCompanionSelectionFromMap<T>(
  companion: CompanionState,
  contract: CompanionSelectionContract,
  candidatesById: ReadonlyMap<string, T>,
  getSelectionId: (candidate: T) => string,
  options: ResolveCompanionSelectionOptions<T> = {},
): CompanionSelectionResolution<T> {
  const id = resolveActiveCompanionSelectionId(companion, contract);
  if (!id) {
    return { id: null, item: null, status: "inactive" };
  }

  if (candidatesById.has(id)) {
    return { id, item: candidatesById.get(id) as T, status: "resolved" };
  }

  if (options.loading) {
    const snapshot = options.loadingSnapshot;
    if (snapshot && getSelectionId(snapshot).trim() === id) {
      return { id, item: snapshot, status: "loading" };
    }
    return { id, item: null, status: "loading" };
  }

  return {
    id,
    item: null,
    status: options.authoritative === false ? "unavailable" : "missing",
  };
}
