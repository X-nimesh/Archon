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
`app/actions/auth.ts`, which call the **Go** backend (`../backend-go`, port 3001)
server-side and store the returned tokens in two `httpOnly` cookies:
`archon_access` (15 min) and `archon_refresh` (30 days, rotated on every use).

Three guards, each with a distinct job:

- `proxy.ts` — optimistic. Checks only that the refresh cookie *exists*, so it
  stays fast on prefetched routes. This file was `middleware.ts` before Next 16.
- `lib/dal.ts` → `verifySession()` — authoritative. Confirms the access token is
  still valid and the account still exists. **Every protected page must call it.**
- `app/api/session/route.ts` — performs the rotation. It has to be a Route
  Handler because **cookies can only be written in a Server Action or Route
  Handler, never during a render**, and a refresh token is single-use — rotating
  in the DAL would invalidate the caller's own cookie with no way to replace it,
  which the backend treats as theft and answers by revoking the whole session
  family.

`proxy.ts` also redirects one way only (no cookie on a protected route →
`/login`). It deliberately does *not* bounce signed-in users off `/login`, because
it cannot tell a valid cookie from a stale one — doing both directions would loop
forever after a backend restart. See rules 1–3 in the root `AGENTS.md`.

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