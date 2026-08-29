# Change Request: Route `storage.filenkov.store` and give WebStorage identity + a stable `auth_session`

> **Status:** Implemented in Auth-Service (`cmd/gateway`, `cmd/api`, `deploy/gateway.yaml`). WebStorage still consumes these headers/cookies on its side.  
> **Repos:** Auth-Service only (`cmd/gateway`, `cmd/api`, `deploy/gateway.yaml`).  
> **Does not** reintroduce gRPC, Auth Postgres, or `WHITELIST_JSON`.  
> **Does** issue a companion opaque cookie `auth_session` for product services (private-vault session key). Edge auth stays access JWT + `rejected_cache`.
>
> **Later (hub list, now shipped):** this CR kept hub exact `/users` as `reject` so WebStorage would call User-Service on `user_network`. Hub **`GET /users`** for `ADMIN` is implemented in [`user-story-admin-directory.md`](user-story-admin-directory.md). **`POST /users` stays rejected.**

**Actor:** Auth Gateway (and Auth API for cookie issue), on behalf of a Google-authenticated person using HomeCloud / WebStorage.

**Goal:** Send browser traffic for `storage.filenkov.store` through this gateway as **one** upstream, inject trusted identity headers (including **storage** role and `X-User-Id`), keep a **stable** `auth_session` cookie for WebStorage’s private vault, and allow long uploads.

**Value:** WebStorage is business logic only. It must not talk gRPC, must not decide login URLs, and must not own TLS/routing.

---

## 1. Background

Today (`docs/auth-gateway.md`, `deploy/gateway.yaml`, `AGENTS.md`):

- Only `filenkov.store` is a live host. `storage.filenkov.store` is a **commented** split (`storage_api` / `storage_ui`).
- Product upstreams get `X-Auth-User-Id`, `X-Auth-Email`, `X-Auth-Name`, `X-Auth-Role` = JWT **`global_role`**.
- `X-User-Id` is set **only** on the User-Service upstream.
- YAML `role_field: storage` is accepted on load and **never applied** at request time.
- Cookies are `auth_access` + `auth_refresh`. `auth_session` was deleted on purpose.
- `timeouts.proxy` is **60s** (response header timeout) for every host.
- Hub exact `/users` was `auth: reject` in this CR so WebStorage would call User-Service on `user_network`, not via the public hub. **Shipped later:** hub `GET /users` is `auth: admin` ([`user-story-admin-directory.md`](user-story-admin-directory.md)); `POST /users` stays rejected.

WebStorage (after its own story) trusts gateway headers, calls User-Service only when a storage role header is missing, and keys the private vault Redis entry on cookie **`auth_session`**.

---

## 2. Assumptions

1. User-Service is already up on `user_network` (`http://user_service:8000`).
2. WebStorage compose attaches **`auth_network` as `external: true`** so the gateway can reach compose hostname **`app:8000`**. Do not publish WebStorage as a second public edge.
3. Host Caddy (already in front of this gateway) adds a site for `storage.filenkov.store` → `127.0.0.1:8083`. That Caddy change is **operator config**, not this repo’s `Caddyfile`.
4. User-Service `GET /users/{id}/roles/storage` exists (service key `storage`). Missing row → `200` `"role": "STRANGER"` (no insert).
5. No backward compatibility with the old gRPC / Redis-session-as-only-credential design.

---

## 3. Out of scope

- Changing hub `filenkov.store` routes, OAuth, `/api/me`, refresh, or logout **except** where needed to **issue/clear** `auth_session`.
- Exposing `GET /users` (collection) on the public hub **in this CR** (kept `match: /users` → `reject` then). **Superseded for GET (shipped):** [`user-story-admin-directory.md`](user-story-admin-directory.md) (`auth: admin`). `POST /users` remains `reject`.
- Forwarding JWTs to User-Service.
- Using hub `global_role` as WebStorage’s authorization (that is WebStorage’s rule). This CR still **injects** `X-Auth-Role` = `global_role` for other products, and **adds** `X-Storage-Role` for storage.

---

## 4. Enable one storage host

`deploy/gateway.yaml`:

```yaml
upstreams:
  # existing…
  storage: "http://app:8000"

timeouts:
  read_header: 15s
  idle: 120s
  proxy: 60s          # default for hub / others

hosts:
  storage.filenkov.store:
    role_field: storage
    # Per-host proxy header timeout (implement if missing). Uploads need this.
    timeouts:
      proxy: 30m
    routes:
      - match: /health
        upstream: storage
        auth: public
      - match: /*
        upstream: storage
        auth: session
```

**One upstream.** No API/UI split. JWT `aud` must include `storage.filenkov.store` (existing `JWTAudiences()` from YAML hosts). `return_to` allowlist must include that host.

Anonymous HTML on `/*` (except `/health`) → existing **302** `{PUBLIC_ORIGIN}/auth?return_to=…`. `/api/*` and XHR → **401** JSON. WebStorage will **not** redirect to OAuth itself.

---

## 5. Identity headers for the storage upstream

Before proxy, strip client-supplied `X-Auth-*`, `X-User-Id`, and **`X-Storage-Role`**.

When access JWT is valid, set on the **storage** upstream:

