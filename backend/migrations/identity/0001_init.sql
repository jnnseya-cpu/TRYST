-- GENERATED from docs/spec/02_Shared_Contracts.md §4 by scripts/gen_migrations.py. Do not edit.
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
