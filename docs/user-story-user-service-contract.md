# User Story: Auth-Service integrates with User-Service (JWT at Gateway, directory in User-Service)

> **Implemented** in Auth-Service: access/refresh JWT cookies, User-Service as the directory, gateway `:8083` / API unpublished `:8000`, no Auth Postgres or gRPC. This file is the original story. For the running system see `README.md`, `AGENTS.md`, and `docs/auth-gateway.md`.
>
> **Superseded (admin list):** Assumption 6, §10.4, and §13 said User-Service had no `GET /users` and the SPA must paste a UUID. User-Service now has `GET /users` (`{ users: […] }` + persisted `services[]`). Hub admin directory (list + email search, no UUID paste) is **implemented** in [`user-story-admin-directory.md`](user-story-admin-directory.md). That is an **Auth Gateway + Auth SPA** change only — not a User-Service CR. `POST /users` and `GET /users/by-google-sub/*` stay rejected at the hub. Do not restore `/api/admin`.

**Actor:** Auth Gateway + Auth API (Filenkov hub), on behalf of a Google-authenticated person and a global `ADMIN` operating the directory.

**Goal:** After Google OAuth, look up or create the person in User-Service, issue access + refresh JWTs in Auth only, and let the Gateway authorize other apps from those JWTs. Admin role changes go to User-Service over the same User-Service HTTP paths. User-Service never sees a JWT.

**Value:** Fast edge auth (signed JWT at the Gateway, no Auth Postgres round-trip) with a single users directory (User-Service UUID + roles). Fresh OAuth: no Auth-DB user migration.

---

## 1. Background — current vs target

### Current (Auth-Service today)

- Redis session cookie `auth_session` (`HttpOnly`, `SameSite=Lax`). Key `session:{sid}` → Auth Postgres user UUID. Also `user:sessions:{uuid}`.
- Auth Postgres `users` table (`internal/users`, `migrations/`): `id`, `google_sub`, `google_email`, `name`, `roles`, `storage_roles`, `resume_roles`, `last_seen_at`. Upsert on Google login (`ON CONFLICT (google_sub)`). First `ADMIN_EMAIL` insert becomes hub `ADMIN`.
- Auth API (`cmd/api`) owns OAuth, `/api/me`, `/api/logout`, **`GET /api/admin/users`**, **`PATCH /api/admin/users/{id}`**, `/health`, legacy gRPC `Validate` / `ListUsers`, whitelist.
- Gateway (`cmd/gateway`) loads the same Redis session + Auth Postgres user, injects `X-Auth-*`, routes via `deploy/gateway.yaml`.
- Compose network `auth`. Published: gateway `127.0.0.1:8083`, API `:8082` → container `8080`, web `:8081`.
- SPA (`web/src/api.ts`, `web/src/pages/Admin.tsx`): `credentials: 'include'`; admin calls `PATCH /api/admin/users/:id`.

**JWT search (Auth-Service docs + code):** there is **no** access/refresh JWT design, package, TODO, or ADR to align to. `docs/auth-gateway.md` lists JWT sessions as a **non-goal** (“Sessions stay Redis-backed; cookie `auth_session`”). Config has `COOKIE_NAME=auth_session` / `SESSION_TTL`, no `JWT_*`. This story **introduces** access + refresh JWTs in Auth (Gateway + API) and requalifies Redis as `rejected_cache` (plus remaining OAuth CSRF state and IP rate-limit keys).

### Target

| Concern | Owner |
| --- | --- |
| Google OAuth code flow, CSRF state | Auth API |
| User directory (UUID, `google_sub`, email, username, roles) | User-Service only |
| Access JWT + refresh JWT issue / validate / rotate | Auth API issues; Gateway validates access at the edge |
| Logout / block / role-change token rejection | Auth Redis `rejected_cache` |
| Admin role writes | Browser → Gateway → User-Service **same paths** (`PUT /users/{id}/role`, `PUT /users/{id}/roles/{service}`) |
| Current user for SPA | `GET /api/me` (Auth API calls User-Service, adapts JSON) |
| Fast auth for other upstreams | Gateway JWT → `X-Auth-*` (never JWT to User-Service) |

---

## 2. Assumptions

