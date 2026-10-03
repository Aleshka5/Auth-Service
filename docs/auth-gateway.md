# Auth Gateway

Auth-Service is the **auth edge** for Filenkov hosts. Caddy stays a thin TLS terminator. Routing, access-JWT checks, identity headers, rate limits, security headers, and JSON access logs live in `cmd/gateway`.

Hermes (`/hermes`, chat PTY): [hermes.md](hermes.md).

## Goals

- Terminate TLS only in **Caddy** (Let’s Encrypt, HTTP→HTTPS). Caddy reverse-proxies all `filenkov.store` traffic to the gateway.
- Put **host + path routing** in a YAML config file, not in Go. `storage.filenkov.store`, `messanger.filenkov.store`, `image.filenkov.store`, and `llm.filenkov.store` are enabled in the default YAML (`storage` → `http://app:8000`, `messanger` → `http://messanger:8000`, `qwen_image` → `http://qwen-image:8000`, `qwen38` → `http://qwen38:4001`).
- Enforce **edge auth** before any upstream: no valid access JWT (except `public` routes) and `BLOCKED` / `rejected_cache` never reach a backend on `session`/`admin` routes.
- Inject trusted identity headers on proxied requests. Strip client-supplied `X-Auth-*`, `X-User-Id`, and `X-Storage-Role` first. **Never forward JWTs to User-Service.** Hermes routes set `inject_access_jwt: true` so the hub access cookie is forwarded as Bearer for `hermes-auth-gateway/plugins/auth-gateway`. Public Hermes + chat: [hermes.md](hermes.md).
- Keep **`cmd/api`** as the internal auth API (OAuth, `/api/me`, `/api/refresh`, `/api/logout`, `/health`) on unpublished `:8000`. Keep the **nginx SPA container** as an internal upstream. Do not merge the SPA into either Go binary.
- Rate-limit by IP at the gateway. Keep API rate limits as defense in depth.
- JSON-capable logs (`slog` JSON), including access logs for Loki.

## Non-goals

- Replacing Caddy ACME / certificate management.
- Adding further product hosts beyond hub, storage, messenger, image, and LLM (those five are in the default YAML).
- Auth Postgres, gRPC `Validate` / `ListUsers`, or `WHITELIST_JSON`. Do not reintroduce them.
- Using `auth_session` / Redis `session:{sid}` as the **edge** credential. That cookie is a companion opaque sid for product vaults only; the gateway authorizes on the access JWT.
- Merging the Vite SPA into the API or gateway binary.
- Putting JWTs in `localStorage` or `Authorization` for the browser; cookies stay httpOnly.
- Calling User-Service on every Hermes / SPA HTML request. Fast auth is the access JWT at the gateway.

## Request flow

```
Client
  → Caddy :443  (TLS, HTTP→HTTPS, certs)
    → gateway :8083  (host/path route, access JWT + rejected_cache, headers, access log, IP rate limit)
      → api:8000          /oauth/*, /api/*, /health
      → web:80            SPA (public hub + /admin)
      → user_service:8000 /users/*  (Cookie/Authorization stripped; X-User-Id set)
      → hermes:8787                 /hermes/* (prefix stripped; Bearer access JWT + X-Forwarded-Prefix)
      → app:8000                    storage.filenkov.store (X-Auth-* + X-User-Id + X-Storage-Role; Cookie forwarded)
      → messanger:8000              messanger.filenkov.store (X-Auth-*; Cookie forwarded)
      → qwen-image:8000             image.filenkov.store (image role FAMILY/ADMIN only; X-Auth-*; Cookie forwarded)
      → qwen38:4001                llm.filenkov.store (hub ADMIN only; X-Auth-*)
```

Caddy does **not** choose `/oauth` vs `/api` vs SPA vs Hermes vs `/users`. One `reverse_proxy` to the gateway is enough.

The gateway process health endpoint is **`GET /healthz`**. It is answered **before** host routing and is not proxied — unless the request host declares an exact `match: /healthz` route (only `image.filenkov.store`, whose app owns that path); then that host's `/healthz` is routed like any other path. Loopback probes (`127.0.0.1:8083`, compose healthcheck) still hit the gateway. Public `GET /health` on `filenkov.store` still goes to the API (Redis ping; no Postgres).

