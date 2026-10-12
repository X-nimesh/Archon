import { NextResponse, type NextRequest } from 'next/server';
import { api } from '@/lib/api';
import { ACCESS_COOKIE, FALLBACK_ACCESS_TTL, FALLBACK_REFRESH_TTL, REFRESH_COOKIE } from '@/lib/session';

/**
 * Exchanges the refresh cookie for a fresh token pair, then redirects back to
 * where the user was going.
 *
 * This is a Route Handler rather than inline logic in a Server Component for a
 * hard constraint: **cookies can only be written in a Server Action or a Route
 * Handler**, never during a page render. Refreshing inside `getUser()` would
 * therefore throw.
 *
 * It is also the only place a rotation can safely happen. Because a refresh
 * token is single-use, rotating one during a render would immediately invalidate
 * it — and if the new value could not be persisted, the *next* render would
 * replay a consumed token and trip the backend's reuse detection, which revokes
 * the entire session family. Doing it here, as a single navigation, means
 * exactly one rotation per expired session.
 *
 * Redirects to a full navigation rather than re-rendering in place: a Route
 * Handler cannot resume an RSC render, and this only fires once per 15 minutes.
 */
export async function GET(request: NextRequest) {
  const refreshToken = request.cookies.get(REFRESH_COOKIE)?.value;
  const target = sanitiseTarget(request.nextUrl.searchParams.get('next'));

  if (!refreshToken) {
    return redirectTo('/login');
  }

  let pair;
  try {
    pair = await api.refresh(refreshToken);
  } catch {
    // Expired, revoked, or already used. Drop the dead cookies so the user
    // lands on the login form instead of bouncing here forever.
    const response = redirectTo('/login');
    response.cookies.delete(REFRESH_COOKIE);
    response.cookies.delete(ACCESS_COOKIE);
    return response;
  }

  const response = redirectTo(target);

  // Cookies are set on the response directly: createSession() writes to
  // next/headers, which cannot be used when building a response by hand.
  response.cookies.set(ACCESS_COOKIE, pair.accessToken, {
    httpOnly: true,
    secure: process.env.NODE_ENV === 'production',
    sameSite: 'lax',
    path: '/',
    maxAge: pair.expiresIn > 0 ? pair.expiresIn : FALLBACK_ACCESS_TTL,
  });

  response.cookies.set(REFRESH_COOKIE, pair.refreshToken, {
    httpOnly: true,
    secure: process.env.NODE_ENV === 'production',
    sameSite: 'lax',
    path: '/',
    maxAge: FALLBACK_REFRESH_TTL,
  });

  return response;
}

/**
 * Builds a redirect with a **relative** Location.
 *
 * `NextResponse.redirect()` needs an absolute URL, which it derives from
 * `request.url`. Inside a container that is the container's own origin
 * (`http://localhost:3000`), not the one the browser used — so an absolute
 * Location sends the user to the wrong host entirely. A relative Location is
 * resolved by the browser against the current origin and is correct behind any
 * port mapping or reverse proxy.
 */
function redirectTo(path: string): NextResponse {
  return new NextResponse(null, {
    status: 307,
    headers: { Location: path },
  });
}

/**
 * `next` is attacker-controllable, so it must never be able to send the user
 * off-origin or to a protocol-relative URL.
 */
function sanitiseTarget(next: string | null): string {
  if (!next) return '/dashboard';
  if (!next.startsWith('/')) return '/dashboard';
  if (next.startsWith('//')) return '/dashboard';
  return next;
}