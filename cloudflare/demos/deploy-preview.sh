#!/bin/sh
# Puts the demos (and the entry screen in ../sync/landing.js, at /crt) on a Worker of their own, ryudit-demos, so that
# they can be opened from any computer: nothing but pages (no database, no files, no accounts). Delete it when done:
#   cf workers delete <id> --force      (cf workers list)
set -e
here=$(cd "$(dirname "$0")" && pwd)
(cd "$here" && node make.mjs >/dev/null)
out=$(mktemp -d)
w=$out/.cloudflare/output/v0/workers/default
mkdir -p "$w/bundle"
cp "$here/../sync/landing.js" "$w/bundle/landing.js"
HERE="$here" W="$w/bundle" node -e '
const fs=require("fs"),path=require("path");
const dist=path.join(process.env.HERE,"dist");
const files={};for(const f of fs.readdirSync(dist))files["/"+f]=fs.readFileSync(path.join(dist,f),"utf8");
files["/"]=files["/index.html"];
const src=`import { landingHTML } from "./landing.js";
const FILES = ${JSON.stringify(files)};
const esc = (s) => String(s).replace(/[&<>"\x27]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", \x27"\x27: "&quot;", "\x27": "&#39;" })[c]);
const H = { "cache-control": "no-store", "x-robots-tag": "noindex" };
export default {
  async fetch(request) {
    const p = new URL(request.url).pathname;
    if (p === "/crt") {
      const n = btoa(String.fromCharCode(...crypto.getRandomValues(new Uint8Array(16))));
      const csp = \`default-src \x27none\x27; script-src \x27nonce-\${n}\x27; style-src \x27unsafe-inline\x27; connect-src \x27self\x27; base-uri \x27none\x27; form-action \x27none\x27; frame-ancestors \x27none\x27\`;
      return new Response(landingHTML(n, "", "", esc), { headers: { ...H, "content-type": "text/html; charset=utf-8", "content-security-policy": csp } });
    }
    const f = FILES[p];
    if (f === undefined) return new Response("not found", { status: 404, headers: H });
    return new Response(f, { headers: { ...H, "content-type": p.endsWith(".js") ? "text/javascript; charset=utf-8" : "text/html; charset=utf-8" } });
  },
};
`;
fs.writeFileSync(path.join(process.env.W,"worker.js"),src);'
echo '{"buildContext":{"isPreview":false,"mode":"production"}}' > "$out/.cloudflare/output/v0/config.json"
echo '{"name":"ryudit-demos","compatibilityDate":"2026-01-01","workersDev":true,"manifest":{"type":"complete","mainModule":"worker.js","modules":{"worker.js":{"type":"esm"},"landing.js":{"type":"esm"}}},"env":{}}' > "$w/worker.config.json"
cd "$out" && cf deploy --prebuilt 2>&1 | grep -E "workers.dev|Deploy complete|Error"
rm -rf "$out"
