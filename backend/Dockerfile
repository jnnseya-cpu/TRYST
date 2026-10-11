# syntax=docker/dockerfile:1.7
# API image (identity-svc with every P1 module). Static binary on distroless, non-root.
#   docker build -t tryst-api backend
# Behind a TLS-intercepting proxy, pass its CA: --secret id=ca,src=/path/ca.crt
ARG GO_IMAGE=golang:1.26-alpine
ARG RUNTIME_IMAGE=gcr.io/distroless/static-debian12:nonroot

FROM ${GO_IMAGE} AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=secret,id=ca,required=false \
    if [ -f /run/secrets/ca ]; then export SSL_CERT_FILE=/run/secrets/ca; fi; go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/identity-svc ./cmd/identity-svc

FROM ${RUNTIME_IMAGE}
COPY --from=build /out/identity-svc /identity-svc
USER nonroot:nonroot
# 8081: API (behind the edge/web proxy). 9090: metrics, internal network only.
EXPOSE 8081 9090
ENV METRICS_ADDR=:9090
ENTRYPOINT ["/identity-svc"]
