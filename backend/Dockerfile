# syntax=docker/dockerfile:1.7
# Backend image: the member platform API (identity-svc, the default entrypoint) and the
# standalone safety service (run with --entrypoint /safety-svc). Static binaries on
# distroless, non-root.
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
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/identity-svc ./cmd/identity-svc && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/safety-svc ./cmd/safety-svc

FROM ${RUNTIME_IMAGE}
COPY --from=build /out/identity-svc /identity-svc
COPY --from=build /out/safety-svc /safety-svc
USER nonroot:nonroot
# identity-svc: 8081 API, 8091 internal (safety service only), 9090 metrics.
# safety-svc:   8082 API, 8092 internal (member platform only), 9092 metrics.
EXPOSE 8081 8082 8091 8092 9090 9092
ENV METRICS_ADDR=:9090
ENTRYPOINT ["/identity-svc"]
