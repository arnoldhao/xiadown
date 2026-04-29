import * as React from "react";
import { Loader2, Plus, X } from "lucide-react";

import type {
  LibraryFileDTO,
  OperationListItemDTO,
} from "@/shared/contracts/library";
import type { Sprite } from "@/shared/contracts/sprites";
import { Button } from "@/shared/ui/button";
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
import { SpriteDisplay } from "@/shared/ui/sprite-player";
import { useCancelOperation } from "@/shared/query/library";
import { getLanguage, resolveI18nText } from "@/shared/i18n";
import { cn } from "@/lib/utils";
import { formatBytes } from "@/shared/utils/formatBytes";
import { buildAssetPreviewURL } from "@/shared/utils/resourceHelpers";
import { getXiaText } from "@/features/xiadown/shared";
import type { SpriteAnimation } from "@/shared/sprites/animation";
import { RUNNING_SPRITE_GLOW_STYLE } from "@/shared/styles/xiadown";
import { resolveOperationKindLabel } from "@/app/main/helpers";

type RunningPageProps = {
  text: ReturnType<typeof getXiaText>;
  operations: OperationListItemDTO[];
  filesById: Map<string, LibraryFileDTO>;
  httpBaseURL: string;
  spriteImageURL: string;
  spriteAnimation: SpriteAnimation;
  sprite: Sprite | null;
  loading?: boolean;
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

type RunningNewDownloadEffect =
  | "water"
  | "fire"
  | "cloud"
  | "sun"
  | "mist"
  | "shadow";

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
const RUNNING_EMPTY_SPRITE_SIZE = 192;
const RUNNING_DOWNLOAD_SPEED_KINDS = new Set<ParsedRunningSpeed["kind"]>([
  "bytes",
]);
const RUNNING_TRANSCODE_SPEED_KINDS = new Set<ParsedRunningSpeed["kind"]>([
  "frames",
  "factor",
]);
const RUNNING_NEW_DOWNLOAD_EFFECTS: RunningNewDownloadEffect[] = [
  "water",
  "fire",
  "cloud",
  "sun",
  "mist",
  "shadow",
];

function pickRunningNewDownloadEffect(): RunningNewDownloadEffect {
  const index = Math.floor(Math.random() * RUNNING_NEW_DOWNLOAD_EFFECTS.length);
  return RUNNING_NEW_DOWNLOAD_EFFECTS[index] ?? "water";
}

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
      <TooltipContent side="top" className="text-xs">
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
    effect?: RunningNewDownloadEffect;
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
          ? "app-running-new-download-button h-10 rounded-full border border-primary/30 bg-primary px-4 text-sm font-semibold text-primary-foreground shadow-[0_16px_34px_-22px_hsl(var(--primary)/0.78)] hover:bg-primary"
          : "h-10 rounded-full border border-primary/[0.15] bg-primary/10 px-4 text-sm font-medium text-primary shadow-sm transition hover:bg-primary/[0.14] hover:text-primary",
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

function resolveOperationThumbnailCoverURL(
  baseURL: string,
  operation: OperationListItemDTO,
) {
  const thumbnailPreviewPath = operation.thumbnailPreviewPath?.trim() ?? "";
  if (!thumbnailPreviewPath) {
    return "";
  }
  return buildAssetPreviewURL(baseURL, thumbnailPreviewPath);
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
  const [newDownloadEffect] = React.useState<RunningNewDownloadEffect>(() =>
    pickRunningNewDownloadEffect(),
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
  const useRunningSpriteGlow = props.spriteAnimation === "working";

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

  if (props.loading) {
    return (
      <div className="flex h-full min-h-0 items-center justify-center">
        <div className="flex items-center gap-3 text-sm text-muted-foreground">
          <Loader2 className="h-4 w-4 animate-spin" />
          <span>{text.running.loading}</span>
        </div>
      </div>
    );
  }

  if (operations.length === 0) {
    return (
      <div className="flex h-full min-h-0 items-center justify-center">
        <div className="flex max-w-sm flex-col items-center text-center">
          <SpriteDisplay
            sprite={props.sprite}
            imageUrl={props.spriteImageURL}
            animation={props.spriteAnimation}
            alt={text.appName}
            className="mb-6"
            glowClassName="h-[18rem] w-[18rem] blur-2xl"
            size={RUNNING_EMPTY_SPRITE_SIZE}
          />
          <RunningActionButton
            label={text.running.emptyAction}
            icon={<Plus className="h-4 w-4" />}
            primary
            effect={newDownloadEffect}
            onClick={props.onNewDownload}
          />
        </div>
      </div>
    );
  }

  return (
    <div className="relative flex h-full min-h-0 items-start justify-center">
      <h1 className="sr-only">{text.running.title}</h1>
      <div className="flex h-full min-h-0 w-full max-w-4xl flex-col">
        <div className="shrink-0 px-6">
          <div className="grid h-32 grid-cols-[minmax(0,1fr)_auto] items-center gap-6 px-[10%]">
            <div className="relative isolate min-w-0">
              <div className="relative z-10 min-w-0">
                <div className="truncate text-2xl font-semibold leading-8 tabular-nums text-foreground">
                  {runningSummaryLine}
                </div>
                {kindSegments.length > 0 ? (
                  <div className="mt-3 flex min-w-0 flex-wrap items-center gap-2">
                    {kindSegments.map((segment) => (
                      <div
                        key={segment.key}
                        className="app-running-speed-segment grid h-8 w-40 grid-cols-[minmax(0,1fr)_auto] items-center gap-2 overflow-visible rounded-lg border border-border/60 bg-background/[0.72] px-2.5 text-xs shadow-sm backdrop-blur-xl"
                        data-speed-kind={segment.key}
                      >
                        <span className="min-w-0 truncate font-semibold text-foreground/82">
                          {segment.label}
                        </span>
                        <span className="shrink-0 font-semibold tabular-nums text-foreground/82">
                          {segment.value}
                        </span>
                      </div>
                    ))}
                  </div>
                ) : null}
              </div>
            </div>

            <div className="relative z-10 flex shrink-0 justify-end">
              <SpriteDisplay
                sprite={props.sprite}
                imageUrl={props.spriteImageURL}
                animation={props.spriteAnimation}
                alt={text.appName}
                className="shrink-0"
                glowClassName={
                  useRunningSpriteGlow
                    ? "h-[10rem] w-[13.5rem] blur-[18px]"
                    : undefined
                }
                glowStyle={
                  useRunningSpriteGlow ? RUNNING_SPRITE_GLOW_STYLE : undefined
                }
              />
            </div>
          </div>
        </div>

        <div className="relative min-h-0 flex-1 px-6">
          <div ref={scrollRef} className="h-full overflow-y-auto pr-3">
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

	                return (
                  <div
                    key={operation.operationId}
                    className="group relative isolate overflow-hidden rounded-[26px] border border-primary/20 bg-[linear-gradient(135deg,hsl(var(--primary)/0.11),hsl(var(--card)/0.82)_46%,hsl(var(--accent)/0.42))] p-4 shadow-[0_22px_52px_-40px_hsl(var(--primary)/0.62)] backdrop-blur-xl"
                  >
                    {thumbnailCoverURL ? (
                      <div
                        className="app-running-thumbnail-stage absolute inset-0 overflow-hidden rounded-[inherit]"
                        data-arriving={thumbnailArrivalActive ? "true" : undefined}
                      >
	                        <img
	                          src={thumbnailCoverURL}
	                          alt=""
	                          aria-hidden="true"
	                          className="app-running-thumbnail-blur pointer-events-none absolute right-0 top-1/2 h-[calc(100%+5rem)] w-[calc(100%+5rem)] -translate-y-1/2 object-cover object-right opacity-[0.74] blur-[54px] saturate-[1.6] contrast-[1.15] brightness-110 transition-opacity duration-300 group-hover:opacity-[0.82]"
	                          style={{
	                            maskImage:
	                              "linear-gradient(90deg, transparent 0%, rgba(0,0,0,0.08) 12%, rgba(0,0,0,0.3) 28%, rgba(0,0,0,0.7) 52%, black 82%, black 100%)",
	                            WebkitMaskImage:
	                              "linear-gradient(90deg, transparent 0%, rgba(0,0,0,0.08) 12%, rgba(0,0,0,0.3) 28%, rgba(0,0,0,0.7) 52%, black 82%, black 100%)",
	                          }}
	                          loading="lazy"
	                          decoding="async"
	                          draggable={false}
	                        />
	                        <div className="absolute inset-y-0 right-0 w-[58%]">
	                          <img
	                            src={thumbnailCoverURL}
	                            alt=""
	                            aria-hidden="true"
	                            className="app-running-thumbnail-detail pointer-events-none absolute inset-0 h-full w-full object-cover object-right opacity-20 blur-[0.9px] saturate-[1.1] contrast-125 brightness-90 mix-blend-luminosity transition duration-300 group-hover:opacity-[0.24]"
	                            style={{
	                              maskImage:
	                                "linear-gradient(90deg, transparent 0%, rgba(0,0,0,0.02) 34%, rgba(0,0,0,0.1) 56%, rgba(0,0,0,0.38) 78%, rgba(0,0,0,0.62) 100%)",
	                              WebkitMaskImage:
	                                "linear-gradient(90deg, transparent 0%, rgba(0,0,0,0.02) 34%, rgba(0,0,0,0.1) 56%, rgba(0,0,0,0.38) 78%, rgba(0,0,0,0.62) 100%)",
	                            }}
	                            loading="lazy"
	                            decoding="async"
	                            draggable={false}
	                          />
	                        </div>
	                        <div
	                          className="absolute inset-y-0 right-0 w-[72%]"
	                          style={{
	                            backdropFilter: "blur(9px) saturate(1.08)",
	                            WebkitBackdropFilter: "blur(9px) saturate(1.08)",
	                            background:
	                              "linear-gradient(90deg,transparent 0%,hsl(var(--card)/0.04) 34%,hsl(var(--background)/0.1) 64%,hsl(var(--card)/0.18) 100%)",
	                            maskImage:
	                              "linear-gradient(90deg,transparent 0%,rgba(0,0,0,0.1) 30%,rgba(0,0,0,0.45) 58%,rgba(0,0,0,0.68) 100%)",
	                            WebkitMaskImage:
	                              "linear-gradient(90deg,transparent 0%,rgba(0,0,0,0.1) 30%,rgba(0,0,0,0.45) 58%,rgba(0,0,0,0.68) 100%)",
	                          }}
	                        />
	                        <div className="absolute inset-0 bg-[linear-gradient(90deg,hsl(var(--card)/0.9)_0%,hsl(var(--card)/0.78)_38%,hsl(var(--background)/0.54)_58%,hsl(var(--background)/0.3)_78%,hsl(var(--primary)/0.08)_100%)]" />
	                        <div
	                          className="absolute inset-y-0 left-[24%] w-[46%]"
	                          style={{
	                            backgroundImage: [
	                              "linear-gradient(90deg,hsl(var(--card)/0.42),hsl(var(--card)/0.2)_46%,transparent)",
	                              "radial-gradient(circle at 12% 18%,hsl(var(--card)/0.44)_0_1px,transparent_2px)",
	                              "radial-gradient(circle at 76% 34%,hsl(var(--background)/0.3)_0_1px,transparent_2px)",
	                              "radial-gradient(circle at 42% 72%,hsl(var(--card)/0.28)_0_1px,transparent_2px)",
	                            ].join(","),
	                            backgroundSize:
	                              "100% 100%, 17px 17px, 23px 23px, 29px 29px",
	                            maskImage:
	                              "linear-gradient(90deg,transparent 0%,black 18%,black 74%,transparent 100%)",
	                            WebkitMaskImage:
	                              "linear-gradient(90deg,transparent 0%,black 18%,black 74%,transparent 100%)",
	                          }}
	                        />
	                        <div className="absolute bottom-0 right-0 h-[76%] w-[82%] bg-[linear-gradient(0deg,hsl(var(--card)/0.82)_0%,hsl(var(--card)/0.52)_34%,hsl(var(--card)/0.2)_66%,transparent_100%)]" />
	                        <div className="absolute inset-0 mix-blend-soft-light bg-[radial-gradient(circle_at_16%_10%,rgba(255,255,255,0.38),transparent_30%),radial-gradient(ellipse_at_86%_70%,hsl(var(--primary)/0.36),transparent_48%),radial-gradient(ellipse_at_96%_18%,hsl(var(--primary-foreground)/0.18),transparent_38%),linear-gradient(120deg,rgba(255,255,255,0.18),rgba(0,0,0,0.2))]" />
	                        <div
	                          className="absolute inset-0 opacity-[0.18] mix-blend-overlay"
	                          style={{
	                            backgroundImage: [
	                              "repeating-radial-gradient(circle at 0 0,rgba(255,255,255,0.2)_0_0.55px,transparent_0.8px_3.8px)",
	                              "repeating-radial-gradient(circle at 1px 1px,rgba(0,0,0,0.14)_0_0.45px,transparent_0.7px_5.6px)",
	                            ].join(","),
	                            backgroundSize: "7px 7px, 11px 11px",
	                          }}
	                        />
	                        <div className="absolute inset-y-0 left-0 w-[58%] bg-[linear-gradient(90deg,hsl(var(--card)/0.58),hsl(var(--card)/0.28)_58%,transparent)]" />
	                        <div className="absolute inset-x-0 bottom-0 h-2/3 bg-[radial-gradient(ellipse_at_68%_100%,hsl(var(--primary)/0.18),transparent_68%)]" />
	                        <div
	                          aria-hidden="true"
	                          className="app-running-thumbnail-sweep pointer-events-none absolute inset-y-[-32%] left-[-34%] w-[34%] rotate-[13deg] opacity-0"
	                        />
	                      </div>
	                    ) : (
                      <>
                        <div className="absolute inset-0 bg-[radial-gradient(ellipse_at_14%_0%,hsl(var(--primary)/0.18),transparent_48%),radial-gradient(ellipse_at_88%_100%,hsl(var(--accent-foreground)/0.12),transparent_54%)]" />
                        <div className="absolute inset-0 bg-[linear-gradient(135deg,hsl(var(--card)/0.86),hsl(var(--background)/0.78)_54%,hsl(var(--primary)/0.1))]" />
                      </>
                    )}
	                    <div className="absolute inset-0 ring-1 ring-inset ring-white/[0.16] dark:ring-white/[0.08]" />
	                    <div className="relative space-y-3">
	                      <div className="flex min-w-0 items-start gap-4">
	                        <div className="min-w-0 flex-1 pt-0.5">
                          <div
                            className="truncate text-base font-semibold text-foreground/86"
                            title={operation.name}
                          >
                            {operation.name}
                          </div>
                        </div>
                        <div className="ml-auto flex shrink-0 items-center gap-2">
                          <div className="flex min-w-0 max-w-full shrink-0 items-center overflow-hidden rounded-lg border border-border/70 bg-background/[0.76] shadow-sm backdrop-blur-xl">
                            <span
                              className="inline-flex h-[var(--app-control-height-compact)] w-[5.75rem] shrink-0 items-center justify-center bg-muted/60 px-2.5 text-[11px] font-medium text-foreground/78"
                              title={kindLabel}
                            >
                              <span className="min-w-0 truncate">{kindLabel}</span>
                            </span>
                            {sourceLabel ? (
                              <DetailValueTooltip label={text.running.source}>
                                <span
                                  className="inline-flex h-[var(--app-control-height-compact)] w-[8.5rem] shrink-0 items-center border-l border-border/70 px-2.5 text-[11px] font-medium text-muted-foreground"
                                  title={sourceLabel}
                                >
                                  <span className="min-w-0 truncate tracking-[0.08em] uppercase">
                                    {sourceLabel}
                                  </span>
                                </span>
                              </DetailValueTooltip>
                            ) : null}
                            {createdLabel ? (
                              <DetailValueTooltip label={text.running.createdAt}>
                                <span
                                  className="inline-flex h-[var(--app-control-height-compact)] w-[6.25rem] shrink-0 items-center border-l border-border/70 px-2.5 text-[11px] font-medium text-muted-foreground"
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
                            className="rounded-lg border-border/70 bg-background/[0.68] text-muted-foreground backdrop-blur-xl hover:border-destructive/25 hover:bg-destructive/10 hover:text-destructive"
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
                        className="h-2.5 bg-primary/[0.10] dark:bg-primary/[0.16]"
                      />
                      <div className="flex items-center justify-between gap-4 text-xs text-muted-foreground">
                        <span className="truncate">
                          {resolveProgressSummary(text, operation) ||
                            text.running.progressFallback}
                        </span>
                        <span className="truncate text-right">
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
          <div
            aria-hidden="true"
            className="pointer-events-none absolute left-6 right-[calc(1.5rem+0.75rem+10px)] top-0 z-20 h-5 bg-gradient-to-b from-background via-background/92 to-transparent"
          />
          <div
            aria-hidden="true"
            className="pointer-events-none absolute bottom-0 left-6 right-[calc(1.5rem+0.75rem+10px)] z-20 h-7 bg-gradient-to-t from-background via-background/92 to-transparent"
          />
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
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle>{text.running.cancelConfirmTitle}</DialogTitle>
            <DialogDescription>
              {formatRunningTemplate(text.running.cancelConfirmDescription, {
                name:
                  cancelConfirmOperation?.name.trim() ||
                  cancelConfirmOperation?.operationId.trim() ||
                  text.running.title,
              })}
            </DialogDescription>
          </DialogHeader>
          {cancelConfirmError ? (
            <div className="rounded-lg border border-destructive/20 bg-destructive/10 px-3 py-2 text-xs text-destructive">
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
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : null}
              {text.actions.cancel}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
