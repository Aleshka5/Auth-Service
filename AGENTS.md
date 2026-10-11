# Auth-Service: Google OAuth2 + JWT cookies + Auth Gateway

Go service for Filenkov hosts. Best practices for Go apply.

## Identity

- Google OAuth2 only. Edge credentials are two httpOnly cookies: `auth_access` and `auth_refresh` (HS256 JWTs). Not `localStorage` / `Authorization` for the SPA.
- Companion httpOnly cookie `auth_session` is an opaque sid for product vaults (WebStorage Redis `private_key:{sid}`). The gateway does **not** authorize on it. Missing `auth_session` must not block proxy.
- Cookie domain `.filenkov.store`, `SameSite=Lax`, `Secure` in production (`AUTH_DEV_HTTP=true` clears Secure and the domain).
- `JWT_SECRET` (≥32 bytes) is shared by **Gateway and Auth API only**. Never send JWTs to User-Service.

## Directory (User-Service)

- **User-Service is the source of truth** for people: UUID `id`, `google_sub`, `email`, `username`, `global_role`, per-service roles.
- Auth has **no Postgres users DB**, no migrations, no `ADMIN_EMAIL`, no Auth-side role columns. Do not reintroduce them.
- Auth API looks up or creates the person over HTTP (`USER_SERVICE_URL`, compose hostname `user_service:8000` on external `user_network`). JWT `sub` is the User-Service UUID.
- Do not reintroduce gRPC `Validate` / `ListUsers`, `WHITELIST_JSON`, or `/api/admin`.

## Roles

User-Service enum (exact strings): `ADMIN` | `FAMILY` | `STRANGER` | `BLOCKED`. Hub identity at the edge is access-JWT `global_role`. Per-service roles live in User-Service under keys such as `storage`, `resume`, and `image` — not Auth table columns. First User-Service user becomes `ADMIN`; later Google logins start as `STRANGER`. `BLOCKED` on a service does not lock the hub.

## Auth Gateway

- **Caddy** terminates TLS (Let’s Encrypt, HTTP→HTTPS only). It reverse-proxies to this gateway. It does not choose `/oauth` vs SPA vs Hermes vs `/users`.
- **Gateway** (`cmd/gateway`) is the public HTTP edge (`:8083`): YAML host/path routing, access-JWT + `rejected_cache`, identity headers, IP rate limits, security headers, JSON access logs.
- Containers run under **Podman**: `make up`, `make down`, `make rebuild`, or `podman compose --env-file .env -f deploy/docker-compose.yml`. Do not switch these back to Docker.
- **API** (`cmd/api`) stays **internal** (`:8000`, unpublished): OAuth, `/api/me`, `/api/refresh`, `/api/logout`, `/api/chat`, `/health`. The Vite SPA stays a separate nginx container (internal upstream). Do not merge the SPA into a Go binary. Do not publish API or web ports.

Routing is YAML (`deploy/gateway.yaml`). Live hosts: `filenkov.store`, `storage.filenkov.store`, `messanger.filenkov.store`, `image.filenkov.store`, and `llm.filenkov.store`. Adding a host is a config change, not a code change. First matching route wins — put specific prefixes first. See `docs/auth-gateway.md`.

Edge policy: no valid access JWT, or `global_role == BLOCKED` / user in `rejected_cache`, is not proxied (except `auth: public`). Before that deny, a `session` or `admin` route with a missing or expired access cookie and a present `auth_refresh` is silently refreshed: the gateway `POST`s the Auth API `/api/refresh` (it still does not mint JWTs), single-flight per refresh token, writes the new cookies, and continues the same request. That is what keeps `llm.filenkov.store` and the other product UIs signed in after the access TTL; they never call `/api/refresh` themselves. Refresh 401 falls through to the anonymous deny. Refresh blocked is `403`. Refresh transport/API failure is `503`. Browser navigations with no usable refresh redirect to `{PUBLIC_ORIGIN}/auth?return_to=…`; `/api/*`, `/users/*`, exact `/users`, and typical XHR get `401` JSON; BLOCKED with a token gets `403`. On the hub, every path not claimed by a specific route is public and reaches the SPA — the resume tree at any depth, `/auth`, and SPA assets (`/architecture/*` is now only a client-side legacy redirect into the tree, not a route). Also public: `/oauth/*`, `/api/refresh`, `/api/logout`, `POST /api/chat` (the owl chat — it greets anonymous readers), and `/health`. `/admin` and `GET /users` are admin-only (`global_role == ADMIN`). `/hermes/*` is session (any signed-in user). `/api/me` requires an access JWT. `POST /users` is rejected.

