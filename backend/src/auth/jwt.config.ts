import { Logger } from '@nestjs/common';
import { randomBytes } from 'node:crypto';

/**
 * Resolves the JWT signing secret.
 *
 * In production a missing secret is fatal — silently falling back would let the
 * app boot with a guessable signing key. In development we generate a random
 * secret instead so the app runs without setup, at the cost of invalidating
 * existing sessions on every restart.
 */
export function resolveJwtSecret(): string {
  const secret = process.env.JWT_SECRET?.trim();

  if (secret) return secret;

  if (process.env.NODE_ENV === 'production') {
    throw new Error(
      'JWT_SECRET is required when NODE_ENV=production. Generate one with: openssl rand -base64 32',
    );
  }

  const logger = new Logger('JwtConfig');
  logger.warn(
    'JWT_SECRET is not set — using a random dev secret. Sessions will not survive a restart.',
  );

  return randomBytes(32).toString('hex');
}