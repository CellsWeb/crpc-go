# Candidate release and migration

This branch prepares v1.0.1. Public v1.0.0 predates the verified hardening and docs
changes. Do not move that tag or describe it as this candidate. Go >=1.27.4 is
required. Source display version 1.0.1 is not proof of a published module.

Installed versioned CLIs generate matching SDK requirements. Source/VCS builds
keep the existing older v1.0.0 fallback because a local revision may not be public.
Select this candidate with an explicit Go workspace. Once v1.0.1 is actually
published, verify a fresh GOWORK=off installation/build against that exact version.
See [installation](getting-started.md) and the sibling Node RELEASE.md plan.

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
local workspaces out of portable production setup. The Node candidate separately
prepares core 1.0.34/client 1.0.24/scaffolder 1.0.30, declares Node >=24.0.0 and preserves
Bun support. Its union fix restores strict nullable/file validation; native schema
vocabularies still differ. Review both changelogs before deployment.
