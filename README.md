# Auth-Service

Google OAuth2 hub for `filenkov.store`: one httpOnly session cookie on `.filenkov.store`, Postgres role enums, Redis sessions, and an internal gRPC `Validate` / `ListUsers` API for other LAN services.

The browser never sees a token in the URL. Product backends (storage, resume, …) extract the `auth_session` cookie and call gRPC `Validate` on every request.

## Layout

```
cmd/api/          HTTP + gRPC process
internal/         config, http, grpc, oauth, session, users, whitelist, ratelimit, log
migrations/       golang-migrate SQL
proto/            Validate + ListUsers service
pkg/authpb/       generated Go stubs (import from other services)
web/              Vite SPA (static nginx image)
deploy/           docker-compose.yml, Dockerfiles
```

## Quick start (Docker Compose)

1. Copy env and fill Google OAuth + admin email:

   ```bash
   cp .env.example .env
   ```

2. In [Google Cloud Console](https://console.cloud.google.com/apis/credentials) create an OAuth client (Web application) with authorized redirect URIs:

   - Production: `https://filenkov.store/oauth/google/callback`
   - Local host Caddy on port 80: `http://localhost/oauth/google/callback`

3. For local HTTP, in `.env` set:

   ```
   AUTH_DEV_HTTP=true
   COOKIE_DOMAIN=
   PUBLIC_ORIGIN=http://localhost
   GOOGLE_REDIRECT_URL=http://localhost/oauth/google/callback
   ```

4. Run from the repo root:

   ```bash
   docker compose --env-file .env -f deploy/docker-compose.yml up --build
   ```

   Compose publishes the API on `127.0.0.1:8082` and the SPA on `127.0.0.1:8081` (override with `API_HTTP_PORT` / `WEB_HTTP_PORT`). Point host Caddy at those ports (see below). gRPC (`:9090`) is **not** published; other containers should call `api:9090` on the compose network.

## Host Caddy (existing proxy)

Do **not** add a second `filenkov.store { }` site — Caddy will refuse a duplicate hostname. Insert these `handle` blocks **above** the existing catch-all `reverse_proxy` (or `handle`) for that site. `reverse_proxy` already sets `X-Forwarded-For` / `X-Forwarded-Proto`.

```caddy
# Extra: Auth-Service API (before the SPA catch-all)
handle /oauth/* {
	reverse_proxy 127.0.0.1:8082
}

handle /api/* {
	reverse_proxy 127.0.0.1:8082
}

handle /health {
	reverse_proxy 127.0.0.1:8082
}
```

Leave the existing catch-all pointing at the SPA (`127.0.0.1:8081` if this stack owns `/`):

```caddy
handle {
	reverse_proxy 127.0.0.1:8081
}
```

If `filenkov.store` is a new site on this host, the full block is:

```caddy
filenkov.store {
	encode gzip

	handle /oauth/* {
		reverse_proxy 127.0.0.1:8082
	}

	handle /api/* {
		reverse_proxy 127.0.0.1:8082
	}

	handle /health {
		reverse_proxy 127.0.0.1:8082
	}

	handle {
		reverse_proxy 127.0.0.1:8081
	}
}
```

Reload with `sudo systemctl reload caddy` (or `caddy reload --config /etc/caddy/Caddyfile`).

## Local Go (without Compose for the API)

Run Postgres and Redis (or the compose `postgres` + `redis` services), then:

```bash
export $(grep -v '^#' .env | xargs)
export DATABASE_URL=postgres://auth:auth@127.0.0.1:5432/auth?sslmode=disable
export REDIS_URL=redis://127.0.0.1:6379/0
export MIGRATIONS_PATH=migrations
go run ./cmd/api
```

The Vite app (`web/`) can proxy `/api` and `/oauth` to `localhost:8080` during `npm run dev`.

```bash
go test ./...
go build -o auth-service ./cmd/api
```

## HTTP API

JSON, same-origin (no CORS). Cookie: `auth_session`, `HttpOnly`, `SameSite=Lax`, `Secure` unless `AUTH_DEV_HTTP=true`, `Domain` from `COOKIE_DOMAIN` (default `.filenkov.store`).

| Method | Path | Auth | Notes |
|--------|------|------|--------|
| `GET` | `/oauth/google?return_to=` | — | CSRF state in Redis; `return_to` must be `https` and an exact whitelist host. With `AUTH_DEV_HTTP=true`, `http://localhost` and `http://127.0.0.1` are also allowed. |
| `GET` | `/oauth/google/callback` | — | Upsert user, set cookie, 302 to `return_to` |
| `POST` | `/api/logout` | cookie | Deletes Redis session, clears cookie |
| `GET` | `/api/me` | cookie | `401` if anonymous, `403` if hub `roles=BLOCKED` |
| `GET` | `/api/admin/users` | cookie + `roles=ADMIN` | `{ "users": [ ... ] }` |
| `PATCH` | `/api/admin/users/:id` | admin | Optional `roles`, `storage_roles`, `resume_roles` (including `BLOCKED` for that service) |
| `GET` | `/health` | — | Postgres + Redis ping |

`GET /api/me` body:

```json
{
  "id": "...",
  "google_email": "...",
  "name": "...",
  "roles": "ADMIN",
  "storage_roles": "STRANGER",
  "resume_roles": "STRANGER",
  "last_seen_at": "2026-08-13T12:00:00Z"
}
```

Signup is open to any Google account. New users get `STRANGER` on every role column. The first insert of `ADMIN_EMAIL` gets `roles=ADMIN`; later logins do not re-apply the env var. Demoting or setting `roles=BLOCKED` on the last active admin returns `409`.

IP rate limits (Redis): OAuth ~10/min, `/api/*` ~60/min.

## gRPC Validate (internal)

Other services pass the session id (from the cookie) and their host:

```text
rpc Validate(ValidateRequest) returns (ValidateResponse);
rpc ListUsers(ListUsersRequest) returns (ListUsersResponse);
```

`ValidateRequest` / `ListUsersRequest`: `session_id`, `caller_host`  
`ValidateResponse`: `map<string,string> fields` projected from the whitelist.  
`ListUsersResponse`: `repeated ListUser users`, each with the same whitelist `fields` map as Validate for that host.

| Host | Fields |
|------|--------|
| `filenkov.store` | `google_email`, `name`, `roles` |
| `storage.filenkov.store` | `id`, `google_email`, `name`, `storage_roles` |
| `resume.filenkov.store` | `name`, `resume_roles` |

There is no LLM audience. If the caller's role column is `BLOCKED` (hub `roles`, `storage_roles`, or `resume_roles` depending on `caller_host`), Validate returns `PERMISSION_DENIED` (`blocked`). A block on one service does not deny the others. Bad sessions get `UNAUTHENTICATED`. Unknown `caller_host` is `PERMISSION_DENIED`.

`ListUsers` authenticates like Validate, then requires a **service admin** for the caller host (`storage_roles=ADMIN` on storage, `resume_roles=ADMIN` on resume, hub `roles=ADMIN`). Non-admins get `PERMISSION_DENIED`. The RPC is read-only: it lists every user with the whitelist projection and does not return session ids or accept role writes. LAN gRPC stays unauthenticated at the transport layer.

Regenerate stubs (requires `protoc`, `protoc-gen-go`, `protoc-gen-go-grpc`):

```bash
protoc -I proto \
  --go_out=pkg/authpb --go_opt=paths=source_relative \
  --go-grpc_out=pkg/authpb --go-grpc_opt=paths=source_relative \
  proto/auth.proto
```

Vendored output lives in `pkg/authpb/` so other services can import `github.com/aleshka5/auth-service/pkg/authpb`.

## Roles

Postgres enums (extend with a new migration + `DEFAULT 'STRANGER'`):

- `roles`: `ADMIN` \| `STRANGER` \| `BLOCKED`
- `storage_roles`, `resume_roles`: `ADMIN` \| `FAMILY` \| `STRANGER` \| `BLOCKED`

`BLOCKED` was added in migration `000002`. Restart the API so golang-migrate can apply it; editing an already-applied `000001` does not re-run.
