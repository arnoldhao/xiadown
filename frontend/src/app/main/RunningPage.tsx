import * as React from "react";
import { Loader2, Plus, X } from "lucide-react";

import type {
  LibraryFileDTO,
  OperationListItemDTO,
} from "@/shared/contracts/library";
import type { Pet } from "@/shared/contracts/pets";
import { Button } from "@/shared/ui/button";
import {
  pickFunButtonEffect,
  type FunButtonEffect,
} from "@/shared/ui/fun-button-effect";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/shared/ui/dialog";
import { Progress } from "@/shared/ui/progress";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/shared/ui/tooltip";
import { PetDisplay } from "@/shared/ui/pet-player";
import {
  WorkspacePage,
  WorkspacePageContent,
  WorkspacePageTopBar,
  defineWorkspacePageContract,
} from "@/shared/ui/workspace-page";
import { RunningPetPlayground } from "@/app/main/RunningPetPlayground";
import { useCancelOperation } from "@/shared/query/library";
import { getLanguage, resolveI18nText } from "@/shared/i18n";
import { cn } from "@/lib/utils";
import { formatBytes } from "@/shared/utils/formatBytes";
import { resolveOperationThumbnailCoverURL } from "@/shared/activity/operations";
import { getXiaText } from "@/features/xiadown/shared";
import type { PetAnimation } from "@/shared/pets/animation";
import { resolveOperationKindLabel } from "@/app/main/helpers";

type RunningPageProps = {
  text: ReturnType<typeof getXiaText>;
  operations: OperationListItemDTO[];
  filesById: Map<string, LibraryFileDTO>;
  httpBaseURL: string;
  petImageURL: string;
  petAnimation: PetAnimation;
  pet: Pet | null;
  loading?: boolean;
  reserveWindowControls?: boolean;
  onNewDownload: () => void;
};

type ParsedRunningSpeed =
  | { kind: "bytes"; amount: number }
  | { kind: "frames"; amount: number }
  | { kind: "factor"; amount: number }
  | { kind: "other"; raw: string };

type RunningSpeedCacheEntry = {
  speed: ParsedRunningSpeed;
  expiresAt: number;
};

type RunningVisualQuality = "full" | "balanced" | "low";

const RUNNING_SPEED_UNIT_MULTIPLIERS: Record<string, number> = {
  b: 1,
  kb: 1024,
  kib: 1024,
  mb: 1024 ** 2,
  mib: 1024 ** 2,
  gb: 1024 ** 3,
  gib: 1024 ** 3,
  tb: 1024 ** 4,
  tib: 1024 ** 4,
};

const RUNNING_SPEED_CACHE_TTL_MS = 3500;
const RUNNING_SPEED_SMOOTHING_WEIGHT = 0.42;
const RUNNING_CANCEL_SUPPRESS_TTL_MS = 12_000;
const RUNNING_DOWNLOAD_SPEED_KINDS = new Set<ParsedRunningSpeed["kind"]>([
  "bytes",
]);
const RUNNING_TRANSCODE_SPEED_KINDS = new Set<ParsedRunningSpeed["kind"]>([
  "frames",
  "factor",
]);
function formatRunningTemplate(
  template: string,
  params: Record<string, string | number>,
) {
  return Object.entries(params).reduce(
    (output, [key, value]) => output.split(`{${key}}`).join(String(value)),
    template,
  );
}

function joinRunningParts(
  text: ReturnType<typeof getXiaText>,
  parts: string[],
) {
  return parts.filter(Boolean).join(text.running.separator);
}

function joinRunningSummaryParts(
  text: ReturnType<typeof getXiaText>,
  parts: string[],
) {
  return parts.filter(Boolean).join(text.running.summarySeparator);
}

function DetailValueTooltip(props: {
  label?: string;
  children: React.ReactElement;
  disabled?: boolean;
}) {
  if (!props.label || props.disabled) {
    return <>{props.children}</>;
  }
  return (
    <Tooltip>
      <TooltipTrigger asChild>{props.children}</TooltipTrigger>
      <TooltipContent side="top" className="app-running-detail-tooltip">
        {props.label}
      </TooltipContent>
    </Tooltip>
  );
}

function RunningActionButton(
  props: React.ButtonHTMLAttributes<HTMLButtonElement> & {
    label: string;
    icon: React.ReactNode;
    primary?: boolean;
    effect?: FunButtonEffect;
  },
) {
  const {
    label,
    icon,
    primary = false,
    effect,
    className,
    type = "button",
    ...rest
  } = props;
  return (
    <Button
      type={type}
      variant={primary ? "default" : "ghost"}
      className={cn(
        primary
          ? "app-running-new-download-button"
          : "app-running-action-button",
        className,
      )}
      aria-label={label}
      data-effect={primary ? effect : undefined}
      {...rest}
    >
      {icon}
      <span>{label}</span>
    </Button>
  );
}

