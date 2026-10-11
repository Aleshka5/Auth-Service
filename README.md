# Auth-Service

Google OAuth2 hub for `filenkov.store`: httpOnly **access + refresh JWTs** on `.filenkov.store`, a companion opaque **`auth_session`** cookie for product vaults, User-Service as the users directory, and an **Auth Gateway** that routes hosts/paths and injects identity headers.

The browser never sees a token in JS or the URL. **Caddy terminates TLS only.** The gateway is the public HTTP edge: routing, JWT auth, `X-Auth-*` / `X-User-Id` / `X-Storage-Role`, IP rate limits, security headers, and JSON access logs. Product backends sit behind the gateway and trust those headers. **User-Service never receives a JWT.**

Auth has **no Postgres users DB and no gRPC**. Those were removed: edge sessions are not Redis cookies, `/api/admin` is gone, and new services must not call a Validate RPC. `auth_session` is **not** the login credential — it is a stable sid for WebStorage’s private vault.

## Layout

```
cmd/api/          Internal auth API (OAuth, /api/me, /api/refresh, /api/logout, /health)
cmd/gateway/      Public HTTP edge after Caddy (:8083)
internal/         config, http, gateway, oauth, token, rejected, usersvc, ratelimit, log
web/              Vite SPA — resume tree + /auth + /admin (static nginx image, internal upstream)
deploy/           docker-compose.yml, Dockerfiles, gateway.yaml
docs/             auth-gateway.md, user-story-*.md (directory contract, storage host, admin directory list, messenger host CR, image host CR — shipped)
```

## Quick start (Podman Compose)

Start the **User-Service** stack first so external network `user_network` and hostname `user_service` exist. Start the **LiteLLM** stack so `llm_network` and hostname `litellm-proxy-server` exist (owl chat), and have the one-off llama.cpp container `qwen38` already on that network. Start the **image** stack so `qwen_image_gen_network` and hostname `qwen-image` exist. Auth compose **attaches** those networks; it does not create User-Service, LiteLLM, `qwen38`, or the image service.

1. Copy env and fill Google OAuth + `JWT_SECRET` (≥32 random bytes):

   ```bash
   cp .env.example .env
   ```