On proxied requests the gateway strips incoming `X-Auth-*`, `X-User-Id`, and `X-Storage-Role`.

- **Non–User-Service** upstreams (`api`, `web`, `hermes`, …): inject `X-Auth-User-Id`, `X-Auth-Email`, `X-Auth-Name` (username), `X-Auth-Role` (access JWT `global_role`). Product services sit **behind the gateway** and trust those headers.
- **Hermes only:** set `X-Forwarded-Prefix: /hermes` and `Authorization: Bearer <access JWT>` (`inject_access_jwt: true`). Hermes `auth-gateway` plugin verifies that JWT (`HERMES_DASHBOARD_AUTH_JWT_SECRET` = `JWT_SECRET`). No second login (`GET /hermes/login` → `/hermes/sessions`). Chat is a PTY WebSocket through the same proxy — [docs/hermes.md](docs/hermes.md). Hermes joins `auth_network`; upstream is `http://hermes:8787`. Do not publish Hermes `:8787` on a public interface. User-Service still never receives a JWT.
- **Storage host** (`role_field: storage`): also inject `X-User-Id` (JWT `sub`) and `X-Storage-Role` from User-Service `GET /users/{id}/roles/storage` (not `global_role`). Forward `Cookie` so `auth_session` reaches WebStorage. User-Service 5xx/timeout → 503, no proxy; user 404 → 401.
- **Messenger host** (`messanger.filenkov.store`): proxy to `http://messanger:8000` (HomeMessanger joins `auth_network` as compose service `messanger`). Catch-all `auth: session` plus public `/health`; `role_field: global_role`, 5m proxy timeout. `X-Auth-*` only — no User-Service call, no `X-User-Id` / per-service role, no JWT injection. Not linked from the hub header. Spec: [docs/user-story-messanger-host.md](docs/user-story-messanger-host.md).
- **Image host** (`image.filenkov.store`, `role_field: image`): proxy to `http://qwen-image:8000` (Qwen Image) on external `qwen_image_gen_network`. Public `/healthz` (the app's own — an exact `match: /healthz` route overrides the gateway's local `/healthz` on that host only), catch-all `auth: session`, 30m proxy timeout. On non-public routes the gateway calls User-Service `GET /users/{id}/roles/image`: `FAMILY`/`ADMIN` → proxy; `STRANGER` (no row), `BLOCKED`, or anything else → 403; user 404 → 401; 5xx/timeout → 503. Hub `global_role` does not grant it. `X-Auth-*` only — no `X-User-Id`, no per-service role header, no JWT. Do not publish `qwen-image:8000`. Spec: [docs/user-story-image-host.md](docs/user-story-image-host.md).
- **LLM host** (`llm.filenkov.store`): proxy to `http://qwen38:4001` (llama.cpp server, container name `qwen38`, not in this compose, already on external `llm_network`). Catch-all `auth: admin` (hub `global_role == ADMIN`). Inject `X-Auth-*` (hub `global_role`). No `inject_access_jwt`. Do not send a JWT to User-Service, and do not reach `qwen38` via a published host port. The hub header links this host as **Chat**, admin-only (no lock). Owl chat still uses LiteLLM internally via `LLM_HOST` (`litellm-proxy-server:4000`); this public host is not that proxy.
- **Owl chat** (`POST /api/chat`, `auth: public`): the Auth API streams one answer at a time from the LiteLLM proxy. `internal/llm` posts to `{LLM_HOST}/chat/completions` with `Authorization: Bearer {LLM_API_KEY}` and `stream: true`, and `internal/http/chat.go` re-emits the deltas as SSE. Defaults: `LLM_HOST=http://litellm-proxy-server:4000/v1`, `LLM_API_KEY=dummy`; `LLM_MODEL` must name a model that proxy actually serves. The `api` container therefore joins `llm_network` too. No history is read, kept, or sent — one question per request, 255 characters max. Reasoning is off: the request carries `reasoning_effort` (`LLM_REASONING_EFFORT`, default `none`; set it empty if the proxy rejects the field), the delta's `reasoning_content` is never mapped, and `internal/llm/reasoning.go` strips inline `<think>…</think>` from the streamed answer, tags straddling deltas included. The system prompt is `internal/llm/prompt.md` (hand-written) plus `internal/llm/nodes.json`, generated from the node folders by `npm run gen:owl-context` in `web/` — regenerate and commit it when you add or rename a node. Requests carry the chat session id as `user`, plus `metadata` with that session id and, for a signed-in visitor, the gateway-injected `X-Auth-User-Id` / `X-Auth-Email`. A missing or broken LLM config disables the route (503); it never blocks sign-in.
- **Owl chat tracing** (`internal/tracing`, optional): one span per question, shipped to MLflow over **OTLP/HTTP** — `POST {MLFLOW_TRACING_ENDPOINT}/v1/traces` with the experiment in the `x-mlflow-experiment-id` header (MLflow ≥ 3.6, SQL backend). Deliberately independent of generation: spans are batched on a background goroutine, so a slow or dead MLflow delays no answer and fails no request, and it does **not** rely on MLflow gateway's own tracing. Unset `MLFLOW_TRACING_ENDPOINT` makes the tracer a no-op. Each span carries the question and the assembled answer under both `mlflow.spanInputs`/`mlflow.spanOutputs`/`mlflow.spanType` and the `gen_ai.*` semantic conventions, plus the same identity already sent to the proxy — session id for everyone, `user.id`/`user.email` only for a signed-in reader. A question rejected by validation never produces a span.
- **User-Service** upstream: do **not** forward `Cookie`, `Authorization`, or JWT query params. Set `X-User-Id` from the access JWT `sub`. User-Service never sees a JWT.

