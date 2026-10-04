<p align="center"><img src="../../assets/brand/tryst-logo.png" alt="TRYST logo" width="220"></p>

# TRYST Specification v1.3 — Baseline

**Private chemistry. Intelligent discretion.** · *Discretion is the product.*

| | |
|---|---|
| Version | **1.3 — Baseline (stable)**. v1.2 added the brand identity and account-holder-only login; v1.3 adds the internal operating system ([09](09_Operating_System.md); [06 §4](06_Decisions_and_Changes.md#4-change-log)) |
| Date | 4 October 2026 |
| Status | Stable for engineering planning and estimation. **Build start remains gated** on the written UK legal opinion and completed DPIA (Phase P0; see [01 §14](01_Product.md#14-compliance-envelope)). |
| Owner | Office of the Group Chief Executive → TRYST SPV (pre-incorporation) |
| Classification | Confidential |

## 1. Document set

The specification is split by audience. Each document owns its topics; the others link to it rather than restating it.

| # | Document | Audience | Owns |
|---|---|---|---|
| 00 | **README** (this file) | Everyone | Index, source precedence, change control |
| 01 | [Product](01_Product.md) | Leadership, product, investors, counsel | Thesis, segments, modes, principles, loops, agent roles, business model, unit economics, GTM, roadmap, team, budget, KPIs, risks, compliance envelope, admission policy, liability architecture |
| 02 | [Shared Contracts](02_Shared_Contracts.md) | Backend **and** frontend | Glossary, enums, verification tiers and gates, data model, API contract, event taxonomy, agent message schemas, retention, error model, requirement catalogue (FR/NFR) with owners |
| 03 | [Backend](03_Backend.md) | Backend, ML, security, SRE, T&S engineering | Services, stack, agents, learning models, matching pipeline, cryptography, identity split, erasure, PSI server, Guardian server side, admission control, MLOps, commerce, backend backlog |
| 04 | [Frontend](04_Frontend.md) | iOS, Android, web, design | Two-surface strategy, client stacks, screens and flows, client discretion, client cryptography, on-device Guardian, PSI client, safety UX, copy rules, store compliance, frontend backlog |
| 05 | [Terms of Service — draft 0.9.3](05_Terms_of_Service_draft.md) | Counsel | Consumer terms for counsel review. **Not for publication.** |
| 06 | [Decisions, errata and change log](06_Decisions_and_Changes.md) | Product owner, tech leads | How v1.1 reconciled its sources, resolved and open decisions, errata fixed |
| 07 | [Source B — founder brief (verbatim) and traceability](07_Source_B_Founder_Brief.md) | Everyone | The founder's 17 Sep 2026 specification word for word, plus where each part is implemented, the agent mapping and the FR-ID crosswalk |
| 08 | [Source A — traceability](08_Source_A_Traceability.md) | Everyone | Where each section of the consolidated Spec + ToS lives in the spec and the code |
| 09 | [Operating System (internal)](09_Operating_System.md) | Leadership, engineering, security, payments, operations | Agent workforce and governance (autonomy levels, always-human actions, kill switch, audit chain), command centres for every user type, self-managing platform layer, BitriPay API door, connector register, ops and partner schemas and APIs, B2B licensing and ACU, Admin Super Control Centre, roadmap |

A combined PDF of 00–09 is at [`TRYST_Spec_v1.3.pdf`](TRYST_Spec_v1.3.pdf). Brand assets (logo exactly as supplied, colour tokens) are in [`assets/brand/`](../../assets/brand/tokens.json).

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
| `L0`–`L3` (agents) | Agent autonomy levels | 09 §5.2 |
| `E-nn` | Errata fixed in v1.1 | 06 §3 |
| `G-xx` | Phase exit gates and no-go conditions | 01 §12–13 |

IDs are never reused or renumbered. A retired requirement keeps its ID with status `Retired`.

## 4. Change control (stabilisation rules)

1. **v1.1 is a baseline.** Changes go through a pull request that edits the owning document and adds a row to [06 §4](06_Decisions_and_Changes.md#4-change-log).
2. **Contract changes come first.** Any change to an API, entity, enum, event or schema is made in **02 Shared Contracts** before 03 or 04 depend on it.
3. **Open decisions** (`D-nn`, status *Open*) carry a stated default. Engineering may build against the default, and the owner must close the decision before the phase gate shown.
4. **Consistency check.** `python3 scripts/check_spec.py` checks that every `FR`/`NFR`/`D`/`E` ID referenced anywhere is defined exactly once, every FR has an owner, and relative links resolve. It must pass before merge.
5. **Version bumps:** editorial → 1.1.x; new or changed requirement → 1.2; change to a principle in 01 §4 → 2.0 (requires sign-off from the Group CEO).

## 5. Source register (revalidate before launch)

Policies, prices and legal requirements must be revalidated before launch, and again at each phase gate.

- **Source B's register** (accessed 17 Sep 2026) is reproduced verbatim in [07 §3](07_Source_B_Founder_Brief.md#3-source-b--full-text-verbatim): Ashley Madison, Gleeden, Victoria Milan, the FTC Ashley Madison settlement, ICO special-category guidance, UK GDPR Art. 9, Apple App Review Guidelines, Google Play inappropriate-content policy, NIST AI RMF and OWASP MAS.
- **Source A's citations** (Ofcom OSA and dating guidance, CRA 2015, CCR 2013, DMCC Act 2024, DPA 2018, card-scheme rules) are in the [consolidated PDF](../source/TRYST_Spec_and_ToS_Consolidated_2026-10-04.pdf).

**Added for v1.2 — not yet checked, verify in P0:**

| Topic | Source to check | Why |
|---|---|---|
| Stripe acceptance | https://stripe.com/legal/restricted-businesses and a written confirmation from Stripe | D-22: Stripe as primary processor (could not be fetched from the build environment) |
| Search-engine spam and AI-content policies | https://developers.google.com/search/docs/essentials/spam-policies | D-21: Herald link and content rules |
| Passkeys / WebAuthn | https://www.w3.org/TR/webauthn-3/ and https://fidoalliance.org/passkeys/ | D-16: login factors |
| Liveness PAD certification | ISO/IEC 30107-3; the provider's iBeta (or equivalent) Level 2 report | NFR-17 |
| Biometric data | ICO biometric data guidance | D-17, D-18, ToS Q14 |
| BitriPay API and webhook formats | The BitriPay API reference and OpenAPI document (supplied developer docs, 4 Oct 2026) | D-26: refund field names and the `BitriPay-Signature` layout are marked ASSUMED in the adapter |
| EU AI Act obligations for P4 markets | Regulation (EU) 2024/1689 and Commission guidance | 09 §13.4 |