1. User-Service is already running on external compose network **`user_network`**, hostname **`user_service`**, listen **`:8000` unpublished**. Auth compose **attaches** that network; it does not create User-Service.
2. Trust to User-Service is **network isolation** (User-Service ADR-001 / architecture). No JWT, no API key, no mTLS in v1.
3. SPA stays same-origin behind the Gateway (`credentials: 'include'`). Cookies remain the browser credential.
4. Fresh OAuth: existing Auth Postgres rows are **not** migrated. People re-appear in User-Service on next Google login (`GET` by `google_sub` or `POST /users`).
5. Hub host remains `filenkov.store` (plus local aliases). Gateway YAML still has commented future hosts.
6. **Original v1 gap (superseded):** this story treated User-Service as having no `GET /users` collection. User-Service later added the list; hub exposure is **shipped** in [`user-story-admin-directory.md`](user-story-admin-directory.md). Role **updates** in this story remain the PUT paths.
7. Google userinfo used by Auth today: `sub`, `email`, `name` (`internal/oauth` → `openid`, `email`, `profile`). Lookup key is **`sub` only**.

---

## 3. Out of scope

- **No User-Service logic or API changes in this story** (no profile PATCH, no JWT endpoints there). List-users was out of scope **here**; User-Service later shipped `GET /users` independently. Hub admin must not invent a second list API in Auth.
- **No Auth Postgres / users DB**, no Auth migrations, no user-row upsert in Auth.
- **No gRPC** `Validate` / `ListUsers`, no whitelist product contract.
- **JWT must not travel to User-Service** (no `Authorization`, no access/refresh cookies forwarded on that proxy).
- No password login, no Auth-side role columns (`roles` / `storage_roles` / `resume_roles` on an Auth table).
- No migration of existing Auth users into User-Service.

---

## 4. User-Service identity model (quote the contract — Auth must follow)

Canonical sources: `User-Service/specs/contracts/openapi.yaml`, `specs/data-model.md`, `specs/decisions.md` ADR-002, `docs/architecture.md`.

> **Canonical id is UUID; login lookup is `google_sub`.** Primary key: UUID generated in User-Service (`gen_random_uuid()`). Unique federated id: `google_sub`. Subsequent logins: `GET /users/by-google-sub/{google_sub}` only. Email is stored, **not unique**, **not** a lookup key. Username is a unique handle (case-insensitive).

> OpenAPI info: “Lookup after OAuth is **by `google_sub` only**. Email is stored, not used for lookup.”

> `users.id`: RFC 4122 UUID. **Never** use `google_sub` as `id`.

> Create body **required:** `google_sub`, `email`, `username` (`CreateUserRequest`). `POST /users` is **insert-only**, **not** upsert. **Does not require `X-User-Id`.**

> Username pattern (OpenAPI `Username` / spec §8.2): `^[A-Za-z][A-Za-z0-9_]{2,31}$` (3–32 chars, **must start with a letter**). Unique `LOWER(username)`.

> `google_sub` pattern: `^[A-Za-z0-9._:-]+$`, 1–255, unique, case-sensitive. Google `sub` is typically **numeric**.

**Implication:** `google_sub` **must not** be sent as `username`. A numeric `sub` fails username validation (`400 VALIDATION_ERROR`). Docs do **not** allow `google_sub` as User-Service `id`.

### 4.1 Auth mapping: Google profile → `POST /users`

Google userinfo → User-Service fields:

| User-Service field | Source |
| --- | --- |
| `google_sub` | Google `sub` (exact) |
| `email` | Google `email`, Auth lowercases/trims before POST (User-Service also stores lowercased) |
| `username` | **Derived handle** (below). Not Google `name` as-is (spaces/dots illegal). Not `google_sub`. |
| `id` | **Not sent.** Assigned by User-Service on `201`. |
| `global_role` | **Not sent.** First user in empty table → `ADMIN`; later → `STRANGER` (User-Service bootstrap). Drop Auth `ADMIN_EMAIL`. |

**Username algorithm** (email local-part + collision retry):

1. Take the email local-part (before `@`). Strip a Gmail-style `+tag` (`ada+foo` → `ada`).
2. Keep only `[A-Za-z0-9_]`. If the result is empty, use `user`.
3. If it does not start with `[A-Za-z]`, prefix `u_`.
4. If length &lt; 3, pad on the right with `x` until length 3.
5. Truncate to 32 characters. Result must still match `^[A-Za-z][A-Za-z0-9_]{2,31}$`.
6. `POST /users` with that `username`.
7. On **`409 USERNAME_EXISTS`**: append `_` + attempt number (`_2`, `_3`, …), re-truncate to 32 from the left, retry POST (max **8** attempts). Do **not** treat this as “user already exists”.
8. On **`409 GOOGLE_SUB_EXISTS`**: stop creating; `GET /users/by-google-sub/{google_sub}` and use that user (callback race).
9. If all username retries fail → fail the login (`503`/`500` JSON); do not invent a User-Service API.

