# syntax=docker/dockerfile:1

# ── deps stage ────────────────────────────────────────────────────────────────
FROM node:22-alpine AS deps

RUN corepack enable

# The lockfile is at the monorepo root, so the install has to happen from the
# root too. Only manifests are copied here: anything else would bust the layer
# cache on every source edit.
#
# Build path MUST match the runtime path. pnpm records absolute paths inside
# node_modules/.modules.yaml; installing at /repo and running at /workspace
# makes it believe the tree is stale and try to purge and relink on every boot.
WORKDIR /workspace

COPY package.json pnpm-workspace.yaml pnpm-lock.yaml ./
# Every workspace member's manifest must be present for a frozen install.
COPY backend-go/package.json backend-go/package.json
COPY frontend/package.json frontend/package.json

RUN pnpm install --frozen-lockfile

# ── runtime ───────────────────────────────────────────────────────────────────
FROM node:22-alpine AS runtime

RUN corepack enable

WORKDIR /workspace

# node_modules lives at the paths compose mounts as named volumes. Docker seeds
# a fresh named volume from the image content at that exact path on first start,
# which is what lets the repo be bind-mounted over /workspace without losing the
# installed dependencies.
COPY --from=deps /workspace/node_modules /workspace/node_modules
COPY --from=deps /workspace/frontend/node_modules /workspace/frontend/node_modules

COPY frontend/package.json /workspace/frontend/

ENV NEXT_TELEMETRY_DISABLED=1
ENV HOSTNAME=0.0.0.0

EXPOSE 3000

# Turbopack's file watcher does not see through Docker Desktop's bind mount, so
# the compose file sets the polling fallbacks.
HEALTHCHECK --interval=15s --timeout=5s --start-period=60s --retries=5 \
  CMD wget -qO- http://127.0.0.1:3000/ >/dev/null 2>&1 || exit 1

CMD ["pnpm", "dev"]