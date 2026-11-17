import net from "node:net";
import readline from "node:readline";
import { randomUUID } from "node:crypto";
import type { RpcContractMap } from "../crpc.d.ts";

export interface CallOptions {
  headers?: Record<string, string>;
  skipMiddlewares?: boolean;
  /**
   * Dependency injection context passed directly to Go's ctx.
   * MUST be namespaced by middleware name, e.g.:
   * context: {
   *   auth: {
   *     user: { id: "usr_1", email: "user@example.com", role: "admin" }
   *   }
   * }
   */
  context?: Record<string, Record<string, unknown>>;
  timeoutMs?: number;
}

export class RpcError extends Error {
  public readonly code: string;
  public readonly details?: unknown;

  constructor(code: string, message: string, details?: unknown) {
    super(message);
    this.name = "RpcError";
    this.code = code;
    this.details = details;
  }
}

interface PendingRequest {
  resolve: (data: any) => void;
  reject: (err: Error) => void;
  timer: NodeJS.Timeout;
}

export interface GoRpcClientConfig {
  socketPath?: string;
}

export class GoRpcClient {
  private socketPath: string;
  private socket: net.Socket | null = null;
  private pending = new Map<string, PendingRequest>();
  private connecting: Promise<net.Socket> | null = null;
  private disconnect: ((error: Error) => void) | null = null;
  private rl: readline.Interface | null = null;

  constructor(config: GoRpcClientConfig = {}) {
    this.socketPath = config.socketPath || process.env.CRPC_IPC_SOCKET || "/tmp/crpc.sock";
  }

  private connect(): Promise<net.Socket> {
    if (this.connecting) return this.connecting;
    if (this.socket && !this.socket.destroyed) {
      return Promise.resolve(this.socket);
    }
    // destroy() marks a socket synchronously, but close arrives later. Retire
    // that generation before a new socket can take ownership of pending calls.
    if (this.socket) {
      this.disconnect?.(new RpcError("network_error", "IPC socket closed before response was received"));
    }

    const sock = net.createConnection({ path: this.socketPath });
    this.socket = sock;
    this.connecting = new Promise((resolve, reject) => {
      let finished = false;
      let connected = false;
      let reader: readline.Interface | null = null;
      const timer = setTimeout(() => {
        disconnect(new RpcError("network_error", "Socket connection timeout"));
      }, 3000);
      const onLine = (line: string) => {
        if (!finished && this.socket === sock) this.handleResponseLine(line);
      };
      const disconnect = (error: Error) => {
        if (finished) return;
        finished = true;
        clearTimeout(timer);
        sock.off("connect", onConnect);
        reader?.off("line", onLine);
        reader?.close();
        if (this.socket === sock) {
          this.socket = null;
          this.rl = null;
          this.connecting = null;
          this.disconnect = null;
          for (const id of this.pending.keys()) this.settleRequest(id, error);
        }
        reject(error); // Also releases callers still awaiting connection setup.
        sock.destroy();
      };
      const onError = (error: Error) => {
        disconnect(new RpcError("network_error", connected
          ? `IPC socket error: ${error.message}`
          : `Failed to connect to Go RPC socket at ${this.socketPath}: ${error.message}`));
      };
      const onClose = () => {
        disconnect(new RpcError("network_error", "IPC socket closed before response was received"));
        sock.off("error", onError);
        sock.off("close", onClose);
        reader?.off("error", onError);
      };
      const onConnect = () => {
        if (finished) return;
        connected = true;
        clearTimeout(timer);
        this.connecting = null;
        reader = readline.createInterface({ input: sock, crlfDelay: Infinity });
        this.rl = reader;
        reader.on("line", onLine);
        // Keep this until socket close: a socket error can also be forwarded
        // by readline during the same event dispatch, even after teardown.
        reader.on("error", onError);
        resolve(sock);
      };
      this.disconnect = disconnect;
      sock.once("connect", onConnect);
      sock.on("error", onError);
      sock.on("close", onClose);
    });
    return this.connecting;
  }

  private settleRequest(id: string, error?: Error, data?: unknown) {
    const pending = this.pending.get(id);
    if (!pending) return;
    this.pending.delete(id);
    clearTimeout(pending.timer);
    if (error) pending.reject(error);
    else pending.resolve(data);
  }

  private handleResponseLine(line: string) {
    if (!line.trim()) return;

    try {
      const resp = JSON.parse(line);
      if (!resp || typeof resp !== "object" || Array.isArray(resp)) return;
      const reqId = resp.id;
      if ((reqId === "" || reqId == null) && resp.status === "error") {
        // The server could not identify the failed frame. No arbitrary pending
        // request can be blamed; abandon this connection without replaying it.
        this.disconnect?.(new RpcError("network_error", `IPC connection error: ${resp.error || "anonymous server error"}`, resp.details));
        return;
      }
      if (typeof reqId !== "string") return;

      if (resp.status === "ok") {
        this.settleRequest(reqId, undefined, resp.data);
      } else {
        this.settleRequest(reqId, new RpcError(resp.code || "rpc_error", resp.error || "RPC call failed", resp.details));
      }
    } catch (err: any) {
      console.error("[crpc-ipc] Failed to parse response line:", err);
    }
  }

  /**
   * Invokes a Go RPC function over high-speed Unix Domain Socket IPC
   */
  public async call<K extends keyof RpcContractMap>(
    functionName: K,
    data: RpcContractMap[K]["input"],
    options: CallOptions = {}
  ): Promise<RpcContractMap[K]["output"]> {
    const sock = await this.connect();
    const id = randomUUID();
    const timeoutMs = options.timeoutMs ?? 5000;

    const payload = {
      id,
      function: functionName,
      data,
      headers: options.headers,
      skipMiddlewares: options.skipMiddlewares,
      context: options.context,
    };
    // Encoding can throw (e.g. circular data). Do it before registering a timer.
    const wire = JSON.stringify(payload) + "\n";
    if (this.socket !== sock || sock.destroyed) {
      throw new RpcError("network_error", "IPC socket closed before response was received");
    }

    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        this.settleRequest(id, new RpcError("timeout", `RPC call ${String(functionName)} timed out after ${timeoutMs}ms`));
      }, timeoutMs);

      this.pending.set(id, { resolve, reject, timer });

      try {
        sock.write(wire, (err) => {
          if (err) this.settleRequest(id, new RpcError("network_error", `Socket write failed: ${err.message}`));
        });
      } catch (err: any) {
        this.settleRequest(id, new RpcError("network_error", `Socket write failed: ${err.message}`));
      }
    });
  }

  public close() {
    this.disconnect?.(new RpcError("network_error", "IPC socket closed before response was received"));
  }
}

// Default singleton instance
export const goRpc = new GoRpcClient();
