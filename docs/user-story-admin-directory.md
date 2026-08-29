# Change Request: Hub admin directory list (Auth Gateway + Auth SPA)

> **Status:** Implemented in Auth-Service (`deploy/gateway.yaml`, `web/src/pages/Admin.tsx`, `web/src/api.ts`, `web/src/types.ts`, gateway + SPA tests).  
> **Repos:** Auth-Service only. **Does not** change User-Service (no CR, no new User-Service API). `GET /users` already existed.  
> **Does not** reintroduce Auth Postgres, gRPC `ListUsers`, `WHITELIST_JSON`, or `/api/admin`.  
> **Operator:** redeploy **gateway + SPA**. No User-Service deploy. No Auth DB migration.

**Actor:** Hub `global_role == ADMIN` on `filenkov.store/admin`, via Auth Gateway → User-Service.

**Goal:** Show every directory person on `/admin` and let that admin change global and service roles. Load the list once. Search by email in the browser. No UUID paste.

**Value:** User-Service already returns the full table (`GET /users`). WebStorage already consumes it on `user_network`. Before this story the hub Auth Gateway **404d** exact `GET /users` even for `ADMIN`, so the SPA was stuck on UUID lookup. Opening the existing list for hub ADMIN is an Auth-only change.

## What shipped

**Gateway** (`deploy/gateway.yaml`, first match wins):

| Request | Policy | Upstream |
| --- | --- | --- |
| `GET /users` | `admin` | `user_service` (`preserve_host: false`; cookies stripped; `X-User-Id` from JWT `sub`) |
| `POST /users` | `reject` (404) | none |
| `GET /users/by-google-sub/*` | `reject` (404) | none |
| `/users/*` | `session` | `user_service` (`GET /users/{id}`, role PUTs) |

Anonymous `GET /users` → **401** JSON `{"error":"unauthorized"}`. Signed-in non-ADMIN → **403** `forbidden`. `BLOCKED` / `rejected_cache` → **403** `blocked`. `GET /users` does **not** write `rejected:user:`.

**SPA** (`web/src/pages/Admin.tsx`, `web/src/api.ts` `fetchUsers`):

| State | UI |
| --- | --- |
| Loading list | `Loading directory…` |
| `GET /users` fails | Error banner (`readError`); no table |
| `users: []` | `No one is in the directory yet.` |
| Search matches nobody | `No one matches that email.` |
| Rows | Full table (email substring filter only) |

- One `GET /users` on mount. No UUID paste. No `/api/admin`. `credentials: 'include'`; SPA does not send `X-User-Id`.
- Always columns: username, email, global, storage, resume. Extra persisted `services[].service` keys become extra columns (union of the loaded list, sorted). Missing services render as `STRANGER` (`serviceRole()`). Instant `PUT` on select; local row merge (`withGlobalRole` / `withServiceRole`); no full-list refetch. Self-edit enabled. **409** `LAST_ADMIN` shows User-Service `message` and leaves the select on the previous value.
- `fetchUser` (`GET /users/{id}` + `/roles`) remains in `api.ts` for optional per-row refresh; **Admin does not call it**.

**Tests:** `go test ./...` — `TestHubUsersListAdminOnly`, `TestHubUsersListProxiesWithXUserId`, `TestLoadDefaultYAML` (`MatchMethod` GET `/users` → `admin`), `TestRouterFirstMatch`, `TestUserServiceCreateAndGoogleSubRejected`. `cd web && npm test` — `Admin.test.tsx` (list, email-only search, empty vs miss, errors, PUTs, Hermes column, LAST_ADMIN, self-edit) and `api.test.ts` (`fetchUsers` credentials / no `X-User-Id` / 401 retry).

**Deltas from the original plan:** PUT success updates local state (no follow-up GET). `TestHubUsersStillRejected` was replaced (not renamed in place). Extra service column titles are first-letter capitalized (`Hermes`). Search input is `type="search"` labeled Email.

Sections below are the original locked spec. **Before this story** describes the old UUID-paste SPA and `auth: reject` on exact `/users`.