## Process layout

| Process | Role |
|---------|------|
| `cmd/api` | OAuth, JWT issue/rotate/logout, `/api/me` (calls User-Service), `/health`. Listen `:8000`, unpublished |
| `cmd/gateway` | Public HTTP after Caddy: YAML router, access JWT, reverse proxy, identity headers. Listen `:8083` |
| `web` (nginx) | Static SPA. Internal only (`http://web:80`) |
| User-Service | Directory (external compose). Gateway proxies `/users/*`; API calls it on `user_network` for login/`/api/me` |
| Caddy (host) | TLS terminator → `127.0.0.1:8083` |

Gateway and API share **Redis** (`rejected_cache`, OAuth state, rate limits, companion `session:{sid}`) and **`JWT_SECRET`**. Neither talks to Auth Postgres. The gateway does not mint JWTs. The gateway calls User-Service only to fill `X-Storage-Role` on hosts with `role_field: storage`.

## YAML schema

File: `deploy/gateway.yaml` (override with `GATEWAY_CONFIG`).

Matching rules:

1. Look up the request **hostname** (Host header, port stripped, lowercased) in `hosts`. Optional `aliases` are extra hostnames for the same block (local HTTP).
2. Walk that host’s `routes` **in list order**. **First match wins.** Put more specific prefixes before catch-alls (`/api/refresh` before `/api/*` before `/*`).
3. A `match` of `/foo` is exact. A `match` of `/foo/*` matches `/foo`, `/foo/`, and any path with prefix `/foo/`. A `match` of `/*` matches every path.
4. Optional `methods:` (e.g. `[GET]`, `[POST]`). Empty/omitted matches any method. First matching route still wins — put a method-specific exact path **before** a prefix that would also match (`GET /users` `admin` before `/users/*` `session`, because `/users/*` matches `/users`).
5. Unknown host → `404` JSON. No matching route → `404` JSON.

Auth policies (`auth`):

| Value | Meaning |
|-------|---------|
| `public` | Proxy without an access JWT. Default for the hub catch-all (home, `/auth`, SPA assets). Also `/api/refresh` and `/api/logout` (refresh-only cookies). |
| `session` | Require a valid access JWT; `global_role == BLOCKED` or `rejected_cache` → 403. **Default for new hosts.** |
| `admin` | Require a valid access JWT whose `global_role` is `ADMIN`. |
| `reject` | **404** JSON, never proxied. Used to hide User-Service create (`POST /users`) and the lookups by `google_sub` and by email. |

Hub YAML `role_field` is **`global_role`**. Allowed values on load: `global_role`, `storage`, `image`, `resume` (future hosts). Hub injects the access JWT **`global_role`** as `X-Auth-Role` and does not call User-Service per request for hub HTML. When `role_field` is `storage`, the gateway calls `GET /users/{sub}/roles/storage` (no JWT) and sets `X-Storage-Role` plus `X-User-Id` on that host’s upstream; every role (including `STRANGER`) is proxied and WebStorage decides. When `role_field` is `image`, the gateway calls `GET /users/{sub}/roles/image` on every non-`public` route and **is** the allow/deny: `FAMILY` / `ADMIN` → proxy; `STRANGER` (including no row), `BLOCKED`, or any other value → **403** `{"error":"forbidden"}`; user 404 → **401**; User-Service 5xx / timeout / network error → **503** `{"error":"unavailable"}`. Nothing extra is injected — no `X-User-Id`, no role header. `public` routes on that host skip the lookup.

`strip_prefix` is Caddy `handle_path` semantics: remove that path prefix before proxying (so `/hermes/foo` becomes `/foo` on Hermes). Hermes already understands `X-Forwarded-Prefix` for cookie Path and asset/base URLs. Hub Hermes routes set `inject_access_jwt: true`; the `auth-gateway` plugin verifies the Bearer. `GET /hermes/login` 302s to `/hermes/sessions`. `GET /chat` and `GET /sessions` 302 to `/hermes/chat` and `/hermes/sessions`. Vite `/assets/*` from a Hermes Referer 302s to `/hermes/assets/*`. JSON `next` of `/` or `/login` becomes `/hermes/sessions`. Details: [hermes.md](hermes.md).

