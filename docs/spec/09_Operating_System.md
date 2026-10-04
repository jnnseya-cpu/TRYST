# 09 — TRYST Operating System (internal engineering) v1.3

Part of the [TRYST v1.3 baseline](00_README.md). Written in response to the founder's *Master AI Operating System Architect* brief (4 Oct 2026), applied to TRYST. Decisions [D-25](06_Decisions_and_Changes.md#2-decision-log)–[D-32](06_Decisions_and_Changes.md#2-decision-log); requirements FR-080–FR-088.

**How to read this document.** The brief asks for an autonomous, self-improving AI infrastructure OS with a full agent workforce, a BitriPay payment door, a connector ecosystem and a commercial engine. **This document keeps every item in the brief.** Each item is either built as asked, adapted so it fits TRYST's product principles ([01 §4](01_Product.md#4-operating-stance-and-principles)), or recorded with a stated reason it does not apply to a members-only discreet platform. The full register is in [§0](#0-brief-coverage-register). Nothing in the brief has been dropped silently.

**The one rule that shapes everything here:** *the AI runs the machine; people make the decisions that cannot be undone.* Agents observe, predict, recommend, draft and carry out reversible or pre-approved actions on their own. A permanent action against a member, money movement, a production code change and any contact made on a member's behalf always need a named human. This is enforced in code by the AI Governance gate (`backend/internal/aigov`, tested), not by policy text ([§5.4](#54-ai-governance-gate-implemented)).

---

## 0. Brief coverage register

Status: **Built** = in the codebase and tested. **Spec** = fully specified here and scheduled. **Adapted** = delivered in a changed form, with the reason. **Not for members** = applies to operators or partners only, never to member data. **Declined** = conflicts with a v2.0-locked principle; recorded with the reason and a stated alternative.

