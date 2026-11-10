# 10 · Growth Intelligence (embedded)

**Status:** built into the platform ([D-73](06_Decisions_and_Changes.md#2-decision-log)), not released to production
until the launch blockers clear. Owner: growth lead · editor.

This file holds the *Trend Intelligence AI developer build and distribution specification
v2.3* (10 November 2026), word for word, below the line. The founder asked for it to be
**embedded into the whole existing platform, not a separate one**, and to **work invisibly:
nothing visible to members**. This header says how the spec maps onto TRYST and where TRYST
departs from it. Where they differ, this header and D-73 win.

## Rules for TRYST

1. **Invisible to members and visitors.** No screen, notice, label, banner, cookie, script,
   email or push in the member app comes from growth intelligence. Members never see that
   it exists. Its only public effect is the wording of a guide page, after an editor
   approves it, and no guide page exists until an editor creates one. Guide pages never
   mention how their wording was chosen; drafts are checked against the claims discipline
   ([01 §15.1](01_Product.md#151-claims-discipline)) and against words such as "AI" or
   "algorithm".
2. **One platform.** It runs as `growth-svc`, a third entrypoint in the same Go image as
   `identity-svc` and `safety-svc`, on the same VM, behind the same load balancer, with the
   same staff sign-in and roster, the same deploy, CI and baseline lock. Staff use it from
   the existing console (**Growth**).
3. **No member data** (keeps [D-21](06_Decisions_and_Changes.md#2-decision-log)'s rule). It has its own database
   (`tryst_growth`) and no credential to any member database. The member platform answers it on
   **one** internal path with weekly counts of **distinct members** by city (Tonight
   starts, intents sent), and only for complete weeks with **at least 50 people**; a
   smaller count is withheld, never sent as zero. Each member is counted once per week;
   the keyed tokens that make this possible are discarded with their key when the week
   ends. Staff can import Search Console and trends exports, but can't type in platform counts.
4. **People approve every change.** Briefs and changes need an editor who is not their
   author. Approval binds the exact wording and page revision. Bounded automatic action
   (§5, score and confidence ≥ 85) is **off**.

## How the spec maps onto TRYST

| Spec | In TRYST |
|---|---|
| Installation, tenancy, row-level security (§12, §15) | One installation: TRYST. Isolation is a separate database and service with no member credentials, not multi-tenant RLS |
| Platform profile (§14) | `backend/internal/growth/profile.tryst.json`, reviewed in the repository and baseline-locked: GB / en-GB, four launch cities and couples, claims quoted from the truth-audited journal posts, prohibited wording from 01 §15.1 |
| Source registry, observations (§3, §5) | `platform_tonight`, `platform_intents` (member platform, k ≥ 50 people), `search_console` (import), `trends_import` (context only) |
| Detection and scoring (§5, §17) | `internal/growth/scoring.go`, `engine.go`: the spec's formulas at full precision; families merged by declared family and shared provenance |
| Growth Brief v1 (§6, Appendix A) | `shared/contracts/growth/growth-brief.v1.schema.yaml` (verbatim) with the spec's fixtures; validated in Go |
| Core API (§6, §16) | `/ops/v1/growth/*` in `shared/contracts/openapi/ops.v1.yaml`, staff only; roles `growth` (prepare) and `editor` (approve, apply, roll back) |
| Inference modes (§2) | `deterministic` templates (launch default) or `deployment` (a model served inside the deployment, `GROWTH_INFERENCE_URL`). No host AI exists in TRYST, and no provider key is read anywhere |
| Channels (§7) | SEO and owned placement on guide pages (live, after approval); organic social as copy for posting by hand. Lifecycle and paid are **off**: TRYST never sends marketing email ([D-71](06_Decisions_and_Changes.md#2-decision-log)) and buys no ads |
| Native owned surface and measurement (§18) | `/guides/{slug}` on growth-svc's public listener, reachable only if `guides_domain` is set in Terraform. Plain HTML, no script, no cookie; views and join clicks counted per page revision and day; reports labelled observational |
| Write safety (§18) | Compare-and-set on page revisions with a deterministic action key; interrupted writes settled by receipt on restart; freeze; rollback only where the page is still at the revision the change produced |
| Commercial control plane (§19) | Not applicable: TRYST is the only installation |

## Conformance (§21) in TRYST's tests

| Scenario | Test |
|---|---|
| TI-01 replay and conflict | `TestIngestReplayAndConflict` |
| TI-02 isolation | `TestGrowthCallerReachesOnlyAggregates`, `TestConsoleRoutesRefuseUnauthenticatedCallers`, safety console refuses growth staff |
| TI-03 reference scores | `TestReferenceScoresAreUnrounded` |
| TI-04 syndication and stale feeds | `TestSyndicatedCopiesCountOnceAndStaleCannotActivate` |
| TI-05 expired brief | `TestExpiredBriefStopsAQueuedChange` |
| TI-06 injected instructions; TI-19 no unapproved route | `TestInjectedTextCannotPublishAnything` |
| TI-07 crash after write | `TestCrashAfterPageWriteIsSettledOnRestart` |
| TI-08 page changed after preview | `TestBriefToPublishedPageAndBack` (stale change refused; rollback over a newer change stops for review) |
| TI-16 no page until created; drafts exportable | `TestGuidePagesAreInvisibleUntilAnEditorCreatesThem`, `e2e/growth.spec.ts` |
| TI-22 no new credential | `TestTrystProfileClaimsAreTheAuditedWording` (unknown profile fields refused) |

Not covered yet: TI-09, TI-11, TI-13 (commercial plane, not applicable), TI-10 and TI-21
with a real bundled model (none is deployed), TI-14 (lexical matching only), TI-15
(growth holds no personal data to erase), TI-17, TI-18, TI-20 (single installation, no host AI).

---

# Trend Intelligence AI
## Developer build and distribution specification

**Version 2.3 · 10 November 2026 · Full keyless workflow requirements**

**Developer start:** §12–22 define the v1 product scope, platform installation, contracts, commercial packaging and release gates. §§1–11 explain the underlying design and source research. Where a pilot illustration differs from an implementation rule, follow §§12–22.

**Product boundary:** a standalone, industry-neutral engine for any platform or business. The core has no default brand, country, language, industry, market signal provider, commerce model, CMS or conversion event. Each installation selects its own sources, taxonomy, policies, channels and outcome definitions. Use the console and imports for a standalone deployment, or connect a host through the API and optional adapters. The full core workflow requires zero new external API keys or connector credentials. AI can reuse the host’s already-configured service or use inference supplied with the product.

### 1. What the product does

A trend reader spots a change in what the platform’s target customers are searching for, asking about or buying. A strategist checks whether the business can serve that demand. If it can, the system sends a shared, time-limited brief to the existing SEO and marketing agents. Those agents adapt their work to the brief. The platform measures the result and uses that evidence to decide whether to continue, stop or revise the playbook.

The product is a control layer for existing agents. It changes their priorities, audiences, offers and approved claims at run time through a versioned brief. Existing agent roles, permission boundaries, product facts and brand policy remain authoritative. An SEO agent can update an approved page. A sales agent can prepare a relevant email. Neither can grant itself publishing privileges. One trend can generate different briefs for different platforms, or no brief at all.

**Build objective:** faster response to relevant demand with more qualified demos, completed orders or other tenant-defined business outcomes. Rank and engagement are intermediate signals. No system can guarantee rankings, market dominance or incremental lift.

### 2. Keyless installation and inference modes

**Product requirement.** 100% of the core workflow, from automatic reading of available signals through approved activation, outcome measurement and rollback, must run without asking the user or developer to obtain or enter a new third-party API key, connector token, OAuth grant or AI provider key. This is a release gate for the core product, beyond successful setup or a draft-only demonstration. A host’s existing AI integration may use a local model or an external provider. The module calls the host’s AI gateway through an existing service interface or a thin server-side adapter. The host keeps its provider credentials and bills usage through its existing account. Keyless onboarding does not remove normal sign-in, workload identity or installation-level permission checks.

| Path | How it works | Failure behaviour |
|---|---|---|
| **Host AI** | Reuse the platform’s already-configured AI service and authorised models. The host gateway owns provider credentials, rate limits and routing. | If missing, revoked, rate-limited or unavailable, use only an explicitly authorised alternate route or switch to deterministic drafts. |
| **Deployment AI** | Ship a model service with the standalone product, using bundled local inference for disconnected installations or included inference for the managed product. Customers supply no AI key. A clean standalone install must demonstrate real model-generated output. | Use deterministic drafts if unavailable. Disconnected deployments use local inference or deterministic mode. |
| **Deterministic** | Local detection, scoring, templates, review and export operate without an AI model or provider account. | Report the degraded or intentional template mode in outputs. |

**Signal and channel access.** Baseline inputs are first-party push, CSV or manual imports, and permitted public feeds that require no credentials. A host can also forward data from integrations it already operates, or apply an approved action through its existing publishing service. The module never takes possession of the connector’s secrets or requires a new connection to the third-party service. If a specific external source or destination has neither permitted key-free access nor a host integration that is already authorised, show that capability as unavailable. Continue the full workflow on available signals and an owned surface. Channel-ready drafts remain exportable. Private third-party data and writes to external accounts depend on existing access. Keyless does not imply universal third-party access.

**Core acceptance matrix**

| Step | Built-in keyless path |
|---|---|
| Read signals automatically | Poll permitted public feeds where configured and aggregate first-party product or owned-page events. First-party events alone can support a deployment when enough independent evidence is available. Imports supplement the automated path. |
| Detect, rank and retarget | Cluster signals and score relevance. Create one approved, versioned Growth Brief that changes SEO and marketing priorities across the included workers or the host’s existing agents. |
| Generate actions | Use existing authorised host AI or included inference. Produce an SEO patch and marketing copy against the same brief. Templates keep the workflow available during model outages. |
| Apply approved changes | Publish the SEO and campaign changes to an owned surface using the built-in page renderer or a host’s existing authorised adapter. Track applied revision, publication receipt, and rollback. |
| Measure and adapt | Use built-in aggregate outcome collection or first-party outcome ingestion to compare against the configured baseline or holdout. Continue, revise or stop according to guardrails. Show insufficient evidence when appropriate. |

**Coverage boundary.** Measure 100% against the rows above, with zero new provider credentials. Direct publication to every external channel, ad buying, private audience insights and live feeds requiring new access are separate capabilities. The owned activation path must still work when all of them are disabled. Data sufficiency, host authorisation and editorial approval remain independent gates.

### 3. Source registry and what each signal can prove

Every source adapter must register access_path (public_keyless, host_mediated or file_import) and source family, access mode, owner, terms and permitted uses, supported territories and languages, cadence, ingestion lag, quota, retention, and failure status. Track raw event time, observation window, ingestion time, coverage, normalization and provenance separately.

| Feed | Access | Use and limitation | Initial cadence |
|---|---|---|---|
| First-party internal search, failed searches, product events, conversions and catalog availability | Tenant event pipe or batch | Reveals demand among current users and whether it can be served. Segment by geo and intent. Suppress small cohorts. | 15–60 min |
| CRM, sales and support topics | Tenant-side aggregate import or connected API | Buyer objections and intent. Send aggregated topics or approved redacted extracts only. Remove staff/test activity and duplicates. | Daily |
| Google Search Console | Host’s existing authorised integration or exported CSV | Owned search performance by query/page/market. API can return a subset of rows. Missing queries are unknown. Use complete reporting windows and distinguish paid, organic and internal search. [S1] | Daily |
| Google Trends Trending Now | Supported public RSS export where available | News-led rising searches. Useful for discovery, weak for niche B2B demand. Relative Trends values are not absolute demand. Compare within a consistent scaling scope. [S2] | 30–60 min |
| Google Trends API | Data supplied by a host with existing approved access | Historical comparisons using consistent scaling. Expose only when the host supplies the data. Manual imports and RSS retain their own scope and do not imitate API coverage. [S3] | Per approved terms |
| Sector and regional public data | Permitted keyless feeds, host-supplied data or imports | Select by industry, geographic coverage and use rights. For example, the UK’s Find a Tender feed provides procurement notices. Count distinct opportunities and track amendments separately when using a procurement feed. [S4] | Source-specific |
| Wikimedia pageviews, curated industry RSS and public bulletins | Approved public feeds | Context or corroboration. Pageviews are attention, not buyer intent. Preserve publication date, licence and source lineage. [S5] | Daily or source-specific |
| Keyword Planner, YouTube, social trends | Public permitted observations, insights forwarded by an existing host integration, or manual research | Keyword Planner has volume estimates. YouTube charts measure platform popularity. TikTok Creative Center can inform creative research. Verify supported regions and permitted use. Access requiring a new connector credential is outside the module’s integration path. [S6–S8] | Daily or manual |

A source marked stale can provide dated context but cannot support fresh automated actions. A feed gap is unknown data, never zero interest. Syndicated stories, mirrors and amendments can contribute one corroborating source family at most. Track whether campaign activity itself generated the apparent spike. Validate semantic clustering in every configured language and market. Use local place aliases, code-switching, spelling variants and reviewer-approved terminology where applicable. Lack of external coverage must remain visible.

### 4. Agent responsibilities

| Worker | Takes | Produces | Authority |
|---|---|---|---|
| Source collector | Allowlisted feed or tenant events | Normalised observations with provenance | Read only. No model necessary. |
| Trend reader | Recent observations, seasonality, current campaigns | Clusters, change evidence, trend class, expiry, missing data | Read only. Deterministic calculations. Local model can label intent and explain evidence. |
| Opportunity strategist | Clusters, platform profile, catalog, capacity, site inventory | Ranked decision, business reason, score breakdown, recommended channels | Recommends. Cannot authorise actions. |
| Brief coordinator | Eligible decision and tenant policy | One shared Growth Brief plus channel-specific job assignments | Dispatches drafts and permitted internal tasks. |
| SEO and marketing agents | Growth Brief and source references | Typed actions for their channel with preview and measurement plan | Existing role scopes only. No direct provider credentials. |
| Review and execution service | Typed action, evidence, approvals, expected asset revision | Verified patch, scheduling intent or delivery receipt | Deterministic policy, rate limits and approval checks. |
| Measurement worker | First-party outcomes, spend, attribution and control groups | Results, rollback signal and playbook recommendations | Can pause authorised actions. Cannot amend policy. |

A single service may host several workers. Keep their outputs and permissions distinct. A deterministic router decides transitions and validates payloads. Authorised inference routes handle semantic work without unrestricted tools. Existing agents should adopt one common SDK to resolve the effective brief at run time. Cache by tenant, platform, market, channel and brief version. On expiry, fetch the latest approved brief or revert to the baseline instructions. Do not rewrite agent prompts permanently.

### 5. Detection and scoring

**Observation schema:** `tenant_scope, source_id, source_family, source_record_id, observed_at, window_start, window_end, ingested_at, geography, locale, segment, metric, value, unit, scaling_scope, coverage, evidence_uri, evidence_hash, terms_version`. Unique source version key prevents replays from inflating counts. Public records live in a shared read-only dataset. All tenant observations and derived artifacts are scoped.

1. Canonicalise accents, spelling, aliases, dates and geography while retaining the original text. Form candidate clusters using lexical matches and local embeddings. Separate different buyer intents, countries and product categories even when language is similar. Keep a cluster revision and evidence list.
2. Use complete source-native windows, such as this week versus the median of the prior eight comparable weeks. Where history permits, compare the same seasonal window last year. For fast feeds, use matched daypart and weekday. Keep trailing seasonal, breaking, persistent and declining classes. Exclude known outages, bots and tracking changes from training and scoring.
3. Calculate per-source movement using a log lift and a robust anomaly score. With comparable current count `c`, baseline median `b`, and historical counts `x`, use `lift = ln((c+1)/(b+1))` and `z = (log1p(c) − median(log1p(x))) / max(1.4826 × MAD(log1p(x)), 0.2)`. Initial count-feed gate is `c >= 20` and `z >= 2`, both adjustable by source and geography. For niche low-volume B2B, allow a verified industry event or comparable authoritative observation to open a monitored topic. Index and rank feeds get source-specific gates. Never apply raw-count thresholds to indexed trends. With fewer than four comparable periods, label insufficient history and keep draft-only.
4. Compute a 0–100 **opportunity score** from 25% demand, 25% platform fit, 20% commercial intent, 15% timing and 15% readiness. Derive demand from calibrated source scores. Fit uses tenant product/category, geography and persona matches. Intent uses explicit behaviour and taxonomy. Timing reflects remaining actionable window. Readiness comes from available stock, partner coverage, capacity and approved assets. Missing readiness blocks activation. Store numeric inputs and reasons. Apply hard vetoes for out-of-market, unsupported claims, restricted topics, unavailable inventory or illegal targeting.
5. Independently compute 0–100 **confidence** from 35% source quality, 25% independent corroboration, 20% sample/coverage and 20% freshness. As a starting rule, cap a single source family at 69. Independent corroboration must be relevant to the same topic, market and time. Record actual coverage and uncertainty. Model certainty is excluded. Before release, calibrate all thresholds against labelled historical cases for the pilot platform.

| Score and confidence | Result |
|---|---|
| Score below 60, or confidence below 55 | Archive with reason. Reopen on new evidence. |
| Score 60–74, or confidence 55–74, with both above archive cutoffs | Watch or draft internally. No live change. |
| Score ≥75 and confidence ≥75 with no veto | Create a reviewable brief. External actions still require policy and channel approval. |
| Score ≥85 and confidence ≥85, with explicit tenant opt-in | Eligible for bounded automatic action in approved low-risk classes only. |

**Example.** A synthetic platform topic scores 80 demand, 95 fit, 85 intent, 90 timing and 80 readiness. Its opportunity score is 86.25. Confidence inputs of 90, 80, 70 and 90 yield 83.5. The output is a reviewable brief. A decision to send marketing or spend money still follows the channel policy. A topic can reopen after cooldown if the evidence or capability changes. Limit one active brief per platform, market and topic, use short validity windows and apply a per-asset cooldown to prevent oscillation.

### 6. The contract connecting all agents

Required Growth Brief v1 fields:

```json
{
  "schema_version": 1,
  "brief_id": "gb_104",
  "brief_version": 3,
  "tenant_id": "tenant_demo",
  "platform_id": "fixture_platform_a",
  "market": "CA",
  "locale": "en-CA",
  "topic_id": "simpler-team-onboarding",
  "opportunity_score": 86.25,
  "confidence": 83.5,
  "valid_from": "2026-11-10T10:00:00Z",
  "expires_at": "2026-11-17T10:00:00Z",
  "evidence_ids": ["ev_341", "ev_342"],
  "audience": "Small teams comparing onboarding options",
  "intent": "evaluate how to onboard a small team",
  "approved_claim_ids": ["claim_onboarding_01"],
  "cta_id": "cta_view_onboarding_guide",
  "allowed_channels": ["seo", "organic_social"],
  "prohibited_claims": ["guaranteed onboarding time", "unsupported third-party endorsements"],
  "objective": "qualified trial starts per eligible session",
  "experiment_id": "exp_91",
  "policy_version": 5
}
```

This is a synthetic payload. Require schema validation with enumerated scopes, safe lengths and rejected extra fields. IDs resolve server-side. The requested action specifies `tenant_id, platform_id, brief_id, brief_version, action_kind, channel, target_id, expected_target_revision, patch, evidence_ids, approved_claim_ids, approver_requirements, action_expiry, idempotency_key`. The authenticated tenant and role come from the session, never from a caller-provided ID alone. Server policies resolve targets, CTAs and allowed claims. Return typed errors for stale revisions, expired briefs, insufficient evidence and disabled connectors.

**Core API**

| Endpoint | Contract |
|---|---|
| POST /v1/observations:batch | Scoped ingestion. Return accepted, duplicate and rejected counts. Validate sizes, event time and registered source. |
| GET /v1/opportunities | Filter by platform, market, status and timeframe. Return score breakdown, evidence, coverage, last refresh and vetoes. |
| GET /v1/briefs:effective | Resolve approved current brief for platform, market, locale, channel and topic. Return version and expiry, or baseline. |
| POST /v1/actions | Submit a proposed patch or draft with an idempotency key. Reject unknown action types. |
| POST /v1/actions/{id}:approve | Bind approval to exact payload hash, target revision, policy and brief version. Editing invalidates approval. |
| POST /v1/actions/{id}:execute | Worker-only. Recheck policy, expiry, role, provider scope and live target revision immediately before execution. |
| POST /v1/actions/{id}:rollback | Restore an owned reversible asset where its current revision matches the system’s applied revision. Otherwise require a conflict review. |
| GET /v1/experiments/{id} | Report primary outcome, control method, guardrails, sample, data maturity and decision. |

**State machines.** Opportunities move from detected to watching, eligible or dismissed, then briefed or expired. An action moves from proposed to validated to pending approval or approved, then queued, executing and applied, failed or reconciliation required. Applied reversible actions can be rolled back. Approval, dispatch and expiry are auditable. Create the action and outbox event in the same transaction. Queue delivery is at-least-once. Derive a durable idempotency key from tenant, brief version, channel, action kind, target, base revision and canonical patch hash. Keep provider receipt and prior asset revision. After a timeout, reconcile against provider receipts or live state before retry. For irreversible sends without safe provider idempotency or lookup, halt uncertain jobs for review. Use capped retries, dead-letter handling and per-tenant quotas. Enforce unique active brief scope and target concurrency at write time. Re-authorise at execution after queue delay or policy change.

### 7. What changes on each channel

| Channel | Actions and boundaries |
|---|---|
| SEO | Match query intent to the existing canonical URL first. Draft additions, title and meta variants, internal links or FAQs based on original knowledge. Flag cannibalisation using query/page overlap and content similarity. Require editorial approval for new indexed pages, claims, canonical changes, schema, redirects or noindex. A live rollout must preserve sitemap, canonical and crawl status unless explicitly approved. Use normal discovery. Google's Indexing API is limited to eligible job postings and livestream pages. [S9–S11] |
| Content and organic social | Use the same proposition and approved claims, adapted to the channel. Draft or queue in an approval workflow. Check licence, brand fit, local vocabulary and CTA. No auto-replies by default. |
| Paid search and social | Prepare keyword groups, negatives, creative and geo restrictions. Human approval for campaign launches, spending changes and targeting policy. Use provider budgets and separate application-level caps. Define loss limits and pause authority in policy. |
| Lifecycle and CRM | Create segment suggestions and drafts. Apply tenant consent/eligibility and suppression at send time and bind the send to the approved list definition and audience cap. Apply the installation’s jurisdiction-specific rules for direct marketing and tracking. The ICO guidance is a UK example. [S12] |
| Landing pages and in-product placements | Prefer reversible feature flags on owned surfaces. Validate stock, service area, claims, accessibility and offer dates. Keep stable control assignment and prevent overlapping tests on the same slot. |

For the pilot, allow automatic creation of drafts and internal tasks only. A later tier may automatically run an approved, reversible owned-site CTA variant for up to 10% of eligible traffic, with a stop rule and rollback. Public posts, new indexed pages, email sends, audience exports, ad spend and legal or safety-sensitive claims remain approval-gated. Public marketing changes cannot always be undone. A sent email cannot be recalled by the engine.

### 8. Architecture, operations and security

**Default pilot stack:** Python and FastAPI, PostgreSQL for records, vector search via pgvector if available, a PostgreSQL-backed job and outbox worker, object storage for permitted raw snapshots, and the host platform's admin UI, authentication and deployment tooling. For optional self-hosted inference, use Qwen3-8B as an initial generation candidate and BGE-M3 as a multilingual embedding candidate, subject to license and performance checks for the pinned artifacts. Use llama.cpp on a development or lower-volume quantised deployment and benchmark vLLM with structured JSON output on GPU for higher concurrency. Version model weights, tokenizer, prompt template, schema and policy. [S13–S16]

If benchmarking a local deployment, start with a 24 GB GPU for a single 8B-class inference worker, controlled context and concurrency. This is a sizing hypothesis. Measure p95 end-to-end generation time, throughput and GPU memory with the actual quantisation, context and batch. A provisional pilot load is 10 tenants, 10,000 aggregate observations/day and up to 100 generated drafts/day. Reserve CPU processing for collection and scoring. Cache embeddings and generation against immutable tenant-scoped inputs. Batch nonurgent work, bound tokens, and apply per-tenant fair scheduling. Estimate monthly cost as GPU-hour rate × active hours, plus CPU/database/storage, transfer and connector fees. For host AI, budget against the host’s existing provider usage and limits. Keyless installation still has compute and operating costs.

**Storage and isolation.** Keep `source_registry`, `connector_state`, `observations`, `clusters`, `evidence`, `opportunities`, `brief_versions`, `claims`, `agent_runs`, `actions`, `approvals`, `outbox`, `experiments`, `outcomes` and `audit_events`. Put tenant/platform on tenant-owned rows, enforce composite tenant foreign keys and row-level security, and separate private embeddings and object prefixes. Use public reference data read-only. Add indexed event windows and unique idempotency indexes. Keep source text only as permitted. Set policy-controlled retention and deletion across raw text, logs, embeddings and backups.

**Trust boundaries.** Treat feed text, user content and scraped or imported text as untrusted data. The model reads sanitized excerpts and returns bounded JSON. The executor interprets only validated typed actions. Restrict outbound connector destinations, enforce safe redirects and response-size limits, use least-privilege internal scopes and keep provider secrets in the host or operator gateway, and sanitise HTML in previews. Carry evidence IDs to citations and validate that each factual marketing claim maps to an approved claim or verified fact. Sensitive, regulated or safety-critical claims go to a qualified human. Track reviewer, decision, evidence hash, model and prompt versions, policy, target revision, receipt and rollback. Do not share tenant demand, experiments or performance across businesses without a separate authorised agreement.

**Operations.** Instrument source lag, coverage, connector 429s, queue age, inference schema errors, rejected claims, changes per asset, conversion and guardrail breaches. The admin UI needs platform profiles, source status, scored opportunities, brief approval, side-by-side diffs, experiment dashboard, receipts and a channel freeze switch. Disable automatic execution for stale data, provider scope changes, a full dead-letter backlog or failed policy checks. Emergency freeze rejects new live work and prevents queued actions from starting. Handle in-flight actions by reconciliation. Test database restoration, worker replay and tenant kill switches before opening any automation tier.

### 9. Platform profiles and practical examples

| Platform type | Signal and action | Example success measure |
|---|---|---|
| Subscription software | Search queries and support topics show growing interest in team onboarding. Refresh the existing guide and draft a launch post using approved capabilities. | Qualified trial starts per eligible session |
| Online store | Searches and sales rise for a category with stock available. Propose an update to the category page and draft campaign copy. | Completed orders and contribution margin, with refunds as a guardrail |
| Marketplace or booking service | Demand rises in a region with verified provider capacity. Draft a local collection or availability page update. | Completed bookings per eligible session, with cancellations as a guardrail |
| Publisher or community | Internal search and reader behaviour show increased interest in a topic. Refresh an existing resource and draft a distribution post. | Qualified subscriptions, registrations or engaged visits |

These examples illustrate different configurations. A Platform Profile supplies the platform’s offer or catalogue, audience, supported markets and locales, topic vocabulary, capacity checks, approved claims and calls to action, available channels, forbidden topics, eligible events, outcomes and permitted asset IDs. Each installation validates those settings against its own policies and data.

### 10. Proving performance and learning

1. At brief approval, freeze hypothesis, baseline, primary outcome, segment, control method, minimum detectable effect or observation target, test window, guardrails and responsible owner. Keep spend, impressions and outcomes at the same grain. Distinguish observed correlation from estimated incremental lift.
2. For first-party surfaces and eligible campaigns, use a stable randomised assignment or holdout where possible. For SEO, use matched untreated URLs or topic clusters with comparable pre-trends and evaluate changes relative to those controls over mature windows. Avoid serving differing SEO content to crawlers. Mark observational SEO effects accordingly. If there is inadequate data or a site-wide confound, record inconclusive.
3. Watch cost per qualified outcome, conversion and unsubscribe/cancellation/refund rates where relevant. Use leading metrics for diagnosis. For live experiments, apply a pre-registered spend or traffic cap, and stop immediately on policy incidents or stock unavailability. Before scaling, require enough data, a credible positive result and no breached guardrail. Do not repeatedly peek and promote on noise.
4. Save action type, audience, model, evidence, test method, outcome, and negative findings. Propose playbook or threshold changes for review. Version them and rerun a historical benchmark before deploying. Do not autonomously retrain the model on outputs or edit policy.

### 11. Delivery sequence and release gates

A planning range for one pilot with existing authentication, CMS and analytics is **6–8 weeks**, with two engineers and part-time product/SEO ownership. This is an estimate. Legacy integration, clean event data, editorial capacity and provider approval can change the schedule.

| Stage | Build | Acceptance gate |
|---|---|---|
| 1. Foundation | Platform profile, tenant isolation, first-party import, public feed, snapshots and source health | Historical and fresh observations replay idempotently. Late or absent data stays unknown. Public and private access tests pass. |
| 2. Intelligence | Baselines, clustering, score/explanation, inference adapter, coverage indicators | Replay hand-labelled historical periods with as-of timestamps. Starting gate of ≥80% actionable precision in the top ten recommendations across multiple windows, with sample size and review decisions recorded. Validate cold starts, holiday spikes and multilingual examples. |
| 3. Agent integration | Brief SDK, one SEO agent, one marketing agent, draft-only workflow, approvals and audit | Both agents consume the same versioned brief. Old or revoked briefs cannot trigger an action. Prompt injection input produces no privileged action. Publishing credentials are absent from workers and prompts. |
| 4. Controlled live pilot | Owned CMS connector, previews, approved patches, control groups and monitoring | At least one approved and measured owned-site change completes end to end. Concurrent edits produce a conflict. Duplicate jobs produce one side effect. Freeze, rollback and timeout reconciliation pass. |
| 5. Expand | Approved social, CRM and paid adapters, more platforms and markets | Each connector has a scope and quota test. Approval binding, suppression and caps pass. Each tenant has a validated platform profile and measurable outcome. |

**Required release test fixtures:** seasonal peaks, sudden irrelevant viral news, four syndicated copies of one story, low-volume high-value procurement, a connector outage, a vendor returning partial data, conflicting geographies, cross-tenant IDs, malicious text embedded in a feed, stale content approval, concurrent CMS writes, retry after uncertain delivery, unavailable offers and a conversion signal generated by the platform’s own campaign.

**Reference pilot.** Select any willing host with usable first-party data, approved claims, a measurable outcome and a reversible test asset. Validate the workflow on two dissimilar host profiles. Use only sources relevant to each installation. A production host can start in shadow mode, then activate changes on a built-in owned page without a third-party integration.

**Host integration decisions still to confirm.** Map this contract to the actual identity provider and tenant boundaries, event taxonomy, current agents, deployment stack, CMS revision semantics, existing AI gateway capabilities or local inference budget, and platform approvals. Keep the brief and action contracts stable. This is a researched build specification. No host repository, credentials, traffic, actual model benchmark or production integration was supplied for verification.

## Part II · Implementation and distribution

### 12. One product with three installation paths

Build a standalone Growth Intelligence service. The platform integration is an HTTPS adapter, regardless of whether the host uses Node, Python, PHP or another stack. Each host remains the system of record for customer identities, assets, permissions, product claims and commercial outcomes. This service owns signals, scored opportunities, briefs, action proposals, experiment records and audit.

| Distribution | V1 scope | Operational boundary |
|---|---|---|
| Organisation-managed integration | Shared engine with a separate installation and credentials per platform and environment. Any conforming host can serve as the pilot. | Separate data and policy scopes. Stage and production use distinct deployment instances or equivalent hard environment separation. |
| Managed SaaS | First commercial release. Same containers and API, customer-specific organisation, installation, profile, credentials and entitlement. | Operator runs the service and provides managed inference without customer-supplied keys. External customers can also use an authorised host AI gateway, integrate their agents or use the standalone console. |
| Private deployment | Commercial expansion after managed pilot. Signed OCI images, documented Helm installation, pinned local models and optional disconnected distribution. | Customer hosts storage and can reuse an existing authorised AI gateway or run local inference. Disconnected mode uses local inference or templates. Separate support contract and upgrade window. An offline licence uses a local public verification key and needs no entitlement call home. |

V1 delivers automated first-party aggregation, first-party event and CSV ingestion, manual trend imports, an optional key-free allowlisted RSS reader, scored opportunities, platform profiles, a shared brief, SEO and marketing generation, review, native owned-page activation and rollback, conforming host adapters, action receipts and outcome reporting. Standalone also supports manual draft export and a recorded manual disposition. Live social, advertising and email execution, unattended new-page creation, model fine-tuning and white-label resale are separately released features. The full product includes owned-surface activation even when a customer has no connected CMS. A restricted draft-only trial must be labelled as such.

**Reference implementation:** Python 3.12, FastAPI, Pydantic, SQLAlchemy, PostgreSQL, Alembic, React admin console, PostgreSQL job/outbox worker, S3-compatible evidence storage and an internal inference gateway. Use lexical clustering initially with optional local embeddings. Add pgvector after benchmark if needed. Reference development install uses Docker Compose. Production packaging uses Helm and immutable image digests. Use tenant-scoped database rows and issuer-bound identities. The internal inference gateway selects host, deployment-supplied or deterministic mode according to installation policy. No implicit cross-provider fallback.

### 13. Build layout and ownership

| Repository path | Responsibility | CI check |
|---|---|---|
| contracts/ | Public OpenAPI, event and profile schemas, fixtures and compatibility rules | Lint, resolve references, schema-positive and negative fixtures, breaking-change check |
| services/api/ | Auth, scope resolution, profiles, ingest, read models, review and quotas | API and database integration tests |
| services/worker/ | Feed normalisation, scoring, brief jobs, inference client, outbox and execution | Time-travel scoring tests, queue and provider timeout tests |
| adapters/reference-cms/ | Fake CMS and production-facing adapter reference | Revision, read-back, replay and rollback conformance |
| sdk/python/ and sdk/typescript/ | Installation configuration, typed API calls, effective-brief resolution and baseline fallback | SDK tests against same contract and fake API |
| console/ | Platform switcher, source health, opportunities, briefs, preview/diff, approvals, results and operator controls | Browser journey for both installed and standalone use |
| services/owned-surface/ | Native public landing-page renderer, immutable page revisions, internal publishing adapter and first-party outcome collection | Publish approved revisions, observe outcome events and roll back with third-party egress blocked |
| deploy/ | Compose, Helm, migration jobs, manifests, SBOM, signed images and runbooks | Fresh install without new third-party credentials, upgrade, rollback, restore and no external inference egress in disconnected mode |
| tests/conformance/ | Golden tenant, fake CMS and negative test matrix | Repeatable end-to-end dry-run and publish/rollback |

Make the Python client the reference implementation. Generate types for TypeScript from the same OpenAPI source before onboarding the second host. Use additive optional fields for v1 response evolution and closed request schemas. Breaking request, state or behaviour changes require /v2 and a documented migration. Every model-facing prompt template and output schema gets an independent revision.

### 14. Install, connect and verify any host

1. **Provision.** Create an organisation for the business and a unique installation for each platform and environment. Set default locale, markets, data retention, entitlement, owner, region and quotas. Default mode is draft only and all write connectors disabled. Use deployment-supplied or deterministic inference until an administrator explicitly authorises an existing host AI binding.
2. **Bind identity.** Configure OIDC issuer and audience for console users. Issue short-lived installation-bound credentials separately to event producers, the host’s agent SDK, workers and CMS adapters. Admin maps authorised installation and role by verified subject or workload identity. No application-wide shared production token.
3. **Install the profile.** Import the product and approved claims, CTA destinations, topics, audience, locale, target assets, available capacity and primary outcome. Until a profile is active, accept observations but keep recommendations in shadow mode. Profile changes create immutable revisions and invalidate queued writes that depend on changed claims or policy.
4. **Map the data.** Map first-party events to canonical observation keys. Connect verified CMS assets by external ID through the host’s existing publishing service, if available. Configure a per-platform conversion definition and an eligible-session denominator. Import at least four comparable reporting periods before live recommendations. Sparse datasets can still create internal research tasks.
5. **Connect existing agents.** Before drafting, the host resolves the effective brief using market, locale, channel and topic. The SDK may cache within a 60-second ceiling and never beyond expiry. Pass the returned brief and claims as data alongside the existing agent instructions. On timeout, disabled entitlement, missing capability or no approved brief, use host baseline behaviour. Never use the cache as authority to write or publish.
6. **Run shadow mode.** Replay dated fixtures and compare manual judgement with the output. Check evidence rights, event duplicates, brand fit, current platform capabilities and geographic targeting. Console users can export drafts without granting any write access.
7. **Enable one reversible write.** Use the reference CMS adapter contract, grant patch-only scope for a test asset, verify read-back and rollback, and nominate separate author and approver roles. Enable each target class explicitly. An installation manifest or subscription alone never authorises publishing.
8. **Go live and transfer ownership.** Record the installation owner, upgrade ring, retention, emergency contact, health checks, baseline performance and support plan. Archive the conformance run. To uninstall, disable connections, revoke service identity, freeze jobs, export permitted records and schedule deletion. Published host content remains under host control.

**Minimum profile.** Supply a platform identifier, a primary market and locale, an approved offer or topic scope, one outcome definition, and at least one signal source. A new installation enables first-party ingestion and manual imports only. Sources and channels outside that baseline require explicit activation. Without a CMS, CRM, external analytics or host agents, use the included agents, native page surface and built-in outcome collection. Where a host does not provide its own outcome data, show native outcomes only. For business outcomes with no instrumented data, mark measurement unavailable. Every live action still requires an approved asset binding and the applicable review policy.

**Portability rule.** Keep country, language, category codes, source defaults, outcome names and platform slugs in profile or adapter configuration. Core services and shared SDKs must never branch on a platform slug or apply an industry-specific taxonomy by default. Shipping a sample profile or optional connector must not activate it on a new installation.

**Host AI adapter contract.** Implement `capabilities()` and `generate_structured(request)` against the platform’s existing server-side AI service. Add `embed(request)` only when supported. The request carries installation scope, request ID, purpose, approved model alias, schema revision, sanitised input, deadline and token limit. The response carries request ID, structured output, resolved model identifier and revision when known, usage and a typed status. Use the existing workload identity or a host-local service binding. Do not add provider-key fields to the module configuration, UI, prompts or logs. Route selection is per installation and server-authorised. Validate schema and claims after inference. On unsupported features, use lexical clustering or templates as applicable. On 429 or temporary error, honour the host retry window within the job deadline. After failure, switch to a different inference service only when that route and its data handling were approved for the installation. Otherwise use templates. Cache by installation, route, model and prompt/schema version, and keep trace and usage records without storing provider secrets.

**Host connector adapter contract.** The host pushes normalised signals through the existing observation endpoint. For writes, use the host-side adapter in §18, with the host checking target permissions and applying approved changes via its existing connector. The engine never receives an OAuth refresh token or provider key. Connector configuration stores a capability identifier and approved scope, never third-party secrets. Mark unsupported or revoked integrations unavailable, with a visible import/export alternative where applicable.

### 15. Persistence and isolation

Use UUID primary keys and UTC timestamps. The installation is the v1 tenancy boundary. Carry installation_id on every private row, including evidence, prompts, metering and logs. Enforce composite foreign keys (installation_id, referenced_id). Public evidence is read-only and copied or referenced under a recorded licence. Never select a private row before resolving installation authority.

| Relation | Minimum implementation constraints |
|---|---|
| organisations / installations / grants | Unique organisation slug and (organisation, platform slug, environment). Explicit per-install grants and disabled timestamp. |
| platform_profile / claim_revision / asset_binding | Unique (installation, revision) or (installation, external asset ID). One active profile revision. Asset binding holds allowed target and allowed fields. Claim stores source and validity. |
| source_registry / observations / evidence | Unique (installation, source, source record, revision). Immutable evidence pointer, hash, permission and coverage. Same key/different hash returns conflict. |
| topic / opportunity_snapshot / brief | Opportunity score frozen by (installation, topic, window end, scoring version, input digest). Unique (installation, brief ID, revision). Active pointer keyed by (installation, market, locale, topic). |
| action / approval / job / outbox | Unique (installation, idempotency key), approval bound to exact canonical payload hash and revision. Job claim uses a lease, attempt and fencing token. External receipt unique within connector scope where available. |
| experiment / outcome / usage_ledger / audit | Deduplicate conversion using a pseudonymous outcome ID per installation. Usage has a unique logical work ID. Audit is append-only for application roles, with restricted read/export. |

Use a migration role distinct from the runtime role. Runtime SQL roles have no BYPASSRLS and do not own tenant tables. Enable and force row-level security on scoped tables. Set tenant context within each transaction using a parameterised server-controlled setting and discard it at transaction end. Workers set scope per claimed job after verifying its installation. Integration tests must exercise the actual runtime role and a pooled connection reused by a second tenant. Bind object prefixes and evidence URLs to authorised scope. [S17]

Default operator policy is 30 days for permitted raw signal payloads, 180 days for observations, and 365 days for operational audit and brief history, subject to customer configuration and applicable retention obligations. Delete or anonymise related evidence, embeddings, logs and metering references as appropriate. Document the backup expiry window. After restore, replay pending erasures and quarantine external execution until reconciliation is complete.

### 16. API rules and reference payloads

Use /v1. Send installation scope via the scoped credential. A multi-install user selects an installation explicitly and the API checks its grant before accessing records. Bearer tokens are issued with matching issuer, audience, expiry and scope. Use a separate installation header only for explicitly granted multi-install identities. All mutation requests have a request ID and Idempotency-Key, scoped by installation and operation. An identical replay returns the stored response. A replay with a different canonical payload returns 409. Keep deduplication records for at least the maximum retry horizon, 30 days by default.

Use JSON, RFC 3339 UTC times, bounded collections and cursor pagination. Defaults are 50 list items, maximum 200, observations up to 500 items and 1 MiB per request. An invalid batch fails atomically with indexed validation errors. A valid batch returns counts for accepted and duplicate events. Source windows use inclusive start and exclusive end. Use 200 on successful reads, 201 on creation, 202 on accepted work, 204 when effective brief has no eligible match, 409 for conflict, 422 for invalid payload, 429 plus Retry-After for quota and 503 plus Retry-After for temporary service unavailability. Unknown or cross-installation entity IDs return 404. A known identity lacking action authority returns 403.

| Operation | Scope | Behaviour |
|---|---|---|
| PUT /v1/profile | profile:write | Save validated profile with If-Match revision. Return immutable new revision. Missing If-Match returns 428 except initial create, which uses If-None-Match: *. |
| POST /v1/observations:batch | events:write | Ingest known sources. Reject personally identifiable text or unregistered sources. Mark late observations for scoring within the configured correction horizon. |
| GET /v1/opportunities | trend:read | List with market, locale, topic, status, source health, score and evidence references. |
| POST /v1/briefs | brief:write | Create a draft from a specific eligible opportunity snapshot. Bind the active profile and policy revision. |
| POST /v1/briefs/{id}:approve | brief:approve | Approve an exact revision with If-Match. Atomically replace the active pointer for the same scope. |
| GET /v1/briefs:effective | brief:read | Resolve one approved, fresh brief for market, locale, topic and channel. Return 204 when unavailable and 503 on transient dependency failure. |
| POST /v1/actions | action:propose | Validate approved brief, asset and action kind. Create a draft without changing the asset. |
| POST /v1/actions/{id}:approve | action:approve | Bind approval to canonical payload hash, target revision and policy. Reject self-approval for a live change. |
| POST /v1/actions/{id}:execute | action:execute | Execution service only. Queue approved action and its outbox record atomically. |
| POST /v1/actions/{id}:rollback | action:rollback | Queue compensating write if supported and approved. Restore only fields owned by the action. |
| GET /v1/actions/{id} | action:read | Return state, revisions, receipt, policy and reconciliation status. |
| POST /v1/outcomes:batch | outcome:write | Ingest deduplicated eligible sessions and conversions using approved definitions. |
| GET /v1/experiments/{id} | experiment:read | Return experiment and evidence of maturity, lift estimate where feasible and guardrail outcomes. |

Error example

```json
{
  "code": "TARGET_REVISION_CONFLICT",
  "message": "The target changed after the action was approved.",
  "request_id": "req_demo_173",
  "retryable": false
}
```

Version 1 installation profile fixture. This fixture matches the Growth Brief in §6. Its platform identifier, market, locale, claims, CTA and outcome are sample data only. Production values come from the installation. Use ISO alpha-2 market codes for country-targeted briefs. Reserve ZZ for explicitly global, non-country-targeted briefs. ZZ never matches a country-targeted lookup by default.

```json
{
  "schema_version": 1,
  "platform_slug": "fixture_platform_a",
  "mode": "draft_only",
  "default_market": "CA",
  "default_locale": "en-CA",
  "allowed_channels": ["seo", "organic_social"],
  "allowed_asset_ids": ["asset_onboarding_page"],
  "approved_claim_ids": ["claim_onboarding_01"],
  "allowed_cta_ids": ["cta_view_onboarding_guide"],
  "outcome": {"conversion": "qualified_trial_started", "denominator": "eligible_session"},
  "timezone": "America/Toronto"
}
```

The SQL model stores an immutable approved claim revision. The example contains only claim references. The agent receives the approved claim text via a scoped lookup or as part of the resolved read model. A channel adapter cannot act on a generic free-text instruction.

### 17. Intelligence implementation and default schedules

Scoring v1 uses the formula and initial thresholds in §5. Normalise source-specific demand scores to 0–100 using a calibrated monotone mapping. Reference fixture mapping for count feeds is clamp(50 + 10 × z, 0, 100) after the volume and history gates. Compute the demand component as the median of the highest-weight valid observation per independent source family, using the most recent complete window for that observation and deterministic tie-break by source ID. Each profile explicitly maps permitted topic and market to 0–100 fit and intent values. Set readiness to 0 and veto live activation when stock, claims or target availability fails. Fit and intent can be suggested by the selected inference route but take effect only from a verified mapping. Score using full precision and round only for display.

To make v1 confidence reproducible, set source quality to the mean of configured 0–100 quality scores for the contributing families. Assign corroboration 0 for one independent family, 80 for two and 100 for three or more. Let coverage be the mean valid observed/expected window ratio times 100, using a documented per-source coverage proxy where counts are unavailable. Freshness is the mean per-family value of 100 at window close tapering linearly to 0 at its configured stale-after limit. For scheduled feeds calculate expected publication delay separately before ageing. Missing required coverage keeps the result in watch/draft mode. Context-only sources cannot satisfy the corroboration minimum. Do not merge distinct geographies because an embedding is similar.

| Job | V1 default |
|---|---|
| First-party ingest | Push in near real time or import hourly. Hourly scoring over complete source windows. |
| Optional RSS or public feed | Enable per installation. Use source-specific cadence, cursor and lookback. Apply source-specific deduplication where supported. |
| Manual imports | Trigger normalisation on receipt and show observation period and age. |
| Scoring | Run per installation, market and topic. Reprocess corrections for seven days. Freeze original experiment evidence and create a superseding snapshot. |
| Brief expiry | Default seven days for persistent topics and 24 hours for breaking topics. Check expiry on every effective-brief lookup and write. |
| Pruning and reconciliation | Reconcile unresolved CMS writes every five minutes up to a 30-minute escalation window. Run retention jobs daily. |

If no authorised inference route is available, persist detection and deterministic prioritisation. Emit a structured template brief in draft status with evidence and a clear degraded flag. Inference only returns schema-validated proposals, with a 60-second draft timeout and up to one retry for invalid structured output. Exhaustion creates a review task. Models, prompts and token limits are configurable per deployment. Keep compute quotas by installation and log output quality without storing unnecessary sensitive prompts.

### 18. Write safety, adapter interface and recovery

V1 live execution supports only seo_metadata_patch and owned_content_patch on prebound assets. Draft artefacts support seo_page_draft and organic_social_draft. Use a typed field allowlist. Title, description and an approved named body block can be patched. URL, redirects, canonical, robots, structured data, permissions and template code are out of scope for v1 automation. Display a before/after preview and the expected target revision. Manual publishing/export is recorded separately from automated execution.

| Adapter method | Semantics |
|---|---|
| get_asset(asset_id) | Return stable ID, current revision, permitted field values and content hash. |
| apply_patch(asset_id, expected_revision, fields, action_key) | Atomic compare-and-set of all allowed fields. Persist action key with resulting revision and provider receipt. Return conflict if expected revision differs. |
| get_receipt(action_key) | Return applied, not found or unknown with provider revision if known. Adapters lacking safe reconciliation are draft-only. |
| restore_patch(asset_id, expected_applied_revision, previous_fields, rollback_key) | Atomic compare-and-set to restore the previous values. Preserve unrelated edits. Conflict goes to manual resolution. |

Implementation order for a live write

1. Validate schema, scope, asset allowlist, current entitlement, installation freeze, fresh active brief, evidence and approved claims. Write action and approval requirements.
2. Approver signs canonical hash and expected revision through console. Edits, expiry or relevant policy/profile changes invalidate approval. Author and approver must differ.
3. In one transaction, transition approved → queued and insert outbox. Worker leases the job, then runs all execution-time guards again. A freeze blocks new claims and new provider calls for work that has not passed its final check. In-flight calls reconcile on return.
4. Save the previous field values and expected revision before calling the adapter. Send the same deterministic action key on any safe retry. Use a bounded retry with jitter for transient service failures. On ambiguous completion, persist reconciliation_required and query receipt before another write.
5. Save applied revision and provider receipt before marking applied. If a database crash follows provider success, recover from the provider receipt using the original key. On conflict, leave the asset intact and ask for a new approval against the new revision.
6. Rollback is a new logged, authorised compensating action. It must match the applied revision or fail safely. A freeze still permits authorised rollback. Expired commercial plans retain rollback and export access during the recovery window.

**Native owned-surface adapter.** Ship a real page renderer at a deployment-managed address with SEO title, description and allowlisted content blocks. Each asset stores installation ID, public slug, draft revision, active revision, approved content blocks and active campaign variant if applicable. Apply approved changes to an existing native page using the same adapter interface. Support one SEO metadata patch and one marketing headline or CTA copy patch. A first native page must be explicitly created and approved by an editor. Keep drafts and previews inaccessible to the public. Public read serves only the active revision. Use compare-and-set and persist the receipt atomically with the revision change. A public marketing draft can also be exported for channels that have no available write path.

**Native measurement.** Capture first-party aggregate eligible page visits and configured CTA completion events from the native surface without an external analytics tag. Apply the installation’s privacy policy, minimise identifiers and suppress small cohorts in reports. Record variant, action or revision ID, denominator and outcome window. Count CTA outcomes as CTA outcomes. Use downstream orders, leads or other conversions only if their data is actually received through first-party ingestion. Show counts, rates, guardrails and sample status. Use an authorised first-party holdout where enabled. If assignment is unavailable, compare reporting windows and label the result observational. Use measurement to recommend continuing, revising or rolling back the approved change. A new live change follows the existing approval policy.

### 19. Commercial control plane

Keep customer plans in data, with stable feature keys and server-side limits. Suggested plan structure below is a packaging decision for discovery, without a published price or committed billing integration.

| Package | Unit of sale | Proposed v1 limits and feature scope |
|---|---|---|
| Internal | Internal allocation per platform and environment | Usage attributed to installation. Default pilot limits are configurable. |
| Managed Starter | Subscription per production installation | 1 market, 2 channels, 3 reviewers, 50 successful generated drafts per month, CSV and public feeds, included inference, approved native page publishing, basic native outcome reports and draft export. |
| Managed Growth | Subscription per production installation plus additional capacity | 3 markets, 4 configured channels, 10 reviewers, 500 successful generated drafts per month, supported integrations, approved CMS writes and experiments. Channel execution remains limited to the supported release. |
| Private / Enterprise | Annual licence per customer deployment plus agreed implementation and support | Private hosting, agreed installation quotas, SSO, deployment assistance and contracted support. Offline packaging is separately acceptance-tested. |

**Entitlement:** record customer, plan revision, installation count, usage caps, effective time and status. Statuses are trial, active, grace, suspended and ended. Provisioning must be idempotent by external customer/order reference. Billing adapter records authoritative, verified events with unique provider event IDs, then reconciles subscription state before changing access. Late events cannot overwrite a more recent provider state. For v1 pilots, use admin-managed entitlements and invoices; introduce payment-provider automation after commercial terms are fixed.

**Metering:** count one completed draft per unique logical generation ID, never attempts or template fallback. Reserve quota atomically before generation, settle once on successful persist and release on final failure/expiry. Use UTC calendar months for the reference plans. Record SKU, plan revision, time, quantity and work ID. Permit customer usage export. Exclude payload content from billing records. At quota return 429 with usage reset timestamp. Default to hard caps with no automatic overage charges.

**Suspension:** grace and suspended installations become read/export/rollback only and may retain baseline SDK response. Stop ingestion and draft generation on suspension. Ended installations have an agreed export and recovery window, proposed at 30 days, then follow the configured deletion policy. For offline, use an asymmetric signed entitlement containing deployment ID, features, limit, expiry and key ID. Validate locally with the product’s public key. Keep private signing keys outside shipped images. Supply a recovery window and test expiry, key rotation and clock drift without forced data loss.

**Sale readiness:** before contracting, confirm the software and model redistribution rights for each pinned artifact, ship required licence texts and notices, produce an SBOM and model manifest, and record which feed licences allow use in a paid product. Keep third-party content exports subject to permitted use. Define onboarding, support boundaries, data export/deletion, service objectives and customer responsibility for permissions on existing channel access. No customer-supplied provider credentials are required for the product. Validate prices from delivery effort, inference cost, support load, market demand and paid pilots. Price, margins and private release dates are commercial decisions. [S13–S16, S18]

### 20. Build order and operational targets

| Gate | Deliverable and exit evidence |
|---|---|
| G1 · Contract and clean install | A fresh reference installation starts without external provider credentials, serves the API, uses bundled or deployment-supplied inference, shows both proposed actions and denies writes until approval. Schema fixtures and two-install identity tests pass. |
| G2 · Reference pilot | A selected host uses verified first-party events and approved product claims. One SEO and one marketing agent resolve the same approved brief. One approved owned CMS change and rollback complete end to end. Capture reviewer outcomes and latency. |
| G3 · Portability | Integrate two dissimilar host profiles using the same core build, the published API and conforming adapters or standalone imports. Require different locales, outcome types and source mixes. One must run without a third-party signal feed. No host-specific core fork. Verify cross-installation isolation. |
| G4 · First external sale | Standalone console or SDK onboarding, account lifecycle, quota, customer data export, backup restore, signed images, licence review and support runbook pass. Run an external design partner in draft-only mode before enabling their write connector. |
| G5 · Private deployment | Clean private installation and rolling upgrade using only distributed images and pinned assets. Disconnected mode works with external egress blocked using local inference or deterministic drafts. Offline entitlement, renewal, and export tested. |

Initial engineering targets for validation on the reference load in §8: p95 effective-brief read under 300 ms excluding host network, ingestion acknowledged under two seconds for a 500-item batch, 95% of queued draft jobs start within five minutes, p95 model draft under 60 seconds on the selected inference route under the pilot load, managed pilot RPO 24 hours and RTO four hours demonstrated by restore. No contractual SLA until operational history supports it. Alert on missed source freshness, queue age over five minutes, unusual rejection rates, ambiguous writes and unexpected entitlement transitions. After restore or deploy rollback, disable live writes until reconciliation verifies queued and previously applied actions.

**Hard release gate for the full product.** Run a clean install with no customer provider secrets, no host AI, no external analytics, no OAuth and no third-party connector. Block third-party egress and use the local deployment inference profile. Simulate eligible first-party activity, generate one shared brief and actual model output, approve and apply SEO and marketing changes to the native public surface, capture eligible visits and configured outcomes, produce a measurement report and restore the prior revision. Assert that no new keys were requested, no external destination was contacted and audit entries connect every stage. Separately verify host-AI reuse and automated ingestion from a permitted public feed with its explicitly enabled network access. A draft-only run does not satisfy this release gate.

### 21. Reference conformance scenarios

| ID | Given and when | Expected |
|---|---|---|
| TI-01 | Fixture ingestion replayed twice, then same source version with altered value | Original persists once. Exact replay returns duplicate. Changed hash returns 409. |
| TI-02 | Worker, SDK and operator for installation A request installation B data | Zero data returned. Cross-scope entity IDs resolve as 404. Database runtime role also denies access directly. |
| TI-03 | The isolated scorer receives the component inputs in §5 and returns 86.25 opportunity and 83.5 confidence | Reviewable brief only. Neither score is rounded before decision. |
| TI-04 | Three syndications of one story and a stale external feed | One source family only. Freshness and corroboration rules prevent automated activation. |
| TI-05 | Approved brief expires while cached and an action is already queued | SDK falls back to baseline. Executor denies the queued action. |
| TI-06 | Model input contains instructions to bypass approvals or change spend | Output cannot produce a privileged action. Invalid JSON or unknown action kind is rejected. |
| TI-07 | Provider succeeds then worker crashes before persisting result | Receipt reconciliation recovers a single CMS side effect. |
| TI-08 | Editor changes the target after preview or after publication | Pre-write or rollback conflict protects the editor’s work. |
| TI-09 | Two concurrent requests compete for the last plan generation | At most one reservation succeeds. Duplicate attempts never double-charge. |
| TI-10 | New installation uses self-hosted weights and rejects outbound model traffic | Detection, brief approval and drafts work. Simulated model failure still produces template drafts. |
| TI-11 | Trial expires with queued work and an existing reversible change | No new live writes. Export and authorised rollback work in the recovery window. |
| TI-12 | A host is onboarded with CRM consent or customer-level information absent | Aggregate first-party mode works. No direct-marketing execution path exists in v1. |
| TI-13 | Plan upgrade arrives after a newer billing event, and provider replays both | Reconciliation preserves the current authoritative plan. No duplicate provisioning or usage. |
| TI-14 | A competitor’s syndicated announcement matches the same keywords as the host product but has unrelated intent | Cluster stays separate or receives an intent veto. No misleading claim or audience assignment. |
| TI-15 | A customer requests erasure, then a backup is restored | Erasure log is reapplied and exports no longer expose deleted tenant evidence. |
| TI-16 | A new installation has no CMS, external signal provider or existing host agent, and imports enough permitted aggregate data | The console produces a reviewable draft or deterministic template as evidence permits. Drafts are exportable. Native writes stay disabled until the first page and action are approved. |
| TI-17 | The same build runs the subscription-service and storefront fixtures with different locales, outcomes and source mixes | Each installation uses its own taxonomy, scope, claims and conversion definition. Neither needs a platform-name condition in core logic. |
| TI-18 | Host AI is already configured and the module is installed with no provider secrets | Structured generation succeeds through the host gateway. Module settings, prompts and logs contain no provider credentials. |
| TI-19 | Host AI is unavailable or unauthorised and no alternate inference route is approved | Detection, scoring and templates continue. No request reaches an unapproved provider. |
| TI-20 | One host has an existing publishing connector and a second has none | The first can apply an approved update via its own adapter without handing over credentials. The second can publish to an approved native page and export drafts. External account writes remain unavailable. |
| TI-21 | Clean install with no provider secrets or third-party egress receives qualifying first-party signals | Included local inference produces real output. The approved SEO and marketing revisions publish on the native page. First-party visits and outcomes appear in the report. Rollback restores the previous revision. |
| TI-22 | A third-party feed or destination needs an access token that the installation does not already have | Report unavailable with the reason, do not prompt for a new credential, and complete the owned-surface workflow with available evidence. |

### 22. Developer start and remaining integration inputs

**First implementation ticket:** create the reference repository with OpenAPI, fixture tenants, one local database migration, event ingest, an outbox worker, and a fake CMS adapter. Run two synthetic installations with different business models. Fixture A is a subscription service using Canadian English and qualified trial starts. Fixture B is a storefront using French in France and completed orders. Run detection, briefing, drafting, approval, application and rollback on the same build using fake CMS bindings. Verify isolation and run Fixture B without public feeds, using independent first-party source families such as internal search and completed-order aggregates. Shared provenance must still be deduplicated. Run one fixture against a fake host AI gateway and the other using bundled local inference. Exercise the deterministic fallback separately. Implement the native owned-surface adapter and prove the full release gate in §20 before optional third-party connector development.

**Inputs required from each platform owner:** repository and deployment owner, identity issuer, host agent entry points, existing AI service interface and permitted model capabilities if available, events and sample data, CMS revision and webhook semantics, approved product claims and CTAs, conversion definitions, market and locale, owner for reviews, environment and data region, and write/rollback contact. If host write inputs are unavailable, run the same workflow on the native owned surface. For host assets with no supported adapter, use reviewed draft export.

**Release definition:** developer-ready for building the reference service against these contracts. Host-specific adapters, actual provider behaviour, model benchmark, customer pilot and commercial terms are implementation and release work. Proposed tiers and limits require product approval before publication.

### Appendix A · Machine-readable agent contract

Save the YAML block below as `contracts/growth-brief.v1.schema.yaml`. It defines the payload returned by the effective-brief endpoint. Use §6's JSON as the canonical positive fixture. The transport supplies the authorised installation identity. The tenant_id and platform_id fields in this payload identify the issuing scope and must match the installation binding in the SDK. Agents must treat this as a closed object. Adding optional read-model fields elsewhere is allowed. Changing this object requires a new schema version, with SDK compatibility documented before activation.

```yaml
$schema: https://json-schema.org/draft/2020-12/schema
title: Growth Brief v1
type: object
additionalProperties: false
required:
  - schema_version
  - brief_id
  - brief_version
  - tenant_id
  - platform_id
  - market
  - locale
  - topic_id
  - opportunity_score
  - confidence
  - valid_from
  - expires_at
  - evidence_ids
  - audience
  - intent
  - approved_claim_ids
  - cta_id
  - allowed_channels
  - prohibited_claims
  - objective
  - experiment_id
  - policy_version
properties:
  schema_version: {const: 1}
  brief_id: {$ref: '#/$defs/id'}
  brief_version: {type: integer, minimum: 1}
  tenant_id: {$ref: '#/$defs/id'}
  platform_id: {$ref: '#/$defs/id'}
  market: {type: string, pattern: '^[A-Z]{2}$'}
  locale: {type: string, minLength: 2, maxLength: 35}
  topic_id: {$ref: '#/$defs/id'}
  opportunity_score: {$ref: '#/$defs/score'}
  confidence: {$ref: '#/$defs/score'}
  valid_from: {$ref: '#/$defs/utc'}
  expires_at: {$ref: '#/$defs/utc'}
  evidence_ids: {$ref: '#/$defs/id_list', minItems: 1}
  audience: {$ref: '#/$defs/short_text'}
  intent: {$ref: '#/$defs/short_text'}
  approved_claim_ids: {$ref: '#/$defs/id_list'}
  cta_id: {$ref: '#/$defs/id'}
  allowed_channels:
    type: array
    minItems: 1
    maxItems: 6
    uniqueItems: true
    items:
      enum: [seo, organic_social, sales_enablement, owned_placement, lifecycle, paid]
  prohibited_claims:
    type: array
    maxItems: 32
    uniqueItems: true
    items: {$ref: '#/$defs/short_text'}
  objective: {$ref: '#/$defs/short_text'}
  experiment_id: {$ref: '#/$defs/id'}
  policy_version: {type: integer, minimum: 1}
$defs:
  id: {type: string, pattern: '^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$'}
  score: {type: number, minimum: 0, maximum: 100}
  utc: {type: string, format: date-time, pattern: 'Z$'}
  short_text: {type: string, minLength: 1, maxLength: 500}
  id_list:
    type: array
    maxItems: 64
    uniqueItems: true
    items: {$ref: '#/$defs/id'}
```

**Runtime rules beyond JSON Schema:** require valid_from < expires_at, require valid_from ≤ now < expires_at, resolve evidence and claims by installation scope, require channel membership and reject a policy mismatch. A schema-valid object can still fail these checks. Enforce string and asset constraints server-side before writes. Validate locale against the installed locale set and market against the active profile. Start with two supported draft routes in v1. The broader channel enum reserves stable routing values for future adapters and makes no feature available on its own. The §6 fixture uses the initial SEO and organic social draft routes. Other route values remain unavailable until an adapter and policy explicitly enable them.

**OpenAPI delivery check.** During G1, publish the API specification for §16 at /openapi.json, including all request/response schemas, authentication and role requirements, error envelopes, headers and examples. Import the Growth Brief schema under components and validate every positive and negative fixture in CI. Generate SDK types from that versioned file. Keep tenant resolution, CMS side effects and quota semantics in integration tests. Static schema validation alone cannot prove them.

### Sources

- [S1 · Google Search Analytics API](https://developers.google.com/webmaster-tools/v1/searchanalytics) and [usage limits](https://developers.google.com/webmaster-tools/limits)
- [S2 · Google Trending Now exports](https://support.google.com/trends/answer/3076011?hl=en-GB) and [Google Trends data methodology](https://support.google.com/trends/answer/4365533?hl=en)
- [S3 · Google Trends API alpha](https://developers.google.com/search/apis/trends)
- [S4 · Find a Tender data and API documentation](https://www.find-tender.service.gov.uk/Developer/Documentation)
- [S5 · Wikimedia pageview analytics](https://doc.wikimedia.org/generated-data-platform/aqs/analytics-api/reference/page-views.html) and [access policy](https://doc.wikimedia.org/generated-data-platform/aqs/analytics-api/documentation/access-policy.html)
- [S6 · Google Ads Keyword Planner historical metrics](https://developers.google.com/google-ads/api/docs/keyword-planning/generate-historical-metrics)
- [S7 · YouTube videos list API](https://developers.google.com/youtube/v3/docs/videos/list)
- [S8 · TikTok Creative Center trends](https://ads.tiktok.com/creative/creativeCenter/trends?LanguageId=1)
- [S9 · Google Search guidance on generated content](https://developers.google.com/search/docs/fundamentals/using-gen-ai-content)
- [S10 · Google Search spam policies](https://developers.google.com/search/docs/essentials/spam-policies)
- [S11 · Google Indexing API scope](https://developers.google.com/search/apis/indexing-api/v3/quickstart)
- [S12 · ICO electronic mail marketing guidance](https://ico.org.uk/for-organisations/direct-marketing-and-privacy-and-electronic-communications/guidance-on-direct-marketing-using-electronic-mail/)
- [S13 · Qwen3-8B model](https://huggingface.co/Qwen/Qwen3-8B)
- [S14 · BGE-M3 model](https://huggingface.co/BAAI/bge-m3)
- [S15 · llama.cpp](https://github.com/ggml-org/llama.cpp)
- [S16 · vLLM structured outputs](https://docs.vllm.ai/en/latest/features/structured_outputs/)
- [S17 · PostgreSQL row security](https://www.postgresql.org/docs/current/ddl-rowsecurity.html)
- [S18 · Apache License 2.0](https://www.apache.org/licenses/LICENSE-2.0)