`preserve_host` defaults to `true` (send the public Host to the upstream). User-Service routes set `false` so the upstream sees `user_service`.

### Complete example (`filenkov.store` + `storage.filenkov.store` + `messanger.filenkov.store` + `image.filenkov.store` + `llm.filenkov.store`)

See `deploy/gateway.yaml`. Shape:

```yaml
listen: ":8083"
public_origin: "https://filenkov.store"

timeouts:
  read_header: 15s   # header only; does not cap upload bodies
  idle: 120s
  proxy: 60s         # reverse-proxy response header timeout; WebSocket hijack is not killed by this

rate_limit:
  requests_per_minute: 180

upstreams:
  api: "http://api:8000"
  web: "http://web:80"
  user_service: "http://user_service:8000"
  hermes: "http://hermes:8787"
  storage: "http://app:8000"
  messanger: "http://messanger:8000"
  qwen_image: "http://qwen-image:8000"
  # llama.cpp server, container qwen38, already on llm_network (not in this compose).
  qwen38: "http://qwen38:4001"

hosts:
  filenkov.store:
    role_field: global_role
    aliases: []          # local: [localhost]
    routes:
      - match: /oauth/*
        upstream: api
        auth: public
      - match: /api/refresh
        upstream: api
        auth: public
      - match: /api/logout
        upstream: api
        auth: public
      - match: /api/me
        upstream: api
        auth: session
      - match: /api/*
        upstream: api
        auth: session    # deny-by-default if something is added
      - match: /users/by-google-sub/*
        auth: reject     # Auth API only, on user_network
      - match: /users/by-email/*
        auth: reject     # service-to-service lookup on user_network only
      - match: /users
        methods: [POST]
        auth: reject     # create is Auth API only
      - match: /users
        methods: [GET]
        upstream: user_service
        auth: admin      # hub ADMIN directory list
        preserve_host: false
      - match: /users/*
        upstream: user_service
        auth: session
        preserve_host: false
      - match: /health
        upstream: api
        auth: public
      - match: /hermes
        upstream: hermes
        auth: session
        strip_prefix: /hermes
        inject_access_jwt: true
      - match: /hermes/login
        methods: [GET]
        auth: session
        redirect: /hermes/sessions
      - match: /hermes/*
        upstream: hermes
        auth: session
        strip_prefix: /hermes
        inject_access_jwt: true
      - match: /chat
        methods: [GET]
        auth: session
        redirect: /hermes/chat
      - match: /sessions
        methods: [GET]
        auth: session
        redirect: /hermes/sessions
      - match: /admin
        upstream: web
        auth: admin
      - match: /admin/*
        upstream: web
        auth: admin
      - match: /*
        upstream: web
        auth: public     # /, /auth, every resume node path, SPA assets

  storage.filenkov.store:
    role_field: storage
    timeouts:
      proxy: 30m         # uploads; file-level default stays 60s for the hub
    routes:
      - match: /health
        upstream: storage
        auth: public
      - match: /*
        upstream: storage
        auth: session

  messanger.filenkov.store:
    role_field: global_role
    timeouts:
      proxy: 5m          # attachment uploads; does not cap the request body
    routes:
      - match: /health
        upstream: messanger
        auth: public
      - match: /*
        upstream: messanger
        auth: session    # catch-all serves both /api/* and the SPA

  image.filenkov.store:
    role_field: image    # GET /users/{sub}/roles/image; FAMILY/ADMIN only
    timeouts:
      proxy: 30m         # one generate can take minutes before the first byte
    routes:
      - match: /healthz
        upstream: qwen_image
        auth: public     # the app's health, not the gateway's
      - match: /*
        upstream: qwen_image
        auth: session    # UI + POST /api/generate

  llm.filenkov.store:
    role_field: global_role
    timeouts:
      proxy: 10m         # chat completions / streaming TTFB
    routes:
      - match: /*
        upstream: qwen38
        auth: admin      # hub global_role == ADMIN only
```

