---
title: "TRYST — Product Requirements & Business Plan"
subtitle: "Private chemistry. Intelligent discretion."
version: "0.1 (Draft for founding team review)"
date: "4 October 2026"
classification: "Confidential — internal"
---

# TRYST

**Private chemistry. Intelligent discretion.**

Product Requirements Document & Business Plan — v0.1 draft, 4 October 2026

Classification: Confidential — internal

---

## Contents

1. Executive summary
2. Market, audience and positioning
3. Relationship modes in scope
4. Product principles and policy boundaries
5. User journeys
6. AI agents and behavioural-learning architecture
7. Multi-person compatibility engine
8. Discretion and privacy controls
9. Functional requirements
10. System architecture, APIs and data model
11. Security
12. Trust, safety and moderation
13. Legal, regulatory and compliance
14. Business model and pricing
15. Unit economics
16. Go-to-market
17. MVP backlog
18. Delivery roadmap and launch gates
19. Competitor research
20. Risks, open questions and appendices

---

## 1. Executive summary

TRYST is a discreet, verified, AI-assisted platform for adults seeking intimate connection outside a conventional monogamous frame: one-off encounters, ongoing extramarital relationships, threesomes, and couple-to-couple connections. It is built for all genders and orientations and treats singles, individuals in relationships, and couples as first-class participants.

The category is proven but under-served. Incumbents either carry legacy trust damage (most notably the 2015 Ashley Madison breach), skew heavily male with poor experiences for women, or — in the case of newer ethical-non-monogamy apps — are designed around openly negotiated relationships and are less suited to members whose primary need is discretion. TRYST's thesis is that the winning product in this category is defined by three things competitors have not combined:

1. **Discretion as an engineering discipline, not a marketing claim.** Disguised app shells, panic exit, decoy PIN, ephemeral media, data minimisation, end-to-end encrypted messaging, and a breach-resilient architecture in which a full database compromise yields as little identifying data as possible.
2. **Verification and safety by default.** Every member is a liveness-verified adult. Couple profiles require each partner in the couple to verify and consent individually. Bots, scammers and sex-work solicitation are actively removed.
3. **Intelligent matching for one-to-one and multi-person configurations.** A compatibility engine that models individuals, couples and groups — including the asymmetric preferences that make threesome and couple-swap matching hard — supported by AI agents that learn from behaviour while respecting strict privacy boundaries.

**Scope note on consent.** TRYST does not require members to obtain approval from a spouse or partner who is *not* on the platform. Members are adults responsible for their own relationships. Consent requirements apply to everyone who *is* a participant: every person in a couple profile must individually verify and opt in, and every person in a group connection must individually accept it. TRYST will never permit one person to represent, photograph or arrange encounters on behalf of another person who has not consented on-platform.

**Business model.** Freemium with subscription tiers (individual and couple), consumable credits, and a high-touch Concierge tier. Base-case targets: 18-month path to 250k verified members across launch markets, blended paying conversion of 9–12%, and contribution-margin positive by month 15 (see §15).

**MVP.** Verified onboarding, discreet shell, one-to-one and couple matching, encrypted chat, ephemeral media, core safety tooling, subscriptions and credits — launching first in the UK, then selected EU markets, then the US (state-by-state age-assurance compliant). Group (3+) matching ships in Phase 2.

---

## 2. Market, audience and positioning

### 2.1 Market context

- **Non-monogamy is mainstreaming.** Survey work over the past decade (e.g., YouGov polling in the US and UK, Kinsey Institute research) consistently finds a significant minority of adults report interest in, or experience of, consensual non-monogamy, and a large share of adults in long-term relationships report some form of infidelity at some point. Precise figures vary widely by methodology; we treat them as directional, not as TAM inputs.
- **Category revenue is real.** Ruby Life (Ashley Madison) has publicly reported tens of millions of dollars in annual revenue and claims tens of millions of cumulative sign-ups; Feeld has reported rapid growth driven by younger, queer and ENM audiences. Gleeden, Illicit Encounters and Victoria Milan monetise regional audiences.
- **Mainstream apps are moving adjacent.** Mainstream dating apps have added group/"double date" and non-monogamy-friendly profile fields, which normalises the behaviour but does not deliver discretion.

> **Action:** commission a sized TAM/SAM study (paid panel, n≥3,000 per launch market) before Series A materials. Figures in this document are planning assumptions.

### 2.2 Target segments

| Segment | Core need | Primary pain with current options |
|---|---|---|
| **Discreet individuals (all genders)** — partnered, seeking one-off or ongoing connection | Secrecy, low friction, real people | Fake profiles, breach fear, app icon visibility, payment traces |
| **Women and non-binary members** seeking connection on their own terms | Safety, control, quality over volume | Unsolicited images, aggressive messaging, unverified men |
| **Couples** seeking a third, or another couple | Joint profile that both partners control; group logistics | Single-account couple profiles (one partner acting alone), "unicorn hunting" stigma, poor group chat |
| **Singles** open to partnered people | Clear expectations, no drama | Mismatched intent on mainstream apps |
| **LGBTQ+ members** across all of the above | Inclusive identity model, discretion around outness | Binary gender fields, outing risk |

### 2.3 Positioning statement

For adults who want intimate connection beyond monogamy and value privacy above all, **TRYST** is the verified, discreet connection platform that pairs genuinely compatible people, couples and groups — unlike legacy affair sites, TRYST is engineered so your secret stays yours, and every person you meet is a real, verified adult who has chosen to be there.

### 2.4 Brand pillars

- **Private** — discretion is the default, never an upsell.
- **Real** — verified humans only; zero tolerance for bots and scams.
- **Intelligent** — matching that understands nuance and configurations beyond pairs.
- **Respectful** — clear intent, consent-forward, non-judgemental.

---

## 3. Relationship modes in scope

Every profile declares one or more **intents**. Intent drives matching eligibility, onboarding copy and safety rules.

| Mode | Description | Participants | MVP? |
|---|---|---|---|
| **One-off** | A single encounter; no expectation of continuation | 1:1 | Yes |
| **Ongoing** | A recurring affair or extramarital relationship | 1:1 | Yes |
| **Threesome (seeking a third)** | A couple seeking one person | Couple + 1 | Yes (couple-to-single) |
| **Threesome (joining)** | An individual seeking a couple | 1 + Couple | Yes |
| **Couple-to-couple** | Two couples connecting (social, swap, full) | Couple + Couple | Yes |
| **Group (3+ individuals)** | Ad-hoc groups not anchored on a couple | 3–4 individuals | Phase 2 |
| **Open / exploring** | Undecided; wants to explore | Any | Yes |

**Relationship-status field** (private by default, visible only to matches when the member chooses): single, partnered (discreet), partnered (open/ENM), married (discreet), married (open/ENM), separated, it's complicated. "Discreet" vs "open" status is a matching signal — e.g., some members only want others with equal discretion stakes.