| Brief item | Status | Where / why |
|---|---|---|
| Forensic review: modules, workflows, journeys, API dependencies, data flows, monetisation, automation | Spec | §2.3 platform weakness review (technical, commercial, scalability, security, operational, AI gaps), each with a fix |
| "Do not remove any existing functionality" | Built | No v1.2 requirement removed; FR-001–FR-079 unchanged |
| Proven patterns only, from the named companies | Spec | Every pattern below cites the company or standard it comes from (§9.1) |
| Learn, adapt, predict, optimise, automate, secure, scale, self-heal | Spec | §5.6 self-managing layer; §9 architecture; Mirror learning loop (03 §4) |
| "…without requiring human intervention" | **Adapted** | Autonomy levels L0–L3 (§5.2). Reversible and runbook actions run without a person; four action classes always need one. UK GDPR Art 22 and principle P4 make full autonomy over members unlawful and unsafe ([D-25](06_Decisions_and_Changes.md#2-decision-log)) |
| AI Command Centre for every user | Spec | §4: one per user type, including a **private Member Command Centre** |
| Chief of Staff, Analyst, Research, Automation, Growth, Security, Knowledge agents in each centre | Adapted | §4.1 maps all seven for members (Growth = "my chances", not acquisition); full set for operators (§4.2–§4.9) |
| Executive agents (CEO, COO, CFO, CTO, CMO, CRO) | Adapted | Analyst copilots for the named executive; no authority of their own ([D-31](06_Decisions_and_Changes.md#2-decision-log)) |
| Product, Engineering, Quality, Cybersecurity, Revenue, Customer, Compliance agents | Spec | §5.3 workforce register; §5.5 core agents |
| Self-managing layer: System Health, Bug Detection, Auto-Repair, Infra Optimisation, Release Management, AI Governance | Built (governance) · Spec (rest) | §5.6. Auto-Repair = runbooks + rollback; code fixes go through a reviewed pull request ([D-28](06_Decisions_and_Changes.md#2-decision-log)) |
| Cybersecurity Command Centre (zero trust, identity, threat, fraud, anti-hacking, data protection) | Built (identity) · Spec | §13; login factors already built (02 §3.3) |
| Data intelligence layer (lake, warehouse, vector DB, knowledge graph, streaming, analytics, predictive, behavioural, recommendation, decision engines) | Spec | §9.6. All ten built on the existing stack; behavioural data follows Mirror's privacy rules |
| BitriPay Integration Gateway (merchant portal, keys, webhooks, sandbox, production, QR, wallet, card, bank, mobile money, SDKs, docs, testing, fees, revenue share, settlement) | Built (adapter, sandbox) · Spec | §7. Live use is gated by market ([D-26](06_Decisions_and_Changes.md#2-decision-log)) |
| Connector ecosystem (payments, banking, identity, comms, CRM, cloud, AI, productivity, storage) | Spec | §8.2 connector register, one row per provider, with data sent and received |
| WhatsApp, CRM for member data, data enrichment, maps tracking, logistics | Declined for member data | §8.2 states why for each ([D-29](06_Decisions_and_Changes.md#2-decision-log)); operator-only uses kept |
| Revenue engine (subscription, transaction, marketplace, AI usage, API, enterprise, licensing, white label) | Spec / Adapted | §12. Member revenue is the v1.2 tier set; B2B lines are licensing of the discretion stack, never member data |
| Dynamic pricing engine | Adapted | Experiments on list price per market, disclosed and identical for every member in a market; no personalised pricing ([D-30](06_Decisions_and_Changes.md#2-decision-log)) |
| AI revenue optimisation, CLV, churn prevention, upsell, cross-sell engines | Adapted | §12.5. Optimise for QCAM-weighted retention; no dark patterns; one-step cancel stays (ToS cl. 16) |
| AI credit / ACU system | Adapted | ACU meters B2B and partner API usage only. Members never buy ACU; Keys already serve that role ([D-27](06_Decisions_and_Changes.md#2-decision-log)) |
| Data intelligence revenue | Declined for member data | P8 forbids data sale. Alternative: licensing of models and privacy technology, and free published aggregate research (§12.4) |
| PRD, TRD, system, infra, AI, agent, security, DB, API, event, DevOps, monitoring, DR, BC, data governance, compliance, scalability, commercial architecture | Spec | §9–§13; index in §17 |
| Complete ERD, API specs, user journeys, agent workflows, DB schemas, security controls, deployment, testing, production-readiness review | Spec | §10 (ERD + DDL), §11 (API + webhooks), §3.2 journeys, §5.5 workflows, §13 controls, §9.9 deployment, §9.10 testing, §15.6 PRR |
| 17 required sections | Spec | §1–§17 below |

---

## 1. Executive product vision

### 1.1 What the OS is

The TRYST operating system is the operating layer that runs TRYST: **a governed workforce of specialised AI agents, the data and event fabric they work on, and the command centres through which members, operators, partners and regulators see and steer them.** It turns TRYST from an app with some models into infrastructure that runs its own operations. It matches, protects, bills, supports, secures, monitors and repairs itself, and puts people in charge of every decision that cannot be undone.

It is built for one product first. Discreet, verified, consent-first connection for partnered adults and couples is the hardest trust problem in consumer software. A system that can run that safely can be licensed to others ([§12.3](#123-b2b-licensing-white-label-and-api-revenue)).

### 1.2 What problem it solves

| For | Problem today | What the OS does |
|---|---|---|
| Members (S1–S6) | Volume without outcome; exposure risk; harassment; fakes; endless chat that never becomes a meeting | Agents pre-qualify, negotiate and protect; the member decides. Measured by QCAM, not time in app |
| Couples (S3, S5) | Coordinating two people's consent with a third or another couple collapses | Envoy negotiates over a fixed schema; joint-veto modes; dual-consent controls |
| TRYST operations | A 17-person team cannot run 24/7 trust and safety, payments, SRE and growth by hand at 150k verified members | Agents carry the volume; people handle the exceptions (§5.6, §14) |
| Regulators and auditors | Opaque recommendation systems; unverifiable safety claims | Every agent decision is logged in a hash-chained audit trail; transparency reporting is generated from it (§4.10) |
| Partners and licensees | Discretion, verification and consent tooling is expensive to build | The same governed stack, licensed with no member data attached (§12.3) |

### 1.3 Why it is different

1. **Resolution over engagement (P1).** Every agent's reward is tied to QCAM, the confirmed and positively rated meets per active member per month. Competitors optimise for time in app.
2. **Agents negotiate; humans decide (P4).** The double opt-in agent handshake (Loop C) is the product. No competitor runs agent-to-agent pre-negotiation for one-to-one, threesome or couple-to-couple matching.
3. **Governance is code.** The always-human classes, the kill switch, autonomy budgets and the tamper-evident audit log are in `aigov` and covered by tests.
4. **Breach-assumed privacy (P3).** E2EE threads, the identity/profile split, cryptographic erasure in under 60 s and self-hosted models for anything touching special-category data.
5. **Payment-processor independence.** One `PaymentProvider` interface across Stripe, the high-risk acquirer and BitriPay. Entitlements never depend on one processor.

### 1.4 Why the market needs it and why it can win

Category leaders built on volume and on paid messaging, and they are held back by fake-profile and breach history (01 §2). The demand is proven (attributed, unaudited: Ashley Madison 80M+/91M+, E-32). The unmet need is **trust that is engineered, not claimed**. TRYST can win because the OS makes the trustworthy behaviour the cheapest to run:

- automation carries trust and safety and support volume, so gross margin survives verification costs;
- Envoy raises meets per intent (target +40% at G-P2), and that outcome is what members pay for;
- the governed agent stack becomes a licensable B2B asset with no member data in it.

---

## 2. Market gap deep review

### 2.1 Competitors

| Platform type | Does well | Fails to solve | Where users are underserved | Where businesses lose money | Missing automation | How the OS fills it |
|---|---|---|---|---|---|---|
| Affair sites (Ashley Madison, Gleeden, Victoria Milan) | Category awareness; explicit positioning | Fake and inactive profiles; breach legacy; pay-per-message mechanics | Women and couples face harassment volume; no real couple model | Chargebacks, refunds and trust loss from fake profiles; ads banned | No pre-qualification; manual moderation | V2 gate (E-01), Envoy handshake, Guardian on device, Keys refunded on reply, no fakes (P8) |
| Mainstream dating (Tinder, Hinge, Bumble) | Scale; polished UX; ranking ML | Will not serve the partnered; engagement-maximising loops | Couples, thirds and the discreet are pushed off-platform | Paywall churn; moderation cost per MAU | Agents limited to profile tips | Purpose-built segments S1–S6 and modes; agents tied to QCAM |
| Kink and couples platforms (Feeld and similar) | Inclusive structures; couple profiles | Weak verification; little discretion engineering | S4 harassment; discretion relies on member habits | Trust and safety cost; store risk | No negotiation layer; no exposure-risk model | ExclusionRing PSI, Curtain, S4 inbound caps, VC co-sign |
| Swinger directories and forums | Community; events | Unverified; open directories; scraping | Discretion; safety | Little monetisation beyond membership | None | Verified, private, agent-mediated |
| General AI agent platforms (OpenAI Agents, Microsoft Copilot Studio, Salesforce Agentforce, ServiceNow) | Orchestration tooling; enterprise governance | Not built for special-category data or member-to-member consent | Not applicable to members | High per-seat cost for SMEs | Domain policy is left to the customer | TRYST packages domain policy (consent, discretion, verification) with orchestration |

### 2.2 The four structural gaps (from 01 §2.2, extended)

| Gap | Evidence | OS answer | Measured by |
|---|---|---|---|
| Outcome gap: volume without meets | Reply rates, ghosting, message volume as a KPI | Broker + Envoy + Aftercare reward loop | QCAM; meets per intent |
| Trust gap: fakes, bots, sextortion | Category history; OSA duties | V2 gate, admission L1–L5, Guardian M6a–e, Fraud agent | Report rate per 1k threads; fake-profile prevalence |
| Discretion gap: exposure by collision, notification, device | Members' primary anxiety (S1) | Curtain, ExclusionRing, decoy/duress/Burn, emergency lock | Exposure incidents; leak tests |
| Operations gap: a small team cannot run 24/7 trust, payments and SRE | Team of 17 to P3 (01 §12) | §5.6 self-managing layer, §5.5 core agents, §14 control centre | Tickets per 1k MAU handled by agents; MTTR; on-call pages per week |

### 2.3 Platform weakness review (v1.2 baseline → fix)

The brief asks for a forensic review of TRYST itself. Findings against the v1.2 codebase and spec:

| Area | Weakness found | Fix (this document) | Phase |
|---|---|---|---|
| Technical | Identity store is in memory; DPoP not yet bound (D-24) | PostgreSQL IDENTITY-DB and DPoP are P1 items; OS adds no dependency on them before P1 | P1 |
| Technical | No central registry of which agent may do what | `aigov` registry and gate (built, FR-080) | P1 |
| Technical | Agent actions are not uniformly audited | Hash-chained audit log for every agent decision (built, FR-082) | P1 |
| Commercial | One processor family; Stripe approval uncertain (D-22) | `PaymentProvider` with three adapters: Stripe, high-risk acquirer, BitriPay (built, sandbox) | P1 |
| Commercial | Revenue limited to B2C | B2B licensing of the discretion and verification stack, metered in ACU (§12.3) | P4+ |
| Scalability | Modular monolith (D-09) with no stated split triggers | Split triggers in §9.8 | P2+ |
| Security | No automated containment for agent misbehaviour | Kill switch per agent and global (built) | P1 |
| Security | Connectors could leak member data to third parties | Connector register with a data map and a no-member-data rule per provider (FR-087) | P1 |
| Operational | On-call relies on people reading dashboards | System Health → Auto-Repair runbooks with SLO-burn rollback (FR-085) | P1 |
| Operational | No single admin view across safety, payments, agents and health | Admin Super Control Centre (§14, FR-086) | P1 |
| AI capability | No member-facing explanation surface for agent decisions | Member Command Centre "why" panels from reason codes (FR-083) | P2 |
| AI capability | Agent evals exist per agent but not for cross-agent workflows | Workflow-level evals and replay in the release gate (§9.10) | P2 |

---

## 3. Complete user ecosystem

### 3.1 User types

Every user type in the brief is listed. A type that does not exist in TRYST is marked as such and kept for the licensing model.

| # | User type (brief) | In TRYST | Examples | Authenticates with | Sees member data? |
|---|---|---|---|---|---|
| U1 | Customers | **Members** (S1–S6, solo and couple) | Attached solo; couple seeking a third | Two biometric factors (mobile) / two factors (desktop) (02 §3.3) | Their own, and what counterparties reveal |
| U2 | Businesses | **Licensees** (P4+) | A dating platform licensing the verification and discretion stack | OIDC SSO + hardware key; API keys | Never TRYST's members |
| U3 | Admins | **Platform admins** | Head of T&S, Principal Engineer, DPO | Hardware security key + device posture + just-in-time role | Redacted by default; break-glass only |
| U4 | Operators | **T&S moderators, support agents, SRE on-call, payments ops** | Moderator on the trust console | As admins, with role scope | Case-scoped and redacted |
| U5 | Partners | **Assurance, payment and content partners** | Age-assurance providers; voucher resellers; digital-PR partners | Mutual TLS + signed webhooks | Only what the contract and data map allow |
| U6 | Agents | **AI agents** (§5) | Envoy, Guardian, Auto-Repair | SPIFFE workload identity + manifest | Only their manifest's scopes |
| U7 | Developers | **TRYST engineers; licensee developers** | Backend engineer; licensee integrator | SSO + hardware key; sandbox keys | Never in production without break-glass |
| U8 | Merchants | **Not in TRYST's member product.** Kept for BitriPay connected accounts in the licensing model (§7.4) | Voucher reseller receiving a commission | BitriPay account + TRYST partner portal | None |
| U9 | Service providers | **Processors and sub-processors** | Cloud, liveness, CDN, email | Contract + DPA | Per the data map (§8.2) |
| U10 | Regulators | **Ofcom, ICO, DPC (IE), card schemes, auditors** | Ofcom information request | Read-only regulator portal; time-boxed | Aggregates and specific lawful disclosures only |
| U11 | Third-party API partners | **Connector providers** | Stripe, BitriPay, Persona | Their own auth; our keys in the vault | Per the data map |

### 3.2 User journeys

The member journeys are Loops A–E (01 §6) and the screens in 04 §4. The operating system adds these journeys:

| Journey | Steps | Agents | Human checkpoint |
|---|---|---|---|
| J1 Member sees why | Opens Command Centre → "Why this slate?" → reason codes rendered as plain sentences → corrects a preference | Chief of Staff (member), Mirror, Broker | Member edits; P5 floor holds |
| J2 Member safety brief | Before a meet: Safe Meet checklist → venue sanity (public, open) → check-in timer → aftercare at 48 h | Guardian, Aftercare | Member sets the check-in; Panic is always available |
| J3 Operator case | Report → Guardian triage score → case queue with redacted evidence → moderator decision → appeal path | Guardian, Compliance | Moderator decides any removal (always-human) |
| J4 Incident self-heal | SLO burn alert → System Health opens incident → Auto-Repair runs rollback runbook → on-call is told, not woken, if the rollback resolves it | System Health, Auto-Repair, Release Manager | Human reviews the post-incident report; any code fix is a reviewed PR |
| J5 Payment exception | Webhook says dispute → Payment agent assembles evidence → Fraud agent scores → payments ops approves refund or contest | Payment, Fraud | Refund is money movement (always-human), unless covered by the pre-approved auto-refund rule (§5.5, Payment) |
| J6 Licensee onboarding | Partner applies → KYB → sandbox keys → integration tests → contract → live keys | Onboarding (B2B), Compliance, API Integration | Live keys need a named approver |
| J7 Regulator request | Request logged → Compliance agent assembles the lawful minimum from audit and transparency data → DPO approves → disclosure | Compliance | DPO approves every disclosure |
| J8 Content publish | Herald drafts → editor approves → publish → rank tracking | Herald (Marketing) | Editor approves every publish |

---

## 4. AI-agent command centres

Each command centre is a role-scoped surface over the same agent fabric. **The data a centre sees is decided by the caller's role and the agents' manifests, never by the UI.** All actions go through the governance gate.

### 4.1 Member Command Centre (U1)

**Design constraints that override the brief's defaults:** private by default; nothing is shown that would help someone holding the member's phone (P3); no engagement counters, streaks or "you're missing out" nudges (P1); the Leave control and decoy behaviour work on every screen; agents never act as the member (P4). Built on the existing `/v1/me` and `/v1/me/security` routes; web route `/account/insights` (FR-083).

| Brief agent | Member version | What it sees | Actions it can perform | What it may decide | Automations the member controls |
|---|---|---|---|---|---|
| Personal AI Chief of Staff | **Concierge brief**: one calm daily summary: new Briefs waiting, a meet to confirm, renewal date | Own Briefs, intents, meets, entitlement | Draft a reply (on device, D-20), set a check-in timer, remind | Nothing; it recommends | Summary time; quiet hours; off switch |
| AI Analyst | **My patterns**: charts of own activity (intents sent, mutual rate, meets, aftercare trend), security activity | Own aggregates only | Explain a chart | — | Personalisation on/off (FR-024) |
| AI Research Agent | **Explainers**: verification, discretion and safety guides; local laws for travel mode | Public content | — | — | — |
| AI Automation Agent | **ENVOY tier proposals**: Envoy pre-qualifies and drafts outreach; each needs one tap | Envoy proposals with rationale codes | Approve / discard | Never sends (D-04) | Cadence; filters |
| AI Growth Agent | **My chances**: which settings narrow the slate (radius, tier floor, modes) and the expected effect | Own settings; aggregate liquidity bands | Suggest a setting change | — | Accept or ignore |
| AI Security Agent | **Security activity**: sign-ins, factors, new devices, locks (built: `/v1/me/security`); session risk from Curtain | Own auth events (90 d) | Sign out everywhere; emergency lock; revoke device | Curtain may only tighten (02 §6.4) | Lock behaviour; decoy |
| AI Knowledge Agent | **My memory**: what TRYST has learned about me (stated vs revealed, α weight) with reset | Own IntentVector, RevealedPreferenceVector summary, version history (FR-079) | Correct, reset, export (DSR) | — | Reset learning |

### 4.2 Couple Command Centre (U1, VC)

Shared panel, visible to both partners, plus each partner's private panel. It shows the joint-veto state, pending thirds or couples, dual-consent requests and Envoy screening for a third. **Neither partner sees the other's private panel, private fields or security activity.** Actions are a co-sign (both partners, two devices; `same_device_cosign` is refused) and dissolve (either partner, immediate, 02 §9).

### 4.3 Operator Command Centre: Trust and Safety (U4)

| Element | Specification |
|---|---|
| Sees | Case queue ordered by Guardian severity and SLA; redacted evidence; member-disclosed content only; prior enforcement; appeal history |
| Agents | Guardian (triage, holds), Compliance (SLA and statement of reasons), Support (drafts), Risk (ban-anchor matches) |
| Actions | Hold (reversible, agent may do); warn, restrict, remove, ban, report to authorities (**always-human**); request more evidence |
| Automations | SLA routing; duplicate merge; known-hash auto-hold pending review |
| Decides or recommends | Guardian recommends with a confidence and reason code; the moderator decides; a second moderator reviews permanent bans (03 §7.3) |

### 4.4 Operator Command Centre: Support (U4)

Sees ticket, entitlement and Keys-ledger state for the ticket's member only (case-scoped). The Support agent drafts and the person sends. Goodwill Keys and refunds are money movement (always-human), except for refunds covered by a published automatic rule (Keys refunded on reply; 14-day statutory refunds, ToS 16.7), which run as ledger operations.

### 4.5 Operator Command Centre: SRE and engineering (U4, U7)

Sees SLOs, error budgets, traces, deploys, cost and the agent decision log. Agents: System Health, Bug Detection, Auto-Repair, Release Manager, Infra Optimiser, Vulnerability. Automations: canary analysis, automatic rollback on SLO burn, scale-out, dependency PRs. A **production change** (promote, patch, rightsizing apply) is always-human.

### 4.6 Operator Command Centre: Payments and finance (U4)

Sees processor health per adapter, authorisation rates, chargeback ratio (alert 0.5%, hard stop 0.65%), disputes, settlement and VAT. Agents: Payment, Fraud, Revenue Analyst, Pricing. Actions: switch the primary processor (always-human), approve refunds and dispute responses, publish a price experiment (always-human, §12.2).

### 4.7 Executive Command Centre (U3)

The brief's CEO, COO, CFO, CTO, CMO and CRO agents are **analyst copilots** for the named executive ([D-31](06_Decisions_and_Changes.md#2-decision-log)). They read warehouse aggregates (never member-level data) and produce: the weekly QCAM and KPI pack (01 §13.1), phase-gate readiness (G-P0 to G-P4), no-go monitor (01 §13.2), unit economics against E-03 thresholds, budget burn and risk register deltas. They hold no actions beyond `recommend`.

### 4.8 Partner and Licensee Command Centre (U2, U5, U8)

API keys (sandbox and live), webhooks with delivery log and replay, usage in ACU, invoices, SLA status, integration test results, data-processing terms. Connected BitriPay accounts (for commission-earning partners such as voucher resellers) show settlement statements from BitriPay directly. **No member data is ever visible here.**

### 4.9 Developer Command Centre (U7)

OpenAPI reference (`contracts/openapi/tryst.v1.yaml`), sandbox keys, webhook tester, the BitriPay sandbox magic numbers (§7.6), SDK downloads, changelog, status. Same surface for internal engineers and licensee developers, with scopes.

### 4.10 Regulator Portal (U10)

Read-only, time-boxed accounts. Shows the transparency report (OSA), the aggregate enforcement statistics, the agent register (which agents exist, what they may do, autonomy level and owner) and the audit-chain verification result. Specific disclosures are made only through J7 with DPO approval.

### 4.11 Agent Command Centre (U6)

Agents also have a "centre": the registry entry (manifest), budget use today, kill state, recent decisions and eval scores. It is rendered inside the Admin Super Control Centre (§14).

---

## 5. Core AI agents

### 5.1 Agent model

Every agent is a separately deployable service (03 §2.2) declared by a **manifest** in the agent registry. The manifest is the contract; anything not in it is denied.

| Manifest field | Meaning | Enforced by |
|---|---|---|
| `id`, `owner` | Agent name and accountable human team | Registry refuses a manifest without an owner |
| `level` | Autonomy level L0–L3 (§5.2) | Gate |
| `actions` | Action name → effect class (read, recommend, reversible, runbook, irreversible, member_safety, money_movement, production_change) | Gate; unlisted action → deny |
| `data_scopes` | Stores and datasets it may read | Registry: a member-data scope (IDENTITY-, PROFILE-, SAFETY-, BAN-DB, event bus, feature store, vector store) is refused unless the agent runs inside the member platform **and** on a self-hosted model |
| `daily_budget` | Maximum autonomous reversible or runbook actions per UTC day | Gate; above budget → needs a human |
| `model_version`, `policy_file`, `eval_suite` | What it runs and how it is tested | Release gate (03 §9) |
| `act_as_member` | Not a valid class | Registry rejects any manifest that contains it (P4) |

Agents talk over typed Protobuf messages on the bus (`agent.*` topics), never free text (A §5), and call each other with gRPC + mTLS under SPIFFE identities.

### 5.2 Autonomy levels

This is how the brief's "without requiring human intervention" is delivered safely. Pattern: the levels of driving automation (SAE J3016, as used by Tesla and others), Google SRE's automation ladder, and Microsoft and Anthropic's published human-in-the-loop guidance for agents.

| Level | Name | Runs without a person | Typical agents |
|---|---|---|---|
| L0 | Observe | Read, report | Monitors in shadow mode |
| L1 | Recommend | Read, recommend, draft | Cartographer, Envoy, Support, Chief of Staff, executive copilots, Infra Optimiser |
| L2 | Reversible | + reversible changes inside a daily budget | Mirror, Broker, Curtain, Guardian (holds), Aftercare, Fraud (holds), System Health, Herald (link repair) |
| L3 | Runbook | + pre-approved runbooks (rollback, restart, scale out, halt canary) | Auto-Repair, Release Manager |

**Always-human classes** (no level unlocks them): `irreversible` (delete, publish, send to an outside party), `member_safety` (freeze, restrict, remove, ban), `money_movement` (refund, payout, price change, credit), `production_change` (code, model or prompt change reaching production). **Never-allowed class:** `act_as_member`.

### 5.3 Workforce register

All agents named in the brief, mapped to TRYST. "Plane" = **M** member platform (self-hosted models, member data allowed by manifest) or **O** operations plane (no member data). The first 23 rows are loaded from `aigov.Workforce()` and pinned by tests; the rest are specified for the phases shown.

| Brief category | Brief agent | TRYST agent | Plane | Level | Phase | Notes |
|---|---|---|---|---|---|---|
| Matching (01 §7) | — | Cartographer | M | L1 | P1 | Member confirms every field |
| Matching | — | Mirror | M | L2 | P2 | Sole writer of RevealedPreferenceVector |
| Matching | — | Broker | M | L2 | P1 | Stage 0 gates cannot be bypassed |
| Matching | — | Envoy | M | L1 | P2 | Never sends; advisory Briefs |
| Matching | — | Curtain | M | L2 | P1 | Tighten only |
| Matching | — | Guardian | M | L2 | P1 | Holds only; removals are human |
| Matching | — | Aftercare | M | L2 | P1 | Private outcome only |
| Core (brief §5) | Onboarding Agent | Onboarding | M | L2 | P1 | §5.5 |
| Core | Risk Agent | Risk | M | L2 | P1 | §5.5 |
| Core | Fraud Detection Agent | Fraud | M | L2 | P1 | §5.5 |
| Core / Customer | Customer Support Agent | Support | M | L1 | P1 | §5.5 |
| Command centre | Personal AI Chief of Staff | Chief of Staff | M | L1 | P2 | §4.1; on device where possible |
| Core / Compliance | Compliance Agent, GDPR Agent | Compliance | M | L2 | P1 | §5.5 |
| Core | Payment Agent | Payment | M | L2 | P1 | §5.5 |
| Core / Marketing | Marketing Agent (Herald) | Herald | O | L2 | P1 | Isolated (FR-075) |
| Revenue / Core | Revenue Agent, CFO Agent | Revenue Analyst | O | L1 | P1 | Aggregates only |
| Self-managing | System Health Agent | System Health | O | L2 | P1 | §5.6 |
| Self-managing / Quality | Bug Detection Agent | Bug Detection | O | L2 | P1 | §5.6 |
| Self-managing | Auto-Repair Agent | Auto-Repair | O | L3 | P1 | §5.6 |
| Self-managing | Infrastructure Optimisation Agent | Infra Optimiser | O | L1 | P2 | §5.6 |
| Self-managing | Release Management Agent | Release Manager | O | L3 | P1 | §5.6 |
| Cybersecurity | Threat Hunter Agent | Threat Hunter | O | L2 | P1 | §13 |
| Cybersecurity | Vulnerability Agent | Vulnerability | O | L2 | P1 | §13 |
| Self-managing | AI Governance Agent | **The gate itself** + Governance reviewer | O | — | P1 | Deterministic code, not a model (§5.4) |
| Cybersecurity | SOC Agent | SOC triage | O | L2 | P1 | Alert dedupe and enrichment; containment runbooks |
| Cybersecurity | Identity Agent | Identity risk | M | L2 | P1 | Login risk, step-up triggers; never bypasses two factors |
| Cybersecurity | Fraud Agent | = Fraud | M | L2 | P1 | — |
| Core | Pricing Agent / Revenue: Pricing | Pricing | O | L1 | P3 | Market-level experiments only (§12.2) |
| Revenue | Monetisation Agent | Monetisation | O | L1 | P3 | Proposes packaging; never dark patterns |
| Revenue | Sales Agent | B2B Sales | O | L1 | P4 | Licensee pipeline; drafts only |
| Customer | Success Agent | Licensee Success | O | L1 | P4 | B2B only |
| Customer | Retention Agent | Retention | M | L1 | P3 | §12.5; no retention flow on cancel |
| Compliance | AML Agent | AML (payments) | O | L1 | P3 | Monitors processor alerts; AML duties sit with the regulated PSP ([D-32](06_Decisions_and_Changes.md#2-decision-log)) |
| Compliance | KYC Agent | = Onboarding (age assurance) + KYB for licensees | M / O | L2 | P1 / P4 | — |
| Compliance | Regulatory Agent | Regulatory | O | L1 | P1 | OSA codes, ICO guidance, store policies; change alerts |
| Core | Data Intelligence Agent | Data Intelligence | O | L1 | P2 | Warehouse aggregates; DP noise on member-derived metrics |
| Core | Operations Agent / COO Agent | Operations | O | L1 | P1 | Queue health, SLA, staffing forecast |
| Core | API Integration Agent | API Integration | O | L2 | P1 | Connector health, key rotation reminders, contract tests |
| Core | Workflow Automation Agent | Orchestrator | O / M | L2 | P1 | Runs declared workflows (§5.7) |
| Core | Predictive Growth Agent | Predictive Growth | O | L1 | P2 | Liquidity forecasts per city and segment |
| Core | Admin Control Agent | Admin Control | O | L1 | P1 | §14; summarises and proposes, never grants access |
| Executive | CEO, COO, CFO, CTO, CMO, CRO Agents | Executive copilots | O | L1 | P1 | §4.7; D-31 |
| Product | Product Architect, UX, Journey, Feature Agents | Product copilots | O | L1 | P2 | Funnel and journey analysis on aggregates; design-review checklists (04 §7) |
| Engineering | Frontend, Backend, Infrastructure, API, Database Agents | Engineering copilots | O | L1 | P1 | Coding assistants that open PRs; every PR is human-reviewed and passes CI |
| Quality | QA, Testing, Performance Agents | Quality copilots | O | L2 | P1 | Generate tests and load profiles; run them in CI; cannot merge |

### 5.4 AI governance gate (implemented)

Code: `backend/internal/aigov` (Go, standard library only). Tests: `aigov_test.go`, 9 tests, including a mutation check that removing `member_safety` from the always-human set fails the suite.

| Behaviour | Implementation | Test |
|---|---|---|
| Unknown agent, unknown action → deny | `decideLocked` | `TestLevelCapsAutonomy` |
| Always-human classes → `needs_human` unless a **registered named human** approved | `alwaysHuman` set; approver allow-list | `TestAlwaysHumanClassesNeedAPerson` |
| An agent cannot approve its own action | Approver must be in the human allow-list | `TestUnknownApproverIsDenied` |
| Autonomy level caps classes | `minLevel` per class | `TestLevelCapsAutonomy` |
| Daily budget, then a person | Per-agent per-UTC-day counter | `TestBudgetThenHuman` |
| Kill switch per agent and global; global overrides approvals | `Kill`, `KillAll`, `Resume` (each recorded with who did it) | `TestKillSwitch` |
| Member-data scopes only for self-hosted member-plane agents; `act_as_member` rejected | `Register` | `TestManifestRules`, `TestOpsAgentsHoldNoMemberScopes` |
| Hash-chained, append-only audit log; any edit or deletion detected | `Entry.digest`, `VerifyChain` | `TestAuditChain` |

**Production wiring (P1):** the gate runs as a sidecar library in each agent and as a central `agent-registry` service. The audit chain is written to an append-only table (§10, `ops.agent_decision`) and anchored hourly to an object-lock (WORM) bucket, so even a database administrator cannot rewrite history unnoticed. Pattern: Certificate Transparency and AWS QLDB-style ledgers.

### 5.5 Core agents (the brief's fifteen)

Each entry gives purpose, inputs, outputs, permissions, triggers, workflow, escalation, APIs used and business value. "Gate" means the action passes through §5.4.

#### Onboarding Agent

| | |
|---|---|
| Purpose | Get a new member from sign-up to V2 with the fewest drop-offs, without ever weakening verification |
| Inputs | Onboarding state, provider responses (age assurance, liveness), device type, jurisdiction |
| Outputs | Next-step routing, plain-language explanations, retry guidance, provider fallback choice |
| Permissions | L2; `identity_db` (state only), `profile_db` (onboarding fields); actions `route_verification_step` (reversible), `explain_step` (recommend) |
| Triggers | `auth.verified`, `assurance.failed`, onboarding idle > 10 min |
| Workflow | 1 read state → 2 choose next step (passkey, B2 enrolment, age assurance, Cartographer) → 3 if provider A fails or is down, route to provider B (03 §2.1) → 4 explain without revealing why a check failed (anti-gaming) |
| Escalation | Two provider failures → manual review queue; suspected minor → Guardian, onboarding stopped (always-human) |
| APIs | `/v1/auth/*`, `/v1/assurance/*`, provider adapters |
| Value | V2 completion 52% → 68% target (01 §13.1); fewer support tickets |

#### Compliance Agent

| | |
|---|---|
| Purpose | Keep TRYST provably compliant: DSRs, retention, OSA duties, consent records |
| Inputs | DSR requests, retention policies (02 §7), consent ledger, enforcement records, regulatory feed |
| Outputs | DSR exports, retention violations, statements of reasons, transparency report drafts, regulator packs |
| Permissions | L2; `run_dsr_export` (reversible), `check_retention` (read), `erase_account` (irreversible → human, except member-initiated erasure, which is the member's own action) |
| Triggers | `dsr.requested`, nightly retention sweep, `enforcement.decided`, regulator request |
| Workflow | DSR: verify requester via step-up → assemble export across stores → member downloads (30-day statutory clock tracked). Retention: diff actual vs policy → open a ticket per violation → Privacy Auditor blocks release if critical |
| Escalation | Any regulator request, any breach indicator → DPO within 1 h; 72 h ICO clock starts on awareness |
| APIs | `/v1/me/export`, `/v1/me` DELETE, `ops.deletion_job`, safety-svc |
| Value | Avoids fines; makes the transparency report a by-product |

#### Risk Agent

| | |
|---|---|
| Purpose | Score sign-up and account risk (ban evasion, fake profiles, coordinated abuse) |
| Inputs | Admission L1–L5 signals (03 §8), device and payment fingerprints (HMAC only), graph features |
| Outputs | Risk score with reason codes; step-up requirement; ban-anchor match proposals |
| Permissions | L2; `score_signup` (recommend), `require_step_up` (reversible), `ban_anchor_match` (member_safety → human) |
| Triggers | `auth.verified`, `assurance.passed`, graph anomaly |
| Workflow | Score → if medium, require step-up → if a ban-anchor match, open a T&S case with the evidence |
| Escalation | Every anchor match → moderator; false-positive appeals go to a second moderator |
| APIs | identity-svc admission, safety-svc |
| Value | Keeps fakes out (the category's main failure) |

#### Revenue Agent

| | |
|---|---|
| Purpose | Explain revenue movement and forecast it |
| Inputs | Warehouse aggregates: entitlements, Keys ledger, processor settlements, churn |
| Outputs | Daily revenue pack, cohort LTV, forecast with intervals, anomaly alerts |
| Permissions | L1; `warehouse_aggregates` only |
| Triggers | Daily 06:00 UTC; anomaly > 3σ |
| Workflow | Aggregate → compare with forecast → explain drivers → post to the finance centre |
| Escalation | Chargeback ratio ≥ 0.5% → payments lead; revenue drop > 15% day on day → CFO |
| APIs | Warehouse SQL (read-only role) |
| Value | Finance runs on facts, not spreadsheets |

#### Pricing Agent

| | |
|---|---|
| Purpose | Design and analyse **market-level** price experiments |
| Inputs | Conversion by market and tier, willingness-to-pay surveys, competitor price list |
| Outputs | Experiment proposals with power analysis; results with confidence intervals |
| Permissions | L1; recommend only; `change_price` is money_movement → human |
| Triggers | Quarterly review; G-P3 pricing decisions (D-05) |
| Workflow | Propose → pricing committee approves → Stripe Prices created by a person → measure → report |
| Escalation | Any proposal that personalises price per member is rejected by policy (§12.2) |
| APIs | Warehouse; Stripe Prices (human-operated) |
| Value | Better price points without eroding trust |

#### Customer Support Agent

| | |
|---|---|
| Purpose | Answer members quickly and discreetly |
| Inputs | Ticket text (member-provided), the member's entitlement and Keys ledger (case-scoped), help centre |
| Outputs | Draft replies; ticket classification; suggested refunds under published rules |
| Permissions | L1; `draft_reply` (recommend); `issue_goodwill_keys` (money_movement → human) |
| Triggers | `ticket.created` |
| Workflow | Classify → retrieve help-centre answer (RAG over public docs) → draft → agent or person sends. Safety keywords → Guardian queue immediately |
| Escalation | Safety, legal or payment disputes → human queue with SLA |
| APIs | Support desk connector (§8.2), `/v1/entitlements` (case-scoped) |
| Value | Lower cost per ticket; faster first response |

#### Marketing Agent (Herald)

Specified in 03 §3.2. Purpose: organic growth through earned search visibility. Inputs: public web, Search Console, aggregate cookieless analytics. Outputs: drafts and refresh proposals. Permissions: L2 for internal-link repair; `publish_page` is irreversible → editor. Escalation: claims-discipline flags → editor. Value: low-CAC channel (D-21).

#### Data Intelligence Agent

| | |
|---|---|
| Purpose | Answer business questions over the warehouse with correct, privacy-safe SQL |
| Inputs | Natural-language question from an operator; semantic layer (metric definitions) |
| Outputs | Query, result, chart, caveats |
| Permissions | L1; read-only warehouse role over aggregate marts; member-level tables not granted |
| Triggers | Operator request; scheduled packs |
| Workflow | Map question to metric definitions → generate SQL → run with row-count floor (k ≥ 20) → add DP noise to member-derived counts → render |
| Escalation | A question that needs member-level data → refused, with the route to a DPO-approved analysis |
| APIs | ClickHouse (read-only), semantic layer |
| Value | Self-serve analytics without exposing members |

#### Operations Agent

Purpose: keep queues inside SLA. Inputs: queue depths, SLA timers, staffing rota. Outputs: staffing forecasts, rebalancing proposals, SLA breach alerts. Permissions: L1. Triggers: every 5 min. Escalation: forecast breach of OSA-relevant SLAs → Head of T&S. Value: 24/7 coverage with a small team.

#### Fraud Detection Agent

| | |
|---|---|
| Purpose | Stop card testing, stolen cards, voucher abuse and Keys fraud |
| Inputs | Processor risk signals (Stripe Radar, acquirer scores, BitriPay status), payment fingerprint HMAC, velocity, device risk |
| Outputs | Transaction score, holds, block-list proposals |
| Permissions | L2; `score_payment` (recommend), `hold_payment` (reversible), `refund` (money_movement → human) |
| Triggers | `payment.attempted`, `voucher.redeem_attempted`, chargeback alert (Ethoca/Verifi) |
| Workflow | Score → hold above threshold → auto-refund on a confirmed fraud alert where the published rule allows (03 §10) → feed L1 admission |
| Escalation | Chargeback ratio ≥ 0.5% → payments lead; 0.65% → hard-stop review |
| APIs | `PaymentProvider` webhooks, voucher service |
| Value | Keeps the MID alive (above 0.9% it is lost, 01 §13.1) |

#### Payment Agent

| | |
|---|---|
| Purpose | Keep money flowing correctly across processors |
| Inputs | Webhooks from Stripe, the acquirer and BitriPay; ledger; processor health |
| Outputs | Reconciliation reports, processor routing proposals, dispute evidence packs |
| Permissions | L2; `reconcile_ledger` and `route_processor` (reversible: failover to the backup MID), `change_price` (money_movement → human) |
| Triggers | Webhook received; nightly reconciliation; authorisation-rate drop |
| Workflow | Verify signature → dedupe by event id → update entitlement and ledger → reconcile daily against settlement statements → if the primary's authorisation rate drops below its floor for 15 min, fail new checkouts over to the backup (reversible) and page payments ops |
| Escalation | Ledger imbalance → stop entitlement writes, page on-call; dispute deadline < 48 h → payments ops |
| APIs | `/v1/billing/*/webhook`, `PaymentProvider` |
| Value | Revenue continuity; dispute win rate |

#### API Integration Agent

Purpose: keep every connector healthy. Inputs: connector contract tests, error rates, key ages. Outputs: health board, rotation reminders, deprecation alerts. Permissions: L2 (open tickets, disable a failing optional connector). Escalation: a required connector failing (age assurance, payments) → on-call. Value: fewer outages caused by third parties.

#### Workflow Automation Agent (Orchestrator)

Runs declared workflows (§5.7) step by step, with retries, compensation and the gate at every action. It never invents a workflow; workflows are code-reviewed definitions. Value: reliable multi-step operations (DSR, erasure, dispute, incident).

#### Predictive Growth Agent

Purpose: forecast liquidity per city, segment and mode (S4 scarcity, gender ratio ≤ 2.2:1). Inputs: aggregates. Outputs: invite pacing and seeding recommendations (01 §11). Permissions: L1. Escalation: a forecast breach of a no-go condition → CEO. Value: avoids launching cities that cannot work.

#### Admin Control Agent

Purpose: help admins run the Super Control Centre (§14). It summarises alerts, proposes actions and drafts runbook steps. It **never grants access**; just-in-time access is approved by a second admin. Permissions: L1.

### 5.6 Self-managing platform layer

| Agent | Monitors / does | Autonomous (inside budget) | Always a person | Pattern |
|---|---|---|---|---|
| **System Health** | Uptime, latency, errors, saturation (golden signals); SLO burn rates (multi-window, multi-burn-rate) | Open incident, page on-call, attach traces | — | Google SRE workbook |
| **Bug Detection** | Error-tracking clusters, regression detection on deploy, flaky-test detection, crash-free rate | File issue with repro and suspected commit (git bisect in CI) | — | Meta SapFix/Sapienz-style triage; Sentry clustering |
| **Auto-Repair** | Runs pre-approved runbooks: rollback the last release, restart a crash-looping pod, scale out, drain a bad node, rotate a leaked key | L3 runbooks (budget 50/day) | **Deploying a code patch** (production_change). It may *open* a fix PR, which follows normal review and CI ([D-28](06_Decisions_and_Changes.md#2-decision-log)) | Kubernetes self-healing; Argo Rollouts; Netflix Spinnaker automated canary analysis |
| **Infrastructure Optimisation** | Compute, storage, bandwidth, cloud spend; GPU utilisation of the vLLM pool | Proposals only | Applying rightsizing | AWS Compute Optimizer; FinOps Foundation |
| **Release Management** | Canary analysis, progressive delivery (1% → 10% → 50% → 100%), feature flags, freeze windows | Halt a canary; roll back | Promoting a release | Google, Facebook progressive rollouts |
| **AI Governance** | The gate (§5.4), model and prompt registry, eval results, drift monitors, red-team results | Deny; kill an agent on policy breach (e.g. a guardrail violation rate above threshold) | Resuming a killed agent; changing a manifest | NIST AI RMF; ISO/IEC 42001; EU AI Act logging duties |

**Self-healing sequence (J4):** SLO burn > 14.4× over 1 h → System Health opens INC → Auto-Repair checks whether a deploy happened in the last 2 h → if yes, rolls it back (runbook) → if the burn stops within 10 min, closes with a report for review; if not, pages on-call with everything gathered. Every step is a gate decision in the audit log.

### 5.7 Orchestration and agent memory

| Concern | Design | Pattern |
|---|---|---|
| Orchestration | Typed state machines per agent (LangGraph-style, 03 §2.1); cross-agent workflows on a durable workflow engine (Temporal) with compensation steps | Uber Cadence/Temporal; Stripe's idempotent job runners |
| Workflow catalogue (P1) | `onboarding`, `handshake` (Loop C), `meet_resolution` (Loop D), `dsr_export`, `erasure` (< 60 s), `report_to_decision`, `dispute`, `incident_selfheal`, `processor_failover`, `herald_publish` | — |
| Memory: short-term | Per-run state in the workflow engine; expires with the run | — |
| Memory: member long-term | Only the structured vectors (IntentVector, RevealedPreferenceVector) and the Discretion Policy. **No free-text memory of members**, no conversation memory (E2EE content is never available server-side) | P3 |
| Memory: operational | Knowledge graph of services, runbooks, incidents and decisions; vector index over runbooks, ADRs and the spec for retrieval | Palantir-style ontology; RAG |
| Tool use | Tools are gRPC methods listed in the manifest; arguments validated against Protobuf schemas | OpenAI and Anthropic tool-use with schema validation |
| Prompt and model registry | Versioned prompts and policies (`policies/<agent>.yaml`); a change is a production_change | MLflow-style registry |
| Injection defence | Untrusted text (profiles, tickets, web pages) is never placed in an instruction position; outputs are schema-validated; the injection red-team suite runs per release (03 §9) | OWASP LLM Top 10 |

---

## 6. Full platform modules

Every module the brief lists, mapped to TRYST. Existing member modules are in 04 §4 and 03 §2.2; this table adds the OS surfaces.

| Module | Users | What it does | Key screens / endpoints | Agents | Phase |
|---|---|---|---|---|---|
| Dashboards | All | Role-scoped overviews (§4) | `/account/insights` (member); `/ops/*` (operators) | Chief of Staff, executive copilots | P1–P2 |
| User accounts | Members | Sign-up, two-factor sign-in, devices, security activity, Burn, emergency lock, export, delete | `/join`, `/sign-in`, `/account`; `/v1/me`, `/v1/me/security` (built) | Onboarding, Identity risk | P1 (built in part) |
| Admin panels | Admins | Super Control Centre (§14) | `/ops/control` | Admin Control | P1 |
| Merchant portals | Partners (U8) | BitriPay connected-account view, commission statements | `/partners/merchant` | Payment | P4 |
| Partner portals | U2, U5 | Keys, webhooks, usage, invoices, contracts | `/partners` | API Integration, Licensee Success | P4 |
| API settings pages | U2, U7 | Create, scope and rotate keys; IP allow-list; webhook endpoints | `/developers/keys`, `/developers/webhooks` | API Integration | P4 |
| Payment settings | Members, payments ops | Member: method, billing email, cancel (one step). Ops: processor routing, descriptors | `/account/billing`; `/ops/payments` | Payment | P1 |
| Notification system | All | Masked push (no names or content, 04 §5), email for statutory notices, in-app inbox | Notification engine (§9.4) | Curtain decides masking | P1 |
| Reporting | Operators, regulators | Transparency report, KPI pack, finance pack | `/ops/reports`; regulator portal | Compliance, Revenue | P1 |
| Analytics | Operators | Semantic-layer metrics, funnels, cohorts, experiment readouts | `/ops/analytics` | Data Intelligence | P2 |
| Billing | Members | Invoices, receipts with neutral descriptor | `/account/billing` | Payment | P1 |
| Subscription | Members | TRYST+, ENVOY, DUO, GHOST; renewal notices; one-step cancel | `/v1/subscription*` | Payment, Retention | P1/P3 |
| Wallet | Members | **Keys balance** is TRYST's wallet: a closed-loop ledger, not stored value. No cash-out, no transfers between members | `/v1/entitlements` | Payment | P1 |
| Commission | Partners | Voucher-reseller and licensee referral commission, paid by split (BitriPay `application_fee`) or invoice | `/partners/commission` | Payment | P4 |
| Transaction | Ops | Ledger explorer: every entitlement and Keys movement with its processor event | `/ops/payments/ledger` | Payment, Fraud | P1 |
| Audit trail | Admins, regulators | Agent decisions (hash chain), admin actions, break-glass, data access | `/ops/audit` | AI Governance | P1 |
| Security | Members, SOC | Member: security activity, devices. SOC: alerts, incidents, containment | `/account`, `/ops/security` | Identity risk, SOC, Threat Hunter | P1 |
| Agent registry | Admins | Manifests, levels, budgets, kill switches, eval scores | `/ops/agents` | AI Governance | P1 |
| Connector registry | Admins | Each connector's data map, DPA status, health, kill switch | `/ops/connectors` | API Integration | P1 |

---

## 7. BitriPay payment gateway API door

### 7.1 Position in TRYST

BitriPay is integrated as a **core payment gateway layer behind the processor-agnostic `PaymentProvider` interface** (FR-078), next to Stripe and the high-risk acquirer ([D-26](06_Decisions_and_Changes.md#2-decision-log), FR-084).

- **Built now:** the adapter (`backend/internal/payments/bitripay.go`), tested against a BitriPay-shaped sandbox server.
- **Live use is gated by market.** BitriPay's documented rails are mobile-money operators in markets TRYST will never enter (01 §4.3; D-13). Live keys are therefore refused in code unless a market is enabled for both TRYST and BitriPay, with a recorded legal opinion and BitriPay's written acceptance of the business model (as for Stripe, D-22).
- **Where BitriPay is commercially useful to TRYST without member exposure:** (a) settling **partner commissions** through connected accounts (voucher resellers, licensees) in any market BitriPay serves; (b) the **licensing model** (§12.3), where a licensee in a BitriPay market runs its own product and is the merchant of record.

### 7.2 API door: what TRYST implements against BitriPay

From the BitriPay developer documentation supplied by the founder. Base URL `https://api.bitripay.com/v1`; test and live keys share it.

| Capability (brief) | BitriPay surface | TRYST implementation | Status |
|---|---|---|---|
| API key generation | `POST /api_keys` (sk_, rk_, pk_ with scopes); `POST /api_keys/{id}/rotate` (live needs step-up) | TRYST holds an **rk_ restricted key** with only the scopes it needs (below), in the vault; rotation every 90 days by the API Integration agent's reminder, done by a person | Spec |
| Sandbox mode | `sk_test_…`; magic MSISDNs (§7.6) | Adapter accepts test keys; CI contract tests use a BitriPay-shaped server | **Built** |
| Live mode | `sk_live_…` / `rk_live_…` | Refused unless `LiveMarkets[market]` is set | **Built** (gate) |
| Webhooks | `POST /webhook_endpoints` (whsec_ once); `BitriPay-Signature` HMAC + platform Ed25519 key from `GET /keys`; at-least-once, dedupe by event id; replay | `VerifyWebhook` (HMAC, 5-min tolerance), shared `Dedup`; Ed25519 second check in P0 once the header is confirmed | **Built** (HMAC) |
| Merchant onboarding | `POST /accounts` (business_name, type, email, country, application_fee_bps) → claim link `POST /accounts/{id}/account_links` | Partner portal creates the partner's connected account; the partner claims it and owns it | Spec (P4) |
| QR payment | `POST /qr_codes` (static BitriQR, EMVCo + signed extension); `qr_payload` on intents | Not used for members (QR payment at a venue would link a member to a place). Used for partner invoices only | Spec |
| Payment links | `POST /payment_links` | Licensee and partner invoices | Spec |
| Wallet payments | `GET /wallets`, `POST /transfers` | Not used for members (no stored value at TRYST). Partners can hold commission in their own BitriPay wallet | Spec |
| Settlement | `/settlement_profiles` (T0/T1/T2/weekly/manual), `/settlement_cycles/{id}/statement` (json, csv, pdf; numbered, hashed) | Payment agent reconciles statements against the ledger nightly | Spec |
| Refunds | `POST /refunds` (atomic reservation), `GET /payment_intents/{id}/refundable` | `Refund` (idempotent key per refund) | **Built** |
| Disputes | `GET /disputes`, `POST /disputes/{id}/respond` | Payment agent assembles evidence; payments ops submits | Spec |
| Commission split | `application_fee_minor` on intents; `BitriPay-Account` header for connected accounts; `/payment_intents/{id}/splits` | Licensee and reseller splits | Spec (P4) |
| Transaction monitoring | `GET /events`, `/payment_intents/{id}/timeline`, `/payment_resolution`, `GET /status` (degraded flags) | Payment agent watches `status` and fails over (reversible) | Spec |
| Developer documentation | API reference + OpenAPI 3.1 | Linked from the Developer Centre; our adapter's contract tests pin what we use | Spec |
| Plugin-ready integration logic | Hosted checkout, embedded widget (pk_), WooCommerce and Shopify plugins, SDKs | TRYST uses **hosted checkout** only (no third-party script on TRYST pages, NFR-20 and CSP) | Spec |

**Scopes TRYST requests (least privilege):** `payment_intents:write`, `payment_intents:read`, `checkout_sessions:write`, `refunds:write`, `refunds:read`, `subscriptions:read`, `subscriptions:write`, `disputes:read`, `disputes:write`, `settlements:read`, `balance:read`, `webhooks:manage`, `events:read`; for P4 partner payouts, `accounts:read`, `accounts:write`. **Never requested:** `payouts:approve` (four-eyes stays with people), `credit:read`, `remittances:*`, `ai:run`.

### 7.3 Merchant and partner onboarding flow

1. Partner applies in the partner portal → KYB by TRYST's KYB connector (§8.2) for licensees; resellers get a lighter check.
2. TRYST calls `POST /accounts` with the partner's business details and the agreed `application_fee_bps` → `acct_…`.
3. TRYST sends the claim link (`POST /accounts/{id}/account_links`, valid 7 days). The partner owns the account; TRYST appears under the partner's Team as a developer, with no access to settlement or payouts.
4. Sandbox integration tests pass (§7.6) → a person approves live mode in the connector registry (always-human).
5. The partner can remove TRYST at any time; `POST /accounts/{id}/detach` ends TRYST's access, and the money and history stay the partner's.

### 7.4 Commission split

For a partner sale billed through BitriPay, the intent carries `application_fee_minor` and the `BitriPay-Account: acct_…` header. The partner is the merchant of record; TRYST's fee is a transparent split shown on both statements. Refund behaviour on splits follows the intent's split-refund policy (`pro_rata` or `merchant_absorbs`, chosen per contract).

### 7.5 Payment state machine (as consumed by TRYST)

`requires_payment_method → processing → succeeded → settled`, with branches `ambiguous` (manual review at BitriPay; TRYST grants **no** entitlement until resolved), `failed` (back to requires_payment_method), `canceled`. Entitlements are written only from verified `payment_intent.succeeded` events; settlement events update the ledger only. An `ambiguous_hold` older than 24 h opens a payments-ops task.

### 7.6 Sandbox

BitriPay's magic MSISDNs drive the real attempt machine with test keys: `+243000000501` succeed; `+243000000404` fail (invalid_msisdn); `+243000000408` ambiguous → manual review; `+243000000500` timeout then succeed; `+243000000503` provider unavailable (retryable); any number ending `0000` declined. TRYST's P0 contract suite runs all six against BitriPay's sandbox and asserts the entitlement outcome for each (no entitlement on ambiguous, failed or declined).

### 7.7 Adapter implementation (built)

| Property | Code | Test |
|---|---|---|
| Bearer auth; `Idempotency-Key` on every money-moving POST, **derived deterministically** from the logical operation, so a retry can never charge twice | `post`, `idempotencyKey` | `TestBitriPayCheckoutSandbox` |
| Only `amount_minor`, `currency`, `description` are sent: no member identifier, nothing that reveals the service beyond a neutral descriptor | `CreateCheckout` | `TestBitriPayCheckoutSandbox` (asserts the exact field set) |
| Descriptor must be neutral and never name TRYST (ToS 16.2) | `ValidDescriptor` | `TestBitriPayRejectsPublishableKeyAndBadDescriptor` |
| Publishable keys refused server-side; live keys refused outside enabled markets; refused calls never reach the network | `check` | `TestBitriPayLiveRefusedOutsideEnabledMarkets` |
| Refunds and one-step cancel | `Refund`, `CancelSubscription` | `TestBitriPayRefundCancelAndErrors` |
| BitriPay error codes surfaced (`scope_denied` etc.) | `APIError` | same |
| Webhook HMAC with replay tolerance; empty secret fails closed | `VerifyWebhook` | `TestBitriPayWebhook` |

Two wire details are not in the supplied documentation and are marked **ASSUMED** in the code: the refund request field names and the `BitriPay-Signature` header layout. They are a P0 check against BitriPay's API reference (00 §5).

---

## 8. Third-party API connectors

### 8.1 Connector rules (FR-087)

1. **Every connector is registered** with: purpose, the module it connects to, the exact data sent and received, lawful basis, DPA and transfer mechanism, region, health check, kill switch and owner.
2. **No special-category or member-identifying data goes to a connector unless the connector is the processor for exactly that purpose** (e.g. the age-assurance provider). Nothing reaches CRM, marketing, enrichment, ad or messaging-app platforms.
3. **Two providers for anything on the critical path** (age assurance, payments, email for statutory notices).
4. **Adapters, not SDK sprawl.** Each category sits behind one internal interface (as `PaymentProvider` does), so switching providers is configuration.
5. **AI providers:** hosted frontier models only for generic copy with PII redacted at the boundary; anything touching member data runs on self-hosted open-weight models in the UK (03 §2.1).

### 8.2 Connector register

| Category | Why needed | Connects to | Sends / receives | Provider options (primary · alternative) | TRYST rule |
|---|---|---|---|---|---|
| Payments | Subscriptions, Keys, deposit, refunds | `commerce` via `PaymentProvider` | Sends amount, currency, neutral descriptor, opaque billing ref; receives payment events | **Stripe** (conditional, D-22) · high-risk acquirer (backup MID) · **BitriPay** (gated, D-26) · Adyen · Checkout.com · PayPal | Adyen and Checkout.com are evaluated as the high-risk backup in P0; PayPal is unlikely to accept the category (verify) |
| Banking-as-a-Service | Not needed for members (TRYST holds no stored value) | Treasury (ops) | Company accounts only | Business bank with API (e.g. Modulr, ClearBank partners) | Operator-only |
| Open banking | Optional pay-by-bank in UK/EU, lower fees, no card data | `commerce` | Payment initiation; no account data retained | TrueLayer · Yapily · Stripe (pay by bank) | P3 evaluation; descriptor rules apply |
| KYC / age assurance | Highly effective age assurance (OSA); V2 | identity-svc | Sends selfie/document to provider; receives pass/fail + token | Two providers: **Yoti** · **Persona** · **Veriff** · **Sumsub** · Onfido | No document image retained by TRYST (A §8.1) |
| KYB | Licensees and commission partners | Partner onboarding | Business registration data | Sumsub KYB · Persona · Companies House API | B2B only |
| AML screening | Sanctions and PEP screening of **businesses** (licensees, partners). Member AML duties sit with the regulated PSP | Partner onboarding | Business and director names | ComplyAdvantage · Sumsub | D-32 |
| Fraud prevention | Card testing, ATO, bots | Payment, identity, edge | Device signals, payment risk | Stripe Radar · Ethoca/Verifi alerts · Cloudflare Bot Management · Fingerprint (device, hashed) | Device IDs hashed; no cross-site tracking |
| Identity verification (liveness B2) | Login factor B2 (02 §3.3) | identity-svc | Liveness session; match result | iProov · FaceTec · Persona | PAD Level 2 (NFR-17); D-18 |
| Email | Statutory notices (renewal, cooling-off), sign-in alerts | Notification engine | Billing email (member-chosen), neutral subject lines | Postmark · Amazon SES · SendGrid · Brevo | No tracking pixels; neutral sender |
| SMS | **Sign-up OTP only**; never a login factor (D-16) | identity-svc | Phone number, code | Twilio Verify · MessageBird | Neutral sender ID |
| WhatsApp | — | — | — | — | **Declined for members** (D-29): exposes the contact on a shared device and to a third party. Operator alerts may use Slack/PagerDuty instead |
| Push notifications | Masked alerts | Notification engine | Opaque payload; content fetched in app | APNs · FCM | No names or content (04 §5) |
| Maps | Venue sanity (public venue), travel mode, zone drawing | Client, Envoy logistics | Coarse geohash only; no live tracking | Mapbox (self-hosted tiles where possible) · Google Maps Platform | Exact locations never leave the device (FR-067) |
| Logistics | Not applicable (no physical goods) | — | — | — | Recorded as not applicable |
| Accounting | Revenue recognition, VAT returns | Finance (ops) | Aggregated revenue journals; no member data | Xero · NetSuite | Aggregates only |
| Tax | VAT/OSS calculation | `commerce` | Country + amount | Stripe Tax · Avalara | — |
| CRM | **B2B pipeline only** (licensees, partners, press) | Sales copilot | Business contacts | HubSpot · Salesforce | **No member data, ever** (D-29) |
| Analytics | Product and marketing-site analytics | Warehouse; marketing site | Cookieless aggregate; pseudonymous product events | Self-hosted: ClickHouse + Plausible/Umami | No third-party tracking scripts (NFR-20) |
| AI model providers | Generic copy (Herald, help centre); coding copilots | `llm-gateway` | Redacted prompts | Anthropic · OpenAI · Google Gemini / Vertex AI · Cohere · Mistral; **self-hosted** open-weight (vLLM) for member data | Zero-retention terms required for hosted providers |
| Cloud | Compute, storage, KMS/HSM | All | — | **AWS eu-west-2** (primary) · Azure / GCP for DR evaluation | UK/EU residency |
| Cloud storage | Encrypted media, backups, WORM audit anchors | media-svc, ops | Ciphertext only | S3 (object lock) · Cloudflare R2 (egress-free CDN origin for marketing assets) | Client-side encryption for member media |
| Authentication | Members: passkeys (built, go-webauthn). Staff: SSO | identity-svc; ops | — | Members: in-house WebAuthn. Staff: Okta / Google Workspace SSO + hardware keys | No third-party IdP for members |
| Productivity | Staff collaboration | Ops | No member data | Google Workspace · Microsoft 365 | DLP rules block member identifiers |
| Document generation | Statements of reasons, DSR exports, invoices, transparency report | Compliance, Payment | Generated server-side | In-house (HTML → PDF with headless Chromium, as the spec PDF) | Not sent to third parties |
| E-signature | Partner, licensee and vendor contracts | Partner onboarding | Business signatories | DocuSign · Dropbox Sign | B2B only |
| Customer support | Ticketing | Support agent | Ticket text, opaque member ref | Zendesk · Help Scout · Intercom (without its tracking widget) | Case-scoped; neutral email domain |
| Data enrichment | — | — | — | — | **Declined for members** (D-29): enrichment would link a pseudonymous member to a real identity, breaking FR-002. B2B lead enrichment allowed in the CRM |
| Currency exchange | Display prices in EUR, SEK, DKK in P4 | `commerce` | Rates only | Processor-native pricing (Stripe multi-currency) · ECB reference rates | Prices set per market, not converted live |
| Subscription billing | Plans, renewals, notices | `commerce` | Via the payment adapters | Stripe Billing · BitriPay `/plans` and `/subscriptions` (gated) | DMCC/CCR notices (03 §10) |
| Observability | Metrics, logs, traces | All services | No member content; pseudonymous IDs | Self-hosted Grafana stack (Prometheus, Loki, Tempo) + OpenTelemetry | Logs scrubbed at source |
| Security | WAF, DDoS, secrets, scanning | Edge, CI | — | Cloudflare · AWS Shield · HashiCorp Vault / AWS Secrets Manager · Snyk / GitHub Advanced Security | — |
| Incident and on-call | Paging | System Health | Incident metadata | PagerDuty · Opsgenie | — |

---

## 9. Production-grade architecture

### 9.1 Proven patterns used, and where they come from

| Pattern | Proven at | Used for |
|---|---|---|
| Idempotency keys on every mutating request | Stripe | Edge API (02 §5), all payment adapters |
| Signed webhooks with timestamp tolerance and replay | Stripe, GitHub | Inbound processor webhooks; outbound partner webhooks (§11.4) |
| Zero trust / BeyondCorp: identity- and device-aware access, no trusted network | Google | Staff access; service-to-service mTLS (SPIFFE) |
| Event streaming with a schema registry | LinkedIn (Kafka), Uber | Redpanda + Protobuf registry (03 §2.1) |
| Durable workflows with compensation | Uber (Cadence → Temporal), Netflix (Conductor) | §5.7 workflow catalogue |
| Progressive delivery with automated canary analysis | Netflix (Kayenta), Google | Release Manager (§5.6) |
| SLOs and error budgets; multi-window burn-rate alerts | Google SRE | System Health |
| Feature store for train/serve parity | Uber (Michelangelo), Airbnb (Zipline) | Feast (03 §2.1) |
| Two-tower retrieval + learned ranking with constraints | YouTube, Pinterest, Airbnb search | Broker (03 §5) |
| Ontology / knowledge graph over operations | Palantir | Operational memory (§5.7) |
| Risk engine with explainable reason codes | Stripe Radar, Goldman Sachs Aladdin risk factors | Fraud, Risk |
| Cell-based architecture to cap blast radius | AWS, Slack | §9.8 (P4) |
| Lakehouse with governed semantic layer | Databricks, Snowflake | §9.6 |
| Edge WAF, bot management, DDoS absorption | Cloudflare | §13 |
| Endpoint and identity threat detection | CrowdStrike | SOC and Threat Hunter design |
| Workflow-centric operations consoles | ServiceNow | Operator command centres (§4) |
| Agent tool use with schema validation and human approval | OpenAI, Anthropic, Microsoft | §5 |

### 9.2 Layered architecture

```
 Clients ─ iOS / Android (native, Rust crypto core) · Web PWA (Next.js) · Marketing site · Ops console · Partner portal
    │  TLS 1.3, passkeys, DPoP-bound tokens
 Edge ─ Cloudflare (WAF, DDoS, bot) → edge-gateway (Go): authN, jurisdiction, tier, consent, rate limit, idempotency
    │  gRPC + mTLS (SPIFFE)
 Domain ─ identity-svc · core-svc (profile, couple, intent, discretion, discovery, reveal, threads/MLS, meet, consent, deletion, commerce)
          media-svc · safety-svc · jobs · webhook-engine · notification-engine · partner-api (P4)
    │  typed Protobuf events (Redpanda + schema registry)
 Agents ─ member plane: Cartographer, Mirror, Broker, Envoy, Curtain, Guardian, Aftercare, Onboarding, Risk, Fraud, Support, Payment, Compliance, Chief of Staff
          ops plane: Herald*, System Health, Bug Detection, Auto-Repair, Release Manager, Infra Optimiser, SOC, Threat Hunter, Vulnerability, analysts
          every action → aigov gate → audit chain        (*Herald: separate cloud account)
 AI runtime ─ llm-gateway (vLLM pool, self-hosted, UK) · hosted LLM proxy (redacted, generic copy only) · model + prompt registry · eval harness
 Data ─ IDENTITY-DB · PROFILE-DB · SAFETY-DB · BAN-DB (isolated PostgreSQL clusters) · Qdrant · Feast · ClickHouse · S3 (ciphertext, object lock) · KMS/HSM
 Platform ─ Kubernetes (EKS) · Argo CD · Terraform · Vault · OpenTelemetry → Grafana stack · PagerDuty · Temporal
```

### 9.3 Component specification

| Component | Specification |
|---|---|
| Frontend | Native Swift / Kotlin with a shared Rust crypto core (E-07); Next.js 16 PWA (built: join, enrol, sign-in, approve, account); ops console and partner portal as separate Next.js apps on separate origins with their own CSP |
| Backend | Go services (03 §2.2); modular monolith `core-svc` with split triggers (§9.8) |
| Database | PostgreSQL 16 clusters isolated per domain; pgvector; migrations generated from 02 §4 (`scripts/gen_migrations.py`, drift-checked in CI) |
| Authentication | Members: account-holder-only passkeys + second factor (built). Staff: SSO + FIDO2 hardware key + device posture. Services: SPIFFE mTLS. Partners: scoped API keys + mTLS for webhooks |
| Role-based access control | RBAC for staff roles, plus ABAC (03 §6.5) on case, purpose and data class; just-in-time elevation with a second approver; all access logged |
| AI orchestration layer | Agent services + Temporal workflows + `aigov` gate (§5) |
| Agent memory layer | Structured member vectors only; operational knowledge graph and runbook vector index (§5.7) |
| Vector database | Qdrant for candidate retrieval (filtered ANN after Stage 0 gates); a separate Qdrant collection in the ops plane for runbook and spec retrieval |
| Event-driven workflows | Redpanda topics `member.*`, `agent.*`, `payment.*`, `safety.*`, `ops.*`; Protobuf schemas with compatibility checks; outbox pattern from PostgreSQL to avoid dual writes |
| API gateway | `edge-gateway` (built skeleton): middleware order in 02 §5; per-key and per-subject rate limits (token bucket in Redis) |
| Webhook engine | Inbound: verify signature → dedupe → enqueue → process idempotently. Outbound (partners, P4): signed `TRYST-Signature` (§11.4), exponential back-off for 72 h, delivery log, replay, endpoint auto-disable after 5 days of failure |
| Payment gateway layer | `PaymentProvider` with Stripe, acquirer and BitriPay adapters; the Payment agent's failover (§5.5) |
| Notification engine | Channel router (push, email, in-app); Curtain-controlled masking; quiet hours; statutory notices bypass quiet hours but stay neutral |
| Audit log | Two chains: agent decisions (`aigov`) and admin actions; both anchored hourly to S3 object lock |
| Admin control layer | Super Control Centre (§14) over the ops API; break-glass with two-person approval |
| Security layer | §13 |
| Observability | OpenTelemetry traces, metrics and logs; member IDs replaced by rotating pseudonyms; content never logged |
| Monitoring | SLOs per user-facing journey (§9.5); synthetic probes for sign-in, slate, checkout; Privacy Auditor probes (03 §9) |
| Error handling | RFC 9457 problem details with stable codes (02 §5.10); uniform `404 not_found` for anything invisible; retries only on idempotent calls; circuit breakers per connector; fail closed for auth, payments and safety |
| Scalability logic | §9.8 |

### 9.4 Event architecture

| Topic family | Producers | Consumers | Retention | Notes |
|---|---|---|---|---|
| `member.*` (02 §8 taxonomy) | Clients via `/v1/signals:batch`, core-svc | Mirror, Broker, analytics | Raw 180 d, then purged | Pseudonymous; no content |
| `agent.*` | Agents | Agents, audit sink | 30 d | Typed turns (02 §6.1) |
| `payment.*` | Webhook engine | Payment, Fraud, ledger | 7 y (financial records), pseudonymised | — |
| `safety.*` | Guardian, safety-svc | T&S console, Compliance | Per SAFETY-DB policy | Access only via the trust console |
| `ops.*` | System Health, CI, deploys | Auto-Repair, Release Manager, Bug Detection | 90 d | No member data |
| `aigov.decision` | Gate | Audit sink, Admin Control | 7 y | Hash chain |

### 9.5 SLOs

| Journey | SLI | SLO |
|---|---|---|
| Sign-in (both factors) | Successful completions / attempts with valid factors | 99.9% / 30 d; p95 < 3 s excluding the member's biometric time |
| Slate | `/v1/slate` success and latency | 99.9%; p95 < 400 ms (NFR-02) |
| Thread delivery | MLS message relayed | 99.95% |
| Checkout | Checkout creation success | 99.9% across processors |
| Erasure | Completed < 60 s | 99% (NFR-01) |
| Emergency lock | Effective < 5 s | 95% (FR-064) |
| Agent gate | Decision latency | p99 < 5 ms in-process |

### 9.6 AI data intelligence layer

| Component (brief) | TRYST implementation | Privacy rule |
|---|---|---|
| Data lake | S3 (eu-west-2), Iceberg tables, partitioned by day; raw events land here pseudonymised | Raw behavioural events purged at 180 d |
| Data warehouse | ClickHouse marts (dbt-modelled); semantic layer of metric definitions (QCAM, mutual rate, etc.) | Aggregate marts only for analysts; k ≥ 20 row floor |
| Vector database | Qdrant (member plane) and a separate ops-plane collection | Member vectors never leave the member plane |
| Knowledge graph | Ops: services, runbooks, incidents, decisions (Neo4j or PostgreSQL + Apache AGE). Member plane: the block and exclusion graph used by Broker Stage 0 | The member graph is used for gates and abuse detection only |
| Event streaming platform | Redpanda + schema registry | §9.4 |
| Real-time analytics engine | ClickHouse materialised views over Redpanda for ops dashboards (queue depth, authorisation rate, SLOs) | No member-level real-time views outside T&S |
| Predictive intelligence engine | Liquidity forecasting (city × segment × mode), churn propensity (aggregate cohorts), capacity forecasting | Forecasts on aggregates; churn scores never shown to support staff as a label on a member |
| Behavioural intelligence engine | Mirror (03 §4): RevealedPreferenceVector, divergence, α weighting with the 15% stated-preference floor | Message content never used; personalisation can be switched off (FR-024) |
| Recommendation engine | Broker: Stage 0 gates → retrieval → M3/M4 ranking → fairness constraints (S4 caps, Gini) | Hard limits inviolable (P2) |
| Decision intelligence engine | Guardian triage, Risk and Fraud scoring, Payment failover rules, all with reason codes and gate decisions | Every decision explainable and audited |

**Data governance architecture:** a data catalogue with owner, classification (public, internal, personal, special-category, safety), lawful basis, retention and permitted purposes per dataset; column-level tags drive access policies; lineage from dbt; the Privacy Auditor tests that the catalogue matches reality each release.

### 9.7 Disaster recovery and business continuity

| Tier | Systems | RPO | RTO | Mechanism |
|---|---|---|---|---|
| T0 | identity-svc, edge, IDENTITY-DB, BAN-DB, KMS/HSM | 1 min | 30 min | Multi-AZ synchronous replicas; cross-region (eu-west-1) async replica; HSM cluster across AZs; quarterly restore drill |
| T1 | core-svc, PROFILE-DB, threads, payments | 5 min | 1 h | As above; Redpanda tiered storage |
| T2 | Agents, analytics | 24 h | 8 h | Rebuild from IaC and the lake |

Erasure must survive restore: deletion jobs are replayed against any restored backup before it serves traffic (crypto-shredded keys stay destroyed, 03 §6.3). **Business continuity:** backup MID live from day one; two age-assurance providers; staff runbooks for a full agent kill (the platform keeps running with agents at L0, falling back to the deterministic P1 baseline ranker, 03 §5.2).

### 9.8 Scalability

| Trigger | Action |
|---|---|
| A `core-svc` module > 40% of CPU or a separate team owns it | Split it into its own service behind the same gRPC contract |
| PROFILE-DB write p95 > 20 ms or > 2 TB | Partition by region cell, then shard by `profile_id` hash |
| > 500k verified members or a second region (P4) | **Cell-based architecture**: a cell = full stack for a market group (UK+IE, EU-North, EU-South); global services limited to identity routing and ban anchors |
| vLLM pool GPU utilisation > 70% p95 | Infra Optimiser proposes scale-out; a person applies it |
| Qdrant p95 retrieval > 40 ms | Add replicas; tighten Stage 0 pre-filters |

### 9.9 Deployment strategy

Trunk-based development; every merge runs CI (spec check, contracts lint, Go, Python, Rust, web, Playwright e2e, as in `.github/workflows/ci.yml`). Images are signed (Sigstore cosign) with SLSA provenance; Argo CD syncs from Git; progressive delivery 1% → 10% → 50% → 100% with automated canary analysis; feature flags for every user-visible change; freeze windows around launches. Agents ship with their eval suites as a release gate (03 §9). Infrastructure is Terraform with policy checks (OPA) in CI.

### 9.10 Testing strategy

| Layer | What | Where today |
|---|---|---|
| Unit | Each package | Go (`go test ./...`, including `aigov` and `payments`), Python (pytest + Hypothesis), Rust (`cargo test`), web (Vitest) |
| Property | Hard-limit inviolability, ranking invariants | `agents/tests` (Hypothesis) |
| Contract | OpenAPI lint; Protobuf compatibility; processor adapters against recorded sandbox behaviour | Redocly in CI; `bitripay_test.go` |
| End to end | Passkey sign-up and two-factor sign-in with virtual authenticators | Playwright (`web/e2e/passkey.spec.ts`) |
| Agent evals | 400-case golden set per agent; injection red team; workflow-level replay | 03 §9 (P1) |
| Governance | Always-human classes, kill switch, audit-chain tamper detection; mutation-checked | `aigov_test.go` |
| Privacy | Privacy Auditor probes: access, retention, model leakage | `jobs` (P1) |
| Load | k6 profiles per journey at 3× forecast peak | P1 |
| Chaos | AZ loss, connector outage (age assurance, payments), agent kill-all | Game days quarterly from P2 |
| Security | SAST, DAST, dependency and container scanning; quarterly pen test; annual red team; bug bounty (NFR-19) | CI + vendors |

---

## 10. Database schema

The member data model is owned by [02 §4](02_Shared_Contracts.md#4-data-model) and is not repeated here. The operating system adds the **ops** and **partner** schemas below, in PROFILE-DB's cluster but with separate roles, so no member-plane role can read partner data and no ops role can read member tables.

### 10.1 ERD (OS additions)

```
agent ───< agent_action           agent ───< agent_decision >─── staff_user (approver)
  │                                  │
  └───< agent_kill_event             └── prev_hash → agent_decision.hash (chain)
connector ───< connector_secret_ref     connector ───< connector_health
partner ───< partner_api_key            partner ───< webhook_endpoint ───< webhook_delivery
partner ───< usage_record (ACU)         partner ───< partner_payout (BitriPay acct)
payment_event (all processors) ──> commerce.entitlement / commerce.keys_ledger (02 §4.5)
staff_user ───< admin_action            staff_user ───< access_grant (JIT)
```

### 10.2 DDL

```sql
CREATE SCHEMA ops; CREATE SCHEMA partner;

CREATE TABLE ops.agent (
  agent_id      TEXT PRIMARY KEY,                       -- e.g. 'guardian'
  owner_team    TEXT NOT NULL,
  level         SMALLINT NOT NULL CHECK (level BETWEEN 0 AND 3),
  plane         TEXT NOT NULL CHECK (plane IN ('member','ops')),
  self_hosted   BOOLEAN NOT NULL,
  daily_budget  INT NOT NULL CHECK (daily_budget >= 0),
  data_scopes   TEXT[] NOT NULL,
  model_version TEXT, policy_version TEXT,
  killed        BOOLEAN NOT NULL DEFAULT false,
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (plane = 'member' OR NOT data_scopes && ARRAY['identity_db','profile_db','safety_db','ban_db','event_bus','feature_store','vector_store']),
  CHECK (plane = 'ops' OR self_hosted)
);
CREATE TABLE ops.agent_action (
  agent_id TEXT REFERENCES ops.agent ON DELETE CASCADE,
  action   TEXT NOT NULL,
  class    TEXT NOT NULL CHECK (class IN ('read','recommend','reversible','runbook','irreversible',
                                          'member_safety','money_movement','production_change')),
  PRIMARY KEY (agent_id, action)
);
CREATE TABLE ops.staff_user (
  staff_id UUID PRIMARY KEY, display_name TEXT NOT NULL, roles TEXT[] NOT NULL,
  can_approve BOOLEAN NOT NULL DEFAULT false, hw_key_enrolled BOOLEAN NOT NULL,
  disabled_at TIMESTAMPTZ
);
CREATE TABLE ops.agent_decision (                       -- append-only; hash chain (aigov)
  seq        BIGINT PRIMARY KEY,
  at         TIMESTAMPTZ NOT NULL,
  kind       TEXT NOT NULL,                             -- register | decide:allow | decide:needs_human | decide:deny | kill | kill_all | resume
  agent_id   TEXT NOT NULL,
  action     TEXT,
  approver   UUID REFERENCES ops.staff_user,
  subject    TEXT,                                      -- pseudonymous reference only
  detail     TEXT,
  prev_hash  TEXT NOT NULL,
  hash       TEXT NOT NULL UNIQUE
);
CREATE INDEX agent_decision_agent_at ON ops.agent_decision (agent_id, at DESC);
CREATE INDEX agent_decision_pending ON ops.agent_decision (at) WHERE kind = 'decide:needs_human';
REVOKE UPDATE, DELETE, TRUNCATE ON ops.agent_decision FROM PUBLIC;

CREATE TABLE ops.admin_action (                         -- append-only; same chain design
  seq BIGINT PRIMARY KEY, at TIMESTAMPTZ NOT NULL, staff_id UUID NOT NULL REFERENCES ops.staff_user,
  action TEXT NOT NULL, target TEXT, justification TEXT NOT NULL, second_approver UUID REFERENCES ops.staff_user,
  break_glass BOOLEAN NOT NULL DEFAULT false, prev_hash TEXT NOT NULL, hash TEXT NOT NULL UNIQUE,
  CHECK (NOT break_glass OR (second_approver IS NOT NULL AND second_approver <> staff_id))
);
CREATE TABLE ops.access_grant (
  grant_id UUID PRIMARY KEY, staff_id UUID NOT NULL REFERENCES ops.staff_user, role TEXT NOT NULL,
  case_ref TEXT, approved_by UUID NOT NULL REFERENCES ops.staff_user,
  starts_at TIMESTAMPTZ NOT NULL, expires_at TIMESTAMPTZ NOT NULL,
  CHECK (approved_by <> staff_id), CHECK (expires_at <= starts_at + interval '8 hours')
);
CREATE TABLE ops.connector (
  connector_id TEXT PRIMARY KEY,                        -- 'stripe', 'bitripay', 'yoti', ...
  category TEXT NOT NULL, purpose TEXT NOT NULL, module TEXT NOT NULL,
  data_sent TEXT[] NOT NULL, data_received TEXT[] NOT NULL,
  member_data BOOLEAN NOT NULL, lawful_basis TEXT, dpa_signed_at DATE, region TEXT NOT NULL,
  live_enabled BOOLEAN NOT NULL DEFAULT false, live_markets TEXT[] NOT NULL DEFAULT '{}',
  live_approved_by UUID REFERENCES ops.staff_user, owner_team TEXT NOT NULL,
  CHECK (NOT member_data OR dpa_signed_at IS NOT NULL),
  CHECK (NOT live_enabled OR live_approved_by IS NOT NULL)
);
CREATE TABLE ops.connector_secret_ref (                 -- the secret lives in Vault; only the path is here
  connector_id TEXT REFERENCES ops.connector, environment TEXT CHECK (environment IN ('sandbox','live')),
  vault_path TEXT NOT NULL, rotated_at TIMESTAMPTZ NOT NULL, PRIMARY KEY (connector_id, environment)
);
CREATE TABLE ops.payment_event (                        -- every processor webhook, verified
  provider TEXT NOT NULL, event_id TEXT NOT NULL, type TEXT NOT NULL, billing_ref TEXT,
  received_at TIMESTAMPTZ NOT NULL, processed_at TIMESTAMPTZ, payload_hash TEXT NOT NULL,
  PRIMARY KEY (provider, event_id)                      -- idempotency
);

CREATE TABLE partner.partner (
  partner_id UUID PRIMARY KEY, kind TEXT NOT NULL CHECK (kind IN ('licensee','reseller','assurance','content')),
  legal_name TEXT NOT NULL, country TEXT NOT NULL, kyb_status TEXT NOT NULL DEFAULT 'none',
  bitripay_account TEXT, application_fee_bps INT CHECK (application_fee_bps BETWEEN 0 AND 3000),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE partner.api_key (
  key_id UUID PRIMARY KEY, partner_id UUID NOT NULL REFERENCES partner.partner,
  prefix TEXT NOT NULL CHECK (prefix IN ('tk_test','tk_live')), secret_hash BYTEA NOT NULL,  -- Argon2id; secret shown once
  scopes TEXT[] NOT NULL, ip_allowlist CIDR[], created_at TIMESTAMPTZ NOT NULL, revoked_at TIMESTAMPTZ
);
CREATE TABLE partner.webhook_endpoint (
  endpoint_id UUID PRIMARY KEY, partner_id UUID NOT NULL REFERENCES partner.partner,
  url TEXT NOT NULL CHECK (url LIKE 'https://%'), secret_ref TEXT NOT NULL, event_types TEXT[] NOT NULL,
  disabled_at TIMESTAMPTZ
);
CREATE TABLE partner.webhook_delivery (
  delivery_id UUID PRIMARY KEY, endpoint_id UUID NOT NULL REFERENCES partner.webhook_endpoint,
  event_id UUID NOT NULL, attempt INT NOT NULL, status_code INT, next_attempt_at TIMESTAMPTZ,
  delivered_at TIMESTAMPTZ
);
CREATE INDEX webhook_delivery_due ON partner.webhook_delivery (next_attempt_at) WHERE delivered_at IS NULL;
CREATE TABLE partner.usage_record (                     -- ACU metering (B2B only, D-27)
  usage_id UUID PRIMARY KEY, partner_id UUID NOT NULL REFERENCES partner.partner,
  operation TEXT NOT NULL, acu NUMERIC(12,3) NOT NULL CHECK (acu >= 0),
  idempotency_key TEXT NOT NULL, at TIMESTAMPTZ NOT NULL, UNIQUE (partner_id, idempotency_key)
);
CREATE TABLE partner.acu_balance (
  partner_id UUID PRIMARY KEY REFERENCES partner.partner, prepaid_acu NUMERIC(14,3) NOT NULL CHECK (prepaid_acu >= 0),
  updated_at TIMESTAMPTZ NOT NULL
);
```

**Permissions.** Roles `member_plane_rw` (02 §4 schemas only), `ops_rw` (ops), `partner_rw` (partner), `analyst_ro` (aggregate marts in ClickHouse, not PostgreSQL), `auditor_ro` (ops.agent_decision, ops.admin_action). No role spans member and partner schemas. **Audit requirements:** append-only tables have UPDATE/DELETE revoked; hourly hash anchoring to S3 object lock; every `access_grant` and break-glass event is itself an `admin_action`.

---

## 11. API specification

### 11.1 Surfaces

| Surface | Base | Auth | Who |
|---|---|---|---|
| Member API (existing) | `/v1` (02 §5; `contracts/openapi/tryst.v1.yaml`) | Passkey session; DPoP from P1 | Members |
| Ops API | `/ops/v1` (internal origin, not internet-routable) | Staff SSO + hardware key + JIT role | Operators, admins |
| Partner API (P4) | `/partner/v1` | `Authorization: Bearer tk_live_…` + optional mTLS; scopes | Licensees, resellers |
| Inbound webhooks | `/v1/billing/{provider}/webhook` | Provider signature | Stripe, acquirer, BitriPay |
| Outbound webhooks | Partner URLs | `TRYST-Signature` | Partners |

### 11.2 Ops API endpoints (OS)

| Method | Path | Permission | Contract |
|---|---|---|---|
| GET | `/ops/v1/agents` | `agents:read` | Registry: manifests, level, budget used today, killed |
| POST | `/ops/v1/agents/{id}/kill` | `agents:kill` | `{reason}` → kill; audit entry. `id = *` kills all |
| POST | `/ops/v1/agents/{id}/resume` | `agents:resume` + second approver | Resume a killed agent |
| GET | `/ops/v1/decisions?agent=&outcome=needs_human` | `decisions:read` | Pending and past gate decisions |
| POST | `/ops/v1/decisions/{seq}/approve` | `decisions:approve` (named human with `can_approve`) | Approves a `needs_human` request; the agent retries with `approver` set |
| GET | `/ops/v1/audit/verify` | `audit:read` | Runs `VerifyChain`; returns `{ok, entries, last_anchor}` |
| GET | `/ops/v1/connectors` | `connectors:read` | Register with health |
| POST | `/ops/v1/connectors/{id}/live` | `connectors:live` + second approver | Enable live in listed markets |
| POST | `/ops/v1/payments/failover` | `payments:route` | Switch new checkouts to the backup processor |
| GET | `/ops/v1/slo` | `ops:read` | SLO status and error budgets |

**Example: approve a pending removal**

```http
POST /ops/v1/decisions/18421/approve
Authorization: Bearer <staff session>
Idempotency-Key: 6f1c…
Content-Type: application/json

{"justification": "Case C-2207: verified sextortion evidence; second review by moderator M-114"}
```
```json
{"seq": 18422, "kind": "decide:allow", "agent_id": "guardian", "action": "remove_member",
 "approver": "b6b1…", "reason": "member_safety approved by named human", "hash": "9c4e…"}
```

### 11.3 Partner API endpoints (P4)

| Method | Path | Scope | Contract | ACU |
|---|---|---|---|---|
| POST | `/partner/v1/consent/handshakes` | `handshake:write` | Run a schema-only agent handshake between two of the **licensee's own** users (the Envoy engine as a service) | 2.0 |
| POST | `/partner/v1/discretion/exclusion-check` | `psi:write` | ExclusionRing-style PSI round (client library does the blinding) | 0.5 |
| POST | `/partner/v1/safety/classify` | `safety:write` | On-premises Guardian model licence check-in (models run in the licensee's environment) | 0 (licence) |
| GET | `/partner/v1/usage` | `usage:read` | ACU used by day and operation | — |
| POST | `/partner/v1/webhook_endpoints` | `webhooks:manage` | Register an endpoint; secret returned once | — |

**Example: handshake request**

```json
POST /partner/v1/consent/handshakes
{"a": {"intent_shape": "one_off", "hard_limits": ["no_photos"], "windows": [["2026-11-02T18:00Z","2026-11-02T23:00Z"]]},
 "b": {"intent_shape": "one_off", "hard_limits": [], "windows": [["2026-11-02T20:00Z","2026-11-03T01:00Z"]]}}
→ 200 {"handshake_id": "hs_…", "outcome": "compatible", "blocking": [], "friction": ["reveal_pace"],
       "briefs": {"a": {...codes...}, "b": {...codes...}}, "acu": 2.0}
```

### 11.4 Webhooks

**Inbound** (processor → TRYST): verify signature (Stripe `Stripe-Signature`; BitriPay `BitriPay-Signature`; acquirer per contract) → 5-minute tolerance → dedupe on `(provider, event_id)` in `ops.payment_event` → process → 2xx. Unverified → 400 with no detail.

**Outbound** (TRYST → partner): header `TRYST-Signature: t=<unix>,v1=<hex HMAC-SHA256(secret, "<t>.<body>")>`; events `handshake.completed`, `usage.threshold_reached`, `invoice.issued`, `key.rotation_due`; at-least-once; partners dedupe on `event_id`; retries at 1 m, 5 m, 30 m, 2 h, 6 h, then every 12 h for 72 h; replay via the partner portal.

### 11.5 Rate limits

| Surface | Limit |
|---|---|
| Member API | Per endpoint (02 §5); sign-in begin 10/min per IP-range bucket and 5/min per contact hint |
| Ops API | 600/min per staff session; kill and approve endpoints 30/min |
| Partner API | Plan-based: Sandbox 60/min; Growth 600/min; Enterprise contractual; burst 2× for 10 s |
| Webhook intake | 2,000/min per provider; above that, 429 and the provider retries |

### 11.6 Error codes (additions to 02 §5.10)

| HTTP | `code` | Meaning |
|---|---|---|
| 403 | `scope_denied` | Key lacks the scope |
| 403 | `approval_required` | Action is always-human; a pending decision was created (`decision_seq` returned) |
| 409 | `agent_killed` | Agent is stopped by the kill switch |
| 402 | `acu_insufficient` | Partner prepaid ACU exhausted |
| 403 | `live_not_enabled` | Connector or key not enabled for live in this market |
| 400 | `signature_invalid` | Webhook signature failed (no further detail) |

---

## 12. Monetisation model

### 12.1 Principles that bind the model

Charge for agency and discretion; make resolution the profitable outcome (01 §10.1). P8 stays in force: no advertising, no data sale, no affiliates, no fake profiles, no unreadable credit mechanics, no engagement paywalls, no charge for deletion or data access, replying always free. Every revenue line below is checked against it.

### 12.2 Revenue lines (brief → TRYST)

| Brief line | TRYST line | Price | Status | Phase |
|---|---|---|---|---|
| Subscription plans | Verified (free + £9.99 refundable deposit), TRYST+ £24.99/mo (£14.99 annual), ENVOY £49.99/mo, DUO £39.99/mo per couple | 01 §10.2 | Canonical | P1/P3 |
| Add-ons | GHOST £7.99/mo (D-05) | 01 §10.2 | Canonical | P3 |
| Transaction fees | **Keys** packs £9.99/60 (intent costs refunded on reply) | 01 §10.2 | Canonical | P1 |
| AI credit / ACU system | **Partner API metering only** (D-27). Members never buy ACU | Illustrative: £0.04 per ACU prepaid; volume bands | Adapted | P4 |
| API usage fees | Partner API plans: Sandbox free; Growth £1,500/mo incl. 50k ACU; Enterprise contract | Illustrative | Spec | P4 |
| Commission model | Reseller commission on voucher sales (paid out, a cost to TRYST); licensee referral split via BitriPay `application_fee` or invoice | Contract | Spec | P3/P4 |
| Merchant fees | Not applicable to members (no merchants on the member platform). For licensees on BitriPay rails, BitriPay's fees apply to them; TRYST takes an application fee where it provides the integration | Contract | Adapted | P4 |
| Partner fees | Assurance and content partners are suppliers, not fee payers | — | Not applicable | — |
| Premium automation fees | The **ENVOY tier** is the premium automation product for members | £49.99/mo | Canonical | P2/P3 |
| Data intelligence revenue | **Declined for member data** (P8; D-30). Alternative: free published aggregate research with differential privacy as a PR asset (Herald) | — | Declined | — |
| White-label licensing | Licensing the governed stack (handshake engine, PSI exclusion, on-device safety models, governance gate) to other platforms; **no member data and no shared member pool** | Illustrative: £60–250k/yr per licensee + ACU | Spec | P4+ |
| Enterprise plans | Enterprise licensing with SLA, on-premises models, dedicated support | Contract | Spec | P4+ |
| BitriPay gateway revenue | Application fees on licensee and reseller flows routed through BitriPay connected accounts | bps per contract | Spec (gated) | P4 |
| Marketplace revenue | No marketplace of people or services (commercial sexual services are prohibited, 01 §4.1). A **vetted integrations marketplace** for licensees (e.g. assurance providers) with referral fees is possible | Contract | Adapted | P4+ |
| Dynamic pricing engine | Market-level list prices tested by experiment; the same price for every member in a market; published; no personalised or surge pricing | — | Adapted (D-30) | P3 |

### 12.3 B2B licensing, white label and API revenue

**What is licensed:** the Envoy handshake engine (schema-only negotiation, asymmetric Briefs), the ExclusionRing PSI protocol and client library, the on-device Guardian models, Curtain's discretion policy engine and the `aigov` governance gate. **What is never licensed or shared:** TRYST members, their data, vectors or any model trained on them without a separate, lawful basis (models for licensing are trained on licensed or synthetic data). Licensees run the stack in their own environment; TRYST's Partner API meters calls in ACU.

### 12.4 Unit economics impact

The OS changes 01 §10.3 in three ways (to be measured in beta, not assumed):

1. **Cost per MAU down:** agent-handled support and T&S triage lower the variable ops cost, the main threat to gross margin identified in E-33.
2. **Payer rate up:** ENVOY's value depends on G-P2 (meets per intent +40%); the OS's job is to make that measurable and explainable.
3. **New revenue not dependent on member growth:** B2B licensing from P4, with near-zero marginal member risk.

### 12.5 Revenue optimisation, CLV, churn, upsell and cross-sell engines

| Engine | What it does | What it may never do |
|---|---|---|
| AI revenue optimisation | Finds revenue leaks: failed renewals (dunning through the processor), processor authorisation drops, refund causes | Raise price for a member individually; obscure costs |
| Customer lifetime value | Cohort CLV by acquisition channel and segment, for CAC decisions (E-03 thresholds) | Label individual members by value in any staff-facing view |
| Churn prevention | Detects cohort-level churn drivers; fixes product causes (slate quality, liquidity) | Retention flows on cancel (one step, ToS cl. 16); guilt copy; hiding cancel |
| Upsell | Shows the relevant tier when a member hits a real limit (e.g. the 3 weekly intents) | Engagement paywalls (P8); upsell during a safety flow |
| Cross-sell | GHOST to members who use discretion features; DUO to linked couples | Selling safety features (core discretion and safety are free, D-05) |

---

## 13. Security, compliance and risk

### 13.1 Cybersecurity Command Centre

| Brief element | TRYST control | Status |
|---|---|---|
| Zero trust: never trust, always verify | Every request authenticated and authorised; mTLS + SPIFFE between services; staff via SSO + hardware key + device posture; no VPN trust | Spec (P1) |
| MFA | Members: two factors always (02 §3.3). Staff: FIDO2 hardware key mandatory | **Built** (members) |
| Biometric authentication | B1 device-bound biometric passkey + B2 liveness match on mobile | **Built** (B2 provider pluggable; fails closed) |
| Device fingerprinting | Device binding by passkey and DPoP key (not cross-site fingerprinting); hashed device risk signals for admission L1 | Spec |
| Risk-based authentication | Identity risk agent can **add** step-up; it can never remove a factor | Spec |
| Real-time monitoring | SIEM over OpenTelemetry + audit chains; SOC agent triage | Spec |
| Behavioural analytics | Login anomaly (impossible travel, new device + new network), admin behaviour baselines | Spec |
| AI anomaly detection | Unsupervised models on ops telemetry and admin actions (not on member content) | Spec |
| Transaction scoring | Fraud agent + Radar + acquirer scores | Spec |
| Behaviour scoring | Admission L1–L5; Guardian graph signals | Spec |
| Device intelligence | Platform attestation (App Attest, Play Integrity) for native builds | Spec (P3) |

### 13.2 Anti-hacking framework

| Threat | Control |
|---|---|
| DDoS | Cloudflare and AWS Shield; per-route rate limits; autoscaling with caps |
| SQL injection | Parameterised queries only (Go `database/sql` placeholders, pglast-validated migrations); least-privilege DB roles; SAST rule |
| XSS | React escaping; strict CSP with no inline scripts on app origins (`web/next.config.ts`); no third-party scripts; Trusted Types in P1 |
| CSRF | No cookie-authenticated mutating endpoints without SameSite=Strict and Origin checks; tokens are DPoP-bound bearer (not ambient) |
| Session hijacking | Sessions bound to the device (DPoP); short lifetimes; sign-out everywhere; emergency lock (built) |
| Account takeover | No passwords, no SMS or email login codes; passkey + second factor; no staff overrides (D-16) |
| Credential stuffing | Nothing to stuff: no passwords exist |
| API abuse | Scoped keys, idempotency, rate limits, anomaly detection on partner usage |
| Bot attacks | Bot management at the edge; V2 verification before any interaction (E-01) |
| Prompt injection and model abuse | §5.7 injection defence; output schemas; red team per release |
| Supply chain | Signed images, SLSA provenance, dependency pinning and scanning, reviewed PRs only (including agent-authored ones) |

### 13.3 Data protection

| Brief element | Control |
|---|---|
| Encryption at rest | AES-256 via KMS for every store; per-member keys for profile data; crypto-shredding for erasure (03 §6.3) |
| Encryption in transit | TLS 1.3 at the edge; mTLS inside |
| Encryption in use | E2EE threads (MLS); client-side media encryption (Rust core, built); on-device Guardian; Nitro Enclaves evaluated for the PSI server (P2) |
| Tokenisation | Card data tokenised by processors (TRYST is SAQ-A scope); contact handles stored as HMAC with an HSM pepper |

### 13.4 Compliance

| Area | How the OS meets it |
|---|---|
| UK GDPR / DPA 2018 (Art 9 special category) | Explicit consent, DPIA, self-hosted models for member data, minimisation, DSR automation, < 60 s erasure |
| UK GDPR Art 22 (automated decisions) | Always-human classes; explanations from reason codes; appeal path (FR-035) |
| Online Safety Act 2023 | Highly effective age assurance; risk assessments; T&S SLAs; transparency reporting generated from audit data |
| EU AI Act (P4 markets) | Agent register, logging, human oversight and transparency labels ("machine-generated" on Briefs, P4) are already in place |
| KYC / KYB | Members: age assurance, not identity KYC (pseudonymous, FR-002). Partners and licensees: KYB |
| AML | TRYST holds no customer funds; AML obligations sit with regulated processors; TRYST screens business partners (D-32) |
| PCI DSS | SAQ-A: hosted checkout only; no card data touches TRYST |
| Audit logs | Hash-chained agent and admin logs; WORM anchoring |
| Secure API keys | Shown once; Argon2id hashed; scopes; IP allow-lists; rotation; leaked-key scanning (GitHub secret scanning) |
| Webhook signing | HMAC with timestamp tolerance in and out; dedupe |
| Transaction monitoring | Payment and Fraud agents; chargeback thresholds |
| Admin action tracking | `ops.admin_action`, two-person break-glass |

### 13.5 AI risk register (additions to 01 §13.3)

| Risk | Likelihood | Impact | Mitigation | Owner |
|---|---|---|---|---|
| An agent takes a harmful action at scale | Medium | Severe | Gate, budgets, kill switch, canary rollout of agent versions | AI Governance |
| Model leaks special-category data | Low | Severe | Self-hosted models; Privacy Auditor leakage tests; no free-text member memory | DPO |
| Prompt injection via profile or ticket text | High | Medium | Injection defence and red team (§5.7) | Security |
| Auto-repair loop worsens an outage | Medium | High | Runbooks only; budget 50/day; rollback-first; pages a person if not resolved in 10 min | SRE |
| Over-reliance: staff rubber-stamp approvals | Medium | High | Approval sampling and second review for permanent bans; approval-rate monitoring per approver | Head of T&S |
| Connector sends member data to a third party | Low | Severe | Connector register checks; egress allow-list; DLP | Security |

---

## 14. Admin Super Control Centre

Internal origin, staff SSO + hardware key, redacted by default, every view and action logged (FR-086).

| Area | Visibility | Actions (and who may take them) |
|---|---|---|
| Users | Counts and cohorts by default; a specific member only through a case (JIT grant) | Hold (Guardian/moderator); permanent actions by moderator + second review |
| Businesses | Licensees and partners with KYB status, contracts, usage | Suspend keys (admin); terminate (two admins) |
| Transactions | Ledger explorer, payment events by processor | Refund (payments ops); failover (payments ops) |
| APIs | Traffic, errors, latency by route and key; deprecations | Throttle a key (admin); revoke (admin) |
| Payments | Authorisation rate, chargebacks, disputes, settlement by processor | Approve dispute responses; switch primary processor (two admins) |
| Agents | Registry, levels, budgets, decision stream, pending approvals, eval scores | Approve pending decisions (named approvers); kill (any on-call); resume (two people) |
| Automations | Workflow runs, failures, compensations | Retry, cancel (ops) |
| Disputes | Member appeals (FR-035) and payment disputes | Decide appeals (moderator not involved in the original decision) |
| Compliance | DSR clock, retention violations, OSA SLAs, regulator requests | Approve disclosures (DPO) |
| Revenue | Daily pack, forecasts, experiment results | Approve price experiments (pricing committee) |
| System health | SLOs, error budgets, incidents, deploys | Freeze deploys; approve promotion |
| Logs | Structured logs (scrubbed), traces | Read (engineers, JIT in production) |
| Alerts | Unified alert stream with dedupe and ownership | Acknowledge, assign, escalate |
| Platform performance | Web vitals, app crash-free rate, latency by journey | — |
| Audit | Chain verification status; last anchor; break-glass events | Export for auditors (DPO) |

**Break-glass:** a member-level view outside a case needs a justification and a second approver, expires in 8 hours (DDL CHECK), and is reported in the monthly access review.

---

## 15. Developer build roadmap

The OS phases align with TRYST's phases and gates (01 §12). The brief's five phases map as: **MVP = P1**, **Beta = P1 closed beta → P2**, **Commercial launch = P3**, **Enterprise version = P4**, **Global scale version = post-P4**.

### 15.1 MVP (P1, M2–M6)

| | |
|---|---|
| Modules | Accounts and two-factor sign-in (built in part), Loops A, B, D, E2EE threads, V2 gating, VC co-sign, Guardian baseline on device, DSR and erasure, payments (Stripe + backup, BitriPay sandbox), agent registry and gate, audit chain, Admin Super Control Centre v1, connector register |
| APIs | Member API (02 §5), Ops API (§11.2), inbound webhooks |
| User flows | J1 (basic), J3, J4, J5, J7, J8 |
| AI agents | Cartographer, Broker (P1 baseline ranker), Curtain, Guardian, Aftercare, Onboarding, Risk, Fraud, Support, Compliance, Payment, Herald, System Health, Bug Detection, Auto-Repair, Release Manager, Threat Hunter, Vulnerability, SOC triage, executive copilots |
| Technical milestones | PostgreSQL IDENTITY-DB; DPoP; real liveness provider; gate as a central service with WORM anchoring; SLOs live; erasure < 60 s p99 |
| Commercial objectives | G-P1: 2,000 verified in the London closed beta; no-go list clear |

### 15.2 Beta (P2, M6–M10)

| | |
|---|---|
| Modules | Envoy, Mirror v1, ExclusionRing PSI, Member Command Centre (insights, "why" panels, my memory), analytics |
| AI agents | + Envoy, Mirror, Chief of Staff (member), Data Intelligence, Predictive Growth, Infra Optimiser, product copilots |
| Technical milestones | Workflow-level evals; chaos game days; Temporal in production |
| Commercial objectives | **G-P2:** meets per intent +40% vs the P1 control. If not met, stop and rethink |

### 15.3 Commercial launch (P3, M10–M15)

| | |
|---|---|
| Modules | DUO and GHOST billing, native apps in both stores, 24/7 T&S, Couple Command Centre, pricing experiments |
| AI agents | + Pricing, Monetisation, Retention, AML (payments monitoring) |
| Technical milestones | App attestation; open-banking evaluation; second processor live-tested |
| Commercial objectives | **G-P3:** 25,000 verified; payer rate ≥ 11%; both stores approved |

### 15.4 Enterprise version (P4, M15–M24)

| | |
|---|---|
| Modules | Partner API, partner and developer portals, ACU metering, KYB, BitriPay connected accounts for partner commissions, regulator portal, EU localisation |
| AI agents | + B2B Sales, Licensee Success, KYB onboarding |
| Technical milestones | Cell architecture design; EU AI Act conformity file; SOC 2 Type II and ISO 27001 / 27701 |
| Commercial objectives | **G-P4:** 150,000 verified; contribution-positive; first two licensees signed |

### 15.5 Global scale version (post-P4)

| | |
|---|---|
| Modules | Multi-cell deployment (UK+IE, EU-North, EU-South, then CA/AU/NZ per 01 §14.5), localised assurance and Guardian, licensee self-serve onboarding |
| AI agents | Per-cell agent instances with a global governance plane |
| Technical milestones | Cell isolation tests; regional key domains; follow-the-sun T&S |
| Commercial objectives | Licensing revenue ≥ 20% of total; per-market contribution margin positive within 12 months of launch |

### 15.6 Production readiness review (every phase gate)

| Area | Must be true |
|---|---|
| Reliability | SLOs defined and met for 30 days in staging/beta; rollback tested; DR restore drill passed this quarter |
| Security | Pen test closed (no high or critical open); threat model updated; secrets rotated; break-glass tested |
| Privacy | DPIA updated; Privacy Auditor green; erasure and DSR timings within NFRs |
| AI | Every agent registered with owner, level, budget, eval pass and red-team pass; kill-all drill done |
| Payments | Webhook replay and dedupe tested per processor; chargeback alerting live; backup MID transaction tested |
| Compliance | OSA risk assessment current; transparency report pipeline tested; store policy check (P3+) |
| Operations | On-call rota; runbooks for top 20 alerts; support macros reviewed against claims discipline (01 §15.1) |

---

## 16. Competitive advantage

| Dimension | How the OS wins | Why it is hard to copy |
|---|---|---|
| More powerful | Agent-to-agent negotiation turns intent into meetings; agents carry safety, support and operations volume | Needs the consent schema, the asymmetric Brief design and member trust built together |
| More profitable | Revenue tied to outcomes (ENVOY, QCAM); lower ops cost per MAU; B2B licensing from P4 | Competitors' revenue depends on message volume and paywalls they cannot drop |
| More scalable | Event-driven services, cell architecture, agents that scale with load rather than headcount | — |
| More intelligent | Learns from outcomes (aftercare reward), not clicks; stated-preference floor keeps members in control | Outcome data only exists if the product produces meetings |
| More trustworthy | Two-factor account-holder-only sign-in, E2EE, crypto-erasure, governance as tested code, no data sale | Incumbents carry breach history and ad-funded or volume-funded models |
| More compliant | OSA, GDPR Art 9 and 22, and EU AI Act controls designed in | Retrofitting into an engagement-optimised stack is slow and expensive |
| More commercially competitive | Processor independence (Stripe, acquirer, BitriPay) protects revenue; licensing opens a second business | Category players rely on a single high-risk processor relationship |

---

## 17. Final output format and index

This document is the developer-ready OS specification for TRYST. The brief's required technical outputs map as follows:

| Required output | Where |
|---|---|
| Product Requirements Document | 01 Product + §1–§4, §6, §12 here |
| Technical Requirements Document | 02 (FR/NFR) + FR-080–FR-088 |
| System, infrastructure, AI, agent architecture | §5, §9.1–§9.3; 03 |
| Security architecture; complete security controls | §13; 03 §6 |
| Database architecture; complete ERD and schemas | §10; 02 §4; generated migrations |
| API architecture; complete API specifications | §11; 02 §5; `contracts/openapi/tryst.v1.yaml` |
| Event architecture | §9.4; 02 §8 |
| DevOps and deployment architecture | §9.9 |
| Monitoring architecture | §9.3 (observability, monitoring), §9.5 SLOs |
| Disaster recovery and business continuity | §9.7 |
| Data governance architecture | §9.6 |
| Compliance architecture | §13.4; 01 §14 |
| Scalability architecture | §9.8 |
| Commercial architecture | §12 |
| Complete user journeys | §3.2; 01 §6; 04 §4 |
| Complete agent workflows | §5.5, §5.6, §5.7 |
| Complete testing strategy | §9.10 |
| Production readiness review | §15.6 |
| Code that implements this document today | `backend/internal/aigov` (governance gate, workforce registry, audit chain), `backend/internal/payments/bitripay.go` (BitriPay adapter), plus the v1.2 identity, payments and agent code |
