# On-call

Internal engineering runbook (D-57). Every alert in `deploy/monitoring/alerts.yml` links
to a section here. Nothing here asks you to look at member content.

## Urgent safety case overdue

A suspected-minor, image-abuse, sextortion or threat case is past its 15-minute or 1-hour
deadline. Page the Trust & Safety lead on rota at once. Suspected-minor reports have
already frozen the reported account automatically; the case still needs a person.

## Safety alert delivery failing

A safety-line, trusted-contact or Panic alert failed twice. Check the alarm-receiving
centre's status line and the message provider's status page. If the centre is down, the
member's phone already shows the 999 / Silent Solution screen; switch to the on-call T&S
rota (D-48) and tell the centre's account manager.

## Safety cases overdue

Ordinary cases (24-hour deadline) are late. Raise with the T&S lead during working hours.

## High server error rate

More than 2% of requests are failing. Check `/readyz` on the API tasks (a database not
answering shows here), recent deploys (roll back by redeploying the previous digest) and
vendor status pages.

## Erasure slow or incomplete

Erasure p99 over 60 s, or an erasure that needed the sweep. The sweep retries every
minute and is safe to run repeatedly. If one keeps failing, the log names the part
(profile, safety, commerce, notifications, identity); fix the cause, don't delete data by
hand. The member's key is destroyed first, so their data is already unreadable.

## Billing webhook errors

Payment events failing or arriving with a bad signature. Check `STRIPE_WEBHOOK_SECRET`
matches the endpoint in Stripe, then replay failed events from the Stripe dashboard;
processing is idempotent.
