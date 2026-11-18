# Release and migration

Version **v1.0.2** adds the stdio frame-limit correction to the earlier hardening/documentation fixes.
The historical v1.0.0 tag predates these changes and remains unchanged. Use
Go>=1.27.4 and install the exact versioned CLI/module as described in
[installation](getting-started.md).

Installed versioned CLIs generate matching SDK requirements. Source/VCS builds
still use the older v1.0.0 fallback; a display version is not a dependency pin.
Use an explicit workspace only to select a local SDK checkout deliberately.
Preserve the migration considerations below when updating an existing app.

Explicit HTTP data:null reaches schema validation/handlers as nil; omission still
defaults to {}. Arrays/scalars stay intact. CallInMemory/CallRaw/Unix IPC accept
the same JSON shapes subject to native schemas. These are not new output-null,
large-integer or JSON Schema keyword guarantees. HTTP 200 application errors stay
the default; standard status mapping remains opt-in.

IsPublic is retained as a deprecated ignored bool. Remove it from new metadata;
there are no badges or replacement flags. Docs availability is global and never
changes authentication. Middleware owns verified state for HTTP/in-memory/IPC;
legacy remote skip/context fields are ignored, not security features.

Unix frames are bounded 2,097,152 bytes. Node client socket-close/anonymous errors
and malformed response JSON reject pending callers promptly, clean resources and
retire the affected connection. Later calls can reconnect without replaying work.

Generated servers honor exported DOCS_ENABLED/DOCS_PATH but do not load .env. Keep
local workspaces out of portable production setup. The Node releases are
core 1.0.35/client 1.0.26/scaffolder 1.0.31 (documentation-only patches;
runtime unchanged from core1.0.34/client1.0.25/scaffolder1.0.30), declare Node>=24.0.0 and preserve
Bun support. Its union fix restores strict nullable/file validation; native schema
vocabularies still differ. Review both changelogs before deployment.