Hub public routes (catch-all `auth: public`): `/`, `/auth`, static SPA assets, and **every path the resume tree serves** — anything not matched by an earlier route reaches the SPA, which resolves it against `web/src/resume/registry.ts` and falls back to `/`. `/architecture/:slug` is now only a client-side redirect into `/about-project/architecture/:slug`, not a gateway route. The tree needed **no** `deploy/gateway.yaml` change: the trailing `- match: /*` already served every node path, and `web/nginx.conf` already does `try_files $uri $uri/ /index.html`. Explicit public API: OAuth, `/api/refresh`, `/api/logout`, `/health`. Protected: `/admin` and **`GET /users`** (`admin`); `/hermes` and `/hermes/*` (session JWT + injected Bearer; [hermes.md](hermes.md)); `/api/me` and other `/api/*` (`session`); `/users/*` (`session`). Hidden: **`POST /users`**, `/users/by-google-sub/*`, and `/users/by-email/*` (`reject`) — both lookups are service-to-service on `user_network` only, and their reject routes must stay above `/users/*`. There is **no** `/api/admin/*` route. Hub admin directory: [user-story-admin-directory.md](user-story-admin-directory.md).

Storage host: `GET /health` is public. Everything else requires a valid access JWT (`auth: session`). Anonymous HTML → **302** hub `/auth?return_to=`; `/api/*` and XHR → **401**. Hub `POST /users` stays **rejected**. Hub `GET /users` is **admin** (not anonymous, not session).

Messenger host (`messanger.filenkov.store`): one upstream `messanger` → `http://messanger:8000` (HomeMessanger joins `auth_network` as compose service `messanger`). `GET /health` is public; the catch-all is `auth: session` and serves both `/api/*` and the SPA (anonymous HTML → **302** hub login; `/api/*` and JSON Accept → **401**). `role_field: global_role`, so hub `global_role` is injected as `X-Auth-Role` and no User-Service call is made on this host — no `X-User-Id`, no per-service role. Per-host proxy timeout **5m** (attachment uploads). Do not send the access JWT to the messenger.

Image host (`image.filenkov.store`): one upstream `qwen_image` → `http://qwen-image:8000` (Qwen Image, one Flask process for UI + `POST /api/generate`) on external `qwen_image_gen_network`. `GET /healthz` is public and proxied to the app. The catch-all is `auth: session` (anonymous HTML → **302** hub login; `/api/*` and XHR → **401**) and, with `role_field: image`, also requires the User-Service `image` role to be `FAMILY` or `ADMIN` — hub `global_role` does not count, so a hub `ADMIN` with no `image` row gets **403**. Grant it in `/admin`. `X-Auth-*` only; no `X-User-Id`, no `X-Image-Role`, no JWT. Per-host proxy timeout **30m**. If `qwen-image` is down, only this host returns a proxy error.

LLM host (`llm.filenkov.store`): one upstream `qwen38` → `http://qwen38:4001` (llama.cpp server, container name `qwen38`, not in this compose, already attached to external `llm_network`). Catch-all `auth: admin` (anonymous HTML → **302** hub login; a signed-in non-admin → **403**). Hub `global_role` is injected as `X-Auth-Role`. No User-Service call and no access JWT. Per-host proxy timeout **10m** (slow first token). The hub header shows **Chat** to hub admins only. Owl chat still calls LiteLLM at `litellm-proxy-server:4000` via `LLM_HOST`; this host is not that proxy. Do not reach `qwen38` through a published host port.

## Auth decisions (redirect vs 401 vs 403)

If the route is not `public`, a missing access JWT, a rejected jti, or `BLOCKED` / user revocation is not proxied.

| Situation | Browser / HTML navigation | API / non-browser |
|-----------|---------------------------|-------------------|
| Anonymous on `public` | Proxy | Proxy |
| Anonymous on `session` or `admin` | **302** to `{public_origin}/auth?return_to=` original URL | **401** JSON `{"error":"unauthorized"}` |
| Access JWT + `BLOCKED` or `rejected:user:` (any non-public route) | **403** JSON `{"error":"blocked"}` | same |
| Access JWT + not `ADMIN` on `admin` route | **403** JSON `{"error":"forbidden"}` | same |
| Access JWT + `image` role not `FAMILY`/`ADMIN` on an image-host non-public route | **403** JSON `{"error":"forbidden"}` | same |
| Per-service role lookup (`storage`, `image`) fails / user gone | **503** `{"error":"unavailable"}` / **401** | same |
| Access JWT OK | Proxy + identity headers | Proxy + identity headers |
| `BLOCKED` on a **`public`** route | Proxy (hub home stays reachable) | Proxy |
| `auth: reject` | **404** `{"error":"not found"}` | same |

