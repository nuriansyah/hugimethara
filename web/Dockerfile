# syntax=docker/dockerfile:1.7
# Build context is the repository root: the web app depends on workspace packages.
FROM node:24-alpine AS deps
WORKDIR /repo
RUN corepack enable
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY web/package.json ./web/package.json
COPY packages/config/package.json ./packages/config/package.json
COPY packages/shared-types/package.json ./packages/shared-types/package.json
COPY packages/api-client/package.json ./packages/api-client/package.json
COPY packages/ui/package.json ./packages/ui/package.json
RUN --mount=type=cache,id=pnpm,target=/root/.local/share/pnpm/store pnpm install --frozen-lockfile

FROM node:24-alpine AS build
WORKDIR /repo
RUN corepack enable
ENV NEXT_TELEMETRY_DISABLED=1
COPY --from=deps /repo ./
COPY . .
RUN pnpm --filter hugimethara-web build

FROM node:24-alpine AS run
WORKDIR /app
ENV NODE_ENV=production NEXT_TELEMETRY_DISABLED=1 PORT=3000 HOSTNAME=0.0.0.0
COPY --from=build --chown=node:node /repo/web/.next/standalone ./
WORKDIR /app/web
COPY --from=build --chown=node:node /repo/web/.next/static ./.next/static
COPY --from=build --chown=node:node /repo/web/public ./public
USER node
EXPOSE 3000
CMD ["node", "server.js"]
