export interface YouTubeSubscriptionRequestToken {
  generation: number;
  identity: string;
  lifecycle: number;
  subject: string;
  subjectGeneration: number;
}

/**
 * Associates an async subscription mutation with the channel/video identity
 * that initiated it. Late results from a previous route or an unmounted view
 * can therefore never update the current screen.
 */
export class YouTubeSubscriptionRequestGate {
  private generation = 0;
  private lifecycle = 0;
  private identity: string;
  private subjectGenerations = new Map<string, number>();

  constructor(identity = "") {
    this.identity = identity;
  }

  activate(identity: string): void {
    if (identity === this.identity) return;
    this.identity = identity;
    this.generation += 1;
  }

  begin(identity: string, subject = identity): YouTubeSubscriptionRequestToken {
    this.activate(identity);
    this.generation += 1;
    const normalizedSubject = subject.trim();
    const subjectGeneration = (this.subjectGenerations.get(normalizedSubject) ?? 0) + 1;
    this.subjectGenerations.set(normalizedSubject, subjectGeneration);
    return {
      generation: this.generation,
      identity,
      lifecycle: this.lifecycle,
      subject: normalizedSubject,
      subjectGeneration,
    };
  }

  isCurrent(token: YouTubeSubscriptionRequestToken): boolean {
    return token.lifecycle === this.lifecycle
      && token.identity === this.identity
      && token.generation === this.generation;
  }

  canReconcile(token: YouTubeSubscriptionRequestToken): boolean {
    return token.lifecycle === this.lifecycle
      && this.subjectGenerations.get(token.subject) === token.subjectGeneration;
  }

  invalidate(): void {
    this.generation += 1;
    this.lifecycle += 1;
  }
}

export function youtubeSubscriptionIdentity(
  channelId: string,
  videoId = "",
): string {
  return `${channelId.trim()}\u0000${videoId.trim()}`;
}

export function resolveYouTubeUploaderSubscriptionSync(
  activeUploaderChannelId: string,
  currentWatchChannelId: string,
  resultChannelId: string,
): { accept: boolean; updateWatch: boolean } {
  const activeUploader = activeUploaderChannelId.trim();
  const currentWatch = currentWatchChannelId.trim();
  const result = resultChannelId.trim();
  const accept = result.length > 0 && result === activeUploader;
  return {
    accept,
    updateWatch: accept && result === currentWatch,
  };
}

export function reconcileYouTubeSubscriptionDetails<
  T extends { channelId?: string; isSubscribed?: boolean },
>(details: T, channelId: string, subscribed: boolean): T;
export function reconcileYouTubeSubscriptionDetails<
  T extends { channelId?: string; isSubscribed?: boolean },
>(details: T | null, channelId: string, subscribed: boolean): T | null;
export function reconcileYouTubeSubscriptionDetails<
  T extends { channelId?: string; isSubscribed?: boolean },
>(details: T | null, channelId: string, subscribed: boolean): T | null {
  if (!details || details.channelId?.trim() !== channelId.trim()) return details;
  return { ...details, isSubscribed: subscribed };
}
