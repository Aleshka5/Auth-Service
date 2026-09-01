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

User-Service enum (exact strings): `ADMIN` | `FAMILY` | `STRANGER` | `BLOCKED`. Hub identity at the edge is access-JWT `global_role`. Per-service roles live in User-Service under keys such as `storage` and `resume` — not Auth table columns. First User-Service user becomes `ADMIN`; later Google logins start as `STRANGER`. `BLOCKED` on a service does not lock the hub.

## Auth Gateway

- **Caddy** terminates TLS (Let’s Encrypt, HTTP→HTTPS only). It reverse-proxies to this gateway. It does not choose `/oauth` vs SPA vs Hermes vs `/users`.
- **Gateway** (`cmd/gateway`) is the public HTTP edge (`:8083`): YAML host/path routing, access-JWT + `rejected_cache`, identity headers, IP rate limits, security headers, JSON access logs.
- **API** (`cmd/api`) stays **internal** (`:8000`, unpublished): OAuth, `/api/me`, `/api/refresh`, `/api/logout`, `/health`. The Vite SPA stays a separate nginx container (internal upstream). Do not merge the SPA into a Go binary. Do not publish API or web ports.

Routing is YAML (`deploy/gateway.yaml`). Live hosts: `filenkov.store`, `storage.filenkov.store`, and `llm.filenkov.store`. Adding a host is a config change, not a code change. First matching route wins — put specific prefixes first. See `docs/auth-gateway.md`.

Edge policy: no valid access JWT, or `global_role == BLOCKED` / user in `rejected_cache`, is not proxied (except `auth: public`). Browser navigations redirect to `{PUBLIC_ORIGIN}/auth?return_to=…`; `/api/*`, `/users/*`, exact `/users`, and typical XHR get `401` JSON; BLOCKED with a token gets `403`. Hub main page, `/auth`, `/architecture`, SPA assets, `/oauth/*`, `/api/refresh`, `/api/logout`, and `/health` are public. `/admin` and `GET /users` are admin-only (`global_role == ADMIN`). `/hermes/*` is session (any signed-in user). `/api/me` requires an access JWT. `POST /users` is rejected.

On proxied requests the gateway strips incoming `X-Auth-*`, `X-User-Id`, and `X-Storage-Role`.

- **Non–User-Service** upstreams (`api`, `web`, `hermes`, …): inject `X-Auth-User-Id`, `X-Auth-Email`, `X-Auth-Name` (username), `X-Auth-Role` (access JWT `global_role`). Product services sit **behind the gateway** and trust those headers.
- **Hermes only:** set `X-Forwarded-Prefix: /hermes` and `Authorization: Bearer <access JWT>` (`inject_access_jwt: true`). Hermes `auth-gateway` plugin verifies that JWT (`HERMES_DASHBOARD_AUTH_JWT_SECRET` = `JWT_SECRET`). No second login (`GET /hermes/login` → `/hermes/sessions`). Chat is a PTY WebSocket through the same proxy — [docs/hermes.md](docs/hermes.md). Hermes joins `auth_network`; upstream is `http://hermes:8787`. Do not publish Hermes `:8787` on a public interface. User-Service still never receives a JWT.
- **Storage host** (`role_field: storage`): also inject `X-User-Id` (JWT `sub`) and `X-Storage-Role` from User-Service `GET /users/{id}/roles/storage` (not `global_role`). Forward `Cookie` so `auth_session` reaches WebStorage. User-Service 5xx/timeout → 503, no proxy; user 404 → 401.
- **LiteLLM host** (`llm.filenkov.store`): proxy to `http://litellm-proxy-server:4000` on external `llm_network`. `auth: session`. Inject `X-Auth-*` (hub `global_role`). Do not publish LiteLLM `:4000` on a public interface. User-Service still never receives a JWT.
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
2. Main page (`filenkov.store`) — public. Sign in vs **username**. Links: llm.filenkov.store, storage.filenkov.store, and `filenkov.store/hermes/chat` (any signed-in user).
3. Admin page (`filenkov.store/admin`) — hub `global_role == ADMIN` only. Directory table from User-Service `GET /users` (full list, email search in the SPA), then `PUT /users/{id}/role` and `PUT /users/{id}/roles/{service}`. Gateway exposes `GET /users` as `auth: admin`; `POST /users` stays rejected. Do not call `/api/admin`. Shipped behavior: `docs/user-story-admin-directory.md`.

SPA: `credentials: 'include'`; on `401` from `/api/me` (and other same-origin calls), `POST /api/refresh` then retry. Display `username` / `email` / `global_role` / `services[]`.

Hub and storage roles in the UI: `ADMIN` | `FAMILY` | `STRANGER` | `BLOCKED`.
