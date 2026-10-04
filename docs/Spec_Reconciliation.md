# TRYST — Spec reconciliation

Compares the founder's **Developer Specification v1.0** (17 Sep 2026, `docs/source/TRYST_Developer_Specification_v1.0.docx`) with the **PRD & Business Plan draft v0.1** (4 Oct 2026, `docs/TRYST_Product_Requirements.md`).

**Rule applied:** v1.0 is the source of truth. Where the PRD conflicts with it, the PRD should change unless the founder decides otherwise (items marked **Decision**).

## 1. Direct conflicts

| # | Topic | Spec v1.0 | PRD v0.1 | Recommendation |
|---|---|---|---|---|
| C1 | Mode names | SPARK, EMBER, THIRD, QUAD, OPEN | One-off, Ongoing, Threesome, Couple-to-couple, Open, Group (3–4 individuals) | Adopt v1.0 names. Drop "Group of individuals"; v1.0 has no such mode. |
| C2 | Core audience | Partnered adults and couples | Also includes singles open to partnered people | **Decision:** v1.0 implies singles are out of scope. A THIRD candidate may be single, so state whether "attached status" is required or only accepted. |
| C3 | Launch platform | Web/PWA first; React Native after store clearance | Native Swift/Kotlin + web at MVP | Adopt v1.0. Note: disguised icon, panic gestures and FLAG_SECURE only work fully in native apps, so the PWA MVP gets a reduced discretion set (neutral title/favicon, quick exit, app lock). |
| C4 | Discovery UX | Small daily set of intent-aligned introductions; no swipe | Daily slate plus browse grid; like / pass / super-like | Adopt v1.0. Remove the browse grid and super-likes; "Private Signals" replace super-likes. |
| C5 | Gender-based pricing | No gender discrimination in pricing or ranking | Free Plus for women and non-binary members | Adopt v1.0. Remove the free-for-women offer; use balanced cohort seeding instead (spec §11.2). |
| C6 | Tiers and prices | Free; Black £24.99/mo or £179/yr; Duo £34.99/mo or £249/yr; Private Signals £6.99/£14.99/£29.99; Vault+ £7.99/mo; Concierge Black £299–£799/mo | Free; Plus; Elite; Couple Plus; Concierge £399 per quarter; credits | Adopt v1.0. Elite has no v1.0 equivalent, so drop it. |
| C7 | Unit economics | Year 1: 7–10% conversion, ARPPU £27–34, churn <7%, LTV:CAC >3x, safety cost £1.20–2.20/MAU | 10% conversion, ARPPU ~£22, churn 14%, LTV:CAC 1.16× at launch, safety cost ~£0.82/MAU | **Decision:** v1.0 figures are targets; the PRD's are base-case assumptions. Keep the v1.0 targets but model sensitivity at 10–14% churn. The PRD probably underestimated verification and safety cost, so use the v1.0 range. |
| C8 | Chat encryption vs scanning | "Encrypted transport/storage"; real-time media scanning and threat escalation in chat (FR-009) | End-to-end encryption (MLS) with on-device classifiers; server never sees content | **Decision (architectural):** server-side real-time scanning requires the server to read content, which rules out true E2EE. Options: (a) E2EE with on-device detection plus reporter-shared evidence; (b) server-side encryption at rest with scanning. (a) is stronger against breaches, (b) is stronger for safety. |
| C9 | Phasing of group modes | THIRD/QUAD in Phase 4, after AI learning | Couple+1 and couple+couple in the MVP | Adopt v1.0. MVP = SPARK, EMBER, OPEN. |
| C10 | Timeline | Phases total about 68 weeks to city launch (6+12+12+12+10+16) | About 40 weeks to public UK launch | Adopt v1.0, but confirm whether any phases overlap. If they are strictly sequential, launch is around month 16. |
| C11 | Ranking model | Explicit weighted score (24/20/15/12/10/8/6/5 minus risk) with max-min for groups | Two-tower retrieval plus gradient-boosted re-ranker; soft-min for groups | Compatible. Use the v1.0 weighted score as the launch baseline (Phase 1–2) and the learned ranker in Phase 3. Soft-min is a smooth form of max-min. |
| C12 | Team | About 30 roles, including PM, UX research, DPO, product counsel, app-store counsel and growth | About 22 FTE | Adopt v1.0. |

## 2. In v1.0 but missing from the PRD (to add)

- **Agents:** Intent Agent, Timing Agent and Privacy Auditor (which can block a release); 10 agents in total, each with an autonomy boundary.
- **Discretion features:** Emergency lock from a separate recovery path; "Vanish" exit state; rotating location cells; neutral web favicon and page title.
- **Discretion Policy as an object:** versioned, enforced at every read path, and can't be silently widened (FR-004, `PUT /v1/discretion-policy`).
- **Reveal ladder:** alias → blurred gallery → selected photos → voice/video, with versioned grants (FR-006).
- **Learned dimensions:** reliability score with anti-retaliation safeguards; communication-fit and timing dimensions.
- **North-star metric:** qualified reciprocal conversations per 1,000 verified weekly active members. The KPI framework covers 8 dimensions.
- **No-go conditions** (spec §14), to be merged into the PRD launch gates.
- **Revenue ethic and "privacy reality" statements**, used as product principles.
- **Entities:** `IntentProfile`, `DiscretionPolicy`, `Introduction` (with `paid_label` and `explanation_codes`), `DeletionJob` (with `completion_proof`).

## 3. In the PRD but not in v1.0 (keep as supplements)

- Legal coverage beyond the UK: the US (state age-verification laws, BIPA, FOSTA-SESTA, state privacy and health-data laws), EU DSA, and geo-blocking of jurisdictions that criminalise adultery or same-sex relations.
- Wider competitor set: Feeld, Illicit Encounters, 3Fun/#Open, swinger communities and mainstream apps, plus lessons from Grindr's penalties.
- Threat model priorities, moderation SLAs by category, and CSAM/NCII hash-matching vendors.
- Non-functional requirements (availability, latency, scale, accessibility, localisation, data residency).
- Fuller data model and API list, including couple dissolution, Couple Charter and data export.
- Payment-discretion caveats (app-store history) and the onboarding "Discretion Check".
- Multi-market sequencing (EU wave 1, then US by state).

## 4. Decisions needed from the founder

1. **C2:** Are unattached singles allowed, at least as THIRD candidates?
2. **C8:** E2EE with on-device safety, or server-side encrypted storage with real-time scanning?
3. **C7:** Should the financial model use the v1.0 targets as its base case, or the more conservative PRD assumptions?
4. **C10:** Are the v1.0 delivery phases sequential, or can Phases 1–3 overlap?
