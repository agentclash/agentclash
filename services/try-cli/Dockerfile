FROM oven/bun:1.3 AS deps
WORKDIR /app
COPY try-cli/packages/core/package.json try-cli/packages/core/
COPY try-cli/packages/core/src try-cli/packages/core/src
COPY services/try-cli/package.json services/try-cli/
COPY services/try-cli/server services/try-cli/server
WORKDIR /app/services/try-cli
RUN bun install --production

FROM oven/bun:1.3
WORKDIR /app/services/try-cli
COPY --from=deps /app /app
COPY try-cli/demos /app/try-cli/demos
ENV NODE_ENV=production
ENV PORT=3001
EXPOSE 3001
CMD ["bun", "run", "server/index.ts"]
