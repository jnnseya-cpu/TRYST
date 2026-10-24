# Human Action List

What only a person can do. The platform can't launch to members until H1 to H7 are done.
Each says what I will do the moment it is.

| ID | Action | Owner | Why it blocks | When done, I will |
|---|---|---|---|---|
| H1 | Contract an identity and age provider (recommended: Yoti, with Veriff as second) and send its API docs plus sandbox keys | Founder | Production refuses to start without `ASSURANCE_URL`; nobody can be verified or sign in on a phone (D1 needs a real-provider test) | Write the gateway mapping to its API (about a day), run sign-up, V1, V2, phone sign-in and recovery against its sandbox on staging |
| H2 | Contract an alarm-receiving centre, or decide to launch without the 24/7 safety line | Founder | Production refuses to start without `ARC_URL` (D-48). Making it optional was blocked by this session's permission check and needs your explicit approval | Either wire its endpoint and test a missed check-in, duress and Panic end to end, or (on your approval) make the safety line optional and hide it |
| H3 | Create the Google Cloud projects with billing; run `deploy/gcp/setup.sh prod` (and `staging` if wanted) | Founder | Nothing is deployed, so nothing about the live system is tested | Run the deployed checks: smoke test, restore drill on Cloud SQL, alert test, deploy and rollback, load test |
| H4 | Staff: the safety team's Google group (at least 3 people, 1 lead) with 2-Step Verification on security keys; the roster file | Founder | The safety service refuses to start without a roster; reports can't be handled | Run preflight and a real console sign-in |
| H5 | Hostinger mailbox password, and SPF, DKIM and DMARC records | Founder | Sign-up codes and trusted-contact alerts are email-only | Send test codes and alerts to Gmail and Outlook and check they land in inboxes |
| H6 | Legal: counsel opinion, DPIA signed by the DPO, Online Safety Act risk assessments, final Terms and privacy notice ([../docs/launch/legal-review-pack.md](../docs/launch/legal-review-pack.md)) | Founder, counsel, DPO | Processing special-category data without these is unlawful; the footer promises the documents at launch | Wire the final documents and versions into the consent and conduct screens |
| H7 | Independent penetration test on staging, high and critical findings fixed | Founder (buy), engineering (fix) | Open P0/P1 are an automatic NO-GO, and no outside party has tested the sign-in, discovery-privacy and encryption claims | Fix and retest every finding |
| H8 | Decide: wire city-by-city matching and the waitlist (D-35, D-36), or keep all four cities open from day one | Founder | The code exists but isn't running; copy no longer claims it | Wire it, or remove the unused package |
| H9 | Optional: Stripe approval and Prices | Founder | Without it launch is free-only (TRYST+ not sold) | Set prices, run checkout, cancel and refund in test mode |
| H10 | Optional: PhotoDNA (with IWF membership) and Sightengine | Operations | Without them every photo waits for a person, and known-abuse hash matching doesn't run | Wire keys and test with the vendors' test images |
| H11 | Set Stripe's statement descriptor to a neutral name and renewal reminders to at least 7 days (if H9) | Founder | Billing page promises both; they're Stripe settings, not code | Check them in Stripe test mode |
| H12 | Merge `claude/gracious-rubin-bp7b7q` to `main` | Founder | Automatic deploys run from `main` | Open the pull request on request |
