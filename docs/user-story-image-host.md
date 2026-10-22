# Change Request: Image host `image.filenkov.store` (Auth Gateway + resume header)

> **Status:** Implemented in Auth-Service (uncommitted). See §12 for what shipped and the decisions taken.
> **Repos:** Auth-Service (`deploy/gateway.yaml`, gateway role check, compose network, Caddy site, resume header, admin directory column).
> **Depends on:** User-Service key `image` ([User-Service `docs/user-story-image-service-role.md`](../../User-Service/docs/user-story-image-service-role.md)). That key needs no new API. The gateway calls the existing `GET /users/{id}/roles/image`.
> **Does not** put a login page on the image app, and does not send it a JWT.

**Actor:** A signed-in person whose User-Service role on `image` is `FAMILY` or `ADMIN`.

**Goal:** Publish Qwen Image at `https://image.filenkov.store`, proxy it only for that service role, and show an **Image** link in the resume header next to Messanger, Storage, and Hermes — only for people who may open it.

**Value:** The GPU service stays unpublished. The edge decides who may use it, from the directory, the same way Storage learns its role. The resume header is how the household finds it.

---

## 1. Background

Live hosts today: `filenkov.store`, `storage.filenkov.store`, `messanger.filenkov.store`, `llm.filenkov.store`. The header (`web/src/components/Layout.tsx`) links Messanger, Storage, and Hermes for everyone, with a lock for guests. Admin is hidden unless `global_role == ADMIN`.

`role_field` may be `global_role`, `storage`, or `resume`. `storage` looks up User-Service and injects `X-Storage-Role`, then still proxies any signed-in user who is not hub-`BLOCKED`. That is not enough here. `STRANGER` on `image` must not be proxied.

Qwen Image is one Flask process (UI + `POST /api/generate`) on container port `8000`. A frame can take minutes. The gateway default proxy timeout is 60s.

The image container is reached on Podman network **`models_network`** (owned by Switch-Service), not `auth_network`. Compose service / container name on that network: **`qwen-image`**.

## 2. Who is allowed

The decision uses the **`image` service role**, not hub `global_role`.

| `GET /users/{id}/roles/image` | Gateway |
| --- | --- |
| `FAMILY` or `ADMIN` | Proxy |
| `STRANGER` (including no row) or `BLOCKED` | `403` `{"error":"forbidden"}`. Do not proxy |
| User-Service 5xx, timeout, network error | `503` `{"error":"unavailable"}`. Do not invent a role |
| User 404 | `401` `{"error":"unauthorized"}`. Do not proxy |

A hub `ADMIN` or hub `FAMILY` with no `image` row is `STRANGER` on this host and is refused. Grant the row from the admin directory (`PUT /users/{id}/roles/image`).

Existing edge checks still apply first: no access JWT → HTML `302` to `{PUBLIC_ORIGIN}/auth?return_to=…`, XHR `401`; hub `global_role == BLOCKED` or `rejected_cache` → `403`.

## 3. Assumptions

1. User-Service is up on `user_network`. `GET /users/{id}/roles/image` already behaves as in its story.
2. Switch-Service creates `models_network` and runs the container as `qwen-image` with no published host port. The gateway joins that network as `external: true`, the same way it joins `llm_network`.
3. Upstream is `http://qwen-image:8000`.
4. The image app does not read roles and does not verify JWTs. This gateway is the only allow/deny.

## 4. Out of scope

- No resume-tree node. Header link only. Label **Image**.
- No `inject_access_jwt`. No `X-Image-Role` that the app is required to trust.
- No change to Storage, Messenger, LLM, or Hermes policy.
- No User-Service code. No image-app login.

`X-Auth-User-Id`, `X-Auth-Email`, `X-Auth-Name`, and `X-Auth-Role` (hub `global_role`) are still injected, as on every other non–User-Service upstream. The image app may ignore them.

## 5. Requested gateway config

```yaml
upstreams:
  qwen_image: "http://qwen-image:8000"

hosts:
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
```

`role_field: image` is new. On this host only, after the access JWT is accepted, call `GET /users/{sub}/roles/image` and apply §2. Do not reuse the Storage behavior of proxying `STRANGER`.

`/healthz` is the app's existing health path. It stays public so an operator can probe the process. The UI and `POST /api/generate` are the catch-all.

`timeouts.proxy: 30m` is the response-header timeout. The file default stays 60s. A generate that runs for minutes must not be cut off at 60s.

Adding the host adds `image.filenkov.store` to JWT `aud` and to the `return_to` allowlist. No `WHITELIST_JSON`.

## 6. Header and admin directory

**Header.** In `web/src/components/Layout.tsx`, add **Image** → `https://image.filenkov.store` beside the other service links.

Show it only when `serviceRole(user, 'image')` is `FAMILY` or `ADMIN`. Hide it from guests, from `STRANGER`, and from `BLOCKED` on `image`. Do not show a lock. A hidden link is the Admin rule: a `403` in the nav is noise.

**Admin.** Add `image` to `FIXED_SERVICES` in `web/src/pages/Admin.tsx`, next to `storage` and `resume`. The column must exist before any row does, so the first grant is a select in the directory, not a hand-written `PUT`. Missing row displays as `STRANGER` (existing `serviceRole`).

## 7. Compose and Caddy

`deploy/docker-compose.yml`: gateway attaches external network `models_network`. Do not create that network in the Auth stack. If it is missing, the image host fails and the other hosts do not.