function formatRelativeTime(value?: string) {
  if (!value) {
    return "";
  }
  const parsed = Date.parse(value);
  if (!Number.isFinite(parsed)) {
    return value;
  }
  const delta = parsed - Date.now();
  const absDelta = Math.abs(delta);
  const locale = getLanguage();
  const rtf =
    typeof Intl !== "undefined" &&
    typeof Intl.RelativeTimeFormat !== "undefined"
      ? new Intl.RelativeTimeFormat(locale, { numeric: "auto", style: "short" })
      : null;

  const units: Array<{ unit: Intl.RelativeTimeFormatUnit; ms: number }> = [
    { unit: "year", ms: 365 * 24 * 60 * 60 * 1000 },
    { unit: "month", ms: 30 * 24 * 60 * 60 * 1000 },
    { unit: "week", ms: 7 * 24 * 60 * 60 * 1000 },
    { unit: "day", ms: 24 * 60 * 60 * 1000 },
    { unit: "hour", ms: 60 * 60 * 1000 },
    { unit: "minute", ms: 60 * 1000 },
    { unit: "second", ms: 1000 },
  ];

  const match =
    units.find((item) => absDelta >= item.ms) ?? units[units.length - 1];
  const amount = Math.round(delta / match.ms);
  if (rtf) {
    return rtf.format(amount, match.unit);
  }
  return value;
}

function formatElapsedDuration(durationMs?: number) {
  if (!durationMs || durationMs <= 0) {
    return "";
  }
  const totalSeconds = Math.max(0, Math.round(durationMs / 1000));
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  if (hours > 0) {
    return `${hours}:${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`;
  }
  return `${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`;
}

function resolveOperationSourceLabel(
  text: ReturnType<typeof getXiaText>,
  operation: Pick<OperationListItemDTO, "domain" | "platform" | "kind">,
) {
  const fallback =
    operation.kind === "transcode"
      ? text.running.localSource
      : operation.platform?.trim() || "";
  const source = operation.domain?.trim() || fallback;
  if (!source) {
    return "";
  }
  return source === text.running.localSource ? source : source.toUpperCase();
}

function resolveRunningVisualQuality(operationCount: number): RunningVisualQuality {
  if (operationCount >= 48) {
    return "low";
  }
  if (operationCount >= 18) {
    return "balanced";
  }
  return "full";
}

function normalizeStageCode(stage?: string) {
  return (stage ?? "")
    .trim()
    .toLowerCase()
    .replace(/[\s-]+/g, "_");
}

function resolveRunningStageLabel(
  text: ReturnType<typeof getXiaText>,
  stage?: string,
) {
  const localized = resolveI18nText(stage);
  if (localized && localized !== stage?.trim()) {
    return localized;
  }
  switch (normalizeStageCode(stage)) {
    case "starting":
      return text.running.stageLabels.starting;
    case "preparing":
      return text.running.stageLabels.preparing;
    case "fetching_metadata":
      return text.running.stageLabels.fetchingMetadata;
    case "transcoding":
      return text.running.stageLabels.transcoding;
    case "downloading":
      return text.running.stageLabels.downloading;
    case "downloading_video":
      return text.running.stageLabels.downloadingVideo;
    case "downloading_audio":
      return text.running.stageLabels.downloadingAudio;
    case "downloading_subtitles":
      return text.running.stageLabels.downloadingSubtitles;
    case "downloading_thumbnail":
      return text.running.stageLabels.downloadingThumbnail;
    case "muxing":
      return text.running.stageLabels.muxing;
    case "cleaning_up":
      return text.running.stageLabels.cleaningUp;
    case "post_processing":
      return text.running.stageLabels.postProcessing;
    case "queued":
      return text.running.stageLabels.queued;
    case "running":
      return text.running.stageLabels.running;
    case "completed":
      return text.running.stageLabels.completed;
    case "failed":
      return text.running.stageLabels.failed;
    case "canceled":
      return text.running.stageLabels.canceled;
    default:
      return stage?.trim() || "";
  }
}

function parseRunningSpeedMetric(
  operation: Pick<OperationListItemDTO, "progress">,
): ParsedRunningSpeed | null {
  const metric = operation.progress?.speedMetric;
  if (!metric) {
    return null;
  }
  if (
    typeof metric.bytesPerSecond === "number" &&
    Number.isFinite(metric.bytesPerSecond) &&
    metric.bytesPerSecond > 0
  ) {
    return { kind: "bytes", amount: metric.bytesPerSecond };
  }
  if (
    typeof metric.framesPerSecond === "number" &&
    Number.isFinite(metric.framesPerSecond) &&
    metric.framesPerSecond > 0
  ) {
    return { kind: "frames", amount: metric.framesPerSecond };
  }
  if (
    typeof metric.factor === "number" &&
    Number.isFinite(metric.factor) &&
    metric.factor > 0
  ) {
    return { kind: "factor", amount: metric.factor };
  }
  if (metric.label?.trim()) {
    return { kind: "other", raw: metric.label.trim() };
  }
  return null;
}

