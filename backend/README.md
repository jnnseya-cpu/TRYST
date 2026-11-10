# backend

Go 1.26. One image (`Dockerfile`), two services:

| Entrypoint | Port | What |
|---|---|---|
| `/identity-svc` | 8081 (public), 8091 (internal), 9090 (metrics) | The member API: sign-in, verification, profiles, discovery, introductions, messages, meets, payments, data rights |
| `/safety-svc` | 8082 (public console API), 8092 (internal), 9092 (metrics) | Trust & Safety: reports, cases, moderator console, appeals |
| `/growth-svc` | 8083 (staff console API), 8084 (guide pages) | Growth intelligence (D-73): aggregate demand signals, Growth Briefs, editor-approved guide pages, measurement, rollback. Invisible to members; no member data |

Each service keeps its working state in memory and writes every change through to
PostgreSQL, so **each runs as exactly one instance**. A database lock enforces it: a second
copy waits until the first has stopped. Deploy by stopping the old one before starting the
new one.

```sh
go test ./...                                   # unit tests (PostgreSQL ones need TRYST_TEST_PG)
TRYST_ENV=dev go run ./cmd/identity-svc         # dev mode: in-memory, stand-in vendors
```

Settings: the header of `cmd/identity-svc/main.go`; production requirements:
[docs/runbooks/deploy.md](../docs/runbooks/deploy.md). Contracts: [../shared](../shared).
