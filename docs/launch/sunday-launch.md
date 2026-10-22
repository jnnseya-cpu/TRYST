# Sunday launch: 25 October 2026

The plan from Thursday 22 to Sunday 25 October. Owner: founder (go/no-go), engineering
lead (everything technical). Setup details are in [../runbooks/go-live.md](../runbooks/go-live.md).

## What "live" can mean on Sunday

There are two levels. Which one happens is decided at the go/no-go on Saturday evening
(§4), not before.

| Level | What members see | Needs |
|---|---|---|
| **A. Public launch** | The website and journal at `www.<domain>`, indexed by search engines. Every join button reads "Opening soon" | Vercel, the domain's DNS. Nothing else |
| **B. Doors open** | Sign-up, verification, introductions, safety line, TRYST+ | Level A, plus every gate in §3 |

Level A is ready now. Level B depends on things only the business can supply: production
**will not start** without an identity provider (`ASSURANCE_URL`) or the alarm-receiving
centre (`ARC_URL`), and must not take members without the legal sign-offs. If any gate is
open on Saturday, launch Level A on Sunday and open the doors the day the last gate closes:
that is one setting on Vercel and one deploy, with no code change.

## 1. Schedule

### Thursday 22 (today)

- [x] Code freeze on `claude/gracious-rubin-bp7b7q`; all checks green: spec, contracts,
      Go (vet, staticcheck, gosec, tests), web (types, 118 tests, build), site (211 tests,
      build), Terraform validate for both stacks.
- [x] Security: Go toolchain to go1.26.10 (GO-2026-6809, crypto/x509); Next.js to 16.4.2
      in the app and the site (critical advisories GHSA-2464-fhr4-w9xw, GHSA-673q-mjm6-r5xr,
      GHSA-p3m5-59mp-jv5r); Node floor 22.23.4. `npm audit`: 0 vulnerabilities.
- [ ] Merge the branch to `main` (pull request, one review).
- [ ] Founder: confirm the status of each gate in §3 and send the missing credentials.

### Friday 23

- [ ] Google Cloud: create the staging and production projects with billing; run
      `deploy/gcp/setup.sh staging` then `setup.sh prod` (go-live.md §2).
- [ ] Hostinger DNS: the A records `setup.sh` prints; SPF, DKIM and DMARC for the mailbox.
- [ ] Google Admin: the safety-team group, 2-Step Verification with security keys only.
- [ ] GitHub: the four Actions variables per environment and a required reviewer on
      production.
- [ ] Vercel: import the repository with root directory `site/`; set
      `NEXT_PUBLIC_SITE_URL`, `NEXT_PUBLIC_APP_URL`, `NEXT_PUBLIC_APP_OPEN=false`; add
      `www.<domain>` and its CNAME at Hostinger.
- [ ] First deploy to staging through the pipeline; every `/readyz` answers.

### Saturday 24

- [ ] Staging smoke test with team accounts and the vendor sandboxes ([../runbooks/staging.md](../runbooks/staging.md) §5).
- [ ] Restore drill on staging; keep the timings.
- [ ] Alert test: stop staging's VM for three minutes; the alert email arrives; the
      Hostinger uptime check emails too.
- [ ] Emails: a sign-up code and a trusted-contact alert land in Gmail and Outlook
      inboxes, not spam.
- [ ] **18:00 go/no-go** (§4).

### Sunday 25

| Time | Level A | Level B (only on a "go") |
|---|---|---|
| 09:00 | Check `www.<domain>` on phone and desktop, certificate, sitemap, feed | Approve the production deploy in GitHub Actions; every `/readyz` answers |
| 10:00 | Submit the sitemap in Google Search Console | Production smoke test with two team members: sign-up, verification, introductions, a meet with a check-in reaching the ARC, report, console sign-in, export, deletion |
| 11:00 | Announce | On a pass: set `NEXT_PUBLIC_APP_OPEN=true` on Vercel and redeploy the site. Doors open |
| All day | | On-call watches alerts and the console ([../runbooks/oncall.md](../runbooks/oncall.md)) |

## 2. Rollback

| Problem | Action | Time |
|---|---|---|
| Bad site release | Vercel → Deployments → previous → Promote | 1 minute |
| Sign-ups must stop | Vercel: `NEXT_PUBLIC_APP_OPEN=false`, redeploy. Existing members unaffected | 2 minutes |
| Bad app release | GitHub Actions → re-run the previous successful deploy | 5–10 minutes |
| Data damaged | Point-in-time restore ([../runbooks/restore.md](../runbooks/restore.md)) | per the drill timings |
| A safety vendor down | The service fails closed (no verification, photos to human review); post on the status channel | at once |

## 3. Gates for Level B

Every box must be ticked by Saturday 18:00. The first four are hard: the service refuses
to start or can't lawfully take members without them.

| # | Gate | Who supplies it | Status |
|---|---|---|---|
| 1 | Identity and age provider: contract, DPA, certificates, sandbox **and** production credentials | Founder / operations | ☐ |
| 2 | Alarm-receiving centre: contract and production endpoint and secret | Founder / operations | ☐ |
| 3 | Legal: counsel opinion, DPIA signed by the DPO, Online Safety Act risk assessments, final Terms and privacy notice ([legal-review-pack.md](legal-review-pack.md) §5) | Counsel, DPO | ☐ |
| 4 | Penetration test on staging with high and critical findings fixed | Pen-test firm | ☐ |
| 5 | Google Cloud projects with billing; safety-team group with security keys | Founder | ☐ |
| 6 | Hostinger mailbox password; SPF, DKIM, DMARC | Founder | ☐ |
| 7 | Staff roster (names and Google accounts of the safety team, at least two people on call) | Founder | ☐ |
| 8 | Stripe approval and Prices. *Optional:* without it TRYST+ isn't sold and launch is free-only | Founder | ☐ |
| 9 | PhotoDNA and Sightengine. *Optional:* without them every photo is checked by a person, which needs the moderators on shift | Operations | ☐ |

Gate 4 normally takes two weeks plus fixes. If it hasn't run, Level B is a **no-go**: the
product's promises (account-holder-only sign-in, discovery privacy, encrypted messages)
need an outside check before members' data is at stake.

## 4. Go/no-go (Saturday 18:00)

Founder, engineering lead and the head of safety. Go only if:

- every gate 1–7 in §3 is ticked;
- the staging smoke test, restore drill and alert test passed;
- two people are on call for Sunday and Monday, with console access working.

Otherwise: Level A on Sunday, and a new date for Level B set from the gate that is still
open.
