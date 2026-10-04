# 08 — Source A: traceability (consolidated Spec + ToS, 4 Oct 2026)

Part of the [TRYST specification baseline](00_README.md).

Source A is the top-precedence source ([00 §2](00_README.md#2-sources-and-precedence)). Its full text is in [`source/TRYST_Spec_and_ToS_Consolidated_2026-10-04.pdf`](../source/TRYST_Spec_and_ToS_Consolidated_2026-10-04.pdf). This table shows where each section lives in the specification and in the code scaffold, and what v1.2 changed.

| Source A section | Specification | Code scaffold | v1.2 changes |
|---|---|---|---|
| 1. Thesis & market gap; operating stance | [01 §2](01_Product.md#2-thesis-and-market-gap), [01 §4.1](01_Product.md#41-stance) | — | — |
| 2. The Name; brand codes | [01 §3](01_Product.md#3-name-and-brand) | `assets/brand/`, `web/` | "Oxblood and bone" → logo palette, oxblood + gold, cream for bone ([D-19](06_Decisions_and_Changes.md#2-decision-log)) |
| 3. Segments & jobs (S1–S6) | [01 §5.1](01_Product.md#51-segments) | `agents/.../broker/entities.py` | Unattached solos: [D-01](06_Decisions_and_Changes.md#2-decision-log) |
| 4. Product loops | [01 §6](01_Product.md#6-product-loops) | — | — |
| 5. Agentic architecture (7 agents) | [01 §7](01_Product.md#7-agents-roles), [03 §3](03_Backend.md#3-agent-services) | `agents/.../envoy/` (schema-strict, no free text) | Herald SEO agent outside the member platform ([D-21](06_Decisions_and_Changes.md#2-decision-log)) |
| 6. Behaviour learning | [03 §4](03_Backend.md#4-behaviour-learning-mirror) | `agents/.../mirror/vectors.py` (α ∈ [0.15, 0.85], decay) | Decay FR-068 |
| 7. Matching & ranking; exposure fairness | [03 §5](03_Backend.md#5-matching-and-ranking-broker) | `agents/.../broker/gates.py` (property-tested), `slate.py`; `backend/internal/keys` (5/15 Keys, refund on reply, inbound cap) | — |
| 8. Trust, verification, consent | [02 §3](02_Shared_Contracts.md#3-verification-tiers-and-gates), [03 §7.3](03_Backend.md#73-escalation-matrix) | `backend/internal/gate` (V2 gate), `backend/internal/couple` (VC co-sign) | **V2 required for intents and threads**, not V1 ([E-01](06_Decisions_and_Changes.md#3-errata-fixed-in-v11)) |
| 9. Discretion architecture; threat model; crypto; ExclusionRing | [03 §6](03_Backend.md#6-security-cryptography-and-discretion-server-side), [04 §5](04_Frontend.md#5-client-discretion-controls) | `backend/internal/erasure` (destroy key → undecryptable), `crypto-core` (per-media key wrap and revoke) | Two-factor login ([D-16](06_Decisions_and_Changes.md#2-decision-log)); emergency lock FR-064 |
| 10. Data model | [02 §4](02_Shared_Contracts.md#4-data-model) | `backend/migrations/` (generated from 02 §4) | Anchor candidates, authenticators, couple invite, discretion policy, reveal grants, threads, safety tables |
| 11. API surface | [02 §5](02_Shared_Contracts.md#5-edge-api-contract-v1) | `contracts/openapi/tryst.v1.yaml` | `/v1/panic` → `/v1/burn` + `/v1/safety/panic` ([E-21](06_Decisions_and_Changes.md#3-errata-fixed-in-v11)); handshake system-only ([E-29](06_Decisions_and_Changes.md#3-errata-fixed-in-v11)); login, emergency-lock and Stripe webhook endpoints |
| 12. Stack & infrastructure | [03 §2](03_Backend.md#2-service-topology-and-stack), [04 §3](04_Frontend.md#3-client-stacks) | Go, Python, Rust, Next.js as specified; native apps documented in `mobile/` | Modular monolith for core services ([D-09](06_Decisions_and_Changes.md#2-decision-log)) |
| 13. MLOps | [03 §9](03_Backend.md#9-mlops-and-release-gates) | Property and invariant tests in CI | — |
| 14. Compliance | [01 §14](01_Product.md#14-compliance-envelope) | `backend/internal/gate` (jurisdiction gate) | Stripe as primary processor, conditional ([D-22](06_Decisions_and_Changes.md#2-decision-log)); Article 6 bases |
| 15. Business model | [01 §10](01_Product.md#10-business-model-and-unit-economics) | `backend/internal/keys`, `backend/internal/payments` | CAC threshold corrected ([E-03](06_Decisions_and_Changes.md#3-errata-fixed-in-v11)); GHOST scope ([D-05](06_Decisions_and_Changes.md#2-decision-log)) |
| 16. Liquidity & GTM | [01 §11](01_Product.md#11-liquidity-and-go-to-market) | `agents/.../broker/slate.py` (caps) | Gender-based seeding: [D-06](06_Decisions_and_Changes.md#2-decision-log); organic search (Herald) |
| 17. Roadmap, team, budget | [01 §12](01_Product.md#12-roadmap-team-budget-and-phase-gates) | — | Insurance and biometric-check costs added |
| 18. KPIs & risk register | [01 §13](01_Product.md#13-kpis-no-go-conditions-and-risk-register) | — | G-NG-8 to G-NG-10; new risks |
| 19. Admission control | [01 §8](01_Product.md#8-trust-verification-and-admission-policy), [03 §8](03_Backend.md#8-admission-control-l1l5) | `backend/internal/admission` (anchors, silent refusal, human-only bans, appeal lifts) | Anchor candidates at V2 ([E-02](06_Decisions_and_Changes.md#3-errata-fixed-in-v11)); biometric anchor off ([D-08](06_Decisions_and_Changes.md#2-decision-log)) |
| 20. Liability & disclaimer architecture | [01 §15](01_Product.md#15-liability-claims-and-insurance) | `web/` safety copy | — |
| 21. Guardian classifier spec | [03 §7](03_Backend.md#7-guardian--safety-models-m6am6e) | Reason codes in the OpenAPI contract; on-device only | — |
| Part B — Terms of Service draft 0.9 | [05](05_Terms_of_Service_draft.md) (draft 0.9.3) | — | cl. 3.5, 6.5, 15.2–15.3, 17.3, protected-trait rule; Q11–Q15 |
