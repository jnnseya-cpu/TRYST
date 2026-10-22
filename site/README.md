# site: the public website and journal

A static website (no server, no cookies, no trackers, no member data) that explains every
TRYST feature and sells it honestly. It's separate from the app (FR-075): the app is never
indexed by search engines; this site is meant to be found. Hosted on **Vercel**.

```sh
npm ci
npm run dev        # http://localhost:3200
npm test           # the editorial rules below
npm run build      # static HTML in out/
```

## Adding or changing a post

Every Markdown file in `content/posts/` becomes a page, a card on the home page and its
topic page, a line in "Why TRYST, in one line each", and an entry in the sitemap and RSS
feed. Copy an existing post and change the front matter:

```yaml
---
title: The feature, as a promise
description: 70–160 characters for search results.
date: 2026-10-22
topic: discretion | security | verification | connections | safety | data | membership
usp: One line, at most 140 characters, that says why it's different.
status: live | coming
order: 27
---
```

The body must include a `## Why it's different` section (the selling point) and a
`## What it doesn't do` section (the honest limits).

## The rules (enforced by `npm test`; CI refuses a post that breaks one)

* Only what is built and switched on today is sold. Features that are built but not on
  sale (ENVOY, DUO, GHOST, Keys) appear only as "not available yet", with `status: coming`.
* Claims discipline (spec 01 §15.1): never "safe", "vetted", "background-checked",
  "guaranteed" or "no fakes"; say "identity checked", "age assured".
* No comparative superlatives ("the only", "nobody else", "the best"): they need evidence
  about every competitor (CPUT, DMCC Act).
* Never the words "AI" or "artificial intelligence" (01 §3.3).
* Prices only as in the launch catalogue (£24.99, £14.99, £179.88).
* British English. Internal links must point at posts that exist.

When a feature changes in the app, change its post in the same pull request.

## Vercel

New project → import the repository → **Root Directory `site`** (framework: Next.js).
Environment variables: `NEXT_PUBLIC_SITE_URL` (for example `https://www.<domain>`) and
`NEXT_PUBLIC_APP_URL` (`https://<domain>`, where "Join" links go). Point `www.<domain>` at
Vercel in Hostinger's DNS. Security headers come from `vercel.json`.