| Header | Value |
| --- | --- |
| `X-Auth-User-Id` | JWT `sub` (User-Service UUID) |
| `X-Auth-Email` | JWT `email` |
| `X-Auth-Name` | JWT `username` |
| `X-Auth-Role` | JWT `global_role` (hub snapshot; WebStorage **must ignore** this for gates) |
| `X-User-Id` | JWT `sub` (same UUID; WebStorage prefers this) |
| `X-Storage-Role` | User-Service **`storage`** role (`ADMIN` \| `FAMILY` \| `STRANGER` \| `BLOCKED`) |

**How to fill `X-Storage-Role`:** honor `role_field: storage`. Call `GET http://user_service:8000/users/{sub}/roles/storage` (network trust, no JWT). Use the JSON `role`.

- User-Service **5xx / timeout / network error** → **do not proxy**. Return **503** JSON (`{"error":"unavailable"}` or existing envelope). Do not invent a role.
- User **404** → **401** or **403** (user gone); do not proxy with a guessed role.
- **200** including implied `STRANGER` → inject and proxy.

Cache this lookup in the gateway if useful (short TTL, invalidate on `rejected:user:`). Correctness beats cache: a stale `ADMIN` after demotion is worse than an extra GET.

**Do not** put `X-Storage-Role` = `global_role`. Hub `ADMIN` is irrelevant to storage.

Other non–User-Service upstreams (`api`, `web`, `hermes`) stay as today (`X-Auth-*` only, no `X-Storage-Role` required).

---

## 6. Companion cookie `auth_session` (stable session id)

WebStorage private vault stores the user’s passphrase-derived key in Redis as `private_key:{session_id}` where `session_id` is the **`auth_session` cookie value**. Access JWT TTL is 15 minutes — **must not** be used as that key.

**Issue (Auth API, on successful OAuth callback and on refresh that already has a sid):**

| Cookie | Name | HttpOnly | SameSite | Secure | Path | Domain |
| --- | --- | --- | --- | --- | --- | --- |
| Opaque session id | `auth_session` | yes | `Lax` | same as JWT cookies | `/` | `COOKIE_DOMAIN` (`.filenkov.store`) |

- Value: new cryptographically random sid (e.g. 32+ bytes hex), **not** the access JWT, **not** the user UUID.
- Redis: `session:{sid}` → User-Service UUID. TTL = `JWT_REFRESH_TTL` (default 168h). This is **not** the edge credential.
- **Stable for the login lifetime.** Do not mint a new sid on every access-token refresh. Rotate only on hub `POST /api/logout` (delete Redis key, `MaxAge=-1` on the cookie) or when the user is revoked (`rejected:user:`).
- If a valid `auth_session` already exists on refresh, keep it and extend Redis TTL.

**Gateway → storage:** **forward** `Cookie` (do not strip) so `auth_session` reaches WebStorage. Continue **stripping** `Cookie` / `Authorization` on the User-Service upstream.

Edge authorization remains: validate `auth_access` + `rejected_cache`. Missing `auth_session` must **not** block proxy to storage (vault just stays locked). Missing access JWT still blocks `auth: session` routes.

---

## 7. Timeouts

Default `timeouts.proxy: 60s` stays for the hub. Storage host (or storage routes) must support **long bodies** (uploads). Implement **per-host** `timeouts.proxy` (30m) if the YAML schema has no override today. `read_header` can stay 15s (does not cap body).

---

## 8. Compose / network

Gateway already sits on `auth_network`. No new published ports. WebStorage `app` joins `auth_network` (external) so `http://app:8000` resolves. Document operator order: User-Service stack, Auth stack, Common (for WebStorage data), then WebStorage.

---

## 9. Acceptance criteria

Implemented in this repo (Auth-Service). WebStorage consumption of the headers/cookie is out of scope here.

1. `Host: storage.filenkov.store` + valid access JWT → proxied to `http://app:8000` with the six headers in §5. `X-Storage-Role` matches User-Service `GET …/roles/storage`, not `global_role`.
2. Client-supplied `X-User-Id` / `X-Storage-Role` / `X-Auth-*` are overwritten or stripped.
3. User-Service role GET fails → **503**, no proxy.
4. Unauthenticated browser navigation to `https://storage.filenkov.store/` → **302** to hub `/auth?return_to=`.
5. `GET /health` on the storage host is **public**.
6. After Google login, browser has `auth_session` **and** JWT cookies. The sid is unchanged across access refresh. Hub logout clears `auth_session`.
7. Storage upstream receives the `auth_session` cookie. User-Service upstream does not receive JWT cookies.
8. Upload-sized request to storage is not killed at 60s (30m per-host proxy timeout).
9. Public hub `POST /users` stays **rejected**. (`GET /users` for hub ADMIN is **shipped** in [`user-story-admin-directory.md`](user-story-admin-directory.md), not this CR.)
10. No gRPC, no Auth Postgres, no `WHITELIST_JSON`.

---

## 10. What to change in this repo (implementation checklist)

| Area | Change |
| --- | --- |
| `deploy/gateway.yaml` | Enable `storage.filenkov.store`, one upstream `storage`, `/health` public, per-host proxy 30m |
| `internal/gateway` | Apply `role_field`; inject `X-User-Id` + `X-Storage-Role` on storage; strip spoofed `X-Storage-Role`; optional cache; per-host timeouts |
| `cmd/api` + Redis | Issue/keep/clear `auth_session` + `session:{sid}` |
| `docs/auth-gateway.md`, `AGENTS.md` | Document storage host, new headers, companion cookie (edge auth is still JWT) |
