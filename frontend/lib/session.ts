import 'server-only';
import { cookies } from 'next/headers';

export const SESSION_COOKIE = 'archon_session';

/** Matches the backend's 7-day token lifetime. */
const MAX_AGE_SECONDS = 60 * 60 * 24 * 7;

/**
 * Stores the backend-issued JWT in an httpOnly cookie so client-side
 * JavaScript can never read it. The browser only ever holds an opaque value.
 */
export async function createSession(accessToken: string): Promise<void> {
  const cookieStore = await cookies();

  cookieStore.set(SESSION_COOKIE, accessToken, {
    httpOnly: true,
    secure: process.env.NODE_ENV === 'production',
    sameSite: 'lax',
    path: '/',
    maxAge: MAX_AGE_SECONDS,
  });
}

export async function readSessionToken(): Promise<string | null> {
  const cookieStore = await cookies();
  return cookieStore.get(SESSION_COOKIE)?.value ?? null;
}

export async function deleteSession(): Promise<void> {
  const cookieStore = await cookies();
  cookieStore.delete(SESSION_COOKIE);
}