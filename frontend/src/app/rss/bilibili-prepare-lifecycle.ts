export interface RSSBilibiliPrepareToken {
  generation: number;
  requestId: number;
}

export interface RSSBilibiliPrepareSettlement {
  current: boolean;
  pending: boolean;
}

let globalPrepareSequence = 0;
let lastPrepareRequestId = 0;

function nextPrepareRequestId() {
  globalPrepareSequence += 1;
  const candidate = Date.now() * 1_000 + (globalPrepareSequence % 1_000);
  lastPrepareRequestId = Math.max(lastPrepareRequestId + 1, candidate);
  return lastPrepareRequestId;
}

/**
 * Renderer-side half of the native Prepare transaction. It gives every
 * effect generation a globally unique request identity, remembers unresolved
 * RPCs, and makes cleanup cancellation idempotent.
 */
export class RSSBilibiliPrepareLifecycle {
  private generation = 0;
  private readonly pending = new Set<number>();

  constructor(private readonly nextRequestId: () => number = nextPrepareRequestId) {}

  begin(): RSSBilibiliPrepareToken {
    this.generation += 1;
    const token = {
      generation: this.generation,
      requestId: this.nextRequestId(),
    };
    this.pending.add(token.requestId);
    return token;
  }

  isCurrent(token: RSSBilibiliPrepareToken) {
    return this.pending.has(token.requestId) && token.generation === this.generation;
  }

  settle(token: RSSBilibiliPrepareToken): RSSBilibiliPrepareSettlement {
    const pending = this.pending.delete(token.requestId);
    return {
      pending,
      current: pending && token.generation === this.generation,
    };
  }

  cancel(token: RSSBilibiliPrepareToken) {
    if (token.generation === this.generation) {
      this.generation += 1;
    }
    return this.pending.delete(token.requestId);
  }
}
