export interface StorageCapacityBarProps {
  availableBytes: number;
  label: string;
  libraryBytes: number;
  otherBytes: number;
  totalBytes: number;
  valueText: string;
}

function clampCapacity(value: number, maximum: number) {
  if (!Number.isFinite(value) || value <= 0) return 0;
  return Math.min(value, maximum);
}

/**
 * Canonical Dream storage-capacity bar. Callers provide byte counts and
 * localized accessibility copy; Dream CSS owns the segment presentation.
 */
export function StorageCapacityBar(props: StorageCapacityBarProps) {
  const totalBytes =
    Number.isFinite(props.totalBytes) && props.totalBytes > 0
      ? props.totalBytes
      : 0;
  const libraryBytes = clampCapacity(props.libraryBytes, totalBytes);
  const otherBytes = clampCapacity(
    props.otherBytes,
    Math.max(0, totalBytes - libraryBytes),
  );
  const availableBytes = clampCapacity(
    props.availableBytes,
    Math.max(0, totalBytes - libraryBytes - otherBytes),
  );
  const usedBytes = libraryBytes + otherBytes;
  const usedPercent = totalBytes > 0 ? (usedBytes / totalBytes) * 100 : 0;
  const segments = [
    { tone: "library", value: libraryBytes },
    { tone: "other", value: otherBytes },
    { tone: "available", value: availableBytes },
  ] as const;

  return (
    <div
      aria-label={props.label}
      aria-valuemax={100}
      aria-valuemin={0}
      aria-valuenow={Math.round(usedPercent)}
      aria-valuetext={props.valueText}
      className="app-dream-storage-capacity"
      data-capacity-known={totalBytes > 0 ? "true" : "false"}
      role="progressbar"
    >
      {segments.map((segment) =>
        segment.value > 0 && totalBytes > 0 ? (
          <span
            className="app-dream-storage-capacity__segment"
            data-tone={segment.tone}
            key={segment.tone}
            style={{ width: `${(segment.value / totalBytes) * 100}%` }}
          />
        ) : null,
      )}
    </div>
  );
}
