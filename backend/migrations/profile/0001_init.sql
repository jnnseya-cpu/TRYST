-- GENERATED from docs/spec/02_Shared_Contracts.md §4 by scripts/gen_migrations.py. Do not edit.
CREATE EXTENSION IF NOT EXISTS vector;

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
