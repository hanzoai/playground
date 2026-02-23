/**
 * Gateway WebSocket Client (Singleton)
 *
 * Persistent WebSocket connection to hanzo.bot gateway.
 * Survives React route changes — not tied to component lifecycle.
 *
 * Protocol: JSON-RPC over WebSocket (ZAP)
 * Flow: connect → challenge → hello → hello-ok → ready
 */

import type {
  ConnectParams,
  ErrorShape,
  EventFrame,
  Frame,
  GatewayConnectionState,
  HelloOk,
  HelloOkFrame,
  RequestFrame,
  ResponseFrame,
  StateSnapshot,
} from '@/types/gateway';

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

const RECONNECT_DELAYS = [1000, 2000, 4000, 8000, 16000, 30000];

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

type EventHandler = (payload: unknown) => void;
type StateChangeHandler = (state: GatewayConnectionState) => void;

interface PendingRequest {
  resolve: (payload: unknown) => void;
  reject: (error: ErrorShape) => void;
  timer: ReturnType<typeof setTimeout>;
}

// ---------------------------------------------------------------------------
// Client
// ---------------------------------------------------------------------------

class GatewayClient {
  private ws: WebSocket | null = null;
  private url: string = '';
  private connectParams: ConnectParams | null = null;
  private state: GatewayConnectionState = 'disconnected';
  private reconnectAttempt = 0;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private requestId = 0;
  private helloRequestId: string | null = null;
  private pending = new Map<string, PendingRequest>();
  private eventHandlers = new Map<string, Set<EventHandler>>();
  private stateHandlers = new Set<StateChangeHandler>();
  private helloOk: HelloOk | null = null;
  private disposed = false;

  // -------------------------------------------------------------------------
  // Public API
  // -------------------------------------------------------------------------

  get connectionState(): GatewayConnectionState {
    return this.state;
  }

  get serverInfo(): HelloOk | null {
    return this.helloOk;
  }

  get snapshot(): StateSnapshot | null {
    return this.helloOk?.snapshot ?? null;
  }

  get isConnected(): boolean {
    return this.state === 'connected';
  }

  /** The WebSocket URL the client is connected (or connecting) to. */
  get wsUrl(): string {
    return this.url;
  }

  /** The auth token used for gateway connection (for iframe embedding). */
  get authToken(): string | undefined {
    return this.connectParams?.auth?.token;
  }

  /**
   * Connect to the gateway WebSocket.
   */
  connect(url: string, params: ConnectParams): void {
    this.url = url;
    this.connectParams = params;
    this.disposed = false;
    this.reconnectAttempt = 0;
    this.doConnect();
  }

  /**
   * Disconnect and stop reconnecting.
   */
  disconnect(): void {
    this.disposed = true;
    this.helloRequestId = null;
    this.clearReconnect();
    this.closeSocket();
    this.rejectAllPending('Client disconnected');
    this.setState('disconnected');
  }

  /**
   * Send an RPC request and await the response.
   */
  rpc<T = unknown>(method: string, params?: unknown, timeoutMs = 30000): Promise<T> {
    return new Promise((resolve, reject) => {
      if (!this.ws || this.state !== 'connected') {
        reject({ code: 'NOT_CONNECTED', message: 'Gateway not connected' });
        return;
      }

      const id = String(++this.requestId);
      const frame: RequestFrame = { type: 'req', id, method };
      if (params !== undefined) frame.params = params;

      const timer = setTimeout(() => {
        this.pending.delete(id);
        reject({ code: 'TIMEOUT', message: `RPC ${method} timed out after ${timeoutMs}ms` });
      }, timeoutMs);

      this.pending.set(id, {
        resolve: resolve as (p: unknown) => void,
        reject,
        timer,
      });

      this.ws.send(JSON.stringify(frame));
    });
  }

  /**
   * Subscribe to a gateway event.
   */
  on(event: string, handler: EventHandler): () => void {
    let handlers = this.eventHandlers.get(event);
    if (!handlers) {
      handlers = new Set();
      this.eventHandlers.set(event, handlers);
    }
    handlers.add(handler);
    return () => {
      handlers!.delete(handler);
      if (handlers!.size === 0) this.eventHandlers.delete(event);
    };
  }

  /**
   * Subscribe to connection state changes.
   */
  onStateChange(handler: StateChangeHandler): () => void {
    this.stateHandlers.add(handler);
    // Immediately emit current state
    handler(this.state);
    return () => {
      this.stateHandlers.delete(handler);
    };
  }

  // -------------------------------------------------------------------------
  // Internal
  // -------------------------------------------------------------------------

