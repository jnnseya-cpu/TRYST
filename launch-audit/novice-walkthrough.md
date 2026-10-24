# Novice walkthrough

The first-time member: on a mid-range phone, on a slow or patchy connection, no one to
explain anything, leaves within 30 seconds if confused.

**Scope of this cycle.** Automated, not a person: Chromium at 360 × 740 with touch, emulated
slow 3G (400 kbps, 400 ms latency), axe accessibility rules, and the 24 browser journeys.
The product is UK-only and English-only at launch, so French and other languages are N/A.
A timed walkthrough by a real person on real iPhone and Android devices has **not** been
done and is required before beta (see below).

## Friction found and fixed

| Where | What a novice hit | Fix |
|---|---|---|
| Landing page | 27 s to see anything on a slow connection: they'd leave | 1.1 MB logo replaced by a 58 KB copy; under 8 s (D10) |
| Landing page | Prices and plans that can't be bought (deposit, ENVOY, DUO, GHOST, Keys) | Two plans only, both real (D3) |
| Join | Asked for "email or mobile number (international format)", then refused a phone | "Email address", email keyboard (D6) |
| Meet check-in | Typing the next PIN while the last check-in was still sending wiped it; "I'm home safe" then failed | PIN cleared as it's sent; buttons wait (D11) |
| Messages | Double tap sent twice; text typed during a slow send vanished | Sends once; text kept or restored (D12) |
| Phone sign-in | Would have been impossible in production | Hosted face check in a new tab, with "I've finished the check" if the tab didn't open (D1) |

## Still to check by hand (before beta)

| Journey | What to watch for |
|---|---|
| Sign-up to first introduction, cold, on an Android phone on 3G, timed | Every hesitation; time to the first introduction (target under 10 minutes, verification included) |
| The hosted face and ID checks with the real provider on iPhone Safari | The new tab opening and returning; what happens if the member switches apps mid-check |
| Opening the sign-up code email on a different device | The code still works; the wording is clear |
| A forgotten passkey on day two | Recovery is findable from the sign-in page, and its 24-hour hold is understood |
| Screen reader (VoiceOver, TalkBack) and keyboard-only through sign-up, a message and Panic | Every control announced; Panic reachable |
| Empty states: first day with no introductions, no messages | A clear next step on each screen |
