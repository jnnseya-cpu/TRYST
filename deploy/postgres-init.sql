-- One database per domain (D-50); the erasure ledger is separate so it can be backed up and
-- restored on its own (D-57).
CREATE DATABASE tryst_identity;
CREATE DATABASE tryst_profile;
CREATE DATABASE tryst_keys;
CREATE DATABASE tryst_safety;
CREATE DATABASE tryst_commerce;
CREATE DATABASE tryst_ledger;