  private doConnect(): void {
    if (this.disposed) return;
    this.closeSocket();
    this.setState('connecting');

    try {
      this.ws = new WebSocket(this.url);
    } catch {
      this.scheduleReconnect();
      return;
    }

    this.ws.onopen = () => {
      this.setState('authenticating');
    };

    this.ws.onmessage = (event) => {
      this.handleMessage(event.data as string);
    };

    this.ws.onclose = () => {
      this.ws = null;
      if (!this.disposed) {
        this.rejectAllPending('Connection lost');
        this.scheduleReconnect();
      }
    };

    this.ws.onerror = () => {
      // onclose will fire after this
    };
  }

  private handleMessage(raw: string): void {
    let frame: Frame;
    try {
      frame = JSON.parse(raw) as Frame;
    } catch {
      return;
    }

    if (frame.type === 'event') {
      this.handleEvent(frame as EventFrame);
    } else if (frame.type === 'hello-ok') {
      // Some gateway versions send hello-ok as a distinct frame type
      this.handleHelloOk((frame as HelloOkFrame).payload as HelloOk ?? {} as HelloOk);
    } else if (frame.type === 'res') {
      this.handleResponse(frame as ResponseFrame);
    }
  }

  private handleEvent(frame: EventFrame): void {
    // Handle connect challenge
    if (frame.event === 'connect.challenge') {
      this.sendHello();
      return;
    }

    // Emit to subscribers
    const handlers = this.eventHandlers.get(frame.event);
    if (handlers) {
      for (const handler of handlers) {
        try {
          handler(frame.payload);
        } catch (e) {
          console.error(`[gateway] Event handler error for ${frame.event}:`, e);
        }
      }
    }
  }

  private handleResponse(frame: ResponseFrame): void {
    // Handle the connect handshake response (sendHello doesn't use rpc(),
    // so the response won't be in the pending map — match by helloRequestId)
    if (this.helloRequestId && frame.id === this.helloRequestId) {
      this.helloRequestId = null;
      if (frame.ok) {
        this.handleHelloOk(frame.payload as HelloOk ?? {} as HelloOk);
      } else {
        console.error('[gateway] Handshake failed:', frame.error?.message ?? 'unknown');
        this.setState('error');
      }
      return;
    }

    // Fallback: detect connect response by state when not tracked by ID
    const pending = this.pending.get(frame.id);
    if (!pending) {
      if (this.state === 'authenticating') {
        if (frame.ok) {
          this.handleHelloOk(frame.payload as HelloOk ?? {} as HelloOk);
        } else {
          console.error('[gateway] Handshake failed:', frame.error?.message ?? 'unknown');
          this.setState('error');
        }
      }
      return;
    }

    this.pending.delete(frame.id);
    clearTimeout(pending.timer);

    if (frame.ok) {
      pending.resolve(frame.payload);
    } else {
      pending.reject(frame.error ?? { code: 'UNKNOWN', message: 'Request failed' });
    }
  }

  private sendHello(): void {
    if (!this.ws || !this.connectParams) return;

    // Bot gateway expects a standard request frame for the handshake:
    // { type: "req", id: "<id>", method: "connect", params: ConnectParams }
    const id = String(++this.requestId);
    this.helloRequestId = id;
    const frame: RequestFrame = {
      type: 'req',
      id,
      method: 'connect',
      params: this.connectParams,
    };

    this.ws.send(JSON.stringify(frame));
  }

  private handleHelloOk(helloOk: HelloOk): void {
    this.helloOk = helloOk;
    this.reconnectAttempt = 0;
    this.setState('connected');
  }

  private setState(state: GatewayConnectionState): void {
    if (this.state === state) return;
    this.state = state;
    for (const handler of this.stateHandlers) {
      try {
        handler(state);
      } catch (e) {
        console.error('[gateway] State handler error:', e);
      }
    }
  }

  private scheduleReconnect(): void {
    if (this.disposed) return;
    this.setState('reconnecting');
    this.clearReconnect();

    const delay = RECONNECT_DELAYS[
      Math.min(this.reconnectAttempt, RECONNECT_DELAYS.length - 1)
    ];
    this.reconnectAttempt++;

    this.reconnectTimer = setTimeout(() => {
      this.doConnect();
    }, delay);
  }

  private clearReconnect(): void {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
  }

  private closeSocket(): void {
    if (this.ws) {
      this.ws.onopen = null;
      this.ws.onmessage = null;
      this.ws.onclose = null;
      this.ws.onerror = null;
      if (this.ws.readyState === WebSocket.OPEN ||
          this.ws.readyState === WebSocket.CONNECTING) {
        this.ws.close();
      }
      this.ws = null;
    }
  }

  private rejectAllPending(reason: string): void {
    for (const [, pending] of this.pending) {
      clearTimeout(pending.timer);
      pending.reject({ code: 'DISCONNECTED', message: reason });
    }
    this.pending.clear();
  }
}

// ---------------------------------------------------------------------------
// Singleton Export
// ---------------------------------------------------------------------------

export const gateway = new GatewayClient();
