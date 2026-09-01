# Hermes plugins live in the sibling repo

Canonical copies: [`hermes-auth-gateway/plugins/`](../../../hermes-auth-gateway/plugins).

| Plugin | Role |
|--------|------|
| [`auth-gateway`](../../../hermes-auth-gateway/plugins/auth-gateway) | Verifies the hub access JWT the gateway injects as Bearer. No password login. |
| [`simple-ui`](../../../hermes-auth-gateway/plugins/simple-ui) | Optional Usual vs Simple UI. |

Public mount, chat PTY, Caddy, and Vite `/assets` redirect: [docs/hermes.md](../../docs/hermes.md).

Start Hermes from that repo (`podman compose up -d`). Compose copies plugins into `$HERMES_HOME/plugins/` and attaches `hermes` to this stack’s `auth_network` (`deploy_auth_network` when compose is started from `deploy/`).
