# CellsWeb RPC Go — Node.js & Next.js IPC Integration

This directory demonstrates how to seamlessly connect a **Node.js**, **Express**, **Fastify**, or **Next.js** application to a compiled **CellsWeb RPC Go** backend over high-speed **Unix Domain Socket IPC** (`/tmp/crpc.sock`).

---

## ⚡ Why Unix Domain Socket IPC?

| Feature | HTTP over `localhost:4001` | Unix Domain Socket IPC (`/tmp/crpc.sock`) |
| :--- | :--- | :--- |
| **Transport** | TCP Loopback (OS Network Stack) | Kernel Memory (Direct socket buffer) |
| **Latency** | 200µs – 500µs | **20µs – 50µs** (10x faster) |
| **Port Clashes** | Can collide with other services | Zero ports; isolated filesystem node |
| **Network Security** | Must firewall `localhost` | Protected by OS file permissions (`0o600`) |
| **Session Injection** | Cannot bypass DB middleware | `skipMiddlewares: true` + typed `context` injection |

---

## 🔑 Crucial Architectural Rule: Context Namespacing

When calling Go RPC functions from Node.js with `skipMiddlewares: true` (e.g., inside Next.js Server Actions where the user is already authenticated by NextAuth / Supabase / Clerk), **dependency injection context MUST be namespaced by the middleware's name**:

```json
{
  "context": {
    "auth": {
      "user": {
        "id": "usr_99",
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

## 🚀 Quickstart

### 1. Start the Go RPC Server

Make sure MongoDB is running, then start the Go server (which opens `/tmp/crpc.sock`):

```bash
cd ../example-mongo
go run ./cmd/server
```

You will see:
```text
🔌 Node.js IPC Socket listening at /tmp/crpc.sock
⚡ CellsWeb RPC Go Engine running at http://0.0.0.0:4001/rpc
📖 Interactive Docs available at http://0.0.0.0:4001/docs
```

### 2. Generate TypeScript Types (`crpc.d.ts`)

In the Go project, run the codegen script:

```bash
cd ../example-mongo
go run ./cmd/codegen ../example-node/crpc.d.ts
```

This inspects all registered Go RPC functions and auto-generates exact TypeScript types into `example-node/crpc.d.ts`.

### 3. Run the Node.js Client

```bash
cd ../example-node
npm install
npm start
```

---

## 💻 Code Examples

### A. Next.js Server Action (`app/actions/user.ts`)

```typescript
"use server";

import { goRpc } from "@/lib/go-rpc";
import { auth } from "@/auth"; // Your NextAuth / Clerk session

export async function getUserProfile(userId: string) {
  const session = await auth();
  if (!session?.user) {
    throw new Error("Unauthorized");
  }

  // Next.js already authenticated the user.
  // We skip redundant Go token verification and inject the session into context!
  const profile = await goRpc.call(
    "user.getProfile",
    { id: userId },
    {
      skipMiddlewares: true,
      context: {
        auth: { // Namespaced under Go middleware name "auth"
          user: session.user,
        },
      },
    }
  );

  return profile;
}
```

### B. Standard Service Call with Headers (Running Go Middlewares)

```typescript
import { goRpc } from "./client.js";

const result = await goRpc.call(
  "user.getProfile",
  { id: "usr_101" },
  {
    headers: {
      authorization: "Bearer eyJhbGciOi...",
    },
  }
);
```

### C. Error Handling

```typescript
import { goRpc, RpcError } from "./client.js";

try {
  await goRpc.call("user.create", { email: "invalid", name: "" });
} catch (err) {
  if (err instanceof RpcError) {
    console.error(`RPC Error [${err.code}]:`, err.message, err.details);
  }
}
```

---

## 🛠️ Regenerating Types on Schema Changes

Whenever you add or modify a Go function in `functions/`:

```bash
cd example-mongo
go run ./cmd/codegen ../example-node/crpc.d.ts
```

TypeScript will immediately highlight any breaking parameter changes in your Node.js/Next.js codebase!
