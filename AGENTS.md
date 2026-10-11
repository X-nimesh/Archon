# Archon — Agent Guide

Single source of truth for **every** coding agent working in this repo.

> If you are Claude Code: `CLAUDE.md` is just `@AGENTS.md`. Edit **this file**, never the shim.

## Repo layout

Monorepo managed by [Turborepo](https://turbo.build) + [pnpm workspaces](https://pnpm.io/workspaces).

```
Archon/
├── package.json        # workspace root — scripts run turbo, never a package directly
├── pnpm-workspace.yaml # package globs + pnpm build allowlist
├── turbo.json          # task graph, caching, persistent-task flags
├── AGENTS.md           # this file
├── CLAUDE.md           # shim → @AGENTS.md
├── backend/            # @archon/backend — NestJS 12 on Fastify
└── frontend/           # @archon/frontend — Next.js 16 (App Router) + React 19
```

## Commands — always run from the repo root

```bash
pnpm install          # never `npm install`, never install inside a package folder

pnpm dev              # runs backend + frontend concurrently, live dashboard
pnpm dev:backend      # just the API
pnpm dev:frontend     # just the UI

pnpm build            # cached, topological
pnpm check            # lint + typecheck + test across all packages
pnpm test             # unit tests
pnpm typecheck        # tsc --noEmit
pnpm clean            # wipe turbo + node_modules caches
```

`pnpm dev` = `turbo run dev`. `dev` and `start` are **persistent** tasks
(`cache: false`, `persistent: true`), so both servers start concurrently and stream
live logs in the turbo TUI. `build`, `lint`, `test`, `typecheck` are cached — turbo
skips them when inputs are unchanged.

## Ports

| Package | Port | Env var |
|---|---|---|
| `@archon/frontend` | 3000 | `PORT` |
| `@archon/backend` | 3001 | `PORT` (see `backend/.env.example`) |

**Both apps are up with one command. They must never collide on a port.**

## Rules

1. **Never run package scripts from inside `backend/` or `frontend/`.** Use the root
   scripts so the task graph, caching, and env wiring stay correct.
2. **Never edit `CLAUDE.md` to add instructions.** Put them in `AGENTS.md`.
3. **One lockfile.** `pnpm-lock.yaml` at the root only. Do not commit a nested lockfile.
4. **Keep `turbo.json` tasks in sync** with the scripts in each `package.json`.
   A script with no matching task is invisible to turbo and will not be cached.
5. **Only the `dev` and `start` tasks are long-running.** Everything else must exit —
   turbo treats a task that exits 0 as complete and will happily start dependents.
6. Add `-` prefix (`"lint": "- eslint"`) if a script's output is useless noise in the TUI.

## Stack specifics

### `@archon/backend`

- NestJS 12, Fastify adapter, ESM (`"type": "module"`), TypeScript 6, `nodenext` resolution.
- Modules live in `src/`. `src/main.ts` is the entrypoint.
- Lint with **oxlint** (not eslint): `pnpm --filter @archon/backend lint`.
- Format with **prettier** (`.prettierrc` at the package root).
- Tests: **vitest** (`vitest.config.ts`), e2e via `vitest.config.e2e.ts`.
- `dist/` is build output — never edit or commit it.
- Env: copy `backend/.env.example` → `backend/.env`. Never commit `.env`.

### `@archon/frontend`

- Next.js 16 App Router, React 19, TypeScript, `bundler` module resolution.
- Path alias `@/*` → the `frontend/` root.
- ⚠️ **This is not the Next.js you remember.** Next 16 has breaking changes. Before
  writing any code, read the relevant guide in `frontend/node_modules/next/dist/docs/`.
  Do not rely on training-data assumptions about Next APIs or file conventions.
- `frontend/AGENTS.md` holds auto-generated Next.js rules that `next dev` rewrites.
  Leave it alone; commit changes to it alongside your work so the tree stays clean.
- Lint with **eslint** (`eslint.config.mjs`).

## Auth

Email + password, with the backend as the only authority on identity.

```
browser ──form POST──> Server Action ──HTTP──> NestJS /auth/*
                            │                      ├─ bcrypt hash + verify
                            │                      └─ issues JWT (7d)
                            └── sets httpOnly cookie `archon_session`
```

| Piece | Location |
|---|---|
| Signup / login / logout | `backend/src/auth/` |
| `POST /auth/signup`, `POST /auth/login`, `GET /auth/me` | `backend/src/auth/auth.controller.ts` |
| JWT bearer guard | `backend/src/auth/jwt-auth.guard.ts` |
| User store (**in-memory**) | `backend/src/users/users.service.ts` |
| Server-only backend client | `frontend/lib/api.ts` |
| Session cookie read/write | `frontend/lib/session.ts` |
| Authoritative auth check | `frontend/lib/dal.ts` (`verifySession`) |
| Optimistic route guard | `frontend/proxy.ts` |
| Server Actions | `frontend/app/actions/auth.ts` |
| Shared Zod schemas | `frontend/lib/validations.ts` |

Rules that matter when touching auth:

1. **Two layers of protection, on purpose.** `proxy.ts` only checks that the
   `archon_session` cookie *exists* — it runs on every request including
   prefetches, so it must not hit the database. `verifySession()` in `lib/dal.ts`
   is the real check. **Every protected page must call `verifySession()`**;
   proxy alone is not authorization.
2. **The browser must never see the token.** It is stored `httpOnly` and only
   ever read server-side. Do not pass session state into Client Components.
3. **`redirect()` throws.** Keep it outside `try/catch` in Server Actions — a
   surrounding catch will swallow the navigation and the form will just sit there.
4. **Validation runs twice** — Zod on the frontend, `class-validator` via the
   global `ValidationPipe` on the backend. Keep both.
5. **`UsersService` is in-memory.** Accounts vanish on restart. That is the
   intended seam for the PostgreSQL swap; keep auth code talking to the service,
   never to the Maps directly.
6. `NEXT_PUBLIC_*` is not needed anywhere. The API is server-only by design.

## Conventions

- Match the style of the surrounding file over any personal preference.
- Prefer editing existing files over creating new ones.
- Keep changes scoped. Unrelated refactors in a diff hide real changes.
- No secrets, tokens, or `.env` contents in code or commits.

## Env

Each package has a `.env.example` to copy from — `backend/.env.example` and
`frontend/.env.example`. `JWT_SECRET` is the only one you must actually set for
anything beyond local dev, and it is mandatory under `NODE_ENV=production`.