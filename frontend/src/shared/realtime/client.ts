export type ConnectionStatus = "disconnected" | "connecting" | "connected";

export type RealtimeEvent = {
  id: string;
  topic: string;
  type?: string;
  seq?: number;
  replay?: boolean;
  payload?: unknown;
  ts: number;
};

type SubscriptionHandler = (event: RealtimeEvent) => void;

export interface WebSocketClientOptions {
  reconnectIntervalMs?: number;
  connectTimeoutMs?: number;
  requestTimeoutMs?: number;
  onStatusChange?: (status: ConnectionStatus) => void;
  onMessage?: (event: RealtimeEvent) => void;
  onMetric?: (kind: "reconnect" | "replay" | "resync-required" | "duplicate-drop") => void;
}

export class WebSocketClient {
  private socket: WebSocket | null = null;
  private readonly url: string;
  private readonly reconnectIntervalMs: number;
  private readonly connectTimeoutMs: number;
  private readonly subscriptions = new Map<string, Set<SubscriptionHandler>>();
  private readonly topicSeq = new Map<string, number>();
  private reconnectTimer: number | null = null;
  private status: ConnectionStatus = "disconnected";
  private readonly onStatusChange?: (status: ConnectionStatus) => void;
  private readonly onMessage?: (event: RealtimeEvent) => void;
  private readonly onMetric?: (kind: "reconnect" | "replay" | "resync-required" | "duplicate-drop") => void;
  private shouldReconnect = true;
  private readonly requestTimeoutMs: number;
  private readonly pending = new Map<
    string,
    { resolve: (value: any) => void; reject: (reason?: any) => void; timeoutId: number }
  >();

  constructor(url: string, options?: WebSocketClientOptions) {
    this.url = url;
    this.reconnectIntervalMs = options?.reconnectIntervalMs ?? 3_000;
    this.connectTimeoutMs = options?.connectTimeoutMs ?? 5_000;
    this.requestTimeoutMs = options?.requestTimeoutMs ?? 5_000;
    this.onStatusChange = options?.onStatusChange;
    this.onMessage = options?.onMessage;
    this.onMetric = options?.onMetric;
  }

  connect() {
    if (this.socket) {
      return;
    }
    this.shouldReconnect = true;
    this.setStatus("connecting");

    const controller = new AbortController();
    const timeout = window.setTimeout(() => controller.abort(), this.connectTimeoutMs);

    try {
      this.socket = new WebSocket(this.url);
      this.socket.addEventListener("open", () => {
        window.clearTimeout(timeout);
        this.setStatus("connected");
        this.flushPendingSubscriptions();
      });
      this.socket.addEventListener("message", (event) => this.handleMessage(event));
      this.socket.addEventListener("close", () => this.handleClose());
      this.socket.addEventListener("error", () => this.handleClose());
    } catch (error) {
      console.error("[ws] failed to connect", error);
      window.clearTimeout(timeout);
      this.handleClose();
    }

    controller.signal.addEventListener("abort", () => {
      this.socket?.close();
      this.socket = null;
    });
  }

