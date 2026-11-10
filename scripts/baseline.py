#!/usr/bin/env python3
"""The baseline lock: nothing built so far may change or disappear without it being recorded.

baseline/manifest.json lists every locked item with a fingerprint: spec decisions and
requirements, API operations, backend routes, tests, pages, journal posts, the message
catalogue, vendor keys, prices, the growth profile and brief contract, CI workflows,
deployment files and the key documents.

    python3 scripts/baseline.py                         check (CI runs this on every push)
    python3 scripts/baseline.py --update --reason "…"   accept the current state on purpose

A check fails when a locked item is removed, changed or added without an update. An update
rewrites the manifest and appends what changed, with the reason, to baseline/CHANGES.md, so
every change to what was built is visible in review. Ordinary code edits are not locked;
what they deliver is, through the contracts, routes, catalogues and tests.
"""
import argparse
import datetime
import glob
import hashlib
import json
import os
import re
import sys

import yaml

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
MANIFEST = os.path.join(ROOT, "baseline", "manifest.json")
CHANGES = os.path.join(ROOT, "baseline", "CHANGES.md")
PRESENT = "present"


def h(text):
    return hashlib.sha256(text.encode("utf-8")).hexdigest()[:16]


def read(rel):
    with open(os.path.join(ROOT, rel), encoding="utf-8") as f:
        return f.read()


def files(pattern):
    return sorted(os.path.relpath(p, ROOT) for p in glob.glob(os.path.join(ROOT, pattern), recursive=True)
                  if "node_modules" not in p and "/.next/" not in p and "/out/" not in p)


def collect():
    items = {}

    # Spec: every decision, requirement, erratum and no-go, by its row.
    defs = {"02_Shared_Contracts.md": r"^\| ((?:FR-\d{3})|(?:NFR-\d{2})) \|",
            "06_Decisions_and_Changes.md": r"^\| ((?:D-\d{2})|(?:E-\d{2})) \|",
            "01_Product.md": r"^\| (G-NG-\d+) \|"}
    for f, pat in defs.items():
        for line in read("docs/spec/" + f).splitlines():
            m = re.match(pat, line)
            if m:
                items["spec:" + m.group(1)] = h(line)

    # Documents whose wording is relied on (spec, runbooks, launch, security, audit).
    for f in (files("docs/spec/*.md") + files("docs/runbooks/*.md") + files("docs/launch/*.md")
              + files("docs/security/*.md") + files("launch-audit/**/*.md")):
        items["doc:" + f] = h(read(f))

    # API contracts: every operation, by its definition.
    for f in files("shared/contracts/openapi/*.v1.yaml"):
        spec = yaml.safe_load(read(f))
        for path, ops in (spec.get("paths") or {}).items():
            for method, op in ops.items():
                if method in ("get", "post", "put", "patch", "delete"):
                    items[f"api:{os.path.basename(f)} {method.upper()} {path}"] = h(json.dumps(op, sort_keys=True, default=str))

    # Backend routes actually registered.
    for f in files("backend/**/*.go"):
        if f.endswith("_test.go"):
            continue
        for m in re.finditer(r'HandleFunc\("((?:GET|POST|PUT|PATCH|DELETE) [^"]+)"', read(f)):
            items["route:" + m.group(1)] = PRESENT

    # Tests: none may be dropped.
    for f in files("backend/**/*_test.go"):
        for m in re.finditer(r"^func (Test\w+)\(", read(f), re.M):
            items[f"test:go:{os.path.dirname(f)}.{m.group(1)}"] = PRESENT
    for f in files("frontend/e2e/*.spec.ts") + files("frontend/test/**/*.ts") + files("site/test/**/*.ts"):
        for m in re.finditer(r'^\s*(?:test|it)\(\s*["`]([^"`]+)["`]', read(f), re.M):
            items[f"test:web:{f}: {m.group(1)}"] = PRESENT
    for f in files("agents/tests/*.py"):
        for m in re.finditer(r"^def (test_\w+)\(", read(f), re.M):
            items[f"test:py:{f}.{m.group(1)}"] = PRESENT

    # Pages of the app, the console and the public site.
    for f in files("frontend/app/**/page.tsx") + files("site/app/**/page.tsx"):
        items["page:" + f] = PRESENT

    # Journal posts, by content.
    for f in files("site/content/posts/*.md"):
        items["post:" + os.path.basename(f)] = h(read(f))

    # Growth intelligence (D-73): the profile (approved claims, prohibited wording, topics,
    # pages) and the Growth Brief contract with its fixtures, by content.
    for f in files("backend/internal/growth/*.json") + files("shared/contracts/growth/**/*"):
        if os.path.isfile(os.path.join(ROOT, f)):
            items["growth:" + f] = h(read(f))

    # Every message TRYST sends, by its catalogue entry.
    comms = read("backend/internal/comms/comms.go")
    for m in re.finditer(r'\{Key: "([^"]+)"(.*?)\},\n', comms, re.S):
        items["message:" + m.group(1)] = h(m.group(0))
    for m in re.finditer(r'\{"([^"]+)", "([^"]+)"\},', comms):
        items["never-sent:" + m.group(1)] = h(m.group(2))

    # Vendor keys the console manages.
    for m in re.finditer(r'\{Name: "([A-Z0-9_]+)".*', read("backend/internal/vendorkeys/vendorkeys.go")):
        items["vendor-key:" + m.group(1)] = h(m.group(0))

    # Prices shown anywhere a member or visitor reads them.
    for f in files("frontend/app/**/*.tsx") + files("site/content/posts/*.md") + files("site/app/**/*.tsx"):
        for p in set(re.findall(r"£\d+(?:\.\d\d)?", read(f))):
            items[f"price:{p} in {f}"] = PRESENT

    # CI gates, deployment and infrastructure: no silent weakening.
    for f in (files(".github/workflows/*.yml") + files("deploy/gcp/**/*") + files("deploy/hostinger/*")
              + files("deploy/monitoring/*") + files("scripts/*.sh") + ["frontend/Dockerfile", "backend/Dockerfile",
              "backend/go.mod", "frontend/package.json", "site/package.json", "site/vercel.json", "frontend/next.config.ts"]):
        path = os.path.join(ROOT, f)
        if os.path.isfile(path) and not f.endswith((".tfstate", ".lock.hcl")) and "/.terraform/" not in f and not f.endswith(".secrets.env"):
            items["build:" + f] = h(read(f))
    return dict(sorted(items.items()))


