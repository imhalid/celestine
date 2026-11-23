// The one page of this Worker, the administrator's ("/admin"). The site's pages (the entry screen, the privacy page, the
// changelog) are the site Worker's, in the celestine-site repository (worker.js: SITE).
import { ADMIN_JS_PATH } from "./admin.gen.js";

function nonce() {
  return btoa(String.fromCharCode(...crypto.getRandomValues(new Uint8Array(16))));
}
function page(html, n, scriptExtra = "") {
  const csp = [
    "default-src 'none'",
    `script-src 'nonce-${n}' ${scriptExtra}`.replace(/\s+/g, " ").trim(),
    "style-src 'unsafe-inline'",
    "img-src 'self'",
    "connect-src 'self'",
    "frame-src 'none'",
    "base-uri 'none'",
    "form-action 'none'",
    "frame-ancestors 'none'",
  ].join("; ");
  return new Response(html, {
    headers: {
      "content-type": "text/html; charset=utf-8",
      "content-security-policy": csp,
      "x-frame-options": "DENY",
      "referrer-policy": "no-referrer",
      "x-content-type-options": "nosniff",
      "strict-transport-security": "max-age=31536000", // the page that holds the admin key: HTTPS only, from then on (#163)
      "cache-control": "no-store",
    },
  });
}

// The administrator's page ("/admin"): a canvas, and the panel drawn on it with synth-ui (admin/main.js, built into
// admin.gen.js and served under /admin/ beside its WebAssembly). The module is the Worker's own, by the page's
// nonce: no script of another site on the page that holds the admin key. WebAssembly is allowed, nothing more.
export function adminPage() {
  const n = nonce();
  const html = `<!doctype html><html lang="tr"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="robots" content="noindex"><title>Celestine — yönetim</title>
<style>html,body{margin:0;height:100%;background:#141210}#panel{display:block;width:100vw;height:100vh;outline:none}</style></head>
<body><canvas id="panel" tabindex="0"></canvas>
<noscript>Bu panel JavaScript ve WebAssembly ister.</noscript>
<script type="module" nonce="${n}" src="/admin/${ADMIN_JS_PATH}"></script></body></html>`;
  return page(html, n, "'wasm-unsafe-eval'");
}