Optional seed if step 2 is empty: first token of Google `name` run through the same sanitizer, then the same retries.

JWT `sub` claim after login = User-Service **`id`** (UUID), never `google_sub`.

---

## 5. User-Service contract Auth MUST use

Base URL from Auth containers: **`http://user_service:8000`** (human + User-Service architecture hostname). OpenAPI `servers` also shows `http://users:8000` as an example; **do not** invent a new User-Service hostname — use **`user_service`**.

JSON `Content-Type: application/json`. User-Service has **no CORS** (not a browser origin API); the **browser talks to the Gateway**, not to `:8000`.

### 5.1 Error envelope (all 4xx/5xx JSON)

```json
{
  "error": {
    "code": "NOT_FOUND",
    "message": "User not found"
  }
}
```

`error.code` enum: `VALIDATION_ERROR`, `INVALID_JSON`, `UNAUTHORIZED`, `FORBIDDEN`, `ACTOR_NOT_FOUND`, `NOT_FOUND`, `GOOGLE_SUB_EXISTS`, `USERNAME_EXISTS`, `LAST_ADMIN`, `INVALID_ROLE`, `INVALID_SERVICE`, `NOT_READY`, `INTERNAL_ERROR`.

### 5.2 Role enum (exact strings)

`ADMIN` | `FAMILY` | `STRANGER` | `BLOCKED`

Wrong case / aliases → **422** `INVALID_ROLE` on writes.

### 5.3 `POST /users` — create after first Google login

- **Auth:** none (`X-User-Id` not required). **Privileged** on `user_network` (first insert → global `ADMIN`). **Must not** be reachable from the public Gateway.
- **Body:** `{ "google_sub": "...", "email": "...", "username": "..." }` — all required; `additionalProperties: false`.
- **201:** User JSON + `Location: /users/{id}`.
- **400:** `VALIDATION_ERROR` / `INVALID_JSON`.
- **409:** `GOOGLE_SUB_EXISTS` or `USERNAME_EXISTS`.
- **500:** `INTERNAL_ERROR`.

**201 User JSON:**

```json
{
  "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "google_sub": "110000000000000000001",
  "email": "ada@example.com",
  "username": "ada",
  "global_role": "ADMIN",
  "created_at": "2026-08-29T11:00:00Z",
  "updated_at": "2026-08-29T11:00:00Z"
}
```

### 5.4 `GET /users/by-google-sub/{google_sub}` — subsequent logins

- URL-encode the path segment. Lookup key is `google_sub` **only**.
- **200:** same User JSON as get-by-id.
- **400:** bad `google_sub` charset.
- **404:** `NOT_FOUND`.
- **Public Gateway must not expose this path.** Auth API calls it on `user_network` only.

### 5.5 `GET /users/{id}` — by UUID

- **200:** User JSON (includes `global_role`).
- **400:** `{id}` not a UUID → `VALIDATION_ERROR`.
- **404:** `NOT_FOUND`.
- No `X-User-Id` required (User-Service). Gateway still requires a valid access JWT before proxying to the browser.

### 5.6 `GET /users/{id}/roles`

**200:**

```json
{
  "user_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "global_role": "ADMIN",
  "services": [
    { "service": "auth", "role": "FAMILY" }
  ]
}
```

`services` = **persisted rows only** (empty array if none). Missing services are not implied here.

### 5.7 `GET /users/{id}/roles/{service}`

- Valid service + no row → **200** `{ "user_id", "service", "role": "STRANGER" }` **without insert**.
- Invalid service charset `^[a-z][a-z0-9_-]{0,62}$` → **400** `INVALID_SERVICE`.
- Unknown user → **404**.

Locked service keys for the current SPA columns (valid charset; replace Auth DB columns):

| Former Auth column | User-Service `{service}` |
| --- | --- |
| hub `roles` | **global** `PUT /users/{id}/role` — not a service row |
| `storage_roles` | `storage` |
| `resume_roles` | `resume` |

### 5.8 `PUT /users/{id}/role` — global role (admin)

