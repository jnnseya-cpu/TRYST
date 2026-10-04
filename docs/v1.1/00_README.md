# TRYST Specification v1.1 — Baseline

**Private chemistry. Intelligent discretion.** · *Discretion is the product.*

| | |
|---|---|
| Version | **1.1 — Baseline (stable)** |
| Date | 4 October 2026 |
| Status | Stable for engineering planning and estimation. **Build start remains gated** on the written UK legal opinion and completed DPIA (Phase P0; see [01 §14](01_Product.md#14-compliance-envelope)). |
| Owner | Office of the Group Chief Executive → TRYST SPV (pre-incorporation) |
| Classification | Confidential |

## 1. Document set

v1.1 splits the specification by audience. Each document owns its topics; the others link to it rather than restating it.

| # | Document | Audience | Owns |
|---|---|---|---|
| 00 | **README** (this file) | Everyone | Index, source precedence, change control |
| 01 | [Product](01_Product.md) | Leadership, product, investors, counsel | Thesis, segments, modes, principles, loops, agent roles, business model, unit economics, GTM, roadmap, team, budget, KPIs, risks, compliance envelope, admission policy, liability architecture |
| 02 | [Shared Contracts](02_Shared_Contracts.md) | Backend **and** frontend | Glossary, enums, verification tiers and gates, data model, API contract, event taxonomy, agent message schemas, retention, error model, requirement catalogue (FR/NFR) with owners |
| 03 | [Backend](03_Backend.md) | Backend, ML, security, SRE, T&S engineering | Services, stack, agents, learning models, matching pipeline, cryptography, identity split, erasure, PSI server, Guardian server side, admission control, MLOps, commerce, backend backlog |
| 04 | [Frontend](04_Frontend.md) | iOS, Android, web, design | Two-surface strategy, client stacks, screens and flows, client discretion, client cryptography, on-device Guardian, PSI client, safety UX, copy rules, store compliance, frontend backlog |
| 05 | [Terms of Service — draft 0.9.1](05_Terms_of_Service_draft.md) | Counsel | Consumer terms for counsel review. **Not for publication.** |
| 06 | [Decisions, errata and change log](06_Decisions_and_Changes.md) | Product owner, tech leads | How v1.1 reconciled its sources, resolved and open decisions, errata fixed |

A combined PDF of 00–06 is at [`TRYST_Spec_v1.1.pdf`](TRYST_Spec_v1.1.pdf).

## 2. Sources and precedence

v1.1 merges three inputs. Where they conflict, the higher-ranked source wins unless [06](06_Decisions_and_Changes.md) records a decision otherwise.

| Rank | Source | File |
|---|---|---|
| **A** | Product & Technical Specification and Terms of Service — consolidated edition (4 Oct 2026; Part A §1–21, Part B ToS draft 0.9) | [`source/TRYST_Spec_and_ToS_Consolidated_2026-10-04.pdf`](../source/TRYST_Spec_and_ToS_Consolidated_2026-10-04.pdf) |
| **B** | AI Discreet Relationship Platform — Developer Specification (17 Sep 2026) | [`source/TRYST_Developer_Specification_2026-09-17.docx`](../source/TRYST_Developer_Specification_2026-09-17.docx) |
| **C** | PRD & Business Plan draft v0.1 (4 Oct 2026) | [`archive/TRYST_PRD_v0.1.md`](../archive/TRYST_PRD_v0.1.md) |

Both A and B call themselves "v1.0", so v1.1 refers to them as **Source A** and **Source B**. In v1.1, references such as `[A §9.3]` point to the source section a requirement came from.

## 3. Identifier conventions (stable across documents)

| Prefix | Meaning | Defined in |
|---|---|---|
| `S1`–`S6` | Member segments | 02 §2.2 |
| `V0`–`V3`, `VC` | Verification tiers | 02 §3 |
| `A1`–`A8` | Threat-model adversaries | 03 §6.1 |
| `M1`–`M9`, `M6a`–`M6e`, `M7b` | Models | 03 §4.3 |
| `L1`–`L5` | Admission-control layers | 03 §8 |
| `FR-nnn`, `NFR-nn` | Functional and non-functional requirements | 02 §10 |
| `D-nn` | Decisions | 06 §2 |
| `E-nn` | Errata fixed in v1.1 | 06 §3 |
| `G-xx` | Phase exit gates and no-go conditions | 01 §12–13 |

IDs are never reused or renumbered. A retired requirement keeps its ID with status `Retired`.

## 4. Change control (stabilisation rules)

1. **v1.1 is a baseline.** Changes go through a pull request that edits the owning document and adds a row to [06 §4](06_Decisions_and_Changes.md#4-change-log).
2. **Contract changes come first.** Any change to an API, entity, enum, event or schema is made in **02 Shared Contracts** before 03 or 04 depend on it.
3. **Open decisions** (`D-nn`, status *Open*) carry a stated default. Engineering may build against the default, and the owner must close the decision before the phase gate shown.
4. **Consistency check.** `python3 scripts/check_spec.py` checks that every `FR`/`NFR`/`D`/`E` ID referenced anywhere is defined exactly once, every FR has an owner, and relative links resolve. It must pass before merge.
5. **Version bumps:** editorial → 1.1.x; new or changed requirement → 1.2; change to a principle in 01 §4 → 2.0 (requires sign-off from the Group CEO).
