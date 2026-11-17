# Candidate changelog

## v1.0.1 — prepared, not published

The existing public `v1.0.0` tag predates the verified hardening patches. This
candidate must be independently verified and released under a **new** tag;
do not move or describe v1.0.0 as the hardened release.

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
  module version; source/VCS builds retain the real older v1.0.0 fallback until
  a matching release is available. Use an explicit workspace for this candidate.
- Remove documentation badges, retain ignored deprecated IsPublic metadata,
  and publish structural middleware schemas without configured secret values.
- Prepare the CLI display version 1.0.1; this source version does not prove the
  tag/module is public. The standalone fallback mechanism is unchanged.

## Migration

Use Go >=1.27.4. Authenticate in middleware for HTTP, in-process calls and IPC;
remote `skipMiddlewares`/context fields do not establish trusted identity.
Omit HTTP data or send `{}` when an empty object is intended. Nullable schemas
must explicitly allow null. Application errors retain HTTP 200 by default;
standard HTTP mapping remains opt-in.

Keep machine-specific Go workspaces local. After v1.0.1 is actually published,
install that exact CLI/module version for the hardened standalone dependency.
Preserve old tags and verify the public module proxy before claiming a successful
released installation. See the Node repository's release plan for coordinated
package order and approval gates.