  disconnect() {
    this.shouldReconnect = false;
    if (this.reconnectTimer) {
      window.clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    this.socket?.close();
    this.socket = null;
    this.setStatus("disconnected");
    this.clearPending(new Error("socket disconnected"));
  }

  send(payload: unknown) {
    if (!this.socket || this.socket.readyState !== WebSocket.OPEN) {
      console.warn("[ws] socket not ready, drop payload", payload);
      return;
    }
    this.socket.send(JSON.stringify(payload));
  }

  sendRequest(method: string, params?: unknown, timeoutMs?: number): Promise<any> {
    const id = `${Date.now()}-${Math.random().toString(16).slice(2)}`;
    const request = { type: "req", id, method, params };
    const timeout = window.setTimeout(() => {
      const entry = this.pending.get(id);
      if (entry) {
        this.pending.delete(id);
        entry.reject(new Error("realtime request timeout"));
      }
    }, timeoutMs ?? this.requestTimeoutMs);
    const promise = new Promise<any>((resolve, reject) => {
      this.pending.set(id, { resolve, reject, timeoutId: timeout });
    });
    this.send(request);
    return promise;
  }

  subscribe(topic: string, handler: SubscriptionHandler): () => void {
    const handlers = this.subscriptions.get(topic) ?? new Set<SubscriptionHandler>();
    const isNewTopic = handlers.size === 0;
    handlers.add(handler);
    this.subscriptions.set(topic, handlers);

    if (isNewTopic && this.socket?.readyState === WebSocket.OPEN) {
      this.sendSubscribe([topic]);
    }

    return () => {
      const current = this.subscriptions.get(topic);
      current?.delete(handler);
      if (current && current.size === 0) {
        this.subscriptions.delete(topic);
        if (this.socket?.readyState === WebSocket.OPEN) {
          this.send({ action: "unsubscribe", topics: [topic] });
        }
      }
    };
  }

  private flushPendingSubscriptions() {
    if (!this.socket || this.socket.readyState !== WebSocket.OPEN) {
      return;
    }
    const topics = Array.from(this.subscriptions.keys()).filter((topic) => topic.trim().length > 0);
    if (topics.length > 0) {
      this.sendSubscribe(topics);
    }
  }

  private sendSubscribe(topics: string[]) {
    const cursors: Record<string, number> = {};
    topics.forEach((topic) => {
      const normalized = topic.trim();
      if (!normalized) {
        return;
      }
      const seq = this.topicSeq.get(normalized) ?? 0;
      if (seq > 0) {
        cursors[normalized] = seq;
      }
    });
    const payload: Record<string, unknown> = { action: "subscribe", topics };
    if (Object.keys(cursors).length > 0) {
      payload.cursors = cursors;
    }
    this.send(payload);
  }

  private scheduleReconnect() {
    this.socket = null;
    if (this.reconnectTimer || !this.shouldReconnect) {
      return;
    }
    this.emitMetric("reconnect");
    this.reconnectTimer = window.setTimeout(() => {
      this.reconnectTimer = null;
      this.connect();
    }, this.reconnectIntervalMs);
  }

  private handleClose() {
    this.setStatus("disconnected");
    this.clearPending(new Error("socket closed"));
    this.scheduleReconnect();
  }

  private handleMessage(event: MessageEvent<string>) {
    const raw = event.data;
    let parsed: any;
    try {
      parsed = JSON.parse(raw);
    } catch {
      console.warn("[ws] ignore non-JSON message", raw);
      return;
    }

    if (parsed?.type === "res" && parsed?.id) {
      const entry = this.pending.get(parsed.id);
      if (entry) {
        window.clearTimeout(entry.timeoutId);
        this.pending.delete(parsed.id);
        if (parsed.ok) {
          entry.resolve(parsed.payload);
        } else {
          entry.reject(parsed.error ?? new Error("realtime request failed"));
        }
      }
      return;
    }

    const isLegacyEvent = parsed?.type === "event" && parsed?.event;
    const topic = parsed?.topic ?? parsed?.Topic ?? (isLegacyEvent ? String(parsed.event) : "");
    if (!topic) {
      return;
    }

    const tsValue = parsed?.ts ?? parsed?.TS;
    const timestamp =
      typeof tsValue === "number" ? tsValue : typeof tsValue === "string" ? Date.parse(tsValue) || Date.now() : Date.now();
    const realtimeEvent: RealtimeEvent = {
      id: parsed?.id ?? parsed?.ID ?? `${Date.now()}`,
      topic: String(topic),
      type: isLegacyEvent ? parsed?.event : parsed?.type ?? parsed?.Type,
      seq: typeof (parsed?.seq ?? parsed?.Seq) === "number" ? Number(parsed?.seq ?? parsed?.Seq) : undefined,
      replay: Boolean(parsed?.replay ?? parsed?.Replay),
      payload: parsed?.payload ?? parsed?.Payload,
      ts: timestamp,
    };

    const seq = realtimeEvent.seq ?? 0;
    if (seq > 0) {
      const lastSeq = this.topicSeq.get(realtimeEvent.topic) ?? 0;
      if (seq <= lastSeq) {
        this.emitMetric("duplicate-drop");
        return;
      }
      this.topicSeq.set(realtimeEvent.topic, seq);
    }

    if (realtimeEvent.replay) {
      this.emitMetric("replay");
    }
    if ((realtimeEvent.type ?? "").trim().toLowerCase() === "resync-required") {
      this.emitMetric("resync-required");
    }

    this.emitMessage(realtimeEvent);

    const handlers = this.subscriptions.get(realtimeEvent.topic);
    if (!handlers || handlers.size === 0) {
      return;
    }

    handlers.forEach((handler) => {
      try {
        handler(realtimeEvent);
      } catch (error) {
        console.error("[ws] subscription handler failed", error);
      }
    });
  }

  private setStatus(status: ConnectionStatus) {
    if (this.status === status) {
      return;
    }
    this.status = status;
    try {
      this.onStatusChange?.(status);
    } catch (error) {
      console.error("[ws] status handler failed", error);
    }
  }

  private clearPending(reason: Error) {
    this.pending.forEach((entry) => {
      window.clearTimeout(entry.timeoutId);
      entry.reject(reason);
    });
    this.pending.clear();
  }

  private emitMetric(kind: "reconnect" | "replay" | "resync-required" | "duplicate-drop") {
    try {
      this.onMetric?.(kind);
    } catch (error) {
      console.error("[ws] metric handler failed", error);
    }
  }

  private emitMessage(event: RealtimeEvent) {
    try {
      this.onMessage?.(event);
    } catch (error) {
      console.error("[ws] message handler failed", error);
    }
  }
}