---

## 1. Background

Before this story (`AGENTS.md`, `README.md` Admin section, `docs/auth-gateway.md`, `deploy/gateway.yaml`):

- `/admin` is `auth: admin` (`global_role == ADMIN`). Unchanged in this story.
- Exact `match: /users` is `auth: reject` (all methods) → **404**, never proxied. That hides `POST /users` (Auth API on `user_network` only) **and** `GET /users` (the list).
- `match: /users/*` proxies to `user_service` with `auth: session` (`GET /users/{id}`, role PUTs).
- `match: /users/by-google-sub/*` is `auth: reject`.
- SPA (`web/src/pages/Admin.tsx`): paste UUID → `GET /users/{id}` + `GET /users/{id}/roles` → table of one row. Copy says “There is no directory list.” Instant `PUT` on `<select>` change. Fixed columns: global, storage, resume.
- `web/src/api.ts` has `fetchUser`, `putGlobalRole`, `putServiceRole`. No list helper. Do not call `/api/admin`.
- `serviceRole()` in `web/src/types.ts` treats a missing service row as **`STRANGER`**.

User-Service (existing contract — **do not change**):

- `GET /users` → **200** `{ "users": UserListItem[] }`. Empty table → `"users": []`. Order `created_at`, then `id`. No pagination, no query filters, no search.
- Each item is User fields (`id`, `google_sub`, `email`, `username`, `global_role`, timestamps) plus **`services`**: persisted rows only (same as `GET /users/{id}/roles`). Missing services are **not** implied in JSON.
- Does **not** require `X-User-Id` (network trust on `user_network`). User-Service does not authorize the list; the **gateway** is the public ACL.
- Canonical sources: `User-Service/specs/spec.md` §4.2, `User-Service/specs/contracts/openapi.yaml` `operationId: listUsers`, `User-Service/docs/api.md`.

User-Service docs still say the public hub must reject `GET /users`. That meant “do not expose create/list anonymously / do not proxy `POST /users`.” This story opens **`GET /users` for hub `ADMIN` only**. No User-Service spec change.

---

## 2. Assumptions

1. User-Service `GET /users` is already implemented and returns `{ users: [...] }` with persisted `services[]`.
2. Gateway YAML already supports optional `methods:` on a route (`internal/gateway/config.go` `Route.Methods`; first match still wins). `match: /users/*` **also matches** exact `/users` — so `GET /users` must be an earlier, method-specific route with `auth: admin`, or FAMILY/STRANGER would list everyone via the session prefix.
3. SPA stays same-origin (`credentials: 'include'`). Cookies remain the browser credential. The SPA does not send `X-User-Id`.
4. Hub `/admin` stays `auth: admin`. Service-only `ADMIN` (e.g. storage ADMIN, global FAMILY) cannot use this page.
5. Family-scale directory: fetching the full table once is acceptable. No server pagination.

---

## 3. Out of scope / non-goals

- **No User-Service CR.** No new User-Service endpoints, no pagination, no server-side search, no extra query params on `GET /users`.
- **No server pagination or server search** at the gateway either. One `GET /users`; filter in the browser.
- **No filters** (no role chips, no service dropdown, no “blocked only”).
- **No confirmation modal.** Instant apply on select (keep current `Admin.tsx` behavior).
- **No UUID paste fallback.** Remove the lookup form entirely. List only.
- **No `/api/admin`.** Do not restore Auth API admin routes or gRPC `ListUsers`.
- **No service-only admins on this page.** Edge + SPA gate is hub `global_role == ADMIN` only.
- **Do not disable the signed-in admin’s row.** Self-edit is allowed. Last-admin demotion is still User-Service **409** `LAST_ADMIN`.
- **Do not** expose `POST /users` or `GET /users/by-google-sub/*` on the public hub (stay `reject`).
- **Do not** change storage-host routing, OAuth, `/api/me`, or identity headers except as implied by proxying `GET /users` like other User-Service GETs (strip cookies; set `X-User-Id`).

