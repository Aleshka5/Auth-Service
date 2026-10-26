# Change Request: Messenger host `messanger.filenkov.store` (Auth Gateway)

> **Status:** **Reverted (2026-10-23).** `upstreams.messanger`, the `messanger.filenkov.store` host block and its tests were removed from `deploy/gateway.yaml` / `config_test.go` / `auth-gateway.md`: there is no messenger container on master-node and no Caddy site, so the host only produced a TLS error. The `/users/by-email/*` and `POST /users` rejects stay — they are hub hardening, not messenger-specific. The rest of this document is the historical record of the request; to bring the host back, restore the block and add the Caddy site first.
> **Repos:** Auth-Service only (`deploy/gateway.yaml` + gateway tests). **Config change, not code** — the gateway already supports everything this needs.
> **Requester:** HomeMessanger (Python/FastAPI 1-to-1 messenger, compose service `messanger`, listen `:8000`, unpublished).
> **Does not** touch Auth API, the SPA, Redis, User-Service, or the JWT contract. No migration. No new auth mode.
> **Operator:** redeploy **gateway** only, plus one new Caddy site and a DNS record.

**Actor:** Any signed-in hub user (`global_role` other than `BLOCKED`) browsing `https://messanger.filenkov.store`.

**Goal:** Route a new product host through the Auth Gateway to `http://messanger:8000` with `auth: session`, so the messenger gets `X-Auth-*` identity and never implements sign-in.

**Value:** Same deal WebStorage already has. The messenger stores no credential, verifies no JWT, and shares no `JWT_SECRET`; the edge stays the single place where identity is decided. Adding the host is a YAML entry because `docs/auth-gateway.md` promises exactly that.

---

## 1. Background

Today `deploy/gateway.yaml` serves three hosts: `filenkov.store`, `storage.filenkov.store`, `llm.filenkov.store`. HomeMessanger is a fourth product behind the same edge, with the simplest possible policy: one public health route and a session-only catch-all.

What the messenger consumes (and nothing more):

| Header | Use in HomeMessanger |
| --- | --- |
| `X-Auth-User-Id` | The identity. Missing or non-UUID → its own `401`. Used as the sender and participant key |
| `X-Auth-Email` | Projected into its local `users` table for display |
| `X-Auth-Name` | Same, as the username |
| `X-Auth-Role` | **Ignored.** The messenger has no roles — you are a participant of a dialog or you are not |

It reads **no cookie**, verifies **no JWT**, and issues **no redirect**. `BLOCKED` is expected to be stopped at the edge, as with any `auth: session` route.

---

## 2. Assumptions

1. HomeMessanger's compose joins the Auth stack network as `external` (`AUTH_DOCKER_NETWORK`, `deploy_auth` from `Auth-Service/deploy`), so `messanger:8000` resolves on `auth_network` — the same arrangement WebStorage uses for `app:8000`.
2. HomeMessanger serves its built SPA **and** `/api/*` from one container, so one upstream and one catch-all route are enough (no second `web`-style upstream).
3. It publishes no host port. The gateway is the only path in, and the gateway strips inbound `X-Auth-*` before proxying — that is what makes header trust safe.
4. Uploads are capped at 25 MB client- and server-side; a home uplink can take minutes, so the host needs a proxy timeout above the 60 s default (the `timeouts.proxy` per-host override already exists — `TestParseHostProxyTimeout`).
5. `role_field` stays the default `global_role`. No per-service role lookup, so **no User-Service call on this host** and no `X-User-Id` / `X-Storage-Role` injection.

---

## 3. Out of scope

- No gateway **code** change. If one appears necessary, this CR has been misread.
- No `inject_access_jwt` (nothing there verifies a JWT), no `preserve_host: false`, no `Cookie` forwarding, no new Redis key, no `strip_prefix`.
- No `/api/admin`, no new auth mode, no change to any existing host's routes except the one hub addition in §5.
- No Auth API or SPA change. The messenger is not linked from the hub header in this CR.

---

## 4. Requested change — `deploy/gateway.yaml`

### 4.1 Upstream

```yaml
upstreams:
  # …existing…
  # HomeMessanger joins auth_network as compose service `messanger` (external network from that stack).
  messanger: "http://messanger:8000"
```

### 4.2 Host block (after `storage.filenkov.store`)

```yaml
  messanger.filenkov.store:
    role_field: global_role
    # Attachment uploads need a generous response-header timeout (it does not cap the request body).
    timeouts:
      proxy: 5m
    routes:
      - match: /health
        upstream: messanger
        auth: public
      - match: /*
        upstream: messanger
        auth: session
```

