#!/bin/sh
# Deploys the Worker with `cf deploy --prebuilt` (wrangler is only used for `wrangler dev`).
#
#   RYUDIT_SYNC_SECRETS=/path/secrets.json ./deploy.sh          the first time, and when a secret changes
#   ./deploy.sh                                                  code only: the secrets stay as they are
#
# secrets.json is {"ADMIN_TOKEN": "…", "TURNSTILE_SECRET": "…", "TURNSTILE_SITEKEY": "…", "RESEND_API_KEY": "…",
# "MAIL_FROM": "Ryudit <davet@resend.halid.dev>", "ADMIN_EMAIL": "…", "CONTACT_EMAIL": "…", "TOKENS": "{…}",
# "DOWNLOAD_URL": "https://<the update Worker>/download"}, kept outside the
# repository; each key in it becomes a secret of the Worker. Before the first deploy:
#   cf r2 buckets create --name ryudit-sync
#   cf d1 create --name ryudit      (its id goes below, and in wrangler.toml)
# The migrations (migrations/*.sql) are applied to the database first.
set -e
D1_ID=2cc8a6d7-c7a1-4ddf-9d50-76a91cdc7296
DOMAIN=ryudit.halid.dev
here=$(cd "$(dirname "$0")" && pwd)
(cd "$here" && npm run --silent build)
(cd "$here" && cf d1 migrations apply "$D1_ID")
out=$(mktemp -d)
w=$out/.cloudflare/output/v0/workers/default
mkdir -p "$w/bundle"
cp "$here"/*.js "$w/bundle/"
echo '{"buildContext":{"isPreview":false,"mode":"production"}}' > "$out/.cloudflare/output/v0/config.json"
# the crons are wrangler.toml's ([triggers] crons), one list for both
CRONS=$(sed -n 's/^crons *= *//p' "$here/wrangler.toml") HERE_JS="$w/bundle" SECRETS="$RYUDIT_SYNC_SECRETS" DOMAIN="$DOMAIN" node -e '
const fs = require("fs");
const modules = Object.fromEntries(fs.readdirSync(process.env.HERE_JS).map((f) => [f, { type: "esm" }]));
const triggers = JSON.parse(process.env.CRONS || "[]").map((schedule) => ({ type: "scheduled", schedule }));
if (!triggers.length) throw new Error("no [triggers] crons in wrangler.toml");
const env = { SYNC: { type: "r2", name: "ryudit-sync" }, DB: { type: "d1", name: "ryudit" } };
if (process.env.SECRETS) for (const k of Object.keys(JSON.parse(fs.readFileSync(process.env.SECRETS, "utf8")))) env[k] = { type: "secret" };
const cfg = {
  name: "ryudit-sync", compatibilityDate: "2026-01-01", workersDev: true, observability: { enabled: true },
  domains: [process.env.DOMAIN], triggers, manifest: { type: "complete", mainModule: "worker.js", modules }, env,
};
fs.writeFileSync(process.env.HERE_JS + "/../worker.config.json", JSON.stringify(cfg));
'
cd "$out"
cf deploy --prebuilt ${RYUDIT_SYNC_SECRETS:+--secrets-file "$RYUDIT_SYNC_SECRETS"}
rm -rf "$out"
