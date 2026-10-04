# 01 — TRYST Product Specification v1.1

Part of the [TRYST v1.1 baseline](00_README.md). Contracts live in [02](02_Shared_Contracts.md); implementation in [03](03_Backend.md) and [04](04_Frontend.md).

---

## 1. Executive summary

> **PRODUCT SCOPE**
>
> TRYST is designed specifically for adults who are already in a relationship and want a discreet one-off affair, a continuing extramarital connection, another couple, or a threesome. Existing-partner permission is not a condition of membership. Every person using TRYST must nevertheless be an independently consenting adult; the platform prohibits coercion, stalking, blackmail, impersonation, minors, non-consensual imagery and commercial sexual services.
>
> *Founder scope statement, verbatim (Source B; re-confirmed 4 Oct 2026). Applied through [§4](#4-operating-stance-and-principles), [§5](#5-segments-modes-and-intent-shapes) and [D-01](06_Decisions_and_Changes.md#2-decision-log).*

TRYST is a discretion-first connection platform for attached adults, solo adults open to couples, and couples. It covers one-off encounters, ongoing extramarital connections, couples seeking a third, and couple-to-couple connections, for all genders and orientations.

Two things make it different:

1. **An agent-to-agent handshake (Envoy).** It resolves intent, boundaries, logistics, discretion needs and verification parity *before* two people speak.
2. **A breach-assumed architecture.** End-to-end encrypted messaging, client-side encrypted media, split identity and profile stores, cryptographic erasure in under 60 seconds, and partner-collision avoidance through private set intersection. Together these make the category's defining disaster — the 2015 Ashley Madison breach — technically very hard to repeat.

**Board summary [A §21].** The category leader in affairs has vacated that position under regulatory and payments pressure. The leader in non-monogamy is a browsing product with a resolution problem. The opportunity is real. The hard part is not the software but the compliance envelope, marketplace liquidity, and an acquisition-cost model that only works through invitation. Build the envelope first, seed the scarce side first, and let the agent layer earn its place against a measured control.

**Gating.** No engineering build starts before a written UK legal opinion and a completed DPIA ([§14](#14-compliance-envelope)).

### 1.1 Executive product decision

*Founder text, verbatim (Source B; re-confirmed 4 Oct 2026).*

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

> **ONE-SENTENCE PITCH**
>
> TRYST is the intelligent private network where attached adults and couples find precisely matched affairs, extra-relationship connections and threesome partners without exposing more of their identity than necessary.

**How this decision maps onto the rest of v1.2:**

| Decision row | Implemented as |
|---|---|
| Core modes | One-off → SPARK; Ongoing → EMBER; Couple+ and Threesome → THIRD (Couple+ = a couple seeking a third; Threesome = a solo joining a couple); Couple-to-Couple → QUAD; plus OPEN for exploration ([§5.2](#52-modes-member-facing--intent-shapes-data)) |
| Moat | Discretion Graph = Discretion Policy + ExclusionRing + zones enforced on every read (Curtain); Behavioural Compatibility Engine = Mirror + Broker + Envoy ([§7](#7-agents-roles)) |
| Revenue | "Credits" = Keys; "premium privacy" = the GHOST add-on, subject to [D-05](06_Decisions_and_Changes.md#2-decision-log) ([§10](#10-business-model-and-unit-economics)) |
| Launch | Web PWA is the full product and the revenue surface; native apps follow store review, approved by G-P3 ([§12](#12-roadmap-team-budget-and-phase-gates), [§14.3](#143-distribution--two-surface-strategy)) |
| Audience | See [D-01](06_Decisions_and_Changes.md#2-decision-log) on unattached solos |

---

## 2. Thesis and market gap

The discreet-connection market is large, proven and badly served. The incumbents monetise frustration; TRYST monetises resolution. [A §1]

**Founder market thesis, verbatim (Source B §1; re-confirmed 4 Oct 2026):**

> The category is already validated. Ashley Madison markets private, anonymous connections and claims more than 91 million members. Gleeden uses a credit economy for extramarital connections; Victoria Milan markets a rapid-exit "panic button." The opportunity is not to prove demand but to rebuild the category around modern privacy engineering, verified authenticity, gender-neutral pricing, transparent AI and higher-quality matching.

| Incumbent pattern | Observed weakness | TRYST response |
|---|---|---|
| Large anonymous inventory | Fake profiles, low trust and expensive dead-end messaging | Liveness, authenticity confidence and conversation-quality scoring. |
| Credit-heavy messaging | Revenue can conflict with real connection outcomes | Subscription provides core value; credits enhance but do not manufacture access. |
| Basic privacy theatre | A panic button cannot protect server-side data | Pseudonymity, data minimisation, isolated vaults, short retention and discreet billing. |
| Pairwise matching | Poor fit for couples and threesomes | Native multi-person matching and room consent. |
| Swipe engagement | High noise and fatigue | Small daily set of high-confidence, intent-aligned introductions. |

*Membership figures differ between sources: Source B cites Ashley Madison's claim of 91M+ members, Source A cites 80M+. Both are the company's own unaudited claims; use the current figure from Ashley Madison's site, with an access date, in any external material ([E-32](06_Decisions_and_Changes.md#3-errata-fixed-in-v11)). "Gender-neutral pricing" supports the default in [D-06](06_Decisions_and_Changes.md#2-decision-log).*

### 2.1 Market

| Player | Position | Observed weakness | TRYST response |
|---|---|---|---|
| **Ashley Madison** | Affair category leader (claims 80M+ members). Rebranded globally as a "discreet dating app" on 24 Feb 2026, moving away from married dating. | Pay-to-initiate credits (men pay to start conversations); recurring bot and cost complaints; 2015 breach (FTC: about 36M users exposed, deletion failures). | Take the affair position it is vacating, and survive the pressure through §9 and §14. Subscription-led pricing; replying is always free. |
| **Feeld** | Owns ethical non-monogamy and couple-seeks-third. Revenue £48.9M in 2024 (+26%, Companies House); 2M+ active users by 2026. | Thin free tier; high-noise discovery; a browsing product, not a resolution product. | Envoy handshake; daily curated slate; meets as the north star. |
| **Gleeden**, **Victoria Milan**, **Illicit Encounters** | Regional affair sites (credits; panic button). | Fake profiles; "privacy theatre" that does not protect server-side data. | Liveness plus age assurance for everyone; real cryptography. |
| **3Fun / #Open / swinger communities** | Couples and thirds niche. | Fake "couples", moderation gaps, dated UX. | Mandatory dual-verified couple profiles (`VC`). |
| **Mainstream apps** (Tinder, Hinge, Bumble, Grindr) | Mass market. | Not discreet; exposure through social graphs. | ExclusionRing; decoy skin; no social login. |

*Third-party figures are as reported by the sources cited in Source A and have not been independently audited.*

### 2.2 Four failures TRYST fixes

*Source: A §1*

| Failure | Why it persists | TRYST answer |
|---|---|---|
| Stated ≠ real intent | Profiles are self-authored marketing | Behaviour-learned RevealedPreferenceVector matched against IntentVector; divergence is a feature |
| 200 messages, zero meets | Engagement-maximising ranking rewards long unresolved threads | Envoy handshake pre-qualifies; the reward function penalises time in app |
| Attention collapse | Rich-get-richer ranking; couples broadcast to everyone | Inbound caps, Gini constraint, asymmetric Key pricing |
| Discretion as marketing | Retention by default and server plaintext are cheaper | E2EE, cryptographic erasure, token-only verification, PSI ExclusionRing |

---

## 3. Name and brand

*Source: A §2*

**TRYST** — *a private, prearranged meeting between lovers.* One syllable, five letters, no explanation needed, and deniable on a lock screen. Tagline: **Private chemistry. Intelligent discretion.** Internal line: **Discretion is the product.**

- **Brand codes:** oxblood and gold on night; serif display, tight tracking. No pink, hearts, silhouettes or lipstick. Reference: a private members' club, not a nightclub.
- **Fallback names** if trademark clearance fails: ASIDE, DUPLEX.
- **Domains:** tryst.app / tryst.co / trystapp.com plus defensive registrations before any UKIPO Class 9/45 filing becomes public.

### 3.1 Logo

<img src="../../assets/brand/tryst-logo.png" alt="TRYST logo: gold and oxblood interlocking crescents above the TRYST serif wordmark" width="260">

The master logo is [`assets/brand/tryst-logo.png`](../../assets/brand/tryst-logo.png) (1254 × 1254 px, transparent background), supplied by the founder ([D-19](06_Decisions_and_Changes.md#2-decision-log)).

- **Use it exactly as supplied.** Do not redraw, recolour, crop, stretch, re-shadow or separate the mark from the wordmark. The repository keeps the original file byte-for-byte (SHA-256 `5f2ceb55…eae2761`).
- **Background:** the logo is designed for the dark `night` ground (§3.2), and is only placed on `night`, `oxblood-deep` or `oxblood`.
- **Never shown where it would expose a member (P3, P9):**
    - on a device in decoy mode;
    - on the lock screen or in notifications;
    - in the app-switcher snapshot;
    - in billing descriptors;
    - in the home-screen icon of a member who has chosen a decoy skin;
    - in Share My Plan messages.
- **Where it appears:**
    - the web PWA (landing page, sign-in, header);
    - inside the native apps after unlock;
    - store listings, subject to the sanitised-store rules ([§14.3](#143-distribution--two-surface-strategy));
    - investor and press materials.

### 3.2 Colour palette

The palette is sampled from the logo. Tokens live in [`assets/brand/tokens.json`](../../assets/brand/tokens.json) and [`assets/brand/tokens.css`](../../assets/brand/tokens.css).

| Token | Hex | Role | Contrast on `night` |
|---|---|---|---|
| `night` | `#0B0607` | Primary background | — |
| `oxblood-deep` | `#3F0000` | Deep surfaces, pressed states | — |
| `oxblood` | `#560001` | Brand surface, primary buttons | 1.34 (surfaces only) |
| `oxblood-light` | `#702A2E` | Hover, borders, dividers | 1.97 (never text) |
| `bronze` | `#836042` | Secondary lines, disabled states | 3.56 (large text only) |
| `gold` | `#CB944F` | Accent, links, icons, headings | 7.56 AA/AAA |
| `champagne` | `#F5D3AC` | Highlight text, active states | 14.18 |
| `cream` | `#FCF2DD` | Body text; light surface (replaces "bone") | 18.1 |

**Text pairs that pass WCAG 2.2 AA (NFR-07):**
- cream, champagne or gold on `night`;
- cream (13.5), champagne (10.6) or gold (5.65) on `oxblood`;
- oxblood on cream (13.5).

Oxblood and oxblood-light are never used for text on dark grounds.

---

## 4. Operating stance and principles

### 4.1 Stance

*Source: A §1*

> **We are indifferent to your marriage; we are absolutist about consent and safety on our surface.**

TRYST does not police whether a member's partner knows; that is unverifiable and not the platform's business. It does enforce, without exception:

- Verified adults only, through highly effective age assurance.
- Consent from every party *on the platform*. Couples are dual-verified; no one is listed by a partner without co-signature. No permission is required from anyone not on TRYST.
- Zero tolerance for image abuse, sextortion, coercion, commercial sexual services, and minors.

### 4.2 Product principles (changing one requires v2.0)

| # | Principle | Source |
|---|---|---|
| P1 | **Resolution over engagement.** Optimise for confirmed, positively rated meets (QCAM), never time in app or message volume. | A §6.4, §18 |
| P2 | **Hard limits are inviolable.** No learned signal can surface a candidate who violates a hard limit, exclusion zone or structure incompatibility. Enforced by tests, not policy. | A §6.2 |
| P3 | **Breach-assumed by design.** Assume breach, subpoena, and a suspicious partner holding the phone. | A §9 |
| P4 | **Agents never impersonate.** Agents negotiate only with other agents; every Brief is labelled as machine-generated; only a double human accept opens a thread. | A §5.4 |
| P5 | **Stated preference keeps a floor.** Revealed behaviour never drops stated preference below 15% weight; members can view, correct and reset learning. | A §6.2, B FR-010 |
| P6 | **No safety theatre.** No "safe", "vetted" or "background-checked" badges, ever. Only factual, dated statements. | A §19.2, §19.8 |
| P7 | **Safety beats discretion in an emergency.** Panic preserves and releases what emergency services need, regardless of retention settings. | A §19.7 |
| P8 | **Revenue ethic.** No advertising, data sale, affiliates, fake profiles, unreadable credit mechanics, engagement paywalls, or charging for deletion or data access. Replying is always free. | A §15, B §11 |
| P9 | **Truthful privacy claims.** Never promise an affair is undetectable. Disclose the limits (device owner, bank, network, recipient screenshots, legal process) and the declared exceptions (ban anchors, legal holds). | B §4, A §19.3 |

### 4.3 Never

- Never the "cheat better" message; the positioning is "Discretion is the product." [A §16]
- Never a "catch a cheater" feature, partner search or exposure tool.
- Never random or anonymous chat; messaging is between matched members only. [A §14.3]
- Never operate in markets that criminalise adultery or same-sex conduct, and never in Africa or MENA under any brand. [A §14.5]

---

## 5. Segments, modes and intent shapes

### 5.1 Segments

*Source: A §3*

Every segment is first-class in the data model.

| Code | Segment | Job to be done | Primary anxiety |
|---|---|---|---|
| S1 | Attached, undisclosed | Meet one person discreetly without changing my primary life | Exposure |
| S2 | Attached, disclosed / open | Quality connections without re-explaining my structure | Judgement, wasted time |
| S3 | Couple seeking a third | Someone both of us want, who wants both of us | Ghosting; friction between partners |
| S4 | Solo open to couples | Be treated as a person, not a utility; a veto that is respected | Harassment volume; bait-and-switch |
| S5 | Couple seeking a couple | Four-way compatibility with low coordination cost | Coordination collapse |
| S6 | Curious / never acted | Find out whether this is for me, at low risk | Committing too early; discovery |

> **S4 is the scarce asset.** S4 supply decides whether S3 and S5 have a product. S4 inbound is rationed, priced and capped ([03 §5.4](03_Backend.md#54-exposure-fairness)). Any ranking change that pushes S4 inbound above the cap is a regression, whatever it does to session metrics.

Solo members may be attached or unattached. Partner awareness is self-declared, never verified, and never shown to others. ([D-01](06_Decisions_and_Changes.md#2-decision-log))

### 5.2 Modes (member-facing) ↔ intent shapes (data)

Source B's mode names are kept as the **member-facing labels**; Source A's intent shapes are the **canonical data values** ([02 §2.3](02_Shared_Contracts.md#23-intent-shapes-and-modes)).

| Mode (UI) | Intent shape(s) | Typical segments | Notes |
|---|---|---|---|
| **SPARK** | `one_off` | S1, S2, S4, S6 | Matched on availability, distance and chemistry |
| **EMBER** | `recurring`, `ongoing` | S1, S2 | Matched on cadence, emotional depth, communication windows |
| **THIRD** | `third` | S3 ↔ S4 | Couple profile must be `VC`; the third is an equal member with a respected veto |
| **QUAD** | `couple` | S5 ↔ S5 | Four independent verified accounts; match only after each couple's veto mode is satisfied |
| **OPEN** | `exploratory` | S2, S6 | Member picks allowed configurations and visibility |

A member may enable several modes, each with its own visibility and filters.

---

## 6. Product loops

*Source: A §4*

| Loop | Cadence | Flow |
|---|---|---|
| **A — Intake** | Once (~7 min, conversational) | Age assurance → Cartographer interview → IntentVector v0 → ExclusionRing enrolment → Discretion Policy → first slate |
| **B — Discovery** | Daily (< 6 min per session target) | Curated slate (up to 18 cards, 15% exploration) → Intent sent (costed) / pass / save → Mirror updates RevealedPreferenceVector → slate recomputed |
| **C — Handshake** *(the product)* | Per mutual intent | Envoy A ⇄ Envoy B negotiate over a fixed schema → asymmetric Brief to each human → accept / decline / amend → E2EE thread opens **only** on double accept |
| **D — Resolution** | Per connection | Thread → logistics agreed → Safe Meet checklist → meet confirmed → Aftercare prompt at 48 h → reward signal |
| **E — Discretion** | Continuous | Curtain scores session risk → geofence, collision suppression, notification masking, retention enforcement |

> Loop C is the product. If it does not measurably raise meets per intent sent over a control cohort (gate **G-P2**), TRYST is a me-too and should not raise a Series A.

---

## 7. Agents (roles)

Seven agents, each an independently deployable service with its own tool surface, memory scope, policy file and eval suite. They talk over a typed internal bus, never by passing free text. [A §5] Implementation: [03 §3](03_Backend.md#3-agent-services).

| Agent | Role | Autonomy boundary |
|---|---|---|
| **Cartographer** | Conversational intake → signed IntentVector; re-run quarterly or on drift | Member confirms every structured field [B] |
| **Mirror** | Behaviour learning; sole writer of RevealedPreferenceVector | Approved features only; message content never used |
| **Broker** | Retrieval and ranking under hard gates and fairness constraints | Cannot bypass Stage 0 gates; paid status cannot override eligibility |
| **Envoy** | Agent-to-agent handshake; asymmetric Briefs; logistics and timing negotiation | Never talks to a human as the member; cannot open a thread; output is advisory |
| **Curtain** | Discretion risk; owns and enforces the versioned Discretion Policy | May tighten exposure automatically; may never loosen it [B] |
| **Guardian** | Safety; veto over every other agent | No permanent action without a named human; never reads message content server-side |
| **Aftercare** | 48 h private outcome capture → reward | Never shown to the counterparty; never a public rating |

Two release-gate functions that are not agents: **Privacy Auditor** (continuous access, retention and model-leakage tests; can block a release) [B] and the **agent eval harness** ([03 §9](03_Backend.md#9-mlops-and-release-gates)).

---

## 8. Trust, verification and admission policy

### 8.1 Verification ladder

The tier definitions and what each one unlocks are canonical in [02 §3](02_Shared_Contracts.md#3-verification-tiers-and-gates). In summary:

- **V0** = contact handle, onboarding only.
- **V1** = liveness.
- **V2** = highly effective age assurance. **Required for discovery, intents, threads, media, Envoy and meets** ([E-01](06_Decisions_and_Changes.md#3-errata-fixed-in-v11)).
- **V3** = optional bank or digital ID.
- **VC** = couple co-signature: both partners independently at V2, each confirming from their own device.

> **VC is the most important trust feature in the product.** It will cost 20–30% of couple sign-ups in year one. Pay it.

### 8.2 Screening reality and admission control

*Source: A §19*

In the UK you cannot lawfully buy sex-offender or criminal-record screening for platform users (ViSOR is not accessible; DBS checks are employer-only). TRYST builds five-layer admission control instead ([03 §8](03_Backend.md#8-admission-control-l1l5)):

| Layer | What it does |
|---|---|
| L1 Identity binding | One human, one account, permanently: liveness challenge, ID match through the provider, device attestation, payment fingerprint, £9.99 refundable deposit |
| L2 Ban anchor | One-way identifiers that survive deletion. **The declared exception to erasure.** |
| L3 Voluntary attestation | Self-submitted Basic DBS shown as a dated fact; Clare's Law explainer (answer never asked for or stored); conduct declaration at V2 |
| L4 Behavioural detection | Guardian M6a–M6e and M7b; most dangerous users have no record, but they do have a pattern |
| L5 Consortium | Industry hash sharing (e.g., Online Dating Association) of anchors plus reason codes only; gated on competition-law and data-protection opinions |

**"Declared unsafe" grounds** (auditable): (1) self-declared intent to harm or coerce; (2) corroborated reports from two or more independent reporters; (3) a court order (SHPO, restraining or non-molestation order); (4) law-enforcement notification; (5) a model signal confirmed by human review; (6) a knowingly false conduct attestation.

**Ladder:** warn → shadow-limit → suspend → permanent ban + Ban Anchor → law-enforcement referral. Grounds 1, 3 and 4 go straight to permanent.

**Due process:** a named human makes every permanent decision; appeals go to a different reviewer within 10 working days; reporter-reputation scoring (retaliatory or coordinated reporting is itself removable — suspicious spouses and rejected counterparties are predictable abuse vectors); the transparency report publishes the overturn rate.

### 8.3 Safety floor

- **Safe Meet:** public venue first; check-in timer with silent escalation.
- **Share My Plan:** encrypted, unbranded, auto-deleting share with a trusted contact.
- **Panic:** escalation to 999 with location, separate from the discretion **Burn** control ([E-21](06_Decisions_and_Changes.md#3-errata-fixed-in-v11)).
- **Emergency override:** safety beats discretion (P7).
- **Victim-support routing** after an incident.
- **24/7 T&S rota** from P3, with published SLAs.

---

## 9. Discretion promise

TRYST minimises platform exposure. It cannot guarantee secrecy from a device owner, bank, employer, network administrator, recipient screenshot, legal process or a determined third party, and it says so (P9).

The design starts from the eight adversaries in [03 §6.1](03_Backend.md#61-threat-model):
- A1 partner holding the device — daily case
- A2 shared bank statement
- A3 social-graph recognition
- A4 mass breach — existential
- A5 extortion by a counterparty
- A6 civil discovery in divorce
- A7 insider
- A8 face-search de-anonymisation

Member-facing controls are in [04 §5](04_Frontend.md#5-client-discretion-controls). Retention defaults are canonical in [02 §7](02_Shared_Contracts.md#7-retention-and-erasure).

---

## 10. Business model and unit economics

### 10.1 Principle

*Source: A §15.1*

Charge for agency and discretion; make resolution the profitable outcome. Subscription plus success-weighted retention, never pay-per-message. The economic model and the loss function must agree.

### 10.2 Tiers (UK)

| Tier | Price | Includes |
|---|---|---|
| **Verified** (free) | £0 + £9.99 refundable deposit (credited 100% as Keys) | Full profile; be discovered; **reply to anyone free**; 3 outbound intents a week; Envoy handshake on mutual intent; full safety suite and core discretion suite |
| **TRYST+** | £24.99/mo · £14.99/mo billed annually | Unlimited intents, filters, incognito, travel mode, extended media, read-receipt control |
| **ENVOY** | £49.99/mo | The agent tier: Envoy *proposes* outreach and pre-qualification, screens risk, negotiates logistics. Every outbound intent needs a one-tap human approval ([D-04](06_Decisions_and_Changes.md#2-decision-log)) |
| **DUO** | £39.99/mo per couple | Linked `VC` profile, dual-consent controls, joint-veto model, third-screening Envoy |
| **Keys** | £9.99 / 60 | Non-subscribers: intent = 5 Keys solo→solo, 15 Keys couple→solo; **fully refunded on reply** |
| **GHOST** add-on | £7.99/mo | Advanced discretion: multiple decoy skins, custom TTLs, extended ExclusionRing heuristics. Core decoy, duress PIN, Burn, masking and voucher billing stay free ([D-05](06_Decisions_and_Changes.md#2-decision-log)) |

Source B's Black / Duo £34.99 / Signals / Vault+ / Concierge Black tiers are superseded ([E-14](06_Decisions_and_Changes.md#3-errata-fixed-in-v11)). Concierge is deferred beyond P4.

### 10.3 Unit economics (illustrative; replace with beta cohorts)

*Source: A §15.3*

| Metric | Value |
|---|---|
| Blended CAC per sign-up (paid + referral, UK) | £38 |
| Sign-up → V2 verified | 52% |
| Verified → payer (month 3) | 11% |
| Blended ARPPU | £31.50/mo |
| Payer gross churn | 19%/mo (average life 5.3 months) |
| Gross revenue per payer (lifetime) | £167 |
| less PSP (high-risk, 5.0%) / verification (2 × £1.10) / infra + inference | −£8 / −£2 / −£14 |
| **Contribution per payer** | **£143** |
| CAC per payer at £38 per sign-up (£38 ÷ 0.11 ÷ 0.52) | **£664 — paid acquisition alone does not work** |

**Corrected viability thresholds** ([E-03](06_Decisions_and_Changes.md#3-errata-fixed-in-v11)): break-even blended CAC per sign-up = £143 × payer rate × 0.52.

| Verified → payer | Break-even CAC per sign-up | CAC for 3× LTV:CAC |
|---|---|---|
| 11% | **£8.18** | £2.73 |
| 16% | £11.90 | £3.97 |
| 18% | £13.38 | £4.46 |

Source A's "< £12" target is only viable at a payer rate of about 16% or more. Two levers decide the business; track both weekly from beta day one:
1. Raise verified → payer toward 18% (ENVOY is the instrument).
2. Drive blended CAC down through invitations and organic channels.

Treat paid acquisition as a seeding cost, never a growth engine.

---

## 11. Liquidity and go-to-market

*Source: A §16*

1. **Density gate (G-City-1).** No city opens below 2,000 verified profiles; a waitlist beats an empty grid.
2. **Balance gate (G-City-2).** Throttle the oversupplied side to keep the ratio at or below 2.2:1 per city, with a genuine, moving waitlist.
3. **Seed the scarce side first.** Invite waves and founding-member status for S4 (and, subject to [D-06](06_Decisions_and_Changes.md#2-decision-log), under-represented cohorts) with free ENVOY for 6 months.
4. **Invite-only for 9 months.** 3 invites per member, refreshed monthly for members in good standing. Referral rewards never reveal the referrer to the invitee [B].
5. **Cities:** London → Manchester → Birmingham → Brighton → Dublin. Not nationwide.
6. **Channels:** assume Meta and Google reject the ad account. Use podcast host-reads in adjacent verticals, ENM and relationship creators, technology-led PR (agent handshake, breach-proof architecture), and an editorial presence. Never synthetic personas or staff posing as members [B].
7. **Positioning:** "Discretion is the product." Never "cheat better."

**Markets:** launch UK + IE · wave 2 NL, BE, DE, SE, DK, ES, PT · wave 3 CA, AU, NZ. The US is out of scope ([E-16](06_Decisions_and_Changes.md#3-errata-fixed-in-v11)). Every market needs its own written legal opinion.

---

## 12. Roadmap, team, budget and phase gates

*Source: A §17*

| Phase | Window | Scope | Exit gate |
|---|---|---|---|
| **P0 Legal & foundation** | M0–M2 | SPV, counsel, DPIA, OSA assessments, PSP + backup MID, two age-assurance providers, threat model | **G-P0:** SPV incorporated · written counsel opinion · DPIA complete · OSA risk assessments drafted · high-risk PSP + backup MID approved · two assurance providers contracted · threat model signed off |
| **P1 Core MVP** | M2–M6 | Loops A, B, D; E2EE; V2 gating; VC co-sign; Guardian baseline (on-device); deletion and DSR machinery; web PWA + native shells. **No Envoy** — this is the control cohort | **G-P1:** closed beta in London with 2,000 verified · erasure < 60 s p99 · no-go list ([§13.2](#132-no-go-conditions)) clear |
| **P2 Differentiator** | M6–M10 | Envoy, Mirror v1, ExclusionRing PSI | **G-P2:** meets per intent sent up ≥ 40% vs the P1 control. If not met, **stop and rethink** |
| **P3 Public UK + IE** | M10–M15 | Invite-only public launch; DUO + GHOST; native apps approved on both stores; 24/7 T&S | **G-P3:** 25,000 verified · payer rate ≥ 11% · both stores approved |
| **P4 EU wave 2** | M15–M24 | NL/BE/DE/SE/DK/ES/PT; per-market opinions; localised assurance and Guardian | **G-P4:** 150,000 verified · contribution-positive |

Source B's phase-level exit criteria are folded into the gates above and into the phase backlogs in [03 §12](03_Backend.md#12-backend-backlog-by-phase) and [04 §10](04_Frontend.md#10-frontend-backlog-by-phase).

**Team to P3 (17):**
- **Product & engineering (12):** Head of Product · Principal Engineer, security-led (**hire first**) · 2 Backend (Go) · iOS · Android · Web · 2 ML/Agents · Data Engineer · Security Engineer · Design.
- **Trust, legal & growth (5):** Head of Trust & Safety (**hire in P0**) · 2 T&S ops · DPO/Counsel (fractional until P2) · Growth/Community.

**24-month budget ≈ £5.9M:**

| Line | Amount |
|---|---|
| People | £3.4M |
| Legal, DPIA, counsel | £260k |
| Age assurance | £180k |
| Infra + inference | £420k |
| Security audits | £150k |
| Seeding & community | £480k |
| PSP reserves & fees | £220k |
| Contingency (15%) | £780k |

**Plus insurance of £85–140k a year at P3 scale** ([§15.3](#153-insurance-and-structure)), which is not in Source A's total. **Plus biometric login checks** (v1.2): an estimated £0.10–0.60 per mobile MAU per month, pending the vendor quote ([03 §6.7](03_Backend.md#67-authentication--account-holder-only-v12)).

---

## 13. KPIs, no-go conditions and risk register

### 13.1 North star and KPIs

*Source: A §18*

**QCAM — Quality Connections per Active Month:** confirmed meets with a positive aftercare score, per active member, per month. If a change raises engagement and lowers QCAM, it does not ship.

| Metric | P1 | P3 |
|---|---|---|
| QCAM | 0.18 | 0.45 |
| Meets per intent sent | 1.8% | 4.5% |
| Mutual reply rate | 22% | 34% |
| Inbound Gini (per city, per segment) | < 0.60 | < 0.55 |
| Gender ratio (per city) | ≤ 2.2:1 | ≤ 2.0:1 |
| S4 30-day retention | 45% | 62% |
| Report rate per 1k threads | < 12 | < 6 |
| V2 completion | 52% | 68% |
| Chargeback ratio | < 0.65% always (above 0.9% the MID is lost) | |
| Erasure latency | < 60 s p99 | |

Secondary (Source B): qualified reciprocal conversations per 1,000 verified weekly active members; exposure incidents; location-inference success in red-team tests; notification leakage; calibration and hard-boundary violations (target 0).

### 13.2 No-go conditions

Do not launch or scale if any of these hold. Each is backed by an automated or audited test ([03 §9](03_Backend.md#9-mlops-and-release-gates)). [B §14, A]

| ID | Condition |
|---|---|
| G-NG-1 | Private media can be accessed outside an active reveal grant |
| G-NG-2 | Deletion or erasure claims are not technically true (including the declared exceptions) |
| G-NG-3 | Exact location can be triangulated in red-team testing |
| G-NG-4 | Age assurance is not "highly effective" across all four Ofcom criteria, or any gated surface is reachable below V2 |
| G-NG-5 | A couple profile can represent an unverified person or be co-signed from one device |
| G-NG-6 | High-severity reports lack 24/7 response within published SLAs |
| G-NG-7 | Ranking violates any hard limit, exclusion zone or ExclusionRing pair |
| G-NG-8 | Any third-party analytics, advertising or attribution SDK is present in a client |
| G-NG-9 | Staff can read message content |
| G-NG-10 | Any login or recovery path issues a session with fewer factors than [02 §3.3](02_Shared_Contracts.md#33-login-assurance--account-holder-only) requires, or allows SMS/email/staff-assisted sign-in |

### 13.3 Risk register

| Risk | Severity | Mitigation |
|---|---|---|
| Breach (existential) | Critical | E2EE, identity split, cryptographic erasure, short TTLs, zero standing access, quarterly pen test, annual red team on A4/A7, rehearsed 72 h playbook |
| App-store rejection or removal (Apple 1.1.4) | High | Sanitised store build; explicit media web-only; matched-only messaging; web-first revenue |
| Merchant account loss | High | Backup MID from day one; alert at 0.5% chargebacks; Ethoca/Verifi; one-click cancel; pre-renewal notice; voucher path |
| Ofcom enforcement | High | Two assurance providers; inbox-level gating; documented assessments; named accountable manager; quarterly compliance review |
| Liquidity failure in month 4 | High | Density and balance gates; scarce side first; invite-only |
| Envoy does not move meets per intent | High | P1 control cohort; G-P2 at +40%; kill the thesis honestly |
| Agent seen as a bot / impersonation scandal | High | P4; labelled Briefs; publish the design |
| Harm incident attributed to the platform | High | T&S in P0; 24/7 from P3; published SLAs; Safe Meet; transparency reporting |
| CAC model unviable | High | §10.3 thresholds; invite loop; weekly tracking |
| Reputational contagion to parent group | Medium | Ring-fenced SPV, brand, counsel, cap table and PR |
| Feeld or Match Group ships an agent layer | Medium | Speed to P2; the moat is discretion architecture plus the aftercare dataset |
| Two-factor biometric login hurts conversion or excludes disabled members | Medium | Fast provider flow (NFR-16); 30-day sessions; accessible alternative (D-17); measure login drop-off weekly |
| Guardian false positives on consensual kink | High | Trajectory-not-vocabulary design; matched-pair ship gate ([03 §7](03_Backend.md#7-guardian--safety-models-m6am6e)) |

---

## 14. Compliance envelope

*This section gates the build. Everything here needs sign-off from UK counsel experienced in the OSA and Article 9; it says what to instruct counsel on, not what they will conclude. Statutory positions and commencement dates must be checked as at the date of use.* [A §14]

### 14.1 Online Safety Act 2023

TRYST is in scope as a user-to-user service. Ofcom treats dating services as posing a risk of significant harm to minors and published dating-specific guidance in May 2026.

Required before public launch:
- illegal-content and children's-access risk assessments;
- highly effective age assurance meeting all four criteria (accuracy, robustness, reliability, fairness), **applied to the inbox and media, not just sign-up**;
- clear terms, in-app reporting, rapid review and removal;
- controls mapped to Ofcom's named dating risks (romance fraud, intimate image abuse, sextortion, harassment, stalking, grooming);
- a named accountable senior manager.

Penalties reach the greater of £18M or 10% of global turnover, with possible criminal liability for senior managers.

### 14.2 UK GDPR / DPA 2018

- **Article 9 explicit consent** — granular, separate from the ToS, withdrawable, recorded in the consent ledger.
- **DPIA mandatory** — two triggers: large-scale special-category data and systematic evaluation.
- **Minimisation as architecture** — no legal name, exact DOB, ID images or coordinates.
- **Article 22** — human review path for suspensions.
- **DSR machinery from sprint one.**
- **Residency** — UK/EU only; no US sub-processor for Article 9 data without a TIA.
- **Breach playbook** — 72 h, rehearsed twice a year.
- **Article 10 conviction data and the ban-anchor lawful basis** — DPA Sch. 1 Pt 2 para 18 assumed; counsel must confirm.

### 14.3 Distribution — two-surface strategy

- **Apple 1.1.4** rejects "hookup" apps that may include pornography or facilitate prostitution.
- **Google Play** requires minor-blocking for dating apps and restricts anonymous or random chat.

Native store builds are therefore **sanitised**: no nudity or explicit copy, matched-only messaging, 18+, "discreet dating" positioning. The **web PWA** carries the full product: explicit media behind V2, and all billing. Details: [04 §2](04_Frontend.md#2-two-surface-strategy).

### 14.4 Payments

- MCC 7273, high-risk; Visa Integrity Risk Program Tier 1 registration; dedicated high-risk acquirer (3–7%); Stripe, Square and PayPal are not usable.
- **Spousal chargebacks** are the category-specific risk. Mitigations: neutral but truthful descriptor with a customer-service URL, offline voucher/gift-code path, pre-renewal email, one-click cancel, Ethoca/Verifi alerts, backup MID, correct MCC.
- **DMCC Act 2024 subscription regime:** commencement has moved repeatedly; build to the full regime now.

### 14.5 Jurisdiction gating and structure

- Hard geo-block at the edge and at sign-up for any market that criminalises adultery or same-sex conduct, or prohibits the category.
- Travel into a blocked market auto-locks the app to the decoy.
- Ring-fenced SPV with its own controller registration, brand, counsel, cap table and PR, and no public link to the parent group.

---

## 15. Liability, claims and insurance

*Source: A §19.8, §20*

### 15.1 Claims discipline

| Never say | Say |
|---|---|
| "Background checked", "Safe", "Vetted" | "Identity verified", "Age assured" |
| "No sex offenders on TRYST" | "We permanently remove and block re-registration where we find harmful conduct" |
| "Screened for domestic violence" | "We detect and act on coercive and abusive behaviour, and we'll show you how to use Clare's Law" |
| "Guaranteed real people" | "Every member completes liveness and ID verification" |
| Green "verified safe" tick | Factual, dated attestations only |

Overclaiming is a misleading commercial practice under CPUT and the DMCC Act, and the first thing a claimant's solicitor will point to.

### 15.2 Liability architecture

- **CRA 2015 s.65** bars excluding liability for death or personal injury caused by negligence (including mental impairment). Under s.65(2), agreeing to a term does not mean accepting a risk.
- **s.62** strikes unfair terms in full.

The draft ToS therefore uses narrow, honestly carved limits:
- correct characterisation of the service;
- an explicit list of what is and isn't verified;
- no liability for consequences to members' relationships;
- a financial cap of the greater of 12 months' fees or £100;
- a load-bearing "what we do not exclude" clause.

**Keep out:** death/injury exclusions, "use at your own risk", mandatory arbitration or class waivers, unilateral no-notice amendment, any screening implication, and forfeiting Keys before an appeal concludes.

The real protection is [§8](#8-trust-verification-and-admission-policy), not the clauses: liability will turn on whether TRYST took reasonable care. The ToS draft is [05](05_Terms_of_Service_draft.md).

### 15.3 Insurance and structure

Cover needed:
- cyber and data liability, sized for an Article 9 breach;
- tech E&O / professional indemnity;
- public and product liability;
- D&O (OSA named-manager exposure);
- media liability.

The SPV provides structural containment. Indicative premium at P3 scale is **£85–140k a year**.