2. In [Google Cloud Console](https://console.cloud.google.com/apis/credentials) create an OAuth client (Web application) with authorized redirect URIs:

   - Production: `https://filenkov.store/oauth/google/callback`
   - Local host Caddy on port 80: `http://localhost/oauth/google/callback`

3. For local HTTP, in `.env` set:

   ```
   AUTH_DEV_HTTP=true
   COOKIE_DOMAIN=
   PUBLIC_ORIGIN=http://localhost
   GOOGLE_REDIRECT_URL=http://localhost/oauth/google/callback
   ```

   Add `localhost` to `aliases` under `filenkov.store` in `deploy/gateway.yaml` if you browse via `http://localhost`.

4. Run from the repo root:

   ```bash
   podman compose --env-file .env -f deploy/docker-compose.yml up --build
   # or: make rebuild
   ```

   Networks: `auth_network` (created here) + `user_network` (`external: true`) + `llm_network` (`external: true`) + `qwen_image_gen_network` (`external: true`, gateway only). Services: `redis` (`redis:7-alpine`), `web`, `api`, `gateway`.

   Compose publishes **only** the gateway:

   | Process | Host port | Notes |
   |---------|-----------|--------|
   | **gateway** | `127.0.0.1:8083` | Point production Caddy here. YAML listen `:8083`. |
   | api | unpublished | Container `HTTP_ADDR=:8000` |
   | web (SPA) | unpublished | Container `:80` |
   | redis | unpublished | `rejected_cache` + OAuth state + rate limits + companion `session:{sid}` |

   Gateway process health: `GET http://127.0.0.1:8083/healthz` (not the proxied `/health`). API healthcheck: `GET http://127.0.0.1:8000/health` inside the container (Redis ping).

If `user_service` is down, Google callback, `/api/me`, and storage-host role lookups fail. If `qwen38` is down, `llm.filenkov.store` returns a gateway proxy error for a hub `ADMIN`. If `litellm-proxy-server` is down, the hub owl chat fails and sign-in still works. If `messanger` is down, `messanger.filenkov.store` returns a gateway proxy error on that host only. If `qwen-image` is down, `image.filenkov.store` returns a gateway proxy error on that host only; the image **network** must still exist for the gateway container to start (`podman network create qwen_image_gen_network` if the image stack is not up yet).

Operator order: User-Service stack, LiteLLM stack (so `llm_network` and `litellm-proxy-server` exist for owl chat) with `qwen38` already on that network, image stack (so `qwen_image_gen_network` and `qwen-image` exist), Auth stack, Common (WebStorage data volumes), then WebStorage (`app` joins `auth_network` as external so `http://app:8000` resolves) and HomeMessanger (`messanger` joins the same network so `http://messanger:8000` resolves).

## Host Caddy (TLS only)

Caddy keeps HTTP→HTTPS and Let’s Encrypt. It does **not** route `/oauth`, `/api`, SPA, `/users`, or `/hermes` — one reverse proxy to the gateway.

Do **not** add a second `filenkov.store { }` site. Replace existing per-path `handle` blocks with:

```caddy
filenkov.store {
	encode gzip
	reverse_proxy 127.0.0.1:8083
}

storage.filenkov.store {
	encode gzip
	reverse_proxy 127.0.0.1:8083
}

messanger.filenkov.store {
	encode gzip
	reverse_proxy 127.0.0.1:8083
}

llm.filenkov.store {
	encode gzip
	reverse_proxy 127.0.0.1:8083
}

image.filenkov.store {
	reverse_proxy 127.0.0.1:8083
}
```

The production site blocks (TLS 1.3, security headers, HTTP→HTTPS redirects) are in `deploy/Caddyfile.filenkov`; the image site has no `encode gzip`.

If the hub site block already exists, delete `handle /oauth/*`, `handle /api/*`, `handle /health`, `handle /hermes/*`, and the SPA catch-all, then use the single `reverse_proxy` above.

Reload with `sudo systemctl reload caddy` (or `caddy reload --config /etc/caddy/Caddyfile`).

API (`:8000`) and web are **not** published on the host. There is no Caddy rollback to `:8081` / `:8082`. The storage, messenger, and LLM Caddy sites are operator config (not in this repo). WebStorage, HomeMessanger, Qwen Image, and LiteLLM stay unpublished behind the gateway. `llm.filenkov.store` reaches `qwen38` on `llm_network`; the gateway does not use a host-published port for that container.

## Local Go (without Compose for the API)

Run Redis (or the compose `redis` service) and a reachable User-Service, then:

```bash
export $(grep -v '^#' .env | xargs)
export REDIS_URL=redis://127.0.0.1:6379/0
export USER_SERVICE_URL=http://user_service:8000  # or a host URL if you port-forward User-Service; not Auth API
go run ./cmd/api
```

Do **not** set `DATABASE_URL`. Gateway (same Redis + `JWT_SECRET`; User-Service client only for storage-role lookups):

```bash
export GATEWAY_CONFIG=deploy/gateway.yaml
export GATEWAY_HTTP_ADDR=:8083
go run ./cmd/gateway
```

`web/vite.config.ts` proxies `/api`, `/oauth`, and `/users` to `http://127.0.0.1:8083` during `npm run dev`. Run the gateway on `:8083` (API stays on unpublished `:8000` behind it), or browse through the gateway directly.

## Tests and builds

```bash
go test ./...
go build -o auth-service ./cmd/api
go build -o auth-gateway ./cmd/gateway
```

Frontend:

```bash
cd web && npm test
```

## HTTP API (Auth API, unpublished `:8000`)

JSON, same-origin behind the gateway (no CORS). Cookies: `auth_access`, `auth_refresh`, and companion `auth_session`, `HttpOnly`, `SameSite=Lax`, `Secure` unless `AUTH_DEV_HTTP=true`, `Domain` from `COOKIE_DOMAIN` (default `.filenkov.store`).

| Method | Path | Gateway auth | Notes |
|--------|------|----------------|--------|
| `GET` | `/oauth/google?return_to=` | public | CSRF state in Redis; `return_to` must be `https` and a host listed in `deploy/gateway.yaml` (plus `PUBLIC_ORIGIN`). With `AUTH_DEV_HTTP=true`, `http://localhost` and `http://127.0.0.1` are also allowed. |
| `GET` | `/oauth/google/callback` | public | User-Service GET-by-`google_sub` or POST `/users`; if not `BLOCKED`, set JWT cookies **and** `auth_session`; 302 to `return_to` |
| `POST` | `/api/refresh` | public | Reads `auth_refresh`; rotates refresh; keeps existing `auth_session` (extends Redis TTL) or mints one; `200` `{ "status": "ok" }` |
| `POST` | `/api/logout` | public | Rejects jtis + user in Redis; deletes `session:{sid}`; clears JWT cookies and `auth_session`; `200` `{ "status": "ok" }` |
| `GET` | `/api/me` | session | Access JWT required. Auth API re-validates, then calls User-Service. `401` if anonymous, `403` if `global_role=BLOCKED` |
| `GET` | `/health` | public | Redis ping (no Postgres). Proxied to the API |
| `GET` | `/healthz` | — | Gateway process liveness (not proxied; not a `filenkov.store` route) |

There is **no** `/api/admin`. Directory reads/writes go through the gateway to User-Service (`GET`/`PUT /users/…`).

`GET /api/me` body (Auth adapts User-Service JSON):

```json
{
  "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "google_sub": "110000000000000000001",
  "email": "ada@example.com",
  "username": "ada",
  "global_role": "ADMIN",
  "services": [
    { "service": "storage", "role": "FAMILY" },
    { "service": "resume", "role": "STRANGER" }
  ]
}
```

Signup is open to any Google account. Auth does not upsert into its own DB: it looks up `google_sub` in User-Service and `POST /users` on first login (derived `username`, not `google_sub`). First directory user is User-Service bootstrap `ADMIN`. Last-admin demotion is a User-Service `409 LAST_ADMIN`.

SPA: if `GET /api/me` returns `401`, `POST /api/refresh` then retry. Token strings stay in httpOnly cookies.

IP rate limits: gateway (default 180/min, YAML) plus API defense in depth (OAuth ~10/min, `/api/*` ~60/min, Redis).

## JWT cookies and Redis `rejected_cache`

| Cookie | Name | TTL env | Default |
|--------|------|---------|---------|
| Access | `auth_access` | `JWT_ACCESS_TTL` | `15m` |
| Refresh | `auth_refresh` | `JWT_REFRESH_TTL` | `168h` |
| Companion sid | `auth_session` | `JWT_REFRESH_TTL` | `168h` |

Alg **HS256**. Access claims: `sub` (User-Service UUID), `jti`, `typ=access`, `global_role`, `email`, `username`, `iss`=`PUBLIC_ORIGIN`, `aud`=YAML hosts (hub + `storage.filenkov.store` + `messanger.filenkov.store` + `image.filenkov.store` + `llm.filenkov.store`, plus loopback when `AUTH_DEV_HTTP`). Refresh claims: `sub`, `jti`, `typ=refresh`, `iss`/`aud`/`exp` only (no roles). Rotation re-reads User-Service.

`auth_session` is a random 32-byte hex sid, **not** the access JWT and **not** the user UUID. Redis `session:{sid}` → User-Service UUID. Stable for the login lifetime (not rotated on access refresh). Edge auth ignores it.

Redis keys:

- `rejected:jti:{jti}` — logout and refresh rotation
- `rejected:user:{uuid}` — logout and successful admin role PUTs (gateway writes this for the **target** user after `200` on `PUT /users/{id}/role` or `PUT /users/{id}/roles/{service}`)
- `session:{sid}` / `session:user:{uuid}` — companion vault sid (cleared on logout and user revocation)

OAuth CSRF remains `oauth:state:{state}`.

## Identity headers (gateway → upstream)

The gateway strips incoming `X-Auth-*`, `X-User-Id`, and `X-Storage-Role` so clients cannot spoof identity.

**Non–User-Service** upstreams, when an access JWT is valid:

| Header | Value |
|--------|--------|
| `X-Auth-User-Id` | JWT `sub` (User-Service UUID) |
| `X-Auth-Email` | JWT `email` |
| `X-Auth-Name` | JWT `username` (User-Service has no display-name field) |
| `X-Auth-Role` | JWT `global_role` (hub YAML `role_field: global_role`) |

**Hermes** (`upstream: hermes`) receives `X-Forwarded-Prefix` and, with `inject_access_jwt: true`, the hub access cookie as `Authorization: Bearer` so `hermes-auth-gateway` can verify it. No second login. Chat is a PTY WebSocket — [docs/hermes.md](docs/hermes.md). Other product upstreams do not get the access JWT.

**Storage host** (`storage.filenkov.store`, `role_field: storage`) also sets `X-User-Id` (same UUID) and `X-Storage-Role` from User-Service `GET /users/{id}/roles/storage`. Cookie is forwarded. User-Service errors → 503 (or 401 if the user is gone).

**Messenger host** (`messanger.filenkov.store`, `role_field: global_role`) is a non–User-Service upstream `http://messanger:8000` on `auth_network`: `X-Auth-*` only, `/health` public, catch-all `auth: session`, 5m proxy timeout. No User-Service call, no `X-User-Id` / `X-Storage-Role`. Cookie is forwarded; the messenger reads none. The access JWT is not sent.

**Image host** (`image.filenkov.store`, `role_field: image`) is a non–User-Service upstream `http://qwen-image:8000` on external `qwen_image_gen_network`: `X-Auth-*` only (hub `global_role` as `X-Auth-Role`), `/healthz` public (the app's, not the gateway's), catch-all `auth: session`, 30m proxy timeout. On session routes the gateway calls `GET /users/{id}/roles/image` and proxies only `FAMILY` / `ADMIN`; `STRANGER` (including no row) and `BLOCKED` → 403, user 404 → 401, User-Service error → 503. A hub `ADMIN` with no `image` row is refused. No `X-User-Id` / per-service role header; the access JWT is not sent.

**LLM host** (`llm.filenkov.store`) is a non–User-Service upstream `http://qwen38:4001` (llama.cpp server, container `qwen38`, not in this compose, on external `llm_network`): `X-Auth-*` only (hub `global_role`), catch-all `auth: admin`, 10m proxy timeout. No User-Service call and no access JWT. The hub header links it as **Chat** for hub admins only. Owl chat still uses LiteLLM at `litellm-proxy-server:4000` via `LLM_HOST`.

**User-Service** upstream: strip `Cookie` and `Authorization`; set `X-User-Id` from JWT `sub`. Do not send JWTs.

`/hermes/*` is `auth: session` at the edge (any signed-in user; `BLOCKED` / `rejected_cache` still 403). The `/hermes` prefix is stripped before proxying to the Hermes container (`http://hermes:8787` on `auth_network`). Hermes receives `X-Forwarded-Prefix: /hermes` and the injected Bearer. Set `HERMES_DASHBOARD_AUTH_JWT_SECRET` = `JWT_SECRET`. See [docs/hermes.md](docs/hermes.md).

## Hub SPA (`web/`)

The hub is a **resume tree**, not one scrolling page. `App.tsx` keeps `/auth` and `/admin`, redirects the legacy `/architecture/:slug`, and serves everything else from a `path="*"` catch-all; an unknown path or a node the reader may not see redirects to `/`.

**Authoring a node is adding a folder** under `web/src/resume/nodes/` with a `meta.ts` and a `node.tsx` — the folder path *is* the URL path. `web/src/resume/registry.ts` discovers the folders with `import.meta.glob`, so there is no central node list and no route to register. 30 nodes today (root + 29). `registry.test.ts` enforces 8 structural invariants (meta/body pairing, existing parent, kebab-case segments, no reserved depth-1 segment, unique sibling `order`, depth ≤ 4, teaser ≤ 120 chars, root at path `''`), so a typo fails the build rather than producing a missing page.

- `meta.ts` is eager — the spine, branch cards, title and meta line paint immediately, with no spinner. `node.tsx` is lazy: one Rollup chunk per node, named after its URL path.
- `/about-project/architecture/<slug>` is a dynamic child: 10 solution pages served by one node body via `meta.dynamicChild`, no folder each.
- Node visibility is declarative — `visibleWhen: 'signed-in' | 'admin'` in `meta.ts`. `achievements/projects/hermes` is the only signed-in node.
- Header (`components/Layout.tsx`) has three scroll states written onto `<html data-header>` by `hooks/useScrollChrome.ts` (no React re-render on scroll), plus service links to Messanger, Storage, Hermes (everyone, locked for guests), Image (only when the `image` service role is `FAMILY` or `ADMIN`, no lock), Chat (`https://llm.filenkov.store`, hub `ADMIN` only, no lock) and Admin (`ADMIN` only). The owl (`components/owl/Owl.tsx`) is a hero in the flow that cross-fades into a "Back to top" button on scroll.
- Styles live in `web/src/styles/{fonts,tokens,base,shell,article,tree,forms}.css`; fonts are self-hosted from `web/public/fonts/` (no Google Fonts request).
- `web/vite.config.ts` emits `sitemap.xml` and `robots.txt` from the node folders and fails the build if initial JS exceeds 95 KB gzipped (88.5 KB today).
- Deep links work because `web/nginx.conf` does `try_files $uri $uri/ /index.html`. **No `deploy/gateway.yaml` change was needed** — the trailing `- match: /*  upstream: web  auth: public` already serves every node path.

Design doc and what shipped: [docs/resume-tree-frontend.md](docs/resume-tree-frontend.md).

## Admin (SPA → gateway → User-Service)

Hub `ADMIN` only (`/admin` is `auth: admin`). The admin page loads the directory once and shows the full table (no pagination). Email search is client-side only. There is no UUID paste and no `/api/admin`.

- `GET /users` — list (`{ "users": [ … ] }` with persisted `services` per item). Gateway policy **`admin`**. Missing services render as `STRANGER` in the SPA.
- `PUT /users/{id}/role` body `{ "role": "FAMILY" }` — instant apply on select.
- `PUT /users/{id}/roles/{service}` body `{ "role": "FAMILY" }` — fixed columns `storage`, `resume`, `image` (shown before any row exists), plus any other persisted service keys on the loaded rows.

The SPA does not send `X-User-Id` (the gateway overwrites it). User-Service error envelope `{ "error": { "code", "message" } }` is proxied unchanged (`409 LAST_ADMIN` included). Self-edit is allowed. Spec + what shipped: [docs/user-story-admin-directory.md](docs/user-story-admin-directory.md).

Tests: `go test ./...` (`TestHubUsersListAdminOnly`, `TestHubUsersListProxiesWithXUserId`) and `cd web && npm test` (`Admin.test.tsx`, `api.test.ts` `fetchUsers`).

## Roles

Canonical enum in User-Service: `ADMIN` \| `FAMILY` \| `STRANGER` \| `BLOCKED`.

Hub edge auth uses access-JWT `global_role`. Per-service roles are User-Service `{service}` keys (`storage`, `resume`, `image`), not Auth DB columns.

## Gateway routing

YAML: `deploy/gateway.yaml`. First matching route wins. Hosts: `filenkov.store` (hub), `storage.filenkov.store` (one upstream `http://app:8000`, `/health` public, 30m proxy timeout), `messanger.filenkov.store` (one upstream `http://messanger:8000` on `auth_network`, `/health` public, catch-all `auth: session`, 5m proxy timeout), `image.filenkov.store` (one upstream `http://qwen-image:8000` on `qwen_image_gen_network`, `role_field: image`, `/healthz` public, catch-all `auth: session`, 30m proxy timeout), and `llm.filenkov.store` (one upstream `http://qwen38:4001` on `llm_network`, llama.cpp container `qwen38` not in this compose, catch-all `auth: admin`, 10m proxy timeout). Hub `role_field` is `global_role`. `GET /users` (exact) is `auth: admin` → User-Service. `POST /users`, `/users/by-google-sub/*`, and `/users/by-email/*` are `auth: reject` (404 at the gateway; both lookups are service-to-service on `user_network` only). `/users/*` proxies to User-Service (`auth: session`: valid access JWT required). Full schema and Caddy notes: [docs/auth-gateway.md](docs/auth-gateway.md).

## What was removed

| Removed | Replacement |
|---------|-------------|
| Redis session cookie as **edge** credential | `auth_access` + `auth_refresh` JWTs (companion `auth_session` is vault-only) |
| Auth Postgres `users`, `DATABASE_URL`, golang-migrate | User-Service directory |
| gRPC `Validate` / `ListUsers`, `proto/`, `GRPC_ADDR` | Gateway `X-Auth-*` (and `X-User-Id` for User-Service) |
| `WHITELIST_JSON` as product contract | Gateway YAML hosts for `return_to` |
| `GET`/`PATCH /api/admin/users` | Gateway proxy `GET`/`PUT /users/…` |
| Published host ports `8081` (web) and `8082` (API) | Only `127.0.0.1:8083` |
| `ADMIN_EMAIL` bootstrap | User-Service first-user `ADMIN` |
