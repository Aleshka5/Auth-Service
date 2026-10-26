# Hermes behind the Auth Gateway

> **Retired (2026-10-23).** The `/hermes` mount is gone from `deploy/gateway.yaml`: `upstreams.hermes`, the `/hermes`, `/hermes/login`, `/hermes/*` routes and the `/chat`, `/sessions` redirects were removed, because no container named `hermes` runs on master-node and every one of those paths answered 502. The dashboard is served on its own host, `https://hermes.filenkov.store` (Caddy → `localhost:9119`), and the SPA nav link points there. `hermes_prefix.go` and its unit tests stay in the tree for the day a `hermes` container is deployed again; this document is kept as the record of how the mount worked.

Public URL: `https://filenkov.store/hermes/`. There is **one** login: hub Google. Hermes does not show a password page.

Sibling plugin: [`hermes-auth-gateway/plugins/auth-gateway`](../../hermes-auth-gateway/plugins/auth-gateway). Caddy site file: [`deploy/Caddyfile.filenkov`](../deploy/Caddyfile.filenkov).

## Identity

```
Browser (auth_access cookie)
  → Caddy :443  (TLS, HTTP/1.1 + HTTP/2 only)
    → gateway :8083  (session JWT, strip client Authorization)
      → hermes:8787
         Authorization: Bearer <access JWT>
         X-Forwarded-Prefix: /hermes
```

| Piece | Rule |
|-------|------|
| Edge | `/hermes`, `/hermes/*` are `auth: session`, `inject_access_jwt: true`, `strip_prefix: /hermes` |
| Plugin | Hermes `auth-gateway` verifies that Bearer with `HERMES_DASHBOARD_AUTH_JWT_SECRET` = Auth `JWT_SECRET` |
| Second login | `GET /hermes/login` 302 → `/hermes/sessions` |
| SPA basename | `GET /chat` and `GET /sessions` 302 → `/hermes/chat` and `/hermes/sessions` |
| Vite lazy chunks | `GET /assets/*` with `Referer` under `/hermes` 302 → `/hermes/assets/*` (`hermesDroppedAssetRedirect`) |
| JSON `next` `/` or `/login` | rewritten to `/hermes/sessions` |

Hermes must bind `0.0.0.0` with a registered `DashboardAuthProvider`. Do not disable Hermes auth. Do not publish `:8787` on a public interface.

## Chat is a PTY WebSocket

`/hermes/chat` is an xterm. Other Hermes pages are ordinary HTTP; chat is not.

1. `POST /hermes/api/auth/ws-ticket` (HTTP, hub JWT + injected Bearer) → single-use ticket.
2. `wss://filenkov.store/hermes/api/pty?ticket=…` → Hermes `@app.websocket("/api/pty")`.

The SPA builds that URL from `__HERMES_BASE_PATH__` / `X-Forwarded-Prefix`. Success on Hermes logs `pty accepted … cred=ticket`.

Do **not** add a Caddy `handle /hermes/api/pty` that bypasses the gateway. Ticket mint and the upgrade both stay on the same session edge.

### Gateway

`httputil.ReverseProxy` already upgrades. Required so the 101 is usable:

- `ForceAttemptHTTP2: false` (HTTP/2 cannot Upgrade)
- `FlushInterval: -1`
- `ModifyResponse` and security headers skipped on `101`
- `statusWriter` implements `Hijack` / `Flush`
- anonymous Upgrade → **401** JSON, not a login **302**

### Caddy

HTTP/3 (`Alt-Svc: h3`) lets pages load and **blocks** `new WebSocket()`. Security headers on a `101` break the handshake.

Live site (`deploy/Caddyfile.filenkov`):

- `servers { protocols h1 h2 }`
- `header { Alt-Svc "clear" }` on `filenkov.store` (drop cached h3)
- security headers only when `Connection` is not `Upgrade`
- no `encode gzip`
- one `reverse_proxy localhost:8083` — no per-path Hermes handles

Persist `/etc/caddy/Caddyfile` from that file; `systemctl reload caddy` otherwise restores HTTP/3.

### Empty dark-green chat

The shell renders even when `ChatPage` never runs. Vite lazy-loads `/assets/ChatPage-*.js` (root-absolute). Without the Referer redirect that hits the hub SPA (`index.html`, `text/html`). No ticket, no PTY, green canvas only.

A hard refresh after the redirect is in the running gateway is enough to load the real module.
