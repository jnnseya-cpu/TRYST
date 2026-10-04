-- GENERATED from docs/spec/02_Shared_Contracts.md §4 by scripts/gen_migrations.py. Do not edit.
CREATE SCHEMA IF NOT EXISTS commerce;
CREATE SCHEMA IF NOT EXISTS ops;

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
