// The pages of the server: the entry screen ("/": the download, and the waitlist of sync), what is kept ("/privacy") and the administrator's
// ("/admin"). Plain HTML and a little
// script; what a person typed (an e-mail, a name) is put on a page only as text, never as markup.
import { landingHTML } from "./landing.js";
import { ADMIN_JS_PATH } from "./admin.gen.js";
const esc = (s) => String(s).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]);

function nonce() {
  return btoa(String.fromCharCode(...crypto.getRandomValues(new Uint8Array(16))));
}
function page(html, n, extraScript = "", noStore = false, scriptExtra = "") {
  const csp = [
    "default-src 'none'",
    `script-src 'nonce-${n}' ${extraScript} ${scriptExtra}`.replace(/\s+/g, " ").trim(),
    "style-src 'unsafe-inline'",
    "connect-src 'self'",
    extraScript ? "frame-src https://challenges.cloudflare.com" : "frame-src 'none'",
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
      ...(noStore ? { "cache-control": "no-store" } : {}),
    },
  });
}

// The entry screen ("/"): see landing.js.
export function waitlistPage(env) {
  const n = nonce();
  const site = env.TURNSTILE_SITEKEY || "";
  const contact = env.CONTACT_EMAIL ? ` To have it deleted, write to ${esc(env.CONTACT_EMAIL)}.` : "";
  // DOWNLOAD_URL is the update server's /download (cloudflare/update), where the buttons lead
  const download = /^https:\/\//.test(env.DOWNLOAD_URL || "") ? env.DOWNLOAD_URL : "";
  return page(landingHTML(n, site, contact, download, esc), n, site ? "https://challenges.cloudflare.com" : "");
}

// The privacy page ("/privacy"): what the server keeps (migrations/*.sql) and how to have it deleted, in plain words.
export function privacyPage(env) {
  const n = nonce();
  const contact = env.CONTACT_EMAIL ? `<a href="mailto:${esc(env.CONTACT_EMAIL)}">${esc(env.CONTACT_EMAIL)}</a>` : "the address your invite came from";
  const html = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Ryudit — privacy</title><meta name="color-scheme" content="light dark"><style>
:root{--bg:#fbfaf7;--fg:#1d1c1a;--mut:#6b6862;--line:#e4e1da;--link:#1d1c1a}
@media (prefers-color-scheme:dark){:root{--bg:#121212;--fg:#ecebe8;--mut:#9a978f;--line:#2a2a2a;--link:#ecebe8}}
*{box-sizing:border-box}body{margin:0;background:var(--bg);color:var(--fg);font:16px/1.6 ui-sans-serif,system-ui,-apple-system,"Segoe UI",sans-serif}
main{max-width:40rem;margin:0 auto;padding:48px 16px 64px}h1{font-size:1.6rem;margin:0 0 8px}h2{font-size:1.05rem;margin:32px 0 8px}
p,li{color:var(--fg)}.mut{color:var(--mut)}ul{padding-left:1.2em}li{margin:6px 0}a{color:var(--link)}
footer{margin-top:48px;padding-top:16px;border-top:1px solid var(--line);font-size:.9rem}
</style></head><body><main>
<h1>Privacy</h1>
<p class="mut">What this server keeps, and how to have it deleted.</p>
<h2>Without an account</h2>
<p>The app needs no account. Your notes stay on your computer, in a file of their own, and nothing of them is sent anywhere.
The app asks the update server now and then whether there is a new version; that request carries the app's version and nothing of yours.</p>
<h2>The waitlist</h2>
<ul>
<li>Your email address, the name if you gave one, when you signed up, and whether an invite was sent.</li>
<li>For an invite: a hash of the code (not the code itself), when it was made and when it expires or was used. The code is emailed to you through Resend.</li>
</ul>
<p>The address is used only to send your invite, and is never shared or sold. A bot check by Cloudflare Turnstile guards the form.</p>
<h2>An account for sync</h2>
<ul>
<li>Your email address as the account's name, when it was made, its quota, how much it uses, and whether it is active.</li>
<li>Your password never reaches the server. It keeps a hash of a value your computer derives from it, the data key sealed by your password
(it cannot open it), and a hash of what your recovery key proves.</li>
<li>For each computer you sign in on: a hash of its token, the name it gave, when it signed in and when it was last seen.</li>
<li>Your notes, encrypted on your computer before they are sent: the server holds sealed bytes it cannot read, with up to 10 earlier versions of a page for 30 days
and the mark of a deleted file for 180 days, so your other computers learn of it.</li>
</ul>
<h2>Logs</h2>
<ul>
<li>To limit abuse, attempts are counted by IP address (and by account) for an hour or so; the counts are deleted within a few hours.</li>
<li>A short log of what happened (a signup, an invite, an account made, recovered or deleted) names the address it concerns.</li>
<li>The server runs on Cloudflare, which keeps its own short request logs.</li>
</ul>
<h2>Having it deleted</h2>
<ul>
<li>An account: in the app, open the account's profile and choose Delete account. At once, your files and their versions, your computers' tokens,
your invite, your row on the waitlist and your address in the log are deleted.</li>
<li>Only the waitlist, or anything else: write to ${contact}, and it is deleted, with your address in the log.</li>
</ul>
<footer><a href="/">Ryudit</a></footer>
</main></body></html>`;
  return page(html, n);
}

// The administrator's page ("/admin"): a canvas, and the panel drawn on it with synth-ui (admin/main.js, built into
// admin.gen.js and served under /admin/ beside its WebAssembly). The module is the Worker's own, by the page's
// nonce: no script of another site on the page that holds the admin key. WebAssembly is allowed, nothing more.
export function adminPage() {
  const n = nonce();
  const html = `<!doctype html><html lang="tr"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="robots" content="noindex"><title>Ryudit — yönetim</title>
<style>html,body{margin:0;height:100%;background:#141210}#panel{display:block;width:100vw;height:100vh;outline:none}</style></head>
<body><canvas id="panel" tabindex="0"></canvas>
<noscript>Bu panel JavaScript ve WebAssembly ister.</noscript>
<script type="module" nonce="${n}" src="/admin/${ADMIN_JS_PATH}"></script></body></html>`;
  return page(html, n, "", true, "'wasm-unsafe-eval'");
}
