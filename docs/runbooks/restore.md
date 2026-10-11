# Restoring from backup

Internal engineering runbook (D-57). **Read before every restore: a restore can bring back
accounts that members deleted after the backup was taken. Step 4 is not optional.**

## Backups

* Managed point-in-time recovery: 35 days on `main` and `ledger`, **7 days on `keys`**, so a
  deleted member's key leaves every backup within a week.
* Logical, encrypted copies: `scripts/backup.sh` (daily) to object-locked buckets; the
  key database goes to its own bucket with 7-day retention.
* The erasure ledger (`tryst_ledger`) records every completed deletion as a one-way code
  and a time. It holds no personal data. Restores always use the **live** ledger.

## Restore

1. Stop traffic to the API (scale the service to 0) so nothing writes during the restore.
2. Restore `main` and `keys` to the chosen time (console or `scripts/restore-drill.sh` with
   a logical backup). Do **not** restore the ledger unless it is itself lost.
3. `identity-svc admin verify`: every service loads, the safety audit hash chain holds,
   every photo decrypts. Stop if `ok` is false.
4. `identity-svc admin reapply-erasures`: every account in the live ledger is erased again
   (keys destroyed, records purged). Record `erased_again` in the incident log.
5. `identity-svc admin verify` again, then scale the API back up.

## Drill

Run monthly and after any change to storage or erasure. The automated drill
(`TestRestoreDrill`, run in CI) backs up, deletes an account, restores, shows the account
came back, re-applies erasures, and checks that the deleted member is unreadable and
everyone else is intact. The manual drill runs `scripts/restore-drill.sh` against
production-shaped data in staging and records the timings it prints (recovery time).
