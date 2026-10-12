'use server';

import { redirect } from 'next/navigation';
import { api, ApiError } from '@/lib/api';
import { createSession, deleteSession, readRefreshToken } from '@/lib/session';
import {
  LoginSchema,
  SignupSchema,
  type FieldErrors,
} from '@/lib/validations';

/** Returned to the client form; a successful action redirects instead. */
export type AuthFormState =
  | {
      errors?: FieldErrors;
      message?: string;
    }
  | undefined;

function field(formData: FormData, name: string): unknown {
  return formData.get(name);
}

export async function login(
  _previous: AuthFormState,
  formData: FormData,
): Promise<AuthFormState> {
  const parsed = LoginSchema.safeParse({
    email: field(formData, 'email'),
    password: field(formData, 'password'),
  });

  if (!parsed.success) {
    return { errors: parsed.error.flatten().fieldErrors };
  }

  try {
    const pair = await api.login(parsed.data);
    await createSession(pair);
  } catch (error) {
    return {
      message:
        error instanceof ApiError
          ? error.message
          : 'Could not reach the server. Please try again.',
    };
  }

  // Outside the try/catch on purpose — redirect() signals by throwing, so
  // catching it would swallow the navigation.
  redirect('/dashboard');
}

export async function signup(
  _previous: AuthFormState,
  formData: FormData,
): Promise<AuthFormState> {
  const parsed = SignupSchema.safeParse({
    name: field(formData, 'name'),
    email: field(formData, 'email'),
    password: field(formData, 'password'),
  });

  if (!parsed.success) {
    return { errors: parsed.error.flatten().fieldErrors };
  }

  try {
    const pair = await api.signup(parsed.data);
    await createSession(pair);
  } catch (error) {
    return {
      message:
        error instanceof ApiError
          ? error.message
          : 'Could not reach the server. Please try again.',
    };
  }

  redirect('/dashboard');
}

export async function logout(): Promise<void> {
  const refreshToken = await readRefreshToken();

  // Revoke server-side even if the cookie was already gone, so a token the
  // browser forgot cannot stay alive.
  if (refreshToken) {
    try {
      await api.logout(refreshToken);
    } catch {
      // Best effort: the local session must be cleared either way.
    }
  }

  await deleteSession();
  redirect('/login');
}