import 'server-only';
import { cache } from 'react';
import { redirect } from 'next/navigation';
import { api, type ApiUser } from './api';
import { readAccessToken, readRefreshToken } from './session';

/**
 * Returns the signed-in user, or `null` when there is no usable session.
 *
 * This never rotates the refresh token. Cookie writes are illegal during a
 * render, and rotating here would invalidate the caller's own cookie without
 * being able to replace it — which the backend would then read as token reuse
 * and revoke the whole session family. Rotation happens in the
 * `/api/session` Route Handler, which is allowed to write cookies.
 *
 * `cache` memoises this per render pass so one page makes at most one call.
 */
export const getUser = cache(async (): Promise<ApiUser | null> => {
  const access = await readAccessToken();
  if (!access) return null;

  try {
    return await api.me(access);
  } catch (error) {
    // A 500 from the backend is not an auth problem, so only swallow 401.
    if (!isAuthFailure(error)) throw error;
    return null;
  }
});

/**
 * The guard for protected routes.
 *
 * If the access token is missing or expired but a refresh cookie exists, the
 * session is still good — it just needs rotating. Hand off to the Route Handler
 * that can perform the swap and send the user onward, rather than throwing them
 * out to the login form.
 */
export const verifySession = cache(async () => {
  const user = await getUser();

  if (!user) {
    if (await readRefreshToken()) {
      redirect('/api/session?next=/dashboard');
    }

    redirect('/login');
  }

  return { isAuth: true, userId: user.id, user };
});

function isAuthFailure(error: unknown): boolean {
  return (
    typeof error === 'object' &&
    error !== null &&
    'status' in error &&
    (error as { status: number }).status === 401
  );
}