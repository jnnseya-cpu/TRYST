# syntax=docker/dockerfile:1.7
# Web image (Next.js standalone server), non-root. Build from the repository root so the
# brand assets (assets/brand) are in the context:
#   docker build -f web/Dockerfile --build-arg IDENTITY_URL=http://api:8081 -t tryst-web .
# Rewrites are fixed at build time: IDENTITY_URL is where /v1/* is proxied, SAFETY_URL where
# reports, appeals, standing and the console API go; OPS_CONSOLE=1
# only for the staff-only console build.
ARG NODE_IMAGE=node:22-alpine

FROM ${NODE_IMAGE} AS build
WORKDIR /repo
COPY web/package.json web/package-lock.json web/
RUN --mount=type=secret,id=ca,required=false \
    if [ -f /run/secrets/ca ]; then export NODE_EXTRA_CA_CERTS=/run/secrets/ca; fi; cd web && npm ci --no-audit --no-fund
COPY assets/brand assets/brand
COPY web web
ARG IDENTITY_URL=http://api:8081
ARG SAFETY_URL=http://safety:8082
ARG OPS_CONSOLE=0
ENV NEXT_TELEMETRY_DISABLED=1 IDENTITY_URL=${IDENTITY_URL} SAFETY_URL=${SAFETY_URL} OPS_CONSOLE=${OPS_CONSOLE}
RUN cd web && npm run build

FROM ${NODE_IMAGE}
# The server needs only node: drop the package managers (and their dependency trees, where
# scanners find advisories) from the runtime image.
RUN rm -rf /usr/local/lib/node_modules/npm /usr/local/lib/node_modules/corepack \
      /usr/local/bin/npm /usr/local/bin/npx /usr/local/bin/corepack /opt/yarn* /usr/local/bin/yarn /usr/local/bin/yarnpkg
WORKDIR /app
ENV NODE_ENV=production NEXT_TELEMETRY_DISABLED=1 PORT=3000 HOSTNAME=0.0.0.0
COPY --from=build --chown=node:node /repo/web/.next/standalone ./
COPY --from=build --chown=node:node /repo/web/.next/static ./.next/static
COPY --from=build --chown=node:node /repo/web/public ./public
USER node
EXPOSE 3000
CMD ["node", "server.js"]
