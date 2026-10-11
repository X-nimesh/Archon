# The Arcon

> In the hush between stars, a name echoes: **The Arcon**.

![Cover Photo Placeholder](https://via.placeholder.com/1200x500?text=+)

The doors are not locked.  
They are waiting.

---

## Development

Monorepo — [Turborepo](https://turbo.build) + [pnpm workspaces](https://pnpm.io/workspaces).

| Package | Stack | Dev port |
|---|---|---|
| [`backend/`](./backend) — `@archon/backend` | NestJS 12 on Fastify, TypeScript 6 | 3001 |
| [`frontend/`](./frontend) — `@archon/frontend` | Next.js 16 App Router, React 19 | 3000 |

### Get started

```bash
pnpm install   # run once, from the repo root
pnpm dev       # starts backend + frontend together
```

`pnpm dev` runs both servers concurrently behind the turbo dashboard, so you get one
command and one combined log stream. Stop everything with `Ctrl+C`.

### All commands

Run these from the **repo root** — they fan out across packages via turbo.

| Command | What it does |
|---|---|
| `pnpm dev` | Both apps concurrently, live logs |
| `pnpm dev:backend` / `pnpm dev:frontend` | One app only |
| `pnpm build` | Cached, topological production build |
| `pnpm lint` | oxlint (backend) + eslint (frontend) |
| `pnpm typecheck` | `tsc --noEmit` |
| `pnpm test` | Unit tests |
| `pnpm check` | lint + typecheck + test — run this before opening a PR |
| `pnpm graph` | Interactive build-dependency graph |

### Adding a package

```bash
mkdir packages/my-service
```

Then add `packages/*` to the globs in `pnpm-workspace.yaml` and run `pnpm install`.
Turbo picks it up automatically as long as its scripts match task names in `turbo.json`.

### Contributing with AI agents

[`AGENTS.md`](./AGENTS.md) is the single source of truth for repo conventions and is
understood by Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and OpenCode.
`CLAUDE.md` is just an `@AGENTS.md` pointer — edit `AGENTS.md`, never the shim.

### Env

```bash
cp backend/.env.example backend/.env
cp frontend/.env.example frontend/.env
```

`JWT_SECRET` is optional in development but **required** under `NODE_ENV=production`.
