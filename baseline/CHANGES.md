# Baseline changes

Every change to what has been built, with the reason. Written by `scripts/baseline.py --update`; never edit by hand.

## 2026-11-08: First baseline: everything built up to a14b57e (D-01 to D-72, launch audit cycle 1, vendor keys, communications)
1090 locked items. Removed 0, changed 0, added 1090.

## 2026-11-10: Growth intelligence embedded in the platform, invisible to members (D-73): growth-svc, console Growth pages, aggregate-only signals (k >= 50 people), Growth Brief contract, guide pages off by default, deploy wiring; nothing removed
1172 locked items. Removed 0, changed 23, added 82.

**Changed:**

- `build:.github/workflows/integration.yml`
- `build:backend/Dockerfile`
- `build:backend/go.mod`
- `build:deploy/gcp/compose.yml`
- `build:deploy/gcp/deploy.sh`
- `build:deploy/gcp/fetch-secrets.sh`
- `build:deploy/gcp/setup.sh`
- `build:deploy/gcp/terraform/lb.tf`
- `build:deploy/gcp/terraform/network.tf`
- `build:deploy/gcp/terraform/secrets.tf`
- `build:deploy/gcp/terraform/sql.tf`
- `build:deploy/gcp/terraform/variables.tf`
- `build:frontend/Dockerfile`
- `build:frontend/next.config.ts`
- `build:scripts/provision-safety-secrets.sh`
- `build:scripts/test-postgres.sh`
- `doc:docs/runbooks/deploy.md`
- `doc:docs/runbooks/go-live.md`
- `doc:docs/spec/00_README.md`
- `doc:docs/spec/03_Backend.md`
- `doc:docs/spec/06_Decisions_and_Changes.md`
- `doc:docs/spec/09_Operating_System.md`
- `doc:launch-audit/human-actions.md`

**Added:**

