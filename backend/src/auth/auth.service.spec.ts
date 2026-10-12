import { ConflictException, UnauthorizedException } from '@nestjs/common';
import { Test, TestingModule } from '@nestjs/testing';
import { JwtModule, JwtService } from '@nestjs/jwt';
import * as bcrypt from 'bcryptjs';
import { AuthService } from './auth.service.js';
import { UsersService } from '../users/users.service.js';

const SECRET = 'test-secret-not-used-in-production';

describe('AuthService', () => {
  let auth: AuthService;
  let users: UsersService;

  beforeEach(async () => {
    const module: TestingModule = await Test.createTestingModule({
      imports: [
        JwtModule.register({ secret: SECRET, signOptions: { expiresIn: '7d' } }),
      ],
      providers: [AuthService, UsersService],
    }).compile();

    auth = module.get(AuthService);
    users = module.get(UsersService);
  });

  const validSignup = {
    name: 'Nimesh',
    email: 'nimesh@archon.dev',
    password: 'supersecret1',
  };

  describe('signup', () => {
    it('returns a user and a token', async () => {
      const result = await auth.signup(validSignup);

      expect(result.user.email).toBe('nimesh@archon.dev');
      expect(result.user).not.toHaveProperty('passwordHash');
      expect(result.accessToken).toEqual(expect.any(String));
    });

    it('never returns the password hash', async () => {
      const { user } = await auth.signup(validSignup);
      expect(JSON.stringify(user)).not.toContain('$2b$');
    });

    it('stores a bcrypt hash, not the raw password', async () => {
      await auth.signup(validSignup);
      const stored = users.findByEmail('nimesh@archon.dev');

      expect(stored?.passwordHash).not.toBe(validSignup.password);
      await expect(bcrypt.compare(validSignup.password, stored!.passwordHash)).resolves.toBe(
        true,
      );
    });

    it('rejects a duplicate email', async () => {
      await auth.signup(validSignup);
      await expect(auth.signup(validSignup)).rejects.toBeInstanceOf(ConflictException);
    });

    it('treats email as case-insensitive', async () => {
      await auth.signup(validSignup);
      await expect(
        auth.signup({ ...validSignup, email: 'NIMESH@ARCHON.DEV' }),
      ).rejects.toBeInstanceOf(ConflictException);
    });
  });

  describe('login', () => {
    beforeEach(async () => {
      await auth.signup(validSignup);
    });

    it('returns a token for correct credentials', async () => {
      const result = await auth.login({
        email: 'nimesh@archon.dev',
        password: 'supersecret1',
      });

      expect(result.user.id).toBeDefined();
      expect(result.accessToken).toEqual(expect.any(String));
    });

    it('accepts a differently-cased email', async () => {
      await expect(
        auth.login({ email: '  Nimesh@ARCHON.DEV  ', password: 'supersecret1' }),
      ).resolves.toMatchObject({ user: { email: 'nimesh@archon.dev' } });
    });

    it('rejects a wrong password', async () => {
      await expect(
        auth.login({ email: 'nimesh@archon.dev', password: 'wrongpassword' }),
      ).rejects.toBeInstanceOf(UnauthorizedException);
    });

    it('rejects an unknown email', async () => {
      await expect(
        auth.login({ email: 'nobody@archon.dev', password: 'supersecret1' }),
      ).rejects.toBeInstanceOf(UnauthorizedException);
    });
  });

  describe('validateToken', () => {
    it('resolves the live user', async () => {
      const { accessToken, user } = await auth.signup(validSignup);
      await expect(auth.validateToken(accessToken)).resolves.toMatchObject({
        id: user.id,
        email: 'nimesh@archon.dev',
      });
    });

    it('rejects a token signed with a different secret', async () => {
      const forged = new JwtService({ secret: 'attacker-secret' }).sign({
        sub: 'some-id',
        email: 'evil@archon.dev',
      });

      await expect(auth.validateToken(forged)).rejects.toBeInstanceOf(
        UnauthorizedException,
      );
    });

    it('rejects garbage', async () => {
      await expect(auth.validateToken('not-a-token')).rejects.toBeInstanceOf(
        UnauthorizedException,
      );
    });

    it('rejects a valid signature for a deleted account', async () => {
      const { accessToken, user } = await auth.signup(validSignup);
      // Simulate the user disappearing while a token is still in flight.
      (users as unknown as { byId: Map<string, unknown> }).byId.delete(user.id);

      await expect(auth.validateToken(accessToken)).rejects.toBeInstanceOf(
        UnauthorizedException,
      );
    });
  });
});