# Archon Backend (NestJS) — RETIRED

> **This service is retired.** The API is now written in Go and lives in
> [`../backend-go`](../backend-go). This folder is kept for reference and is
> **deliberately excluded from the pnpm workspace**, so `pnpm dev`, `pnpm build`,
> and `pnpm check` never touch it — turbo does not know it exists.
>
> The tree here is frozen as of the Go migration. Its `node_modules/` is stale.
> Do not add features; do not run its scripts expecting them to be part of CI.
>
> Note: port **3001** now belongs to the Go server. Starting Nest here will
> collide with it.

Archon's original backend API — will eventually handle requirement parsing,
architecture generation, plan/apply orchestration, and agent activity.

Part of the Archon monorepo. See the [root README](../README.md) for all commands.

## Stack

NestJS 12 on Fastify, TypeScript 6 (ESM). PostgreSQL (not yet connected).

## Commands

These only work from inside this folder and are **not** part of the workspace:

```bash
npm install
npm run start:dev
```

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