- `api:internal.v1.yaml GET /internal/v1/growth/aggregates`
- `api:ops.v1.yaml GET /growth/actions/{id}`
- `api:ops.v1.yaml GET /growth/actions/{id}/export`
- `api:ops.v1.yaml GET /growth/audit`
- `api:ops.v1.yaml GET /growth/briefs/effective`
- `api:ops.v1.yaml GET /growth/briefs/{id}`
- `api:ops.v1.yaml GET /growth/experiments/{id}`
- `api:ops.v1.yaml GET /growth/opportunities`
- `api:ops.v1.yaml GET /growth/overview`
- `api:ops.v1.yaml POST /growth/actions`
- `api:ops.v1.yaml POST /growth/actions/{id}/approve`
- `api:ops.v1.yaml POST /growth/actions/{id}/execute`
- `api:ops.v1.yaml POST /growth/actions/{id}/rollback`
- `api:ops.v1.yaml POST /growth/briefs`
- `api:ops.v1.yaml POST /growth/briefs/{id}/approve`
- `api:ops.v1.yaml POST /growth/briefs/{id}/drafts`
- `api:ops.v1.yaml POST /growth/freeze`
- `api:ops.v1.yaml POST /growth/observations`
- `api:ops.v1.yaml POST /growth/pages`
- `api:ops.v1.yaml POST /growth/refresh`
- `doc:docs/spec/10_Growth_Intelligence.md`
- `growth:backend/internal/growth/profile.tryst.json`
- `growth:shared/contracts/growth/fixtures/negative-audience-too-long.json`
- `growth:shared/contracts/growth/fixtures/negative-duplicate-channel.json`
- `growth:shared/contracts/growth/fixtures/negative-empty-audience.json`
- `growth:shared/contracts/growth/fixtures/negative-extra-field.json`
- `growth:shared/contracts/growth/fixtures/negative-id-with-space.json`
- `growth:shared/contracts/growth/fixtures/negative-lowercase-market.json`
- `growth:shared/contracts/growth/fixtures/negative-missing-brief-id.json`
- `growth:shared/contracts/growth/fixtures/negative-no-evidence.json`
- `growth:shared/contracts/growth/fixtures/negative-non-utc-time.json`
- `growth:shared/contracts/growth/fixtures/negative-policy-version-zero.json`
- `growth:shared/contracts/growth/fixtures/negative-schema-version-2.json`
- `growth:shared/contracts/growth/fixtures/negative-score-over-100.json`
- `growth:shared/contracts/growth/fixtures/negative-unknown-channel.json`
- `growth:shared/contracts/growth/fixtures/positive-spec-s6.json`
- `growth:shared/contracts/growth/growth-brief.v1.schema.yaml`
- `page:frontend/app/ops/growth/briefs/[id]/page.tsx`
- `page:frontend/app/ops/growth/page.tsx`
- `route:GET /guides/sitemap.xml`
- `route:GET /guides/{slug}`
- `route:GET /internal/v1/growth/aggregates`
- `route:GET /ops/v1/growth/actions/{id}`
- `route:GET /ops/v1/growth/actions/{id}/export`
- `route:GET /ops/v1/growth/audit`
- `route:GET /ops/v1/growth/briefs/effective`
- `route:GET /ops/v1/growth/briefs/{id}`
- `route:GET /ops/v1/growth/experiments/{id}`
- `route:GET /ops/v1/growth/opportunities`
- `route:GET /ops/v1/growth/overview`
- `route:POST /guides/{slug}/join`
- `route:POST /ops/v1/growth/actions`
- `route:POST /ops/v1/growth/actions/{id}/approve`
- `route:POST /ops/v1/growth/actions/{id}/execute`
- `route:POST /ops/v1/growth/actions/{id}/rollback`
- `route:POST /ops/v1/growth/briefs`
- `route:POST /ops/v1/growth/briefs/{id}/approve`
- `route:POST /ops/v1/growth/briefs/{id}/drafts`
- `route:POST /ops/v1/growth/freeze`
- `route:POST /ops/v1/growth/observations`
- `route:POST /ops/v1/growth/pages`
- `route:POST /ops/v1/growth/refresh`
- `spec:D-73`
- `test:go:backend/cmd/growth-svc.TestConsoleRoutesRefuseUnauthenticatedCallers`
- `test:go:backend/cmd/growth-svc.TestContractMatchesRoutes`
- `test:go:backend/cmd/growth-svc.TestProductionRefusesMissingConfig`
- `test:go:backend/cmd/growth-svc.TestPublicListenerServesGuidePagesOnly`
- `test:go:backend/cmd/identity-svc.TestGrowthCallerReachesOnlyAggregates`
- `test:go:backend/internal/growth.TestApprovalRulesRolesAndFreeze`
- `test:go:backend/internal/growth.TestBriefContractMatchesTheSchemaAndFixtures`
- `test:go:backend/internal/growth.TestBriefToPublishedPageAndBack`
- `test:go:backend/internal/growth.TestCrashAfterPageWriteIsSettledOnRestart`
- `test:go:backend/internal/growth.TestExpiredBriefStopsAQueuedChange`
- `test:go:backend/internal/growth.TestGuidePagesAreInvisibleUntilAnEditorCreatesThem`
- `test:go:backend/internal/growth.TestIngestReplayAndConflict`
- `test:go:backend/internal/growth.TestInjectedTextCannotPublishAnything`
- `test:go:backend/internal/growth.TestReferenceScoresAreUnrounded`
- `test:go:backend/internal/growth.TestSyndicatedCopiesCountOnceAndStaleCannotActivate`
- `test:go:backend/internal/growth.TestTrystProfileClaimsAreTheAuditedWording`
- `test:go:backend/internal/growthsignals.TestOnlyCompleteWeeksAboveTheCohortFloorLeave`
- `test:go:backend/internal/profilesvc.TestTonightCountsTheCityOnly`
- `test:web:frontend/e2e/growth.spec.ts: growth: staff only, imports and scores, pages exist only when an editor creates them, members see nothing`

## 2026-11-10: CI fixes after the runner moved to Ubuntu 26.04: restore drill tolerates newer pg_restore (transaction_timeout), browser job pinned to ubuntu-24.04, terraform fmt of secrets.tf
1172 locked items. Removed 0, changed 3, added 0.

**Changed:**

- `build:.github/workflows/integration.yml`
- `build:deploy/gcp/terraform/secrets.tf`
- `build:scripts/restore-drill.sh`

## 2026-11-10: Founder decisions: launch without an alarm-receiving centre with the safety line hidden (D-74); journal posts and landing copy describe only what runs; guide pages stay off; staff security keys stay required
1175 locked items. Removed 0, changed 9, added 3.

**Changed:**

- `doc:docs/runbooks/deploy.md`
- `doc:docs/spec/06_Decisions_and_Changes.md`
- `doc:launch-audit/README.md`
- `doc:launch-audit/human-actions.md`
- `post:meet-check-ins-and-duress-pin.md`
- `post:tryst-plus-honest-pricing.md`
- `vendor-key:ARC_API_KEY`
- `vendor-key:ARC_SECRET`
- `vendor-key:ARC_URL`

**Added:**

- `spec:D-74`
- `test:go:backend/internal/profilesvc.TestWithoutAnAlarmCentreTheSafetyLineIsHiddenAndNeverClaimed`
- `test:go:backend/internal/vendors.TestContactAlertingWithoutAnAlarmCentre`