function parseRunningSpeed(raw?: string): ParsedRunningSpeed | null {
  const value = raw?.trim() ?? "";
  if (!value) {
    return null;
  }

  const bytesMatch = value.match(/([\d.]+)\s*([kmgt]?i?b)\s*\/\s*s/i);
  if (bytesMatch) {
    const amount = Number.parseFloat(bytesMatch[1]);
    const unit = bytesMatch[2].toLowerCase();
    const multiplier = RUNNING_SPEED_UNIT_MULTIPLIERS[unit];
    if (Number.isFinite(amount) && multiplier) {
      return { kind: "bytes", amount: amount * multiplier };
    }
  }

  const framesMatch = value.match(/([\d.]+)\s*fps\b/i);
  if (framesMatch) {
    const amount = Number.parseFloat(framesMatch[1]);
    if (Number.isFinite(amount)) {
      return { kind: "frames", amount };
    }
  }

  const factorMatch = value.match(/([\d.]+)\s*x\b/i);
  if (factorMatch) {
    const amount = Number.parseFloat(factorMatch[1]);
    if (Number.isFinite(amount)) {
      return { kind: "factor", amount };
    }
  }

  return { kind: "other", raw: value };
}

function isRunningOperation(operation: OperationListItemDTO) {
  return (operation.status ?? "").trim().toLowerCase() === "running";
}

function isOperationKind(operation: OperationListItemDTO, kind: string) {
  return (operation.kind ?? "").trim().toLowerCase() === kind;
}

function resolveOperationRawSpeed(
  operation: Pick<OperationListItemDTO, "progress">,
) {
  return (
    parseRunningSpeedMetric(operation) ??
    parseRunningSpeed(operation.progress?.speed)
  );
}

function smoothParsedRunningSpeed(
  previous: ParsedRunningSpeed | undefined,
  next: ParsedRunningSpeed,
) {
  if (!previous || previous.kind !== next.kind) {
    return next;
  }

  switch (next.kind) {
    case "bytes":
    case "frames":
    case "factor": {
      if (previous.kind !== next.kind) {
        return next;
      }
      const amount =
        previous.amount * (1 - RUNNING_SPEED_SMOOTHING_WEIGHT) +
        next.amount * RUNNING_SPEED_SMOOTHING_WEIGHT;
      return Number.isFinite(amount) && amount > 0 ? { ...next, amount } : next;
    }
    case "other":
    default:
      return next;
  }
}

function resolveOperationDisplaySpeed(
  operation: OperationListItemDTO,
  speedCache: Map<string, RunningSpeedCacheEntry>,
) {
  const currentSpeed = resolveOperationRawSpeed(operation);
  const operationId = operation.operationId.trim();
  const cachedEntry = operationId ? speedCache.get(operationId) : undefined;
  const cachedSpeed =
    cachedEntry && cachedEntry.expiresAt > Date.now()
      ? cachedEntry.speed
      : undefined;
  if (cachedSpeed && (currentSpeed || isRunningOperation(operation))) {
    return cachedSpeed;
  }
  return currentSpeed;
}

function useRunningSpeedCache(operations: OperationListItemDTO[]) {
  const [speedCache, setSpeedCache] = React.useState<
    Map<string, RunningSpeedCacheEntry>
  >(() => new Map());

  React.useEffect(() => {
    const now = Date.now();
    setSpeedCache((previous) => {
      const next = new Map<string, RunningSpeedCacheEntry>();
      operations.forEach((operation) => {
        const operationId = operation.operationId.trim();
        if (!operationId) {
          return;
        }

        const currentSpeed = resolveOperationRawSpeed(operation);
        const previousEntry = previous.get(operationId);
        if (currentSpeed) {
          next.set(operationId, {
            speed: smoothParsedRunningSpeed(previousEntry?.speed, currentSpeed),
            expiresAt: now + RUNNING_SPEED_CACHE_TTL_MS,
          });
          return;
        }

        if (
          isRunningOperation(operation) &&
          previousEntry &&
          previousEntry.expiresAt > now
        ) {
          next.set(operationId, previousEntry);
        }
      });
      return next;
    });
  }, [operations]);

  React.useEffect(() => {
    if (speedCache.size === 0) {
      return;
    }

    const now = Date.now();
    let nextExpiresAt = Number.POSITIVE_INFINITY;
    speedCache.forEach((entry) => {
      nextExpiresAt = Math.min(nextExpiresAt, entry.expiresAt);
    });
    if (!Number.isFinite(nextExpiresAt)) {
      return;
    }

    const timeoutId = window.setTimeout(() => {
      setSpeedCache((previous) => {
        const expiryTime = Date.now();
        const next = new Map<string, RunningSpeedCacheEntry>();
        previous.forEach((entry, operationId) => {
          if (entry.expiresAt > expiryTime) {
            next.set(operationId, entry);
          }
        });
        return next.size === previous.size ? previous : next;
      });
    }, Math.max(0, nextExpiresAt - now) + 80);

    return () => window.clearTimeout(timeoutId);
  }, [speedCache]);

  return speedCache;
}

