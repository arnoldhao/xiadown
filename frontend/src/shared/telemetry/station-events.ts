import { Call } from "@wailsio/runtime";

const TELEMETRY_HANDLER_SERVICE =
  "xiadown/internal/presentation/wails.TelemetryHandler";
const STATION_RETRY_DELAYS = [100, 250, 500, 1_000, 2_000, 4_000] as const;
const MAX_PENDING_STATIONS = 16;
const KNOWN_STATIONS = new Set([
  "library",
  "music",
  "sniff",
  "rss",
  "youtube",
]);

type PendingStation = {
  station: string;
  failures: number;
};

type StationBridgeCall = (station: string) => Promise<boolean>;
type StationRetryScheduler = (callback: () => void, delayMs: number) => number;

const normalizeStation = (station: string) => {
  const normalized = station.trim().toLowerCase();
  if (!normalized) {
    return "";
  }
  return KNOWN_STATIONS.has(normalized) ? normalized : "other";
};

const defaultBridgeCall: StationBridgeCall = (station) =>
  Call.ByName(
    `${TELEMETRY_HANDLER_SERVICE}.TrackStationOpened`,
    station,
  ).then((accepted) => accepted === true);

const defaultRetryScheduler: StationRetryScheduler = (callback, delayMs) =>
  window.setTimeout(callback, delayMs);

/**
 * A session-scoped, bounded dispatcher. Each coarse station is recorded at
 * most once, while transient bridge startup failures are retried in order.
 */
export class StationTelemetryDispatcher {
  private readonly pending: PendingStation[] = [];
  private readonly seen = new Set<string>();
  private sending = false;
  private disposed = false;

  constructor(
    private readonly bridgeCall: StationBridgeCall = defaultBridgeCall,
    private readonly scheduleRetry: StationRetryScheduler =
      defaultRetryScheduler,
  ) {}

  track(station: string) {
    const normalized = normalizeStation(station);
    if (!normalized || this.disposed || this.seen.has(normalized)) {
      return;
    }
    this.seen.add(normalized);
    if (this.pending.length >= MAX_PENDING_STATIONS) {
      const dropIndex = this.sending && this.pending.length > 1 ? 1 : 0;
      const [dropped] = this.pending.splice(dropIndex, 1);
      if (dropped) {
        this.seen.delete(dropped.station);
      }
    }
    this.pending.push({ station: normalized, failures: 0 });
    this.drain();
  }

  dispose() {
    this.disposed = true;
    this.pending.length = 0;
    this.seen.clear();
  }

  private drain() {
    if (this.disposed || this.sending || this.pending.length === 0) {
      return;
    }
    const entry = this.pending[0];
    if (!entry) {
      return;
    }
    this.sending = true;
    void this.bridgeCall(entry.station).then(
      (accepted) => {
        this.sending = false;
        if (accepted) {
          this.removeEntry(entry);
          this.drain();
          return;
        }
        this.retry(entry);
      },
      () => {
        this.sending = false;
        if (this.disposed) {
          return;
        }
        this.retry(entry);
      },
    );
  }

  private retry(entry: PendingStation) {
    this.removeEntry(entry);
    this.seen.delete(entry.station);
    const retryDelay = STATION_RETRY_DELAYS[entry.failures];
    entry.failures += 1;
    if (retryDelay !== undefined) {
      this.scheduleRetry(() => {
        if (this.disposed || this.seen.has(entry.station)) {
          return;
        }
        this.seen.add(entry.station);
        this.pending.unshift(entry);
        this.drain();
      }, retryDelay);
    }
    this.drain();
  }

  private removeEntry(entry: PendingStation) {
    const index = this.pending.indexOf(entry);
    if (index >= 0) {
      this.pending.splice(index, 1);
    }
  }
}

const stationTelemetry = new StationTelemetryDispatcher();

export function trackStationOpened(station: string) {
  stationTelemetry.track(station);
}

type MainWindowDocument = Pick<Document, "visibilityState" | "hasFocus">;

export function trackStationWhenMainWindowActive(
  station: string,
  documentRef: MainWindowDocument = document,
  track: (value: string) => void = trackStationOpened,
) {
  if (
    documentRef.visibilityState !== "visible" ||
    !documentRef.hasFocus()
  ) {
    return false;
  }
  track(station);
  return true;
}
