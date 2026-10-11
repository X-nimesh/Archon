import { ValidationPipe } from '@nestjs/common';
import { NestFactory } from '@nestjs/core';
import { FastifyAdapter, NestFastifyApplication } from '@nestjs/platform-fastify';
import { existsSync } from 'node:fs';
import cors from '@fastify/cors';
import { AppModule } from './app.module.js';

// Load backend/.env if present. process.loadEnvFile is built into Node 20.12+,
// so no dotenv dependency is needed.
const envPath = new URL('../.env', import.meta.url).pathname;
if (existsSync(envPath)) {
  process.loadEnvFile(envPath);
}

async function bootstrap() {
  const app = await NestFactory.create<NestFastifyApplication>(
    AppModule,
    new FastifyAdapter(),
  );

  // The frontend is a separate origin (port 3000 vs 3001), so auth requests
  // are cross-origin. Comma-separated list via CORS_ORIGIN.
  await app.register(cors, {
    origin: (process.env.CORS_ORIGIN ?? 'http://localhost:3000').split(','),
    credentials: true,
  });

  app.useGlobalPipes(
    new ValidationPipe({
      whitelist: true,
      forbidNonWhitelisted: true,
      transform: true,
    }),
  );

  app.enableShutdownHooks();

  await app.listen(process.env.PORT ?? 3001, '0.0.0.0');
}
await bootstrap();