Non-browser detection (any one is enough):

- Path prefix `/api/` or `/users/` (and exact `/users`)
- `Accept` contains `application/json`
- `X-Requested-With` is present (typical XHR)

`return_to` is the original request URL (`https://` + public host + path + query). Only hosts listed in the gateway YAML (plus `PUBLIC_ORIGIN`, and loopback when `AUTH_DEV_HTTP=true`) are allowed — same open-redirect rule as OAuth.

Public routes with a valid access JWT still get identity headers on non–User-Service upstreams (the SPA can show a username). Anonymous public requests get no `X-Auth-*` after the strip.

## Header injection

Before proxying, the gateway **deletes incoming `X-Auth-*`, `X-User-Id`, and `X-Storage-Role`** (case-insensitive) so clients cannot spoof identity.

### Non–User-Service upstreams (`api`, `web`, `hermes`, …)

When a valid access JWT exists:

| Header | Value |
|--------|--------|
| `X-Auth-User-Id` | JWT `sub` (User-Service UUID) |
| `X-Auth-Email` | JWT `email` |
| `X-Auth-Name` | JWT `username` |
| `X-Auth-Role` | JWT `global_role` |

Cookie is forwarded (including companion `auth_session`).

### Storage host (`role_field: storage` → upstream `storage`)

When a valid access JWT exists, the six headers below are set. `X-Auth-Role` remains hub `global_role` (WebStorage must ignore it for gates). `X-Storage-Role` comes from User-Service `GET /users/{sub}/roles/storage` (network trust, no JWT). Missing row is `200` `"role": "STRANGER"` with no insert.

| Header | Value |
|--------|--------|
| `X-Auth-User-Id` | JWT `sub` |
| `X-Auth-Email` | JWT `email` |
| `X-Auth-Name` | JWT `username` |
| `X-Auth-Role` | JWT `global_role` |
| `X-User-Id` | JWT `sub` (same UUID) |
| `X-Storage-Role` | User-Service **storage** role (`ADMIN` \| `FAMILY` \| `STRANGER` \| `BLOCKED`) |

- User-Service **5xx / timeout / network error** → **503** `{"error":"unavailable"}`, no proxy. Do not invent a role.
- User **404** → **401** `{"error":"unauthorized"}`, no proxy.
- Do **not** set `X-Storage-Role` = `global_role`.

### User-Service upstream

1. Do **not** forward `Cookie`, `Authorization`, or JWT query params (`access_token`, `token`, `jwt`, cookie names, …).
2. Do **not** send `X-Auth-*` as authorization (User-Service loads actor roles from its own store).
3. Set **`X-User-Id`** = access JWT `sub` on every proxied request (GETs ignore it; PUTs require it). Overwrite; never pass the client value.

After a **200** on `PUT /users/{id}/role` or `PUT /users/{id}/roles/{service}`, the gateway writes `rejected:user:{id}` for the **path** UUID (target user) so a stale embedded `global_role` cannot authorize that person, and deletes that user’s companion `auth_session` mapping.

Forwarding (Caddy is the TLS hop; gateway is the next hop):

- `X-Forwarded-For` — append client IP (ReverseProxy default)
- `X-Forwarded-Proto` — keep Caddy’s value if present, else `https` (or `http` when `AUTH_DEV_HTTP`)
- `X-Forwarded-Host` — public Host
- `Host` — public hostname unless the route sets `preserve_host: false`

Security headers on **gateway responses** (including proxied ones):

- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `Referrer-Policy: strict-origin-when-cross-origin`
- `Permissions-Policy: camera=(), microphone=(), geolocation=()`
- `Strict-Transport-Security: max-age=31536000; includeSubDomains` when cookies are Secure (production)

