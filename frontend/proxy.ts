import { NextResponse, type NextRequest } from 'next/server';

const REFRESH_COOKIE = 'archon_refresh';

/** Routes that require a session cookie to be present. */
const PROTECTED_PREFIXES = ['/dashboard'];

/**
 * Optimistic route guard.
 *
 * Checks only whether a refresh cookie is *present* — it cannot validate the
 * token without a backend round-trip. That is intentional: proxy runs on every
 * request including prefetches, so it must stay cheap. The authoritative check
 * lives in `lib/dal.ts` (`verifySession`), which each protected page calls.
 *
 * The *refresh* cookie is the one to check, not the access cookie: the access
 * token expires after 15 minutes, and keying off it would bounce a signed-in
 * user to /login every quarter hour.
 *
 * It deliberately does NOT redirect signed-in users away from public routes.
 * Doing both directions is what the Next.js guide suggests, but that example
 * works only because proxy can decrypt the cookie itself. Ours is an opaque
 * backend token, so proxy cannot tell a valid cookie from a stale one. A stale
 * cookie (e.g. after a backend restart regenerates JWT_SECRET) would then bounce
 * /dashboard -> /login -> /dashboard forever.
 *
 * Instead, /login and /signup call `getUser()` and redirect on their own when
 * the session is genuinely valid. One-way here, authoritative there.
 *
 * Next.js 16: this file was `middleware.ts` before v16.
 */
export function proxy(request: NextRequest) {
  const { pathname } = request.nextUrl;
  const isProtected = PROTECTED_PREFIXES.some((p) => pathname.startsWith(p));

  if (isProtected && !request.cookies.get(REFRESH_COOKIE)?.value) {
    return NextResponse.redirect(new URL('/login', request.url));
  }

  return NextResponse.next();
}

export const config = {
  // Skip API routes, Next internals, and static assets.
  matcher: ['/((?!api|_next/static|_next/image|favicon.ico|.*\\.(?:svg|png|jpg|jpeg|gif|webp)$).*)'],
};