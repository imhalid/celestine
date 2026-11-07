#!/bin/sh
# Deploys the sync Worker with `cf deploy --prebuilt` (wrangler is only used for `wrangler dev`).
#
#   RYUDIT_SYNC_SECRETS=/path/secrets.json ./deploy.sh
#
# secrets.json is {"TOKENS": "{\"<device token>\":\"<account>\", …}"}, kept outside the repository.
# The bucket "ryudit-sync" must exist (cf r2 buckets create --name ryudit-sync).
set -e
here=$(cd "$(dirname "$0")" && pwd)
out=$(mktemp -d)
w=$out/.cloudflare/output/v0/workers/default
mkdir -p "$w/bundle"
cp "$here/worker.js" "$w/bundle/worker.js"
echo '{"buildContext":{"isPreview":false,"mode":"production"}}' > "$out/.cloudflare/output/v0/config.json"
cat > "$w/worker.config.json" <<'JSON'
{"name":"ryudit-sync","compatibilityDate":"2026-01-01","workersDev":true,"observability":{"enabled":true},
 "manifest":{"type":"complete","mainModule":"worker.js","modules":{"worker.js":{"type":"esm"}}},
 "env":{"SYNC":{"type":"r2","name":"ryudit-sync"},"TOKENS":{"type":"secret"}}}
JSON
cd "$out"
cf deploy --prebuilt ${RYUDIT_SYNC_SECRETS:+--secrets-file "$RYUDIT_SYNC_SECRETS"}
rm -rf "$out"
