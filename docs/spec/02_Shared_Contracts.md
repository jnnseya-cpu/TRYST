# 02 — TRYST Shared Contracts v1.1

Part of the [TRYST v1.1 baseline](00_README.md). This document is the **single source of truth** for everything both backend ([03](03_Backend.md)) and frontend ([04](04_Frontend.md)) depend on. Change it first ([00 §4](00_README.md#4-change-control-stabilisation-rules)).

---

## 1. Conventions

| Topic | Rule |
|---|---|
| Contract formats | Edge API: **OpenAPI 3.1** (`contracts/openapi/`). Internal gRPC, the agent bus and events: **Protobuf** in the schema registry (`contracts/proto/`). Generated client and server types; no hand-written DTOs ([D-07](06_Decisions_and_Changes.md#2-decision-log)). |
| IDs | UUIDv7, opaque, never sequential or enumerable. Prefixed in logs only (`usr_`, `prf_`, `thr_`). |
| Time | RFC 3339 UTC on the wire. Availability windows carry an IANA `tz`. |
| Money | Integer minor units + ISO 4217 (`{"amount": 2499, "currency": "GBP"}`). |
| Location | **Geohash-5 (~5 km) or coarser** on the wire and at rest. Exact coordinates are sent to the server **only** by the Panic flow (§5.8). |
| Age | `age_band` only (`18-24`, `25-29`, `30-34`, `35-39`, `40-44`, `45-49`, `50-59`, `60+`). No DOB anywhere. |
| Free text | Everything a counterparty writes (bios, messages, agent turns) is **untrusted input**: schema-validated, never executed or interpreted as instructions. |
| Versioning | URL major version (`/v1`). Additive changes only within a major version. Enum additions are additive; clients must tolerate unknown values. |

---

## 2. Glossary and enumerations

### 2.1 Glossary

| Term | Definition |
|---|---|
| **Member** | A person with a TRYST account (one human, one account). |
| **Entity** | The matchable unit: a `solo` profile or a `couple` profile. |
| **Couple profile** | An entity linking two members, each verified to V2 and each co-signing from a distinct device (`VC`). |
| **Intent** | A costed outbound expression of interest from one entity to another. |
| **Mutual intent** | Both entities have sent an intent to each other. In P2+ it triggers a handshake. |
| **Handshake** | Envoy-to-Envoy negotiation over a fixed schema (§6). |
| **Brief** | The asymmetric, agent-generated summary each side receives after a handshake. Always labelled as machine-generated. |
| **Thread** | An E2EE conversation (MLS group) between the entities' members. Opens on double accept (P2+) or mutual intent (P1 control). |
| **Keys** | Prepaid credits. Spent on intents; refunded in full on reply. |
| **Discretion Policy** | A member's versioned privacy configuration, enforced on every read path (§6.4). |
| **Reveal grant** | A recipient-specific, expiring, revocable permission to view a media item or move up the reveal ladder. |
| **ExclusionRing** | A set of bidirectional, silent suppression pairs (PSI contacts, heuristics, manual). |
| **Exclusion Identifier / Ban anchor** | An irreversible one-way identifier kept after deletion for permanently removed members. The **declared exception** to erasure. |
| **Burn** | Discretion control: wipe local data, revoke device keys, sign out everywhere. |
| **Panic** | Safety control: escalate to emergency services with live location. **Never the same control as Burn.** |
| **QCAM** | Quality Connections per Active Month (north star; [01 §13](01_Product.md#131-north-star-and-kpis)). |

### 2.2 Segments

`segment ∈ {S1, S2, S3, S4, S5, S6}` — definitions in [01 §5.1](01_Product.md#51-segments). A solo entity is S1, S2, S4 or S6; a couple entity is S3 or S5.

### 2.3 Intent shapes and modes

| `intent_shape` (data) | Mode label (UI) | Entity pairs allowed |
|---|---|---|
| `one_off` | SPARK | solo↔solo |
| `recurring` | EMBER | solo↔solo |
| `ongoing` | EMBER | solo↔solo |
| `exploratory` | OPEN | any, subject to `seeking` |
| `third` | THIRD | couple(S3)↔solo(S4) |
| `couple` | QUAD | couple(S5)↔couple(S5) |

### 2.4 Other enumerations

| Enum | Values |
|---|---|
| `structure` | `attached_undisclosed`, `attached_disclosed`, `open_relationship`, `unattached`, `couple_seeking_third`, `couple_seeking_couple`, `prefer_not_to_say` |
| `visibility` | `public` (discoverable to eligible), `matched` (visible only to matches), `incognito` (discoverable only after own outbound intent) |
| `veto_mode` (couple) | `either` (either partner can decline), `both` (both must accept before an intent or decision counts) |
| `handshake_decision` | `pending`, `accept`, `decline`, `amend` |
| `intent_state` | `sent`, `queued_inbound_cap`, `delivered`, `replied`, `declined`, `expired`, `refunded` |
| `desire_tag_level` | `yes`, `curious`, `no` |
| `consent_kind` | `art9_processing`, `media_view`, `meet`, `couple_link`, `retention`, `conduct_attestation`, `safety_training_use`, `biometric_login` |
| `reveal_scope` | `alias`, `blurred_gallery`, `photo_set`, `voice`, `video` (the reveal ladder, in order) |
| `report_category` | `minor`, `ncii`, `sextortion`, `coercion`, `threat_violence`, `stalking`, `romance_fraud`, `commercial_sex`, `harassment`, `impersonation`, `protected_trait_harassment`, `exposure_threat`, `other` |
| `enforcement_action` | `warn`, `shadow_limit`, `suspend`, `permanent`, `law_enforcement_referral` |
| `unsafe_ground` | `g1_self_declared`, `g2_corroborated_reports`, `g3_court_order`, `g4_law_enforcement`, `g5_model_confirmed`, `g6_false_attestation` |
| `guardian_reason_code` | `m6a_coercive_control`, `m6b_boundary_violation`, `m6c_off_platform_pressure`, `m6d_identity_location_pressure`, `m6e_image_coercion`, `m7_authenticity`, `m7b_ban_evasion`, `payment_solicitation` |
| `tier` (commerce) | `verified_free`, `tryst_plus`, `envoy`, `duo`; add-on `ghost` |

---

## 3. Verification tiers and gates

### 3.1 Tiers

| Tier | Method | Retained | Source |
|---|---|---|---|
| **V0** | Email or phone OTP; device attestation | HMAC contact hash; device key | A §8.1 |
| **V1** | Liveness selfie with randomised challenge, matched to profile photos | Boolean + match score. **No biometric template.** | A §8.1, §19.3 |
| **V2** | Highly effective age assurance through a third-party provider (one of two, abstracted), including an ID-document match | `is_adult` boolean + provider token + ban-anchor candidates (§4.1). **No ID image, no DOB.** | A §8.1, §19.3 L1 |
| **V3** | Optional open banking or digital ID | Boolean + token | A §8.1 |
| **VC** | Couple co-signature: both partners at V2, each confirming from their own distinct device | Link record, both profile IDs, device hashes | A §8.1 |

### 3.2 Capability gates (enforced server-side; mirrored in UI)

**Corrected in v1.1:** Source A's tier table let V1 send intents and open threads, which contradicted ToS cl. 5.1, Source B FR-001 and the OSA inbox requirement. V2 is now the gate for all interaction ([E-01](06_Decisions_and_Changes.md#3-errata-fixed-in-v11)).

| Capability | V0 | V1 | V2 | V3 | Couple (VC) |
|---|---|---|---|---|---|
| Onboarding, Cartographer intake, discretion setup, waitlist | ✓ | ✓ | ✓ | ✓ | ✓ |
| See other members' profiles (slate) | — | — | ✓ | ✓ | ✓ |
| Send intents / receive deliveries | — | — | ✓ | ✓ | ✓ |
| Envoy handshake / Briefs | — | — | ✓ | ✓ | ✓ |
| Threads (E2EE) | — | — | ✓ | ✓ | ✓ |
| Media exchange; explicit media (web only) | — | — | ✓ | ✓ | ✓ |
| Meets, Safe Meet, Share My Plan | — | — | ✓ | ✓ | ✓ |
| Trust badge; set `min_counterparty_tier = V3` | — | — | — | ✓ | — |
| Publish couple profile | — | — | — | — | requires both partners at V2 + co-sign |

**Rules:**
- No other member's content is shown before V2, and none during the verification flow itself.
- `min_counterparty_tier` (default `V2`, option `V3`) is a Stage 0 hard gate in both directions.
- Re-verification triggers: a Guardian `minor` signal, a change of device plus a profile-photo change, a credible report, or a 12-month expiry.

### 3.3 Login assurance — account holder only

**v1.2, founder requirement ([D-16](06_Decisions_and_Changes.md#2-decision-log)).** Only the account holder can sign in. The rules depend on the surface:

| Surface | Factors required at **every login** | Both required? |
|---|---|---|
| **Native app** (iOS, Android) and **PWA on a mobile device** | **B1** device-bound biometric passkey **+ B2** liveness face match to the account holder | Yes — two biometric factors |
| **Desktop** (web browser or installed PWA on a computer) | **F1** passkey (platform authenticator with user verification, or FIDO2 security key) **+ F2** approval from the member's registered mobile device, which itself requires B1 (or a second registered FIDO2 security key) | Yes — two factors |

**Factor definitions:**
- **B1 — Device biometric passkey.** A WebAuthn/FIDO2 credential bound to the device's secure element. `userVerification = required` and must be satisfied by **biometrics only** for login:
    - iOS: `biometryCurrentSet`;
    - Android: `BIOMETRIC_STRONG`, key invalidated on biometric enrolment change;
    - mobile PWA: platform authenticator.
  The biometric never leaves the device. **If anyone adds or changes a fingerprint or face on the device, the credential is invalidated** and the member must log in again with B1 re-registration + B2 (FR-059). This defeats the partner who enrols their own fingerprint (A1).
- **B2 — Liveness face match.** A randomised challenge-response liveness check (ISO 30107-3 PAD Level 2 certified provider), matched 1:1 against the account holder's reference enrolled at V1. The reference is held by the provider under a deletion obligation, never by TRYST ([D-18](06_Decisions_and_Changes.md#2-decision-log)). Requires the explicit `biometric_login` consent.
- **F1 — Desktop passkey.** Windows Hello, Touch ID or a roaming FIDO2 key with PIN or biometric. Phishing-resistant, origin-bound.
- **F2 — Registered-device approval.** The desktop shows a QR code; the member scans it **in the TRYST app** on a registered mobile device and approves with B1. No push notification is sent, so nothing appears on a lock screen. Alternative: a second registered FIDO2 security key.

**Never accepted as a login factor:** passwords (TRYST has none), SMS or email OTP, magic links, security questions, or support-desk overrides. Contact OTP is used **only** at sign-up to prove ownership of the contact handle (V0).

**Login vs unlock:**
- **Login** creates a session on a device: first use on a device, after sign-out or Burn, after session expiry, after a biometric-enrolment change, or on a risk step-up.
- **Unlock** reopens the app within a live session: device biometric (or the duress PIN, which opens the decoy) per FR-027.

| Session rule | Mobile (app / PWA) | Desktop |
|---|---|---|
| Maximum session | 30 days | 12 hours |
| Idle lock | 2 min default (member-adjustable 1–15) | 15 min → full re-login |
| Token binding | DPoP to the device key | DPoP to a non-extractable WebCrypto key |

**Step-up** (repeat both factors for the surface, valid 5 min) is required to:
- add or revoke a device;
- broaden the Discretion Policy;
- change contact handle or payment method;
- co-sign or change a couple link or veto mode;
- export data;
- view the device list;
- start account recovery.

It is **never** required for Burn, Panic, report, block, subscription cancel or account deletion. Deletion requires B1 only, so it stays one step (ToS cl. 16.3, 25.1).

**Before V1:** a V0 member has no B2 reference, so they can only continue onboarding on the device they signed up on. Login on any other device is unavailable until V1 is complete.

**Downgrade resistance:** the server decides the surface from attestation (native) and from the registered authenticator's properties. It does not trust a client claim. The desktop path still requires a second, separately registered authenticator, so claiming "desktop" from a phone gains nothing.

**Account recovery** (lost or replaced device, FR-061):
1. B2 liveness match.
2. Fresh V2 age assurance with an ID match, whose document anchor must equal the account's stored `anchor_doc` (same document).
3. A 24-hour hold, cancellable from any still-registered device.
4. New passkey registration.

No staff member can bypass recovery.

---

## 4. Data model

Three isolated PostgreSQL 16 clusters with separate credentials and separate KMS domains. **No plaintext join between identity and profile.** The join key is `HMAC(identity_id, HSM pepper_join)`, computed only inside the identity service. [A §9.2, §10]

### 4.1 IDENTITY-DB

```sql
CREATE TABLE identity (
  identity_id      UUID PRIMARY KEY,
  contact_hash     BYTEA UNIQUE NOT NULL,   -- HMAC(phone|email, HSM pepper_contact)
  contact_ct       BYTEA,                   -- encrypted under root key; for OTP/notices only
  age_assured      BOOLEAN NOT NULL DEFAULT FALSE,
  assurance_tier   TEXT CHECK (assurance_tier IN ('V0','V1','V2','V3')),
  assurance_token  TEXT,                    -- provider ref. NO ID image, NO DOB
  assurance_at     TIMESTAMPTZ,
  billing_ref      TEXT,                    -- PSP token. No PAN, ever
  jurisdiction     CHAR(2) NOT NULL,
  root_key_ref     TEXT NOT NULL,           -- KMS handle; destroy = erasure
  created_at       TIMESTAMPTZ DEFAULT now(),
  deleted_at       TIMESTAMPTZ
);

-- v1.1 addition (E-02): ban anchors can only be written at ban time if candidates exist.
-- Candidates are erased with the account unless a permanent ban copies them to BAN-DB.
CREATE TABLE anchor_candidate (
  identity_id      UUID PRIMARY KEY REFERENCES identity,
  anchor_doc       BYTEA NOT NULL,          -- HMAC(doc_number, HSM pepper_ban)
  anchor_device    BYTEA[] NOT NULL,        -- HMAC(attestation_id, pepper_ban)
  anchor_payment   BYTEA,                   -- HMAC(psp_instrument_token, pepper_ban)
  created_at       TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE device (
  device_id        UUID PRIMARY KEY,
  identity_id      UUID NOT NULL REFERENCES identity,
  device_pubkey    BYTEA NOT NULL,          -- DPoP + MLS credential binding
  device_hash      BYTEA NOT NULL,          -- HMAC(attestation_id, pepper_device)
  platform         TEXT CHECK (platform IN ('ios','android','web')),
  surface          TEXT CHECK (surface IN ('mobile_app','mobile_pwa','desktop')), -- v1.2
  revoked_at       TIMESTAMPTZ
);

-- v1.2 addition (D-16): login authenticators. No biometric data is stored by TRYST.
CREATE TABLE authenticator (
  credential_id    BYTEA PRIMARY KEY,       -- WebAuthn credential id
  identity_id      UUID NOT NULL REFERENCES identity,
  device_id        UUID REFERENCES device,  -- NULL for roaming security keys
  public_key       BYTEA NOT NULL,
  aaguid           UUID,
  kind             TEXT CHECK (kind IN ('platform_biometric','platform_uv','roaming_key')),
  bio_enrolment_bound BOOLEAN NOT NULL,     -- invalidated on biometric enrolment change
  sign_count       BIGINT DEFAULT 0,
  created_at       TIMESTAMPTZ DEFAULT now(),
  revoked_at       TIMESTAMPTZ
);

CREATE TABLE login_reference (              -- pointer to the provider-held B2 face reference
  identity_id      UUID PRIMARY KEY REFERENCES identity,
  provider         TEXT NOT NULL,
  reference_token  TEXT NOT NULL,           -- provider ref only; deleted at provider on erasure
  enrolled_at      TIMESTAMPTZ NOT NULL
);

CREATE TABLE auth_event (                   -- 90-day retention, IP truncated (NFR-11)
  event_id UUID PRIMARY KEY, identity_id UUID, device_id UUID,
  kind TEXT,                                -- login_ok|login_fail|step_up|recovery_*|bio_change
  factors TEXT[], surface TEXT, created_at TIMESTAMPTZ DEFAULT now()
);
```

### 4.2 BAN-DB (separate store, separate key domain; survives erasure)

```sql
CREATE TABLE ban_anchor (
  anchor_id        UUID PRIMARY KEY,
  anchor_doc       BYTEA, anchor_device BYTEA[], anchor_payment BYTEA,
  anchor_biometric BYTEA,                   -- NULL unless counsel confirms lawful basis (D-08)
  reason_code      TEXT NOT NULL,           -- unsafe_ground enum; NOT free text, NOT evidence
  tier             TEXT CHECK (tier IN ('permanent','law_enforcement_referred')),
  reviewed_by      TEXT NOT NULL,           -- staff id; always a human decision
  created_at       TIMESTAMPTZ DEFAULT now(),
  expires_at       TIMESTAMPTZ,             -- NULL for permanent; destroyed on successful appeal
  consortium_share BOOLEAN DEFAULT FALSE    -- L5, gated on opinion (Q8)
);
```

### 4.3 PROFILE-DB

```sql
CREATE TABLE profile (
  profile_id       UUID PRIMARY KEY,
  segment          TEXT CHECK (segment IN ('S1','S2','S3','S4','S5','S6')),
  entity_type      TEXT CHECK (entity_type IN ('solo','couple')),
  display_name     TEXT,                    -- pseudonym; real name never required
  pronouns         TEXT,
  gender_identity  TEXT,                    -- free text, not enum
  seeking          TEXT[],
  modes            TEXT[],                  -- intent_shape values enabled
  age_band         TEXT,
  city_geohash5    TEXT,
  bio_ct           BYTEA,
  visibility       TEXT CHECK (visibility IN ('public','matched','incognito')),
  status           TEXT DEFAULT 'active',   -- active|paused|vanished|suspended|deleted
  version          INT NOT NULL DEFAULT 1,  -- couple profiles publish a version both approved
  created_at       TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE couple_invite (                -- v1.1 addition (E-23)
  invite_id        UUID PRIMARY KEY,
  inviter_profile  UUID NOT NULL REFERENCES profile,
  token_hash       BYTEA NOT NULL,
  inviter_device_hash BYTEA NOT NULL,
  expires_at       TIMESTAMPTZ NOT NULL,    -- 24h
  consumed_at      TIMESTAMPTZ
);

CREATE TABLE couple_link (
  couple_id        UUID PRIMARY KEY,
  couple_profile   UUID NOT NULL REFERENCES profile, -- the entity_type='couple' profile
  profile_a        UUID NOT NULL REFERENCES profile,
  profile_b        UUID NOT NULL REFERENCES profile,
  a_cosigned_at    TIMESTAMPTZ NOT NULL,
  b_cosigned_at    TIMESTAMPTZ NOT NULL,
  a_device_hash    BYTEA NOT NULL,
  b_device_hash    BYTEA NOT NULL,
  approved_version INT NOT NULL,            -- profile version both approved (B FR-005)
  veto_mode        TEXT DEFAULT 'either' CHECK (veto_mode IN ('either','both')),
  dissolved_at     TIMESTAMPTZ,
  CONSTRAINT distinct_devices CHECK (a_device_hash <> b_device_hash),
  CONSTRAINT distinct_members CHECK (profile_a <> profile_b)
);

CREATE TABLE intent_vector (
  profile_id       UUID PRIMARY KEY REFERENCES profile,
  intent_shape     TEXT[],
  structure        TEXT,
  desire_tags      JSONB,                   -- {tag: yes|curious|no}; ~140 tags
  hard_limits      TEXT[] NOT NULL,         -- NEVER overridable
  discretion       JSONB,                   -- windows, device risk (zones live in discretion_policy)
  logistics        JSONB,                   -- radius_km, hosting, notice_hours, venue_budget_band
  min_counterparty_tier TEXT DEFAULT 'V2',  -- v1.1: V2 floor (E-01)
  embedding        VECTOR(128),
  private_fields   TEXT[],                  -- computable, never emitted by Envoy
  confirmed_at     TIMESTAMPTZ,             -- member confirmed structured output
  updated_at       TIMESTAMPTZ
);

CREATE TABLE revealed_preference (
  profile_id       UUID PRIMARY KEY REFERENCES profile,
  embedding        VECTOR(256),
  alpha            REAL CHECK (alpha BETWEEN 0.15 AND 0.85),
  divergence       REAL, confidence REAL, n_signals INT,
  personalisation_enabled BOOLEAN DEFAULT TRUE,
  computed_at      TIMESTAMPTZ
);

CREATE TABLE discretion_policy (            -- v1.1 addition from Source B (FR-004)
  profile_id       UUID REFERENCES profile,
  version          INT,
  policy_ct        BYTEA NOT NULL,          -- §6.4 schema, encrypted
  zones_geohash    TEXT[],                  -- home/work exclusion cells (geohash-6), encrypted column
  created_at       TIMESTAMPTZ DEFAULT now(),
  PRIMARY KEY (profile_id, version)
);

CREATE TABLE media (
  media_id         UUID PRIMARY KEY,
  owner_profile    UUID REFERENCES profile,
  object_key       TEXT NOT NULL,           -- client-encrypted ciphertext
  tier             TEXT CHECK (tier IN ('profile_public','profile_private','thread')),
  explicit         BOOLEAN DEFAULT FALSE,   -- web surface only, V2 only
  moderation_state TEXT DEFAULT 'pending',
  expires_at       TIMESTAMPTZ,             -- thread media: 7d default
  created_at       TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE reveal_grant (
  grant_id         UUID PRIMARY KEY,
  owner_profile    UUID, recipient_profile UUID,
  scope            TEXT,                    -- reveal_scope enum
  media_id         UUID,                    -- NULL for voice/video scope
  owner_version    INT,
  wrapped_key_ref  TEXT,                    -- destroyed on revoke
  expires_at       TIMESTAMPTZ, revoked_at TIMESTAMPTZ,
  created_at       TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE slate_item (                   -- "Introduction" in Source B
  slate_id         UUID, profile_id UUID, candidate_id UUID,
  score            REAL, model_version TEXT,
  explanation_codes TEXT[],                 -- 2–4 non-sensitive codes
  exploration      BOOLEAN, paid_label BOOLEAN,
  created_at       TIMESTAMPTZ,
  PRIMARY KEY (slate_id, candidate_id)
);

CREATE TABLE intent (
  intent_id        UUID PRIMARY KEY,
  from_profile     UUID, to_profile UUID,
  keys_spent       INT NOT NULL DEFAULT 0,
  via_envoy_proposal UUID,                  -- ENVOY tier; always human-approved (D-04)
  state            TEXT NOT NULL,           -- intent_state enum
  created_at       TIMESTAMPTZ, delivered_at TIMESTAMPTZ
);

CREATE TABLE handshake (
  handshake_id     UUID PRIMARY KEY,
  a_profile UUID, b_profile UUID,
  transcript_ct    BYTEA,                   -- encrypted agent turns, 30d TTL
  brief_a_ct BYTEA, brief_b_ct BYTEA,
  compatibility REAL, meet_probability REAL,
  a_decision TEXT, b_decision TEXT,         -- handshake_decision
  opened_thread    UUID,                    -- non-null ONLY on double accept
  created_at       TIMESTAMPTZ
);

CREATE TABLE thread (
  thread_id        UUID PRIMARY KEY,
  handshake_id     UUID,                    -- NULL in P1 control cohort
  mls_group_id     BYTEA NOT NULL,
  participant_profiles UUID[] NOT NULL,
  retention_days   INT DEFAULT 30 CHECK (retention_days IN (7,30,90)),
  safety_state     TEXT DEFAULT 'normal',   -- normal|frozen|closed
  burned_at        TIMESTAMPTZ,
  created_at       TIMESTAMPTZ
);

CREATE TABLE message_envelope (             -- ciphertext only; server cannot decrypt
  thread_id UUID, seq BIGINT, sender_device UUID,
  ciphertext BYTEA NOT NULL, expires_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (thread_id, seq)
);

CREATE TABLE meet (
  meet_id UUID PRIMARY KEY, thread_id UUID,
  state TEXT,                               -- proposed|confirmed|cancelled|occurred
  window_start TIMESTAMPTZ, checkin_due_at TIMESTAMPTZ,
  aftercare_due_at TIMESTAMPTZ              -- +48h
);

CREATE TABLE exclusion_ring (
  profile_id UUID, excluded_profile_id UUID,
  source TEXT,                              -- psi_contact|device_graph|manual|payment|geo|wifi
  confidence REAL,
  PRIMARY KEY (profile_id, excluded_profile_id)
);  -- ALWAYS bidirectional, ALWAYS silent

CREATE TABLE consent_ledger (
  consent_id UUID PRIMARY KEY, profile_id UUID NOT NULL,
  kind TEXT NOT NULL,                       -- consent_kind enum
  granted BOOLEAN NOT NULL, scope JSONB,
  prev_hash BYTEA, entry_hash BYTEA NOT NULL, -- hash chain; append-only
  created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE aftercare (
  meet_id UUID PRIMARY KEY, reporter UUID, counterparty UUID,
  occurred BOOLEAN, as_represented SMALLINT, would_repeat BOOLEAN, safety_flag BOOLEAN,
  created_at TIMESTAMPTZ
);  -- NEVER surfaced to counterparty. NEVER a public rating.
```

### 4.4 SAFETY-DB (T&S; access only through the trust console)

```sql
CREATE TABLE guardian_signal (              -- scores and reason codes only; no content
  signal_id UUID PRIMARY KEY, thread_id UUID, subject_profile UUID,
  model_id TEXT, model_version TEXT, score REAL, reason_code TEXT,
  source TEXT CHECK (source IN ('on_device','server_profile','server_graph')),
  created_at TIMESTAMPTZ
);
CREATE TABLE report_case (
  case_id UUID PRIMARY KEY, reporter_pseudo BYTEA, subject_profile UUID,
  category TEXT, severity SMALLINT, disclosed_content_ref TEXT, -- reporter-disclosed only
  training_consent BOOLEAN DEFAULT FALSE,   -- D4 source; never a condition of action
  state TEXT, sla_due_at TIMESTAMPTZ, legal_hold BOOLEAN DEFAULT FALSE,
  created_at TIMESTAMPTZ
);
CREATE TABLE enforcement (
  enforcement_id UUID PRIMARY KEY, subject_profile UUID,
  action TEXT, ground TEXT, reason_code TEXT,
  decided_by TEXT,                          -- REQUIRED for permanent (human)
  appeal_state TEXT, appeal_reviewer TEXT, created_at TIMESTAMPTZ
);
CREATE TABLE staff_access_log (             -- immutable (WORM)
  entry_id UUID PRIMARY KEY, staff_id TEXT, approver_id TEXT,
  resource TEXT, reason_code TEXT, created_at TIMESTAMPTZ
);
```

### 4.5 Commerce and operations (PROFILE-DB schema `commerce`, `ops`)

```sql
CREATE TABLE commerce.entitlement (profile_id UUID, tier TEXT, addon_ghost BOOLEAN,
  status TEXT, source TEXT CHECK (source IN ('web','voucher','app_store','play')),
  renews_at TIMESTAMPTZ, renewal_notice_sent_at TIMESTAMPTZ, PRIMARY KEY (profile_id));
CREATE TABLE commerce.keys_ledger (entry_id UUID PRIMARY KEY, profile_id UUID,
  delta INT, reason TEXT, intent_id UUID, balance_after INT, created_at TIMESTAMPTZ);
CREATE TABLE commerce.deposit (profile_id UUID PRIMARY KEY, amount_minor INT,
  state TEXT, credited_keys INT, created_at TIMESTAMPTZ);
CREATE TABLE ops.deletion_job (job_id UUID PRIMARY KEY, subject_ref BYTEA,
  systems TEXT[], started_at TIMESTAMPTZ, completed_at TIMESTAMPTZ,
  completion_proof BYTEA);                  -- signed record of key destruction + purges
```

---

## 5. Edge API contract (v1)

**Cross-cutting middleware, in order:**
1. TLS 1.3
2. OAuth2 access token **DPoP-bound** to the device key
3. Jurisdiction gate
4. Tier gate (§3.2)
5. Article 9 consent check
6. Per-endpoint rate limit
7. Idempotency (`Idempotency-Key` header required on all POST/PUT/DELETE)

The **Min** column is the minimum verification tier. `sys` means server-to-server only.

### 5.1 Auth, devices, assurance

| Method | Path | Min | Contract |
|---|---|---|---|
| POST | `/v1/auth/start` | — | **Sign-up only.** `{contact}` → OTP sent. Uniform response whether or not the account exists. **Never a login factor** (§3.3) |
| POST | `/v1/auth/verify` | — | **Sign-up only.** `{contact, otp, jurisdiction?}` → onboarding session (device binding via DPoP is a P1 hardening item, [D-24](06_Decisions_and_Changes.md#2-decision-log)). Refused with `account_exists_sign_in` if the account already has a passkey: an OTP can never add one. Ban-anchor check runs here for new identities ([03 §8](03_Backend.md#8-admission-control-l1l5)) |
| POST | `/v1/auth/passkeys/register` (`begin` / `finish`) | V0 | Register B1 (mobile) or F1 (desktop) WebAuthn credential; step-up required after the first one |
| POST | `/v1/auth/login/begin` | — | `{contact_hint?}` → WebAuthn challenge + `required_factors` (decided by the server, §3.3). Uniform response |
| POST | `/v1/auth/login/passkey` | — | WebAuthn assertion (B1 or F1) → `login_state = factor1_ok` (no session yet) |
| POST | `/v1/auth/login/liveness` | — | Mobile: start B2 provider session → provider callback (`sys`) → **session issued only on match** |
| POST | `/v1/auth/login/cross-device` | — | Desktop: → QR token (60 s); `GET .../{id}` polls status → **session issued only on approval** |
| POST | `/v1/auth/approvals/{qr_token}` | V1 | Called from the registered mobile app with a fresh B1 assertion; approves or denies the desktop login |
| POST | `/v1/auth/step-up` | V0 | Same factor flow as login → step-up token (5 min) for §3.3 sensitive actions |
| POST | `/v1/auth/enrol-device` · `/v1/auth/enrol-device/redeem` | V0 (onboarding) | During onboarding only: one-time 10-minute link that lets the member's phone register its own passkey on the same account. Refused once the account is sealed (device add then needs step-up) |
| POST | `/v1/auth/login/second-key/begin` · `/v1/auth/login/second-key` | — | Desktop F2 alternative: assertion with a **different** registered security key |
| POST | `/v1/auth/approvals/{qr_token}/begin` | — | Assertion options restricted to the account's registered **phone** passkeys |
| GET | `/v1/me` | V0 | Session summary (kind, surface, tier, passkey count). No contact or identity data |
| POST | `/v1/auth/logout` | V0 | Ends this session |
| POST | `/v1/auth/recovery/begin` · `/finish` | — | B2 + fresh V2 with matching `anchor_doc` → 24 h hold → new passkey (FR-061) |
| POST | `/v1/auth/refresh` | V0 | DPoP refresh |
| GET / DELETE | `/v1/devices` · `/v1/devices/{id}` | V0 | List and revoke devices (**step-up required**, §3.3) |
| POST | `/v1/assurance/session` | V0 | `{kind: liveness|age}` → provider hand-off URL |
| POST | `/v1/assurance/callback` | sys | Signed provider token → `is_adult` boolean + anchor candidates. Raw document and biometric never stored |
| POST | `/v1/attestations/conduct` | V1 | Conduct declaration (ToS cl. 3.5) → consent ledger. Required to complete V2 |

### 5.2 Account, consent, data rights

| Method | Path | Min | Contract |
|---|---|---|---|
| GET / POST | `/v1/consents` | V0 | Read the ledger; append grant or withdrawal (never mutate) |
| POST | `/v1/account/pause` | V0 | Removed from discovery immediately |
| POST | `/v1/emergency-lock` | — | **No session needed.** `{contact_hint, emergency_code}` → freeze sessions, hide profile, disable media links (FR-064). Uniform response; rate-limited; the code is single-use and reissued after recovery |
| POST | `/v1/account/vanish` | V0 | Hidden + threads closed; account kept |
| POST | `/v1/account/export` | V0 | SAR/portability export (encrypted), < 30 days; target < 24 h |
| DELETE | `/v1/account` | V0 | Cryptographic erasure, **< 60 s p99**. Returns `deletion_job_id` and, later, `completion_proof` |

### 5.3 Intake, profile, discretion

| Method | Path | Min | Contract |
|---|---|---|---|
| POST | `/v1/intake/turn` | V0 | Cartographer conversational turn |
| GET | `/v1/intake/vector` | V0 | Proposed IntentVector (structured) |
| POST | `/v1/intake/vector/confirm` | V0 | Member confirms or edits each field; stored with `confirmed_at` |
| PATCH | `/v1/profile` | V0 | Pseudonymous fields only |
| POST / DELETE | `/v1/profile/media` · `/{id}` | V1 | Client-encrypted blob + wrapped keys; public-tier photos go through server moderation ([03 §7.5](03_Backend.md#75-profile-and-media-moderation-server-side)) |
| PUT | `/v1/discretion/policy` | V0 | New version (§6.4). **Rejects any change that broadens exposure unless `confirm_broaden=true`** is sent from a fresh biometric/PIN session |
| PUT | `/v1/discretion/zones` | V0 | Home/work exclusion geofences (geohash-6 cells) |
| POST | `/v1/exclusion/psi/blind` | V2 | OPRF round 1 |
| POST | `/v1/exclusion/psi/commit` | V2 | OPRF round 2 → suppression written. **Nothing returned about the intersection** |
| POST | `/v1/burn` | V0 | **Renamed from `/v1/panic` ([E-21](06_Decisions_and_Changes.md#3-errata-fixed-in-v11)).** Revoke device keys, sign out everywhere; the client wipes local data |

### 5.4 Couples

| Method | Path | Min | Contract |
|---|---|---|---|
| POST | `/v1/couple/invite` | V2 | → co-sign token (24 h) |
| POST | `/v1/couple/cosign` | V2 | **Must come from a device whose hash differs from the inviter's.** Both partners must approve the exact profile `version` before publication |
| PUT | `/v1/couple/{id}/veto-mode` | VC | Change requires both partners' confirmation |
| DELETE | `/v1/couple/{id}` | V2 | Either partner dissolves immediately; no reason shown to the other |

### 5.5 Discovery and learning

| Method | Path | Min | Contract |
|---|---|---|---|
| GET | `/v1/slate` | V2 | Up to 18 cards incl. 15% exploration; each with `explanation_codes` (2–4) and `paid_label`. Re-authorised against the current Discretion Policy and block graph on every read |
| POST | `/v1/intent` | V2 | `{target, keys_spent}` → `202 {intent_id, state}`. Inbound-cap overflow returns `state=sent` to the sender and is queued (the cap is never revealed) |
| POST | `/v1/pass` · `/v1/save` | V2 | Private |
| POST | `/v1/signals:batch` | V0 | ≤ 50 behaviour events (§8) |
| GET | `/v1/preferences/insights` | V2 | Category-level view of what has been learned (never a raw vector) |
| POST | `/v1/preferences/correction` | V2 | "More / less like this" and category corrections |
| POST | `/v1/preferences/reset` | V2 | Neutralise RevealedPreferenceVector; α returns to 0.15; rebuild from the IntentVector |
| PUT | `/v1/preferences/personalisation` | V2 | Turn personalisation on or off |

### 5.6 Handshake, ENVOY tier, reveal

| Method | Path | Min | Contract |
|---|---|---|---|
| — | *(system)* handshake created on mutual intent | — | Not client-callable in v1.1 ([E-29](06_Decisions_and_Changes.md#3-errata-fixed-in-v11)) |
| GET | `/v1/handshake/{id}/brief` | V2 | The caller's own asymmetric Brief only (§6.2) |
| POST | `/v1/handshake/{id}/decision` | V2 | `accept|decline|amend` (+ amend fields). Thread opens **only** when both are `accept` (couples: per `veto_mode`) |
| GET | `/v1/envoy/proposals` | V2 + `envoy` | Agent-proposed intents with rationale |
| POST | `/v1/envoy/proposals/{id}/approve` | V2 + `envoy` | **One-tap human approval required for every outbound intent ([D-04](06_Decisions_and_Changes.md#2-decision-log))** |
| POST | `/v1/reveal-grants` | V2 | `{recipient, scope, media_id?, expires_at}` |
| DELETE | `/v1/reveal-grants/{id}` | V2 | Destroys the wrapped key; access ends at once (p95 < 5 s, including CDN) |

### 5.7 Threads (E2EE; the server relays ciphertext only)

| Method | Path | Min | Contract |
|---|---|---|---|
| POST / GET | `/v1/threads/{id}/messages` | V2 | MLS ciphertext envelopes; `expires_at` set from `retention_days` |
| PUT | `/v1/threads/{id}/retention` | V2 | 7 / 30 / 90 days; the shortest elected by any participant wins |
| POST | `/v1/threads/{id}/meet` | V2 | `propose|confirm|cancel` |
| POST | `/v1/threads/{id}/checkin` | V2 | Safe Meet check-in; a missed check-in triggers silent escalation to the trusted contact |
| POST | `/v1/threads/{id}/burn` | V2 | Revoke keys on both sides; thread closed |
| POST | `/v1/aftercare/{meet_id}` | V2 | Private outcome (48 h after the meet) |

### 5.8 Safety

| Method | Path | Min | Contract |
|---|---|---|---|
| POST | `/v1/report` | V0 | `{subject, category, disclosed_content?, training_consent?}` → SLA routing. The reporter is never revealed |
| POST | `/v1/block` | V0 | Bidirectional and immediate; removes thread access |
| POST | `/v1/guardian/signals` | V2 | On-device scores: `{thread_id, model_id, version, score, reason_code}`. **Never content** |
| POST | `/v1/safety/panic` | V0 | **Emergency.** Live location + preserved evidence → emergency escalation. Overrides TTLs (P7) |
| POST | `/v1/safety/share-plan` | V2 | Encrypted, unbranded, auto-deleting plan for a trusted contact |
| POST | `/v1/appeals` | V0 | Appeal an enforcement action; a different reviewer decides within 10 working days |
| POST | `/v1/attestations/dbs` | V2 | Self-submitted Basic DBS through the issuing route; shown as a dated fact (P3) |

### 5.9 Commerce (web-first)

| Method | Path | Min | Contract |
|---|---|---|---|
| GET | `/v1/entitlements` | V0 | Tier, add-ons, Keys balance, renewal date |
| POST | `/v1/deposit` | V1 | £9.99 refundable deposit → Keys credit |
| POST | `/v1/keys/purchase` | V1 | Keys packs |
| POST | `/v1/vouchers/redeem` | V0 | Offline voucher/gift code |
| POST | `/v1/subscription` | V1 | Neutral, truthful descriptor enforced at the PSP; pre-contract and cooling-off notices |
| POST | `/v1/billing/stripe/webhook` | sys | Stripe events (signature-verified, idempotent) → entitlement and Keys-ledger updates (v1.2, D-22) |
| POST | `/v1/subscription/cancel` | V0 | One step; no retention flow |

### 5.10 Error model

RFC 9457 `application/problem+json` with a stable `code`.

- **Existence must not leak.** Anything not visible to the caller, for any reason (block, ExclusionRing, policy, jurisdiction of the target), returns the same `404 not_found`.

| HTTP | `code` | Meaning |
|---|---|---|
| 401 | `auth_required`, `dpop_invalid`, `second_factor_required` (+`factor`), `step_up_required`, `authenticator_invalidated` (biometric enrolment changed) | |
| 409 | `account_exists_sign_in` | Sign-up attempted for an account that already has a passkey |
| 503 | `liveness_unavailable` | B2 provider unavailable; login fails closed |
| 403 | `tier_required` (+`required_tier`), `consent_required` (+`consent_kind`), `jurisdiction_blocked`, `entitlement_required` | |
| 404 | `not_found` | Uniform for invisible resources |
| 409 | `idempotency_conflict`, `version_conflict` | |
| 402 | `keys_insufficient` | |
| 422 | `validation_failed`, `broadening_requires_confirmation`, `same_device_cosign` | |
| 429 | `rate_limited` (+`retry_after`) | |

---

## 6. Agent and policy schemas

### 6.1 Envoy negotiation turn (internal bus; Protobuf mirror)

Six dimensions only: intent alignment, structure alignment, boundary intersection, logistics feasibility, discretion compatibility, verification parity. [A §5.4]

```json
{
  "turn": 3,
  "from": "envoy:usr_7f3a",
  "to": "envoy:usr_91bc",
  "assert": {
    "intent_shape": "one_off",
    "structure": "attached_undisclosed",
    "notice_required_hours": 48,
    "availability": [{"dow": [2,3], "start": "18:00", "end": "23:00", "tz": "Europe/London"}],
    "travel_radius_km": 25,
    "hard_limits": ["no_photos_exchanged", "no_home_visit", "no_overnight"],
    "verification_tier": "V2",
    "min_counterparty_tier": "V2"
  },
  "query": ["intent_shape", "notice_required_hours", "hosting_capability"],
  "private_fields": ["exclusion_zones"]
}
```

**Rules:**
- No free-text fields.
- `private_fields` are computable but **never** appear in any output.
- Counterparty turns are untrusted data.
- At most 6 turns per handshake.
- The transcript is encrypted with a 30-day TTL.

### 6.2 Brief (caller-specific, asymmetric)

```json
{
  "label": "agent_generated",
  "compatibility": 0.81,
  "aligned": ["intent_shape", "availability", "travel_radius", "verification"],
  "friction": [{"field": "hosting", "detail_code": "neither_can_host",
                "suggested_resolution": "hotel_or_venue"}],
  "blocking": [],
  "recommended_next": "open_thread",
  "estimated_meet_probability": 0.34
}
```

`detail_code` values map to localised copy on the client. A Brief never contains anything the counterparty marked private, nor any hard limit the counterparty has not chosen to disclose.

### 6.3 IntentVector (wire view)

The fields follow `intent_vector` (§4.3). `desire_tags` uses `yes|curious|no`. `hard_limits` is drawn from a controlled vocabulary (`contracts/vocab/hard_limits.yaml`). Free-text goals entered during intake are converted by Cartographer into controlled values, and the member confirms them.

### 6.4 Discretion Policy (versioned)

```json
{
  "version": 7,
  "visibility": "incognito",
  "distance_band_km": 10,
  "zones": {"home": ["gcpvj0"], "work": ["gcpuvx"]},
  "zone_behaviour": "hide_profile_while_inside",
  "reveal_rules": {"default_scope": "blurred_gallery", "require_mutual_handshake": true,
                   "max_grant_hours": 72},
  "notifications": {"enabled": false, "text": "1 update"},
  "device_risk": "high",
  "min_counterparty_tier": "V2",
  "retention": {"messages_days": 30, "media_days": 7}
}
```

**Monotonic safety:** Curtain may tighten any field automatically. Broadening needs an explicit member action, a fresh local authentication and a new version. Every read path evaluates `(requester, resource, reveal_grant, block graph, ExclusionRing, discretion version, safety_state, jurisdiction)` — ABAC [B §9].

---

## 7. Retention and erasure

This table is canonical; 03 and 04 implement it, and the ToS (cl. 15) and Privacy Notice must match it.

| Data | Default | Member options | Mechanism |
|---|---|---|---|
| Messages | 30 d | 7 / 90 d | Envelope `expires_at`; the client purges its copy too |
| Thread media | 7 d | — (GHOST: shorter) | Per-object key destroyed at expiry |
| Profile media | Until removed | — | Per-object key; removal destroys it |
| Handshake transcripts | 30 d | — | Encrypted; TTL |
| Behaviour events (raw) | 180 d → aggregated features | Reset / opt-out | ClickHouse TTL |
| Aftercare | 365 d, pseudonymous | — | |
| Logs | 90 d, IP truncated at ingest | — | |
| Consent ledger | Life of account + statutory period | Export | Hash chain |
| Ban anchors | Permanent (or until a successful appeal) | Appeal | **Declared exception** (ToS cl. 15.3) |
| Evidence under legal hold | Duration of hold | — | **Declared exception** (ToS cl. 17.4); narrowly scoped |
| Panic-preserved location/evidence | As required by emergency services | — | **Declared exception** (ToS cl. 20; P7) |
| B2 login face reference (v1.2) | Life of account; held by the provider, not TRYST | — | Deleted at the provider on erasure; deletion receipt included in `completion_proof` |
| Auth events | 90 d, IP truncated | — | TTL |
| Backups | 35 d rolling, disclosed in the Privacy Notice | — | Stored under per-member keys, so an erased member's data in backups is undecryptable from the moment of erasure |

**Account deletion:**
1. Destroy the root key in KMS.
2. Remove from discovery immediately.
3. Purge primaries.
4. Ciphertext in backups becomes undecryptable.
5. Issue a signed `completion_proof`.

Target < 60 s p99 ([NFR-01](#102-non-functional-requirements)).

---

## 8. Event taxonomy

Events go to Redpanda with schema-registry enforcement, are pseudonymous at rest, and are batched from clients through `/v1/signals:batch`. [A §6.1] **No message content, ever.**

| Class | Events |
|---|---|
| Attention | `card_impression`, `dwell_ms`, `photo_index_viewed`, `bio_scroll_depth`, `profile_revisit`, `expand_tag` |
| Decision | `intent_sent`, `pass`, `save`, `undo`, `hesitation_ms` |
| Conversation | `msg_sent`, `reply_latency_s`, `msg_len_bucket`, `typing_abandoned`, `question_ratio`, `thread_decay` — computed on device from metadata |
| Handshake | `brief_viewed`, `brief_accepted`, `friction_dismissed`, `amend_requested` |
| Resolution | `logistics_agreed`, `meet_scheduled`, `meet_confirmed`, `no_show`, `aftercare_score` |
| Discretion | `burn_invoked`, `session_aborted`, `geofence_hit`, `screenshot_detected`, `decoy_opened` |
| Negative | `report`, `block`, `unmatch`, `ghost_detected`, `refund_request` |

Envelope: `{event_id, pseudo_subject, type, ts, context:{surface, app_version}, consent_basis, ttl}`. The pseudo-subject rotates per training window ([03 §4](03_Backend.md#4-behaviour-learning-mirror)).

---

## 9. Couple and group semantics

- **Veto:** `veto_mode=either` means either partner can decline and a decline is final. `both` means both partners must accept before the couple's intent or handshake decision counts.
- **THIRD:** a couple(S3) ↔ solo(S4) handshake involves both partners' Envoy contexts. The solo's veto is always respected. The thread is an MLS group of 3 members.
- **QUAD:** couple ↔ couple; 4 MLS members. A thread opens only when both couples' veto rules are satisfied.
- **Leaving:** any participant can leave a group thread privately. The thread then closes for that member, and others see a neutral notice.
- **Dissolution:** immediately hides the couple profile, closes its threads and keeps both individual accounts.

---

## 10. Requirement catalogue

**Owner:** `BE` backend, `FE` frontend (native + web), `Both`. **Phase:** first phase in which the requirement must be met ([01 §12](01_Product.md#12-roadmap-team-budget-and-phase-gates)). **Src:** A/B/C plus section.

### 10.1 Functional requirements

| ID | Requirement | Owner | Phase | Src |
|---|---|---|---|---|
| FR-001 | Discovery, intents, Envoy, threads, media and meets are locked below V2; nothing from other members is shown during verification | Both | P1 | A §8.1, B FR-001, E-01 |
| FR-002 | Pseudonymous identity: no legal name, employer or exact address is ever required or shown | Both | P1 | A §14.2, B FR-002 |
| FR-003 | Cartographer conversational intake produces an IntentVector; the member confirms every structured field; re-run quarterly or on drift | Both | P1 | A §5.1, B |
| FR-004 | A member can enable several modes, each with its own visibility and filters | Both | P1 | B FR-003 |
| FR-005 | Versioned Discretion Policy enforced on every read path; cannot be broadened silently | BE | P1 | B FR-004 |
| FR-006 | Couple profile requires `VC`: both at V2, distinct devices, 24 h invite, both approve the exact profile version | Both | P1 | A §8.1, B FR-005 |
| FR-007 | Either partner can dissolve a couple immediately, with no reason shown to the other | Both | P1 | Part B cl. 7.1 |
| FR-008 | Reveal ladder (alias → blurred → photo set → voice → video); each grant records recipient, scope, version and expiry, and is revocable | Both | P1 | B FR-006 |
| FR-009 | Incognito members are discoverable only after their own outbound intent | BE | P1 | B FR-007 |
| FR-010 | ExclusionRing PSI: on-device blinding; the server never learns contacts; suppression is bidirectional and silent | Both | P2 | A §9.3, B FR-008 |
| FR-011 | Home/work exclusion zones suppress both sides and hide the profile while inside them | Both | P1 | A §5.5, §9.4 |
| FR-012 | Daily slate of up to 18 cards with a 15% exploration floor | Both | P1 | A §4, §7 |
| FR-013 | Intent costing: free tier 3/week; Keys 5 (solo→solo) / 15 (couple→solo); refund on reply; inbound cap default 12/24 h (adjustable 4–40), overflow queued and re-scored | BE | P1 | A §7.4, §15.2 |
| FR-014 | Hard limits, zones, structure compatibility, ExclusionRing, blocks, tier parity and jurisdiction are Stage 0 pre-retrieval gates; any violation is a P0 incident; enforced by property tests | BE | P1 | A §6.2, §7 |
| FR-015 | Handshake on mutual intent: six-dimension Envoy negotiation; asymmetric Briefs; private fields never emitted; Briefs labelled | Both | P2 | A §5.4 |
| FR-016 | A thread opens only on double human accept (P2+); in P1 the control cohort opens on mutual intent | Both | P1 | A §4, §17 |
| FR-017 | E2EE messaging (MLS, RFC 9420); the server stores ciphertext only and cannot read content | Both | P1 | A §9.2 |
| FR-018 | Thread media is client-encrypted with a per-object key wrapped per recipient; revocation destroys the wrapped key | Both | P1 | A §9.2 |
| FR-019 | Thread burn revokes keys on both sides and closes the thread | Both | P1 | A §11 |
| FR-020 | Meet propose/confirm/cancel; Safe Meet checklist (public venue first); check-in timer with silent escalation | Both | P1 | A §19.7 |
| FR-021 | Share My Plan: encrypted, unbranded, auto-deleting share with a trusted contact | Both | P1 | A §19.7 |
| FR-022 | Panic: escalation to 999 with live location and preserved evidence; a separate control from Burn | Both | P1 | A §19.7, E-21 |
| FR-023 | Private Aftercare prompt 48 h after a meet; never visible to the counterparty | Both | P1 | A §5.7 |
| FR-024 | Preference transparency: view learned categories, correct, disable personalisation, reset learning | Both | P2 | B FR-010 |
| FR-025 | Each slate card shows 2–4 non-sensitive reasons and a correction control | Both | P2 | B §6.2 |
| FR-026 | Notifications off by default; if enabled, content-free with member-chosen neutral wording (default "You have an update") — never a name, image or excerpt | FE | P1 | A §9.4, B §4 |
| FR-027 | Decoy skin (neutral icon/name) and silent duress PIN opening a populated innocuous state (native) | FE | P1 | A §9.4 |
| FR-028 | Burn: wipe local storage, revoke device keys, sign out everywhere; confirmable in < 2 s | Both | P1 | A §9.4 |
| FR-029 | Quick exit hides content and clears the client view and cache in < 500 ms on all surfaces; onboarding states plainly that it cannot hide network or device history | FE | P1 | B FR-011, B §4 |
| FR-030 | Screenshot protection (FLAG_SECURE) or detection with counterparty notice (iOS); obscured app switcher; no cloud backup; keys in Secure Enclave/StrongBox | FE | P1 | A §9.4 |
| FR-031 | Entering a blocked jurisdiction auto-locks to the decoy and hides the profile | Both | P1 | A §9.4, §14.5 |
| FR-032 | Report and block from every surface; the reporter is never revealed; reporters may disclose decrypted content and separately opt in to training use | Both | P1 | A §21.3, Part B cl. 19 |
| FR-033 | Guardian M6a–M6e run on device for E2EE threads and send only score + reason code | Both | P1 | A §21.6 |
| FR-034 | Escalation matrix routing and SLAs ([03 §7.3](03_Backend.md#73-escalation-matrix)) | BE | P1 | A §8.4 |
| FR-035 | Enforcement ladder; permanent actions need a named human; appeal to a different reviewer within 10 working days | BE | P1 | A §19.6 |
| FR-036 | Ban anchors are checked before account creation; a match is silently refused | BE | P1 | A §19.3 L2 |
| FR-037 | No commercial sex: solicitation and payment patterns go to review; no payments between members | Both | P1 | A §8.4, B FR-015 |
| FR-038 | Account deletion = cryptographic erasure < 60 s; immediate discovery removal; signed completion proof | Both | P1 | A §9.2, B FR-012 |
| FR-039 | DSRs: SAR export including the consent ledger, rectification, portability | Both | P1 | A §14.2 |
| FR-040 | Hash-chained consent ledger; Article 9 consent separate from the ToS accept | BE | P1 | A §8.2 |
| FR-041 | Web-first billing: TRYST+ and Keys (P1), DUO, ENVOY and GHOST (P3); neutral truthful descriptor; vouchers; one-step cancel; pre-renewal and cooling-off notices | Both | P1 | A §14.4, §15 |
| FR-042 | £9.99 refundable verification deposit credited as Keys | Both | P1 | A §15.2 |
| FR-043 | Jurisdiction gate at the edge and at sign-up | BE | P1 | A §14.5 |
| FR-044 | Conduct attestation at V2, logged separately | Both | P1 | A §19.3 L3 |
| FR-045 | Basic DBS self-submission shown as a dated fact; Clare's Law explainer before the first meet (outcome never requested or stored) | Both | P3 | A §19.3 L3 |
| FR-046 | Pause and vanish | Both | P1 | B §3 |
| FR-047 | Profile-photo protection: EXIF strip, face-blur default, AI-generation and reverse-image checks on every upload, adversarial perturbation on public-tier photos | Both | P1 | A §9.1 A8, §19.4 |
| FR-048 | ENVOY tier: agent-proposed outreach, each intent approved by the human with one tap | Both | P3 | A §15.2, D-04 |
| FR-049 | Couple `veto_mode` (`either`/`both`); THIRD and QUAD matching and MLS group threads | Both | P1 | A §3, §10 |
| FR-050 | Exposure-fairness monitor (M9) adjusts the fairness penalty when inbound Gini > 0.55 per city/segment | BE | P1 | A §7.4 |
| FR-051 | Native store builds are sanitised (no explicit media or copy); explicit media lives only on web behind V2 | FE | P1 | A §14.3 |
| FR-052 | Semi-annual transparency report: removals by ground, appeals, overturn rate, disclosures | BE | P3 | A §19.6, Part B cl. 18.3 |
| FR-053 | Travel mode: set a planned location for discovery (premium) | Both | P3 | A §15.2 |
| FR-054 | In-product safety notice at intake and before a first meet ([04 §7](04_Frontend.md#7-safety-ux)) | FE | P1 | A §20.3 |
| FR-055 | Envoy and other agents never send messages to a human as the member | BE | P2 | A §5.4 |
| FR-056 | Passkey-only authentication: no passwords; SMS/email OTP, magic links and support overrides are never login factors (contact OTP only at sign-up) | Both | P1 | D-16 |
| FR-057 | Native app and mobile PWA login requires two biometric factors: B1 device-bound biometric passkey **and** B2 liveness face match to the account holder (§3.3) | Both | P1 | D-16 |
| FR-058 | Desktop login (browser or desktop PWA) requires two factors: F1 passkey **and** F2 approval from a registered mobile device via B1 (or a second FIDO2 key) (§3.3) | Both | P1 | D-16 |
| FR-059 | A change to the device's biometric enrolment invalidates its passkey and forces a full login (B1 re-registration + B2) | Both | P1 | D-16 |
| FR-060 | Step-up re-authentication for sensitive actions (§3.3); never for Burn, Panic, report, block, cancel or deletion | Both | P1 | D-16 |
| FR-061 | Account recovery needs B2 + fresh V2 matching the stored document anchor + 24 h hold cancellable from registered devices; no staff bypass | Both | P1 | D-16 |
| FR-062 | Session limits: mobile 30 d max with biometric unlock and 2 min idle lock; desktop 12 h max, 15 min idle → full login | Both | P1 | D-16 |
| FR-063 | Brand: logo used exactly as supplied and palette tokens applied on every branded surface; logo never shown in decoy mode, on lock screens, in notifications, app-switcher snapshots, descriptors or Share My Plan | FE | P1 | D-19 |
| FR-064 | Emergency lock from a separate path: with only the contact handle and a one-time emergency code issued at sign-up (no login), freeze all sessions, hide the profile and disable all media links; unlocking needs full recovery (FR-061). Uniform response; abuse only locks, never exposes | Both | P1 | B §3, §4 |
| FR-065 | Location veil: distances shown only as bands, computed from approximate cells whose offset rotates at least daily; never live or exact home/work coordinates | Both | P1 | B §4 |
| FR-066 | Conversation suggestions (openers, boundary questions, respectful replies) are opt-in, generated on device, draft-only and never sent without the member's own action | FE | P3 | B §5, D-20 |
| FR-067 | Timing and availability matching use only member-provided windows, never routines inferred from location or activity | BE | P2 | B §5 |
| FR-068 | Behavioural signals decay over time (default half-life 90 days); members can reset all inferred preferences (FR-024) | BE | P2 | B §6.2 |
| FR-069 | Encrypted media safety: the sending device accepts only allow-listed image/video formats, re-encodes them (stripping active content) before encryption, and shows a consent/NCII warning before any intimate image is sent; public-tier media is malware-scanned on the server | Both | P1 | B §9, §10 |
| FR-070 | Role separation: finance and billing staff cannot access profile, intent or safety data; T&S staff cannot see billing identity; enforced in the ABAC policy engine | BE | P1 | B §9 |
| FR-071 | Viewing reporter-disclosed intimate evidence needs dual approval and a reason code, and is logged immutably | BE | P1 | B §9 |
| FR-072 | Community Rules and enforcement prohibit fetishisation of, and harassment based on, protected characteristics; reportable as `protected_trait_harassment` | Both | P1 | B §10 |
| FR-073 | Public marketing site built for search: server-rendered/static pages, XML sitemaps, canonical tags, hreflang (en-GB, en-IE, later wave-2 locales), schema.org markup (Organization, WebSite, Article, FAQPage, BreadcrumbList), descriptive internal links, a quick-exit button and a neutral-favicon option | FE | P1 | v1.2 |
| FR-074 | Herald SEO agent: keyword and topic research, content briefs and drafts, on-page optimisation, internal-link graph maintenance, content refresh from performance data, rank tracking, broken-link repair, and link-earning outreach drafts; **every publication and outreach message is approved by a human editor** | BE | P1 | v1.2, D-21 |
| FR-075 | Herald is isolated from member data: separate cloud account and network, no credentials for any member store, inputs limited to public web data, Search Console and cookieless aggregate site analytics; no member content, profiles or stories (unless separately consented and anonymised) are ever published | BE | P1 | v1.2 |
| FR-076 | No member surface is indexable: the app origin serves `noindex, nofollow` and robots disallow; there are no public profile URLs; shared links never reveal a member | Both | P1 | v1.2 |
| FR-077 | White-hat link policy: paid links, PBNs, link exchanges, spam, cloaking, doorway pages, unreviewed scaled AI content and fake reviews are prohibited; any paid or sponsored link carries `rel="sponsored"`; the link profile is audited monthly | BE | P1 | v1.2, D-21 |
| FR-078 | Payments run through a processor-agnostic `PaymentProvider` layer with **Stripe as primary** (Checkout, Billing, Customer Portal one-step cancel, Radar, Tax) and a high-risk acquirer as the live backup; Stripe receives only the opaque billing reference and payment data; the neutral descriptor never names TRYST | Both | P1 | v1.2, D-22 |
| FR-079 | Every IntentVector change (member edit or Cartographer re-run) is kept as an append-only, member-viewable version history with audit records | BE | P1 | B §13 |
| FR-080 | Every agent action passes the AI Governance gate before it runs: unknown agents or unlisted actions are denied; `irreversible`, `member_safety`, `money_movement` and `production_change` actions always need a registered named human; autonomy levels L0–L3 and daily budgets cap everything else ([09 §5.4](09_Operating_System.md#54-ai-governance-gate-implemented)) | BE | P1 | founder OS brief, D-25 |
| FR-081 | Agents are declared by manifest (owner, level, actions, data scopes, budget). A member-data scope is refused unless the agent runs inside the member platform on a self-hosted model; no manifest may contain an act-as-member action | BE | P1 | founder OS brief, P4 |
| FR-082 | Agent decisions and admin actions are written to hash-chained, append-only logs anchored hourly to write-once storage; a kill switch per agent and for all agents takes effect immediately and is itself logged | BE | P1 | founder OS brief |
| FR-083 | Member Command Centre: private insights (own activity charts, security activity, "why this slate" from reason codes, what TRYST has learned with reset); no engagement counters or streaks; works with Leave, decoy and emergency lock | Both | P2 | founder OS brief, P1, P5 |
| FR-084 | BitriPay adapter behind `PaymentProvider`: sandbox supported; live keys refused unless the market is enabled for TRYST and BitriPay with a recorded legal opinion; deterministic idempotency keys; only amount, currency and a neutral descriptor are sent; signed webhooks verified | BE | P1 | founder OS brief, D-26 |
| FR-085 | Self-healing: SLO burn triggers incident creation and pre-approved runbooks (rollback, restart, scale out, halt canary) within budget; code or model changes reach production only through a reviewed pull request | BE | P1 | founder OS brief, D-28 |
| FR-086 | Admin Super Control Centre on an internal origin: redacted by default; member-level access only via a case-scoped just-in-time grant approved by a second person (max 8 h); break-glass needs two people; every view and action logged | Both | P1 | founder OS brief |
| FR-087 | Connector register: every third-party connector records purpose, module, data sent and received, lawful basis, DPA, region, health, kill switch and owner; no member data to CRM, messaging-app, enrichment or advertising platforms; live mode needs a named approver | BE | P1 | founder OS brief, D-29 |
| FR-088 | Partner API (licensing): scoped `tk_` keys shown once and stored hashed, ACU metering with idempotent usage records, signed outbound webhooks with retries and replay; no TRYST member data is reachable from any partner scope | BE | P4 | founder OS brief, D-27 |
| FR-089 | Public sign-up in the launch cities (London, Manchester, Birmingham, Brighton) with no invitation required; matching in a city switches on at 2,000 verified members, and members see how many more are needed | Both | P1 | Founder v1.3, D-35 |
| FR-090 | Gender-balance waitlist per city: among solo men and women the larger side may not exceed the cap (2.2:1 in P1, 2.0:1 from P3); breaching members wait first come, first served, see their position and are admitted automatically when balance allows; couples and other genders never wait; departures never reopen seeding; gender never affects price or ranking | BE | P1 | A §16, D-35 |

### 10.2 Non-functional requirements

| ID | Requirement | Owner |
|---|---|---|
| NFR-01 | Erasure < 60 s p99 | BE |
| NFR-02 | Retrieval < 40 ms p95; ranking < 120 ms p95 for 200 candidates; `/v1/slate` < 400 ms p95 | BE |
| NFR-03 | On-device Guardian inline < 80 ms p95 per thread turn | FE |
| NFR-04 | Quick exit < 500 ms; Burn confirmable < 2 s; reveal revocation effective < 5 s p95 | Both |
| NFR-05 | Availability: core API 99.9%; message relay 99.95% | BE |
| NFR-06 | UK/EU residency; primary AWS eu-west-2 | BE |
| NFR-07 | WCAG 2.2 AA (web), platform accessibility APIs (native) | FE |
| NFR-08 | OWASP ASVS L3 (API), MASVS L2 + R (mobile); quarterly pen test; annual red team on A4/A7 | Both |
| NFR-09 | No third-party analytics, advertising or attribution SDKs in any client | FE |
| NFR-10 | Zero standing production access; break-glass needs dual authorisation and writes an immutable log | BE |
| NFR-11 | Logs kept 90 d, IP truncated at ingest | BE |
| NFR-12 | Scale: P3 25k verified; P4 150k; design headroom 500k verified / 50k concurrent | BE |
| NFR-13 | RPO ≤ 5 min, RTO ≤ 1 h; backups stay under per-member keys | BE |
| NFR-14 | Localisation: EN-GB/EN-IE at P1; NL, DE, SV, DA, ES, PT at P4 (Guardian per-language evaluation) | Both |
| NFR-15 | Platforms: iOS 17+, Android 10+ (StrongBox where present), evergreen browsers (last 2 versions) | FE |
| NFR-16 | Login completion p95: mobile < 20 s including liveness; desktop < 30 s including cross-device approval | Both |
| NFR-17 | B2 provider: ISO/IEC 30107-3 PAD Level 2 certified; face-match FAR ≤ 1:10,000 at FRR ≤ 3%; fairness across demographic groups reported by the provider and audited (D-17) | BE |
| NFR-18 | Key management: automatic yearly rotation of KMS root keys and HSM peppers (dual-run); every KMS/HSM key use streamed to the SIEM with anomaly alerts | BE |
| NFR-19 | Assurance: public bug bounty from the end of P1 (public beta); annual independent privacy audit; quarterly pen test and annual red team (NFR-08) | Both |
| NFR-20 | Marketing site Core Web Vitals at p75: LCP < 2.5 s, INP < 200 ms, CLS < 0.1; WCAG 2.2 AA; no third-party tracking scripts (cookieless, self-hosted analytics only) | FE |