**Identity model.** Gender is free-form with a curated list (woman, man, non-binary, trans woman, trans man, genderqueer, agender, two-spirit, and self-describe), plus separate fields for "show me to" and "interested in" so members can, e.g., be visible only to women and couples. Orientation and kinks are optional, private-by-default, and stored as special-category data (§13).

---

## 4. Product principles and policy boundaries

### 4.1 Principles

1. **Discretion is free.** All core privacy features ship on the free tier. We monetise reach, convenience and premium experience, not safety.
2. **Every participant consents on-platform.** No one is represented without their own verified account and explicit acceptance.
3. **Minimum data, maximum control.** Collect only what powers matching and safety; give members deletion that actually deletes.
4. **The member owns the pace.** Women and other members who receive disproportionate inbound volume get strong filtering and rate controls.
5. **AI serves the member, never surveils them.** Behavioural learning operates on in-app signals the member can inspect and reset.
6. **No judgement in tone; no compromise on safety.**

### 4.2 Hard boundaries (prohibited — enforced in ToS, moderation and product)

- Anyone under 18 (or the local age of majority if higher) — zero tolerance, immediate ban and reporting where required.
- Commercial sex, escorting, "sugar" arrangements involving payment for sex, or any solicitation of money — banned to mitigate trafficking risk and to comply with FOSTA-SESTA (US) and equivalent laws.
- Non-consensual intimate imagery, sharing others' images, "revenge" content, recording without consent.
- Profiles representing a person who has not verified (e.g., one partner operating a "couple" profile alone).
- Use of the platform to locate, monitor or expose another person, including a member's own spouse (anti-stalking, anti-outing).
- Harassment, hate speech, threats, extortion/sextortion.
- Unsolicited explicit media (gated by consent controls — §9.5).

### 4.3 Explicit non-goals

- TRYST is not a "catch a cheater" service and will never offer partner-search, reverse lookup or exposure features.
- TRYST is not an adult-content marketplace.
- TRYST does not provide relationship counselling, though it signposts support resources.

---

## 5. User journeys

### 5.1 Individual onboarding (target: < 6 minutes to first match card)

1. Install discreet app (shell name/icon chosen at first launch — e.g., "Notes+", "Calc", "Weather").
2. Create account with email **or** anonymous handle + passkey; phone number optional (used only for 2FA, stored hashed).
3. Age assurance + liveness selfie (via vetted provider; selfie template discarded after verification unless the member opts into "verified photo match" badge).
4. Intent, configurations, identity, preferences (≤ 12 taps).
5. Photos: upload with face-blur defaults on; "reveal on mutual match" toggled by default.
6. Discretion setup: lock PIN, decoy PIN, notification disguise, panic gesture.
7. First curated matches from the Matchmaker agent.

### 5.2 Couple onboarding