WebSocket (Hermes chat PTY): see [hermes.md](hermes.md). `httputil.ReverseProxy` handles `Upgrade`. The proxy transport disables `ForceAttemptHTTP2`. `ModifyResponse` and security headers are skipped on `101`. Unauthenticated Upgrade requests get `401` JSON, not a login `302`. Do not wrap the handler in `http.TimeoutHandler`. `ReadHeaderTimeout` is header-only so large uploads and hijacked sockets are not killed by it. File-level `timeouts.proxy` is a **response header** timeout (default 60s). A host may override with `hosts.<name>.timeouts.proxy` (storage and image use **30m**; messenger uses **5m**; LLM uses **10m**).

## JWT at the edge

Cookies `auth_access` / `auth_refresh` (`HttpOnly`, `SameSite=Lax`, `Secure` unless `AUTH_DEV_HTTP`). The gateway validates **access** only (signature, `typ`, `exp`, `aud`/`iss`) plus Redis `rejected_cache`. It does not mint tokens. JWT `aud` includes every host in the gateway YAML (so `storage.filenkov.store`, `messanger.filenkov.store`, `image.filenkov.store`, and `llm.filenkov.store` are valid audiences).

Companion cookie **`auth_session`**: opaque 32-byte hex sid, same cookie flags as the JWTs, TTL = `JWT_REFRESH_TTL`. Redis `session:{sid}` → User-Service UUID. Issued on Google callback; **kept** (TTL extended) on `POST /api/refresh` if the cookie still maps to that user; cleared and deleted on `POST /api/logout` and when the user is revoked. Not checked at the edge — missing sid does not block proxy to storage.

Gateway rejects an access JWT if:

1. Signature / `typ` / `exp` / `aud` fail, or the cookie is missing (treated as anonymous),
2. `rejected:jti:{jti}` exists, or
3. `rejected:user:{sub}` exists and token `iat` is at or before that revocation timestamp.

`auth: session` = valid access JWT, not rejected, `global_role != BLOCKED`.  
`auth: admin` = session + `global_role == ADMIN`.

Auth API issues tokens after Google login and on `POST /api/refresh`. It must not set cookies when User-Service `global_role == BLOCKED`.

## Docker topology and env

Compose networks: **`auth_network`** (created here), **`user_network`** (`external: true`), and **`llm_network`** (`external: true`). `web` and `redis` sit only on `auth_network`. `api` and `gateway` attach `user_network` so they can reach `user_service:8000`. The gateway also attaches `llm_network` so it can reach `qwen38:4001`. The API attaches the same network so owl chat can reach `litellm-proxy-server:4000`.

WebStorage’s `app` container must join **`auth_network` as `external: true`** so the gateway can reach `http://app:8000`. Do not publish WebStorage as a second public edge. Hermes is started from `hermes-auth-gateway` compose on the same network (`http://hermes:8787`). Do not publish Hermes `:8787` on a public interface. Dashboard auth: JWT plugin only ([hermes.md](hermes.md)). **`llm_network`** is external. The gateway joins it so `http://qwen38:4001` resolves (`qwen38` is a one-off llama.cpp container, not a compose service). The API joins it so owl chat can reach `http://litellm-proxy-server:4000`. Do not publish LiteLLM `:4000` on a public interface. The gateway does not use a host-published port for `qwen38`.

Upstreams use **container names**. Hermes is the `hermes` container on **`auth_network`** (`hermes-auth-gateway` compose joins that network as `external: true`, default Docker name `deploy_auth_network`):

```text
hermes: "http://hermes:8787"
```

Published: **`127.0.0.1:8083:8083` only**. API `:8000` and web `:80` are unpublished.

Gateway env (JWT + Redis; no Postgres):

| Variable | Purpose |
|----------|---------|
| `JWT_SECRET` | HS256 secret, ≥32 bytes, shared with the API. Never sent upstream to User-Service |
| `JWT_ACCESS_TTL` / `JWT_REFRESH_TTL` | Defaults `15m` / `168h` |
| `REDIS_URL` | `rejected_cache` + rate-limit counters + companion `session:{sid}` |
| `COOKIE_DOMAIN` / `AUTH_DEV_HTTP` | Same cookie flags as the API |
| `PUBLIC_ORIGIN` | Login redirect base + return_to default |
| `GATEWAY_CONFIG` | Path to YAML (container: `/etc/auth/gateway.yaml`) |
| `GATEWAY_HTTP_ADDR` | Optional listen override (else YAML `listen`) |
| `USER_SERVICE_URL` | Directory base (`http://user_service:8000`). Gateway uses it for `X-Storage-Role` and image-host role lookups |

