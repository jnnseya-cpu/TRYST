# 07 — Source B: Founder Developer Specification (verbatim) and traceability

Part of the [TRYST specification baseline](00_README.md).

This appendix reproduces **Source B** — *TRYST AI Discreet Relationship Platform — Developer Specification, v1.0, 17 September 2026* — **word for word**, extracted directly from [`source/TRYST_Developer_Specification_2026-09-17.docx`](../source/TRYST_Developer_Specification_2026-09-17.docx). The founder re-confirmed it on 4 October 2026.

- **Headings, tables and callouts** keep the original wording. Only the formatting has been converted to Markdown.
- **Precedence:** where Source B conflicts with Source A (the consolidated Spec + ToS), [00 §2](00_README.md#2-sources-and-precedence) decides, and [06](06_Decisions_and_Changes.md) records the outcome.
- **Traceability:** §1 below shows where each Source B section is implemented and what v1.2 added to cover its gaps.

## 1. Traceability — where each Source B section lives

| Source B section | Implemented in | Notes |
|---|---|---|
| Product scope | [01 §1](01_Product.md#1-executive-summary) (verbatim) | Unattached solos: [D-01](06_Decisions_and_Changes.md#2-decision-log) |
| Executive product decision, pitch | [01 §1.1](01_Product.md#11-executive-product-decision) (verbatim + mapping) | |
| 1. Market thesis | [01 §2](01_Product.md#2-thesis-and-market-gap) (verbatim) | Member counts: [E-32](06_Decisions_and_Changes.md#3-errata-fixed-in-v11) |
| 2. Users and modes | [01 §5.2](01_Product.md#52-modes-member-facing--intent-shapes-data), [02 §2.3](02_Shared_Contracts.md#23-intent-shapes-and-modes), [02 §9](02_Shared_Contracts.md#9-couple-and-group-semantics) | Mode names are the UI labels ([D-14](06_Decisions_and_Changes.md#2-decision-log)) |
| 3. Experience architecture | [04 §4](04_Frontend.md#4-screens-and-flows-by-loop); loops in [01 §6](01_Product.md#6-product-loops) | Stage 9 "emergency lock" → **FR-064 (new in v1.2)** |
| 4. Discretion product system | [04 §5](04_Frontend.md#5-client-discretion-controls), [02 §6.4](02_Shared_Contracts.md#64-discretion-policy-versioned) | New in v1.2: **FR-064** emergency lock, **FR-065** rotating location cells; FR-026 wording and FR-029 disclosure updated |
| Privacy reality | [01 §4.2](01_Product.md#42-product-principles-changing-one-requires-v20) P9, [01 §9](01_Product.md#9-discretion-promise) | |
| 5. AI agentic operating system | [01 §7](01_Product.md#7-agents-roles), [03 §3](03_Backend.md#3-agent-services) | Agent mapping in §2 below; new **FR-066** (conversation drafts) and **FR-067** (member-provided windows only) |
| 6.1 Learned dimensions | [03 §4](03_Backend.md#4-behaviour-learning-mirror) | New **FR-068** behaviour decay |
| 6.2 Ranking model | [03 §5.2](03_Backend.md#52-launch-baseline-p1-before-m2m3-are-trained) (P1 baseline), [03 §5.3](03_Backend.md#53-multi-person-scoring-thirdquad) | Optimisation target: Source A's QCAM (meets) is the north star; B's "qualified reciprocal conversations" is a secondary KPI ([01 §13.1](01_Product.md#131-north-star-and-kpis)) |
| 7. Functional requirements (FR-001–015) | [02 §10.1](02_Shared_Contracts.md#101-functional-requirements) | B's FR numbers were folded into the v1.1+ catalogue (each row cites "B") |
| 8. Architecture and data, 8.1 entities, 8.2 API | [02 §4](02_Shared_Contracts.md#4-data-model), [02 §5](02_Shared_Contracts.md#5-edge-api-contract-v1), [03 §2](03_Backend.md#2-service-topology-and-stack) | Stack: Source A's native apps replace React Native ([E-07](06_Decisions_and_Changes.md#3-errata-fixed-in-v11)) |
| 9. Security and privacy doctrine | [03 §6](03_Backend.md#6-security-cryptography-and-discretion-server-side), [01 §14.2](01_Product.md#142-uk-gdpr--dpa-2018) | |
| 10. Trust, safety and misuse controls | [03 §7](03_Backend.md#7-guardian--safety-models-m6am6e), [03 §8](03_Backend.md#8-admission-control-l1l5) | |
| 11. Business model, 11.1, 11.2 | [01 §10](01_Product.md#10-business-model-and-unit-economics), [01 §11](01_Product.md#11-liquidity-and-go-to-market) | Tiers and economics superseded by Source A ([E-14](06_Decisions_and_Changes.md#3-errata-fixed-in-v11), [D-03](06_Decisions_and_Changes.md#2-decision-log)); the revenue ethic is kept as P8 |
| 12. Delivery plan, 12.1 team | [01 §12](01_Product.md#12-roadmap-team-budget-and-phase-gates) | Source A's P0–P4 and 17-person team replace B's plan ([D-12](06_Decisions_and_Changes.md#2-decision-log)) |
| 13. MVP backlog and acceptance | [03 §12](03_Backend.md#12-backend-backlog-by-phase), [04 §10](04_Frontend.md#10-frontend-backlog-by-phase) | |
| 14. KPIs, governance, no-go | [01 §13](01_Product.md#13-kpis-no-go-conditions-and-risk-register) (G-NG-1 to G-NG-7 come from B) | |
| 15. Source register | §3 below (verbatim) | Revalidate before launch |
| Final directive | [01 §4](01_Product.md#4-operating-stance-and-principles), [01 §12](01_Product.md#12-roadmap-team-budget-and-phase-gates) | |

## 1.1 Requirement-ID crosswalk (Source B FR → catalogue FR)

**Caution:** Source B numbers its own FR-001–FR-015, and those numbers do **not** mean the same as the catalogue's ([02 §10.1](02_Shared_Contracts.md#101-functional-requirements)). Inside the verbatim text in §3, an FR number is Source B's. Everywhere else, FR numbers are the catalogue's.

| Source B ID | Source B requirement | Catalogue requirement(s) |
|---|---|---|
| B FR-001 | 18+ entry | FR-001 (V2 gate) |
| B FR-002 | Pseudonymous identity | FR-002 |
| B FR-003 | Intent modes | FR-004 |
| B FR-004 | Discretion policy | FR-005 |
| B FR-005 | Couple representation | FR-006 |
| B FR-006 | Reveal ladder | FR-008 |
| B FR-007 | Private discovery | FR-009 |
| B FR-008 | Contact suppression | FR-010 |
| B FR-009 | Chat safety | FR-032 (report, block), FR-033 (real-time on-device scanning, [D-02](06_Decisions_and_Changes.md#2-decision-log)), FR-034 (escalation), FR-013 (rate and inbound limits) |
| B FR-010 | Behaviour controls | FR-024 |
| B FR-011 | Quick exit | FR-029 |
| B FR-012 | Deletion | FR-038 |
| B FR-013 | Neutral notifications | FR-026 |
| B FR-014 | Reports/appeals | FR-034, FR-035 |
| B FR-015 | No commercial sex | FR-037 |

## 1.2 Architecture, entity and endpoint crosswalk (Source B §8 → contracts)

**Layers.** Every Source B layer is kept. The one exception is "React Native iOS/Android", which is replaced by native Swift/Kotlin apps with a shared Rust crypto core ([E-07](06_Decisions_and_Changes.md#3-errata-fixed-in-v11), [04 §3](04_Frontend.md#3-client-stacks)).
- "OIDC/passkeys" → OAuth2 + DPoP with passkey-only, two-factor login ([02 §3.3](02_Shared_Contracts.md#33-login-assurance--account-holder-only)).
- "Alias service" → pseudonymous `display_name` in PROFILE-DB, joined to identity only through the HSM pepper.
- "Relationship/block graph projection" → the Stage 0 gates ([03 §5.1](03_Backend.md#51-pipeline)).

| Source B entity | Contract entity ([02 §4](02_Shared_Contracts.md#4-data-model)) |
|---|---|
| User | `identity` (IDENTITY-DB) + `profile.status` |
| AliasProfile | `profile` (`display_name`, `bio_ct`, `visibility`, `version`) + `media` |
| IntentProfile | `intent_vector` |
| DiscretionPolicy | `discretion_policy` (versioned) |
| CoupleUnit | `couple_link` + `couple_invite` |
| Introduction | `slate_item` |
| RevealGrant | `reveal_grant` |
| Room | `thread` (MLS group) |
| BehaviourEvent | event envelope ([02 §8](02_Shared_Contracts.md#8-event-taxonomy)) |
| ReportCase | `report_case` + `enforcement` |
| DeletionJob | `ops.deletion_job` |

| Source B endpoint | Contract endpoint ([02 §5](02_Shared_Contracts.md#5-edge-api-contract-v1)) |
|---|---|
| `POST /v1/identity/verify` | `POST /v1/assurance/session` (+ `/v1/assurance/callback`) |
| `PUT /v1/discretion-policy` | `PUT /v1/discretion/policy` |
| `POST /v1/intents` | `POST /v1/intake/vector/confirm` (member-confirmed intent profile); outbound interest is `POST /v1/intent` |
| `POST /v1/couple-units` | `POST /v1/couple/invite` + `POST /v1/couple/cosign` |
| `GET /v1/discovery` | `GET /v1/slate` |
| `POST /v1/introductions/{id}/decisions` | `POST /v1/intent` · `/v1/pass` · `/v1/save` · `/v1/preferences/correction` |
| `POST /v1/reveal-grants` / `DELETE …/{id}` | Same paths |
| `POST /v1/reports` | `POST /v1/report` |
| `POST /v1/preferences/reset` | Same path |
| `DELETE /v1/users/me` | `DELETE /v1/account` |

## 2. Agent mapping (Source B's ten → v1.2's seven)

| Source B agent | v1.2 home | Boundary carried over |
|---|---|---|
| Chemistry Agent | Mirror (learning) + Broker (ranking) | Ranks; never fabricates interest or sends messages (FR-055) |
| Intent Agent | Cartographer | Member confirms every structured field (FR-003) |
| Discretion Agent | Curtain | May tighten, never loosen (02 §6.4) |
| Timing Agent | Envoy (logistics dimension) | **Member-provided windows only, never inferred routines (FR-067)** |
| Conversation Agent | On-device drafting assistant ([D-20](06_Decisions_and_Changes.md#2-decision-log)) | **Draft-only, opt-in, never auto-sent (FR-066)** |
| Authenticity Agent | Guardian (M7, M7b) + admission control L1 | Friction/hold automatic; permanent ban only by a human (FR-035) |
| Risk Agent | Guardian (M6a–e, escalation matrix) | Escalates, restricts, preserves evidence (FR-034) |
| Group Agent | Broker (max-min scoring) + Envoy + couple semantics | Cannot add anyone or reveal anyone without consent (FR-006, FR-049) |
| Learning Agent | Mirror | Approved features only; no message content (03 §4.2) |
| Privacy Auditor | Release gate (not an agent) | Can block a release ([03 §9](03_Backend.md#9-mlops-and-release-gates)) |

## 3. Source B — full text (verbatim)

CONFIDENTIAL PRODUCT BLUEPRINT

> TRYST

> Private chemistry. Intelligent discretion.

AI-AGENTIC PLATFORM FOR AFFAIRS, EXTRA-RELATIONSHIP CONNECTIONS AND THREESOMES

Developer-ready product, AI, privacy, commercial and delivery specification
Version 1.0 | 17 September 2026

> PRODUCT SCOPE

> TRYST is designed specifically for adults who are already in a relationship and want a discreet one-off affair, a continuing extramarital connection, another couple, or a threesome. Existing-partner permission is not a condition of membership. Every person using TRYST must nevertheless be an independently consenting adult; the platform prohibits coercion, stalking, blackmail, impersonation, minors, non-consensual imagery and commercial sexual services.

### Executive product decision

Build TRYST as a privacy-first, affair-specific connection platform rather than a generic dating app. Its strategic advantage is an AI relationship-intelligence layer that learns attraction, discretion requirements, availability, risk tolerance and interaction quality while deliberately collecting less identifying data than incumbents.

| Decision | Specification | Why it wins |
|---|---|---|
| Name | TRYST | One sharp word meaning a private romantic meeting; instantly communicates the category. |
| Audience | Partnered adults of any gender/orientation; couples seeking a third or another couple | Clear, high-intent niche rather than mass-market dating. |
| Promise | Private chemistry. Intelligent discretion. | Combines emotional benefit with the functional moat. |
| Core modes | One-off, Ongoing, Couple+, Threesome, Couple-to-Couple | Intent is explicit before matching. |
| Moat | Discretion Graph + Behavioural Compatibility Engine | Learns fit without exposing identity or partner data. |
| Revenue | Subscription-led with credits and premium privacy | Predictable recurring revenue plus high-margin intent transactions. |
| Launch | Web/PWA first, then native apps subject to store review | Reduces store dependency and permits rapid privacy iteration. |

> ONE-SENTENCE PITCH

> TRYST is the intelligent private network where attached adults and couples find precisely matched affairs, extra-relationship connections and threesome partners without exposing more of their identity than necessary.

### 1. Market thesis

The category is already validated. Ashley Madison markets private, anonymous connections and claims more than 91 million members. Gleeden uses a credit economy for extramarital connections; Victoria Milan markets a rapid-exit “panic button.” The opportunity is not to prove demand but to rebuild the category around modern privacy engineering, verified authenticity, gender-neutral pricing, transparent AI and higher-quality matching.

| Incumbent pattern | Observed weakness | TRYST response |
|---|---|---|
| Large anonymous inventory | Fake profiles, low trust and expensive dead-end messaging | Liveness, authenticity confidence and conversation-quality scoring. |
| Credit-heavy messaging | Revenue can conflict with real connection outcomes | Subscription provides core value; credits enhance but do not manufacture access. |
| Basic privacy theatre | A panic button cannot protect server-side data | Pseudonymity, data minimisation, isolated vaults, short retention and discreet billing. |
| Pairwise matching | Poor fit for couples and threesomes | Native multi-person matching and room consent. |
| Swipe engagement | High noise and fatigue | Small daily set of high-confidence, intent-aligned introductions. |

### 2. Users and modes

| Mode | User need | Eligibility / system rule |
|---|---|---|
| SPARK | Discreet one-off encounter | Attached status accepted; match on availability, distance and chemistry. |
| EMBER | Ongoing affair or recurring connection | Match on cadence, emotional intensity, communication windows and expectations. |
| THIRD | A couple seeks one additional person | Both couple members must separately verify the shared couple profile; the third is treated as an equal user. |
| QUAD | Two couples seek one another | Four independent accounts; shared match only after each side’s configured quorum. |
| OPEN | Flexible extra-relationship exploration | User selects allowed configurations and visibility rules. |

TRYST does not ask a user to prove permission from a spouse or external partner. It only requires consent from the people who are actually participating through TRYST. A couple profile cannot represent the second member without that member’s own account and confirmation.

### 3. Experience architecture

| Stage | User experience | Platform behaviour |
|---|---|---|
| 1. Quiet entry | Neutral landing page, email alias or private number, discreet notifications | No contact upload; no social-login requirement; risk-screen session. |
| 2. Adult/authenticity check | Age assurance and liveness; legal name stays private | Store verification token/result separately from profile. |
| 3. Discretion setup | Choose icon, notification wording, app lock, visibility and location fuzzing | Generate a Discretion Policy applied across all services. |
| 4. Intent map | Select SPARK, EMBER, THIRD, QUAD or OPEN and define boundaries | Build explicit intent vector and exclusions. |
| 5. Discovery | Receive a limited ranked set with compatibility explanations | Candidate retrieval excludes blocks, contacts hash list if opted in, risk conflicts and impossible schedules. |
| 6. Reveal ladder | Alias → blurred gallery → selected photos → voice/video | Each reveal is separately approved and revocable. |
| 7. Private room | Chat, disappearing media, availability coordination | Encrypted content, screenshot friction, abuse/scam detection. |
| 8. Outcome learning | Private “more/less like this” and quality feedback | Update preference model; feedback is not shown to the other user. |
| 9. Exit | Pause, vanish, delete or emergency lock | Immediate discovery removal; deletion and retention workflow starts. |

### 4. Discretion product system

| Control | Requirement |
|---|---|
| Neutral shell | Optional generic icon/name on device where technically possible; neutral web favicon and page title option. |
| Notification privacy | Default notification says “You have an update”; never includes names, images or message excerpts. |
| App lock | Independent PIN/biometric lock; session re-authentication after configurable inactivity. |
| Quick exit | Immediate neutral screen plus client cache clearing; clearly state this cannot hide network/device history. |
| Location veil | Use distance bands and rotating approximate cells; never reveal live or exact home/work coordinates. |
| Invisible mode | Only people the member explicitly likes can see them, subject to mutual eligibility. |
| Recognition shield | Optional on-device contact hashing to suppress known contacts; raw address book never uploaded. |
| Media vault | Face blur, selective reveal, recipient watermark, expiry, revocation and no public URLs. |
| Discreet billing | Neutral merchant descriptor where lawful and accurate; transparent receipt settings; no deceptive descriptor. |
| Emergency lock | Freeze logins and sessions, hide profile and disable media links from a separate recovery path. |
| True deletion | Free account deletion; documented purge and backup expiry; no false “full delete” claim. |

> PRIVACY REALITY

> TRYST must never promise that an affair is undetectable. It can minimise platform exposure, but cannot guarantee secrecy from a device owner, bank, employer, network administrator, recipient screenshot, legal process or determined third party.

### 5. AI agentic operating system

| Agent | Role | Autonomy boundary |
|---|---|---|
| Chemistry Agent | Learns attraction patterns from explicit choices and reciprocal outcomes | Ranks; never fabricates interest or sends messages. |
| Intent Agent | Transforms free-text goals into SPARK/EMBER/THIRD/QUAD/Open vectors | User confirms every structured field. |
| Discretion Agent | Creates and enforces each member’s privacy policy | May reduce exposure automatically; cannot weaken settings. |
| Timing Agent | Matches compatible communication and meeting windows | Uses user-provided windows, not inferred home routines. |
| Conversation Agent | Suggests openers, boundary questions and respectful replies | Draft-only; no impersonation or autonomous chat. |
| Authenticity Agent | Detects bots, scams, stolen images and identity inconsistency | Can add friction/hold; permanent ban follows policy/review. |
| Risk Agent | Detects harassment, blackmail, coercion, trafficking and underage signals | Escalates, restricts and preserves evidence under defined rules. |
| Group Agent | Solves multi-person compatibility and quorum for THIRD/QUAD | Cannot add anyone or reveal one user to another without product consent. |
| Learning Agent | Updates behavioural embeddings and experiment assignments | Uses approved features only; sensitive raw content excluded by default. |
| Privacy Auditor | Continuously tests access, retention and model leakage | Can block a release when thresholds fail. |

### 6. Behaviour-learning design

#### 6.1 Learned dimensions

| Dimension | Signals | Treatment |
|---|---|---|
| Attraction | Likes, passes, private preference corrections | Embeddings; never disclose inferred sensitive traits. |
| Intent fit | Mode, desired duration, emotional depth, exclusivity expectation | Hard/soft constraints controlled by user. |
| Discretion fit | Visibility, response windows, reveal pace, distance bands | Mismatch strongly penalised. |
| Communication fit | Message cadence, length preference, voice/video comfort | Aggregate features; raw content short-lived and excluded from training by default. |
| Reliability | Mutual attendance feedback, cancellations, ghosting pattern | Private score with anti-retaliation safeguards. |
| Respect/safety | Blocks, reports, boundary violations, pressure patterns | Safety model; reports never become visible reputation badges. |
| Group symmetry | Each participant’s attraction and boundaries | Maximise minimum compatibility, not average score alone. |

#### 6.2 Ranking model

> TRYST MATCH SCORE

> Eligibility filter first. Then: 24% reciprocal attraction + 20% intent fit + 15% discretion fit + 12% communication fit + 10% availability/logistics + 8% reliability + 6% novelty + 5% model confidence − risk penalties. THIRD/QUAD adds a max-min fairness constraint so one person’s strong interest cannot overwhelm another participant’s low fit.

- Hard exclusions: age uncertainty, block edges, missing participating-couple confirmation, prohibited purpose, incompatible mode, critical safety risk.
- Behaviour decays over time; the user can reset all inferred preferences.
- Every introduction displays two to four non-sensitive reasons and offers a correction control.
- No ranking uplift based on gender. Paid visibility applies only inside an already eligible set and is visibly labelled.
- Optimisation target: qualified reciprocal conversations and user-rated fit—not time spent, message volume or credit burn.

### 7. Functional requirements

| ID | Requirement | Acceptance criterion |
|---|---|---|
| FR-001 | 18+ entry | Discovery remains locked until age-confidence threshold and liveness pass. |
| FR-002 | Pseudonymous identity | Public profile never requires legal name, employer or exact address. |
| FR-003 | Intent modes | A member may enable multiple modes with separate visibility and filters. |
| FR-004 | Discretion policy | Every API returning profile data evaluates the requester against current policy. |
| FR-005 | Couple representation | THIRD/QUAD shared profile publishes only after all represented members approve exact version. |
| FR-006 | Reveal ladder | Each media/identity disclosure records grant, recipient, version and expiry. |
| FR-007 | Private discovery | Invisible members are discoverable only following their outbound interest. |
| FR-008 | Contact suppression | Optional hashing occurs on device; server stores salted blinded tokens only. |
| FR-009 | Chat safety | Rate limits, block, report, media scanning and blackmail/threat escalation operate in real time. |
| FR-010 | Behaviour controls | Member can view categories, correct assumptions, disable personalisation and reset learning. |
| FR-011 | Quick exit | One action hides content and clears client view in under 500 ms on supported clients. |
| FR-012 | Deletion | Profile disappears immediately; purge jobs meet published retention SLA and generate audit proof. |
| FR-013 | Neutral notifications | Lock-screen content contains no relationship, profile or message detail. |
| FR-014 | Reports/appeals | 24/7 urgent queue; reasoned enforcement and appeal workflow. |
| FR-015 | No commercial sex | Solicitation/payment-for-contact patterns route to review and enforcement. |

### 8. Architecture and data

| Layer | Developer specification |
|---|---|
| Clients | Next.js PWA/web; React Native iOS/Android after policy clearance; local encrypted vault; biometric/PIN lock. |
| Edge | WAF, bot management, API gateway, rate limiting, device attestation and privacy-safe abuse fingerprints. |
| Identity plane | OIDC/passkeys, verification adapter, alias service and separated identity vault. |
| Core services | Profile, Intent, Discretion Policy, Discovery, Match, Group, Conversation, Media, Safety, Billing, Deletion. |
| Data plane | PostgreSQL transactions; relationship/block graph projection; Redis; Kafka-compatible events; encrypted object storage. |
| AI plane | Feature registry/store, vector retrieval, ranker, LLM gateway, model registry, evaluation and experiment platform. |
| Operations | Trust console with redacted defaults; SIEM; audit service; privacy/deletion orchestrator; incident command. |

#### 8.1 Core entities

| Entity | Essential fields |
|---|---|
| User | id, status, jurisdiction, verification_state, deletion_state |
| AliasProfile | user_id, display_alias, bio, approved_media, visibility, version |
| IntentProfile | user_id, modes[], boundaries, cadence, emotional_depth, availability |
| DiscretionPolicy | user_id, visibility, distance_band, reveal_rules, notification_policy, version |
| CoupleUnit | id, member_ids, approval_version, active_state |
| Introduction | subjects, candidate, score, model_version, explanation_codes, paid_label |
| RevealGrant | resource_id, owner, recipient, scope, expires_at, revoked_at |
| Room | id, participant_ids, mode, membership_version, safety_state |
| BehaviourEvent | pseudonymous_subject, type, context, consent_basis, ttl |
| ReportCase | reporter, subject, taxonomy, severity, evidence_refs, decision, appeal |
| DeletionJob | user_id, systems[], due_at, completion_proof |

#### 8.2 API surface

| Method / endpoint | Purpose / rule |
|---|---|
| POST /v1/identity/verify | Begin age/liveness flow; returns status token, not document. |
| PUT /v1/discretion-policy | Versioned update; cannot silently broaden exposure. |
| POST /v1/intents | Create user-confirmed mode-specific intent. |
| POST /v1/couple-units | Invite represented participant; no spouse approval requirement unless spouse is represented in-platform. |
| GET /v1/discovery | Policy-filtered ranked introductions; cache result re-authorised on read. |
| POST /v1/introductions/{id}/decisions | Private like/pass/correction; idempotent. |
| POST /v1/reveal-grants | Recipient-specific, expiring access. |
| DELETE /v1/reveal-grants/{id} | Immediate revocation and CDN/key invalidation. |
| POST /v1/reports | Create protected report and evidence references. |
| POST /v1/preferences/reset | Delete/neutralise learned vectors and rebuild from explicit choices. |
| DELETE /v1/users/me | Immediate disable and deletion workflow initiation. |

### 9. Security and privacy doctrine

TRYST processes exceptionally sensitive information: sex life, sexual orientation, relationship status, private messages, intimate media and location. Under UK GDPR, sex-life and sexual-orientation data is special category data. The controller needs an Article 6 lawful basis, an Article 9 condition, a DPIA, explicit-consent records where relied upon, strict retention, rights handling and international-transfer controls.

| Control family | Mandatory implementation |
|---|---|
| Minimisation | No real name on profile; no mandatory contact upload; no home/work address; age result instead of DOB where possible. |
| Separation | Identity vault, profile store, media vault, analytics and billing linked through scoped pseudonymous identifiers. |
| Encryption | TLS 1.3; envelope encryption; per-object media keys; HSM/KMS; key rotation and access telemetry. |
| Authorisation | ABAC using requester, resource, reveal grant, block graph, discretion version, safety state and jurisdiction. |
| Private media | Short-lived signed access, recipient binding, no public CDN, malware scanning, watermark option, rapid key revocation. |
| Payments | Tokenised PSP; store minimum references; truthful neutral descriptor; finance staff cannot browse profiles. |
| Admin | Just-in-time access, dual approval for intimate content, reason codes, immutable logs and anomaly detection. |
| Deletion | Free, simple, verified purge across primary stores; backup expiry disclosed; legal holds narrowly scoped. |
| Incident readiness | Specific runbooks for outing, sextortion, intimate-media leak, insider abuse, account takeover and mass breach. |
| Assurance | OWASP ASVS/MASVS, threat modelling, independent penetration tests, bug bounty and annual privacy audit. |

> ASHLEY MADISON LESSON

> The FTC reported that the 2015 Ashley Madison breach exposed information associated with 36 million users and alleged security and deletion failures. TRYST must treat privacy claims as verifiable product requirements: collect less, separate identity, delete truthfully and assume attackers will target the platform specifically because of its members’ vulnerability to exposure.

### 10. Trust, safety and misuse controls

| Risk | Controls |
|---|---|
| Fake profiles / bots | Liveness, device risk, behaviour graph, photo provenance checks, friction and human review. |
| Blackmail / sextortion | Payment-demand and threat detection, rapid room freeze, evidence preservation, victim resources and law-enforcement policy. |
| Stalking / doxxing | Coarse location, visibility controls, search throttling, network-level blocks and repeat-account detection. |
| Non-consensual media | Hash matching where lawful, upload warnings, recipient controls, rapid takedown and account sanctions. |
| Coercion in threesomes | Independent accounts, individual approval, private withdrawal, no single “couple owner.” |
| Underage access | Age assurance, re-check triggers, content/report triage and legally required reporting. |
| Commercial sex / trafficking | No payments between members, solicitation detection, specialist review and jurisdictional reporting. |
| Bias / predatory targeting | No sensitive-trait ad targeting; exposure audits; prohibit fetishisation/harassment based on protected traits. |
| Retaliatory reports | Reporter credibility signals, evidence review, appeal and no automatic permanent ban from one uncorroborated report. |

### 11. Business model

| Offer | Indicative UK price | Included value |
|---|---|---|
| TRYST Free | £0 | Verified profile, intent setup, limited introductions, mutual matches, block/report and deletion. |
| TRYST Black | £24.99/month or £179/year | Unlimited curated discovery, invisible mode, advanced filters, travel, preference insights and priority support. |
| TRYST Duo | £34.99/month or £249/year per couple | THIRD/QUAD tools, joint profile, multi-person matching and shared room coordination. |
| Private Signals | £6.99 / £14.99 / £29.99 packs | Labelled high-intent signals or temporary visibility; never bypass filters or consent. |
| Vault+ | £7.99/month | Extended encrypted media vault, granular expiry and multi-device recovery. |
| Concierge Black | £299-£799/month | Human-curated introductions for verified members; no escorting or outcome guarantee. |

> REVENUE ETHIC

> Do not monetise fake profiles, unreadable credit mechanics, gender discrimination, safety, deletion or access to one’s data. TRYST earns more when members trust the platform enough to stay—not when it manufactures frustration.

#### 11.1 Illustrative unit economics

| Metric | 12-month target | Scale target |
|---|---|---|
| Free-to-paid conversion | 7-10% | 12-16% |
| Blended ARPPU | £27-£34/month | £35-£45/month |
| Monthly paid churn | <7% | <4.5% |
| Gross margin | >72% | >82% |
| CAC payback | <8 months | <5 months |
| LTV:CAC | >3x | >4.5x |
| Verification + safety cost / MAU | £1.20-£2.20 | £0.55-£1.10 |
| Chargeback rate | <0.7% | <0.4% |

#### 11.2 Go-to-market

1. Launch a private web beta in London, then Birmingham and Manchester only after local liquidity thresholds are met.
1. Acquire through privacy-led content, discreet affiliates, relationship/lifestyle publishers and invitation codes—not explicit advertising that triggers platform rejection.
1. Seed verified, balanced cohorts; never use synthetic personas or employees posing as members.
1. Lead PR with modern privacy engineering and authenticity rather than moral provocation.
1. Create referral rewards that reveal no referrer relationship to invitees and expire automatically.
1. Expand by city only when compatible inventory, moderation coverage and safety SLAs are proven.

### 12. Delivery plan

| Phase | Duration | Deliverables | Exit gate |
|---|---|---|---|
| 0. Discovery | 6 weeks | Research, legal/DPIA, naming clearance, threat model, prototypes, store review | No fatal legal/platform blocker; user value validated. |
| 1. Privacy core | 12 weeks | Identity vault, profile, intent, discretion engine, deletion, admin audit | Independent architecture/security review. |
| 2. Closed beta | 12 weeks | Discovery, chat, reveal ladder, billing, reports and moderation | High-severity response SLA and zero critical access failures. |
| 3. AI learning | 12 weeks | Retrieval/ranking, explanations, reset, experiments and model governance | Quality lift without safety/privacy regression. |
| 4. Group modes | 10 weeks | Duo profiles, THIRD/QUAD matching, quorum rooms | Independent-participant consent tests pass. |
| 5. City launch | 16 weeks | Growth, SRE, 24/7 operations, native app decision | Retention, paid conversion and liquidity gates. |

#### 12.1 Initial team

| Function | Capacity |
|---|---|
| Leadership | Product Development Director, CTO/Engineering Director, Trust & Safety Director |
| Product/Design | Senior PM, UX researcher, 2 product designers |
| Engineering | Tech lead, 4 backend, 2 mobile/web, platform/SRE, QA automation, security engineer |
| AI/Data | ML lead, 2 ML engineers, data engineer/analyst |
| Trust/Operations | Policy lead, investigator lead, moderation coverage, customer operations |
| Legal/Privacy | DPO, product counsel, app-store counsel, external penetration/privacy specialists |
| Commercial | Growth lead, lifecycle marketer, partnerships/affiliate manager |

### 13. MVP backlog and acceptance

| Epic | P0 delivery | Definition of done |
|---|---|---|
| Quiet onboarding | Alias, private contact, age/liveness, device lock | No public/legal identity leakage; unverified users excluded. |
| Intent engine | Five modes, hard boundaries, availability and discretion | User confirms structured output; change history audited. |
| Discretion engine | Visibility, neutral notifications, location veil, invisible mode | Every read path policy-tested; stale caches cannot expose. |
| Discovery | Candidate retrieval, ranker baseline, explanations | Hard exclusions are deterministic; paid status cannot override. |
| Reveal ladder | Blurred/private media, grants, expiry and revoke | Revocation invalidates active access quickly and is measured. |
| Chat | Encrypted transport/storage, disappearing options, block/report | Blocked users lose all room access; evidence flow is isolated. |
| Safety console | Cases, severity, actions, appeals and audit | Urgent cases page on-call; admin access is least privilege. |
| Billing | Free, Black, Duo and signals | Clear prices/refunds; neutral truthful descriptor; safety remains free. |
| Deletion | Pause, vanish and full delete | Published SLA; primary-store purge verified; backups age out. |
| AI controls | Preference view/correct/reset/opt-out | Reset removes inferred vectors; model card and rollback exist. |

### 14. KPIs, governance and no-go gates

| Dimension | Metric |
|---|---|
| North star | Qualified reciprocal conversations per 1,000 verified weekly active members. |
| Match quality | Mutual interest, reply rate, private fit rating, repeat-connection rate, unwanted-contact rate. |
| Discretion | Exposure incidents, location inference success in red-team tests, notification leakage, deletion completion. |
| Authenticity | Verified ratio, bot prevalence, scam report rate, successful account-takeover rate. |
| Safety | Severe incidents per 10,000 rooms, response time, repeat offender, appeal overturn and victim-support SLA. |
| AI | Calibration, hard-boundary violation, explanation usefulness, exposure fairness, drift and privacy leakage. |
| Commercial | Conversion, ARPPU, churn, CAC payback, contribution margin, refunds and chargebacks. |
| Reliability | Crash-free sessions, p95 latency, message delivery, RPO/RTO and emergency-lock latency. |

> NO-GO CONDITIONS

> Do not launch or scale if private media can be accessed outside an active grant, deletion claims are not technically true, exact location can be triangulated, age assurance is ineffective, couple profiles can represent unverified people, high-severity reports lack 24/7 response, or ranking violates a hard boundary.

### 15. Source register

Accessed 17 September 2026. Policies, prices and legal requirements must be revalidated before launch.

| Source | URL |
|---|---|
| Ashley Madison official positioning | https://www.ashleymadison.com/ |
| Gleeden official site | https://en.gleeden.com/ |
| Victoria Milan official privacy features | https://www.victoriamilan.com/ |
| FTC Ashley Madison settlement and breach findings | https://www.ftc.gov/news-events/news/press-releases/2016/12/operators-ashleymadisoncom-settle-ftc-state-charges-resulting-2015-data-breach-exposed-36-million |
| ICO special category data guidance | https://ico.org.uk/for-organisations/uk-gdpr-guidance-and-resources/lawful-basis/special-category-data/what-is-special-category-data/ |
| UK GDPR Article 9 | https://www.legislation.gov.uk/eur/2016/679/article/9 |
| Apple App Review Guidelines | https://developer.apple.com/app-store/review/guidelines/ |
| Google Play inappropriate content policy | https://support.google.com/googleplay/android-developer/answer/9878810 |
| NIST AI Risk Management Framework | https://www.nist.gov/itl/ai-risk-management-framework |
| OWASP Mobile Application Security | https://mas.owasp.org/ |

### Final directive

> BUILD TRYST

> Build the platform the brief demands: an adult-only, gender-inclusive, privacy-first system for discreet affairs, continuing extra-relationship connections and threesomes. The investable difference is not secrecy theatre. It is the combination of high-intent modes, authentic users, a Discretion Policy enforced at every data access, multi-person matching, transparent behavioural learning and security designed for the consequences of exposure.

| Non-negotiable | Launch standard |
|---|---|
| Market clarity | Partnered adults, affairs and threesomes are explicit product categories—not hidden behind generic dating language. |
| Discretion | Every exposure path is controlled by policy, tested adversarially and described without false guarantees. |
| Intelligence | AI learns attraction, intent, timing and communication fit while users can inspect, correct, reset or disable learning. |
| Authenticity | Age, liveness and anti-fraud controls make the network credible without publishing legal identity. |
| Economics | Subscriptions fund core value; optional signals add margin without manufacturing frustration or overriding consent. |
| Readiness | No public launch until privacy, deletion, media access, safety response and city liquidity gates are proven. |