`return_to` allowlist is the set of hosts in the gateway YAML (https in production). Do not add `WHITELIST_JSON`.

Do not reintroduce Auth Postgres lookup in the gateway, or gRPC as the product contract for new services. Redis `session:{sid}` exists only as the companion `auth_session` map (UUID, refresh TTL) — not as the edge credential.

## Rate limit

IP rate limit at the **gateway**. Keep API limits as defense in depth.

## Logging

JSON-capable (`slog` JSON) including gateway access logs, for Loki later.

## Redis

`redis:7-alpine`: `rejected_cache` (`rejected:jti:{jti}`, `rejected:user:{uuid}`), OAuth CSRF state, IP rate-limit counters, and companion `session:{sid}` → User-Service UUID (plus `session:user:{uuid}` reverse index) for `auth_session`. **Not** an edge session store.

## Frontend (Vite, production static)

1. Auth page (`filenkov.store/auth`) — Google OAuth2.
2. Hub (`filenkov.store`) — public **resume tree**, not one scrolling page. Every path not claimed by `/auth`, `/admin`, or the legacy `/architecture/:slug` redirect is a tree node served by the SPA catch-all (`path="*"` → `NodeRoute`). Unknown paths and nodes the reader may not see redirect to `/`. Design doc + what shipped: [docs/resume-tree-frontend.md](docs/resume-tree-frontend.md).
   - A node **is a folder** under `web/src/resume/nodes/`, and the folder path is the URL path. `web/src/resume/registry.ts` discovers them with `import.meta.glob` — there is **no central list of nodes**. 30 nodes today (root + 29). To add one: add a folder with `meta.ts` + `node.tsx`. Do not add a route, a data file, or a registry entry.
   - `web/src/resume/registry.test.ts` enforces 8 structural invariants (meta/body pairing, parent exists, kebab-case segments, no reserved depth-1 segment, unique sibling `order`, depth ≤ 4, teaser ≤ 120 chars, root at path `''`). A typo fails the build instead of shipping a missing page.
   - `meta.ts` is eager, so the spine, branch cards, title and meta line paint with no spinner. `node.tsx` is lazy — one Rollup chunk per node, named after the URL path.
   - `/about-project/architecture/<slug>` is a **dynamic child**: 10 solution pages served by one node body with no folder each, declared via `meta.dynamicChild` in `web/src/resume/nodes/about-project/architecture/meta.ts`.
   - Visibility is declarative: `visibleWhen: 'signed-in' | 'admin'` in `meta.ts`. `achievements/projects/hermes` is the only `signed-in` node today.
   - Header (`web/src/components/Layout.tsx`): three scroll states written straight onto `<html data-header>` by `web/src/hooks/useScrollChrome.ts` — no React re-render on scroll. Sign in vs **username**. Service links: messanger.filenkov.store, storage.filenkov.store, `filenkov.store/hermes/chat` (shown to everyone, with a lock for guests), image.filenkov.store (only when `serviceRole(user, 'image')` is `FAMILY` or `ADMIN`; no lock), **Chat** → `https://llm.filenkov.store` (`global_role == ADMIN` only, no lock), and `/admin` (`global_role == ADMIN` only).
   - The owl is no longer in the header: `web/src/components/owl/Owl.tsx` is a hero owl in the flow that cross-fades into a fixed "Back to top" button at 180 px of scroll.
   - **The owl chat.** The hero owl is a button that opens `OwlChat`; the docked owl stays "Back to top". Landing on `/` is the greeting: `OwlBubble` types the text out letter by letter beside the owl (`greeting.ts` holds it), every arrival — no session gate. It is read once at mount, so walking into the tree and back does not replay it; a reload does. Landing on a node page does not greet. **The panel never opens on its own** — only the reader's click on the owl opens it, on any page, and that click also retires the greeting and focuses the field. The panel is `React.lazy`, so `OwlChat` + `web/src/chat.ts` stay out of the initial chunk. One question, one answer, nothing stored: a new question replaces the last, the request body is `{question}` and nothing else, and the input is capped at 255 characters on both sides. `chat.ts` streams SSE from `POST /api/chat` and sends `X-Chat-Session`, a per-browser-session id used only for grouping the proxy's logs — never for authorization. Page paths in the answer become links into the tree: `nodeRefs.tsx` resolves every path-shaped token through `registry.resolve`, so an invented path stays plain text and a node the reader may not see is never linked to them.
   - Styles: `web/src/styles/{fonts,tokens,base,shell,article,tree,forms,chat}.css`, imported in that order by `main.tsx`. `web/src/index.css` is gone. Fonts are self-hosted from `web/public/fonts/` — no `fonts.googleapis.com` request.
   - `web/vite.config.ts` generates `sitemap.xml` + `robots.txt` from the node folders and fails the build if initial JS exceeds 95 KB gzipped (88.5 KB today).
   - `web/nginx.conf` `try_files $uri $uri/ /index.html` is unchanged and is why every deep node URL works. `deploy/gateway.yaml` needed **no** change — the trailing `- match: /*  upstream: web  auth: public` already serves every node path.
3. Admin page (`filenkov.store/admin`) — hub `global_role == ADMIN` only. Directory table from User-Service `GET /users` (full list, email search in the SPA), then `PUT /users/{id}/role` and `PUT /users/{id}/roles/{service}` (fixed columns `storage`, `resume`, `image`; missing row shows `STRANGER`). Gateway exposes `GET /users` as `auth: admin`; `POST /users` stays rejected. Do not call `/api/admin`. Shipped behavior: `docs/user-story-admin-directory.md`.

SPA: `credentials: 'include'`; on `401` from `/api/me` (and other same-origin calls), `POST /api/refresh` then retry. Display `username` / `email` / `global_role` / `services[]`.

Hub and storage roles in the UI: `ADMIN` | `FAMILY` | `STRANGER` | `BLOCKED`.
