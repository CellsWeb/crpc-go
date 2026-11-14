---
layout: default
title: Node.js & Next.js IPC Integration
nav_order: 6
---

# Node.js & Next.js IPC Integration

CellsWeb RPC Go features native, high-performance Inter-Process Communication (IPC) via **Unix Domain Sockets** (`/tmp/crpc.sock`) and **Stdio Streaming**.

This allows fullstack frameworks like **Next.js**, **Express**, **Fastify**, and **Remix** to invoke compiled Go RPC functions with **<50µs latency** while preserving complete end-to-end TypeScript type safety.

---

## ⚡ Unix Domain Socket vs. HTTP

| Metric | HTTP (`localhost:4001/rpc`) | Unix Domain Socket IPC (`/tmp/crpc.sock`) |
| :--- | :--- | :--- |
| **Transport** | TCP Loopback Stack | Direct Kernel Socket Buffers |
| **Call Latency** | 250µs – 600µs | **20µs – 50µs** *(10x faster)* |
| **Port Clashes** | Can collide on dev/prod | Zero network ports; isolated filesystem node |
| **Access Control** | Open to localhost | Restricted by OS file permissions (`0o600`) |
| **Auth Injection** | Must re-verify JWT in Go | Injects verified Node session directly into `ctx` |

---

## 🔑 Crucial Architectural Rule: Context Namespacing

When calling Go RPC functions from Node.js with `skipMiddlewares: true` (e.g. inside Next.js Server Actions where the user is already authenticated by NextAuth / Supabase / Clerk), **dependency injection context MUST be namespaced by the middleware's name**:

```json
{
  "function": "user.getProfile",
  "data": { "id": "usr_101" },
  "skipMiddlewares": true,
  "context": {
    "auth": {
      "user": {
        "id": "usr_101",
        "email": "user@example.com",
        "role": "admin"
      }
    }
  }
}
```

> **Why?**
> In Go, handlers read middleware output via `ctx.Get("auth")` (or `rpc.GetContext[AuthOutput](ctx, "auth")`).
> When you pass `context: { auth: { user: ... } }`, Go maps it directly to `ctx.Set("auth", ...)`.
> Your Go business logic behaves **identically** whether the context was populated by Go's `auth` middleware or injected by a Node.js session!

---

## 🛠️ Step 1: Exporting TypeScript Definitions

In your Go project, generate exact TypeScript interfaces from your Go structs and schemas:

```go
package main

import (
	"os"
	"github.com/CellsWeb/crpc-go/pkg/nodebridge"
	"github.com/CellsWeb/crpc-go/pkg/rpc"
)

func main() {
	app, _ := rpc.New(rpc.Config{RpcPath: "/rpc", CORS: &rpc.CorsConfig{AllowOrigins: []string{"*"}}})
	app.UseGlobalRegistry()

	dts := nodebridge.GenerateTypeScriptDeclarations(app.Functions())
	os.WriteFile("crpc.d.ts", []byte(dts), 0644)
}
```

This generates `crpc.d.ts` with strict input/output contracts and the `RpcContractMap` interface:

```typescript
export interface RpcContractMap {
  "user.getProfile": {
    input: UserGetProfileInput;
    output: UserGetProfileOutput;
  };
}
```

---

## 💻 Step 2: Zero-Dependency Node.js Client

Create `lib/go-rpc.ts`:

```typescript
import net from "node:net";
import readline from "node:readline";
import { randomUUID } from "node:crypto";
import type { RpcContractMap } from "./crpc.d.ts";

export class GoRpcClient {
  private socketPath: string;
  private socket: net.Socket | null = null;
  private pending = new Map<string, { resolve: Function; reject: Function; timer: any }>();

  constructor(socketPath = process.env.CRPC_IPC_SOCKET || "/tmp/crpc.sock") {
    this.socketPath = socketPath;
  }

  private async connect(): Promise<net.Socket> {
    if (this.socket && !this.socket.destroyed) return this.socket;

    return new Promise((resolve, reject) => {
      const sock = net.createConnection({ path: this.socketPath }, () => {
        this.socket = sock;
        const rl = readline.createInterface({ input: sock, crlfDelay: Infinity });
        rl.on("line", (line) => {
          const resp = JSON.parse(line);
          const p = this.pending.get(resp.id);
          if (p) {
            this.pending.delete(resp.id);
            clearTimeout(p.timer);
            resp.status === "ok" ? p.resolve(resp.data) : p.reject(new Error(resp.error));
          }
        });
        resolve(sock);
      });
      sock.on("error", reject);
    });
  }

  public async call<K extends keyof RpcContractMap>(
    functionName: K,
    data: RpcContractMap[K]["input"],
    options: {
      headers?: Record<string, string>;
      skipMiddlewares?: boolean;
      context?: Record<string, Record<string, unknown>>;
    } = {}
  ): Promise<RpcContractMap[K]["output"]> {
    const sock = await this.connect();
    const id = randomUUID();

    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(id);
        reject(new Error(`RPC ${String(functionName)} timed out`));
      }, 5000);

      this.pending.set(id, { resolve, reject, timer });
      sock.write(JSON.stringify({ id, function: functionName, data, ...options }) + "\n");
    });
  }
}

export const goRpc = new GoRpcClient();
```

---

## ⚡ Step 3: Next.js Server Action Usage

```typescript
// app/actions/user.ts
"use server";

import { goRpc } from "@/lib/go-rpc";
import { auth } from "@/auth"; // e.g. NextAuth or Clerk session

export async function getUserProfile(userId: string) {
  const session = await auth();
  if (!session?.user) {
    throw new Error("Unauthorized");
  }

  // Fully typed: Autocompletes "user.getProfile", enforces userId, returns typed output
  const user = await goRpc.call(
    "user.getProfile",
    { id: userId },
    {
      skipMiddlewares: true,
      context: {
        auth: { // 🔒 Namespaced under Go middleware "auth"
          user: session.user,
        },
      },
    }
  );

  return user;
}
```
