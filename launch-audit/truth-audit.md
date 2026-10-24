# Truth audit

Every checkable claim in the public journal (`site/content/posts`, 25 live posts; `envoy.md`
is marked "coming") and in the app's own copy was traced to the code that implements it.
Classification: REAL · PARTIAL · STUBBED · ABSENT · BROKEN. Done 24 October 2026; every
non-REAL item was then checked by hand before anything changed.

## Result

* **REAL:** 126 claims across the 25 live posts were implemented end to end and wired into
  the running services.
* **BROKEN, now fixed in code:** phone sign-in face check (D1), duress reported as a missed
  check-in (D2), alert times in UTC (D5).
* **BROKEN or ABSENT, claim removed:** the landing page's agents, deposit, ENVOY, DUO,
  GHOST and Keys plans, TRYST+ extras, app disguise and decoy PIN, screenshot blocking and
  fingerprint-change sign-out (D3); the join page's mobile number (D6); city-by-city
  matching and the waitlist (D14).
* **PARTIAL, fixed in code:** check-in timers lost on restart (D4), message retention
  (D8), known-abuse image matches (D9).
* **PARTIAL, copy corrected:** cryptographic erasure scope (D7) and eight smaller
  over-claims (D18). Five remain PARTIAL but accurately described (R1 to R5 in
  [defects.md](defects.md)).
* **UNVERIFIED (settings, not code):** Stripe statement descriptor, renewal reminder timing,
  email sender name. On the Human Action List (H5, H11).

## What was built to make a claim true

* Phone sign-in face check through a hosted provider session (D1).
* Duress reaches responders as duress (D2).
* Check-ins survive restarts and deploys (D4).
* Alert times in UK time (D5).
* Expired messages swept, and shorter retention applies to messages already sent (D8).
* Known-abuse image matches open a case with an NCA reporting deadline (D9).

## What was removed because it isn't true at launch

* Landing page: negotiating agents, the deposit, ENVOY, DUO, GHOST, Keys, app disguise and
  decoy PIN, screenshot blocking, fingerprint-change sign-out, TRYST+ filters, incognito,
  travel planning and read receipts.
* App: city-by-city matching and the waitlist (D-69).
* Journal: "everything unreadable, including in backups" (photos only; the rest is described).
