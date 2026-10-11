import {
  CanActivate,
  ExecutionContext,
  Injectable,
  UnauthorizedException,
  createParamDecorator,
} from '@nestjs/common';
import { PublicUser } from '../users/user.entity.js';
import { AuthService } from './auth.service.js';

/**
 * A request that has passed {@link JwtAuthGuard}.
 *
 * Structurally typed rather than importing `FastifyRequest`: Nest does not
 * re-export the Fastify types, and `getHttpRequest()` is a plain cast, so the
 * guard only ever touches these two properties.
 */
export interface AuthenticatedRequest {
  headers: { authorization?: string };
  user: PublicUser;
}

function extractBearer(header: string | undefined): string | null {
  if (!header) return null;

  const [scheme, token] = header.split(' ');

  if (scheme?.toLowerCase() !== 'bearer' || !token) {
    return null;
  }

  return token.trim();
}

/**
 * Verifies the `Authorization: Bearer <jwt>` header and attaches the caller to
 * the request. Apply with `@UseGuards(JwtAuthGuard)`.
 */
@Injectable()
export class JwtAuthGuard implements CanActivate {
  constructor(private readonly auth: AuthService) {}

  async canActivate(context: ExecutionContext): Promise<boolean> {
    const request = context.switchToHttp().getRequest<AuthenticatedRequest>();
    const token = extractBearer(request.headers.authorization);

    if (!token) {
      throw new UnauthorizedException('Missing bearer token');
    }

    request.user = await this.auth.validateToken(token);
    return true;
  }
}

/** Injects the authenticated user resolved by {@link JwtAuthGuard}. */
export const CurrentUser = createParamDecorator(
  (_data: unknown, context: ExecutionContext): PublicUser => {
    const request = context.switchToHttp().getRequest<AuthenticatedRequest>();
    return request.user;
  },
);