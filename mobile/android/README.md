# TRYST ANDROID app — placeholder

Native Kotlin / Jetpack Compose (Android 10+) per docs/spec/04_Frontend.md §3. React Native is not used (E-07).

Not scaffolded yet: this build environment has no Android SDK/Gradle toolchain, and nothing is committed here that has not been built and tested.

Requirements the app must meet from day one:
- Login: B1 device-bound biometric passkey (key invalidated on biometric enrolment change) + B2 liveness face match (02 §3.3, FR-057, FR-059).
- Keys in StrongBox (TEE fallback); `FLAG_SECURE`; `allowBackup=false`; activity-alias for decoy skins; Play Integrity attestation.
- Decoy skin, duress PIN, Burn (distinct from Panic), masked notifications, quick exit (04 §5).
- Shared Rust crypto core (`/crypto-core`) via JNI/UniFFI; on-device Guardian M6a–e.
- Sanitised store build: no explicit media or copy (FR-051); brand per 04 §7.2.
