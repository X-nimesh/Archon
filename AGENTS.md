# Archon — Agent Guide

Single source of truth for **every** coding agent working in this repo.

> If you are Claude Code: `CLAUDE.md` is just `@AGENTS.md`. Edit **this file**, never the shim.

## Repo layout

Monorepo managed by [Turborepo](https://turbo.build) + [pnpm workspaces](https://pnpm.io/workspaces).

```
Archon/
├── compose.yaml        # postgres + api + web, one container each
├── Dockerfile?         # backend-go/Dockerfile and frontend/Dockerfile
├── package.json        # workspace root — scripts run turbo, never a package directly
├── pnpm-workspace.yaml # package globs + pnpm build allowlist
├── turbo.json          # task graph, caching, persistent-task flags
├── AGENTS.md           # this file
├── CLAUDE.md           # shim → @AGENTS.md
├── backend-go/         # @archon/backend-go — the API. Go 1.26, net/http, pgx
├── frontend/           # @archon/frontend — Next.js 16 (App Router) + React 19
└── backend/            # RETIRED NestJS service. Not in the workspace. Ignore it.
```

`backend/` is the retired NestJS implementation, kept only for reference. It is
deliberately **absent from `pnpm-workspace.yaml`**, so turbo and pnpm ignore it
entirely — it never builds, lints, tests, or starts. Do not re-add it, and do not
add work to it. Its `node_modules/` is stale. Port 3001 belongs to `backend-go`.

## Commands — always run from the repo root

### Everything in Docker

```bash
cp .env.example .env      # once
docker compose up -d      # postgres + api + web, one container each
docker compose ps         # wait for "healthy"
docker compose logs -f api
docker compose down       # stop; data survives
docker compose down -v    # stop AND delete the database volume
```

The repo root is bind-mounted into the app containers, so edits on the host
hot-reload without a rebuild. Postgres uses a **named volume** instead — a bind
mount there would put database files on the host and make `down` unsafe.

### Native (database only in Docker)

```bash
pnpm install
docker compose up -d postgres   # only the database
pnpm dev                        # both apps on the host
```

### Package scripts

```bash
pnpm dev              # backend + frontend concurrently, live logs
pnpm dev:backend      # just the API
pnpm dev:frontend     # just the UI

pnpm build            # cached, topological
pnpm check            # lint + typecheck + test across all packages
pnpm test             # unit tests
pnpm typecheck        # tsc --noEmit / go vet
pnpm clean            # wipe turbo caches and node_modules
```

`pnpm dev` = `turbo run dev`. `dev` and `start` are **persistent** tasks
(`cache: false`, `persistent: true`), so both servers start concurrently and stream
live logs in the turbo TUI. `build`, `lint`, `test`, `typecheck` are cached — turbo
skips them when inputs are unchanged.

### Go tests against a real database

Unit tests need nothing. The integration suite needs Postgres and is skipped
without it:

```bash
DATABASE_URL='postgres://archon:archon@localhost:5433/archon?sslmode=disable' \
  go test ./internal/integration/...
```

Each test creates and drops its own schema, so it never touches dev data.

## Ports

Two ways to run the stack. Ports differ between them.

| Service | `pnpm dev` (host) | Docker | Container port |
|---|---|---|---|
| `@archon/frontend` | 3000 | 3002 | 3000 |
| `@archon/backend-go` | 3001 | 3001 | 3001 |
| PostgreSQL | 5433 | 5433 | 5432 |

Docker ports are remapped because 3000 and 5432 are usually already taken.
Override them in the gitignored root `.env`. **Inside the compose network the
containers always use the standard ports** (`api:3001`, `postgres:5432`), so
`BACKEND_URL` and `DATABASE_URL` are injected and should not be edited by hand.

**Both apps are up with one command. They must never collide on a port.**

## Rules

1. **Never run package scripts from inside `backend-go/` or `frontend/`.** Use the root
   scripts so the task graph, caching, and env wiring stay correct.
2. **Never edit `CLAUDE.md` to add instructions.** Put them in `AGENTS.md`.
3. **One lockfile.** `pnpm-lock.yaml` at the root only. Do not commit a nested lockfile.
4. **Keep `turbo.json` tasks in sync** with the scripts in each `package.json`.
   A script with no matching task is invisible to turbo and will not be cached.
5. **Only the `dev` and `start` tasks are long-running.** Everything else must exit —
   turbo treats a task that exits 0 as complete and will happily start dependents.
6. Add `-` prefix (`"lint": "- eslint"`) if a script's output is useless noise in the TUI.
7. **If you change the database schema, update the Database section below in the
   same commit.** A migration without the matching documentation here is
   incomplete work — this file is the only reference for what the schema is.

## Stack specifics

### `@archon/backend-go`

- Go 1.26. **No web framework** — routing uses Go 1.22 method patterns on
  `http.ServeMux` (`mux.HandleFunc("POST /auth/login", …)`).
- Layout: `cmd/api` (entrypoint) → `internal/{config,db,user,auth,session,password,throttle,secure,cors,httpx,server}`.
- Tests: `go test ./...`. Lint: `gofmt` (via `scripts/lint.sh`, which fails on
  unformatted files). Typecheck: `go vet ./...`.
- **Format Go code** with `pnpm --filter @archon/backend-go format` (`gofmt -w .`).
- `pnpm dev:backend` runs `go run ./cmd/api`, which does **not** hot-reload.
- Driver: `pgx/v5` with a `pgxpool`. No ORM — SQL is explicit and reviewed.

### `@archon/frontend`

- Next.js 16 App Router, React 19, TypeScript, `bundler` module resolution.
- Path alias `@/*` → the `frontend/` root.
- Routing: `app/(app)/` is the authenticated shell (sidebar + top bar);
  `app/(auth)/` is the centred card used by /login and /signup.
- ⚠️ **This is not the Next.js you remember.** Next 16 has breaking changes. Before
  writing any code, read the relevant guide in `frontend/node_modules/next/dist/docs/`.
  Do not rely on training-data assumptions about Next APIs or file conventions.
- `frontend/AGENTS.md` holds auto-generated Next.js rules that `next dev` rewrites.
  Leave it alone; commit changes to it alongside your work so the tree stays clean.
- Lint with **eslint** (`eslint.config.mjs`).

## Database

PostgreSQL 18, run by Docker. **Migrations live in `backend-go/migrations/` as
numbered `.up.sql` / `.down.sql` pairs and are embedded into the binary**, so no
loose files are needed at runtime.

### ⚠️ PG18 data layout

The volume is mounted at **`/var/lib/postgresql`**, not `/var/lib/postgresql/data`.

PostgreSQL 18 changed its on-disk layout: the image puts the cluster in a
version-specific subdirectory (`/var/lib/postgresql/18/docker`) and **aborts on
boot if it finds data at the old path**. Mounting the parent directory lets the
image choose the version directory itself, so a future 19 lands beside 18 rather
than colliding, and `pg_upgrade --link` stays inside one mount boundary.

If you ever change the major version here: a PG17-format volume cannot be read
by PG18 in place. Dump first (`pg_dumpall`), recreate the volume, then restore.

```bash
docker compose exec postgres pg_dumpall -U archon --clean > .backups/dump.sql
docker compose down && docker volume rm archon_postgres-data
docker compose up -d postgres && docker compose exec -T postgres psql -U archon -d archon < .backups/dump.sql
```

`--clean` emits harmless errors about the connected database and the current
role; that is expected. Dumps land in `.backups/` (gitignored — they contain
password and refresh-token hashes).

### Migrations

- `NNNN_name.up.sql` and `NNNN_name.down.sql` — every up needs a down.
- Applied automatically at boot when `RUN_MIGRATIONS=true`, and tracked in
  `schema_migrations (version, name, checksum, applied_at)`.
- **Never edit a migration that has already been applied.** The runner compares
  checksums and refuses to start on a mismatch. Write a new migration instead.
- Runs are serialised by a Postgres advisory lock, so multiple instances booting
  at once cannot race.

To add a migration:

```bash
# 0002_add_something.up.sql and .down.sql in backend-go/migrations/
pnpm dev:backend    # applies it on boot
```

### `users`

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| `id` | `UUID` | no | `gen_random_uuid()` | PK. Immutable. |
| `name` | `TEXT` | no | — | `CHECK char_length BETWEEN 1 AND 80`. |
| `email` | `TEXT` | no | — | Stored lowercased+trimmed by the app. |
| `password_hash` | `TEXT` | **yes** | — | Argon2id PHC string. Nullable for future OAuth-only accounts. |
| `credentials_changed_at` | `TIMESTAMPTZ` | no | `now()` | Bumped on password change; invalidates older refresh tokens. |
| `failed_login_count` | `INTEGER` | no | `0` | Drives the account lockout. |
| `locked_until` | `TIMESTAMPTZ` | **yes** | — | Non-null means locked out. |
| `last_login_at` | `TIMESTAMPTZ` | **yes** | — | |
| `disabled_at` | `TIMESTAMPTZ` | **yes** | — | Non-null means the account is closed. |
| `created_at` | `TIMESTAMPTZ` | no | `now()` | |
| `updated_at` | `TIMESTAMPTZ` | no | `now()` | Maintained by the `users_touch_updated_at` trigger. |

Indexes:
- `users_email_key` — **UNIQUE on `lower(email)`**. Case-insensitive uniqueness.
  This index is the authority on duplicate signup, so two concurrent signups
  cannot both win. (An expression index rather than `CITEXT`: both behave the
  same, but CITEXT requires a database-wide extension whose type is only
  resolvable via `search_path`, which breaks under a non-default search path.)
- `users_locked_until_idx` — partial, `WHERE locked_until IS NOT NULL`.

### `refresh_tokens`

| Column | Type | Null | Notes |
|---|---|---|---|
| `id` | `UUID` | no | PK |
| `user_id` | `UUID` | no | FK → `users(id)` `ON DELETE CASCADE` |
| `token_hash` | `TEXT` | no | **UNIQUE.** SHA-256 hex of the token. The plaintext is never stored. |
| `credentials_changed_at` | `TIMESTAMPTZ` | no | Snapshot at issue time; compared against `users` on refresh. |
| `issued_at` | `TIMESTAMPTZ` | no | `now()` |
| `expires_at` | `TIMESTAMPTZ` | no | |
| `rotated_at` | `TIMESTAMPTZ` | **yes** | Set when spent. Non-null + presented ⇒ reuse. |
| `revoked_at` | `TIMESTAMPTZ` | **yes** | Set by logout / reuse detection. |
| `user_agent` | `TEXT` | **yes** | Abuse investigation. |
| `ip_address` | `TEXT` | **yes** | Abuse investigation. |

Indexes: `token_hash` (unique), `refresh_tokens_user_id_idx`,
`refresh_tokens_expires_at_idx`, and partial `refresh_tokens_active_idx`.

### `login_attempts`

| Column | Type | Null | Notes |
|---|---|---|---|
| `id` | `BIGSERIAL` | no | PK |
| `scope` | `TEXT` | no | `CHECK IN ('ip','account')` |
| `key` | `TEXT` | no | SHA-256 of the IP or email — hashed so a leak does not enumerate visitors. |
| `attempted_at` | `TIMESTAMPTZ` | no | `now()` |
| `succeeded` | `BOOLEAN` | no | `FALSE` rows are what the limiter counts. |

Index: `login_attempts_lookup_idx (scope, key, attempted_at DESC)` — the only
query the limiter runs.

### `schema_migrations`

Bookkeeping, created by the runner: `version` (PK), `name`, `checksum`,
`applied_at`.

## Auth

Email + password, with the backend as the only authority on identity.

```
browser ──form POST──> Server Action ──HTTP──> Go /auth/*
                            │                      ├─ Argon2id hash + verify
                            │                      ├─ per-IP throttle + account lockout
                            │                      └─ issues access + refresh pair
                            └── sets two httpOnly cookies
```

| Route | Auth | Purpose |
|---|---|---|
| `POST /auth/signup` | none | Create account → token pair (201) |
| `POST /auth/login` | none | → token pair (200) |
| `POST /auth/refresh` | refresh token | Rotate the pair (200) |
| `POST /auth/logout` | refresh token | Revoke all sessions (204) |
| `GET /auth/me` | access token | Current user (401 otherwise) |
| `GET /health` | none | Liveness |

| Piece | Location |
|---|---|
| Route table | `backend-go/internal/server/server.go` |
| Signup / login / refresh / logout | `backend-go/internal/auth/service.go` |
| HTTP handlers | `backend-go/internal/auth/handler.go` |
| Bearer guard | `backend-go/internal/auth/middleware.go` |
| Argon2id hashing | `backend-go/internal/password/password.go` |
| Access tokens (JWT) | `backend-go/internal/session/access.go` |
| Refresh tokens + rotation | `backend-go/internal/session/refresh.go` |
| Per-IP throttle | `backend-go/internal/throttle/throttle.go` |
| Error envelope (**contract**) | `backend-go/internal/httpx/httpx.go` |
| Migrations + runner | `backend-go/migrations/`, `internal/db/` |
| Security headers | `backend-go/internal/secure/secure.go` |
| Server-only backend client | `frontend/lib/api.ts` |
| Session cookies | `frontend/lib/session.ts` |
| Refresh Route Handler | `frontend/app/api/session/route.ts` |
| Authoritative auth check | `frontend/lib/dal.ts` (`verifySession`) |
| Optimistic route guard | `frontend/proxy.ts` |
| Server Actions | `frontend/app/actions/auth.ts` |
| Shared Zod schemas | `frontend/lib/validations.ts` |
| D2 compile + SVG sanitiser | `frontend/lib/d2.ts` |

Cookies: `archon_access` (15 min) and `archon_refresh` (30 days), both `httpOnly`.

### Rules that matter when touching auth

1. **Two layers of protection.** `proxy.ts` only checks that the refresh cookie
   *exists* — it runs on every request including prefetches, so it must not hit
   the database. `verifySession()` in `lib/dal.ts` is the real check. **Every
   protected page must call it**; proxy alone is not authorization.
2. **Rotation happens in `frontend/app/api/session/route.ts` only.** Cookies
   cannot be written during a render, and a refresh token is single-use — so
   rotating inside `getUser()` would invalidate the caller's own cookie without
   being able to replace it, which the backend reads as **reuse** and responds to
   by revoking the entire session family. Do not "optimise" this into the DAL.
3. **Redirects must use a relative `Location`.** `NextResponse.redirect()` derives
   an absolute URL from `request.url`, which inside a container is the
   container's own origin, not the one the browser used.
4. **Never let a failed request silently succeed.** `verifyPassword` compares
   against a decoy hash when no account exists, so the failure path costs the
   same as the success path. Unknown-email and wrong-password return byte-identical
   responses — the login form must never become an account-enumeration oracle.
5. **Two distinct brute-force controls, on purpose.** Per-IP throttling stops one
   host spraying many accounts; the account lockout on `users` stops a botnet
   hammering one account from many IPs. Do not add a third — an earlier version
   had a per-account rate limit *and* a lockout and they raced to produce the
   error.
6. **The Go error envelope is a contract.** `{"message": string|string[],
   "error": string, "statusCode": number}` is parsed by `frontend/lib/api.ts`.
   Validation strings in `internal/auth/auth.go` match the original NestJS wording
   deliberately. Change one, change both.
7. **The browser must never see a token.** Both cookies are `httpOnly` and read
   server-side only. Do not pass session state into Client Components.
8. **`redirect()` throws.** Keep it outside `try/catch` in Server Actions — a
   surrounding catch will swallow the navigation.
9. **Validation runs twice** — Zod on the frontend, hand-rolled in
   `internal/auth` on the backend. Keep both. `DisallowUnknownFields` on the Go
   side is what blocks mass-assignment (e.g. a `role` field); keep it.
10. `NEXT_PUBLIC_*` is not needed anywhere. The API is server-only by design.

## Diagrams

`/architect` renders an architecture diagram written in [D2](https://d2lang.com),
compiled to SVG at request time by `frontend/lib/d2.ts`.

- **`@terrastruct/d2` is a WASM wrapper**, and three of its behaviours will bite
  anyone who assumes it behaves like a normal library. They are commented at each
  site in `lib/d2.ts`:
  1. **A `D2` instance deadlocks on concurrent `compile()` calls.** It keeps the
     pending resolve callback in a single instance field, so overlapping calls
     overwrite each other and the first never settles. All compiles are therefore
     serialised through a promise chain. Do not "optimise" that into `Promise.all`.
  2. **Each instance spawns a `worker_threads` Worker holding a 22 MB WASM binary
     and there is no public `terminate`.** One lazily-created instance is reused
     for the process lifetime. Creating one per request leaks a thread per render.
  3. **`pad` is broken in v0.1.33** — a non-default value emits a `viewBox` that
     does not match the drawing. Leave it alone.
- **`sanitizeSvg()` is a hard requirement, not defensive padding.** SVG is a
  scripting context: an `<svg onload=…>` executes on parse, so library output is
  untrusted input the moment it is inlined. The sanitiser is reject-or-die — it
  never repairs. If you loosen it, you have removed the only thing standing
  between a diagram and stored XSS.
- **Colour is addressed by D2 slot, not baked in.** `applyDesignTokens()` rewrites
  D2's generated `.fill-N7` rules to `var(--d2-*)` so the diagram is dark-first and
  flips in light mode for free. Do not reintroduce a hex palette.
- A D2 compile error must surface as a readable error panel, never a 500.

> **The D2 source is currently a placeholder.** There is no architecture endpoint
> yet, so `PLACEHOLDER_D2` in `app/(app)/architect/page.tsx` holds a hand-written
> document describing Archon itself. Delete that constant when the API lands.

## Conventions

- Match the style of the surrounding file over any personal preference.
- Prefer editing existing files over creating new ones.
- Keep changes scoped. Unrelated refactors in a diff hide real changes.
- No secrets, tokens, or `.env` contents in code or commits.

## Env

Each package has a `.env.example` to copy from — `backend-go/.env.example` and
`frontend/.env.example`, plus a root `.env.example` for compose variables.
`JWT_SECRET` is the only one you must actually set for anything beyond local dev,
and it is mandatory under `NODE_ENV=production`. The root `.env` is gitignored;
never commit real secrets.