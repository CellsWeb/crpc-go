# Changelog

## v1.0.1 — released

Public module and CLI v1.0.1 include the following changes. The historical
v1.0.0 tag is unchanged and predates these fixes.

- Reject duplicate multipart routing headers before middleware/handler dispatch.
- Preserve arbitrary JSON shapes in in-memory calls, Unix IPC and TestCaller.
  HTTP preserves explicit null; omitted HTTP data still defaults to `{}`.
- Bound IPC frames at 2,097,152 bytes. Newline counts toward the JSON-Lines limit;
  the four-byte prefix is separate from the length-framed payload maximum.
- Node IPC example rejects pending calls promptly on connection close, anonymous
  connection errors and malformed response JSON; cleanup/reconnection never
  automatically replays a failed call.
- Generated documentation honors exported DOCS_ENABLED/DOCS_PATH. Standalone
  projects no longer require a parent SDK checkout. Installed CLIs select their
  module version; source/VCS builds still retain the older v1.0.0 fallback.
  Use an explicit workspace for deliberate local SDK development.
- Remove documentation badges, retain ignored deprecated IsPublic metadata,
  and publish structural middleware schemas without configured secret values.
- CLI display version1.0.1 matches the public release. Versioned installs
  select SDK v1.0.1; the source-build fallback mechanism is unchanged.

## Migration

Use Go >=1.27.4. Authenticate in middleware for HTTP, in-process calls and IPC;
remote `skipMiddlewares`/context fields do not establish trusted identity.
Omit HTTP data or send `{}` when an empty object is intended. Nullable schemas
must explicitly allow null. Application errors retain HTTP 200 by default;
standard HTTP mapping remains opt-in.

Keep machine-specific Go workspaces local. Install the publicv1.0.1
CLI/module version for the hardened standalone dependency.
Preserve old tags and verify the public module proxy before claiming a successful
released installation. See the Node repository's release plan for coordinated
package order and approval gates.