First match wins, so `/health` must precede the catch-all. The catch-all serves both `/api/*` and the SPA.

---

## 5. Recommended companion change on the hub

HomeMessanger needs an email→user lookup, requested from User-Service as `GET /users/by-email/{email}` (that repo's `docs/user-story-lookup-by-email.md`). It is a service-to-service call on `user_network` — **not** a browser route. When it ships, reject it at the hub exactly as `by-google-sub` is rejected, or any signed-in user could enumerate household emails through the edge:

```yaml
  filenkov.store:
    routes:
      - match: /users/by-google-sub/*
        auth: reject
      - match: /users/by-email/*        # ← add
        auth: reject
      # …/users POST reject, GET admin, /users/* session stay unchanged…
```

Order matters: this must sit **above** `- match: /users/*` (`auth: session`), or the prefix route claims it first.

---

## 6. Edge policy for the new host (expected, from existing behaviour)

| Request | Expected |
| --- | --- |
| `GET /health`, anonymous | `200`, proxied |
| Browser navigation, no valid access JWT | `302` to `{PUBLIC_ORIGIN}/auth?return_to=https://messanger.filenkov.store/…` |
| XHR / `/api/*`, no valid access JWT | `401` JSON |
| Valid JWT, `global_role != BLOCKED` | Proxied with `X-Auth-User-Id` / `X-Auth-Email` / `X-Auth-Name` / `X-Auth-Role` |
| `BLOCKED`, or user in `rejected_cache` | `403`; never reaches the messenger |
| Client-supplied `X-Auth-*` | Stripped, then re-injected by the gateway |
| `messanger` container down | Gateway proxy error on this host only |

`return_to` is allowlisted by the set of hosts in the YAML, so adding the host also makes its own return URL valid — no `WHITELIST_JSON`.

---

## 7. Acceptance

- `deploy/gateway.yaml` parses with the new upstream and host; `TestLoadDefaultYAML` asserts `messanger.filenkov.store` exists, `/health` is `public`, the catch-all is `session`, and `timeouts.proxy` is 5 m.
- `TestRouterFirstMatch`-style coverage: `/health` resolves to the public route, `/api/me` and `/` resolve to the session catch-all.
- Handler coverage on the new host mirroring existing cases: anonymous navigation redirects, anonymous XHR gets `401`, a valid JWT is proxied with the four `X-Auth-*` headers, `BLOCKED` gets `403`, and client-supplied `X-Auth-User-Id` is overwritten.
- `AllowedHostsMap` includes the new host (`TestAllowedHostsMap`), so `return_to` to it is accepted.
- No User-Service call is made on this host.
- `go test ./...` green.

---

## 8. Operator steps

1. DNS: `messanger.filenkov.store` → the same address as the other hosts.
2. Caddy (TLS only, no routing): the site now lives in the canonical host file
   [`deploy/Caddyfile.filenkov`](../deploy/Caddyfile.filenkov), next to
   `storage.filenkov.store` and with the same TLS and header settings — no
   `encode gzip`, per [auth-gateway.md](auth-gateway.md#migration-caddyfile).

   ```sh
   sudo cp deploy/Caddyfile.filenkov /etc/caddy/Caddyfile
   sudo systemctl reload caddy
   ```
3. Redeploy the gateway with the new YAML.
4. Start order: User-Service → Auth → Common → HomeMessanger. Until the messenger container joins `auth_network`, that host returns a proxy error and nothing else is affected.

Verify:

```sh
curl -sS -i https://messanger.filenkov.store/health        # 200, anonymous
curl -sS -i https://messanger.filenkov.store/api/me        # 401/redirect anonymous; 200 signed in
```

---

## 9. Migration notes

| Area | Change |
| --- | --- |
| `deploy/gateway.yaml` | `upstreams.messanger`; `messanger.filenkov.store` host block; hub `/users/by-email/*` reject (§5, when the User-Service CR ships) |
| `internal/gateway/config_test.go` | Default-YAML assertions for the new host |
| `internal/gateway/handler_test.go` | Anonymous / signed-in / BLOCKED / header-injection cases on the new host |
| `README.md`, `AGENTS.md`, `docs/auth-gateway.md` | Add the host to the live-hosts lists **when this ships**, not before |

No Auth DB migration (there is no Auth DB). No SPA rebuild. No User-Service deploy for this file — §5 only becomes relevant once that repo ships its own CR.
