import { Injectable, UnauthorizedException } from '@nestjs/common';
import { JwtService } from '@nestjs/jwt';
import * as bcrypt from 'bcryptjs';
import { UsersService } from '../users/users.service.js';
import { PublicUser, toPublicUser } from '../users/user.entity.js';
import { AuthResponse, JwtPayload } from './auth.types.js';
import { LoginDto } from './dto/login.dto.js';
import { SignupDto } from './dto/signup.dto.js';

const SALT_ROUNDS = 10;

@Injectable()
export class AuthService {
  constructor(
    private readonly users: UsersService,
    private readonly jwt: JwtService,
  ) {}

  async signup(dto: SignupDto): Promise<AuthResponse> {
    const passwordHash = await bcrypt.hash(dto.password, SALT_ROUNDS);
    // UsersService throws ConflictException if the email is already taken.
    const user = await this.users.create({
      name: dto.name,
      email: dto.email,
      passwordHash,
    });

    return { accessToken: this.sign(user), user };
  }

  async login(dto: LoginDto): Promise<AuthResponse> {
    const found = this.users.findByEmail(dto.email);

    // Compare against a dummy hash when the user is missing so that a wrong
    // email and a wrong password take the same amount of time to answer,
    // which stops attackers enumerating registered emails by timing.
    const passwordHash = found?.passwordHash ?? DUMMY_HASH;
    const passwordMatches = await bcrypt.compare(dto.password, passwordHash);

    if (!found || !passwordMatches) {
      throw new UnauthorizedException('Invalid email or password');
    }

    return { accessToken: this.sign(found), user: toPublicUser(found) };
  }

  /** Verifies a bearer token and returns the live user record. */
  async validateToken(token: string): Promise<PublicUser> {
    let payload: JwtPayload;
    try {
      payload = await this.jwt.verifyAsync<JwtPayload>(token);
    } catch {
      throw new UnauthorizedException('Invalid or expired session');
    }

    const user = this.users.findById(payload.sub);
    if (!user) {
      // Token signature is valid but the account is gone.
      throw new UnauthorizedException('Invalid or expired session');
    }

    return toPublicUser(user);
  }

  private sign(user: { id: string; email: string }): string {
    const payload: JwtPayload = { sub: user.id, email: user.email };
    return this.jwt.sign(payload);
  }
}

/** A valid bcrypt hash of a value nobody knows — used for constant-time login. */
const DUMMY_HASH = '$2b$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy';