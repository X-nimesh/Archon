# The Arcon

> In the hush between stars, a name echoes: **The Arcon**.

<img src='images/archon-cover.png'>

The doors are not locked.  
They are waiting.

---

## Development

Monorepo — [Turborepo](https://turbo.build) + [pnpm workspaces](https://pnpm.io/workspaces) + Docker.

| Package | Stack | Port (Docker) |
|---|---|---|
| [`backend-go/`](./backend-go) — `@archon/backend-go` | Go 1.26, `net/http`, PostgreSQL via pgx | 3001 |
| [`frontend/`](./frontend) — `@archon/frontend` | Next.js 16 App Router, React 19 | 3002 |
| PostgreSQL 18 | Docker, named volume `postgres-data` | 5433 |

> The original NestJS service in [`backend/`](./backend) is retired. It is not in
> the pnpm workspace, so `pnpm dev` / `build` / `check` never touch it.

### Run the whole stack

```bash
cp .env.example .env
docker compose up -d
docker compose ps          # wait for all three to report "healthy"
```

Then open **http://localhost:3002**.

One container per service. The repo root is bind-mounted into `api` and `web`, so
edits hot-reload without a rebuild. Postgres keeps its data in a **named volume**
(not a bind mount) so `docker compose down` can never endanger it.

```bash
docker compose logs -f api   # tail the API
docker compose down          # stop; data survives
docker compose down -v       # stop AND delete the database volume
```

### Or run the apps on your host

```bash
pnpm install
docker compose up -d postgres   # database only
pnpm dev                        # frontend :3000, api :3001
```

### All commands

Run these from the **repo root** — they fan out across packages via turbo.

| Command | What it does |
|---|---|
| `pnpm dev` | Both apps concurrently, live logs |
| `pnpm dev:backend` / `pnpm dev:frontend` | One app only |
| `pnpm build` | Cached, topological production build |
| `pnpm lint` | gofmt (backend) + eslint (frontend) |
| `pnpm typecheck` | `go vet` + `tsc --noEmit` |
| `pnpm test` | Unit tests |
| `pnpm check` | lint + typecheck + test — run this before opening a PR |
| `pnpm format` | gofmt / formatters |

### Tests

Go unit tests need nothing. Integration tests need Postgres and are skipped
without `DATABASE_URL`:

```bash
DATABASE_URL='postgres://archon:archon@localhost:5433/archon?sslmode=disable' \
  go test ./internal/integration/...     # from backend-go/
```

Each integration test creates and drops its own schema, so it never touches dev data.

### Auth

Argon2id password hashing, short-lived access tokens plus rotating refresh
tokens, per-IP rate limiting, and per-account lockout. See
[`AGENTS.md`](./AGENTS.md#auth) for the full map, or
[`backend-go/README.md`](./backend-go/README.md).

### Adding a package

```bash
mkdir packages/my-service
```

Then add `packages/*` to the globs in `pnpm-workspace.yaml` and run `pnpm install`.
Turbo picks it up automatically as long as its scripts match task names in `turbo.json`.

A Go package participates the same way: give it a `package.json` whose scripts
shell out to Go (`go run ./cmd/api`, `go test ./...`), exactly as
`backend-go/package.json` does. Turborepo doesn't need Go-specific config.

### Schema changes

Migrations live in `backend-go/migrations/` as numbered `.up.sql` / `.down.sql`
pairs and run automatically at boot. **Adding or changing a migration means
updating the Database section of [`AGENTS.md`](./AGENTS.md#database) in the same
commit.**

### Contributing with AI agents

[`AGENTS.md`](./AGENTS.md) is the single source of truth for repo conventions and is
understood by Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and OpenCode.
`CLAUDE.md` is just an `@AGENTS.md` pointer — edit `AGENTS.md`, never the shim.
