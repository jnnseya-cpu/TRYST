#!/usr/bin/env python3
"""Generate backend SQL migrations from docs/spec/02_Shared_Contracts.md §4.

The spec is the source of truth (docs/spec/00_README.md §4). Run with --check in CI to
fail when the committed migrations drift from the spec.
"""
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
SPEC = ROOT / "docs" / "spec" / "02_Shared_Contracts.md"
OUT = ROOT / "backend" / "migrations"

# Spec subsection heading prefix -> (database, file prelude)
TARGETS = {
    "### 4.1 IDENTITY-DB": ("identity", ""),
    "### 4.2 BAN-DB": ("ban", ""),
    "### 4.3 PROFILE-DB": ("profile", "CREATE EXTENSION IF NOT EXISTS vector;\n\n"),
    "### 4.4 SAFETY-DB": ("safety", ""),
    "### 4.5 Commerce and operations": ("profile_commerce_ops",
                                       "CREATE SCHEMA IF NOT EXISTS commerce;\nCREATE SCHEMA IF NOT EXISTS ops;\n\n"),
}
HEADER = "-- GENERATED from docs/spec/02_Shared_Contracts.md §4 by scripts/gen_migrations.py. Do not edit.\n"


def extract() -> dict[str, str]:
    text = SPEC.read_text(encoding="utf-8")
    out = {}
    for heading, (db, prelude) in TARGETS.items():
        start = text.index(heading)
        block = re.search(r"```sql\n(.*?)```", text[start:], flags=re.S)
        if not block:
            raise SystemExit(f"no sql block after {heading}")
        out[db] = HEADER + prelude + block.group(1)
    return out


def main() -> int:
    check = "--check" in sys.argv
    rc = 0
    for db, sql in extract().items():
        path = OUT / db / "0001_init.sql"
        if check:
            if not path.exists() or path.read_text(encoding="utf-8") != sql:
                print(f"DRIFT: {path.relative_to(ROOT)} differs from spec; run scripts/gen_migrations.py")
                rc = 1
        else:
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(sql, encoding="utf-8")
            print(f"wrote {path.relative_to(ROOT)}")
    if check and rc == 0:
        print("OK: migrations match spec")
    return rc


if __name__ == "__main__":
    sys.exit(main())
