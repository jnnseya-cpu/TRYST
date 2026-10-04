<p align="center"><img src="assets/brand/tryst-logo.png" alt="TRYST logo" width="240"></p>

# TRYST

**Private chemistry. Intelligent discretion.** — *Discretion is the product.*

TRYST is a discretion-first connection platform for attached adults, solo adults open to couples, and couples: one-off and ongoing connections, threesomes and couple-to-couple, for all genders and orientations.

## Current specification — v1.3 baseline

Start at [`docs/spec/00_README.md`](docs/spec/00_README.md).

| Document | For |
|---|---|
| [01 Product](docs/spec/01_Product.md) | Leadership, product, investors, counsel |
| [02 Shared Contracts](docs/spec/02_Shared_Contracts.md) | Backend **and** frontend — data model, API, enums, schemas, FR/NFR catalogue |
| [03 Backend](docs/spec/03_Backend.md) | Backend, ML, security, SRE, T&S engineering |
| [04 Frontend](docs/spec/04_Frontend.md) | iOS, Android, web, design |
| [05 Terms of Service — draft 0.9.3](docs/spec/05_Terms_of_Service_draft.md) | Counsel (not for publication) |
| [06 Decisions, errata, change log](docs/spec/06_Decisions_and_Changes.md) | Product owner, tech leads |
| [07 Source B — founder brief (verbatim)](docs/spec/07_Source_B_Founder_Brief.md) | Everyone — the founder's specification word for word, with traceability |
| [08 Source A — traceability](docs/spec/08_Source_A_Traceability.md) | Everyone — where each section of the consolidated Spec + ToS lives in the spec and code |
| [09 AI Operating System](docs/spec/09_AI_Operating_System.md) | Everyone — agent workforce and governance, command centres, self-managing layer, BitriPay door, connectors, ops/partner APIs, B2B licensing |
| [Combined PDF](docs/spec/TRYST_Spec_v1.3.pdf) | Reading and sharing |

Consistency check (runs in CI): `python3 scripts/check_spec.py`

## Code scaffold

> **Scaffold only.** Per the spec (01 §14), production engineering is gated on the written UK legal opinion and the completed DPIA (phase P0). Nothing here holds member data, calls a real provider or is deployed.

| Path | What | Checks |
|---|---|---|
| `contracts/openapi/` | Edge API contract (OpenAPI 3.1), generated from spec 02 §5 | Redocly lint |
| `contracts/proto/` | Agent bus and event schemas (Protobuf) | `protoc` compile |
| `backend/` | Go: jurisdiction and V2 tier gates, two-factor login state machine, couple co-sign (VC), Keys ledger and inbound cap, cryptographic erasure, ban anchors, enforcement ladder and appeals (ToS 17–18), Stripe webhook verification, BitriPay adapter (sandbox only), AI Governance gate (agent registry, always-human actions, kill switch, hash-chained audit); SQL migrations generated from spec 02 §4 | `go vet`, `go test -race`, migration drift check |
| `agents/` | Python: Broker Stage 0 hard gates (property-tested), slate assembly, Envoy schemas and Briefs, Mirror maths | `ruff`, `pytest` + Hypothesis |
| `crypto-core/` | Rust: per-media keys, per-recipient wrapping, revoke | `cargo fmt`, `clippy`, `test` |
| `web/` | Next.js PWA: brand tokens, logo (checksum-verified), two-factor sign-in steps, quick exit, `noindex` | `tsc`, `vitest`, `next build` |
| `mobile/` | Native iOS/Android requirements (placeholders; no toolchain in the build environment) | — |

Run everything locally with `make check`. CI runs the same checks (`.github/workflows/ci.yml`).

## Sources and history

- `docs/source/` — founder source documents (consolidated Spec + ToS, 4 Oct 2026; Developer Specification, 17 Sep 2026)
- `assets/brand/` — logo (exactly as supplied) and colour tokens
- `docs/archive/` — superseded drafts (PRD v0.1 and the v1.0-vs-v0.1 reconciliation)
