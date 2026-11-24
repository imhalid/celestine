#!/usr/bin/env bash
# Runs shard K of N of the tests (#580): the root package's tests dealt round, one in N to each shard in the order
# `go test -list` gives them, and the other packages' with shard 1. The release runs four at once, beside its
# builds: what took one runner eighteen minutes takes four about five each.
#
#   scripts/test-shard.sh 4 2 [go test flags]
set -euo pipefail
cd "$(dirname "$0")/.."
n=$1 k=$2
shift 2
root=$(go list .)
names=$(go test -list '.' . | grep -E '^(Test|Example|Fuzz)' | awk -v n="$n" -v k="$k" '(NR - 1) % n == k - 1')
if [ -n "$names" ]; then
	go test -timeout 20m -run "^($(echo $names | tr ' ' '|'))\$" "$@" .
fi
if [ "$k" = 1 ]; then
	go test -timeout 20m "$@" $(go list ./... | grep -vx "$root")
fi