- **Requires `X-User-Id`** (actor UUID). Gateway **overwrites** this from the access JWT `sub`. **Never** forward the client’s `X-User-Id`.
- Actor **global** `ADMIN` only. Service-only ADMIN is **not** enough.
- Body: `{ "role": "BLOCKED" }` (or `ADMIN` / `FAMILY` / `STRANGER`).
- **200:** `{ "user_id": "...", "role": "FAMILY" }`.
- **401** missing header `UNAUTHORIZED`; **400** malformed UUID `VALIDATION_ERROR`; **403** `FORBIDDEN` or `ACTOR_NOT_FOUND`; **404** target; **409** `LAST_ADMIN`; **422** `INVALID_ROLE`.

User-Service **ignores role headers**. Do not send `X-Auth-Role` as authorization.

### 5.9 `PUT /users/{id}/roles/{service}` — per-service role

- **Requires `X-User-Id`.** Actor global `ADMIN` **or** `ADMIN` on **that same** `{service}`.
- Body: `{ "role": "FAMILY" }`.
- **200:** `{ "user_id", "service", "role" }`.
- Same 400/401/403/404/422 matrix; invalid `{service}` → **400** `INVALID_SERVICE`.

### 5.10 Login orchestration (Auth API → User-Service, insert-only + 409)

On Google callback, after userinfo (`sub`, `email`):

1. `GET http://user_service:8000/users/by-google-sub/{google_sub}` (encode path).
2. **200** → use that User (`id`, `global_role`, …). Skip POST.
3. **404** → `POST /users` with mapped body (§4.1).
4. **201** → use created User.
5. **409 `GOOGLE_SUB_EXISTS`** → `GET` by `google_sub` again; proceed with that User.
6. **409 `USERNAME_EXISTS`** → bump username (§4.1) and POST again.
7. **400/500** → fail login; do not issue JWTs.
8. If `global_role == BLOCKED` → **do not issue tokens**; **403** `{ "error": "blocked" }` (User-Service still returns the row; Auth decides tokens — User-Service spec §3).

Auth API uses this path **directly** on `user_network`. No JWT on these calls. No `X-User-Id` on POST/GET lookup.

---

## 6. JWT access + refresh (Auth only — new design)

**Not present in Auth today.** Specify as follows. Signing and cookies stay in Gateway + Auth API. User-Service stores **no** JWT columns.

### 6.1 Transport (explicit choice)

SPA already uses **HttpOnly cookies** (`credentials: 'include'`, cookie `auth_session`). **Keep cookies; do not put JWTs in `localStorage` or `Authorization` for the browser.**

| Cookie | Name | HttpOnly | SameSite | Secure | Path | Domain |
| --- | --- | --- | --- | --- | --- | --- |
| Access JWT | `auth_access` | yes | `Lax` | yes unless `AUTH_DEV_HTTP=true` | `/` | `COOKIE_DOMAIN` (default `.filenkov.store`; empty in local HTTP) |
| Refresh JWT | `auth_refresh` | yes | `Lax` | same | `/` | same |

**Delete** cookie `auth_session` (`COOKIE_NAME`). Do not send these cookies to User-Service (Gateway strips `Cookie` / `Authorization` on that upstream).

### 6.2 Signing and claims

- **Alg:** `HS256`.
- **Secret:** `JWT_SECRET` (required, ≥32 random bytes). Shared by **Gateway and Auth API** only. Not sent to User-Service.
- **iss:** `PUBLIC_ORIGIN` (e.g. `https://filenkov.store`).
- **aud:** public hub host (`filenkov.store`; include YAML aliases such as `localhost` when `AUTH_DEV_HTTP`).

**Access JWT**

| Claim | Value |
| --- | --- |
| `sub` | User-Service UUID `id` |
| `jti` | New UUID per access token |
| `typ` | `"access"` |
| `global_role` | Snapshot from User-Service at issue/refresh time (`ADMIN`/`FAMILY`/`STRANGER`/`BLOCKED`) |
| `email` | User-Service `email` |
| `username` | User-Service `username` |
| `iat` / `nbf` / `exp` | unix seconds |

**Do not** put `google_sub` in the access token (not needed at the edge). **Do not** embed the full `services[]` map (keeps tokens small). Hub `role_field` is **`global_role`**.

**Refresh JWT:** `sub`, `jti`, `typ: "refresh"`, `iss`, `aud`, `iat`, `exp` only. **No roles.** Rotation always re-reads User-Service.

### 6.3 TTLs

| Token | TTL | Env default |
| --- | --- | --- |
| Access | **15 minutes** | `JWT_ACCESS_TTL=15m` |
| Refresh | **168h** (same as today’s `SESSION_TTL`) | `JWT_REFRESH_TTL=168h` |

### 6.4 Refresh endpoint

`POST /api/refresh` on **Auth API**.

