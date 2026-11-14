import { goRpc, RpcError } from "./client.js";

async function main() {
  console.log("🚀 Testing Go RPC IPC from Node.js...\n");

  try {
    // -------------------------------------------------------------------------
    // 1. Calling a Public Function (Zero Middlewares)
    // -------------------------------------------------------------------------
    console.log("👉 1. Calling public.hello...");
    const helloRes = await goRpc.call("public.hello", {
      name: "Node.js Developer",
    });
    console.log("   Result:", helloRes.greeting);

    // -------------------------------------------------------------------------
    // 2. Calling with Headers through Go Middleware
    // -------------------------------------------------------------------------
    console.log("\n👉 2. Calling user.getProfile through Go 'auth' middleware...");
    const profileRes = await goRpc.call(
      "user.getProfile",
      { id: "usr_from_node_headers" },
      {
        headers: {
          authorization: "Bearer valid_token_example",
        },
      }
    );
    console.log("   Result:", profileRes);

    // -------------------------------------------------------------------------
    // 3. Calling with skipMiddlewares + Injected Context (e.g. Next.js Server Action)
    // -------------------------------------------------------------------------
    console.log("\n👉 3. Calling user.getProfile with skipMiddlewares & injected context...");
    console.log("   (Context is namespaced by middleware name 'auth' -> user)");
    const actionRes = await goRpc.call(
      "user.getProfile",
      { id: "" }, // empty ID so handler falls back to authenticated context user.id
      {
        skipMiddlewares: true,
        context: {
          auth: {
            user: {
              id: "usr_injected_from_nextjs_session",
              email: "minhaz@cellsweb.com",
              role: "superadmin",
            },
          },
        },
      }
    );
    console.log("   Result (retrieved user injected by Node session):", actionRes);

    // -------------------------------------------------------------------------
    // 4. Testing Error Handling (Missing Token on Protected Function)
    // -------------------------------------------------------------------------
    console.log("\n👉 4. Testing Error Handling: Calling protected function without token...");
    try {
      await goRpc.call("user.getProfile", { id: "usr_fail" });
      console.log("   ❌ Expected error but succeeded!");
    } catch (err) {
      if (err instanceof RpcError) {
        console.log(`   ✓ Caught expected RpcError: code=${err.code}, message="${err.message}"`);
      } else {
        console.log("   Unexpected error:", err);
      }
    }

    console.log("\n✅ All IPC Node.js tests completed successfully!");
  } catch (err: any) {
    console.error("\n❌ Test execution failed:", err.message);
  } finally {
    goRpc.close();
  }
}

main();
