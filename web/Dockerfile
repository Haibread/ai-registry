FROM node:22-alpine AS deps
WORKDIR /app
# Build context is the repo root; web sources live under web/
COPY web/package.json web/package-lock.json* ./
RUN npm ci

FROM node:22-alpine AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
# Copy web sources
COPY web/ .
# Copy API spec so `npm run generate` can regenerate types at build time.
# Path must match the generate script: openapi-typescript ../api/openapi.yaml
COPY api/openapi.yaml /api/openapi.yaml
RUN npm run generate || true
ENV NEXT_TELEMETRY_DISABLED=1
RUN npm run build

FROM node:22-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
RUN addgroup --system --gid 1001 nodejs && adduser --system --uid 1001 nextjs
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static
COPY --from=builder /app/public ./public
USER nextjs
EXPOSE 3000
ENV PORT=3000
ENV HOSTNAME=0.0.0.0
CMD ["node", "server.js"]
