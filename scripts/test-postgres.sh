#!/usr/bin/env bash
# Start a throwaway PostgreSQL 16 for tests and print the DSN.
#   eval "$(scripts/test-postgres.sh start)"   # exports TRYST_TEST_PG
#   scripts/test-postgres.sh stop
set -euo pipefail
DIR=${PGTEST_DIR:-/tmp/tryst-pgtest}
PORT=${PGTEST_PORT:-55440}
BIN=$(ls -d /usr/lib/postgresql/*/bin 2>/dev/null | sort -V | tail -1)
run() { if [ "$(id -u)" = 0 ]; then su postgres -c "$*"; else bash -c "$*"; fi; }
case "${1:-start}" in
  start)
    if [ ! -d "$DIR" ]; then
      mkdir -p "$DIR"; [ "$(id -u)" = 0 ] && chown postgres "$DIR"
      run "$BIN/initdb -D $DIR -A trust >/dev/null"
    fi
    psql -h /tmp -p "$PORT" -U postgres -c 'select 1' >/dev/null 2>&1 || run "$BIN/pg_ctl -D $DIR -o '-p $PORT -k /tmp' -l $DIR/log start >/dev/null"
    for _ in $(seq 1 30); do psql -h /tmp -p "$PORT" -U postgres -c 'select 1' >/dev/null 2>&1 && break; sleep 0.3; done
    for db in tryst_identity tryst_profile tryst_keys tryst_safety tryst_commerce tryst_test; do
      psql -h /tmp -p "$PORT" -U postgres -tc "select 1 from pg_database where datname='$db'" | grep -q 1 || psql -h /tmp -p "$PORT" -U postgres -qc "create database $db"
    done
    echo "export TRYST_TEST_PG='postgres://postgres@localhost:$PORT/tryst_test?host=/tmp&sslmode=disable'"
    ;;
  stop) run "$BIN/pg_ctl -D $DIR stop >/dev/null" || true ;;
esac
