import { ConflictException, Injectable } from '@nestjs/common';
import { randomUUID } from 'node:crypto';
import { NewUser, PublicUser, User, toPublicUser } from './user.entity.js';

@Injectable()
export class UsersService {
  /**
   * In-memory store, keyed by user id and indexed by email.
   *
   * This is the repo's persistence seam: once PostgreSQL is connected (see
   * `backend/.env.example`), swap the two Maps for a TypeORM repository and
   * delete nothing else — the auth module only talks to this service.
   */
  private readonly byId = new Map<string, User>();
  private readonly idByEmail = new Map<string, string>();

  /** Email is stored lowercased + trimmed so lookups are case-insensitive. */
  static normalizeEmail(email: string): string {
    return email.trim().toLowerCase();
  }

  findByEmail(email: string): User | undefined {
    const id = this.idByEmail.get(UsersService.normalizeEmail(email));
    return id ? this.byId.get(id) : undefined;
  }

  findById(id: string): User | undefined {
    return this.byId.get(id);
  }

  async create(input: NewUser): Promise<PublicUser> {
    const email = UsersService.normalizeEmail(input.email);

    if (this.idByEmail.has(email)) {
      throw new ConflictException('An account with that email already exists');
    }

    const user: User = {
      id: randomUUID(),
      name: input.name.trim(),
      email,
      passwordHash: input.passwordHash,
      createdAt: new Date(),
    };

    this.byId.set(user.id, user);
    this.idByEmail.set(email, user.id);

    return toPublicUser(user);
  }
}