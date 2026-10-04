#!/usr/bin/env python3
"""Consistency check for the TRYST specification baseline (docs/v1.1).

Checks:
  1. Every FR/NFR/D/E/G-NG identifier referenced anywhere is defined exactly once.
  2. Every FR row declares an owner (BE, FE or Both) and a phase (P0-P4).
  3. Every relative Markdown link resolves to an existing file and, if it has
     a #fragment, to a heading anchor in that file (GitHub slug rules).

Exit status is non-zero if any check fails.
"""
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
SPEC = ROOT / "docs" / "v1.1"

ID_PATTERNS = {
    "FR": r"\bFR-\d{3}\b",
    "NFR": r"\bNFR-\d{2}\b",
    "D": r"(?<![\w-])D-\d{2}\b",
    "E": r"(?<![\w-])E-\d{2}\b",
    "G-NG": r"\bG-NG-\d+\b",
}
# Where each identifier family is defined: (file, regex matching a definition row).
DEFINITIONS = {
    "FR": ("02_Shared_Contracts.md", r"^\| (FR-\d{3}) \|"),
    "NFR": ("02_Shared_Contracts.md", r"^\| (NFR-\d{2}) \|"),
    "D": ("06_Decisions_and_Changes.md", r"^\| (D-\d{2}) \|"),
    "E": ("06_Decisions_and_Changes.md", r"^\| (E-\d{2}) \|"),
    "G-NG": ("01_Product.md", r"^\| (G-NG-\d+) \|"),
}
LINK = re.compile(r"\[[^\]]*\]\(([^)\s]+)\)")


def slug(heading: str) -> str:
    s = heading.strip().lower()
    s = re.sub(r"[^\w\- ]", "", s)
    return s.replace(" ", "-")


def anchors(path: Path) -> set:
    seen, out = {}, set()
    in_code = False
    for line in path.read_text(encoding="utf-8").splitlines():
        if line.startswith("```"):
            in_code = not in_code
            continue
        m = None if in_code else re.match(r"^#{1,6}\s+(.*)$", line)
        if m:
            base = slug(m.group(1))
            n = seen.get(base, 0)
            out.add(base if n == 0 else f"{base}-{n}")
            seen[base] = n + 1
    return out


def main() -> int:
    errors = []
    files = sorted(SPEC.glob("*.md"))
    if not files:
        print(f"no spec files found in {SPEC}")
        return 1

    defined = {}
    for fam, (fname, pat) in DEFINITIONS.items():
        ids = re.findall(pat, (SPEC / fname).read_text(encoding="utf-8"), flags=re.M)
        dupes = {i for i in ids if ids.count(i) > 1}
        for d in sorted(dupes):
            errors.append(f"{fname}: {d} defined more than once")
        defined[fam] = set(ids)

    fr_rows = re.findall(
        r"^\| (FR-\d{3}) \|[^|]+\| ([^|]+) \| ([^|]+) \|",
        (SPEC / "02_Shared_Contracts.md").read_text(encoding="utf-8"),
        flags=re.M,
    )
    for fr, owner, phase in fr_rows:
        if owner.strip() not in {"BE", "FE", "Both"}:
            errors.append(f"02_Shared_Contracts.md: {fr} has invalid owner '{owner.strip()}'")
        if not re.fullmatch(r"P[0-4]", phase.strip()):
            errors.append(f"02_Shared_Contracts.md: {fr} has invalid phase '{phase.strip()}'")

    anchor_cache = {}
    for f in files:
        text = f.read_text(encoding="utf-8")
        for fam, pat in ID_PATTERNS.items():
            for ref in set(re.findall(pat, text)):
                if ref not in defined[fam]:
                    errors.append(f"{f.name}: reference to undefined {ref}")
        for target in LINK.findall(text):
            if re.match(r"^[a-z]+:", target):
                continue
            path_part, _, frag = target.partition("#")
            dest = (f.parent / path_part).resolve() if path_part else f
            if not dest.exists():
                errors.append(f"{f.name}: broken link -> {target}")
                continue
            if frag and dest.suffix == ".md":
                if dest not in anchor_cache:
                    anchor_cache[dest] = anchors(dest)
                if frag not in anchor_cache[dest]:
                    errors.append(f"{f.name}: missing anchor -> {target}")

    counts = ", ".join(f"{k}={len(v)}" for k, v in defined.items())
    if errors:
        print("\n".join(errors))
        print(f"\nFAILED: {len(errors)} problem(s). Definitions: {counts}")
        return 1
    print(f"OK: {len(files)} files checked. Definitions: {counts}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