`deploy/Caddyfile.filenkov`: one TLS site, same shape as the other product hosts — `reverse_proxy localhost:8083`, no path routing, no `encode gzip`. HTTP → HTTPS redirect for `image.filenkov.store`. DNS points at the same address as the other hosts.

## 8. Edge behavior

| Request | Expected |
| --- | --- |
| `GET /healthz`, anonymous | `200`, proxied |
| Browser navigation, no access JWT | `302` to hub `/auth?return_to=https://image.filenkov.store/…` |
| XHR or `POST /api/generate`, no access JWT | `401` JSON |
| Valid JWT, `image` role `FAMILY` or `ADMIN` | Proxied to `http://qwen-image:8000` with `X-Auth-*` |
| Valid JWT, `image` role `STRANGER` or `BLOCKED` | `403`, not proxied |
| Hub `BLOCKED` or `rejected_cache` | `403`, not proxied |
| User-Service role GET fails | `503`, not proxied |
| Client-supplied `X-Auth-*` | Stripped, then re-injected from the JWT |
| `qwen-image` down | Proxy error on this host only |

## 9. Acceptance

- `deploy/gateway.yaml` parses. `role_field: image` is a known field. `TestLoadDefaultYAML` sees `image.filenkov.store`, public `/healthz`, session catch-all, proxy timeout 30m.
- Handler tests: anonymous HTML redirects; anonymous XHR is `401`; `FAMILY` and `ADMIN` on `image` are proxied; `STRANGER` and `BLOCKED` on `image` are `403` with no proxy; a hub `ADMIN` whose `image` role is `STRANGER` is `403`; User-Service 5xx is `503`; user 404 is `401`; spoofed `X-Auth-User-Id` is overwritten.
- `AllowedHostsMap` includes `image.filenkov.store`.
- Header test: Image is present for `image` `FAMILY` and `ADMIN`, absent for a guest and for `image` `STRANGER`. Messanger, Storage, and Hermes stay as they are.
- Admin test: the Image column renders for a user with no `image` row and a `PUT` sends `/users/{id}/roles/image`.
- `go test ./...` and `cd web && npm test` pass.
- Gateway compose lists `models_network` as external. Caddy site is in `deploy/Caddyfile.filenkov`.

## 10. Docs to update when this ships

`README.md`, `AGENTS.md`, and `docs/auth-gateway.md`: add the host, the `image` role check, the network, and the header rule. Not before it ships.

## 11. Operator order

1. User-Service stack (`user_network`).
2. Switch-Service `make network` and the image model, so `models_network` and the name `qwen-image` exist.
3. Auth stack, so the gateway joins that network.
4. DNS `image.filenkov.store`. Copy `deploy/Caddyfile.filenkov` to `/etc/caddy/Caddyfile` and reload Caddy.
5. In `/admin`, set `image` to `FAMILY` or `ADMIN` for each person who should see the link.

Until step 2, only `image.filenkov.store` errors. The hub and the other hosts stay up.

## 12. What shipped

- **Gateway.** `role_field: image` is a known field. `applyServiceRole` (`internal/gateway/identity.go`) runs after `authorize`: on non-`public` routes of an image host it calls `GET /users/{sub}/roles/image` and proxies only `FAMILY` / `ADMIN`. `STRANGER`, `BLOCKED`, and any other value are `403` `{"error":"forbidden"}` (deny by default); user 404 is `401`; any other error is `503` `{"error":"unavailable"}`. Storage keeps its old behavior (lookup, inject `X-Storage-Role`, never deny on the role). The image host gets the four `X-Auth-*` only.
- **`/healthz`.** The gateway used to answer `GET /healthz` itself on every host, which would have hidden the app's health path. It now answers locally unless the request host has a literal `match: /healthz` route; only `image.filenkov.store` does. Loopback and compose healthchecks still hit the gateway.
- **Config.** `deploy/gateway.yaml` adds upstream `qwen_image` and host `image.filenkov.store` exactly as §5. `deploy/docker-compose.yml` attaches the gateway to external `models_network`. `deploy/Caddyfile.filenkov` has the TLS site and the HTTP→HTTPS redirect.
- **SPA.** `Layout.tsx` shows **Image** only for `image` `FAMILY` / `ADMIN`, no lock. `Admin.tsx` has `image` in `FIXED_SERVICES`.
- **Tests.** `TestLoadDefaultYAML`, `TestParseRoleFieldImage`, `TestHealthzLocalUnlessHostRoutesIt`, `TestImageHealthzPublicNoRoleLookup`, `TestImageHealthzWithoutUserService`, `TestImageAnonymousRedirectVs401`, `TestImageAllowedRolesProxied`, `TestImageDeniedRolesForbidden`, `TestImageHubBlockedAndRejectedForbidden`, `TestImageRoleLookupFailure`; `Layout.test.tsx` and `Admin.test.tsx` Image cases.

Decisions and deviations:

1. Public routes on the image host skip the role lookup, so `/healthz` stays reachable for a signed-in `STRANGER` and while User-Service is down.
2. A missing `models_network` stops the whole gateway container from starting — Compose refuses a missing `external` network, same as `llm_network`. §7 and §11 ("only the image host fails") hold only once the network exists and `qwen-image` is down. Create the network first (`podman network create models_network`) if the image stack is not up.
3. As on storage, messenger, and LLM, the browser `Cookie` header is forwarded, so `auth_access` reaches the image app as a cookie. No `Authorization` header is set.
4. The role is looked up on every non-public request (UI assets included), like storage. There is no cache.
