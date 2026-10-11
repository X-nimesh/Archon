/**
 * The stored user record. `passwordHash` never leaves the backend — always
 * project to `PublicUser` before sending anything over the wire.
 */
export interface User {
  id: string;
  name: string;
  email: string;
  passwordHash: string;
  createdAt: Date;
}

/** The safe subset of a user that is allowed to cross the API boundary. */
export interface PublicUser {
  id: string;
  name: string;
  email: string;
  createdAt: string;
}

export interface NewUser {
  name: string;
  email: string;
  passwordHash: string;
}

export function toPublicUser(user: User): PublicUser {
  return {
    id: user.id,
    name: user.name,
    email: user.email,
    createdAt: user.createdAt.toISOString(),
  };
}