import type { PublicUser } from '../users/user.entity.js';

/** Shape returned by `POST /auth/signup` and `POST /auth/login`. */
export interface AuthResponse {
  accessToken: string;
  user: PublicUser;
}

/** Claims embedded in the issued JWT. */
export interface JwtPayload {
  sub: string;
  email: string;
}