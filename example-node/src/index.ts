import { goRpc, RpcError } from "./client.js";

async function main() {
  console.log("Calling the example daemon; use matching generated contracts.");
  try {
    const hello = await goRpc.call("public.hello", { name: "Node.js Developer" });
    console.log(hello.greeting);
    const token = process.env.GO_RPC_TOKEN;
    if (token) {
      // The Go application's middleware verifies this credential. No caller
      // session/context is trusted and no authentication check is skipped.
      const profile = await goRpc.call("user.getProfile", { id: "user-1" }, {
        headers: { authorization: `Bearer ${token}` },
      });
      console.log(profile);
    } else {
      console.log("Set GO_RPC_TOKEN to exercise the application's protected route.");
    }
  } catch (error) {
    if (error instanceof RpcError) console.error(error.code, error.message);
    else console.error(error);
    process.exitCode = 1;
  } finally {
    goRpc.close();
  }
}
void main();
