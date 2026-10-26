# Auth Gateway — design and implementation plan

Status: implemented (`deploy/gateway.yaml` is the single source of truth).
Scope: replace the old `cmd/gateway` + Caddy split with one Go edge that proxies
and injects identity. Caddy keeps TLS only.

## Goals

- Single YAML routing table for every filenkov host.
- Access JWT is the only edge credential; refresh stays inside Auth API.
- Identity is propagated as `X-Auth-*` headers.
- `GET /users` is hub-admin-only. `POST /users` stays rejected at the edge.
- No gRPC. No Auth Postgres. `WHITELIST_JSON` is gone.
- The live hosts are `filenkov.store`, `storage.filenkov.store`, `image.filenkov.store`, `llm.filenkov.store`, and `rag.filenkov.store`; add the others later. `messanger.filenkov.store` and the `/hermes` mount were removed in October 2026 (see `docs/hermes.md`).

## Non-goals

- **Not in scope:** gRPC, Auth Postgres lookup, per-service role tables in Auth, `WHITELIST_JSON`, `POST /users` through the hub, adding more product hosts beyond hub, storage, messenger, image, LLM, and RAG;
- **Not in scope:** `session`-scoped roles, `public`-scoped routes, or any
  route that reads `Cookie` / `auth_session` at the edge;
- **Not in scope:** a second login for Hermes — the injected access JWT is it;
- **Not in scope:** JWT verification inside User-Service.

## Topology

```text
Browser
  -> Caddy :443 (TLS, h1/h2, HTTP->HTTPS, security headers)
  -> gateway :8083 (this service, public edge)
  -> api:8000         filenkov.store /api, /oauth, /health
  -> web:80           filenkov.store SPA + /admin
  -> user_service:8000  /users* (X-User-Id; no Cookie/JWT)
  -> app:8000         storage.filenkov.store (X-Auth-* + X-User-Id + X-Storage-Role)
  -> qwen-image:8000  image.filenkov.store (X-Auth-*; Cookie forwarded)
  -> qwen38:4001      llm.filenkov.store (X-Auth-*)
  -> rag-edge:8080    rag.filenkov.store (X-Auth-*; Cookie forwarded)
```

Storage sits on `auth_network`; User-Service on external
`user_network`; Qwen Image on external `models_network`; llama.cpp
(`qwen38`) on external `models_network` (both owned by Switch-Service); RAG-Chat (`rag-edge`) on external `rag_edge`.

## Config

`deploy/gateway.yaml` — example shape:

```yaml
listen: ":8083"
public_origin: "https://filenkov.store"

timeouts:
  read_header: 15s
  idle: 120s
  proxy: 60s # default (hub / others)

rate_limit:
  requests_per_minute: 180

upstreams:
  api: "http://api:8000"
  web: "http://web:80"
  user_service: "http://user_service:8000"
  storage: "http://app:8000"
  qwen_image: "http://qwen-image:8000"
  qwen38: "http://qwen38:4001"
  rag: "http://rag-edge:8080"

hosts:
  filenkov.store:
    role_field: global_role
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
      - match: /api/*
        upstream: api
        auth: session
      - match: /users/by-google-sub/*
        auth: reject
      - match: /users/by-email/*
        auth: reject
      - match: /users
        methods: [POST]
        auth: reject
      - match: /users
        methods: [GET]
        upstream: user_service
        auth: admin
      - match: /users/*
        upstream: user_service
        auth: session
      - match: /health
        upstream: api
        auth: public
      - match: /admin
        upstream: web
        auth: admin
      - match: /admin/*
        upstream: web
        auth: admin
      - match: /*
        upstream: web
        auth: public

  storage.filenkov.store:
    role_field: storage
    timeouts:
      proxy: 30m
    routes:
      - match: /health
        upstream: storage
        auth: public
      - match: /*
        upstream: storage
        auth: session

  image.filenkov.store:
    role_field: image
    timeouts:
      proxy: 30m
    routes:
      - match: /healthz
        upstream: qwen_image
        auth: public
      - match: /*
        upstream: qwen_image
        auth: session

  llm.filenkov.store:
    role_field: global_role
    timeouts:
      proxy: 10m
    routes:
      - match: /*
        upstream: qwen38
        auth: admin

  rag.filenkov.store:
    role_field: global_role
    timeouts:
      proxy: 5m
    routes:
      - match: /health
        upstream: rag
        auth: public
      - match: /*
        upstream: rag
        auth: session
```

