# 03 — TRYST Backend Specification v1.1

Part of the [TRYST v1.1 baseline](00_README.md). Implements the contracts in [02](02_Shared_Contracts.md). Product intent is in [01](01_Product.md).

---

## 1. Scope

This document covers server-side services, agents, ML, data stores, cryptography (server side), trust and safety tooling, commerce, compliance machinery and operations. Everything a client can see or call is defined in 02. This document does not redefine those contracts.

---

## 2. Service topology and stack

### 2.1 Stack

*Source: A §12*

| Layer | Choice | Rationale |
|---|---|---|
| Edge API | **Go**, gRPC-gateway (REST/JSON out, gRPC in) | Concurrency, small attack surface, fast cold start |
| Agents | **Python**, FastAPI, typed state machines (LangGraph-style) | Deterministic orchestration; every agent turn can be replayed for evals |
| LLM, profile-touching | **Self-hosted open-weight models, UK region, vLLM** | Article 9 data must not leave the region or reach a third-party API. Non-negotiable |
| LLM, generic copy | Hosted frontier model, PII redacted at the boundary | Only where no personal data is involved |
| Primary stores | PostgreSQL 16 + pgvector — **identity, profile, safety and ban clusters isolated** | Identity split enforced in the infrastructure |
| Vector search | Qdrant | Filtered ANN after hard-gate pre-filtering |
| Events | Redpanda + Schema Registry (Protobuf) | Kafka API, lower ops burden |
| Analytics | ClickHouse | Behavioural aggregation; raw events purged at 180 d |
| Feature store | Feast | Train/serve parity (M3/M4) |
| Objects | S3-compatible; **client-side encrypted** | The server never holds plaintext media |
| Keys | AWS KMS + CloudHSM | Destroying the root key is the erasure primitive; peppers held in the HSM |
| E2EE | MLS (RFC 9420) via OpenMLS (delivery service + key packages) | Standardised; group threads for couples |
| Age assurance | Two providers behind one interface | Ofcom expects layered protection and due diligence; one provider is a single point of failure |
| Region | AWS eu-west-2 (London), UK/EU residency only | |
| IaC / CI / CD | Terraform, GitHub Actions, Argo CD | Auditable change control |

### 2.2 Deployables

