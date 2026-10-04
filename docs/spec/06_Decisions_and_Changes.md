# 06 — Decisions, errata and change log (v1.1)

Part of the [TRYST v1.1 baseline](00_README.md).

## 1. How v1.1 was reconciled

- **Precedence:** Source A (consolidated Spec + ToS, 4 Oct 2026) > Source B (Developer Specification, 17 Sep 2026) > Source C (PRD draft v0.1). See [00 §2](00_README.md#2-sources-and-precedence).
- **Where A is silent,** B's requirements are kept: the Discretion Policy object, reveal ladder, preference transparency, quick-exit timing, Privacy Auditor, no-go conditions, the weighted P1 ranker and mode names.
- **From C:** non-functional requirements, the couple dissolution and data-export APIs, and the competitor rows. C's US expansion, its pricing and its unit economics are dropped.
- **Where A contradicts itself or contains an arithmetic error,** v1.1 fixes it and records it in §3.

## 2. Decision log

**Status:** *Resolved* = decided by a source document. *Default* = decided in v1.1 to remove ambiguity, and reversible by the owner. *Open* = needs an owner decision by the gate shown; build against the stated default until then.

| ID | Decision | Status | Resolution / default | Owner · deadline |
|---|---|---|---|---|
| D-01 | Are unattached singles allowed? | **Open** (v1.2) | The founder scope statement ([01 §1](01_Product.md#1-executive-summary)) says TRYST is designed for adults already in a relationship, while Source A §3 includes solos open to couples (S4) and the curious (S6), who may be unattached. **Default:** the product and marketing target partnered adults and couples; unattached solos are *accepted* as S4 (needed for THIRD liquidity) and S6, but not targeted. Partner awareness is self-declared and never verified or displayed. | Founder · before P1 beta invites |
| D-02 | Chat encryption vs server-side scanning | **Resolved** (A §9.2, §21.6) | Full E2EE (MLS). Guardian runs on device and sends score + reason code only. Content reaches the server only through member-disclosed reports or Panic. Source B FR-009's "media scanning in real time" is met on device. | — |
| D-03 | Financial base case | **Resolved** (A §15.3) | A's illustrative model is the base case (CAC £38, 11% payer rate, 19% churn). B's targets (< 7% churn, > 3× LTV:CAC) are superseded. Thresholds corrected in E-03. | — |
| D-04 | What does the ENVOY tier's "runs outreach" mean? | **Default** | Envoy *proposes* intents with rationale; every outbound intent needs a one-tap human approval. Envoy never sends anything to a human as the member (P4; ToS cl. 7.3). | Head of Product · before P3 |
| D-05 | GHOST add-on charges for decoy skin, duress PIN and voucher billing, which conflicts with B's "never monetise safety" and C's "discretion is free" | **Open** | Default: core decoy, duress PIN, Burn, masking, zones and voucher billing are **free**. GHOST sells multiple decoy skins, custom TTLs and extended ExclusionRing heuristics at £7.99. Revenue impact is to be modelled. | CEO · before P3 pricing |
| D-06 | "Free ENVOY for 6 months for women-identifying users" vs gender-neutral pricing (B) and the Equality Act 2010 | **Open** (counsel) | Default: scarce-side seeding by **segment** (S4) and city-balance need, not gender. The gender-based variant ships only with a written counsel opinion. | DPO/Counsel · before P1 beta invites |
| D-07 | Contract formats | **Default** | OpenAPI 3.1 at the edge; Protobuf for gRPC, the agent bus and events (schema registry); generated types on both sides. | Principal Eng · P0 |
| D-08 | Biometric ban anchor | **Open** (counsel) | Default **off**. Anchors use document, device and payment HMACs only. V1 retains no biometric template (A §8.1). | DPO/Counsel · before P1 |
| D-09 | Microservices vs modular monolith for core domain services | **Default** | Agents are separate deployables (as A requires). Core domain services form a modular monolith (`core-svc`) with enforced module boundaries, given a 2-person backend team; split on evidence. Identity, ban, media and safety are always separate. | Principal Eng · P0 |
| D-10 | Hash matching of private (E2EE) media | **Open** (counsel + vendor) | Default: on-device pre-send nudity classifier always; on-device perceptual hash matching against licensed lists where licensing permits. Otherwise rely on reports, StopNCII and the NCII escalation path. Must be addressed in the OSA risk assessment (ToS Q7). | Head of T&S · P0 |
| D-11 | In-app purchase in store builds | **Default** | Web-first. Store builds use reader/link-out entitlements where rules allow; where IAP is required, offer TRYST+ only through IAP, with entitlements unified server-side. | Head of Product · P3 |
| D-12 | Delivery timeline | **Resolved** (A §17) | A's P0–P4 over 24 months, with phase gates, replaces B's 68-week sequential plan. | — |
| D-13 | Launch markets | **Resolved** (A §14.5) | UK + IE, then EU wave 2, then CA/AU/NZ. US out of scope; never Africa or MENA. | — |
| D-14 | Mode naming | **Default** | B's SPARK/EMBER/THIRD/QUAD/OPEN as UI labels; A's intent shapes as data values ([02 §2.3](02_Shared_Contracts.md#23-intent-shapes-and-modes)). | Design · P1 |
| D-15 | Concierge tier (B, C) | **Default** | Deferred beyond P4. A's tiers are canonical. | CEO · P4 review |
| D-16 | Who can sign in, and with what | **Resolved** (founder, v1.2) | Only the account holder. App and mobile PWA: two biometric factors (device-bound biometric passkey + liveness face match). Desktop: two factors (passkey + registered-phone approval or a second security key). No passwords, SMS or email codes, or staff overrides ([02 §3.3](02_Shared_Contracts.md#33-login-assurance--account-holder-only)). | — |
| D-17 | Members who cannot use face or fingerprint biometrics (disability, device) | **Open** (counsel) | Default alternative: two registered FIDO2 security keys with PIN, plus a provider video liveness check with assisted human review in place of B2. Required so biometric consent is freely given and to meet the Equality Act (ToS Q14, Q15). | DPO/Counsel · before P1 beta |
| D-18 | Where the B2 face reference is held | **Open** (counsel + vendor) | Default: held by the liveness provider under contract (TRYST stores only a token); deleted at the provider on erasure. Alternative rejected by default: TRYST-held encrypted template (contradicts A §8.1 "no biometric template"). | DPO/Counsel · P0 |
| D-19 | Brand identity | **Resolved** (founder, v1.2) | Founder-supplied logo used exactly as supplied; palette sampled from it (oxblood and gold on night, cream replacing "bone"). Discretion rules still keep the logo off decoy, lock-screen and notification surfaces ([01 §3](01_Product.md#3-name-and-brand)). | — |
| D-20 | Source B's Conversation Agent (suggested openers and replies) is not among Source A's seven agents | **Default** | Ship in P3 as an opt-in, on-device, draft-only assistant inside the member's own client. It never sends anything and never talks to another member as the member (FR-066; P4). | Head of Product · P3 |
| D-21 | SEO approach for "dynamic AI agent SEO, hyperlinks, backlinks and top ranking" (founder request, v1.2) | **Default** | Build Herald as a dynamic AI SEO agent with human editorial approval, isolated from member data. Links are earned through digital PR, partnerships and open-source work, never bought or spammed. Top-3 rankings are targets tracked weekly, not guarantees. Black-hat tactics are prohibited because a penalty would remove the business's main low-CAC channel (FR-073–FR-077). | Growth lead · P1 |
| D-22 | Payment processor | **Open — conditional** (founder chose Stripe, v1.2) | Stripe is primary, **conditional on Stripe's written approval** of TRYST's business model in P0, because Source A §14.4 states Stripe prohibits adult content and the web surface carries explicit media. A high-risk acquirer is integrated as the backup MID from day one and becomes primary if Stripe declines. Stripe's restricted-businesses policy could not be checked from the build environment. Fallbacks if Stripe approves dating but not explicit media: (a) keep explicit media off the Stripe-billed surface, or (b) bill everything through the high-risk acquirer. | CEO + Head of Payments · G-P0 |
| D-23 | Source B's final directive ("partnered adults, affairs and threesomes are explicit product categories — not hidden behind generic dating language") vs Source A §14.3 (store builds positioned as "discreet dating" to survive Apple 1.1.4) | **Default** | Explicit categories (SPARK/EMBER/THIRD/QUAD/OPEN, affairs, threesomes) on the **web PWA, marketing site and Herald SEO content**. Only **app-store listings and in-store-build copy** use neutral "discreet dating" wording, with no explicit imagery. Positioning never says "cheat better". | Head of Product · before store submission (P3) |
| D-24 | Identity service iteration 1 scope | **Default** | Implemented and tested: contact OTP sign-up (dev returns the code), passkey registration (user verification required, discoverable), mobile login B1 + B2 (pluggable liveness provider; fails closed; dev provider refused in prod), desktop login F1 + F2 (registered-phone approval or a second security key), session limits, lockout, Burn. **Next:** DPoP-bound tokens (bearer over TLS until then), PostgreSQL IDENTITY-DB store (in-memory now), native biometric-only attestation, real liveness provider, step-up and recovery endpoints. | Principal Eng · P1 |

## 3. Errata fixed in v1.1

| ID | Issue (source) | Fix in v1.1 | Where |
|---|---|---|---|
| E-01 | A §8.1 lets **V1** (liveness only) send intents and open threads, contradicting ToS cl. 5.1 ("adult verified before Intent"), B FR-001 and the OSA expectation that age assurance covers the inbox. | **V2 required** for discovery, intents, Envoy, threads, media and meets. V0/V1 = onboarding only. `min_counterparty_tier` default raised to V2. | 02 §3.2; FR-001 |
| E-02 | A §19.3 L2 computes ban anchors from the ID document number and payment token **at ban time**, but A §8.1/§10 retain neither, so anchors could never be written. | `anchor_candidate` HMACs are computed at V2 and held in IDENTITY-DB under the member's keys. They are erased with the account unless a permanent ban copies them to BAN-DB. Disclosed in ToS cl. 15.3. | 02 §4.1; 03 §8 |
| E-03 | A §15.3 says viability requires "blended CAC < £12". At the stated 11% payer rate and 52% V2 rate, break-even blended CAC per sign-up is **£8.18** (£143 × 0.11 × 0.52). £12 breaks even only at ≥ 16% payer rate. | Thresholds table added: £8.18 at 11%, £11.90 at 16%, £13.38 at 18%; 3× LTV:CAC needs £2.73–£4.46. | 01 §10.3 |
| E-07 | B specifies React Native for mobile; A requires native Swift/Kotlin for security. | Native (A). React Native not used. Shared Rust crypto core instead. | 04 §3 |
| E-09 | ToS Q8 cites "spec §19.5" for the consortium; it is §19.3 L5. | Corrected in ToS draft 0.9.1. | 05 cl. 29 |
| E-10 | ToS cl. 17.3 relies on "a false declaration under clause 3.2", but cl. 3.2 contains no declaration. A §19.3 L3 defines a conduct attestation at V2. | New ToS cl. 3.5 (conduct declaration); cl. 17.3 now refers to it. | 05 cl. 3.5, 17.3 |
| E-11 | A §9.2 media retention of "7 d default" was ambiguous between profile and thread media. | 7 d applies to **thread** media; profile media persists until removed. | 02 §7 |
| E-12 | A §8.1 says V1 retains "no biometric template", but A §19.3 L2 lists `anchor_biometric`. | Biometric anchor off by default (D-08). | 02 §4.2 |
| E-13 | Sources A and B both call themselves "v1.0 / 17 Sep 2026" (A's cover says 4 Oct 2026, its footer says 17 Sep). | Renamed Source A / Source B with file dates. | 00 §2 |
| E-14 | B's tiers (Black £24.99, Duo £34.99, Signals, Vault+, Concierge Black) conflict with A's. | A's tiers are canonical; Signals ≈ Keys; Vault+ folded into TRYST+ (extended media); Concierge deferred (D-15). | 01 §10.2 |
| E-16 | C planned a US launch, which A excludes. | US out of scope. C's US legal notes archived for reference only. | 01 §11 |
| E-21 | A §11 `POST /v1/panic` performs a discretion wipe, while A §19.7 defines Panic as emergency escalation and says it must be "distinct from Burn". | Wipe endpoint renamed `POST /v1/burn`; emergency is `POST /v1/safety/panic`. Distinct UI controls. | 02 §5.3, §5.8; 04 §5, §7 |
| E-23 | A §10 `couple_link` requires both co-sign timestamps to be NOT NULL but has no pending state for the invite flow. | `couple_invite` table added; `couple_link` written only on the second co-sign. | 02 §4.3 |
| E-26 | Slate size given as "12–18" (A §4) and "18" (A §11). | "Up to 18" cards including a 15% exploration floor. | 02 §5.5 |
| E-29 | A §11 lists `POST /v1/handshake` as client-callable, but handshakes must start only on mutual intent. | System-initiated only; clients read Briefs and post decisions. | 02 §5.6 |
| E-30 | A's §17 budget (£5.9M) excludes the £85–140k/yr insurance from A §20.5. | Called out as an addition in 01 §12. | 01 §12 |
| E-32 | Ashley Madison membership cited as 91M+ (B §1) and 80M+ (A §1). | Both kept as attributed, unaudited claims; external materials must use a current, dated figure. | 01 §2 |
| E-33 | Source B §11.1 targets verification + safety cost of £1.20–£2.20 per MAU alongside gross margin > 72%. At B's own conversion (7–10%) and ARPPU (£27–£34), revenue per MAU is only about £1.89–£3.40 before fees, so that cost alone consumes most of the revenue and makes 72% unreachable. | B's economics remain superseded by Source A's model (D-03); the inconsistency is recorded so B's table is not reused externally. | 01 §10.3; 07 |
| E-31 | B places THIRD/QUAD in delivery Phase 4; A treats S3/S5 as first-class and ships VC in P1. | THIRD/QUAD matching in P1 (A); DUO **billing** in P3. | FR-049 |

(Gaps in the E-numbering are intentional; IDs are never reused.)

## 4. Change log

| Version | Date | Change | Author |
|---|---|---|---|
| 1.0 (Source B) | 17 Sep 2026 | Developer Specification | Founder |
| 0.1 (Source C) | 4 Oct 2026 | PRD & business plan draft | Claude Code (draft) |
| 1.0 (Source A) | 4 Oct 2026 | Consolidated Spec + ToS draft 0.9 | Office of the Group CEO |
| 1.1 | 4 Oct 2026 | Merged A + B + C; split into Product / Shared Contracts / Backend / Frontend / ToS; decisions D-01–D-15; errata E-01–E-31; baseline and change control established | Claude Code, for founder review |
| **1.2** | **4 Oct 2026** | Founder logo added exactly as supplied (`assets/brand/`); brand palette and tokens from the logo; account-holder-only login: two biometric factors on app/mobile PWA, two factors on desktop (D-16–D-18, FR-056–FR-063, NFR-16/17, ToS 0.9.2 cl. 6.5, Q14–Q15); source register (00 §5); D-23 market-clarity vs store wording; Stripe as primary processor, conditional on approval (D-22, FR-078); intent change history (FR-079); E-33; Herald dynamic SEO agent and marketing site (FR-073–FR-077, NFR-20, D-21); full Source B added verbatim as appendix 07 with traceability; Source B §9–10 coverage (03 §6.8), Article 6 bases, FR-069–FR-072, NFR-18–19, ToS 0.9.3; FR-064–FR-068, D-20; founder product-scope statement, executive product decision, one-sentence pitch and market thesis added verbatim (01 §1–§2), D-01 reopened, E-32; spec folder renamed `docs/spec` | Claude Code, for founder review |
