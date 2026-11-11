#!/bin/sh
# Deploys the update Worker with `cf deploy --prebuilt`, as cloudflare/sync/deploy.sh does the sync one: worker.js
# over the R2 bucket celestine-releases, at dl.celestine.halid.dev. It has no secrets and no crons.
#
#   ./deploy.sh
#
# Before the first deploy:
#   cf r2 buckets create --name celestine-releases
# The release workflow fills the bucket (.github/workflows/release.yml); until the first release, /update.json is a 404.
set -e
DOMAIN=dl.celestine.halid.dev
here=$(cd "$(dirname "$0")" && pwd)
out=$(mktemp -d)
w=$out/.cloudflare/output/v0/workers/default
mkdir -p "$w/bundle"
cp "$here/worker.js" "$w/bundle/"
echo '{"buildContext":{"isPreview":false,"mode":"production"}}' > "$out/.cloudflare/output/v0/config.json"
# Observability stays off: the Worker logs nothing of the user (worker.js).
cat > "$w/worker.config.json" <<EOF
{
  "name": "celestine-updates", "compatibilityDate": "2026-01-01", "workersDev": false,
  "domains": ["$DOMAIN"],
  "manifest": { "type": "complete", "mainModule": "worker.js", "modules": { "worker.js": { "type": "esm" } } },
  "env": { "RELEASES": { "type": "r2", "name": "celestine-releases" } }
}
EOF
cd "$out"
cf deploy --prebuilt
rm -rf "$out"