- Gateway route: **`auth: public`** (access cookie may already be expired; API reads `auth_refresh`).
- Validate refresh signature, `typ`, `exp`, `aud`/`iss`; reject if `jti` or `sub` is in `rejected_cache`.
- `GET http://user_service:8000/users/{sub}`. If **404** or `global_role == BLOCKED` → reject refresh `jti`, clear both cookies, **403** `blocked` or **401**.
- **Rotate refresh:** new refresh `jti`; put **old** refresh `jti` in `rejected_cache` with TTL = remaining old refresh TTL (or full refresh TTL).
- Issue new access JWT (fresh `global_role` / email / username from User-Service). Set both cookies. **200** `{ "status": "ok" }` (no token in JSON body).

SPA: if `GET /api/me` returns **401**, `POST /api/refresh` then retry `/api/me`. Do not expose token strings to JS.

### 6.5 Redis `rejected_cache`

Requalify Auth Redis: **not** a session store. Same `redis:7-alpine` instance may still hold `oauth:state:*` and rate-limit keys (different prefixes).

| Key | Value | When | TTL |
| --- | --- | --- | --- |
| `rejected:jti:{jti}` | `1` | Logout (access **and** refresh `jti`); refresh rotation (old refresh `jti`) | Remaining lifetime of that token (≥1s) |
| `rejected:user:{uuid}` | unix seconds of revocation | Successful **global** role PUT (and service PUT that should kill hub access, including `BLOCKED`) for **that target user** | `JWT_REFRESH_TTL` |

Gateway rejects an access JWT if:

1. Signature / `typ` / `exp` / `aud` fail, or
2. `rejected:jti:{jti}` exists, or
3. `rejected:user:{sub}` exists **and** token `iat` ≤ that timestamp (or simply: key exists — simpler; TTL covers the window).

**Why `rejected:user:`:** logout has the caller’s cookies (`jti`s). An admin blocking **someone else** does not have their `jti`s. Embedding `global_role` in access tokens requires invalidating that user’s tokens on role-change (human: reject on role-change if tokens embed roles).

**Logout** `POST /api/logout`:

- Gateway: `auth: public` (refresh-only cookies must still work).
- If access and/or refresh cookies present and parse: write `rejected:jti:{jti}` for each.
- Also write `rejected:user:{sub}` (covers stolen copies).
- Clear `auth_access` and `auth_refresh` (`MaxAge=-1`).
- **200** `{ "status": "ok" }` even if cookies were already missing.

### 6.6 BLOCKED (Gateway must not issue / must reject)

- **Issue time (Auth API):** OAuth callback and `POST /api/refresh` — User-Service `global_role == BLOCKED` → no new tokens.
- **Edge (Gateway):** access `global_role == BLOCKED` **or** user in `rejected_cache` → same as today: **403** `{ "error": "blocked" }` on non-`public` routes. `public` hub routes still proxy (today: BLOCKED can still hit `/`).
- User-Service does **not** filter BLOCKED from GET; Auth owns token policy.

### 6.7 Fast auth vs user info

- **Fast auth = JWT at Gateway:** validate access cookie + `rejected_cache`; inject `X-Auth-*` from **access claims** (no User-Service hop for Hermes / SPA HTML).
- **User info = User-Service:** `/api/me` and directory GETs/PUTs.

---

## 7. Auth Gateway responsibilities

Process: `cmd/gateway`. Stop using Auth Postgres and `session:{sid}`. Validate **access JWT** + `rejected_cache`. Keep YAML router, IP rate limit, security headers, `/healthz`.

### 7.1 Edge auth (other upstreams) vs User-Service proxy

**All proxied requests:** strip incoming `X-Auth-*` and incoming `X-User-Id` (clients must not spoof).

**Non–User-Service upstreams** (`api`, `web`, `hermes`, future apps) when access JWT is valid:

| Header | Source |
| --- | --- |
| `X-Auth-User-Id` | JWT `sub` (User-Service UUID) |
| `X-Auth-Email` | JWT `email` |
| `X-Auth-Name` | JWT `username` (User-Service has no `name` field) |
| `X-Auth-Role` | For hub: JWT `global_role`. YAML `role_field` becomes `global_role` (replace `roles`). Future host: service name → optional later `GET /users/{id}/roles/{service}`; **v1 hub does not call User-Service on every Hermes request.** |

**User-Service upstream only:**

