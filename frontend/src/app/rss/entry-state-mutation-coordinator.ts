import type { RSSSetEntryStateRequest } from "./types";

interface RSSStateMutationQueue {
  generation: number;
  tail: Promise<void>;
  active: number;
}

export interface RSSStateMutationTicket {
  readonly identity: string;
  readonly generation: number;
  readonly previous: Promise<void>;
  readonly completion: Promise<void>;
  finished: boolean;
  retired: boolean;
  release: () => void;
}

/**
 * Reserves mutations in user-intent order and executes writes serially for one
 * entry field. Different entries and fields remain independent. The generation
 * lets React Query ignore an older response once a newer intent exists.
 */
export class RSSLatestEntryStateMutationCoordinator {
  private readonly queues = new Map<string, RSSStateMutationQueue>();

  reserve(request: Pick<RSSSetEntryStateRequest, "id" | "field">) {
    const identity = rssStateMutationIdentity(request);
    const queue = this.queues.get(identity) ?? {
      generation: 0,
      tail: Promise.resolve(),
      active: 0,
    };
    const previous = queue.tail;
    let releaseCompletion = () => {};
    const releaseSignal = new Promise<void>((resolve) => {
      releaseCompletion = resolve;
    });
    // A cancelled reservation still occupies its original position. Its
    // completion cannot release a later write until every predecessor settles.
    const completion = previous.then(() => releaseSignal);
    queue.generation += 1;
    queue.active += 1;
    queue.tail = completion;
    this.queues.set(identity, queue);

    let released = false;
    const ticket: RSSStateMutationTicket = {
      identity,
      generation: queue.generation,
      previous,
      completion,
      finished: false,
      retired: false,
      release: () => {
        if (released) return;
        released = true;
        ticket.finished = true;
        releaseCompletion();
      },
    };
    return ticket;
  }

  async execute<T>(ticket: RSSStateMutationTicket, operation: () => Promise<T>) {
    await ticket.previous;
    try {
      return await operation();
    } finally {
      ticket.release();
    }
  }

  cancel(ticket: RSSStateMutationTicket) {
    ticket.release();
  }

  isLatest(ticket: RSSStateMutationTicket) {
    return this.queues.get(ticket.identity)?.generation === ticket.generation;
  }

  retire(ticket: RSSStateMutationTicket) {
    const queue = this.queues.get(ticket.identity);
    if (!queue || ticket.retired) return;
    ticket.retired = true;
    queue.active = Math.max(0, queue.active - 1);
    if (queue.active === 0) {
      this.queues.delete(ticket.identity);
    }
  }
}

function rssStateMutationIdentity(
  request: Pick<RSSSetEntryStateRequest, "id" | "field">,
) {
  return `${request.id.trim()}\u0000${request.field}`;
}
