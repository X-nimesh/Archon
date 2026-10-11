import 'server-only';
import { cache } from 'react';
import { redirect } from 'next/navigation';
import { api, type ApiUser } from './api';
import { readSessionToken } from './session';

/**
 * Returns the signed-in user, or `null` if there is no usable session.
 *
 * `cache` memoizes this per render pass so a page that calls it in several
 * components still makes exactly one backend request.
 */
export const getUser = cache(async (): Promise<ApiUser | null> => {
  const token = await readSessionToken();
  if (!token) return null;

  try {
    // Confirms the token is still signed and the account still exists — the
    // cookie alone only proves it was once valid.
    return await api.me(token);
  } catch {
    return null;
  }
});

/**
 * The guard for protected routes. Redirects to /login unless a live session
 * exists. Returns the user, so callers never need a null check afterwards.
 */
export const verifySession = cache(async () => {
  const user = await getUser();

  if (!user) {
    redirect('/login');
  }

  return { isAuth: true, userId: user.id, user };
});