1. Partner A creates a **couple shell** and generates a one-time invite link/QR.
2. Partner B creates their own account and verifies individually (age + liveness).
3. Both partners independently confirm the couple link and agree the **Couple Charter** (shared boundaries: what's on/off the table, who can message, whether both must approve matches).
4. The couple profile becomes active only after both confirmations. Either partner can **dissolve** the link at any time; dissolution freezes the couple profile, notifies matches neutrally and preserves each partner's individual account.
5. Couple settings: *joint approval* (both must like before a match is created) or *delegated approval* (either partner can match within Charter limits).

### 5.3 Matching to meeting

Discover → like / super-like → mutual match → encrypted chat (ephemeral by default) → optional photo reveal → optional voice/video call (no number exchange) → **Plan** (date-safety check-in, safe-word to trusted contact, venue suggestions) → post-meet private feedback (feeds learning; never shown to the other party verbatim).

### 5.4 Group connection (couple + single / couple + couple)

Mutual interest across *all* participants is required. A **Group Room** is created only after every individual has accepted. Each individual can leave at any time, which closes the room for that person and notifies others neutrally. Group rooms support shared boundaries checklists, a poll-based scheduler, and per-person media permissions.

---

## 6. AI agents and behavioural-learning architecture

TRYST uses a set of narrowly scoped agents orchestrated by a policy layer. Agents act *for the member* and are subject to the same privacy guarantees as the rest of the platform.

### 6.1 Agent roster

| Agent | Purpose | Inputs | Outputs | Guardrails |
|---|---|---|---|---|
| **Matchmaker** | Curates a daily slate of high-compatibility candidates | Profile, preferences, behavioural embeddings, compatibility scores | Ranked slate with plain-language "why" reasons | Never uses message content; explanations avoid sensitive inference |
| **Profile Coach** | Helps write a compelling, safe bio and choose photos | Draft bio, photo set | Suggestions; flags identifying details (workplace, street signs, kids, wedding ring) | Runs on-device where possible; suggestions only, never auto-publish |
| **Conversation Copilot** (opt-in) | Icebreakers, tone suggestions, boundary-setting phrasing | Local thread context (on-device) | Suggestions | On-device model; no server-side message reading; disabled in E2EE rooms unless member enables locally |
| **Discretion Guardian** | Proactive privacy hygiene | Device/app settings, upload metadata, location settings | Nudges: strip EXIF, blur faces, enable decoy PIN, warn on screenshots | Cannot be disabled for EXIF stripping |
| **Safety Sentinel** | Detects scams, bots, coercion, minors, solicitation, harassment | Account signals, reports, server-visible metadata, client-side classifier verdicts | Risk scores, friction, escalation to human review | Human review before bans except for high-confidence CSAM/age hashes; appealable |
| **Group Coordinator** | Logistics for 3+ person connections | Group availability, Charter overlaps | Shared boundaries summary, schedule polls | Only acts after all members accept |
| **Concierge Assistant** | Supports human concierges for premium members | Member brief (explicit), concierge notes | Shortlists, outreach drafts | Human-in-the-loop; member approves every introduction |

### 6.2 Behavioural-learning architecture

**Signals (implicit):** dwell time on cards, profile-detail expansions, like/pass, reply latency, conversation depth (message counts, not content), match-to-meet conversion, post-meet ratings, block/report events, time-of-day activity.

**Signals (explicit):** stated preferences, dealbreakers, Charter, "more like this / less like this" feedback, post-meet private rating.

**Learning layers:**

1. **Two-tower retrieval model** — member tower (stated prefs + behavioural embedding) and candidate tower (profile features + engagement quality). Retrieves top-K from the eligible pool after hard filters.
2. **Re-ranker** — gradient-boosted model predicting *mutual* outcome (P(A likes B) × P(B likes A) × P(conversation ≥ N turns) × P(meet)), not just one-sided attraction. This counters the "attractiveness collapse" where a few profiles absorb all attention.
3. **Fairness and exposure controls** — exposure caps per profile per day, freshness boosts for new verified members, and demographic parity monitoring across gender/orientation cohorts to prevent systemic invisibility.
4. **Stated-vs-revealed reconciliation** — when behaviour persistently diverges from stated preferences, the member is *asked* ("You've been liking couples — want to add couples to your preferences?") rather than silently overridden. Members can view and reset their learned profile in **My Signals**.
5. **Bandit exploration** — a small exploration budget (≈10% of slate) using Thompson sampling to avoid filter bubbles and to cold-start new members.

**Privacy constraints on learning:**

- Message content is **never** used for server-side training. E2EE makes this structurally impossible for 1:1 chat.
- Sensitive attributes (orientation, kinks, relationship status) are used as *matching filters chosen by the member*, not as learned targets for inference.
- Training data is pseudonymised; member IDs are rotated in the feature store; models are retrained on rolling 90-day windows; deleted members are purged from the next training cycle and their features dropped from serving immediately.
- Differential-privacy noise on aggregate analytics; k-anonymity threshold (k ≥ 50) on any cohort reporting.
- LLM components use providers under zero-data-retention terms, or self-hosted open-weight models; no member PII in prompts beyond what is strictly necessary.

### 6.3 Evaluation

Offline: NDCG@10 on mutual-like, calibration of mutual-outcome predictions, exposure Gini. Online: mutual-match rate, conversations ≥10 messages, meet-confirmation rate, 7/30-day retention, report rate per 1k matches, women's inbound-unwanted-message rate. Every model release passes a safety and fairness review gate.

---

## 7. Multi-person compatibility engine

### 7.1 Entities

- **Person** — a single verified individual.
- **Unit** — the matching entity: either a Person or a **Couple** (exactly two linked Persons).
- **Configuration** — the target shape a Unit is seeking: `1:1`, `couple+1`, `1+couple`, `couple+couple`, `group(n≤4)`.

### 7.2 Pairwise compatibility

For persons *i* and *j*: `C(i,j) = H(i,j) · S(i,j)` where

- `H(i,j) ∈ {0,1}` — hard constraints (age range, gender/orientation mutual interest, distance, intent overlap, dealbreakers, block lists, discretion requirements). Must be satisfied **in both directions**.
- `S(i,j) ∈ [0,1]` — soft score: learned mutual-outcome prediction blended with explicit preference similarity (schedules, discretion level, experience level, interests, chemistry indicators).

### 7.3 Unit-level compatibility

For a couple *U = {a, b}* and a single *x* (threesome):

- **Every pairing that the configuration requires must pass hard constraints.** The Couple Charter declares which interactions are in scope (e.g., "both partners with the third", or "only partner a with the third"). Only required edges are evaluated.
- **Score aggregation uses a soft-minimum**, not an average, so one strong partner cannot mask a poor fit with the other:
  `S(U,x) = −τ · log Σ_e exp(−S_e / τ)` over required edges *e*, with temperature τ tuned (≈0.1).
- **Couple coherence bonus** — rewards candidates whom *both* partners have independently engaged with similar profiles.

For couple-to-couple *U1 × U2*, required edges are defined by the intersection of both Charters (e.g., "swap only" vs "full"), and the same soft-min applies across up to four cross-edges.

### 7.4 Group (Phase 2) formation

Group formation for 3–4 individuals is framed as **constrained clique search** on the compatibility graph: find sets where all required edges pass hard constraints and the soft-min score exceeds threshold θ. Candidate generation uses seed expansion from a member's top pairwise matches, bounded to keep latency < 300 ms. Groups are *proposed*; every individual must accept.

### 7.5 Asymmetry and demand balancing

The single-to-couple market is famously imbalanced (many couples seek a third woman; far fewer single women seek couples). The engine:

- Applies exposure caps so in-demand singles are not flooded.
- Offers singles **inbound gating** (e.g., "show me only to couples with joint approval and verified both partners").
- Surfaces couples to single men and couple-to-couple matches transparently, instead of hiding the imbalance.
- Uses credit pricing (§14) partially to ration high-demand inbound.

### 7.6 Cold start

New members receive a calibrated slate from stated preferences, a freshness boost (days 0–7), and exploration slots. Couples are cold-started using the union of each partner's onboarding signals.

---

## 8. Discretion and privacy controls

### 8.1 Device-level

| Control | Detail | Tier |
|---|---|---|
| **Disguised app shell** | Alternate name and icon (iOS alternate icons; Android activity-alias). Note: app-store listing and purchase history still show "TRYST" — disclosed clearly at onboarding | Free |
| **App lock** | Biometric/PIN required on every open; configurable timeout | Free |
| **Decoy PIN** | Opens a plausible empty "notes" or "budget" view | Free |
| **Panic exit** | Shake / triple-tap / back-tap → instantly switch to decoy and clear recents thumbnail | Free |
| **Notification disguise** | Neutral text ("You have 1 new update") and neutral icon; or no notifications | Free |
| **Screenshot protection** | Android FLAG_SECURE; iOS screen-capture detection with blur + sender alert | Free |
| **Web access** | Private-browsing prompt, no persistent cookies beyond session, quick-exit button | Free |

### 8.2 Identity and content

- **Photo privacy:** face-blur/veil by default; per-match reveal; revoke reveal at any time (images served via short-lived signed URLs, so revocation is effective for future views).
- **Ephemeral media:** view-once, timed, watermarked with recipient's handle (discourages leaks; aids takedown).
- **Location fuzzing:** distance shown in bands (e.g., "< 5 km"); location stored at coarse geohash precision server-side; travel mode for planned trips.
- **Incognito/Ghost mode:** visible only to profiles you've liked.
- **Hide from contacts:** optional private set intersection (PSI) against hashed contact list so that people in your address book are mutually hidden — the server never learns the contacts. Opt-in, on-device hashing.
- **Hide from neighbourhood/workplace:** exclusion zones (e.g., 2 km around a chosen point).

### 8.3 Payments discretion

- Neutral billing descriptor (e.g., "TRY*DIGITAL SVCS") — must still be accurate and recognisable to the cardholder to satisfy card-network rules and avoid chargebacks; agreed with acquirer.
- App-store billing shows the app name; members are told this explicitly. Web checkout offers alternative descriptors, prepaid cards, and (where permitted) privacy-preserving wallets.
- Receipts sent only to a member-chosen email; option for no email receipts.

### 8.4 Account lifecycle

- **Pause** (hidden instantly), **Delete** (hard-delete pipeline completes within 30 days, backups expire within 35), **Self-destruct timer** (auto-delete after N days of inactivity, member-configurable).
- **Data export** (GDPR Art. 15/20) delivered encrypted to the member.

### 8.5 Honest limits

TRYST must be explicit about what discretion cannot cover: app-store purchase history, shared Apple/Google family accounts, device backups, network-level monitoring, and the conduct of other members. The onboarding "Discretion Check" walks members through these.

---

## 9. Functional requirements

Priority: **P0** = MVP launch blocker, **P1** = MVP-desirable / fast follow, **P2** = later.

### 9.1 Accounts & onboarding

| ID | Requirement | Pri |
|---|---|---|
| ACC-01 | Sign up with email or anonymous handle + passkey; optional phone for 2FA | P0 |
| ACC-02 | Age assurance + liveness verification before any discovery | P0 |
| ACC-03 | Disguised shell selection at first launch | P0 |
| ACC-04 | Couple linking with independent verification and confirmation by both partners | P0 |
| ACC-05 | Couple Charter creation and joint/delegated approval modes | P0 |
| ACC-06 | Couple dissolution by either partner, preserving individual accounts | P0 |
| ACC-07 | Pause, delete, self-destruct timer, export | P0 |
| ACC-08 | Verified-photo-match badge (opt-in selfie re-match) | P1 |

### 9.2 Profiles

| ID | Requirement | Pri |
|---|---|---|
| PRO-01 | Intents, configurations, identity, interested-in, show-me-to | P0 |
| PRO-02 | Photo upload with automatic EXIF strip and face-blur default | P0 |
| PRO-03 | Per-match photo reveal / revoke | P0 |
| PRO-04 | Private profile sections (kinks, boundaries) visible only after match | P1 |
| PRO-05 | Profile Coach agent with identifying-detail detection | P1 |

### 9.3 Discovery & matching

| ID | Requirement | Pri |
|---|---|---|
| MAT-01 | Hard-filter eligibility (bidirectional) | P0 |
| MAT-02 | Daily curated slate + browse grid | P0 |
| MAT-03 | Like, pass, super-like; mutual match creation | P0 |
| MAT-04 | Couple joint-approval gating | P0 |
| MAT-05 | Couple+1 and couple+couple matching | P0 |
| MAT-06 | "Why this match" explanations | P1 |
| MAT-07 | My Signals: view/reset learned preferences | P1 |
| MAT-08 | Group (3–4 individuals) proposals | P2 |
| MAT-09 | Travel mode | P1 |

### 9.4 Messaging & calls

| ID | Requirement | Pri |
|---|---|---|
| MSG-01 | E2EE 1:1 messaging (MLS or Signal protocol) | P0 |
| MSG-02 | E2EE group rooms (MLS) for couple+1 / couple+couple | P0 |
| MSG-03 | Ephemeral messages default (24h), configurable | P0 |
| MSG-04 | View-once / timed media with recipient watermark | P0 |
| MSG-05 | In-app voice/video calls (no number exchange), E2EE | P1 |
| MSG-06 | Message requests gating (who can message first) | P0 |
| MSG-07 | Conversation Copilot (on-device, opt-in) | P2 |

### 9.5 Consent & safety in-product

| ID | Requirement | Pri |
|---|---|---|
| SAF-01 | Explicit-media consent toggle per conversation; unconsented explicit images auto-blurred via on-device classifier | P0 |
| SAF-02 | Report, block, unmatch from every surface; report includes optional evidence the reporter chooses to share (decrypts locally, re-uploads to T&S) | P0 |
| SAF-03 | Date Safety: share plan with trusted contact, timed check-in, emergency shortcut | P1 |
| SAF-04 | Scam/off-platform-payment warnings triggered by client-side heuristics | P0 |
| SAF-05 | Rate limits on outbound likes/messages for new accounts | P0 |
| SAF-06 | Support resources (sexual health, IPV, mental health) by locale | P0 |

### 9.6 Monetisation

| ID | Requirement | Pri |
|---|---|---|
| PAY-01 | Subscriptions (individual / couple) via app stores and web | P0 |
| PAY-02 | Credits wallet: boosts, super-likes, priority messages, reveals of "who liked you" | P0 |
| PAY-03 | Concierge application, intake, and billing | P1 |
| PAY-04 | Discreet descriptors and receipt controls | P0 |
| PAY-05 | Refunds/chargeback handling; regional pricing | P0 |

### 9.7 Non-functional requirements

| Area | Target |
|---|---|
| Availability | 99.9% monthly (P0 services) |
| Latency | Slate fetch p95 < 400 ms; message send p95 < 250 ms |
| Scale (MVP) | 500k registered, 50k DAU, 5M messages/day |
| Accessibility | WCAG 2.2 AA |
| Localisation | EN-GB, EN-US, FR, DE, ES, NL, IT at launch of respective markets |
| Platforms | iOS 17+, Android 10+, responsive web (PWA) |
| Data residency | EU/UK member data in EU/UK regions |

---

## 10. System architecture, APIs and data model

### 10.1 Architecture overview

- **Clients:** iOS (Swift/SwiftUI), Android (Kotlin/Compose), Web (Next.js PWA). Shared crypto core in Rust (MLS via OpenMLS) compiled to each platform.
- **Edge:** CDN + WAF + bot management; API gateway with mTLS to services; per-device attestation (App Attest / Play Integrity).
- **Core services (containerised, Kubernetes):** Identity, Profile, Verification adapter, Couple/Group, Discovery, Matching, Messaging relay (stores only ciphertext), Media (encrypted object storage, signed URLs), Payments, Trust & Safety, Notifications, Agents orchestrator.
- **Data:** PostgreSQL (primary OLTP, row-level encryption for sensitive columns), Redis (sessions, rate limits), vector store for embeddings, Kafka for events, object storage for encrypted media, analytics warehouse (pseudonymised only).
- **ML:** feature store (pseudonymised IDs), offline training pipeline, online inference service for retrieval/re-ranking; model registry with approval gates.
- **Secrets & keys:** cloud KMS + HSM-backed envelope encryption; per-member data keys for sensitive fields (crypto-shredding on delete).

### 10.2 Key APIs (REST/JSON over HTTPS; WebSocket for realtime)

| Method | Endpoint | Description |
|---|---|---|
| POST | `/v1/auth/register` | Create account (email or handle + passkey) |
| POST | `/v1/auth/passkey/assert` | Passkey login |
| POST | `/v1/verification/sessions` | Start age + liveness session with provider; returns client token |
| POST | `/v1/verification/webhook` | Provider callback (signed); sets `verified_at`, discards biometric payload |
| GET/PATCH | `/v1/me/profile` | Read/update own profile |
| POST | `/v1/media/upload-url` | Get pre-signed upload URL; server strips EXIF, generates blurred variant |
| POST | `/v1/couples` | Create couple shell; returns invite token |
| POST | `/v1/couples/{id}/accept` | Partner accepts link (requires verified account) |
| PUT | `/v1/couples/{id}/charter` | Update Couple Charter (requires both partners' confirmation for material changes) |
| DELETE | `/v1/couples/{id}` | Dissolve couple (either partner) |
| GET | `/v1/discovery/slate` | Today's curated slate with reasons |
| POST | `/v1/likes` | Like / super-like / pass a Unit |
| GET | `/v1/matches` | List matches and group rooms |
| POST | `/v1/matches/{id}/reveal` | Grant photo reveal; `DELETE` to revoke |
| POST | `/v1/groups/proposals/{id}/respond` | Accept/decline group proposal |
| WS | `/v1/realtime` | Ciphertext message relay, typing, presence (presence optional/off by default) |
| POST | `/v1/reports` | Submit report with optional member-shared evidence |
| POST | `/v1/billing/checkout` | Web checkout session |
| POST | `/v1/billing/store-receipts` | Validate App Store / Play receipts |
| GET | `/v1/me/signals` | View learned preference summary; `DELETE` resets |
| POST | `/v1/me/export` | Request data export |
| DELETE | `/v1/me` | Delete account (crypto-shred) |

All write endpoints are idempotent via `Idempotency-Key`; all responses avoid leaking existence of other accounts (uniform errors).

### 10.3 Core data model (simplified)

```
person(id, handle, email_hash, email_enc, passkey_creds, verified_at, verification_level,
       dob_year_enc, gender, pronouns, created_at, status, data_key_id, region)
profile(person_id, bio_enc, intents[], relationship_status_enc, interested_in[],
        show_me_to[], orientation_enc, discretion_level, geo_cell, travel_geo_cell,
        visibility_mode)
couple(id, person_a, person_b, status{pending,active,dissolved}, approval_mode,
       created_at, activated_at)
charter(couple_id, version, allowed_configs[], allowed_edges[], boundaries_enc,
        confirmed_by_a_at, confirmed_by_b_at)
unit(id, kind{person,couple}, ref_id)
preference(unit_id, age_min, age_max, distance_band, configs[], dealbreakers_enc)
photo(id, person_id, storage_key, blurred_key, is_primary, moderation_state)
like(id, from_unit, to_unit, kind{like,super,pass}, actor_person_id, created_at)
match(id, unit_a, unit_b, created_at, status)
group_room(id, config, created_at, status)
group_member(room_id, person_id, accepted_at, left_at)
reveal(match_id, granter_person_id, grantee_unit_id, granted_at, revoked_at)
conversation(id, match_or_room_id, mls_group_id)          -- ciphertext only in relay store
report(id, reporter_person_id, subject_person_id, category, evidence_ref_enc, state)
enforcement(id, person_id, action, reason_code, reviewer_id, appeal_state)
subscription(id, unit_id, payer_person_id, tier, store, status, renews_at)
credit_ledger(id, person_id, delta, reason, balance_after, created_at)
signal_event(pseudo_id, event_type, target_pseudo_id, ts)  -- feature store; rotating IDs
```

`*_enc` fields use per-member envelope encryption; deleting the member's data key renders them unrecoverable (crypto-shredding), including in backups.

---

## 11. Security

**Threat model priorities:** (1) mass data breach and extortion (the Ashley Madison scenario), (2) targeted de-anonymisation of a member by a spouse, colleague or stalker, (3) account takeover, (4) scraping and bot farms, (5) insider abuse, (6) sextortion by malicious members.

**Controls:**

- **Data minimisation:** no real names required; DOB stored as verified-adult flag + birth year only; precise location never stored; biometric data not retained after verification.
- **Encryption:** TLS 1.3 everywhere; E2EE for messages, calls and group rooms; field-level envelope encryption for sensitive attributes; encrypted media with per-object keys.
- **Breach resilience:** a database dump must not yield readable sensitive fields, message content, or a list of real identities. Email is stored as a salted hash for lookup plus encrypted copy for contact; payment data held only by PCI-DSS Level 1 processor (tokens only).
- **Authentication:** passkeys first; TOTP/WebAuthn 2FA; device binding; anomaly detection on new devices; session revocation UI.
- **Anti-scraping:** device attestation, rate limits, behavioural bot detection, no enumerable IDs, blurred-by-default photos, watermarking.
- **Insider controls:** least-privilege, just-in-time access with approval, all support-tool access logged and reviewed; support staff see redacted views; no staff access to message content (technically impossible under E2EE).
- **Secure SDLC:** threat modelling per feature, SAST/DAST, dependency scanning, signed builds, mobile app hardening, annual third-party pen test plus pre-launch test, public bug bounty from launch.
- **Certifications:** SOC 2 Type II within 12 months of launch; ISO 27001 within 24 months.
- **Incident response:** 24/7 on-call; GDPR 72-hour regulator notification playbook; pre-drafted member communications; extortion-response plan (law enforcement liaison, no-payment policy, member support).

---

## 12. Trust, safety and moderation

### 12.1 Layered model

1. **Prevention:** verification gate, device attestation, new-account rate limits, explicit-media consent gating, Discretion Guardian nudges.
2. **Detection:** Safety Sentinel scores (bot/scam/minor/solicitation/harassment), hash-matching for known CSAM (PhotoDNA / NCMEC hash lists, IWF in UK) and known NCII (StopNCII hashes), image classifiers for nudity/minors/violence on profile photos (profile photos are not E2EE and are moderated server-side), text classifiers on profile bios.
3. **Member reporting:** one tap from any surface; reporter may share decrypted evidence voluntarily — the only way T&S ever sees message content.
4. **Human review:** 24/7 follow-the-sun team; specialist queue for minors/trafficking/sextortion; reviewer wellness programme.
5. **Enforcement:** warn → restrict → suspend → ban; device and verification-based ban evasion detection; transparent appeals.
6. **Transparency:** semi-annual transparency report (DSA-aligned).

### 12.2 Service levels

| Category | Target time to action |
|---|---|
| Suspected minor / CSAM | < 1 hour; mandatory reporting (NCMEC in US, relevant authorities elsewhere) |
| Credible threat of violence / sextortion | < 2 hours |
| Solicitation / scam | < 12 hours |
| Harassment / other | < 24 hours |
| Appeals | < 7 days |

### 12.3 Specific risk playbooks

- **Sextortion:** in-chat warnings when off-platform migration or payment requests are detected client-side; rapid account takedown; StopNCII integration; member support guide.
- **Outing/exposure threats:** treat threats to expose a member to their spouse/employer as extortion — immediate action.
- **Coercion in couples:** either partner can privately signal concern to T&S; dissolution is always available unilaterally; group rooms allow private leave.
- **Romance/financial scams:** off-platform payment detection, crypto-scam pattern library, warning interstitials.

### 12.4 Staffing (launch)

~1 moderator per 15–20k MAU blended with automation, scaling down as classifiers mature; Head of Trust & Safety hired pre-MVP.

---

## 13. Legal, regulatory and compliance

> This section summarises the landscape as understood at drafting (October 2026). It is not legal advice; external counsel must validate each launch market.

### 13.1 Data protection

- **GDPR / UK GDPR:** data about sex life and sexual orientation is *special category* (Art. 9) — processing requires explicit consent; DPIA mandatory; DPO appointment; data residency and transfer mechanisms (SCCs/IDTA, EU–US Data Privacy Framework). Regulators have fined dating platforms for special-category handling (e.g., Norway's Datatilsynet fine against Grindr, upheld on appeal).
- **US state privacy:** CCPA/CPRA (sensitive personal information; "limit use" right), plus state comprehensive privacy laws in many states (Virginia, Colorado, Connecticut, Texas, Oregon, etc.) treating sex life/orientation as sensitive data requiring opt-in. **Washington's My Health My Data Act** and similar consumer-health-data laws may capture sexual-health information.
- **Biometrics:** Illinois BIPA (private right of action), Texas CUBI, Washington biometric law — verification vendor must support written consent, retention schedules and deletion; TRYST discards biometric templates after verification by default.

### 13.2 Online safety and age assurance

- **UK Online Safety Act 2023:** user-to-user service duties (illegal content risk assessments, children's access assessment). Ofcom's age-assurance requirements for services allowing pornographic/harmful content have been enforced since July 2025 — "highly effective age assurance" is required. TRYST's verification gate is designed to meet this standard.
- **EU Digital Services Act:** notice-and-action, statements of reasons, transparency reporting, trusted flaggers; VLOP obligations would not apply at launch scale. Monitor EU-level age-verification initiatives (Commission age-verification blueprint/app).
- **US:** a growing number of states require age verification for sites with substantial adult content; the US Supreme Court upheld Texas's law (*Free Speech Coalition v. Paxton*, June 2025). TRYST does not host pornography, but universal age verification puts it ahead of these requirements regardless. Also monitor state app-store age-verification laws (e.g., Utah, Texas) affecting distribution.
- **Australia:** eSafety Commissioner codes and the online dating industry code; age-assurance developments. Phase 3+ market.

### 13.3 Trafficking and solicitation

- **FOSTA-SESTA (US):** removes Section 230 protection for platforms that knowingly facilitate prostitution/sex trafficking. TRYST's ban on commercial sex and payment solicitation, detection tooling and law-enforcement cooperation process are essential.
- UK Modern Slavery Act statement once thresholds are met; equivalent EU national laws.

### 13.4 Adultery and orientation laws — geo-restriction

- Adultery remains a criminal offence in a number of jurisdictions (including several Middle Eastern, South and South-East Asian countries — e.g., Indonesia's revised Criminal Code, in force from January 2026, criminalises sex outside marriage on complaint), and same-sex relations are criminalised in dozens of countries. A few US states still have rarely enforced adultery statutes (New York repealed its statute in 2024).
- **Policy:** TRYST will not operate in, market to, or permit registration from jurisdictions where adultery or same-sex relations carry criminal penalties, and will apply geo-blocking plus app-store territory restrictions. Members travelling into such jurisdictions see a warning and profiles are auto-hidden.

### 13.5 Consumer, payments and platform rules

- Auto-renewal laws (California ARL, FTC "click-to-cancel" considerations, EU consumer rights withdrawal rules) — easy cancellation, clear renewal disclosure.
- Card network rules on dating services and billing descriptors; acquirer underwriting for "high-risk" dating MCC (7273); chargeback ratio < 0.65%.
- Apple App Store and Google Play guidelines: dating apps permitted; no sexually explicit content in-app; robust UGC moderation, reporting and blocking required; 17+/18+ rating; in-app purchase rules for digital goods (with regional exceptions for web purchases, e.g., post-*Epic v. Apple* US anti-steering changes and EU DMA).
- Advertising restrictions: Meta/Google restrict dating and adult-oriented ads; affair-oriented messaging is frequently refused — informs GTM (§16).

### 13.6 Compliance deliverables before launch

DPIA; ROPA; Online Safety Act illegal-harms and children's-access risk assessments; age-assurance conformity evidence; ToS, Privacy Notice, Community Guidelines, Cookie policy; law-enforcement request policy (with warrant requirement and member notification where lawful); CSAM reporting registrations; vendor DPAs; PCI scope attestation.

---

## 14. Business model and pricing

### 14.1 Revenue streams

1. **Subscriptions** (primary): individual and couple tiers, monthly/quarterly/annual.
2. **Credits** (consumables): boosts, super-likes, priority messages, "who liked you" reveals, travel-mode activation, group-room boosts.
3. **Concierge** (premium high-touch): curated introductions by trained human matchmakers assisted by the Concierge agent.
4. **Later:** partner perks (discreet hotels, venues) on a referral basis — no member data shared with partners.

### 14.2 Tiers (UK pricing; USD/EUR set via purchasing-power adjustment)

| Tier | Monthly | Quarterly (per mo) | Annual (per mo) | Key inclusions |
|---|---|---|---|---|
| **Free** | £0 | — | — | Verification, full discretion suite, limited daily likes (15), match & chat with mutuals, ephemeral media, safety tools |
| **Plus** | £24.99 | £18.99 | £12.99 | Unlimited likes, see who liked you, advanced filters, travel mode, 5 super-likes/week, incognito |
| **Elite** | £49.99 | £37.99 | £27.99 | Plus + priority placement, read receipts control, monthly boost, verified-photo badge priority, profile review by Profile Coach Pro |
| **Couple Plus** | £34.99 | £26.99 | £19.99 | Plus for a linked couple (covers both partners), group rooms, joint-approval tools, couple-to-couple filters |
| **Concierge** | £399 / 3 months | — | — | Dedicated matchmaker, curated introductions (target 3–6 per quarter), date planning support; includes Elite |

**Credits:** packs of 10 (£9.99), 30 (£24.99), 75 (£49.99). Indicative costs: boost 30 min = 5 credits, super-like = 2, priority message (lands at top of a message-request queue) = 3, group-room boost = 6.

**Women and non-binary members:** Plus features free for verified women and non-binary members during the first 12 months in each launch market, to balance the marketplace (common category practice). Reviewed for legality under local equality law (UK Equality Act — counsel review required; alternative is a neutral "inbound-balance" programme offered to any under-represented cohort).

### 14.3 Pricing principles

- Safety and discretion are never paywalled.
- No dark patterns: clear renewal terms, one-tap cancellation, pro-rated refunds where required.
- Web checkout incentivised (lower price) where store rules permit, improving margin and payment discretion.

---

## 15. Unit economics (base case, UK launch market)

### 15.1 Key assumptions

| Metric | Assumption | Notes |
|---|---|---|
| Blended paying conversion (of MAU) | 10% | Category benchmarks typically 5–15% |
| Paying mix | Plus 60% / Elite 15% / Couple Plus 22% / Concierge 3% | |
| Billing mix | 45% monthly / 30% quarterly / 25% annual | |
| Blended ARPPU (net of VAT) | ~£22 / month | Before platform fees |
| Credit spend | +£3.50 per paying member / month; +£0.40 per non-payer | |
| Platform/payment fees | 22% blended | Store 15–30%, web ~4% incl. high-risk processing |
| Paid-member monthly churn | 14% | Category typically 10–20% |
| Blended CAC (paid member) | £95 | Mix of performance, influencer, PR, referral |
| Verification cost | £0.60 per verified sign-up | Vendor quote range £0.30–£1.20 |
| Infra + AI cost | £0.18 per MAU / month | Incl. inference, media, E2EE relay |
| T&S cost | £0.22 per MAU / month | Human review + tooling |

### 15.2 Per paying member

- Monthly net revenue: (£22 + £3.50) × (1 − 0.22) ≈ **£19.90**
- Variable cost (infra, T&S, support allocated to payer + share of free-user cost at 9:1 ratio): ≈ **£4.40**
- Monthly contribution: ≈ **£15.50**
- Expected lifetime (1 / 0.14): ≈ **7.1 months**
- **LTV (contribution):** ≈ **£110**
- **LTV:CAC:** ≈ **1.16×** at launch — *insufficient*; target ≥ 3× by month 18 via:
  - Reduce churn to ≤ 10% (lifetime 10 months → LTV ≈ £155) through better matching and couple tiers (couples churn less).
  - Lift annual plan mix to 35%.
  - Reduce CAC to ≤ £50 via referral, PR and organic/SEO as brand matures.
  - At churn 10% and CAC £50: LTV:CAC ≈ **3.1×**.

### 15.3 Plan-level targets (base case)

| | M6 | M12 | M18 | M24 |
|---|---|---|---|---|
| Verified members (cumulative) | 40k | 130k | 250k | 450k |
| MAU | 22k | 65k | 120k | 210k |
| Paying members | 1.8k | 6.5k | 13k | 24k |
| MRR (net) | £36k | £129k | £259k | £478k |
| Monthly contribution after CAC | negative | negative | ~breakeven (M15–M18) | positive |

**Seed requirement (indicative):** £3.5–4.5M to reach M18 including a 22-person team, security certifications, T&S operations and marketing. Sensitivity analysis and a full three-statement model to accompany this document.

---

## 16. Go-to-market

### 16.1 Market sequencing

1. **UK (London, Manchester, Birmingham)** — high category awareness, English-language, clear regulatory regime (OSA, UK GDPR), dense cities.
2. **EU wave 1 (Netherlands, Germany, France, Spain)** — strong ENM and swinger cultures; localised brand; EU data residency.
3. **US (state-sequenced)** — start with large states with favourable regulatory posture; age-verification compliance by state; BIPA-ready verification.
4. **Later:** Australia, Canada, selected LATAM markets — subject to legal review; never in criminalising jurisdictions (§13.4).

### 16.2 Liquidity strategy

- **City-by-city launch** with waitlist and invite codes to concentrate density; open a city only when ≥ 3k verified members within 25 km and gender/configuration ratios meet thresholds.
- **Supply-side seeding:** women, non-binary members and couples prioritised from waitlist; free Plus for these cohorts (subject to counsel review).
- **No fake profiles, ever** — a public pledge, contrasting with category history.

### 16.3 Channels

- **PR & narrative:** "the affair app built by security engineers" — data-privacy-first story; transparency report; third-party security audit published.
- **Content & SEO:** relationships, non-monogamy, discretion guides; anonymous member stories.
- **Creators & podcasts:** ENM, sex-positive and relationship creators where platform policies allow.
- **Out-of-home:** tasteful, provocative OOH in launch cities (subject to ASA/CAP code compliance).
- **Performance:** limited due to ad-platform restrictions; use networks that accept dating, with careful creative; app-store optimisation.
- **Referral:** credits for both parties when an invitee verifies; couples invite couples.
- **Partnerships:** ENM community events, lifestyle venues (no data sharing).

### 16.4 Launch messaging pillars

"Your secret stays yours." · "Every profile is a verified adult." · "Chemistry for two, three or four." · "No judgement. No fakes. No traces."

---

## 17. MVP backlog

**MVP definition:** a verified, discreet 1:1 and couple-based matching product with E2EE chat, safety core and monetisation, launched in London.

| Epic | Stories (abridged) | Pri | Est. (pt) |
|---|---|---|---|
| E1 Identity & onboarding | Account creation; passkeys; 2FA; verification integration; disguised shell; discretion setup | P0 | 55 |
| E2 Couples | Shell/invite; dual verification; Charter; approval modes; dissolution | P0 | 40 |
| E3 Profiles & media | Profile CRUD; EXIF strip; blur variants; reveal/revoke; moderation pipeline | P0 | 45 |
| E4 Discovery & matching v1 | Hard filters; heuristic + light ML ranker; slate; likes; mutual match; couple+1; couple+couple | P0 | 70 |
| E5 Messaging | MLS integration; 1:1 and group rooms; ephemeral; view-once media; watermarks; message requests | P0 | 80 |
| E6 Trust & safety | Reporting; blocking; moderation console; CSAM/NCII hashing; classifiers; enforcement; appeals | P0 | 65 |
| E7 Discretion suite | App lock; decoy PIN; panic exit; notification disguise; screenshot protection; location fuzzing; exclusion zones | P0 | 40 |
| E8 Payments | Store subscriptions; web checkout; credits ledger; descriptors; refunds | P0 | 45 |
| E9 Account lifecycle | Pause; delete/crypto-shred; self-destruct; export | P0 | 20 |
| E10 Platform & security | Infra-as-code; KMS; observability; rate limits; attestation; pen test fixes | P0 | 60 |
| E11 Analytics | Pseudonymised event pipeline; core dashboards; DP aggregates | P0 | 20 |
| E12 Agents v1 | Profile Coach (identifying-detail detection); Discretion Guardian; "why this match" | P1 | 30 |
| E13 Date Safety | Trusted-contact check-ins | P1 | 15 |
| E14 Calls | E2EE voice/video | P1 | 30 |

**Total MVP (P0):** ≈ 540 points ≈ 24–26 weeks for the team in §18.3.

---

## 18. Delivery roadmap and launch gates

### 18.1 Phases

| Phase | Window | Scope |
|---|---|---|
| **0 — Foundations** | Weeks 0–8 | Team hires (Head of T&S, Security Lead), legal entity & counsel, DPIA, vendor selection (verification, hashing, payments), architecture, design system, brand |
| **1 — MVP build** | Weeks 6–30 | Epics E1–E11; internal dogfood at week 22 |
| **2 — Closed beta (London)** | Weeks 30–38 | 2–5k invited members; pen test; bug bounty; Ofcom/ICO-facing documentation finalised |
| **3 — Public launch UK** | Week ~40 | Subscriptions live; PR launch; Concierge pilot |
| **4 — Group & intelligence** | Months 10–15 | Group (3–4) matching; Conversation Copilot; My Signals; calls; EU wave 1 |
| **5 — Scale** | Months 15–24 | US state-sequenced launch; SOC 2 Type II; Concierge scale-up; partner perks |

### 18.2 Launch gates (all must pass)

**Gate A — Closed beta entry**
- Verification live with false-accept rate for minors < 0.1% in vendor testing; manual review fallback.
- E2EE verified by independent cryptographic review.
- CSAM/NCII hash matching operational; NCMEC/appropriate authority reporting process tested.
- DPIA signed off; Privacy Notice and ToS approved by counsel.
- Discretion suite complete; data-breach tabletop exercise completed.

**Gate B — Public launch**
- External pen test: zero critical/high findings open.
- Report-to-action SLAs met for 4 consecutive weeks in beta.
- Bot/fake rate < 1% of active profiles (sampled audit).
- Women's unwanted-inbound rate below target threshold set in beta.
- Payments: acquirer approval, descriptor confirmed, chargeback flow tested.
- OSA risk assessments filed internally; age-assurance evidence pack complete.
- Liquidity: ≥ 3k verified members in London with configuration-ratio thresholds met.

**Gate C — New market entry**
- Local counsel review (adultery/orientation laws, privacy, age assurance, consumer law).
- Localisation and local-language T&S coverage.
- Data residency confirmed.

### 18.3 Team (MVP)

Founders (CEO, CPO), CTO, Security Lead, Head of T&S; 3 iOS/Android, 1 web, 4 backend, 1 crypto/platform, 2 ML/data, 2 product design, 1 QA/automation, 4 moderators (scaling), 1 marketing lead, 1 community/partnerships, part-time DPO and fractional GC. ≈ 22 FTE equivalents.

---

## 19. Competitor research (as of October 2026)

> Competitive data is drawn from public sources and company statements; figures should be refreshed before external use.

| Competitor | Positioning | Strengths | Weaknesses / openings for TRYST |
|---|---|---|---|
| **Ashley Madison** (Ruby Life, Canada) | The best-known affair site; global | Brand awareness; scale; women-free model; credits economy | 2015 breach and fake-profile ("fembot") history and subsequent FTC settlement; dated UX; weak verification perception |
| **Gleeden** (France) | Affair site "designed by women" | Strong in FR/IT/ES; women free | Limited couple/multi-person features; Europe-centric; mixed verification |
| **Illicit Encounters** (UK) | UK affair site | UK market focus; long tenure | Legacy web UX; limited AI/matching sophistication |
| **Victoria Milan** (Norway origin) | Affair app with panic button | Early discretion features (panic button, blurred photos) | Fake-profile complaints; limited innovation |
| **Feeld** (UK) | Open-minded/ENM dating for individuals and couples | Strong brand with younger, queer and ENM users; linked couple profiles; rapid growth | Positioned around openly consensual non-monogamy; discretion not the core promise; verification optional |
| **3Fun / #Open / Kasual** | Threesome and ENM-focused apps | Niche focus on couples and thirds | Reported fake profiles and moderation issues; minimal discretion engineering |
| **SDC / Kasidie / swinger communities** | Lifestyle/swinger communities | Couple-to-couple focus, events | Dated; web-centric; older demographic |
| **Mainstream apps** (Tinder, Hinge, Bumble, Grindr) | Mass-market dating | Liquidity; mainstream adoption of group/"double date" and ENM-friendly fields | Not discreet; social-graph exposure; partnered users unwelcome on some |

**Differentiation summary:** TRYST is the only offering planned to combine (1) mandatory verification of every participant, including each partner in a couple; (2) E2EE across 1:1 and group connections; (3) breach-resilient, crypto-shredding data design; (4) a multi-person compatibility engine; and (5) privacy-preserving behavioural AI with member-visible controls.

**Lessons from category history:**
- The Ashley Madison breach (2015) exposed paid "full delete" that did not delete, and led to extortion campaigns and regulatory action (FTC, Canadian and Australian privacy commissioners). → TRYST: free, real deletion; crypto-shredding; minimal identity.
- Fake-profile accusations have repeatedly damaged incumbents. → TRYST: no-fakes pledge, verification for all, published audits.
- Grindr's regulatory penalties over sharing sensitive data with ad partners. → TRYST: no ad-tech SDKs, no sale or sharing of data, no third-party advertising in-app.

---

## 20. Risks, open questions and appendices

### 20.1 Key risks and mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Data breach / extortion | Medium | Severe | §11 controls; crypto-shredding; E2EE; minimal identity; IR plan; cyber insurance |
| App-store removal or rejection | Medium | High | Strict content rules; strong moderation; PWA fallback; proactive store relations |
| Payments de-risking by acquirer | Medium | High | Two acquirers; low chargeback ratio; transparent descriptors |
| Marketplace imbalance (gender, couples vs singles) | High | High | Supply-side seeding; exposure caps; inbound gating; city density gates |
| Reputational / press backlash ("app for cheaters") | High | Medium | Lead with privacy, safety and inclusivity narrative; no shaming; responsible messaging |
| Regulatory change (age assurance, privacy) | High | Medium | Over-comply now; compliance roadmap; counsel on retainer |
| Misuse for trafficking/solicitation | Medium | Severe | Ban + detection + LE cooperation; FOSTA-SESTA compliance |
| AI bias / unfair exposure | Medium | Medium | Fairness monitoring; exposure caps; release gates |
| Verification friction hurts conversion | High | Medium | Fast vendor flow; explain "why"; verification as brand promise |

### 20.2 Open questions

1. Legal viability of free Plus for women/non-binary members under UK and EU equality law vs a neutral balancing programme.
2. Build vs buy for MLS group messaging and E2EE calls (OpenMLS + LiveKit vs vendor).
3. Whether to allow *unverified browsing* of blurred profiles pre-verification (conversion vs safety).
4. Concierge staffing model: employed vs vetted contractors; background checks.
5. Self-hosted LLM vs zero-retention API for Profile Coach and Concierge agent.
6. Brand naming clearance: trademark search for "TRYST" in UK/EU/US classes 9, 42, 45; app-store name availability.

### 20.3 Glossary

- **Unit:** matching entity (person or couple).
- **Couple Charter:** couple's shared, mutually confirmed boundaries and approval rules.
- **E2EE / MLS:** end-to-end encryption / Messaging Layer Security (RFC 9420).
- **Crypto-shredding:** deleting the encryption key so encrypted data becomes unrecoverable.
- **PSI:** private set intersection — comparing contact lists without revealing them.
- **ENM:** ethical non-monogamy.
- **OSA:** UK Online Safety Act 2023. **DSA:** EU Digital Services Act.

### 20.4 Document control

| Version | Date | Author | Change |
|---|---|---|---|
| 0.1 | 4 Oct 2026 | Founding team | Initial draft for review |

*Next steps:* founding-team review → counsel review of §13 and §14.2 → refreshed competitor and TAM data → financial model → v0.2.
