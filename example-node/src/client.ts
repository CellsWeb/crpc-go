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
  private isConnecting = false;
  private rl: readline.Interface | null = null;

  constructor(config: GoRpcClientConfig = {}) {
    this.socketPath = config.socketPath || process.env.CRPC_IPC_SOCKET || "/tmp/crpc.sock";
  }

  private connect(): Promise<net.Socket> {
    if (this.socket && !this.socket.destroyed) {
      return Promise.resolve(this.socket);
    }

    if (this.isConnecting) {
      return new Promise((resolve, reject) => {
        const interval = setInterval(() => {
          if (this.socket && !this.socket.destroyed) {
            clearInterval(interval);
            resolve(this.socket);
          }
        }, 10);
        setTimeout(() => {
          clearInterval(interval);
          reject(new Error("Socket connection timeout"));
        }, 3000);
      });
    }

    this.isConnecting = true;

    return new Promise((resolve, reject) => {
      const sock = net.createConnection({ path: this.socketPath }, () => {
        this.socket = sock;
        this.isConnecting = false;

        this.rl = readline.createInterface({
          input: sock,
          crlfDelay: Infinity,
        });

        this.rl.on("line", (line) => {
          this.handleResponseLine(line);
        });

        resolve(sock);
      });

      sock.on("error", (err) => {
        this.isConnecting = false;
        reject(new Error(`Failed to connect to Go RPC socket at ${this.socketPath}: ${err.message}`));
      });

      sock.on("close", () => {
        this.socket = null;
        if (this.rl) {
          this.rl.close();
          this.rl = null;
        }
      });
    });
  }

  private handleResponseLine(line: string) {
    if (!line.trim()) return;

    try {
      const resp = JSON.parse(line);
      const reqId = resp.id;
      const pending = this.pending.get(reqId);
      if (!pending) return;

      this.pending.delete(reqId);
      clearTimeout(pending.timer);

      if (resp.status === "ok") {
        pending.resolve(resp.data);
      } else {
        pending.reject(new RpcError(resp.code || "rpc_error", resp.error || "RPC call failed", resp.details));
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

    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(id);
        reject(new RpcError("timeout", `RPC call ${String(functionName)} timed out after ${timeoutMs}ms`));
      }, timeoutMs);

      this.pending.set(id, { resolve, reject, timer });

      sock.write(JSON.stringify(payload) + "\n", (err) => {
        if (err) {
          clearTimeout(timer);
          this.pending.delete(id);
          reject(new RpcError("network_error", `Socket write failed: ${err.message}`));
        }
      });
    });
  }

  public close() {
    if (this.socket) {
      this.socket.destroy();
      this.socket = null;
    }
    if (this.rl) {
      this.rl.close();
      this.rl = null;
    }
  }
}

// Default singleton instance
export const goRpc = new GoRpcClient();
