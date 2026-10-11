# Archon Backend

Archon's backend API — will eventually handle requirement parsing, architecture
generation, plan/apply orchestration, and agent activity.

Part of the Archon monorepo. See the [root README](../README.md) for all commands.

## Stack

NestJS 12 on Fastify, TypeScript 6 (ESM). PostgreSQL (not yet connected).

## Commands

Install from the **repo root** first, then run these:

```bash
pnpm dev:backend
pnpm --filter @archon/backend build
pnpm --filter @archon/backend test
```

Dev server runs on **port 3001** (`PORT` in `.env`).

## Auth

Email + password. The backend is the only authority on identity — it hashes
with bcrypt and issues a 7-day JWT that the frontend stores in an httpOnly cookie.

| Route | Auth | Purpose |
|---|---|---|
| `POST /auth/signup` | none | Create an account, returns `{ accessToken, user }` |
| `POST /auth/login` | none | Returns `{ accessToken, user }` |
| `GET /auth/me` | bearer | Resolves the current user, 401 otherwise |

Protect a route with `@UseGuards(JwtAuthGuard)` and read the caller with
`@CurrentUser()`. See `src/auth/`.

### ⚠️ Users are stored in memory

`UsersService` keeps accounts in a `Map`, so **every account disappears when the
backend restarts**. That is deliberate — it keeps the persistence seam obvious
until PostgreSQL is connected. Swap the two Maps for a repository and nothing
else in the auth module needs to change.

### Env

```bash
cp .env.example .env
```

`JWT_SECRET` is optional in dev (a random one is generated per boot, which
invalidates sessions on restart) but **required** when `NODE_ENV=production`.

## Status

Auth is wired end to end. No domain modules beyond Nest's default yet.