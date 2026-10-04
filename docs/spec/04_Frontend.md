# 04 — TRYST Frontend Specification v1.1

Part of the [TRYST v1.1 baseline](00_README.md). Consumes the contracts in [02](02_Shared_Contracts.md). Product intent is in [01](01_Product.md).

---

## 1. Scope

This document covers the native iOS and Android apps, the web PWA, the shared client crypto core, on-device models, UX flows, discretion and safety UI, copy rules, accessibility and store compliance. It does not redefine API shapes, enums or retention; those live in 02.

---

## 2. Two-surface strategy

Apple Guideline 1.1.4 and Google Play's age-restricted and anonymous-chat policies make two surfaces a **requirement** [A §14.3]:

| | Native store builds (iOS, Android) | Web PWA |
|---|---|---|
| Role | Discreet companion: discovery, Briefs, threads, meets, safety, discretion | **The full product** and the **primary revenue surface** |
| Content | **Sanitised.** No nudity, no explicit imagery or copy in the app, listing or screenshots | Explicit media allowed **behind V2 only**, never shown during verification |
| Messaging | Matched only — never random or anonymous chat | Same |
| Billing | Web-first; in-app purchase only if a store requires it ([D-11](06_Decisions_and_Changes.md#2-decision-log)) | All billing, vouchers, deposit |
| Positioning | "Discreet dating"; 18+; Play Console minor-blocking on; Apple age questionnaire done | Full brand |
| Discretion strength | **Strongest:** decoy skin, duress PIN, Secure Enclave/StrongBox, FLAG_SECURE, backup exclusion | **Reduced** (browser limits): quick exit, neutral title/favicon option, session lock, private-browsing guidance |

Explicit media sent from the web is delivered to native clients as a placeholder ("View on web"). It is never rendered in the store build (FR-051).

**Store basics** that cause most rejections: in-app account deletion, a web deletion link, privacy manifests, a working reviewer demo account (sanitised seeded data), and a Data Safety form that exactly matches the Privacy Notice.

---

### 2.1 Public marketing site and SEO surface (v1.2)

- **Separate host** from the app (e.g., `www.` marketing vs `app.` product) so search indexing, caching and analytics never touch member surfaces (FR-076).
- **Next.js SSG/ISR** with a headless CMS fed by Herald drafts after editor approval (FR-073, FR-074). XML sitemaps, canonical tags, hreflang, schema.org, clean URLs and breadcrumb navigation. City pages exist only for launched or waitlisted cities and carry real local content (waitlist status, safety resources), never doorway pages.
- **Performance and accessibility:** Core Web Vitals and WCAG targets in NFR-20. Brand tokens and the logo per [§7.2](#72-brand-application-v12).
- **Discretion on a public site:** quick exit, a neutral-favicon option, no third-party trackers or pixels, and cookieless analytics. "Join" leads to the app origin, which is `noindex`.

## 3. Client stacks

*Source: A §12*

| Surface | Stack | Notes |
|---|---|---|
| iOS | **Swift / SwiftUI**, iOS 17+ | Keys in Secure Enclave; `NSFileProtectionComplete`; files excluded from backup; alternate app icons for the decoy |
| Android | **Kotlin / Jetpack Compose**, Android 10+ | Keys in StrongBox where present (TEE fallback); `FLAG_SECURE`; `allowBackup=false` and data-extraction rules; activity-alias for the decoy |
| Web | **Next.js PWA** (installable), TypeScript | No third-party scripts; strict CSP; non-extractable WebCrypto keys in IndexedDB; service worker caches no personal data |
| Crypto core | **Rust** (OpenMLS, AES-GCM, OPRF client) compiled to Swift (UniFFI), Kotlin (JNI) and WASM | One audited implementation across all three surfaces |
| On-device ML | Core ML (iOS), TFLite/NNAPI (Android), ONNX Runtime Web (WASM/WebGPU) | Guardian M6a–e, nudity pre-send check, conversation-feature extraction |
| API client | Generated from `contracts/openapi` | DPoP proof per request, signed with the device key |

**React Native is not used.** Source A rejects cross-platform shells because Secure Enclave/StrongBox, FLAG_SECURE, screenshot detection, decoy skin and backup exclusion are unreliable through them ([E-07](06_Decisions_and_Changes.md#3-errata-fixed-in-v11)).

**Client non-negotiables:**
- **NFR-09** — no third-party analytics, advertising, attribution or crash SDKs that send data off-platform. Crash reporting is self-hosted with scrubbed payloads.
- **No social login. No contact upload** — contacts are only ever blinded on device for PSI.

---

## 4. Screens and flows (by loop)

### 4.0 Sign-in — account holder only (v1.2)

Implements [02 §3.3](02_Shared_Contracts.md#33-login-assurance--account-holder-only) (FR-056–FR-062).

**Native app and mobile PWA — two biometric factors:**
1. **B1:** a system biometric prompt (Face ID / Touch ID / BiometricPrompt `BIOMETRIC_STRONG`) unlocks the device-bound passkey. Device-passcode fallback is **disabled** for login.
2. **B2:** an in-app liveness capture with a randomised challenge (turn your head, read a short phrase) from the provider SDK (native) or web flow (PWA). The camera view uses brand colours with the `night` background. No logo is shown if a decoy skin is active.
3. The session is issued only after both succeed. Failure copy never says which factor failed.

**Desktop browser / desktop PWA — two factors:**
1. **F1:** the browser passkey prompt (Windows Hello / Touch ID / security key).
2. **F2:** a QR code appears on screen. The member opens TRYST on their phone, goes to **Approve sign-in**, scans it, and confirms with B1. The phone shows the desktop's browser, approximate city and time. Nothing is pushed to the phone's lock screen. Alternative: tap a second registered security key.

**Other screens:**
- **Biometric change detected:** "Your phone's fingerprints or face settings changed, so we've signed you out to protect your account." Then a full B1 re-registration + B2 login.
- **Unlock** (existing session): biometric or PIN per FR-027. The duress PIN opens the decoy.
- **Step-up sheet** for sensitive actions (02 §3.3). **Burn, Panic, report, block, cancel and delete never show a step-up.**
- **Recovery:** liveness → age/ID check with the same document → a 24 h countdown that any registered device can cancel → new passkey. The copy explains that support cannot bypass this.
- **Consent:** a separate `biometric_login` explicit-consent screen before V1, alongside the Article 9 consent, with a link to the accessible alternative (D-17).

### 4.1 Loop A — Quiet entry and intake

1. **Neutral landing / first launch.** On native, the member picks a decoy skin and neutral name at first launch.
2. **Contact (V0).** Email or phone OTP. Device key generated in the secure element; attestation sent.
3. **Discretion setup** (before any personal data): app PIN/biometric, duress PIN, notification mode (default **off**), Burn gesture, home/work zones (map picker → geohash-6 cells), and an honest **"What discretion can't cover"** card (P9).
4. **Liveness (V1)** through the provider SDK/web flow.
5. **Age assurance (V2)** with the ID match, then the **conduct declaration** (separate screen, logged) and the **Article 9 explicit consent** (separate from the ToS accept).
6. **Cartographer interview** (~7 min, conversational). Each proposed structured field is shown as an editable chip; the member confirms (FR-003). Desire tags offer three levels: yes / curious / no. Hard limits come from a controlled list with plain-language descriptions.
7. **ExclusionRing enrolment** (P2+). Opt-in. Explains that contacts are blinded on the device and that nobody learns who matched. Runs the OPRF client.
8. **Profile:** pseudonym, pronouns, free-text gender, age band (from assurance), photos. EXIF is stripped on device and face-blur is on by default; reveal ladder defaults come from the Discretion Policy.
9. **Safety notice** (FR-054) → first slate, or the waitlist if the city is gated.

**Couples (VC).** Partner A creates the couple and shows a QR or link (24 h). Partner B must complete V2 **on their own device**. Both review and approve the exact profile version. Veto mode is chosen jointly. The UI explains that either partner can dissolve the couple at any time without giving a reason.

### 4.2 Loop B — Discovery

- **Slate view:** up to 18 cards; no infinite scroll, no swipe mechanic.
- **Each card shows:** pseudonym, age band, distance band, modes, verification tier, and 2–4 reason chips with a "not quite" correction control (FR-025). Paid placement is labelled visibly.
- **Actions:** Send Intent (shows the Keys cost and the "refunded if they reply" promise), Pass, Save.
- Intent sending is **never** blocked by the counterparty's inbound cap, and the UI never reveals it.
- **My Signals** (P2): categories the system has learned, with correct, reset and personalisation on/off (FR-024).

### 4.3 Loop C — Handshake

- **Brief screen:** a persistent **"Prepared by Envoy (automated)"** label; aligned items; friction items with suggested resolutions; blocking items; estimated meet probability shown as a band (low / medium / high), never a precise number to the member.
- **Decisions:** Accept / Decline / Amend (amend chooses alternative values from the schema; no free text).
- **Couples:** the decision UI reflects the veto mode (e.g., "Waiting for your partner").
- **ENVOY tier** (P3): a proposal inbox where every proposed intent needs **one-tap approval**. Nothing is sent automatically (D-04).

### 4.4 Loop D — Thread and resolution

- **Thread (MLS):** text, view-once/timed media, voice/video calls (reveal-ladder gated). Retention selector (7/30/90 days; the shortest wins). Burn-thread action.
- **Reveal ladder control:** grant or revoke per recipient and scope. A revoked item immediately shows as unavailable on the recipient's device.
- **Meet planner:** propose/confirm. Safe Meet checklist (public venue first). Check-in timer. **Share My Plan** (FR-021). Clare's Law explainer before the first meet (P3).
- **Aftercare** (48 h): a two-tap private prompt (happened? as represented? would repeat? + optional safety flag).

### 4.5 Account

Pause, Vanish, Delete (shows the erasure promise **and** the declared exceptions in plain words), Export, Devices, Consents, Subscription (one-step cancel), Vouchers, Appeals.

---

## 5. Client discretion controls

| Control | Native | Web | Req |
|---|---|---|---|
| **Decoy skin** — generic utility (calculator / notes) with a neutral icon and name; the real app needs a distinct PIN or biometric | ✓ | Neutral title/favicon option only | FR-027 |
| **Duress PIN** — opens a populated, innocuous decoy state; silent | ✓ | — | FR-027 |
| **Quick exit** — one action hides content and clears the view in < 500 ms (web: replace history and navigate to a neutral page) | ✓ | ✓ | FR-029 |
| **Emergency lock** — if a phone is lost or taken, the member visits a neutral web page and enters their contact handle + emergency code (shown once at sign-up, to store offline) to freeze everything (FR-064) | — | ✓ | FR-064 |
| **Location veil** — distance bands only, from rotating cells (FR-065) | ✓ | ✓ | FR-065 |
| **Burn** — long-press, confirm in < 2 s: wipe local storage, revoke device keys (`POST /v1/burn`), sign out everywhere. **Visually and gesturally distinct from Panic** | ✓ | ✓ | FR-028, E-21 |
| **Notification masking** — off by default; if on, the payload is content-free ("1 update") and the push contains no name, preview or image | ✓ | ✓ (web push) | FR-026 |
| **Screenshot protection** — Android `FLAG_SECURE`; iOS capture detection that blurs content and notifies the counterparty; obscured app-switcher snapshot | ✓ | Best-effort blur on visibility change | FR-030 |
| **No cloud backup** — excluded from iCloud/Google backup; keys non-exportable | ✓ | Keys non-extractable | FR-030 |
| **Zone awareness** — coarse client location checked against zones on device; "You're inside your work zone — hiding profile for 2 h" | ✓ | Optional | FR-011 |
| **Jurisdiction auto-lock** — entering a blocked country switches to the decoy and hides the profile | ✓ | Edge-enforced | FR-031 |
| **Watermarking** — recipient-specific visible/invisible watermark on revealed media; no-download viewer | ✓ | ✓ | A §9.1 A5 |

Location never leaves the device at better than geohash-5, except in the Panic flow.

---

## 6. Client cryptography and on-device intelligence

- **Device identity:** a key pair generated in the secure element, used for DPoP proofs and as the MLS credential. Rotated on re-install; revoked by Burn.
- **Messaging:** OpenMLS. One MLS group per thread (2, 3 or 4 members). Key packages are uploaded to the delivery service.
- **Media:** a per-media AES-256-GCM key; ciphertext uploaded through a pre-signed URL; the key is wrapped per recipient (MLS exporter secret) and sent in band. **Revocation** asks the server to destroy the wrapped key, and the recipient client purges any cached plaintext.
- **PSI client:** normalise contacts (E.164 / lowercased email) → hash → blind with a random `r` → server evaluates → unblind → upload the PRF set. Raw contacts never leave the device; nothing is shown to the member about matches.
- **Guardian on device (FR-033):** M6a–e run on each thread turn over a rolling window (< 80 ms p95). Only `{model_id, version, score, reason_code}` is posted. Local UI responses:
    - M6d auto-redacts addresses and phone numbers in outgoing text and asks the sender to confirm;
    - M6c shows a caution banner to the counterparty;
    - M6e locks the thread pending review.
- **Pre-send media check:** on-device nudity classifier (explicit media only to V2 web, with the counterparty's media-view consent), plus licensed hash matching where permitted ([D-10](06_Decisions_and_Changes.md#2-decision-log)).
- **Conversation features** for M5 and Mirror (length bucket, latency, question ratio) are computed on device. **No content leaves the device.**
- **Reporting:** the reporter may choose to attach decrypted messages or media. This is an explicit, separately confirmed disclosure, with a separate opt-in for safety-model training (never a condition of action).

---

## 7. Safety UX

**Safety notice** (FR-054; shown at intake and before a first meet) [A §20.3]:

> **Before you meet anyone.** We verify that members are real adults. We do not — and cannot — check anyone's criminal record, their honesty, or whether they are who they say they are once you are off this app. No UK platform can. Meet in public the first time. Tell someone where you are going; Share My Plan does it privately. Trust your instincts and leave if something feels wrong. If anyone pressures you, threatens you, or pushes past a boundary you set, report them — we act on it, and we do it fast.

- **Panic** (FR-022): a prominent, distinct control in threads and the meet screen. It confirms with a hold, sends live location + preserved evidence through `POST /v1/safety/panic`, and offers a one-tap call to 999. It works even when notifications are masked or the app is in decoy mode (reachable through a gesture from the decoy).
- **Share My Plan:** pick a trusted contact (stored locally only). It sends a neutral, unbranded, auto-deleting message with venue and time window through the device's share sheet or an encrypted link. The wording is chosen for someone who cannot tell their partner where they are.
- **Check-in timer:** a missed check-in triggers silent escalation to the trusted contact.
- **Report / block:** one tap on every profile, Brief, thread and media item. The reporter is never revealed.

### 7.1 Copy rules (claims discipline, P6)

*Source: A §19.8*

| Never | Use |
|---|---|
| "Background checked", "Safe", "Vetted", green "safe" ticks | "Identity verified", "Age assured", dated facts ("Submitted a Basic DBS dated 14 Mar 2026") |
| "No sex offenders" | "We permanently remove and block re-registration where we find harmful conduct" |
| "Guaranteed real" | "Every member completes liveness and ID verification" |
| "Undetectable", "No trace" | "We minimise what we store, and here's what we can't control" |
| "Cheat better" or any affair-shaming or affair-glamorising copy in the store build | "Discreet dating"; "Discretion is the product" (web) |

### 7.2 Brand application (v1.2)

**Logo.** Use [`assets/brand/tryst-logo.png`](../../assets/brand/tryst-logo.png) **exactly as supplied**: no redraws, recolours, crops or effects. Place it on `night`, `oxblood-deep` or `oxblood` only. Never show it in decoy mode, on lock screens, in notifications, app-switcher snapshots or Share My Plan (FR-063; [01 §3.1](01_Product.md#31-logo)). For small sizes (favicon, ≤ 48 px) use the PNG scaled down. If it is illegible at that size, the web favicon defaults to the neutral option (FR-027) rather than a modified logo.

**Colour.** Import [`assets/brand/tokens.css`](../../assets/brand/tokens.css) (web) or generate native colour assets from [`tokens.json`](../../assets/brand/tokens.json). Defaults:

| Element | Token |
|---|---|
| Background | `night` |
| Cards | `oxblood-deep` |
| Body text | `cream` |
| Headings and accents | `gold` |
| Primary button | `oxblood` with `champagne` text |
| Borders | `oxblood-light` |
| Disabled | `bronze` |

**Type:** serif display with tight tracking for headings, matching the wordmark; a clean sans for body text. **Never** pink, hearts or silhouettes [A §2]. The decoy skins use their own neutral system palettes, never brand tokens.

---

## 8. Accessibility, localisation, performance

- **Accessibility:** WCAG 2.2 AA on web (brand text pairs pass AA, [01 §3.2](01_Product.md#32-colour-palette)); VoiceOver/TalkBack labels on native; Dynamic Type; reduced motion. The decoy and quick-exit controls must be operable by assistive technologies **without announcing "TRYST"** (NFR-07). Members who cannot use face or fingerprint biometrics get the accessible sign-in alternative ([D-17](06_Decisions_and_Changes.md#2-decision-log)).
- **Localisation:** EN-GB/EN-IE at P1; NL, DE, SV, DA, ES, PT at P4 (NFR-14). Brief and Guardian copy is code-based for localisation.
- **Performance:** cold start < 2 s on mid-tier devices; slate first paint < 1 s after the API response; quick exit < 500 ms; Burn confirmable < 2 s (NFR-04).
- **Platforms:** iOS 17+, Android 10+, evergreen browsers (NFR-15).

---

## 9. Client testing and release

- Contract tests against generated mocks of 02. A UI tier-gate test checks that no screen shows other members' content below V2.
- Security: MASVS L2 + R checks, jailbreak/root behaviour, screenshot and backup tests, SBOM scan with zero third-party telemetry SDKs (G-NG-8).
- On-device model benchmarks per device class (NFR-03).
- **Store review pack:** sanitised build flag, reviewer demo account, privacy manifest, Data Safety form diffed against the Privacy Notice in CI.

---

## 10. Frontend backlog by phase

| Phase | Epics (frontend) | Definition of done |
|---|---|---|
| **P0** (M0–M2) | Design system (oxblood/bone); discretion UX prototypes (decoy, duress, quick exit, Burn vs Panic) usability-tested; Rust crypto core spike on three targets; store pre-review consultation | Prototypes validated; crypto core builds on iOS, Android and WASM |
| **P1** (M2–M6) | Web PWA (full product) + native apps (TestFlight / internal track): onboarding V0–V2, conduct and Article 9 consent, Cartographer UI, profile/media with client encryption, couple co-sign, slate, intents/Keys, threads (MLS), reveal ladder, meet/Safe Meet/Share My Plan/Panic, Aftercare, discretion suite, on-device Guardian baseline, report/block, account/DSR, web billing (TRYST+, Keys, deposit, vouchers) | FR-001–009, 011–013, 016–023, 026–033, 037–039, 041–042, 044, 046–047, 049, 051, 054, 056–065, 069, 072 met (client side); NFR-03/04/07/09/15 met |
| **P2** (M6–M10) | Brief screens + decisions; My Signals; ExclusionRing enrolment (PSI client); reason chips | FR-010, 015, 024, 025 |
| **P3** (M10–M15) | DUO, ENVOY (proposal inbox), GHOST UI; travel mode; DBS attestation and Clare's Law explainer; store submissions approved on both platforms | G-P3 store approval; FR-045, 048, 053 |
| **P4** (M15–M24) | Localisation (6 languages); market-specific copy and assurance flows | NFR-14 |