Source A requires each agent to be independently deployable. Core domain services start as a **modular monolith** with hard module boundaries, to fit a 2-person backend team, and are split when load or team size justifies it ([D-09](06_Decisions_and_Changes.md#2-decision-log)).

| Deployable | Language | Owns | Store |
|---|---|---|---|
| `edge-gateway` | Go | AuthN (OAuth2 + DPoP), jurisdiction gate, tier gate, consent gate, rate limits, idempotency | Redis |
| `identity-svc` | Go | Identity, devices, **authenticators and login/step-up/recovery flows (§6.7)**, assurance adapters, anchor candidates, ban-anchor check, join-key HMAC, root keys | IDENTITY-DB, BAN-DB, KMS/HSM |
| `core-svc` (modular monolith) | Go | Modules: `profile`, `couple`, `intent`, `discretion`, `discovery-api`, `reveal`, `threads` (MLS delivery service), `meet`, `consent`, `deletion`, `commerce` | PROFILE-DB |
| `agent-cartographer`, `agent-mirror`, `agent-broker`, `agent-envoy`, `agent-curtain`, `agent-guardian`, `agent-aftercare` | Python | One agent each (§3) | Own schemas / Feast / Qdrant |
| `llm-gateway` | Python | vLLM pool, prompt/policy registry, PII redaction, injection filters | — |
| `media-svc` | Go | Pre-signed upload/download, ciphertext lifecycle, public-tier moderation, perturbation | Object store |
| `safety-svc` | Go | Report intake, SLA routing, case management, enforcement, appeals, transparency data | SAFETY-DB |
| `trust-console` | Web (internal) | Moderator UI; redacted by default; break-glass flows | via `safety-svc` |
| `jobs` | Go | TTL purges, deletion orchestration, renewal notices, aftercare scheduling, Privacy Auditor probes | — |

**Bus.** Agents communicate over typed Protobuf messages on Redpanda topics (`agent.*`), never with free text [A §5]. Synchronous calls between agents use gRPC with mTLS and SPIFFE identities.

### 2.3 Request path (slate)

`edge-gateway` (DPoP, tier ≥ V2, jurisdiction, consent) → `core-svc/discovery-api` → `agent-broker`:
- Stage 0 gates
- Qdrant retrieval
- M3/M4 ranking
- constraints

→ `agent-curtain` exposure check → re-authorisation against the current Discretion Policy and block graph → response. Cached slates are **re-authorised on read** [B §8.2].

---

## 3. Agent services

Every agent has a policy file (`policies/<agent>.yaml`), a tool allow-list, a memory scope, a 400-case golden eval set, and an injection red-team suite (§9). [A §5, §13]

| Agent | Inputs | Writes | Tools | Hard constraints |
|---|---|---|---|---|
| **Cartographer** | Intake turns | Proposed IntentVector (unconfirmed) | `llm-gateway` (self-hosted), tag vocabulary | Structured output only; the member confirms each field (FR-003); re-runs quarterly or when Mirror reports drift |
| **Mirror** | Behaviour events, aftercare | RevealedPreferenceVector (**sole writer**), α, divergence | Feast, training pipeline | Message content excluded; personalisation off → α frozen at 0.15 and no updates; reset supported (FR-024) |
| **Broker** | Effective vectors, gates, fairness state | Slates, `slate_item` | Qdrant, M2/M3/M4/M9 | Stage 0 gates are not scored and cannot be bypassed; paid labels only within the eligible set; no gender-based uplift |
| **Envoy** | Both IntentVectors (incl. private fields), Discretion Policies, tiers | Handshake transcript, two Briefs; ENVOY-tier proposals | Rules engine + small model for friction ranking; `llm-gateway` only to render Brief copy from codes | Six dimensions; ≤ 6 turns; no free text; private fields never emitted; never messages a human; cannot open a thread (FR-015, FR-055) |
| **Curtain** | Session context, geofence hits, device risk, discretion events | Policy tightening, suppressions, nudges | M8, zone matcher | May only **tighten** policy; broadening needs a member action (02 §6.4) |
| **Guardian** | On-device scores, profile/media uploads, reports, graph | Signals, freezes, holds, case escalation | M6a–e, M7, M7b, hash lists | Veto over all agents; no permanent action without a human; never reads message content server-side (§7) |
| **Aftercare** | Meet events | Aftercare records → reward | Scheduler | Never visible to the counterparty; never aggregated into a public rating |

### 3.1 Envoy negotiation algorithm

1. **Trigger:** on mutual intent (P2+), `core-svc/intent` publishes `agent.handshake.requested`.
2. **Load both sides** inside Envoy's memory scope. Private fields stay inside it.
3. **Score each dimension deterministically:**
   - *intent:* shape compatibility matrix.
   - *structure:* segment pair, veto mode, partner participation.
   - *boundaries:* set intersection of hard limits and desire tags; any `no` × `yes` collision is **blocking**.
   - *logistics:* overlap of availability windows (tz-aware), intersection of travel radii, notice, hosting. Windows come **only** from what the member entered, never from inferred routines (FR-067).
   - *discretion:* zone intersection (must be empty), reveal pace, device risk.
   - *verification:* both sides' `min_counterparty_tier` satisfied.
4. **Up to 6 turns** in schema form to settle open queries (02 §6.1). Unknown fields → `friction`.
5. **Emit asymmetric Briefs** (02 §6.2) built from codes; `llm-gateway` renders only localised copy for codes, with no counterparty free text.
6. **Encrypt** the transcript and both Briefs (30 d TTL). Publish `agent.handshake.completed`.
7. **Guardian** may veto at any step (`blocking: ["safety_hold"]`, no detail disclosed).

**ENVOY tier:** Envoy generates `envoy_proposal` items (candidate + rationale codes). An intent exists **only** after `POST /v1/envoy/proposals/{id}/approve` ([D-04](06_Decisions_and_Changes.md#2-decision-log)).

---

## 4. Behaviour learning (Mirror)

### 4.1 Vectors

*Source: A §6.2*

```
IntentVector              stated    dim 128   source Cartographer  updated quarterly / on confirm
RevealedPreferenceVector  revealed  dim 256   source Mirror        nightly batch + hourly delta
DivergenceScore = 1 − cosine(project(IntentVector), RevealedPreferenceVector)
EffectiveVector = α · Revealed + (1 − α) · project(Intent)
α = σ(w · [interaction_count, days_active, divergence_stability, aftercare_count]),  α ∈ [0.15, 0.85]
```

- Divergence is used as a signal, not corrected away.
- The 0.15 floor on stated preference is a **product rule** (P5) and is enforced as a constraint, not left to the optimiser.

### 4.2 Privacy constraints

- **Inputs:** only events in 02 §8. **No message content.** Conversation features are computed on device from metadata.
- **Pseudonyms:** pseudo-subject IDs rotate every training window. Deleted members are dropped from serving immediately and from the next training run.
- **Raw events:** kept 180 d, then aggregated features only. Cohort reporting needs k ≥ 50.
- **Decay:** behavioural signals are time-weighted (default half-life 90 days) so old behaviour fades (FR-068).
- **Emergency lock** (FR-064) is handled by `identity-svc`: it verifies the emergency-code HMAC, revokes all sessions and authenticators' sessions, sets the profile to `vanished` and destroys all active reveal-grant wrapped keys.
- **Excluded from training:** protected or sensitive characteristics are never targets or inferred labels. Orientation, kinks and structure are used only as filters or features the member chose.

### 4.3 Model inventory

*Source: A §6.3, §19.3, §21*

| ID | Model | Type | Serving | Target |
|---|---|---|---|---|
| M1 | Intent classifier | Multi-label transformer, 9 classes | Batch + on edit | Macro F1 > 0.82 |
| M2 | Two-tower retrieval | Dual encoder, 256-d, ANN | < 40 ms p95 | Recall@200 > 0.91 |
| M3 | Cross-encoder ranker | GBDT + neural blend | < 120 ms p95 / 200 candidates | Mutual-reply AUC > 0.78 |
| M4 | Meet propensity | Survival (time-to-meet) | Nightly | C-index > 0.70 |
| M5 | No-show / flake risk | GBDT on on-device conversation features | Pre-meet | Precision@20% > 0.55 |
| M6a–e | Guardian behavioural family | §7 | **On device** (+ server graph) | §7.6 |
| M7 | Authenticity (bot/catfish) | Ensemble: image, text, device, graph | Sign-up + continuous | Catch rate > 0.93 |
| M7b | Ban-evasion graph | Style similarity, device/network adjacency | Server | Human review before any anchor extension |
| M8 | Discretion risk | Contextual bandit | Per session | Reduction in Burn invocations |
| M9 | Exposure fairness monitor | Gini + constraint solver | Hourly | Inbound Gini < 0.55 |

### 4.4 Reward

*Source: A §6.4*

```
R = 0.10·mutual_reply + 0.15·sustained_thread(≥6 turns, both sides) + 0.20·logistics_agreed
  + 0.35·meet_confirmed + 0.25·aftercare_positive
  − 0.40·report − 0.20·block − 0.15·no_show − 0.10·ghost_after_intent
  − 0.05·session_time_beyond_8min
```

Time in app is a **cost**. A model that improves AUC but lowers ΔR or the meet rate is rejected (§9).

### 4.5 Cold start and exploration

- **Turns 1–20:** content-based from the IntentVector; α = 0.15.
- **Exploration:** a permanent 15% of every slate, chosen by Thompson sampling.
- **New-profile boost:** 72 h decaying multiplier, capped by the inbound limiter.
- **Couples:** a blended vector from both partners plus a learned `joint_veto_model`. The true preference is the **intersection** of the two partners', not the union.

---

## 5. Matching and ranking (Broker)

### 5.1 Pipeline

*Source: A §7*

```
STAGE 0  HARD GATES (non-scored, non-bypassable, bidirectional)
  ├─ verification tier ≥ V2 and ≥ each side's min_counterparty_tier
  ├─ hard-limit intersection: zero collisions
  ├─ structure compatibility matrix (02 §2.3) and couple VC validity
  ├─ ExclusionRing (PSI + heuristics)
  ├─ geofence exclusion zones (both sides)
  ├─ block graph / report holds / safety_state
  ├─ jurisdiction gate
  └─ visibility (incognito: only if caller already received their intent)
STAGE 1  RETRIEVAL (M2)          ~50k → 200     < 40 ms
STAGE 2  RANKING (M3 + M4)       200 → 40       < 120 ms
STAGE 3  CONSTRAINTS             exposure fairness, inbound caps, diversity, paid labels
STAGE 4  SLATE                   ≤ 18 cards incl. 15% exploration, 2–4 explanation codes each
```

### 5.2 Launch baseline (P1, before M2/M3 are trained)

Source B's transparent weighted score is the **P1 ranker** and later the fallback:

```
score = 0.24·reciprocal_attraction + 0.20·intent_fit + 0.15·discretion_fit
      + 0.12·communication_fit + 0.10·availability_logistics + 0.08·reliability
      + 0.06·novelty + 0.05·model_confidence − risk_penalties
```

### 5.3 Multi-person scoring (THIRD/QUAD)

Edge scores are computed for every required pairing. The entity score is a **max-min** (soft-min with τ ≈ 0.1 for differentiability) over required edges, so one person's strong interest cannot mask another participant's poor fit [B §6.2]. Each couple's veto rules (02 §9) and the solo's veto apply on top.

### 5.4 Exposure fairness

1. **Inbound cap:** 12 intents per 24 h by default (member-adjustable 4–40). Overflow is queued and re-scored, never delivered in a burst, and the cap is never revealed to the sender.
2. **Gini constraint:** M9 runs hourly per city and segment. If Gini > 0.55, the ranker's fairness penalty rises until it recovers. Fairness is a constraint, not the objective.
3. **Asymmetric outreach cost:** couple→solo costs 3× the Keys (15 vs 5), fully refunded on reply.

S4 inbound above the cap is a release-blocking regression (§9).

### 5.5 Hard-limit inviolability

Stage 0 is implemented as a pure function with **property-based tests**: for random entities and constraint sets, no output may violate a gate. These tests run on every PR touching `broker/` or `contracts/`. A failing case is a P0 incident, even in staging [A §6.2].

---

## 6. Security, cryptography and discretion (server side)

### 6.1 Threat model

*Source: A §9.1*

| ID | Adversary | Likelihood | Primary server-side controls |
|---|---|---|---|
| A1 | Partner with physical access to the device | Very high | Device revocation, Burn endpoint, session TTLs, masked push payloads (no content in APNs/FCM) |
| A2 | Partner reading a shared bank statement | Very high | Neutral truthful descriptor, voucher path, 7-day pre-renewal notice |
| A3 | Recognition through the shared social graph | High | ExclusionRing PSI + heuristics, progressive reveal, perturbation, geofence |
| A4 | Mass data breach | Medium / existential | E2EE, client-side media encryption, identity/profile split, cryptographic erasure, short TTLs |
| A5 | Targeted extortion by a counterparty | Medium | Watermarked media, screenshot signals, M6e freeze, evidence preservation |
| A6 | Civil discovery in divorce proceedings | Medium | Minimal retention, erasure, published legal-process policy, transparency report |
| A7 | Insider | Medium | Zero standing access, dual-authorised break-glass, WORM audit, no content access by design |
| A8 | Face-search de-anonymisation | Medium, rising | Adversarial perturbation on public-tier photos; private photos never served unencrypted |

### 6.2 Cryptographic design

*Source: A §9.2*

- **Messaging:** MLS (RFC 9420). The server is the Delivery Service and Authentication Service: it stores key packages and ciphertext only. It **cannot** read content.
- **Media:** the client generates a per-media AES-256-GCM key; ciphertext goes to the object store; the key is wrapped per recipient and delivered in band. **Revoke = destroy the wrapped key.**
- **Identity split:** IDENTITY-DB and PROFILE-DB use separate credentials and KMS domains. The join key is `HMAC(identity_id, pepper_join)`, computed in `identity-svc` from an HSM-held pepper. Neither dump alone identifies anyone.
- **Peppers** (`contact`, `join`, `device`, `ban`) are HSM-resident and non-exportable, with yearly rotation and dual-run.
- **Field encryption:** sensitive PROFILE-DB columns (`bio_ct`, `policy_ct`, zones, transcripts, Briefs) use envelope encryption under the **member's data key**, which is derived from the member's root key in KMS.

### 6.3 Erasure (< 60 s p99)

`DELETE /v1/account` runs `deletion_job`:
1. Mark the profile deleted and remove it from Qdrant and slates **synchronously**.
2. Revoke all devices and tokens.
3. **Destroy the root key** (KMS schedule-deletion with the minimum window, plus immediate disable; the HSM-wrapped data key is zeroised).
4. Delete rows and object keys (best effort; they are already undecryptable).
5. Drop features from Feast online and mark the pseudo-subject for training exclusion.
6. If no ban applies, delete `anchor_candidate`. Revoke all authenticators and request deletion of the provider-held B2 login reference; the provider's deletion receipt is attached to the proof (it may arrive after the 60 s on-platform erasure).
7. Write a signed `completion_proof` (key-destruction receipt + purge summary).

**Exceptions** (02 §7): ban anchors, legal hold, Panic preservation. Each is written to the job record.

### 6.4 ExclusionRing PSI (server half)

*Source: A §9.3*

- OPRF over ristretto255 (RFC 9497). The server key `k` lives in the HSM.
- Round 1: the server evaluates blinded points.
- Round 2: the client uploads unblinded PRF outputs. The server intersects them with the member-identifier PRF set (each member's own contact hash, PRF-evaluated at enrolment) and writes **bidirectional** `exclusion_ring` rows.
- The server **returns nothing** about the intersection, and stores no uploaded PRF outputs beyond the intersection computation.
- Rate-limit enrolment (contacts ≤ 5,000 per call; ≤ 4 calls per day) to block membership-oracle attacks.
- **Heuristic suppression** (suppress only, never notify): shared payment fingerprint, sustained shared home geohash, device-graph adjacency, shared Wi-Fi BSSID history (only where the client opts in).

### 6.5 Authorisation (ABAC)

Every read evaluates `(requester, resource, reveal_grant, block graph, ExclusionRing, discretion version, safety_state, jurisdiction, tier)` in a central policy engine (OPA/Cedar) shared by all `core-svc` modules [B §9]. Denials are indistinguishable from not-found (02 §5.10). Policy tests are generated per read path; the Privacy Auditor runs them continuously against staging and production canaries.

### 6.6 Engineering non-negotiables

*Source: A §12*

- No third-party analytics, advertising or attribution SDK (server or client); NFR-09.
- Zero standing production data access; break-glass needs dual authorisation and writes `staff_access_log` (WORM); NFR-10.
- Staff cannot read message content; this is an architectural property.
- Quarterly external pen test; annual red team scoped to A4/A7. OWASP ASVS L3.
- 72-hour breach playbook, rehearsed twice a year, with specific runbooks for outing, sextortion, intimate-media leak, insider abuse, account takeover and mass breach [B §9].

---

### 6.7 Authentication — account holder only (v1.2)

Implements [02 §3.3](02_Shared_Contracts.md#33-login-assurance--account-holder-only) (FR-056–FR-062, [D-16](06_Decisions_and_Changes.md#2-decision-log)).

- **WebAuthn relying party** in `identity-svc`. `userVerification=required`. Attestation is verified for native authenticators; the AAGUID allow-list is maintained.
    - B1 credentials must be platform authenticators flagged `bio_enrolment_bound`. The native apps send an App Attest / Play Integrity statement that the key was created with biometric-only access control and invalidation on enrolment change.
    - On the mobile PWA the browser cannot prove biometric-only verification. The server therefore requires B2 on **every** PWA login and on every step-up, while native B1 attestation lets step-ups inside an existing session use B1 alone, except for device add and recovery.
- **Login state machine** (Redis, 5 min TTL): `begin → factor1_ok → (liveness_pending | approval_pending) → session`. **No session token exists until both factors pass.** Partial states are bound to the DPoP key that started them.
- **Surface decision** is made on the server: native attestation → `mobile_app`; a registered platform authenticator on a mobile UA with no attestation → `mobile_pwa` (B1 + B2); anything else → `desktop` (F1 + F2). An unrecognised authenticator falls back to the strictest path available.
- **B2 provider adapter:** shares its interface with the two age-assurance providers ([02 §3.1](02_Shared_Contracts.md#31-tiers)). Uses a randomised challenge and a 1:1 match against `login_reference`. Only `{match: bool, score, pad_pass: bool}` comes back to TRYST; no image or template. **Fail closed:** a provider outage blocks new logins, not existing sessions. An in-session unlock never needs B2.
- **Cross-device approval (F2):** the QR code carries a single-use 60 s token bound to the desktop's DPoP key and coarse location. The approving app shows the desktop's browser, approximate city and time, and needs a fresh B1 assertion. No push notification is sent.
- **Risk controls:** rate limits per contact hint, device and IP; lockout escalation (5 failures → 15 min; 10 → recovery only). Every success and failure writes `auth_event`. A new-device login sends a masked in-app notice to the other registered devices. Impossible-travel and new-AAGUID signals force recovery-grade checks.
- **Recovery:** B2 + fresh V2 whose HMAC `anchor_doc` must equal the stored candidate (02 §4.1), then a 24 h hold. The hold is cancellable from any registered device and is never bypassable by staff (no break-glass path exists for authentication).
- **Lawful basis:** B2 is biometric processing for unique identification (Art. 9). It uses explicit consent `biometric_login`, recorded in the ledger. The accessible alternative (D-17) keeps the consent freely given (ToS Q14).
- **Cost:** about 1–2 B2 checks per mobile MAU per month (30 d sessions plus new devices and step-ups). At an assumed £0.10–0.30 per check this is £0.10–0.60 per MAU per month; it is added to the infra line in [01 §12](01_Product.md#12-roadmap-team-budget-and-phase-gates). Replace with the vendor quote in P0.

### 6.8 Source B §9–§10 control coverage (v1.2)

Every control in the founder's Security and privacy doctrine (§9) and Trust, safety and misuse controls (§10) ([07](07_Source_B_Founder_Brief.md)) maps to an implementation.

| Source B control | Implementation |
|---|---|
| Article 6 + Article 9 + DPIA + consent records + transfers | [01 §14.2](01_Product.md#142-uk-gdpr--dpa-2018); consent ledger (FR-040); UK/EU residency (NFR-06) |
| Minimisation | FR-002; age band, not DOB; geohash-5; no contact upload (PSI only) |
| Separation (identity, profile, media, analytics, billing) | Isolated clusters and peppered join (§6.2); media-svc and ciphertext-only store; ClickHouse pseudonymous; `billing_ref` only in IDENTITY-DB |
| Encryption, key rotation, access telemetry | §6.2; **NFR-18** |
| Authorisation (ABAC) | §6.5; FR-005; **FR-070** role separation |
| Private media | Signed short-lived access + recipient-bound wrapped keys (FR-018); no public CDN; watermarking; revocation < 5 s (NFR-04); **FR-069** format allow-list and re-encode on device, server malware scan for public tier |
| Payments | Tokenised PSP, neutral truthful descriptor (§10); **FR-070** finance staff cannot browse profiles |
| Admin | Zero standing access, dual-approved break-glass, WORM log (NFR-10); **FR-071** dual approval for intimate evidence |
| Deletion | §6.3; backups 35 d disclosed ([02 §7](02_Shared_Contracts.md#7-retention-and-erasure)); legal holds narrowly scoped (`report_case.legal_hold`) |
| Incident readiness | §6.6 runbooks: outing, sextortion, intimate-media leak, insider abuse, account takeover, mass breach |
| Assurance | ASVS L3 / MASVS L2+R (NFR-08); threat model (P0); **NFR-19** bug bounty + annual privacy audit |
| Fake profiles / bots | §8 L1, M7, M7b, fake-profile stack, human review |
| Blackmail / sextortion | M6e freeze, evidence preservation, victim resources, Action Fraud path (§7.3) |
| Stalking / doxxing | Coarse location + rotating cells (FR-065); visibility (FR-009); **no free-text member search exists**, and slate size is capped; M6d; targeted network blocks as an enforcement action only (VPNs are scored, never blanket-blocked); M7b repeat-account detection |
| Non-consensual media | Hash matching where lawful (D-10, StopNCII); **FR-069** upload warning; reveal/revoke controls; 1 h takedown; sanctions |
| Coercion in threesomes | Independent accounts and VC (FR-006); per-person approval and veto; private withdrawal from group threads (02 §9); either partner dissolves; no "couple owner" |
| Underage access | V2 gate (FR-001), re-check triggers (02 §3.2), 15 min escalation and mandatory reporting |
| Commercial sex / trafficking | No member-to-member payments; solicitation detection; specialist review; jurisdictional reporting (FR-037) |
| Bias / predatory targeting | No advertising at all; exposure audits (M9, fairness audits); **FR-072** protected-trait harassment rule |
| Retaliatory reports | Reporter-reputation scoring; evidence review; appeal; no permanent ban from one uncorroborated report (§8) |

## 7. Guardian — safety models (M6a–M6e)

### 7.1 Central problem

*Source: A §21.1*

**Consensual power exchange looks identical to coercion at the token level.** Guardian therefore classifies the **trajectory** of a conversation, not its content:

```
consent_state(t) = f(explicit_affirmations, explicit_refusals, limit_declarations,
                     enthusiasm_markers, reciprocity, initiative_balance)
RISK fires on:   refusal(t) ∧ escalation(t+k) · limit_stated(t) ∧ limit_approached(t+k)
                 disengagement(t) ∧ persistence(t+k) · sustained asymmetric initiative
RISK never fires on: intensity, vocabulary, kink taxonomy, power-exchange framing, explicitness
```

Any model that separates consensual from coercive only through topic vocabulary is rejected at eval.

### 7.2 Models and thresholds

*Source: A §19.3 L4, §21.2*

| ID | Target | Threshold action |
|---|---|---|
| M6a | Coercive control (isolation, monitoring, guilt leverage, secrecy framing imposed on the counterparty, identity erosion) | > 0.75 → queue; > 0.92 → thread freeze pending review |
| M6b | Boundary violation (escalation after a refusal, repeated declined request, approaching a hard limit, recasting a "no" as a "maybe") | > 0.70 → send-block + warning; 2nd → strike; 3rd → suspend |
| M6c | Off-platform pressure | > 0.65 → caution banner to the counterparty; feeds M5 and M6a |
| M6d | Identity/location pressure | Auto-redact in transit (client) + banner + strike on repeat |
| M6e | Image coercion and sextortion | Any hit > 0.60 → **immediate freeze**, human within 1 h. Tuned for recall |

**Serving:**
- M6a–e run **on device** for E2EE threads, inline < 80 ms p95. Only a score + reason code + model version is sent to `/v1/guardian/signals`.
- Server-side: cross-account correlation on reason codes (the same script used against many people), M7/M7b, and profile/media moderation.
- Scores are calibrated (isotonic) so moderators can triage by probability. Thresholds are set per model, locale and segment, and reviewed monthly against the overturn rate.

### 7.3 Escalation matrix

*Source: A §8.4*

| Signal | Automatic | Human SLA | External |
|---|---|---|---|
| Suspected minor | Immediate full freeze of both accounts; content preserved under legal hold | 15 min | Mandatory report; law-enforcement liaison |
| Intimate image abuse / NCII | Media quarantine, hash to StopNCII, sender frozen | 1 h | Takedown log; victim-support routing |
| Sextortion / coercion | Thread freeze, evidence preserved, victim safety banner | 1 h | Action Fraud referral path |
| Threat of violence / stalking | Account suspended, mutual geo-suppression, device ban | 1 h | Police referral on request |
| Romance fraud pattern | Payment-link stripping, shadow-limit, cohort sweep | 4 h | Bank/PSP intelligence sharing |
| Commercial sexual services | Immediate removal | 4 h | — |
| Harassment (M6 > 0.9) | Send-block, warning, strike | 24 h | — |

**Evidence.** Server-side evidence exists only when a member reports and **chooses** to disclose decrypted content, or when Panic preserves it. Reporter identity is stripped before evidence reaches labelling.

### 7.4 Training data

*Source: A §21.3–21.4*

**Permitted sources only:**
- D1: expert-authored synthetic corpus — 12k threads, **matched consensual/coercive pairs**, written with DV and kink-education specialists.
- D2: published taxonomies, used as the label schema.
- D3: public toxicity transfer, **not** as a source of coercion labels.
- D4: reported content with separate, revocable training consent (from P3).
- D5: active learning on the moderator queue (overturns are the most informative labels).

**Never:** scraped platform conversations without a lawful basis, purchased corpora of uncertain provenance, or survivors' disclosures collected for another purpose.

**Annotation:**
- Triple annotation on the coercion classes; Krippendorff's α > 0.70 to accept a batch.
- Senior adjudication.
- Annotator welfare budgeted: rotation caps, clinical supervision, exposure limits on M6e material.

**Bootstrapping:**

| Phase | Data | Expected performance |
|---|---|---|
| P1 | Rules + D3 | recall ~0.55 / precision ~0.30 |
| P2 | + D1, D2 | ~0.82 / 0.61 |
| P3 | + D4, D5 | ~0.91 / 0.74 |
| P4 | Per-language evaluation | |

### 7.5 Profile and media moderation (server side)

Public-tier profile photos and bios are **not** E2EE and are moderated on the server:
- EXIF strip
- AI-generation detector and reverse-image search on **every** upload
- face consistency across the photo set
- C2PA provenance where present
- nudity classification (explicit content is web-only and V2-only)
- CSAM hash matching (IWF/NCMEC lists) before storage
- adversarial perturbation for A8

Private-tier media is client-encrypted; the sending client runs on-device checks before encrypting, including hash matching against licensed lists where licensing permits ([D-10](06_Decisions_and_Changes.md#2-decision-log)).

### 7.6 Evaluation gates and limits

*Source: A §21.5–21.9*

**Ship gates:**

| Gate | Requirement |
|---|---|
| Matched-pair separation (hard gate) | AUC > 0.85 |
| M6b | Recall > 0.90 at FPR < 0.03 |
| M6e | Recall > 0.95, precision ≥ 0.35 |
| Dialect FPR parity | Within 1.5× across groups |
| Counterfactual gender swap | Mean score delta < 0.05 |
| Appeal overturn | < 8% |
| Moderator agreement | > 75% |

Fairness slices: dialect/register (AAVE, MLE, regional), non-native English, gender, orientation/structure across S1–S6, age band. Audited quarterly.

**Adversarial resistance:**
- Quarterly red-team retainer.
- Rolling windows across messages and sessions.
- Cross-account correlation.
- Weekly drift checks. A sudden drop in positives means an evasion technique has been found, not that behaviour improved.

**Guardian never:**
- takes a permanent action without a named human;
- scores protected or inferred characteristics;
- produces a public or counterparty-visible score;
- reads message content on the server;
- flags on vocabulary alone.

---

## 8. Admission control (L1–L5)

| Layer | Server responsibilities |
|---|---|
| **L1 Identity binding** | Liveness with a randomised challenge (provider); ID-document match through the provider; device attestation verification (App Attest / Play Integrity), emulator/root detection; payment-instrument fingerprint (token); £9.99 deposit. VPN/datacentre IPs are **scored, never blocked** |
| **L2 Ban anchor** | At V2, `identity-svc` stores `anchor_candidate` HMACs ([E-02](06_Decisions_and_Changes.md#3-errata-fixed-in-v11)). On a permanent ban (human decision, `reason_code`), the candidates are copied to BAN-DB. On `auth/verify` (new identity), attestation and payment checks, and V2 completion, every anchor is checked; a match → **silent refusal** with a generic error that never names the anchor. Biometric anchor only if counsel confirms a lawful basis ([D-08](06_Decisions_and_Changes.md#2-decision-log)). A successful appeal destroys the anchors |
| **L3 Attestation** | Conduct declaration stored in the consent ledger; Basic DBS verified through the issuing route (no PDF uploads) and stored as `{dated_fact}`; Clare's Law content is static, with **no field** for the outcome |
| **L4 Behavioural** | Guardian M6a–e (on device), M7b ban-evasion graph → shadow-limit → human review → anchor extension |
| **L5 Consortium** | Interface built behind a feature flag: anchors + reason code only. Disabled until competition-law and data-protection opinions (ToS Q8) |

**Fake-profile stack [A §19.4]:**

| Stage | Checks |
|---|---|
| Sign-up | Liveness · ID match · GAN/deepfake detection on selfie and video · attestation · deposit |
| Media | C2PA · reverse-image · EXIF anomaly · face consistency · AI-generation classifier on every upload |
| Network | Score, never block |
| Graph | Sign-up bursts · shared device or payment · referral-chain anomalies |
| Behaviour | Typing cadence · templating · copy-paste · latency uniformity · slate-interaction entropy |

Detectors are refreshed quarterly; a static detector is a known future failure.

**Enforcement pipeline (FR-035):** warn → shadow-limit → suspend → permanent + anchor → law-enforcement referral. Grounds g1, g3 and g4 go straight to permanent. `enforcement.decided_by` is required for `permanent` (DB constraint). Appeals are assigned to a different reviewer (enforced in the console) within 10 working days. Reporter-reputation scoring down-weights coordinated or retaliatory reports; one uncorroborated report never leads to a permanent ban [B §10].

---

## 9. MLOps and release gates

**Model ship gates:**
- Offline replay on held-out cohorts measuring **ΔR**, not AUC alone.
- Shadow → 5% → 25% → 100% rollout, with automatic rollback on report-rate or S4-churn regression.
- Model card + DPIA annex for each model.

**Agent gates:**
- 400-case golden set per agent: Envoy negotiation fidelity, Guardian adversarial recall, Cartographer extraction accuracy.
- Prompt-injection corpus.
- Run on every PR touching a prompt, policy file or agent code.

**Audits:**
- Quarterly fairness audit per protected characteristic and segment.
- Monthly counterfactual audit for popularity collapse (the 15% exploration floor is the structural defence).

**Privacy Auditor [B §5]:** continuous probes. It **blocks a release** if any of the following holds:

| Probe | Blocks release if… | Mapped to |
|---|---|---|
| Media access | Media is reachable without an active grant | G-NG-1 |
| Erasure | Erasure proof is missing or slow | G-NG-2 |
| Location | Location can be triangulated (distance-band oracle tests) | G-NG-3 |
| Tier | Any gated endpoint is reachable below V2 | G-NG-4 |
| Couple co-sign | A same-device co-sign is accepted | G-NG-5 |
| Hard gates | Any hard-gate property test fails | G-NG-7 |
| Client SDKs | A third-party SDK appears in a client SBOM | G-NG-8 |
| Staff access | Any staff role can reach message plaintext | G-NG-9 |

---

## 10. Commerce

- **PSP:** dedicated high-risk acquirer (MCC 7273, Visa IRP Tier 1 registration) plus a **backup MID from day one**. Card data is tokenised and never touches TRYST.
- **Descriptor:** neutral and truthful, with a customer-service URL; enforced per MID.
- **Vouchers:** offline codes redeemed through `/v1/vouchers/redeem`, single-use, with rate limits and fraud scoring.
- **Subscriptions** (TRYST+, ENVOY, DUO, GHOST add-on): state machine with pre-contract information, cooling-off notice (CCR 2013), pre-renewal email (≥ 7 days), one-step cancel and end-of-contract notices. Built to the full DMCC regime now.
- **Keys ledger:** double-entry, idempotent. Intent spend is held and **refunded on reply**. Keys don't expire while the account is open. They are refunded on removal unless the ground is in ToS cl. 17.3, and are never forfeited before an appeal ends.
- **Chargebacks:** Ethoca/Verifi alerts → auto-refund rules. Alert at 0.5%; hard stop review at 0.65%.
- **Payment fingerprint** feeds L1 and the ExclusionRing heuristics (suppress only).
- **App-store purchases:** web-first. Where a store build must offer in-app purchase, entitlements are unified in `commerce.entitlement` ([D-11](06_Decisions_and_Changes.md#2-decision-log)).

---

## 11. Compliance engineering and operations

- **Consent ledger:** append-only, hash-chained (`entry_hash = H(prev_hash ‖ record)`); verified nightly; exportable on SAR.
- **DSR service:** SAR export, rectification, erasure (§6.3) and portability, all built in sprint one [A §14.2].
- **OSA:** report SLAs are measured and published. Transparency data is produced from SAFETY-DB (FR-052). A named accountable manager owns the on-call escalation.
- **Article 22:** human review path for suspensions; decisions record `decided_by`.
- **Jurisdiction gate:** GeoIP at the edge + sign-up declaration + travel detection from coarse client location. The block list is versioned config, and each enabled market references its legal opinion ID.
- **Legal process:** published policy. Warrants or court orders are required. The member is notified where lawful. Only what exists is disclosed (ciphertext and minimal metadata).
- **Observability:** OpenTelemetry. Logs carry no content and truncate IPs at ingest (NFR-11). SIEM, plus anomaly alerts on staff access.
- **SLOs:** NFR-01, -02, -05, -13.

---

## 12. Backend backlog by phase

| Phase | Epics (backend) | Definition of done |
|---|---|---|
| **P0** (M0–M2) | Threat model; architecture review; assurance-provider adapters (×2) spike; PSP + backup MID integration spike; IaC landing zone (eu-west-2), KMS/HSM, isolated clusters; contracts repo (OpenAPI + Protobuf) generated from 02 | Threat model signed off; G-P0 legal items done by others |
| **P1** (M2–M6) | `edge-gateway` (DPoP, gates); `identity-svc` (sign-up OTP, devices, **passkeys + two-factor login, step-up, recovery**, assurance, anchors, join HMAC); `core-svc` modules: profile, couple (VC), intent (Keys, caps), discretion (policy, zones), reveal, threads (MLS DS), meet, consent, deletion, commerce (TRYST+, Keys, deposit, vouchers); Cartographer; Broker P1 (Stage 0 + weighted baseline); Curtain v1; Guardian server (signals intake, escalation, M7, media moderation); Aftercare; `safety-svc` + trust console; Privacy Auditor probes; DSR | FR-001–009, 011–014, 016–023, 026–044, 046–047, 049–051, 054, 056–062, 064–065, 069–072 met (backend side); NFR-01/02/05/06/10/11 met; no-go list clear; control cohort running (no Envoy) |
| **P2** (M6–M10) | Envoy (agent + Briefs); Mirror v1 (M1–M4, α, reset/insights); ExclusionRing PSI; Guardian P2 models (D1/D2); FR-024/025 | **G-P2:** meets per intent sent ≥ +40% vs control |
| **P3** (M10–M15) | DUO, ENVOY, GHOST billing; ENVOY proposals; travel mode; DBS attestation; transparency report; 24/7 T&S tooling; scale to 25k verified | G-P3 |
| **P4** (M15–M24) | Multi-market jurisdiction config; localisation; per-language Guardian; consortium interface (if cleared); scale to 150k | G-P4 |