---

## 4. AuthZ

| Layer | Rule |
| --- | --- |
| SPA `/admin` | `isAdmin(user)` → `global_role === 'ADMIN'`. Non-admin → `Navigate` home (unchanged). |
| Gateway `/admin`, `/admin/*` | `auth: admin` (unchanged). |
| Gateway `GET /users` | **`auth: admin`** (this story). Anonymous → **401** JSON (`/users` is treated as API). Signed-in non-ADMIN → **403** `{"error":"forbidden"}`. `BLOCKED` / `rejected_cache` → **403** `blocked`. |
| Gateway `POST /users` | **`auth: reject`** → **404**, no proxy. |
| Gateway `GET /users/by-google-sub/*` | **`auth: reject`** → **404**, no proxy. |
| Gateway `/users/*` | **`auth: session`** (unchanged): `GET /users/{id}`, `PUT …/role`, `PUT …/roles/{service}`. User-Service still enforces actor global ADMIN (or service ADMIN on that `{service}`) on PUTs via `X-User-Id`. |
| User-Service `GET /users` | No `X-User-Id` required. Gateway still **sets** `X-User-Id` from the access JWT `sub` on every User-Service proxy (existing director). |

There is **no** Auth-side user table. Directory truth stays User-Service.

---

## 5. Gateway change

File: `deploy/gateway.yaml` (hub `filenkov.store` routes). First match wins.

**Today** (rejects all methods on exact `/users`):

```yaml
      - match: /users/by-google-sub/*
        auth: reject
      - match: /users
        auth: reject
      - match: /users/*
        upstream: user_service
        auth: session
        preserve_host: false
```

**Target:**

```yaml
      - match: /users/by-google-sub/*
        auth: reject
      - match: /users
        methods: [POST]
        auth: reject              # create is Auth API on user_network only
      - match: /users
        methods: [GET]
        upstream: user_service
        auth: admin               # directory list — hub ADMIN only
        preserve_host: false
      - match: /users/*
        upstream: user_service
        auth: session
        preserve_host: false
```

| Request | Policy | Upstream |
| --- | --- | --- |
| `GET /users` | `admin` | `user_service` |
| `POST /users` | `reject` (404) | none |
| `GET /users/by-google-sub/*` | `reject` (404) | none |
| `GET /users/{id}`, `GET /users/{id}/roles`, PUTs | `session` | `user_service` |

User-Service proxy rules unchanged: strip `Cookie` / `Authorization` / JWT query params; set `X-User-Id` from JWT `sub`; `preserve_host: false`. `GET /users` must **not** write `rejected:user:` (that stays on **200** `PUT /users/{id}/role` and `PUT /users/{id}/roles/{service}`).

Do **not** leave `GET /users` to fall through to `/users/*` (`auth: session`). That prefix matches exact `/users` and would list the directory for any signed-in non-BLOCKED user.

---

## 6. Frontend behavior

Files: `web/src/pages/Admin.tsx`, `web/src/api.ts`, `web/src/types.ts`, `web/src/pages/Admin.test.tsx`.

### 6.1 Load

On mount (admin already confirmed): **one** `GET /users`. Show the **full** table. No pagination. No second round-trip per row.

Map each list item to the existing SPA `User` shape (`id`, `google_sub`, `email`, `username`, `global_role`, `services`). Timestamps may be ignored.

### 6.2 Search

- **Email only**, client-side. Case-insensitive substring on `email`.
- No username / UUID / `google_sub` search.
- Clearing the box shows everyone again. Do not re-fetch.

### 6.3 Columns and roles

Always show: username, email, **global**, **storage**, **resume**.

**Plus** any other `services[].service` keys that appear on **any** loaded user (union across the list). Extra columns/selects for those persisted services (e.g. `hermes` if someone has a row). Do not invent empty product columns that nobody has.

Missing implied service (storage, resume, or a dynamic extra that another row has) still renders as **`STRANGER`** via `serviceRole()` — same as today. Do not PUT just to materialize `STRANGER`.