The API also sets `GATEWAY_CONFIG` so OAuth `return_to` uses the **same host list** as the gateway.

Do not set `DATABASE_URL`, `GRPC_ADDR`, `ADMIN_EMAIL`, `COOKIE_NAME`, `SESSION_TTL`, or `WHITELIST_JSON`.

Gateway compose healthcheck: `GET http://127.0.0.1:8083/healthz`. API: `GET http://127.0.0.1:8000/health`.

Operator order: User-Service stack (so `user_network` exists), LiteLLM stack (so `llm_network` and `litellm-proxy-server` exist for owl chat) with the one-off `qwen38` container already on that network, image stack (so `qwen_image_gen_network` and `qwen-image` exist — the gateway will not start without that network), Auth stack (creates `auth_network`, joins `user_network`, `llm_network`, and `qwen_image_gen_network`), `hermes-auth-gateway` compose (Hermes copies plugins into `$HERMES_HOME/plugins/` and joins `auth_network` as `hermes`), Common (for WebStorage data), then WebStorage (`app` on `auth_network`).

## Migration: Caddyfile

Canonical host file: [`deploy/Caddyfile.filenkov`](../deploy/Caddyfile.filenkov). Copy to `/etc/caddy/Caddyfile` (needs sudo).

One `reverse_proxy localhost:8083` per product host. HTTP→HTTPS and certificates stay in Caddy. **No** `encode gzip`. **No** per-path Hermes handles (do not proxy `/hermes/api/pty` to `:8787`).

Hermes chat WebSocket needs HTTP/1.1 or HTTP/2. HTTP/3 `Alt-Svc` and security headers on `101` break `/hermes/chat` — see [hermes.md](hermes.md).

```caddy
{
	servers {
		protocols h1 h2
	}
}

https://filenkov.store {
	reverse_proxy localhost:8083
	header {
		Alt-Svc "clear"
	}
	@notWebsocket {
		not header Connection *Upgrade*
	}
	header @notWebsocket {
		Strict-Transport-Security "max-age=31536000; includeSubDomains"
		X-Content-Type-Options "nosniff"
		X-Frame-Options "DENY"
		Referrer-Policy "strict-origin-when-cross-origin"
		Permissions-Policy "camera=(), microphone=(), geolocation=()"
	}
}
```

Reload: `sudo systemctl reload caddy`. A reload of an **old** on-disk Caddyfile re-enables HTTP/3.

## Test plan

