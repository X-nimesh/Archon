# Archon Backend (Go)

Archon's backend API — requirement parsing, architecture generation,
plan/apply orchestration, and agent activity.

Part of the Archon monorepo. See the [root README](../README.md) for all commands.

## Stack

Go 1.26, **no web framework** — routing uses Go 1.22 method patterns on
`http.ServeMux`. PostgreSQL 17 via `pgx/v5` (explicit SQL, no ORM).

## Commands

```bash
pnpm dev:backend
pnpm --filter @archon/backend-go build
pnpm --filter @archon/backend-go test
```

Direct Go tooling works too:

```bash
go run ./cmd/api
go test ./...
```

> `pnpm dev:backend` runs `go run`, which does **not** hot-reload. For a watcher,
> install [air](https://github.com/air-verse/air) once (`go install
> github.com/air-verse/air@latest`) and swap the `dev` script to `air`.
> In Docker, editing a mounted file restarts nothing by itself either — use
> `docker compose restart api`.

Runs on **port 3001**.

## Layout

```
cmd/api/main.go            entrypoint, wiring, migrations, graceful shutdown
migrations/                NNNN_name.up.sql / .down.sql (embedded via embed.FS)
internal/config/           env + .env loading, all tunables
internal/db/               pgx pool, migration runner, error helpers
internal/user/             User model + Repository interface + Postgres Store
internal/password/         Argon2id hashing (PHC strings)
internal/session/          access tokens (JWT) and refresh tokens (rotation)
internal/auth/             signup/login/refresh/logout, guard, validation
internal/throttle/         per-IP rate limiting + lockout policy constants
internal/secure/           defensive response headers
internal/cors/             minimal CORS middleware
internal/httpx/            JSON helpers + the error envelope
internal/server/           route table, request logging
internal/integration/      end-to-end tests against a real database
```

## Auth

| Route | Auth | Purpose |
|---|---|---|
| `POST /auth/signup` | none | Create account → token pair (201) |
| `POST /auth/login` | none | → token pair (200) |
| `POST /auth/refresh` | refresh token | Rotate the pair (200) |
| `POST /auth/logout` | refresh token | Revoke every session (204) |
| `GET /auth/me` | bearer | Current user |
| `GET /health` | none | Liveness |

Protect a route with `app.RequireAuth`, then read the account with
`auth.AccountFrom(ctx)`.

### What "secure" means here

- **Argon2id** (OWASP's first recommendation) at 64 MiB / t=3 / p=2, tunable via
  env. Hashes are stored as PHC strings, so parameters can be raised later and old
  hashes are transparently upgraded on the next successful login.
- **Access tokens** — HS256 JWT, 15 min, `typ` claim pinned, `exp` required, and
  re-checked against the database on every use so disabling an account takes
  effect immediately.
- **Refresh tokens** — 256-bit opaque values, stored only as SHA-256 hashes,
  rotated on every use. Replaying a rotated token revokes the whole family: that
  is what makes rotation worth the complexity.
- **Constant-time failure paths** — a login for an unknown email still performs
  an Argon2 comparison against a decoy hash, and returns byte-identical errors
  for unknown-email and wrong-password, so the form is not an enumeration oracle.
- **Two brute-force controls** — per-IP rate limiting in `login_attempts`, and a
  per-account lockout on the `users` row that follows the account across IPs.
- **Defence in depth** — `DisallowUnknownFields` blocks mass assignment, request
  bodies are size-capped (Argon2's memory cost scales with input), security
  headers are set, and auth responses are `no-store`.

### ⚠️ Response contract

`internal/httpx` emits the error envelope the frontend parses:

```json
{ "message": "string or string[]", "error": "Bad Request", "statusCode": 400 }
```

**Do not change it** without updating the frontend. Validation messages in
`internal/auth/auth.go` are worded to match the UI's expectations too.

### Migrations

Numbered `.up.sql` / `.down.sql` pairs in `migrations/`, embedded into the binary
and applied at boot when `RUN_MIGRATIONS=true`.

- Tracked in `schema_migrations`, with a checksum. **Editing an applied migration
  makes the server refuse to start** — write a new one.
- Serialised by a Postgres advisory lock, so several instances can boot at once.
- The full schema is documented in the root [`AGENTS.md`](../AGENTS.md#database).
  **Update it in the same commit as any schema change.**

## Env

```bash
cp .env.example .env
```

`DATABASE_URL` is required. `JWT_SECRET` is optional in dev (a random key is
generated per boot, which invalidates sessions on restart) but **required** when
`NODE_ENV=production`.

## Status

Auth and the database are wired end to end. No domain modules yet.