Role enum in selects: `ADMIN` \| `FAMILY` \| `STRANGER` \| `BLOCKED` (`SITE_ROLES` / `SERVICE_ROLES`).

### 6.4 Writes

Instant apply on `<select>` change (keep current UX):

- Global: `PUT /users/{id}/role` body `{ "role": "FAMILY" }`.
- Service: `PUT /users/{id}/roles/{service}` body `{ "role": "FAMILY" }` (`storage`, `resume`, or a dynamic extra key).

`credentials: 'include'`. Do not send `X-User-Id`. Existing `request()` refresh-on-401 stays.

After **200**: update that row in local state (merge PUT result, or optional `GET /users/{id}` + `GET /users/{id}/roles` for that id only). Do **not** re-fetch the full list. Do **not** bring back UUID paste to “reload” a person.

**409** `LAST_ADMIN`: show User-Service `error.message` (or `code`); leave the select on the previous value.

Other 4xx/5xx: same error banner pattern as today (`readError` already parses `{ "error": { "code", "message" } }`).

### 6.5 Self-edit

The signed-in admin’s row stays enabled. Demoting yourself is allowed; User-Service **409** `LAST_ADMIN` if you are the last hub ADMIN.

### 6.6 States

| State | UI |
| --- | --- |
| Loading list | Empty/loading copy (not the UUID form). |
| `GET /users` fails | Error banner; no table (or stale empty). |
| `users: []` | Empty directory copy (distinct from search miss). |
| Search matches nobody | No-match copy (directory was non-empty). |
| Rows | Full table (filtered by email if the box is non-empty). |

Remove: UUID input, “Look up a person”, “There is no directory list.”

---

## 7. API usage (SPA → gateway → User-Service)

| Method | Path | When |
| --- | --- | --- |
| `GET` | `/users` | Page load. Sole source for the table. |
| `PUT` | `/users/{id}/role` | Global select change. |
| `PUT` | `/users/{id}/roles/{service}` | Storage / resume / extra service select change. |
| `GET` | `/users/{id}` and `GET /users/{id}/roles` | **Optional** after a PUT to refresh one row. **Not** used as a paste-UUID entry point. List items already include identity + persisted `services`. |

Still **forbidden** in the SPA: `/api/admin`, `POST /users`, `GET /users/by-google-sub/*`.

---

## 8. Acceptance criteria

1. Hub `ADMIN` `GET /users` is proxied to `user_service:8000/users` with `X-User-Id` set and **no** JWT cookies. **200** `{ "users": [...] }` reaches the SPA.
2. Hub `FAMILY` / `STRANGER` `GET /users` → gateway **403** forbidden; no User-Service hit required (policy fails first).
3. Anonymous `GET /users` → **401** JSON, not a list.
4. `POST /users` (even as ADMIN) → **404**; no proxy. `GET /users/by-google-sub/*` → **404**.
5. `/admin` still `auth: admin`. `/api/admin` still does not exist (falls through to `/api/*` session; Auth API has no such route).
6. Admin SPA shows every directory user after one `GET /users`. No UUID field.
7. Email box filters the table client-side; no extra HTTP. No other filters. No pager.
8. Storage and resume show `STRANGER` when omitted from `services[]`. Extra persisted service keys appear as extra columns.
9. Changing a select fires the matching PUT immediately. **409** `LAST_ADMIN` is visible and does not leave a lying select.
10. The signed-in admin can change their own row.
11. Tests that asserted `GET /users` **404** for ADMIN (`TestHubUsersStillRejected` in `internal/gateway/handler_test.go`) are updated. Tests that asserted no `/api/admin` stay.
12. No User-Service code or spec change in this story.

---

## 9. Test plan

### Gateway (`internal/gateway/handler_test.go`, `config_test.go`, `match_test.go`)

