#!/usr/bin/env bash
# The sync Worker's tests (#164): the Worker of cloudflare/sync, run on this machine by wrangler dev with a database
# and a bucket of its own, made for the run and thrown away after it, and the tests that speak to a real Worker run
# against it: an account's life and its recovery (account_test.go), a sync through it and its limits (the quota, a
# write that lost a race giving its room back, the sign-ins a name may try; sync_worker_test.go). Nothing is deployed.
#
#   scripts/worker-test.sh              (npm ci in cloudflare/sync first; the nightly runs it)
#
# PORT sets the port (8799).
set -euo pipefail
cd "$(dirname "$0")/.."
port=${PORT:-8799}
state=$(mktemp -d)
admin="worker-test-$RANDOM$RANDOM"
pid=""
cleanup() {
	[ -n "$pid" ] && kill "$pid" 2>/dev/null || true
	pkill -f -- "--persist-to $state" 2>/dev/null || true # wrangler's own workerd
	rm -rf "$state"
}
trap cleanup EXIT

(cd cloudflare/sync && npm run --silent build >/dev/null && npx wrangler d1 migrations apply celestine --local --persist-to "$state" >/dev/null)
(cd cloudflare/sync && exec npx wrangler dev --local --port "$port" --persist-to "$state" --var "ADMIN_TOKEN:$admin") >"$state/dev.log" 2>&1 &
pid=$!
for _ in $(seq 1 90); do
	if curl -sf -o /dev/null -X POST -H 'content-type: application/json' -d '{"username":"x"}' "http://localhost:$port/api/prelogin"; then
		break
	fi
	if ! kill -0 "$pid" 2>/dev/null; then
		cat "$state/dev.log" >&2
		exit 1
	fi
	sleep 1
done
# each test on its own, the counters of tries emptied before it: every test comes from this one address, which the
# Worker counts the tries of (ten codes an hour), as it would a guesser's
status=0
for test in $(go test -list 'Live$' . | grep '^Test'); do
	(cd cloudflare/sync && npx wrangler d1 execute celestine --local --persist-to "$state" --command "DELETE FROM hits" >/dev/null)
	CELESTINE_SYNC_URL="http://localhost:$port" CELESTINE_ADMIN_TOKEN="$admin" go test -count=1 -run "^$test\$" -v . "$@" || status=1
done
exit $status