First matching route wins. Put specific prefixes first.

## Request flow

1. Parse `Host`, look up the `hosts` entry.
2. Match path (and optional method list) against routes.
3. Resolve auth policy:
   - `public`: proxy without JWT checks.
   - `session`:
     - valid access JWT and not rejected → continue;
     - missing/expired access JWT with a valid refresh JWT → call
       `POST {auth_api}/api/refresh` (single-flight per refresh token),
       write new cookies, re-validate, continue the same request;
     - refresh 401 / no refresh → anonymous deny:
       - browser navigation (no `/api/*`, `/users/*`, exact `/users`, or XHR
         markers) → 302 `{public_origin}/auth?return_to=…`
       - otherwise → 401 JSON
     - access JWT present, `global_role == BLOCKED`, or `rejected:user:{sub}` → 403.
   - `admin`: same as session, then require `global_role == ADMIN`.
4. Rejected jti: `rejected:jti:{jti}` → 401 even if the token still verifies.
5. For User-Service upstreams: strip `Cookie`, `Authorization`, `?access_token=`,
   `?refresh_token=`; set `X-User-Id: <sub>`.
6. For storage: on top of `X-Auth-*`, set `X-User-Id: <sub>` and
   `X-Storage-Role` from `GET /users/{sub}/roles/storage`.
   - 5xx / timeout → 503, no proxy
   - 404 user → 401
7. For all other non–User-Service upstreams: inject identity headers (below)
   from the access JWT.

## Identity headers

| Header | Value |
|---|---|
| `X-Auth-User-Id` | User-Service UUID (`sub`) |
| `X-Auth-Email` | `email` |
| `X-Auth-Name` | `username` (fall back to `email`) |
| `X-Auth-Role` | JWT `global_role` (hub) or the service role |
| `X-User-Id` | UUID — storage and User-Service routes only |
| `X-Storage-Role` | storage host only, from User-Service role lookup |

The gateway strips incoming `X-Auth-*`, `X-User-Id`, and `X-Storage-Role`.
The messenger host forwards `Cookie` (its own session cookie + the companion
`auth_session`) but adds no extra headers beyond `X-Auth-*`.

## Rejected cache

Redis keys (TTL 7 days):

- `rejected:jti:{jti}` — signed-out token must not work.
- `rejected:user:{uuid}` — global BLOCKED.

Auth API `/api/refresh` writes `rejected:jti:{old_jti}` on rotation and on
logout. User-Service role changes to `BLOCKED` should set `rejected:user:{uuid}`
(AfterUserUpdate hook).

## return_to allowlist

Hosts in the YAML are the allowlist (https in production). No `WHITELIST_JSON`.
The list sets the JWT `aud`: `filenkov.store` (hub) plus `storage.filenkov.store`,
`image.filenkov.store`, `llm.filenkov.store`, and
`rag.filenkov.store`.

## Caddy

`deploy/Caddyfile.filenkov` keeps one site block per host and only:

- `tls`
- `reverse_proxy localhost:8083` (or the gateway host port)
- HTTP→HTTPS redirect
- `Strict-Transport-Security` and friends; WebSocket `@notWebsocket` handling
- `protocols h1 h2` in the global options (HTTP/3 breaks Hermes PTY WS)

Caddy does not know about `/oauth`, `/admin`, or `/users`.

## Go layout

```text
cmd/gateway/
  main.go
internal/gateway/
  config.go        # YAML load + validation
  server.go        # routes, match, proxy
  auth.go          # JWT verify, rejected, refresh
  user_service.go  # role lookup, X-User-Id routes
  headers.go       # X-Auth-* / strip / security headers
  ratelimit.go     # Redis IP counter
  logs.go          # slog JSON access logs
  config_test.go
  server_test.go
```

## What changes in this repo

