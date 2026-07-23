import type {
  RSSArticleProgress,
  RSSEntry,
  RSSEntryStateField,
  RSSSetEntryStateRequest,
} from "./types";

export function buildRSSReadStateRequest(
  entry: Pick<RSSEntry, "id" | "fieldRevisions">,
  read: boolean,
): RSSSetEntryStateRequest {
  return buildRSSStateRequest(entry, "read", { read });
}

export function buildRSSStarredStateRequest(
  entry: Pick<RSSEntry, "id" | "fieldRevisions">,
  starred: boolean,
): RSSSetEntryStateRequest {
  return buildRSSStateRequest(entry, "starred", { starred });
}

export function buildRSSArticleProgressStateRequest(
  entry: Pick<RSSEntry, "id" | "fieldRevisions" | "revision">,
  fraction: number,
  anchor = "",
): RSSSetEntryStateRequest {
  const articleProgress: RSSArticleProgress = {
    fraction: clamp(fraction, 0, 1),
    anchor: anchor.trim().slice(0, 256) || undefined,
    contentRevision: Math.max(1, Math.trunc(entry.revision)),
  };
  return buildRSSStateRequest(entry, "articleProgress", { articleProgress });
}

export function buildRSSVideoProgressStateRequest(
  entry: Pick<RSSEntry, "id" | "fieldRevisions">,
  currentTime: number,
  duration: number,
): RSSSetEntryStateRequest {
  const safeDuration = Math.max(0, finiteNumber(duration));
  const safeCurrentTime = clamp(
    finiteNumber(currentTime),
    0,
    safeDuration || Number.MAX_SAFE_INTEGER,
  );
  return buildRSSStateRequest(entry, "videoProgressSeconds", {
    videoProgressSeconds: safeCurrentTime,
    ...(safeDuration > 0 ? { videoDurationSeconds: safeDuration } : {}),
  });
}

export function createRSSMutationID() {
  return typeof crypto !== "undefined" && "randomUUID" in crypto
    ? crypto.randomUUID()
    : `${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

export function rssFieldRevision(
  entry: Pick<RSSEntry, "fieldRevisions">,
  field: RSSEntryStateField,
) {
  return Math.max(0, Math.trunc(entry.fieldRevisions?.[field] ?? 0));
}

function buildRSSStateRequest(
  entry: Pick<RSSEntry, "id" | "fieldRevisions">,
  field: RSSEntryStateField,
  value: Omit<
    RSSSetEntryStateRequest,
    "id" | "field" | "expectedRevision" | "mutationId"
  >,
): RSSSetEntryStateRequest {
  return {
    id: entry.id,
    field,
    expectedRevision: rssFieldRevision(entry, field),
    mutationId: createRSSMutationID(),
    ...value,
  };
}

function finiteNumber(value: number) {
  return Number.isFinite(value) ? value : 0;
}

function clamp(value: number, minimum: number, maximum: number) {
  return Math.max(minimum, Math.min(maximum, finiteNumber(value)));
}
