#!/usr/bin/env bash
# Local-only verification. Integration tests refuse a DSN without `_test`.
set -euo pipefail

cd "$(dirname "$0")/.."
test_dsn="${TEST_DATABASE_URL:-postgres://toko:toko-local-only@localhost:55432/toko_loop_test?sslmode=disable}"

TEST_DATABASE_URL="$test_dsn" QA_ALLOW_REMOTE_TEST_DB="${QA_ALLOW_REMOTE_TEST_DB:-}" python3 - <<'PY'
import os
import sys
from urllib.parse import urlparse

u = urlparse(os.environ["TEST_DATABASE_URL"])
db = u.path.strip("/")
if not db.endswith("_test"):
    sys.exit("Refusing a database whose URL path does not end in _test; no database was touched.")
if u.hostname not in {"localhost", "127.0.0.1", "::1"} and os.environ["QA_ALLOW_REMOTE_TEST_DB"] != "1":
    sys.exit("Refusing a non-loopback test host. Set QA_ALLOW_REMOTE_TEST_DB=1 only for a dedicated remote test database.")
PY

echo "Running local backend validation against a dedicated test database."
env -u GOROOT TEST_DATABASE_URL="$test_dsn" go test -timeout 90s ./...
env -u GOROOT go vet ./...