1. `cmd/gateway` becomes the YAML-driven edge (listen `:8083`).
2. `cmd/api` keeps OAuth + `/api/me` + `/api/refresh` + `/api/logout` + `/api/chat` + `/health` on `:8000` (internal).
   `LLM_HOST`/`LLM_API_KEY`/`LLM_MODEL` from `.env` (no Postgres for the owl chat).
   `MLFLOW_TRACING_ENDPOINT`/`MLFLOW_TRACING_EXPERIMENT_ID` (both optional)
   enable the per-question span shipped over OTLP/HTTP — see `internal/tracing`.
   Unset endpoint → the tracer is a no-op and the chat is unaffected.
3. `deploy/gateway.yaml` is the single routing source of truth.
4. `deploy/Caddyfile.filenkov` shrinks to TLS + reverse-proxy only.
5. SPA + nginx: unchanged.
6. User-Service: unchanged except BLOCKED → rejected cache hook.
7. Hermes: unchanged (plugin already exists).
8. WebStorage: unchanged (trusts `X-Auth-*` + `X-Storage-Role` + `X-User-Id`).

## What stays

- Cookie names, domain, and `SameSite=Lax` unchanged.
- HS256 JWT, `JWT_SECRET` shared only by Gateway + Auth API.
- `api` keeps `:8000` internal and `web` is a separate nginx container (no Go SPA merge).

## Observability

- `slog` JSON on every proxied request: host, path, policy, jti, role, status, latency.
- Redis `INCR` + `EXPIRE 60s` per IP per minute; `429` + `Retry-After` when over.

## Test plan

- `TestLoadDefaultYAML` parses `deploy/gateway.yaml` and asserts storage and messenger
  hosts, public `/health` routes, and the storage/messenger/image/llm proxy timeouts.
- Unit: first-match route order, method lists, unknown host → 404, bad policy → load error.
- Unit: expired access + valid refresh → proxy after refresh; refresh 401 → 401 JSON / auth redirect.
- Unit: `rejected:jti` → 401; `rejected:user` / `global_role==BLOCKED` → 403.
- Unit: User-Service route strips Cookie/JWT and sets `X-User-Id`; storage 5xx → 503;
  storage 404 → 401.
- Unit: incoming `X-Auth-*` is stripped before proxy.
- Unit: rate limit 429 after N hits.
- Integration (podman): compose up, `curl` gateway `/healthz`, proxy `/` to web,
  `/oauth` to api.
- `curl` with the Host header for the storage.filenkov.store route reaches app:8000 (public `GET /health`, session catch-all) — the route is now live.
- `curl` with the Host header for the rag.filenkov.store route reaches rag-edge:8080 (public `GET /health`, catch-all session), and TestLoadDefaultYAML asserts the rag host, its 5m proxy timeout, and the return_to allowlist entry.

## Rollout

1. RAG-Chat stack up (so `rag_edge` and hostname `rag-edge` exist), then compose Auth (upsert gateway.yaml, restart api + gateway).
2. Point Caddy site blocks at the gateway (`:8083`).
3. Smoke: login, `/admin` (ADMIN), storage 30m upload, messenger attach, image 30m generate, Hermes chat, `GET /users` admin-only, `POST /users` → 405/404-ish reject.
4. Keep the old `cmd/gateway` behavior docs in git history; delete duplicate routing code paths.

## Config knobs (env)

| Env | Default | Notes |
|---|---|---|
| `GATEWAY_CONFIG` | `/etc/auth/gateway.yaml` | mounted YAML |
| `GATEWAY_HTTP_ADDR` | `:8083` | |
| `JWT_SECRET` | required | same as Auth API |
| `USER_SERVICE_URL` | `http://user_service:8000` | role lookups |
| `AUTH_API_URL` | `http://api:8000` | refresh only |
| `REDIS_URL` | `redis://redis:6379/0` | rejected + rate limit |
| `AUTH_DEV_HTTP` | `false` | clear Secure cookie in dev |

## Security notes

- Gateway is the only component that verifies access JWTs for proxying.
- Never forward `Authorization` / `Cookie` to User-Service.
- `return_to` must be https (prod) and in the YAML host allowlist.
- Hermes Bearer JWT is stripped after the plugin verifies it (plugin `next(c)`).
