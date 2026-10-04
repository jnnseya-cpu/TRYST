-- GENERATED from docs/spec/02_Shared_Contracts.md §4 by scripts/gen_migrations.py. Do not edit.
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
