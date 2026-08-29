# auth-gateway — Hermes dashboard auth behind Auth-Service

Hermes refuses `hermes dashboard --host 0.0.0.0` unless a
`DashboardAuthProvider` is registered. There is no unauthenticated public
dashboard (`--insecure` is a no-op).

This plugin is that provider. It does **not** add a second login. The Auth
Gateway already requires an admin access JWT on `/hermes/*`, injects
`Authorization: Bearer <access JWT>`, and this plugin verifies that JWT
with the same `JWT_SECRET`.

## Install into the Hermes home the container actually uses

Copy the plugin directory (not a symlink into this git repo) into Hermes
plugins, then enable it:

```bash
cp -a deploy/hermes-plugins/auth-gateway /path/to/HERMES_HOME/plugins/
# inside the Hermes environment:
hermes plugins enable auth-gateway
```

Set these on the **Hermes** process (`.env` is correct — they are secrets /
auth config the plugin reads from the environment):

```
HERMES_DASHBOARD_AUTH_JWT_SECRET=<same bytes as Auth-Service JWT_SECRET>
HERMES_DASHBOARD_AUTH_JWT_ISSUER=https://filenkov.store
HERMES_DASHBOARD_AUTH_JWT_AUDIENCE=filenkov.store
```

Local HTTP (`AUTH_DEV_HTTP=true`) typically needs:

```
HERMES_DASHBOARD_AUTH_JWT_ISSUER=http://localhost
HERMES_DASHBOARD_AUTH_JWT_AUDIENCE=localhost
```

In `config.yaml` bind a non-loopback address and declare the public URL
(even when the process listens on `0.0.0.0` behind Docker):

```yaml
dashboard:
  host: "0.0.0.0"
  port: 8787
  public_url: "https://filenkov.store/hermes"
```

Do **not** publish `:8787` on the public internet. Anyone who can reach
that port and who has `JWT_SECRET` (or a stolen admin access JWT) can call
Hermes directly. Keep it on the Docker host / `host.docker.internal` path
only. Restart Hermes after installing the plugin or changing the secret.

## Checks

```bash
python3 deploy/hermes-plugins/auth-gateway/tests/test_jwtutil.py
```

After restart, dashboard logs should include a provider list containing
`auth-gateway` instead of “Refusing to bind dashboard to 0.0.0.0”.