1. **Do not forward** `Cookie`, `Authorization`, or JWT query params.
2. **Do not** send JWTs in any header.
3. Set **`X-User-Id`** = access JWT `sub` (UUID) on **every** proxied User-Service request (GETs ignore it; PUTs require it). Overwrite; never pass the client value.
4. Do **not** treat `X-Auth-Role` as User-Service authorization (User-Service loads actor roles from its DB/cache).

After a **200** on `PUT /users/{id}/role` or `PUT /users/{id}/roles/{service}`, Gateway writes `rejected:user:{id}` for the **path** `{id}` (target user), so stale embedded `global_role` cannot authorize that person. (Actor’s own tokens stay valid unless they blocked themselves.)

### 7.2 YAML routing (target)

Upstreams (compose DNS):

```yaml
upstreams:
  api: "http://api:8000"          # Auth API — internal only
  web: "http://web:80"
  user_service: "http://user_service:8000"
  hermes: "http://host.docker.internal:8787"
```

`filenkov.store` routes (**first match wins**; more specific prefixes first):

| match | upstream | auth | Notes |
| --- | --- | --- | --- |
| `/oauth/*` | `api` | `public` | Google start + callback |
| `/api/refresh` | `api` | `public` | Refresh cookie only |
| `/api/logout` | `api` | `public` | Reject jtis + clear cookies |
| `/api/me` | `api` | `session` | Access JWT required |
| `/api/*` | `api` | `session` | No other `/api` routes remain; keep as deny-by-default if something is added |
| `/users/by-google-sub/*` | — | **reject** | **404** at Gateway. Not a public path. |
| `POST /users` exact | — | **reject** | **404**. Create is Auth API → `user_network` only. |
| `GET /users` exact | `user_service` | **admin** | Hub ADMIN directory list. See [`user-story-admin-directory.md`](user-story-admin-directory.md). |
| `/users/*` | `user_service` | `session` | GET/PUT by UUID and roles. Strip cookies; set `X-User-Id`. |
| `/health` | `api` | `public` | Auth API liveness/readiness (Redis) |
| `/hermes/*` | `hermes` | `admin` | `strip_prefix: /hermes`; `X-Auth-*` |
| `/admin`, `/admin/*` | `web` | `admin` | `global_role == ADMIN` |
| `/*` | `web` | `public` | SPA |

**Drop** `/api/admin/*` routes entirely.

**Shipped:** exact `GET /users` is `auth: admin` → `user_service`. `POST /users` stays reject. See [`user-story-admin-directory.md`](user-story-admin-directory.md).

`auth: session` = valid access JWT, not rejected, `global_role != BLOCKED`.  
`auth: admin` = session + `global_role == ADMIN`.  
Anonymous on session/admin: same as today (HTML **302** `/auth?return_to=`, API/JSON **401** `{ "error": "unauthorized" }`).

Gateway listen remains **`:8083`**. Process **`GET /healthz`** is local (not proxied).

### 7.3 BLOCKED and token issue

Gateway **does not mint** JWTs. It **rejects** BLOCKED access tokens on non-public routes. Auth API must not set cookies when User-Service says `BLOCKED`.

---

## 8. Auth API responsibilities (`cmd/api`, container **`:8000`**, **not published**)

Remaining HTTP surface:

| Method | Path | Gateway auth | Behavior |
| --- | --- | --- | --- |
| `GET` | `/oauth/google?return_to=` | public | CSRF state in Redis `oauth:state:{state}` (TTL 10m, keep). Redirect to Google. |
| `GET` | `/oauth/google/callback` | public | Consume state; exchange code; User-Service GET/POST (§5.10); if not BLOCKED, set `auth_access` + `auth_refresh`; **302** `return_to`. |
| `GET` | `/api/me` | session | See §8.1 |
| `POST` | `/api/logout` | public | §6.5 |
| `POST` | `/api/refresh` | public | §6.4 |
| `GET` | `/health` | public | Ping Redis (and optionally `http://user_service:8000/readyz`). **No Postgres.** |

**Delete** `GET /api/admin/users`, `PATCH /api/admin/users/{id}`, gRPC, whitelist persist, Auth user store.

OAuth `return_to` allowlist: still gateway YAML hosts + `PUBLIC_ORIGIN` (+ loopback if `AUTH_DEV_HTTP`).

### 8.1 `GET /api/me` — Auth calls User-Service, adapts for SPA

1. Parse access JWT (Gateway already required it; API re-validates + `rejected_cache` as defense in depth).
2. `GET http://user_service:8000/users/{id}`.
3. `GET http://user_service:8000/users/{id}/roles`.
4. If user missing → **401**. If `global_role == BLOCKED` → **403** `{ "error": "blocked" }`.

