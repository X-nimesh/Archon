import 'server-only';
import type { LoginInput, SignupInput } from './validations';

/**
 * Server-side client for the Go backend.
 *
 * Every call happens on the server, so it bypasses CORS entirely and keeps the
 * backend unreachable from the browser.
 */
const BACKEND_URL = process.env.BACKEND_URL ?? 'http://localhost:3001';

export interface ApiUser {
  id: string;
  name: string;
  email: string;
  createdAt: string;
}

/** The backend's TokenPair. */
export interface TokenPair {
  accessToken: string;
  refreshToken: string;
  /** Access-token lifetime in seconds. */
  expiresIn: number;
  tokenType: string;
  user: ApiUser;
}

/** A non-2xx response from the backend, with its status preserved. */
export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

/**
 * The backend returns `{ message: string | string[], ... }`. Collapse both
 * shapes into one readable sentence for display in the form.
 */
async function toApiError(response: Response): Promise<ApiError> {
  let message = `Request failed with status ${response.status}`;

  try {
    const body: unknown = await response.json();

    if (body && typeof body === 'object' && 'message' in body) {
      const raw = (body as { message: unknown }).message;

      if (Array.isArray(raw)) {
        message = raw.filter((m): m is string => typeof m === 'string').join(' ');
      } else if (typeof raw === 'string') {
        message = raw;
      }
    }
  } catch {
    // Non-JSON error body — keep the generic message.
  }

  return new ApiError(message, response.status);
}

async function request<T>(
  path: string,
  body: unknown,
  init?: RequestInit,
): Promise<T> {
  const response = await fetch(`${BACKEND_URL}${path}`, {
    ...init,
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...init?.headers },
    body: JSON.stringify(body),
    // Auth state must never be served from a stale cache.
    cache: 'no-store',
  });

  if (!response.ok) {
    throw await toApiError(response);
  }

  // 204 No Content (logout).
  if (response.status === 204) {
    return undefined as T;
  }

  return (await response.json()) as T;
}

export const api = {
  signup(input: SignupInput): Promise<TokenPair> {
    return request<TokenPair>('/auth/signup', input);
  },

  login(input: LoginInput): Promise<TokenPair> {
    return request<TokenPair>('/auth/login', input);
  },

  refresh(refreshToken: string): Promise<TokenPair> {
    return request<TokenPair>('/auth/refresh', { refreshToken });
  },

  logout(refreshToken: string): Promise<void> {
    return request<void>('/auth/logout', { refreshToken });
  },

  /** Resolves the current user from an access token. Throws 401 when invalid. */
  async me(accessToken: string): Promise<ApiUser> {
    const response = await fetch(`${BACKEND_URL}/auth/me`, {
      headers: { Authorization: `Bearer ${accessToken}` },
      cache: 'no-store',
    });

    if (!response.ok) {
      throw await toApiError(response);
    }

    return (await response.json()) as ApiUser;
  },
};