function resolveProgressSummary(
  text: ReturnType<typeof getXiaText>,
  operation: OperationListItemDTO,
) {
  const current = operation.progress?.current ?? 0;
  const total = operation.progress?.total ?? 0;
  const parsedSpeed = resolveOperationRawSpeed(operation);
  if (parsedSpeed?.kind === "bytes" && current > 0 && total > 0) {
    return formatRunningTemplate(text.running.rangeLine, {
      current: formatBytes(current),
      total: formatBytes(total),
    });
  }
  if (
    (parsedSpeed?.kind === "frames" || parsedSpeed?.kind === "factor") &&
    current > 0 &&
    total > 0
  ) {
    return formatRunningTemplate(text.running.rangeLine, {
      current: formatElapsedDuration(current),
      total: formatElapsedDuration(total),
    });
  }
  if (operation.progress?.message?.trim()) {
    return resolveProgressMessage(text, operation.progress.message);
  }
  return "";
}

function resolveProgressMessage(
  text: ReturnType<typeof getXiaText>,
  message?: string,
) {
  const trimmed = message?.trim() ?? "";
  if (!trimmed) {
    return "";
  }
  const localized = resolveI18nText(trimmed);
  if (localized && localized !== trimmed) {
    return localized;
  }
  const stageLabel = resolveRunningStageLabel(text, trimmed);
  return stageLabel && stageLabel !== trimmed ? stageLabel : trimmed;
}

function formatParsedRunningSpeed(
  text: ReturnType<typeof getXiaText>,
  parsed: ParsedRunningSpeed,
) {
  switch (parsed.kind) {
    case "bytes":
      return formatRunningTemplate(text.running.units.bytesPerSecond, {
        value: formatBytes(parsed.amount).replace(/\s+/g, ""),
      });
    case "frames":
      return formatRunningRate(
        parsed.amount,
        text.running.units.framesPerSecond,
      );
    case "factor": {
      const value = parsed.amount
        .toFixed(parsed.amount >= 10 ? 0 : 1)
        .replace(/\.0$/, "");
      return formatRunningTemplate(text.running.units.speedFactor, { value });
    }
    case "other":
      return parsed.raw;
    default:
      return "";
  }
}

function resolveProgressMeta(
  text: ReturnType<typeof getXiaText>,
  operation: OperationListItemDTO,
  speedCache: Map<string, RunningSpeedCacheEntry>,
) {
  const parsedSpeed = resolveOperationDisplaySpeed(operation, speedCache);
  const parts = [
    parsedSpeed ? formatParsedRunningSpeed(text, parsedSpeed) : "",
    resolveRunningStageLabel(text, operation.progress?.stage),
    typeof operation.progress?.percent === "number"
      ? formatRunningTemplate(text.running.percentLabel, {
          value: Math.round(operation.progress.percent),
        })
      : "",
  ].filter(Boolean);
  return joinRunningParts(text, parts);
}

function formatRunningRate(
  value: number,
  template: string,
) {
  if (!Number.isFinite(value) || value <= 0) {
    return "";
  }
  const formatted =
    value >= 100
      ? Math.round(value).toString()
      : value.toFixed(1).replace(/\.0$/, "");
  return formatRunningTemplate(template, { value: formatted });
}

function resolveRunningAggregateSpeed(
  text: ReturnType<typeof getXiaText>,
  operations: OperationListItemDTO[],
  speedCache: Map<string, RunningSpeedCacheEntry>,
  allowedKinds: ReadonlySet<ParsedRunningSpeed["kind"]>,
) {
  let bytesPerSecond = 0;
  let framesPerSecond = 0;
  let speedFactor = 0;
  const extras = new Set<string>();

  operations.forEach((operation) => {
    const parsed = resolveOperationDisplaySpeed(operation, speedCache);
    if (!parsed || !allowedKinds.has(parsed.kind)) {
      return;
    }
    switch (parsed.kind) {
      case "bytes":
        bytesPerSecond += parsed.amount;
        break;
      case "frames":
        framesPerSecond += parsed.amount;
        break;
      case "factor":
        speedFactor += parsed.amount;
        break;
      case "other":
        extras.add(parsed.raw);
        break;
    }
  });

  const parts = [
    bytesPerSecond > 0
      ? formatParsedRunningSpeed(text, { kind: "bytes", amount: bytesPerSecond })
      : "",
    formatRunningRate(framesPerSecond, text.running.units.framesPerSecond),
    speedFactor > 0
      ? formatParsedRunningSpeed(text, { kind: "factor", amount: speedFactor })
      : "",
  ].filter(Boolean);

  return parts.length > 0 ? joinRunningParts(text, parts) : [...extras][0] ?? "";
}