**Response JSON (SPA contract after this story):**

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

- `services` is the User-Service roles document array (persisted rows only). Implied `STRANGER` for `storage`/`resume` if omitted — SPA treats missing service as `STRANGER` (same as User-Service GET-one-service).
- **Dropped vs today:** `google_email` (use `email`), `name` (use `username`; User-Service does not store Google display name), `roles` (use `global_role`), `storage_roles` / `resume_roles` (use `services[]`), `last_seen_at` (User-Service has no last-seen).

**401** if anonymous. JSON error shape for Auth API may stay `{ "error": "<string>" }` for Gateway-facing SPA calls; User-Service envelope `{ "error": { "code", "message" } }` is **proxied unchanged** on `/users/*`.

---

## 9. Compose target

**Networks**

```yaml
networks:
  auth_network:
    driver: bridge
  user_network:
    external: true
```

Create **new** `auth_network`. **Attach existing** `user_network` (do not create it here). User-Service compose must already define `user_network`.

**Services (exact names)**

| Service | Image / build | Networks | Ports | Role |
| --- | --- | --- | --- | --- |
| `web` | existing Vite → nginx (`deploy`/`web/Dockerfile`) | `auth_network` only | **none published** (drop `8081`) | SPA, container `:80` |
| `redis` | `redis:7-alpine` | `auth_network` only | none | `rejected_cache` + OAuth state + rate limits. `redis-server --save "" --appendonly no` |
| `api` | `deploy/Dockerfile.api` | `auth_network` **and** `user_network` | **none published** (drop `8082`) | `HTTP_ADDR=:8000`. No gRPC. No `DATABASE_URL`. `REDIS_URL=redis://redis:6379/0`. `USER_SERVICE_URL=http://user_service:8000` |
| `gateway` | `deploy/Dockerfile.gateway` | `auth_network` **and** `user_network` | **`127.0.0.1:8083:8083`** only | YAML listen `:8083`. Redis + `JWT_SECRET`. **No Postgres.** `extra_hosts: host.docker.internal:host-gateway` |

**Drop** compose service `postgres`, volume `postgres_data`, API/gateway `DATABASE_URL` / `MIGRATIONS_PATH`, published `API_HTTP_PORT` / `WEB_HTTP_PORT`.

`api` depends_on: `redis` healthy. `gateway` depends_on: `redis`, `api` healthy, `web` started. User-Service is **external**; if `user_service` is down, OAuth callback and `/api/me` fail (document operator order: start User-Service stack first).

Healthchecks: `api` `GET http://127.0.0.1:8000/health`; `gateway` `GET http://127.0.0.1:8083/healthz`.

---

## 10. Frontend impact

Files: `web/src/api.ts`, `web/src/pages/Admin.tsx`, `web/src/types.ts`, `web/src/auth.tsx`, `web/src/pages/Auth.tsx`.

1. **Drop** `fetchAdminUsers()` → `GET /api/admin/users` and `patchUser()` → `PATCH /api/admin/users/:id`.
2. **Role updates** (same User-Service paths, same origin via Gateway):

   - Global: `PUT /users/{id}/role` body `{ "role": "FAMILY" }`.
   - Service: `PUT /users/{id}/roles/storage` and `PUT /users/{id}/roles/resume` body `{ "role": "FAMILY" }`.
   - `credentials: 'include'`. Do not send `X-User-Id` (Gateway overwrites).

3. **Types:** `User` matches `/api/me` (§8.1). `isAdmin` → `global_role === 'ADMIN'`. Display **`username`** instead of `name`; **`email`** instead of `google_email`.
4. **Admin table of all users (superseded):** this story used UUID lookup because User-Service had no list yet. **Shipped instead:** [`user-story-admin-directory.md`](user-story-admin-directory.md) — `GET /users` via gateway `auth: admin`, SPA table + email search, no UUID paste. Do **not** call Auth `/api/admin`.
5. **Auth page / session copy:** cookies are JWTs, not Redis `auth_session`. `POST /api/refresh` on 401 from `/api/me`.
6. Parse User-Service error envelope when talking to `/users/*`: `body.error.message` or `body.error.code` (today `readError` only reads `error` as a string).

---

## 11. What to DELETE from Auth-Service