- **Route matching:** host, path exact vs prefix, `methods:`, `strip_prefix`, first-match-wins, aliases, `auth: reject` → 404 and no proxy. Hub `GET /users` is `admin` + `user_service`; `POST /users` is `reject`. Tests: `TestLoadDefaultYAML`, `TestRouterFirstMatch`, `TestHubUsersListAdminOnly`, `TestHubUsersListProxiesWithXUserId`.
- **Auth:** anonymous `public` OK; anonymous `session`/`admin` → redirect (HTML) or 401 (`/api/*`, `/users/*`, JSON Accept, `X-Requested-With`); STRANGER on `/hermes` → 200 (session); STRANGER on `/admin` → 403; ADMIN OK; BLOCKED → 403 on session/admin routes; BLOCKED still served on `public`.
- **JWT / cache:** invalid signature → anonymous; rejected jti → 401 on session routes; `rejected:user:` after role PUT → 403; refresh/logout stay `public`.
- **Headers:** strip spoofed `X-Auth-*` / `X-User-Id` / `X-Storage-Role`; inject id/email/username/`global_role` for non–User-Service; storage host also gets `X-User-Id` + `X-Storage-Role` from User-Service (not `global_role`); User-Service gets `X-User-Id` only and no cookies; storage receives `auth_session`.
- **Storage host:** anonymous HTML → 302 hub login; `/health` public; User-Service role GET 5xx → 503; per-host proxy timeout 30m.
- **Companion cookie:** issued on login, stable across access refresh, cleared on logout.
- **Open redirect:** `return_to` only for hosts in the YAML (and `PUBLIC_ORIGIN`; loopback if `AUTH_DEV_HTTP`), including `storage.filenkov.store`, `messanger.filenkov.store`, `image.filenkov.store`, and `llm.filenkov.store`.
- **Messenger host:** `/health` public; anonymous HTML → 302 hub login, anonymous XHR → 401; session JWT → proxy to `http://messanger:8000` with the four `X-Auth-*`; BLOCKED → 403; client-supplied `X-Auth-User-Id` overwritten; no User-Service call; per-host proxy timeout 5m.
- **Image host:** anonymous `/healthz` proxied (no role call, also for a signed-in `STRANGER`); anonymous HTML → 302 hub login, anonymous XHR / `POST /api/generate` → 401; `image` `FAMILY`/`ADMIN` → proxied with the four `X-Auth-*`; `image` `STRANGER` / `BLOCKED` and hub `ADMIN` with image `STRANGER` → 403, not proxied; hub BLOCKED / `rejected_cache` → 403 before any role call; role GET 5xx → 503; user 404 → 401; spoofed `X-Auth-*` overwritten; per-host proxy timeout 30m.
- **LLM host:** anonymous HTML → 302 hub login; signed-in non-admin → 403; hub `ADMIN` → proxy to `http://qwen38:4001` with `X-Auth-*` and no access JWT; per-host proxy timeout 10m. Owl chat is unchanged (`LLM_HOST` → `litellm-proxy-server:4000`).
- **Proxy:** httptest upstream sees the path after strip; `Host` preserved by default; User-Service `preserve_host: false`. Hermes: injected Bearer, login skip, `/chat`/`/sessions` redirects, Vite `/assets` Referer redirect, JSON `next` `/` → `/hermes/sessions`, WebSocket 101 (`TestHermesInjectAccessJWTWhenEnabled`, `TestHermesLoginSkippedWhenGatewayAuthed`, `TestHermesLazyAssetsRedirectWhenRefererIsHermes`, `TestHermesWebSocketUpgrade`).
- **Health:** `/healthz` is local even when `/health` is a proxied API route, and on every host without an exact `/healthz` route; on the image host it is proxied.
- `go test ./...` for API + gateway. Frontend: `cd web && npm test`.

## Rollout

1. Start User-Service so `user_network` exists, and the LiteLLM stack so `llm_network` and `litellm-proxy-server` exist. The one-off `qwen38` container must already be on `llm_network`. Recreate Auth compose (`redis`, `web`, `api`, `gateway`) so the gateway joins `llm_network`. Confirm `127.0.0.1:8083/healthz` and a browser hit through Caddy (or `curl -H Host:filenkov.store`).
2. Switch Caddy to `reverse_proxy 127.0.0.1:8083`. Reload.
3. Smoke: public home, Google login (JWT cookies **and** `auth_session`), `/api/me`, `/admin` directory list (`GET /users`) + `PUT /users/{id}/role`, `/hermes/` (any signed-in user), anonymous `/admin` → login. Confirm `/api/admin` is 404 and `POST /users` is 404. Hit `Host: storage.filenkov.store` with a valid access JWT and confirm `X-Storage-Role` / `X-User-Id`. Hit `Host: llm.filenkov.store` with a hub `ADMIN` access JWT and confirm proxy to `qwen38` plus `X-Auth-*`; a non-admin token is 403. Hit `Host: messanger.filenkov.store`: `/health` anonymous is 200, and `/api/me` with a valid access JWT proxies to the messenger with `X-Auth-*`. Hit `Host: image.filenkov.store`: `/healthz` anonymous is 200 from the app; `/` with an `image` `FAMILY` user proxies, with an `image` `STRANGER` user is 403.

## Product services

Other product services **sit behind this gateway** and trust `X-Auth-*` (storage also trusts `X-User-Id` / `X-Storage-Role` and the `auth_session` cookie). They do not call Auth-Service for a user whitelist. User-Service is the directory; Auth issues JWTs and the gateway authorizes at the edge. There is no gRPC Validate for new callers.
