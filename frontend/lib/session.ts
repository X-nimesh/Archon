import 'server-only';
import { cookies } from 'next/headers';

/**
 * Two httpOnly cookies mirroring the backend's token pair.
 *
 * The access token is short-lived (15 min) and is sent as a bearer header by
 * `dal.ts`. The refresh token is long-lived (30 days) and is only ever used to
 * mint a new access token.
 *
 * Neither is readable from client-side JavaScript.
 */
export const ACCESS_COOKIE = 'archon_access';
export const REFRESH_COOKIE = 'archon_refresh';

const FALLBACK_ACCESS_TTL = 15 * 60;
const FALLBACK_REFRESH_TTL = 30 * 24 * 60 * 60;

// Exported so the /api/session Route Handler, which cannot use next/headers to
// build a response, can set the same lifetimes.
export { FALLBACK_ACCESS_TTL, FALLBACK_REFRESH_TTL };

/** Cookies are secure in production only; on plain http://localhost a secure
 *  cookie would be dropped by the browser and login would appear to silently fail. */
function secure(): boolean {
  return process.env.NODE_ENV === 'production';
}

export async function createSession(pair: {
  accessToken: string;
  refreshToken: string;
  expiresIn: number;
}): Promise<void> {
  const store = await cookies();

  store.set(ACCESS_COOKIE, pair.accessToken, {
    httpOnly: true,
    secure: secure(),
    sameSite: 'lax',
    path: '/',
    maxAge: pair.expiresIn > 0 ? pair.expiresIn : FALLBACK_ACCESS_TTL,
  });

  // Refresh token is rotated on every use, so its expiry must outlive the
  // access token by a wide margin.
  store.set(REFRESH_COOKIE, pair.refreshToken, {
    httpOnly: true,
    secure: secure(),
    sameSite: 'lax',
    path: '/',
    maxAge: FALLBACK_REFRESH_TTL,
  });
}

export async function readAccessToken(): Promise<string | null> {
  const store = await cookies();
  return store.get(ACCESS_COOKIE)?.value ?? null;
}

export async function readRefreshToken(): Promise<string | null> {
  const store = await cookies();
  return store.get(REFRESH_COOKIE)?.value ?? null;
}

export async function deleteSession(): Promise<void> {
  const store = await cookies();
  store.delete(ACCESS_COOKIE);
  store.delete(REFRESH_COOKIE);
}