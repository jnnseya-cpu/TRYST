-- GENERATED from docs/spec/02_Shared_Contracts.md §4 by scripts/gen_migrations.py. Do not edit.
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
