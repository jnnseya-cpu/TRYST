# syntax=docker/dockerfile:1.7
# Web image (Next.js standalone server), non-root. Build from the repository root so the
# brand assets (shared/brand) are in the context:
#   docker build -f frontend/Dockerfile --build-arg IDENTITY_URL=http://api:8081 -t tryst-web .
# Rewrites are fixed at build time: IDENTITY_URL is where /v1/* is proxied, SAFETY_URL where
# reports, appeals, standing and the console API go; OPS_CONSOLE=1
# only for the staff-only console build.
ARG NODE_IMAGE=node:22-alpine
# Lowest Node.js 22 with the 20 Oct 2026 security fixes (remote code execution). Raise it
# with each Node security release; the build fails on anything older.
ARG NODE_MIN=22.23.4

FROM ${NODE_IMAGE} AS build
ARG NODE_MIN
RUN node -e 'const [a,b,c]=process.versions.node.split(".").map(Number),[x,y,z]=process.argv[1].split(".").map(Number);if(a!==x||b<y||(b===y&&c<z)){console.error("Node "+process.versions.node+" is older than "+process.argv[1]+": pull a patched base image");process.exit(1)}' "$NODE_MIN"
WORKDIR /repo
COPY frontend/package.json frontend/package-lock.json frontend/
RUN --mount=type=secret,id=ca,required=false \
    if [ -f /run/secrets/ca ]; then export NODE_EXTRA_CA_CERTS=/run/secrets/ca; fi; cd frontend && npm ci --no-audit --no-fund
COPY shared/brand shared/brand
COPY frontend frontend
ARG IDENTITY_URL=http://api:8081
ARG SAFETY_URL=http://safety:8082
ARG OPS_CONSOLE=0
ENV NEXT_TELEMETRY_DISABLED=1 IDENTITY_URL=${IDENTITY_URL} SAFETY_URL=${SAFETY_URL} OPS_CONSOLE=${OPS_CONSOLE}
RUN cd frontend && npm run build

FROM ${NODE_IMAGE}
ARG NODE_MIN
RUN node -e 'const [a,b,c]=process.versions.node.split(".").map(Number),[x,y,z]=process.argv[1].split(".").map(Number);if(a!==x||b<y||(b===y&&c<z)){console.error("Node "+process.versions.node+" is older than "+process.argv[1]+": pull a patched base image");process.exit(1)}' "$NODE_MIN"
# The server needs only node: drop the package managers (and their dependency trees, where
# scanners find advisories) from the runtime image.
RUN rm -rf /usr/local/lib/node_modules/npm /usr/local/lib/node_modules/corepack \
      /usr/local/bin/npm /usr/local/bin/npx /usr/local/bin/corepack /opt/yarn* /usr/local/bin/yarn /usr/local/bin/yarnpkg
WORKDIR /app
ENV NODE_ENV=production NEXT_TELEMETRY_DISABLED=1 PORT=3000 HOSTNAME=0.0.0.0
COPY --from=build --chown=node:node /repo/frontend/.next/standalone ./
COPY --from=build --chown=node:node /repo/frontend/.next/static ./.next/static
COPY --from=build --chown=node:node /repo/frontend/public ./public
USER node
EXPOSE 3000
CMD ["node", "server.js"]