def diff(old, new):
    removed = sorted(k for k in old if k not in new)
    added = sorted(k for k in new if k not in old)
    changed = sorted(k for k in new if k in old and old[k] != new[k])
    return removed, changed, added


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--update", action="store_true")
    ap.add_argument("--reason", default="")
    a = ap.parse_args()
    new = collect()
    old = {}
    if os.path.exists(MANIFEST):
        with open(MANIFEST, encoding="utf-8") as f:
            old = json.load(f)["items"]
    removed, changed, added = diff(old, new)

    if a.update:
        if not a.reason.strip():
            sys.exit("--update needs --reason: say why the baseline changes")
        if not (removed or changed or added) and old:
            print("baseline unchanged")
            return 0
        os.makedirs(os.path.dirname(MANIFEST), exist_ok=True)
        with open(MANIFEST, "w", encoding="utf-8") as f:
            json.dump({"items": new, "count": len(new)}, f, indent=1, ensure_ascii=False)
            f.write("\n")
        day = datetime.date.today().isoformat()
        entry = [f"\n## {day}: {a.reason.strip()}\n", f"{len(new)} locked items. "
                 f"Removed {len(removed)}, changed {len(changed)}, added {len(added)}.\n"]
        for title, keys in (("Removed", removed), ("Changed", changed), ("Added", added)):
            if keys and old:
                entry.append(f"\n**{title}:**\n\n" + "".join(f"- `{k}`\n" for k in keys))
        new_file = not os.path.exists(CHANGES)
        with open(CHANGES, "a", encoding="utf-8") as f:
            if new_file:
                f.write("# Baseline changes\n\nEvery change to what has been built, with the reason. Written by "
                        "`scripts/baseline.py --update`; never edit by hand.\n")
            f.write("".join(entry))
        print(f"baseline updated: {len(new)} items (removed {len(removed)}, changed {len(changed)}, added {len(added)})")
        return 0

    if not old:
        sys.exit("no baseline yet: run scripts/baseline.py --update --reason \"first baseline\"")
    if not (removed or changed or added):
        print(f"OK: baseline holds ({len(new)} locked items)")
        return 0
    for title, keys in (("REMOVED (dropped without a recorded decision)", removed), ("CHANGED", changed), ("ADDED", added)):
        if keys:
            print(f"{title}: {len(keys)}")
            for k in keys[:200]:
                print("  " + k)
    print("\nIf this is intended, record it: python3 scripts/baseline.py --update --reason \"<why>\"")
    return 1


if __name__ == "__main__":
    sys.exit(main())
