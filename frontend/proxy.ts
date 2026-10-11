import { NextResponse, type NextRequest } from 'next/server';

const SESSION_COOKIE = 'archon_session';

/** Reachable without a session. */
const PUBLIC_ROUTES = ['/', '/login', '/signup'];

function isPublic(pathname: string): boolean {
  return PUBLIC_ROUTES.includes(pathname);
}

/**
 * Optimistic route guard.
 *
 * This only checks whether the session cookie is *present* — it cannot confirm
 * the token is still valid without a backend round-trip. That is intentional:
 * proxy runs on every request including prefetches, so it must stay cheap. The
 * authoritative check lives in `lib/dal.ts` (`verifySession`), which each
 * protected page calls.
 *
 * Next.js 16: this file was `middleware.ts` before v16.
 */
export function proxy(request: NextRequest) {
  const { pathname } = request.nextUrl;
  const hasSessionCookie = Boolean(request.cookies.get(SESSION_COOKIE)?.value);

  if (!hasSessionCookie && !isPublic(pathname)) {
    return NextResponse.redirect(new URL('/login', request.url));
  }

  if (hasSessionCookie && isPublic(pathname)) {
    return NextResponse.redirect(new URL('/dashboard', request.url));
  }

  return NextResponse.next();
}

export const config = {
  // Skip API routes, Next internals, and static assets.
  matcher: ['/((?!api|_next/static|_next/image|favicon.ico|.*\\.(?:svg|png|jpg|jpeg|gif|webp)$).*)'],
};