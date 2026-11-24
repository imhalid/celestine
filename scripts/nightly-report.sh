#!/bin/bash
# Writes the nightly run's report, as Markdown, to stdout: the commits it tested, each step's outcome, the tests that
# failed with the end of their output, the slowest tests, the performance figures, and the known vulnerabilities.
# .github/workflows/nightly.yml runs it with the steps' outcomes in the environment (OUT_FMT, OUT_BUILD, OUT_RACE,
# OUT_TEST, OUT_PERF, OUT_VULN, OUT_WORKER: success, failure or skipped), FROM the commit the last run tested ("" when none), and
# the outputs in report/. The issue it becomes is cut to GitHub's limit.
set -u
R=report
HEAD_SHA=$(git rev-parse HEAD)

mark() { case "$1" in success) echo "✅" ;; failure) echo "❌" ;; *) echo "–" ;; esac; }
block() { # a file's last lines in a fenced block, or nothing when it is empty
	[ -s "$1" ] || return 0
	echo '```'
	tail -n "${2:-40}" "$1"
	echo '```'
}

echo "Run: ${RUN_URL:-}"
echo
echo "## Commits tested"
echo
if [ -n "${FROM:-}" ] && git cat-file -e "$FROM^{commit}" 2>/dev/null; then
	echo "Since the last nightly run (\`${FROM:0:7}\`), up to \`${HEAD_SHA:0:7}\`:"
	echo
	git log --no-merges --format='- `%h` %s' "$FROM..$HEAD_SHA"
else
	echo "The last day's, up to \`${HEAD_SHA:0:7}\`:"
	echo
	git log --no-merges --since="24 hours ago" --format='- `%h` %s' "$HEAD_SHA"
fi
echo
echo "## Steps"
echo
echo "| Step | |"
echo "| --- | --- |"
echo "| gofmt and vet | $(mark "${OUT_FMT:-}") |"
echo "| builds: darwin, linux, windows without cgo; js/wasm | $(mark "${OUT_BUILD:-}") |"
echo "| the library under -race | $(mark "${OUT_RACE:-}") |"
echo "| every test | $(mark "${OUT_TEST:-}") |"
echo "| performance with a big library | $(mark "${OUT_PERF:-}") |"
echo "| govulncheck | $(mark "${OUT_VULN:-}") |"
echo "| the sync Worker under wrangler dev | $(mark "${OUT_WORKER:-}") |"

if [ "${OUT_FMT:-}" = failure ]; then
	echo
	echo "## gofmt and vet"
	echo
	block $R/vet.txt
fi
if [ "${OUT_BUILD:-}" = failure ]; then
	echo
	echo "## Builds"
	echo
	block $R/builds.txt
fi
if [ "${OUT_WORKER:-}" = failure ]; then
	echo
	echo "## The sync Worker"
	echo
	block $R/worker.txt 80
fi
if [ "${OUT_RACE:-}" = failure ]; then
	echo
	echo "## Race detector"
	echo
	block $R/race.txt 60
fi

if [ -s $R/test.json ]; then
	failed=$(jq -r 'select(.Action=="fail" and .Test != null) | "\(.Package | sub(".*/celestine"; "celestine"))\t\(.Test)"' $R/test.json | sort -u)
	echo
	echo "## Tests"
	echo
	passed=$(jq -r 'select(.Action=="pass" and .Test != null) | .Test' $R/test.json | wc -l | tr -d ' ')
	nfail=$(printf '%s' "$failed" | grep -c . || true)
	echo "$passed passed, $nfail failed."
	if [ "$nfail" -gt 0 ]; then
		echo
		printf '%s\n' "$failed" | while IFS=$'\t' read -r pkg name; do
			echo "<details><summary><code>$name</code> ($pkg)</summary>"
			echo
			echo '```'
			jq -r --arg t "$name" 'select(.Test==$t and .Action=="output") | .Output' $R/test.json | grep -v '^=== \|^--- ' | tail -n 15
			echo '```'
			echo "</details>"
		done
	fi
	echo
	echo "The slowest:"
	echo
	echo "| Test | Seconds |"
	echo "| --- | --- |"
	jq -r 'select(.Action=="pass" and .Test != null and (.Test | contains("/") | not)) | "\(.Elapsed)\t\(.Test)"' $R/test.json |
		sort -rn | head -n 10 | while IFS=$'\t' read -r s name; do echo "| \`$name\` | $s |"; done
	for pkg in $(jq -r 'select(.Action=="pass" or .Action=="fail") | select(.Test == null) | "\(.Package)=\(.Elapsed)"' $R/test.json); do
		echo
		echo "Package \`${pkg%=*}\`: ${pkg#*=} s in all."
	done
fi

echo
echo "## Performance"
echo
if [ -s $R/perf.txt ]; then
	echo "TestPerfBudget, ${PERF_PAGES:-} pages (the budgets are in perf_test.go):"
	echo
	echo '```'
	grep -E 'pages: open|over the budget' $R/perf.txt | sed 's/^ *//'
	echo '```'
fi
if [ -s $R/bench.txt ]; then
	echo
	echo "Benchmarks:"
	echo
	echo '```'
	grep -E '^Benchmark' $R/bench.txt
	echo '```'
fi

echo
echo "## Known vulnerabilities"
echo
if [ "${OUT_VULN:-}" = success ]; then
	echo "None found."
else
	block $R/vuln.txt 80
fi
