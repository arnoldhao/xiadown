export interface RSSSitePrepareToken {
  generation: number;
  requestId: number;
}

let globalSitePrepareSequence = 0;
let lastSitePrepareRequestId = 0;

function nextSitePrepareRequestId() {
  globalSitePrepareSequence += 1;
  const candidate = Date.now() * 1_000 + (globalSitePrepareSequence % 1_000);
  lastSitePrepareRequestId = Math.max(lastSitePrepareRequestId + 1, candidate);
  return lastSitePrepareRequestId;
}

/**
 * Prevents a late native Prepare from resurrecting a page after the user has
 * already selected another entry or left RSS Station.
 */
export class RSSSitePrepareLifecycle {
  private generation = 0;
  private readonly pending = new Set<number>();

  constructor(private readonly nextRequestId: () => number = nextSitePrepareRequestId) {}

  begin(): RSSSitePrepareToken {
    this.generation += 1;
    const token = {
      generation: this.generation,
      requestId: this.nextRequestId(),
    };
    this.pending.add(token.requestId);
    return token;
  }

  isCurrent(token: RSSSitePrepareToken) {
    return this.pending.has(token.requestId) && token.generation === this.generation;
  }

  settle(token: RSSSitePrepareToken) {
    const pending = this.pending.delete(token.requestId);
    return {
      pending,
      current: pending && token.generation === this.generation,
    };
  }

  cancel(token: RSSSitePrepareToken) {
    if (token.generation === this.generation) this.generation += 1;
    return this.pending.delete(token.requestId);
  }
}