| Area | Delete / stop using |
| --- | --- |
| Postgres | compose `postgres`, `postgres_data`, `DATABASE_URL`, golang-migrate, `migrations/` (`000001` users enums/table, etc.) |
| Auth users store | `internal/users` Postgres repository (`UpsertFromGoogle`, `List`, `UpdateRoles`, `TouchLastSeen`, role columns) |
| Sessions | `internal/session`, Redis `session:{sid}`, `user:sessions:{uuid}`, cookie `auth_session`, `COOKIE_NAME`, `SESSION_TTL` as session TTL |
| gRPC | `cmd/api` gRPC server, `internal/grpc`, `proto/`, `pkg/authpb`, `GRPC_ADDR`, compose `expose: 9090` |
| Whitelist | `internal/whitelist`, `WHITELIST_JSON`, Redis key `whitelist` |
| Admin HTTP | `GET /api/admin/users`, `PATCH /api/admin/users/{id}`, `handleAdminList` / `handleAdminPatch`, gateway `/api/admin/*` |
| Publish | host `8081` (web), `8082` (API). Keep **only** `127.0.0.1:8083` |
| Bootstrap | `ADMIN_EMAIL` (User-Service first-user `ADMIN`) |
| Gateway | Postgres user lookup, `users.Repository` in `cmd/gateway` |

Keep: OAuth (`internal/oauth` + Redis state), Gateway YAML router / proxy / rate limit / headers, Vite `web/`, Redis container (new role).

---

## 12. Acceptance criteria (testable)

1. **No JWT on User-Service:** tcp/HTTP traces (or unit tests of the User-Service proxy director) show no `Authorization`, no `auth_access`/`auth_refresh` cookies, and no JWT body to `user_service:8000`. `X-User-Id` is the UUID from the access `sub`, overwritten by Gateway.
2. **First Google login:** unknown `sub` → Auth `POST /users` → `201` → cookies `auth_access` + `auth_refresh` set → `GET /api/me` returns User-Service `id` (UUID) and `google_sub`. First directory user has `global_role: ADMIN`.
3. **Subsequent login:** `GET /users/by-google-sub/{sub}` 200 → **no** POST (or POST 409 `GOOGLE_SUB_EXISTS` then GET) → same UUID. Email/username in User-Service **unchanged** (insert-only).
4. **Username collision:** two Google accounts whose derived local-parts clash → second login retries username and still `201` or succeeds after `USERNAME_EXISTS`.
5. **Numeric `google_sub` is not `username` and not `id`.**
6. **BLOCKED:** User-Service `global_role: BLOCKED` → callback does not set cookies (403). Existing access JWT with `global_role: BLOCKED` or `rejected:user:` after admin PUT → Gateway **403** on `/admin` and `/api/me`; public `/` still served.
7. **Logout:** `POST /api/logout` clears both cookies; reuse of old access `jti` is **401** at Gateway; refresh `jti` cannot mint a new access.
8. **Refresh:** after access TTL, `POST /api/refresh` with valid refresh cookie sets a new access cookie; User-Service `global_role` is re-read (demotion/block takes effect on refresh even without `rejected:user` if that write was missed).
9. **Admin PUT:** from SPA, `PUT /users/{uuid}/role` with `{ "role": "FAMILY" }` reaches User-Service; actor is Gateway `X-User-Id`; **200** User-Service `GlobalRole` JSON. Non-admin actor → User-Service **403**. Last ADMIN demotion → **409** `LAST_ADMIN`.
10. **Service PUT:** `PUT /users/{uuid}/roles/storage` works for global ADMIN; Auth **has no** `/api/admin` routes (**404**).
11. **Compose:** `auth_network` exists; `user_network` is `external: true`; `web`/`redis` only on `auth_network`; `api`/`gateway` on both; only published port is **8083** on gateway; `api` listens **8000** unpublished; no Auth `postgres` service.
12. **gRPC / whitelist / Auth migrations / `session:` keys** are gone (build has no proto server; Redis has no `session:` after login).
13. **SPA** uses `/users/...` for role writes and `/api/me` shape in §8.1; no `PATCH /api/admin`.

---

## 13. Open risks

**Superseded:** User-Service now has list-users (`GET /users` collection). Hub UUID-only admin is replaced by the **shipped** [`user-story-admin-directory.md`](user-story-admin-directory.md) (Auth Gateway `GET /users` `auth: admin` + SPA table). Do not add a fake list in Auth and do not restore `/api/admin`.

No other User-Service field gaps block this story: `id` (UUID), `google_sub`, `email`, `username`, `global_role`, and service roles cover login, `/api/me`, and admin updates. Google display `name` and `last_seen_at` are **not** in User-Service v1 — `/api/me` uses `username` and omits last-seen.
)
