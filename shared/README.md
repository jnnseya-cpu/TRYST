# shared

The contract between the frontend and the backend. Nothing here is deployed; both sides
build and test against it, so a change here runs both pipelines.

| Path | What | Used by |
|---|---|---|
| `contracts/openapi/tryst.v1.yaml` | Member API (OpenAPI 3.1) | Backend route-security test; frontend `lib/*` clients |
| `contracts/openapi/ops.v1.yaml` | Staff console API | Safety service; console pages |
| `contracts/openapi/internal.v1.yaml` | Signed calls between the API and the safety service | Both Go services |
| `contracts/proto/` | Agent bus and event schemas | Agents |
| `contracts/vocab/` | Controlled vocabularies and test vectors (`guardian_vectors.json` must pass in Go **and** TypeScript) | Backend and frontend tests |
| `brand/` | Logo (exactly as supplied, checksum-verified) and colour tokens | Frontend build (`scripts/sync-brand.mjs`), docs |

Rules: change a contract here first, in the same pull request as both sides' changes;
`shared.yml` lints the contracts and checks the spec still agrees.
