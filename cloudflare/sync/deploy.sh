#!/bin/sh
# Deploys the Worker with `cf deploy --prebuilt` (wrangler is only used for `wrangler dev`).
#
#   RYUDIT_SYNC_SECRETS=/path/secrets.json ./deploy.sh          the first time, and when a secret changes
#   ./deploy.sh                                                  code only: the deployed secrets stay as they are
#
# secrets.json is {"ADMIN_TOKEN": "…", "TURNSTILE_SECRET": "…", "TURNSTILE_SITEKEY": "…", "RESEND_API_KEY": "…",
# "MAIL_FROM": "Ryudit <davet@resend.halid.dev>", "ADMIN_EMAIL": "…", "CONTACT_EMAIL": "…",
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
cp "$here"/*.js "$here"/*.bin "$w/bundle/"
echo '{"buildContext":{"isPreview":false,"mode":"production"}}' > "$out/.cloudflare/output/v0/config.json"
# Without a secrets file, the secrets the deployed version has are named in the new one, without a value: cf keeps
# each as it is. A secret not named is dropped with the old version.
SECRET_NAMES=""
if [ -z "$RYUDIT_SYNC_SECRETS" ]; then
  live=$(cf workers deployments list --worker ryudit-sync | node -e 'const d = JSON.parse(require("fs").readFileSync(0, "utf8")); console.log((d.deployments || d)[0].versions[0].version_id)')
  SECRET_NAMES=$(cf workers versions get "$live" --worker-id ryudit-sync | node -e '
const v = JSON.parse(require("fs").readFileSync(0, "utf8"));
const names = (v.resources?.bindings || v.bindings || []).filter((b) => b.type === "secret_text").map((b) => b.name);
console.log(names.filter((n) => n !== "TOKENS").join(" "));  // TOKENS: the old way in, gone')
  [ -n "$SECRET_NAMES" ] || { echo "the deployed Worker has no secrets: give RYUDIT_SYNC_SECRETS" >&2; exit 1; }
fi
# the crons are wrangler.toml's ([triggers] crons), one list for both
SECRET_NAMES="$SECRET_NAMES" CRONS=$(sed -n 's/^crons *= *//p' "$here/wrangler.toml") HERE_JS="$w/bundle" SECRETS="$RYUDIT_SYNC_SECRETS" DOMAIN="$DOMAIN" node -e '
const fs = require("fs");
// the scripts, and the WebAssembly of the admin panel as bytes (admin.bin, served as a file)
const modules = Object.fromEntries(fs.readdirSync(process.env.HERE_JS).map((f) => [f, { type: f.endsWith(".bin") ? "data" : "esm" }]));
const triggers = JSON.parse(process.env.CRONS || "[]").map((schedule) => ({ type: "scheduled", schedule }));
if (!triggers.length) throw new Error("no [triggers] crons in wrangler.toml");
const env = { SYNC: { type: "r2", name: "ryudit-sync" }, DB: { type: "d1", name: "ryudit" } };
// the secrets: those of the file, or with none, those the deployed version has, named without a value to be kept
const given = process.env.SECRETS ? JSON.parse(fs.readFileSync(process.env.SECRETS, "utf8")) : null;
const unset = Object.keys(given || {}).filter((k) => !given[k] || /^BURAYA_|^TODO/.test(given[k]));
if (unset.length) throw new Error("secrets with no value yet: " + unset.join(", "));
const secrets = given ? Object.keys(given) : process.env.SECRET_NAMES.split(" ");
for (const k of secrets) env[k] = { type: "secret" };
const cfg = {
  name: "ryudit-sync", compatibilityDate: "2026-01-01", workersDev: true, observability: { enabled: true },
  domains: [process.env.DOMAIN], triggers, manifest: { type: "complete", mainModule: "worker.js", modules }, env,
};
fs.writeFileSync(process.env.HERE_JS + "/../worker.config.json", JSON.stringify(cfg));
'
cd "$out"
cf deploy --prebuilt ${RYUDIT_SYNC_SECRETS:+--secrets-file "$RYUDIT_SYNC_SECRETS"}
rm -rf "$out"
