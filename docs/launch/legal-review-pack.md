# Legal review pack

What counsel and the DPO need to review before launch, and where each item is in the
product. It collects the open questions from the Terms draft (05 §29) and the decisions made
while building (06, D-60 to D-65). It is a brief for advisers, not legal advice.

Launch shape: UK only (London first), web app, Verified (free) and TRYST+ on sale; DUO,
ENVOY, GHOST, Keys packs and the deposit built but not sold (D-60).

## 1. Documents to review

| Document | Where |
|---|---|
| Terms of Service draft (incl. Community Rules, Safety Policy references) | `docs/spec/05_Terms_of_Service_draft.md` |
| Privacy notice | To be written from 02 §7 (retention), §4 (data model) and this pack |
| DPIA | To be completed: large-scale special-category data, systematic evaluation, biometrics (01 §14.2) |
| Online Safety Act risk assessments (illegal content; children's access) | To be completed from 03 §7 (safety), FR-123, FR-124 |
| Records of processing (Art. 30) | From the vendor list in [vendor-checklist.md](vendor-checklist.md) |
| Consent wording, versioned | `frontend/lib/consent.ts` (`CONSENT_COPY`) and `backend/internal/consent` (`Versions`) |
| Conduct declaration (ToS 3.5) | `frontend/app/verify/page.tsx`, version `tos-3.5/2026-10` |

## 2. Open questions from the Terms draft (05 §29)

Q1 to Q16 stand as written in 05 §29. The build has added facts that bear on them:

| Q | New facts from the build |
|---|---|
| Q1, Q13 | The document code is an HMAC of the ID number under a pepper, made on arrival; the number is discarded (D-61). It's created for every verified member (one account per document), deleted on erasure, and kept only for permanent removals as a ban (`ban` records) |
| Q7 | Messages are end-to-end encrypted (D-51); moderation relies on member reports with chosen messages and on-device signals (FR-114, FR-123) |
| Q12 | Discovery, Tonight, introductions and conversations all need V2 in every environment, including the over-18 re-check TRYST makes itself (FR-124). Under-18s are refused and signed out at once |
| Q14, Q15 | The face check now needs a separate `biometric_login` consent, recorded in the ledger before any check (FR-126). Sign-in on a computer with two security keys stays the alternative |
| Q16 | Partner shield is built: contacts are hashed on the device and keyed on the server; nothing readable is stored (FR-127, D-64). A match hides both members silently |

## 3. New questions from the build

| # | Question | Where |
|---|---|---|
| N1 | On account deletion the consent ledger is **deleted** rather than kept for the "statutory period" in 02 §7. Is that right, given nothing is processed on those consents afterwards? | D-63 |
| N2 | Article 9 consent is asked separately from the Terms and is required for discovery and introductions; withdrawal removes the member at once. Is the wording (`CONSENT_COPY.art9_processing`) explicit enough, and is making discovery conditional on it "freely given"? | FR-126 |
| N3 | Account recovery uses a fresh face check and ID check on the same document, then a 24-hour hold. No staff override exists. Is "we cannot recover your account for you" acceptable for complaints handling and the DMCC subscription regime (a locked-out subscriber)? | FR-061, FR-125 |
| N4 | Emergency lock can be triggered by anyone holding the contact and the one-time code, without signing in. It can only lock, never expose. Any consumer-law issue with a lock the member didn't make, for a paying member? | FR-064, D-64 |
| N5 | Couples: a link changes how both partners appear to others ("couples") and gives each a say on introductions under a veto mode both agreed. Does the partner's role in another member's handshake make TRYST a joint processor of anything, or need its own notice? | FR-128, D-65 |
| N6 | Introductions use a weighted score and show 2–4 reason codes per card; nothing is paid placement. Is this "profiling" under Art. 22 (it has no legal or similarly significant effect, and members confirm every input)? What does the privacy notice need to say? | FR-128 |
| N7 | Intents cost Keys only beyond 3 free a week and are refunded in full on any reply. At launch Keys can't be bought. When Keys go on sale, are they "digital content" and how do cancellation rights apply to Keys held for an unanswered intent? | D-60, FR-013 |
| N8 | Inbound cap: overflow intents are queued silently and senders never see a decline. Any transparency issue in not telling a sender that their intent hasn't been delivered yet? | FR-013 |
| N9 | Reporting confirmed child sexual exploitation to the NCA: confirm the statutory regime and commencement, and the 24-hour internal deadline (D-59) | FR-123 |
| N10 | The ban list keyed by document code survives erasure as the declared exception in ToS 15.3; confirm the Sch. 1 condition (overlaps Q1) | D-61 |

## 4. Contracts counsel should see

The data processing agreements and security schedules in [vendor-checklist.md](vendor-checklist.md),
especially: the identity provider (biometrics, ID data, deletion timing against our
60-second erasure, Q14); the alarm-receiving centre (location data in an emergency, ToS
20); Stripe (payments, the neutral statement descriptor).

## 5. Sign-off needed before launch

- [ ] Written UK counsel opinion (G-P0, 01 §11).
- [ ] DPIA complete and signed by the DPO; ICO prior consultation decided.
- [ ] OSA risk assessments complete.
- [ ] Terms, privacy notice, Community Rules and Safety Policy final, and versions recorded
      so the conduct declaration and consents point at them.
- [ ] Answers to Q1–Q16 and N1–N10 recorded in 06 (decision log) with owners.
