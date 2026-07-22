import { CircleAlert } from "lucide-react";

const LISTEN_LOCAL_PROBE_ERROR_DETAIL_LIMIT = 160;

export function formatListenLocalProbeWarning(
  message: string,
  error: string,
) {
  const label = message.trim();
  const detail = error.trim().replace(/\s+/g, " ");
  if (!detail) {
    return label;
  }
  const compactDetail =
    detail.length <= LISTEN_LOCAL_PROBE_ERROR_DETAIL_LIMIT
      ? detail
      : `${detail.slice(0, LISTEN_LOCAL_PROBE_ERROR_DETAIL_LIMIT - 1).trimEnd()}…`;
  return label ? `${label}: ${compactDetail}` : compactDetail;
}

export function ListenLocalProbeWarning(props: {
  message: string;
  error: string;
  className?: string;
}) {
  const description = formatListenLocalProbeWarning(
    props.message,
    props.error,
  );
  if (!description) {
    return null;
  }
  return (
    <span
      aria-label={description}
      className={[
        "listen-status-text inline-flex min-w-0 items-center gap-1",
        props.className,
      ]
        .filter(Boolean)
        .join(" ")}
      data-listen-local-probe-warning="true"
      data-tone="warning"
      role="note"
      title={description}
    >
      <CircleAlert aria-hidden="true" className="h-3 w-3 shrink-0" />
      <span className="truncate">{props.message}</span>
    </span>
  );
}
