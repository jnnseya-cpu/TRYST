# TRYST IOS app — placeholder

Native Swift / SwiftUI (iOS 17+) per docs/spec/04_Frontend.md §3. React Native is not used (E-07).

Not scaffolded yet: this build environment has no Xcode/Swift toolchain, and nothing is committed here that has not been built and tested.

Requirements the app must meet from day one:
- Login: B1 device-bound biometric passkey (key invalidated on biometric enrolment change) + B2 liveness face match (02 §3.3, FR-057, FR-059).
- Keys in the Secure Enclave; `NSFileProtectionComplete`; excluded from iCloud backup; screenshot detection with counterparty notice; alternate icons for decoy skins.
- Decoy skin, duress PIN, Burn (distinct from Panic), masked notifications, quick exit (04 §5).
- Shared Rust crypto core (`/crypto-core`) via UniFFI; on-device Guardian M6a–e.
- Sanitised store build: no explicit media or copy (FR-051); brand per 04 §7.2.