export function RunningPage(props: RunningPageProps) {
  const cancelOperation = useCancelOperation();
  const text = props.text;
  const scrollRef = React.useRef<HTMLDivElement | null>(null);
  const thumbnailURLByOperationRef = React.useRef<Map<string, string>>(
    new Map(),
  );
  const thumbnailArrivalTimeoutsRef = React.useRef<Map<string, number>>(
    new Map(),
  );
  const cancelSuppressTimeoutsRef = React.useRef<Map<string, number>>(
    new Map(),
  );
  const [thumbnailArrivalIds, setThumbnailArrivalIds] = React.useState<
    Set<string>
  >(() => new Set());
  const [cancelConfirmOperation, setCancelConfirmOperation] =
    React.useState<OperationListItemDTO | null>(null);
  const [cancelConfirmError, setCancelConfirmError] = React.useState("");
  const [cancelSuppressedIds, setCancelSuppressedIds] = React.useState<
    Set<string>
  >(() => new Set());
  const [newDownloadEffect] = React.useState<FunButtonEffect>(() =>
    pickFunButtonEffect(),
  );
  const suppressCanceledOperation = React.useCallback((operationId: string) => {
    const trimmed = operationId.trim();
    if (!trimmed) {
      return;
    }
    setCancelSuppressedIds((current) => {
      if (current.has(trimmed)) {
        return current;
      }
      const updated = new Set(current);
      updated.add(trimmed);
      return updated;
    });
    const existingTimeout = cancelSuppressTimeoutsRef.current.get(trimmed);
    if (existingTimeout) {
      window.clearTimeout(existingTimeout);
    }
    const timeout = window.setTimeout(() => {
      cancelSuppressTimeoutsRef.current.delete(trimmed);
      setCancelSuppressedIds((current) => {
        if (!current.has(trimmed)) {
          return current;
        }
        const updated = new Set(current);
        updated.delete(trimmed);
        return updated;
      });
    }, RUNNING_CANCEL_SUPPRESS_TTL_MS);
    cancelSuppressTimeoutsRef.current.set(trimmed, timeout);
  }, []);
  const confirmCancelOperation = React.useCallback(async () => {
    const operation = cancelConfirmOperation;
    const operationId = operation?.operationId.trim() ?? "";
    if (!operationId || cancelOperation.isPending) {
      return;
    }
    setCancelConfirmError("");
    try {
      await cancelOperation.mutateAsync({ operationId });
      suppressCanceledOperation(operationId);
      setCancelConfirmOperation(null);
    } catch (error) {
      setCancelConfirmError(
        error instanceof Error ? error.message : String(error),
      );
    }
  }, [
    cancelConfirmOperation,
    cancelOperation,
    suppressCanceledOperation,
  ]);
  const operations = React.useMemo(
    () =>
      props.operations
        .filter((operation) => {
          const operationId = operation.operationId.trim();
          return !operationId || !cancelSuppressedIds.has(operationId);
        })
        .sort((left, right) => {
          const parsedLeftTime = left.createdAt
            ? new Date(left.createdAt).getTime()
            : 0;
          const parsedRightTime = right.createdAt
            ? new Date(right.createdAt).getTime()
            : 0;
          const leftTime = Number.isFinite(parsedLeftTime) ? parsedLeftTime : 0;
          const rightTime = Number.isFinite(parsedRightTime)
            ? parsedRightTime
            : 0;
          return leftTime - rightTime;
        }),
    [props.operations, cancelSuppressedIds],
  );
  const visualQuality = React.useMemo(
    () => resolveRunningVisualQuality(operations.length),
    [operations.length],
  );
  const runningSpeedCache = useRunningSpeedCache(operations);
  const runningCount = React.useMemo(
    () =>
      operations.filter((operation) => isRunningOperation(operation)).length,
    [operations],
  );
  const queuedCount = React.useMemo(
    () =>
      operations.filter(
        (operation) =>
          (operation.status ?? "").trim().toLowerCase() === "queued",
      ).length,
    [operations],
  );
  const hasDownloadOperation = React.useMemo(
    () => operations.some((operation) => isOperationKind(operation, "download")),
    [operations],
  );
  const hasTranscodeOperation = React.useMemo(
    () => operations.some((operation) => isOperationKind(operation, "transcode")),
    [operations],
  );
  const downloadSpeed = React.useMemo(
    () =>
      hasDownloadOperation
        ? resolveRunningAggregateSpeed(
            text,
            operations.filter((operation) => isOperationKind(operation, "download")),
            runningSpeedCache,
            RUNNING_DOWNLOAD_SPEED_KINDS,
          )
        : "",
    [text, operations, runningSpeedCache, hasDownloadOperation],
  );
  const transcodeSpeed = React.useMemo(
    () =>
      hasTranscodeOperation
        ? resolveRunningAggregateSpeed(
            text,
            operations.filter((operation) => isOperationKind(operation, "transcode")),
            runningSpeedCache,
            RUNNING_TRANSCODE_SPEED_KINDS,
          )
        : "",
    [text, operations, runningSpeedCache, hasTranscodeOperation],
  );
  const runningSummaryLine = React.useMemo(() => {
    const parts = [
      runningCount > 0
        ? formatRunningTemplate(text.running.runningCountLine, {
            count: runningCount,
          })
        : "",
      queuedCount > 0
        ? formatRunningTemplate(text.running.queuedCountLine, {
            count: queuedCount,
          })
        : "",
    ];
    return joinRunningSummaryParts(text, parts) || text.running.progressFallback;
  }, [
    text.running.progressFallback,
    text.running.queuedCountLine,
    text.running.runningCountLine,
    text.running.summarySeparator,
    queuedCount,
    runningCount,
  ]);
  const kindSegments = React.useMemo(() => {
    const segments: Array<{ key: string; label: string; value: string }> = [];
    if (hasDownloadOperation) {
      segments.push({
        key: "download",
        label: text.running.downloadBadge,
        value: downloadSpeed || text.running.unavailable,
      });
    }
    if (hasTranscodeOperation) {
      segments.push({
        key: "transcode",
        label: text.running.transcodeBadge,
        value: transcodeSpeed || text.running.unavailable,
      });
    }
    return segments;
  }, [
    text.running.downloadBadge,
    text.running.transcodeBadge,
    text.running.unavailable,
    downloadSpeed,
    transcodeSpeed,
    hasDownloadOperation,
    hasTranscodeOperation,
  ]);
  const useRunningPetGlow = props.petAnimation === "running";
  const pageContract = defineWorkspacePageContract({
    presentation: "primary",
    recipe: "operational",
    routeLabel: text.running.title,
    topBar: "drag",
    heading: "assistive",
    contentLayout: "canvas",
    footer: "none",
    scroll: "content",
    density: "regular",
    immersion: "edge-to-edge",
  });

  React.useEffect(() => {
    if (!scrollRef.current || operations.length === 0) {
      return;
    }
    scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
  }, [operations.length]);

  React.useLayoutEffect(() => {
    const previous = thumbnailURLByOperationRef.current;
    const next = new Map<string, string>();
    const arrived: string[] = [];

    operations.forEach((operation) => {
      const operationId = operation.operationId.trim();
      if (!operationId) {
        return;
      }
      const thumbnailSignalKey = operation.thumbnailPreviewPath?.trim() ?? "";
      next.set(operationId, thumbnailSignalKey);
      if (thumbnailSignalKey && previous.get(operationId) !== thumbnailSignalKey) {
        arrived.push(operationId);
      }
    });

    thumbnailURLByOperationRef.current = next;
    if (arrived.length === 0) {
      return;
    }

    setThumbnailArrivalIds((current) => {
      const updated = new Set(current);
      arrived.forEach((operationId) => updated.add(operationId));
      return updated;
    });

    arrived.forEach((operationId) => {
      const existingTimeout = thumbnailArrivalTimeoutsRef.current.get(operationId);
      if (existingTimeout) {
        window.clearTimeout(existingTimeout);
      }
      const timeout = window.setTimeout(() => {
        thumbnailArrivalTimeoutsRef.current.delete(operationId);
        setThumbnailArrivalIds((current) => {
          if (!current.has(operationId)) {
            return current;
          }
          const updated = new Set(current);
          updated.delete(operationId);
          return updated;
        });
      }, 1450);
      thumbnailArrivalTimeoutsRef.current.set(operationId, timeout);
    });
  }, [operations]);

  React.useEffect(
    () => () => {
      thumbnailArrivalTimeoutsRef.current.forEach((timeout) => {
        window.clearTimeout(timeout);
      });
      thumbnailArrivalTimeoutsRef.current.clear();
      cancelSuppressTimeoutsRef.current.forEach((timeout) => {
        window.clearTimeout(timeout);
      });
      cancelSuppressTimeoutsRef.current.clear();
    },
    [],
  );

  const renderShell = (children: React.ReactNode) => (
    <WorkspacePage
      contract={pageContract}
      className="app-main-page app-main-running-page"
      data-operation-count={operations.length}
      data-running-state={
        props.loading ? "loading" : operations.length > 0 ? "tasks" : "empty"
      }
      data-visual-quality={visualQuality}
    >
      <WorkspacePageTopBar
        className="app-running-page-toolbar"
        reserveWindowControls={props.reserveWindowControls}
      />
      <WorkspacePageContent
        ref={scrollRef}
        className={cn(
          "app-running-page-content p-0",
          operations.length > 0 && "app-running-card-scroll",
        )}
      >
        {children}
      </WorkspacePageContent>
    </WorkspacePage>
  );

  if (props.loading) {
    return renderShell(
      <div className="flex min-h-full items-center justify-center px-5 pb-5">
        <div className="app-running-loading flex items-center gap-3">
          <Loader2 className="h-4 w-4 app-motion-spin" />
          <span>{text.running.loading}</span>
        </div>
      </div>,
    );
  }

  if (operations.length === 0) {
    return renderShell(
      <div
        className="h-full min-h-0 px-5 pb-5"
        data-running-empty-state="true"
      >
        <RunningPetPlayground
          pet={props.pet}
          imageUrl={props.petImageURL}
          animation={props.petAnimation}
          alt={text.appName}
          hint={text.running.playgroundHint}
        >
          <RunningActionButton
            label={text.running.emptyAction}
            icon={<Plus className="h-4 w-4" />}
            primary
            effect={newDownloadEffect}
            onClick={props.onNewDownload}
          />
        </RunningPetPlayground>
      </div>,
    );
  }

  return renderShell(
    <div className="relative flex min-h-full items-start justify-center px-5 pb-5">
      <div className="flex w-full max-w-4xl flex-col">
        <div className="shrink-0 px-6">
          <div className="app-running-summary-panel grid h-32 grid-cols-[minmax(0,1fr)_auto] items-center gap-6 px-[10%]">
            <div className="relative isolate min-w-0">
              <div className="relative z-10 min-w-0">
                <div className="app-running-summary-line truncate">
                  {runningSummaryLine}
                </div>
                {kindSegments.length > 0 ? (
                  <div className="mt-3 flex min-w-0 flex-wrap items-center gap-2">
                    {kindSegments.map((segment) => (
                      <div
                        key={segment.key}
                        className="app-running-speed-segment grid h-8 min-w-40 w-fit max-w-full grid-cols-[minmax(0,1fr)_auto] items-center gap-2 overflow-visible px-2.5"
                        aria-label={`${segment.label} ${segment.value}`}
                        data-speed-kind={segment.key}
                      >
                        <span className="app-running-speed-text min-w-0 justify-self-start truncate">
                          {segment.label}
                        </span>
                        <span className="app-running-speed-text app-running-speed-value shrink-0 justify-self-end whitespace-nowrap">
                          {segment.value}
                        </span>
                      </div>
                    ))}
                  </div>
                ) : null}
              </div>
            </div>

            <div className="relative z-10 flex shrink-0 justify-end">
              <PetDisplay
                pet={props.pet}
                imageUrl={props.petImageURL}
                animation={props.petAnimation}
                alt={text.appName}
                className="shrink-0"
                glowVariant={useRunningPetGlow ? "running-summary" : undefined}
              />
            </div>
          </div>
        </div>

        <div className="relative px-6">
          <div className="pr-3">
            <div className="flex flex-col gap-3 pb-7 pt-5">
              {operations.map((operation) => {
                const thumbnailCoverURL = resolveOperationThumbnailCoverURL(
                  props.httpBaseURL,
                  operation,
                );
                const percent = Math.max(
                  0,
                  Math.min(100, operation.progress?.percent ?? 0),
                );
                const kindLabel = resolveOperationKindLabel(text, operation.kind);
                const sourceLabel = resolveOperationSourceLabel(text, operation);
                const createdLabel = operation.createdAt
                  ? formatRelativeTime(operation.createdAt)
                  : "";
                const thumbnailArrivalActive = thumbnailArrivalIds.has(
                  operation.operationId,
                );
                const kindCode = normalizeStageCode(operation.kind) || "operation";
                const statusCode = normalizeStageCode(operation.status) || "unknown";
                const stageCode =
                  normalizeStageCode(operation.progress?.stage) || statusCode;

                return (
                  <div
                    key={operation.operationId}
                    className="app-main-running-card app-dream-card group relative isolate overflow-hidden p-4"
                    data-cover-arriving={thumbnailArrivalActive ? "true" : undefined}
                    data-kind={kindCode}
                    data-stage={stageCode}
                    data-status={statusCode}
                  >
                    {thumbnailCoverURL ? (
                      <div
                        className="app-running-thumbnail-stage"
                        data-arriving={thumbnailArrivalActive ? "true" : undefined}
                      >
                        <img
                          src={thumbnailCoverURL}
                          alt=""
                          aria-hidden="true"
                          className="app-running-thumbnail-blur"
                          loading="lazy"
                          decoding="async"
                          draggable={false}
                        />
                        <div className="app-running-thumbnail-detail-host">
                          <img
                            src={thumbnailCoverURL}
                            alt=""
                            aria-hidden="true"
                            className="app-running-thumbnail-detail"
                            loading="lazy"
                            decoding="async"
                            draggable={false}
                          />
                        </div>
                        <div className="app-running-thumbnail-glass-veil" />
                        <div className="app-running-thumbnail-base-wash" />
                        <div className="app-running-thumbnail-grain-field" />
                        <div className="app-running-thumbnail-bottom-wash" />
                        <div className="app-running-thumbnail-light-mix" />
                        <div className="app-running-thumbnail-texture" />
                        <div className="app-running-thumbnail-leading-fade" />
                        <div className="app-running-thumbnail-bottom-glow" />
                        <div
                          aria-hidden="true"
                          className="app-running-thumbnail-sweep"
                        />
                      </div>
                    ) : (
                      <div className="app-running-thumbnail-fallback" />
                    )}
                    <div className="app-running-card-ring" />
                    <div className="relative z-10 space-y-3">
                      <div className="flex min-w-0 items-start gap-4">
                        <div className="min-w-0 flex-1 pt-0.5">
                          <div
                            className="app-running-operation-name truncate"
                            title={operation.name}
                          >
                            {operation.name}
                          </div>
                        </div>
                        <div className="ml-auto flex shrink-0 items-center gap-2">
                          <div className="app-running-meta-strip flex min-w-0 max-w-full shrink-0 items-center overflow-hidden">
                            <span
                              className="app-running-meta-cell app-running-meta-cell-primary app-running-meta-cell--kind"
                              title={kindLabel}
                            >
                              <span className="min-w-0 truncate">{kindLabel}</span>
                            </span>
                            {sourceLabel ? (
                              <DetailValueTooltip label={text.running.source}>
                                <span
                                  className="app-running-meta-cell app-running-meta-cell--source"
                                  title={sourceLabel}
                                >
                                  <span className="app-running-meta-source min-w-0 truncate">
                                    {sourceLabel}
                                  </span>
                                </span>
                              </DetailValueTooltip>
                            ) : null}
                            {createdLabel ? (
                              <DetailValueTooltip label={text.running.createdAt}>
                                <span
                                  className="app-running-meta-cell app-running-meta-cell--created"
                                  title={createdLabel}
                                >
                                  <span className="min-w-0 truncate">
                                    {createdLabel}
                                  </span>
                                </span>
                              </DetailValueTooltip>
                            ) : null}
                          </div>
                          <Button
                            type="button"
                            variant="outline"
                            size="icon"
                            className="app-running-cancel-button"
                            title={text.actions.cancel}
                            aria-label={text.actions.cancel}
                            onClick={() => {
                              setCancelConfirmError("");
                              setCancelConfirmOperation(operation);
                            }}
                          >
                            <X className="h-4 w-4" />
                          </Button>
                        </div>
                      </div>
                      <Progress
                        value={percent}
                        className="app-running-progress h-2.5"
                        data-kind={kindCode}
                        data-stage={stageCode}
                      />
                      <div className="app-running-progress-meta flex items-center justify-between gap-4">
                        <span className="truncate">
                          {resolveProgressSummary(text, operation) ||
                            text.running.progressFallback}
                        </span>
                        <span className="app-running-progress-detail truncate">
                          {resolveProgressMeta(
                            text,
                            operation,
                            runningSpeedCache,
                          )}
                        </span>
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        </div>
      </div>
      <Dialog
        open={Boolean(cancelConfirmOperation)}
        onOpenChange={(open) => {
          if (open || cancelOperation.isPending) {
            return;
          }
          setCancelConfirmOperation(null);
          setCancelConfirmError("");
        }}
      >
        <DialogContent className="w-[calc(100vw-2rem)] max-w-sm max-h-[calc(100vh-2rem)]">
          <DialogHeader>
            <DialogTitle className="app-running-cancel-title overflow-hidden break-words pr-6">
              {text.running.cancelConfirmTitle}
            </DialogTitle>
            <DialogDescription className="app-running-cancel-description max-h-32 overflow-y-auto break-words">
              {formatRunningTemplate(text.running.cancelConfirmDescription, {
                name:
                  cancelConfirmOperation?.name.trim() ||
                  cancelConfirmOperation?.operationId.trim() ||
                  text.running.title,
              })}
            </DialogDescription>
          </DialogHeader>
          {cancelConfirmError ? (
            <div className="app-dream-status-message app-running-cancel-error max-h-24 overflow-y-auto break-words px-3 py-2" data-intent="danger">
              {cancelConfirmError}
            </div>
          ) : null}
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => {
                setCancelConfirmOperation(null);
                setCancelConfirmError("");
              }}
              disabled={cancelOperation.isPending}
            >
              {text.actions.cancelDialog}
            </Button>
            <Button
              type="button"
              variant="destructive"
              onClick={() => void confirmCancelOperation()}
              disabled={cancelOperation.isPending}
            >
              {cancelOperation.isPending ? (
                <Loader2 className="h-4 w-4 app-motion-spin" />
              ) : null}
              {text.actions.cancel}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>,
  );
}
