let lastBilibiliVideoSurfaceSequence = 0;

/**
 * Native geometry requests are ordered across component remounts, not just
 * within one React instance. Date-based values retain useful diagnostics while
 * the module-level high-water prevents same-millisecond remount collisions.
 */
export function nextRSSBilibiliVideoSurfaceSequence(now = Date.now()) {
  const timeCandidate = Math.max(0, Math.trunc(now)) * 1_000;
  lastBilibiliVideoSurfaceSequence = Math.max(
    lastBilibiliVideoSurfaceSequence + 1,
    timeCandidate,
  );
  return lastBilibiliVideoSurfaceSequence;
}
