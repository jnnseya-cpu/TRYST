# Run every check CI runs. Requires Go 1.24+, Python 3.11+, Rust stable, Node 22.
.PHONY: check spec contracts backend agents crypto web e2e

check: spec contracts backend agents crypto web

spec:
	python3 scripts/check_spec.py
	python3 scripts/gen_migrations.py --check

contracts:
	cd contracts/openapi && npx --yes @redocly/cli@1 lint tryst.v1.yaml

backend:
	cd backend && test -z "$$(gofmt -l .)" && go vet ./... && go test ./... && go build ./...

agents:
	cd agents && (test -d .venv || python3 -m venv .venv) && . .venv/bin/activate && pip install -q -e '.[dev]' && ruff check src tests && python -m pytest -q

crypto:
	cd crypto-core && cargo fmt --check && cargo clippy --all-targets -q -- -D warnings && cargo test -q

web:
	cd web && npm ci --silent && npm run typecheck && npm test && npm run build

e2e:
	cd web && CHROMIUM_PATH=$${CHROMIUM_PATH:-} npx playwright test
