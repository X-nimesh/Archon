# Archon Frontend

Archon's web control-plane UI.

Part of the Archon monorepo. See the [root README](../README.md) for all commands.

## Stack

Next.js 16 (App Router), React 19, TypeScript, Zod.

## Commands

Install from the **repo root** first, then run these:

```bash
pnpm dev:frontend
pnpm --filter @archon/frontend build
pnpm --filter @archon/frontend lint
```

Dev server runs on **port 3000**.

## Pages

| Route | Access | Notes |
|---|---|---|
| `/` | public | Landing; shows different CTAs when signed in |
| `/login` | public | Redirects to `/dashboard` if already signed in |
| `/signup` | public | Same |
| `/dashboard` | **protected** | Calls `verifySession()` |

## Auth

The browser never talks to the API. Forms POST to Server Actions in
`app/actions/auth.ts`, which call the backend server-side and store the returned
JWT in the httpOnly `archon_session` cookie.

Two guards, deliberately:

- `proxy.ts` — optimistic. Checks only that the cookie *exists*, so it stays fast
  on prefetched routes. This file was `middleware.ts` before Next 16.
- `lib/dal.ts` → `verifySession()` — authoritative. Confirms the token is still
  valid and the account still exists, then redirects to `/login` otherwise.
  **Every protected page must call it.**

## Notes for agents and new contributors

`AGENTS.md` here is **generated and rewritten by `next dev`**. Commit its changes
alongside your work rather than fighting it.

Next.js 16 has breaking changes versus older versions — `middleware.ts` became
`proxy.ts`, and `cookies()` is async. Check `node_modules/next/dist/docs/` before
assuming an API behaves the way it used to.

## Status

Auth, dashboard, and landing are wired. No domain features yet.

## Env

```bash
cp .env.example .env
```

`BACKEND_URL` only needs changing if the API is not on `http://localhost:3001`.

> The session cookie is marked `secure` whenever `NODE_ENV=production`. Browsers
> drop secure cookies on plain `http://`, so `pnpm start` against a production
> build only completes the login flow over HTTPS or on `localhost` in a browser
> that treats it as a secure context. Use `pnpm dev` for local work.