- `TestHubUsersListAdminOnly` (replaces `TestHubUsersStillRejected`): ADMIN `GET /users` proxies; FAMILY/STRANGER **403** `forbidden`; BLOCKED **403** `blocked`; anonymous **401** `unauthorized`; ADMIN `POST /users` and `by-google-sub` **404**. Also covered by `TestUserServiceCreateAndGoogleSubRejected`.
- `TestHubUsersListProxiesWithXUserId`: strips cookies/`Authorization`, overwrites `X-User-Id`, does **not** write `rejected:user:`.
- `TestLoadDefaultYAML`: `MatchMethod(…, "GET", "/users")` → `auth: admin`, upstream `user_service`. `MatchMethod(…, "POST", "/users")` → `auth: reject`. `by-google-sub` still reject. `GET /users/{uuid}` still `session`.
- `TestRouterFirstMatch`: GET `/users` is `admin`; POST `/users` is `reject`; PUT `/users/{id}/role` is `/users/*` `session`.

### SPA (`web/src/pages/Admin.test.tsx`, `web/src/api.test.ts`)

- Non-admin still redirected home.
- Mock `GET /users` `{ users: [admin, stranger] }`: both rows render; **no** UUID textbox; **no** `/api/admin` URLs.
- Email substring hides non-matching rows; empty search shows all. Username / UUID / `google_sub` are not search keys.
- Search miss vs empty `users: []` are distinguishable.
- `GET /users` 500 / 403 → error copy.
- Select change → `PUT /users/{id}/role` or `PUT /users/{id}/roles/{service}` with JSON `{ "role": "…" }`.
- **409** `LAST_ADMIN` → message shown.
- User with only `services: [{ service: "hermes", role: "FAMILY" }]` → extra Hermes column; storage/resume still `STRANGER`.
- Signed-in admin row is not `disabled`.
- `fetchUsers` uses `credentials: 'include'`, does not send `X-User-Id`, retries once after 401 + `/api/refresh`.

`go test ./...` and `cd web && npm test`.

---

## 10. Migration notes

| Area | Change |
| --- | --- |
| `deploy/gateway.yaml` | Split exact `/users`: `POST` reject, `GET` admin + `user_service`. Keep `by-google-sub` reject and `/users/*` session. |
| `web/src/pages/Admin.tsx` | Delete UUID form and lookup copy. Load `GET /users`. Email search. Dynamic extra service columns. Instant PUT. Self-edit enabled. |
| `web/src/api.ts` | Add list fetch (`GET /users`). Keep PUTs. `fetchUser` only if still used to refresh one row after PUT. |
| `web/src/pages/Admin.test.tsx` | Drop UUID lookup cases; assert list + email filter + no `/api/admin`. |
| `internal/gateway/handler_test.go` | `TestHubUsersListAdminOnly` / `TestHubUsersListProxiesWithXUserId` (replaced `TestHubUsersStillRejected`) |
| `internal/gateway/config_test.go` | Default YAML: GET `/users` is `admin`. |
| Docs | This file; `AGENTS.md`; `README.md` Admin; `docs/auth-gateway.md`; stale “no list-users” notes in `docs/user-story-user-service-contract.md` and `docs/user-story-storage-host-headers-session.md`. |

Operator: redeploy gateway + SPA. No User-Service deploy required. No Auth DB migration.

---

## 11. What changed in this repo (shipped)

| Area | Change |
| --- | --- |
| `deploy/gateway.yaml` | §5 routes |
| `internal/gateway` tests | `TestHubUsersListAdminOnly`, `TestHubUsersListProxiesWithXUserId`; `TestLoadDefaultYAML` / `TestRouterFirstMatch` |
| `web/src/api.ts` | `fetchUsers` (`GET /users`); types in `web/src/types.ts` |
| `web/src/pages/Admin.tsx` | Directory table + email search + dynamic services |
| `web/src/pages/Admin.test.tsx` | List-driven tests |
| Docs listed in §10 | Describe the running system |

Do not add User-Service tasks. User-Service docs may still say the public hub rejects `GET /users`; that was an explicit non-goal of this CR (Auth Gateway is the public ACL).
