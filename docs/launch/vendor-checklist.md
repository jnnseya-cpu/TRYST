# Vendor checklist

Every outside service TRYST depends on at launch, what has to be signed, what each one
receives, and how it is plugged in. Code for each adapter is in `backend/internal/vendors`,
`backend/internal/outbound` and `backend/internal/payments`; settings are named in
[../runbooks/deploy.md](../runbooks/deploy.md). Owner: operations; DPAs reviewed by the DPO
and counsel ([legal-review-pack.md](legal-review-pack.md)).

For each vendor: ☐ contract ☐ DPA with UK/EU data location and sub-processor list ☐
security evidence ☐ sandbox credentials into staging ☐ staging test passed ☐ production
credentials into Secrets Manager ☐ entry in the records of processing.

## Launch-critical (the service won't start or can't verify anyone without these)

### Identity and age assurance provider (×2, D-41, D-49, D-61)

* **Does:** liveness (V1), photo-ID check with chip read where possible (V2), face match at
  sign-in (B2).
* **Receives:** face images and ID images at the provider only (hosted check). TRYST gets
  pass/fail, the over-18 decision, birth year-month and the document number (turned into a
  one-way code at once).
* **Contract must include:** deletion of face and ID data within an agreed time after each
  check (Q14: compare with our 60-second erasure); no training on member data; UK/EU
  processing; breach notice within 24 hours.
* **Certificates needed** (recorded in `ASSURANCE_CERTS`; checks are refused without a valid
  one, G-NG-4): ISO/IEC 30107-3 presentation-attack detection (iBeta Level 2 or equivalent)
  and an injection-attack certificate for web capture; UK age-assurance certification
  (ACCS/UKAS) for the age decision.
* **Integration:** signed gateway contract: `POST /assurance/sessions` → hosted check →
  signed callback to `/v1/assurance/callback` (`X-Provider-Timestamp`,
  `X-Provider-Signature`). Settings: `ASSURANCE_URL`, `ASSURANCE_API_KEY`,
  `ASSURANCE_SECRET`, `ASSURANCE_CALLBACK_SECRET`, `ASSURANCE_PROVIDER`, `ASSURANCE_CERTS`;
  B2 sign-in: `LIVENESS_URL`, `LIVENESS_API_KEY`, `LIVENESS_SECRET`. If the provider's API
  differs, a thin gateway translates it (D-56).
* **Two providers** so one outage or lapsed certificate doesn't stop sign-ups (G-P0).

### Alarm-receiving centre (D-48)

* **Does:** 24/7 safety line for missed check-ins, duress and Panic; 60-second
  acknowledgement; heartbeat failover.
* **Receives:** only the pinned alert fields (03 §7.7; ToS 20), and nothing else about the
  member.
* **Contract:** BS EN 50518 accreditation, BS 8484 operation, a written escalation script,
  case reference returned on every alert, data retention terms.
* **Settings:** `ARC_URL`, `ARC_API_KEY`, `ARC_SECRET` (required in production).

### Sign-up code delivery

* **Postmark** (email): server token `POSTMARK_SERVER_TOKEN`, sender `OTP_EMAIL_FROM`; open
  and link tracking off; neutral wording that never names TRYST (D-55).
* **Twilio** (SMS): `TWILIO_ACCOUNT_SID`, `TWILIO_AUTH_TOKEN`, `TWILIO_FROM` or
  `TWILIO_MESSAGING_SERVICE`; UK sender registration.
* At least one is required in production.

### Stripe (payments, D-22, D-53)

* **Condition:** Stripe's written approval of the business model (D-22 is open until then);
  a high-risk acquirer as backup.
* **Receives:** an opaque billing reference, the billing email the member chooses, card
  data (at Stripe only). Never a name unless the card network requires it; never Article 9
  data.
* **Set up:** Prices for TRYST+ monthly and annual (`STRIPE_PRICE_TRYST_PLUS`,
  `STRIPE_PRICE_TRYST_PLUS_ANNUAL`); the other items stay unsold (D-60); webhook to
  `/v1/billing/stripe/webhook`; renewal reminder emails at least 7 days ahead; a neutral
  statement descriptor. Secrets `STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET`.

### Cloudflare Access (staff sign-in, D-59)

* Access application for the console host, safety-team group, hardware-key sign-in
  (`require_amr = "hwk"`). Separate applications for staging and production.

### AWS (hosting, London)

* Business support plan; KMS keys per environment; organisation SCPs; the Terraform in
  `deploy/terraform`. DPA is AWS's standard GDPR addendum.

## Photo safety (required for photos to be published automatically)

Without these, every photo goes to a person (fail closed, FR-120).

* **Sightengine:** synthetic-image and nudity checks. `SIGHTENGINE_USER`,
  `SIGHTENGINE_SECRET`. No retention of submitted images.
* **Microsoft PhotoDNA:** known-CSAM hash matching. `PHOTODNA_KEY`. Requires Microsoft's
  vetting; also an IWF membership for hash lists and reporting.

## After launch (not needed for Verified + TRYST+)

* Self-hosted model hosting in the UK for the conversational intake and the Broker (P2).
* Native app stores (Apple, Google) when the apps ship.
* A second payment processor if Stripe declines.

## Status

| Vendor | Contract | DPA | Security evidence | Staging | Production |
|---|---|---|---|---|---|
| Identity provider 1 | ☐ | ☐ | ☐ | ☐ | ☐ |
| Identity provider 2 | ☐ | ☐ | ☐ | ☐ | ☐ |
| Alarm-receiving centre | ☐ | ☐ | ☐ | ☐ | ☐ |
| Postmark | ☐ | ☐ | ☐ | ☐ | ☐ |
| Twilio | ☐ | ☐ | ☐ | ☐ | ☐ |
| Stripe | ☐ | ☐ | ☐ | ☐ | ☐ |
| Cloudflare Access | ☐ | ☐ | ☐ | ☐ | ☐ |
| AWS | ☐ | ☐ | ☐ | ☐ | ☐ |
| Sightengine | ☐ | ☐ | ☐ | ☐ | ☐ |
| PhotoDNA / IWF | ☐ | ☐ | ☐ | ☐ | ☐ |
