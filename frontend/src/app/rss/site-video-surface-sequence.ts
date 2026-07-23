let lastSequence = 0;
let sequenceSuffix = 0;

/** Monotonic geometry ordering survives same-millisecond updates and clock rollback. */
export function nextRSSSiteVideoSurfaceSequence(now = Date.now()) {
  sequenceSuffix = (sequenceSuffix + 1) % 1_000;
  const candidate = now * 1_000 + sequenceSuffix;
  lastSequence = Math.max(lastSequence + 1, candidate);
  return lastSequence;
}
