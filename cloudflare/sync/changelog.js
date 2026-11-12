// The changelog ("/changelog"): every release of the app, newest first, as the release workflow wrote it to the
// releases bucket (changelog.json, made by `go run ./scripts/release changelog` from the git tags). A new tag shows
// here without a deploy of this Worker: the page is drawn from the file and kept in Cloudflare's cache for a few
// minutes, so the bucket is read once in that time, not at every visit. The app's "what's new" note links here.
//
// The file has no issue numbers, no addresses and no internal work (scripts/release/changelog.go); this page adds
// none. It is plain HTML with no script, in the entry screen's tokens (landing/app.css).

const CACHE_SECONDS = 300;
const esc = (s) => String(s).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]);

// changelogPage answers GET /changelog, from the cache when it can.
export async function changelogPage(request, env, ctx) {
  const cache = typeof caches !== "undefined" ? caches.default : null;
  const key = new Request(new URL("/changelog", request.url).toString(), { method: "GET" });
  if (cache) {
    const hit = await cache.match(key);
    if (hit) return hit;
  }
  let data = null;
  try {
    const obj = env.RELEASES ? await env.RELEASES.get("changelog.json") : null;
    if (obj) data = await obj.json();
  } catch (e) {
    console.error("changelog.json:", e);
  }
  const res = new Response(changelogHTML(data?.releases || []), {
    headers: {
      "content-type": "text/html; charset=utf-8",
      "content-security-policy": "default-src 'none'; style-src 'unsafe-inline'; img-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'",
      "x-frame-options": "DENY",
      "referrer-policy": "no-referrer",
      "x-content-type-options": "nosniff",
      "cache-control": `public, max-age=${CACHE_SECONDS}`,
    },
  });
  if (cache && data) ctx?.waitUntil?.(cache.put(key, res.clone()));
  return res;
}

// longDate is "November 11, 2026" for "2026-11-11", read as a day, not an instant.
function longDate(d) {
  const t = new Date(d + "T12:00:00Z");
  if (isNaN(t)) return esc(d);
  return t.toLocaleDateString("en-US", { year: "numeric", month: "long", day: "numeric", timeZone: "UTC" });
}

// item is one change: an "Area: text" title shows its area as a quiet label before the text.
function item(s) {
  const m = /^([^:]{1,40}): (.+)$/.exec(s);
  if (m && m[1].split(" ").length <= 2) {
    const text = /^[a-z][^_\s]*(\s|$)/.test(m[2]) ? m[2][0].toUpperCase() + m[2].slice(1) : m[2];
    return `<li><span class="tag">${esc(m[1])}</span>${esc(text)}</li>`;
  }
  return `<li>${esc(s)}</li>`;
}

function releaseHTML(r, latest) {
  const id = esc(r.version);
  const badges = [latest ? `<span class="badge latest">Latest</span>` : "", r.prerelease ? `<span class="badge pre">Preview</span>` : ""].join("");
  let body;
  if (r.first) body = `<p class="lede">The first release.</p>`;
  else if (!r.groups?.length) body = `<p class="lede">Smaller fixes and improvements.</p>`;
  else
    body = r.groups
      .map((g) =>
        g.area
          ? `<section class="group"><h3>${esc(g.area)}</h3><ul>${g.items.map((s) => `<li>${esc(s)}</li>`).join("")}</ul></section>`
          : `<ul class="lead">${g.items.map(item).join("")}</ul>`,
      )
      .join("");
  return `<article class="release" id="${id}">
<header class="meta"><a class="ver" href="#${id}">${esc(r.version.replace(/^v/, ""))}</a><time datetime="${esc(r.date)}">${longDate(r.date)}</time><div class="badges">${badges}</div></header>
<div class="changes">${body}</div></article>`;
}

// changelogHTML is the page for a list of releases, newest first.
export function changelogHTML(releases) {
  const list = releases.length
    ? releases.map((r, i) => releaseHTML(r, i === 0)).join("\n")
    : `<p class="empty">No releases yet. The first one will be listed here.</p>`;
  return `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover">
<title>Changelog — Celestine</title><link rel="icon" href="/icon.png"><link rel="apple-touch-icon" href="/icon.png">
<meta name="description" content="What is new in each version of Celestine."><meta name="color-scheme" content="light dark"><style>
:root{color-scheme:light dark;
  --bg:light-dark(oklch(97% .003 286),oklch(0% 0 0));
  --card:light-dark(oklch(100% 0 0),oklch(16% .004 286));
  --text:light-dark(oklch(23% .004 286),oklch(97% .003 286));
  --muted:light-dark(oklch(51% .008 286),oklch(68% .007 286));
  --faint:light-dark(oklch(65% .006 286),oklch(52% .006 286));
  --hairline:light-dark(oklch(0% 0 0 / 8%),oklch(100% 0 0 / 10%));
  --accent:light-dark(oklch(56% .19 256),oklch(68% .17 256));
  --accent-soft:color-mix(in oklab,var(--accent) 12%,transparent);
  --pre:light-dark(oklch(55% .13 70),oklch(78% .12 75));
  --pre-soft:color-mix(in oklab,var(--pre) 14%,transparent);
  --shadow:0 1px 2px light-dark(oklch(0% 0 0 / 4%),oklch(0% 0 0 / 40%)),0 8px 24px light-dark(oklch(0% 0 0 / 5%),oklch(0% 0 0 / 40%));
  --font-sans:-apple-system,BlinkMacSystemFont,"SF Pro Text","Helvetica Neue",Inter,system-ui,sans-serif;
  --font-mono:ui-monospace,"SF Mono",Menlo,Consolas,monospace}
*{box-sizing:border-box}
html,body{margin:0;background:var(--bg)}
body{color:var(--text);font:15px/1.6 var(--font-sans);-webkit-font-smoothing:antialiased}
::selection{background:color-mix(in oklab,var(--accent) 28%,transparent)}
a{color:inherit}
.wrap{max-width:60rem;margin:0 auto;padding:0 16px}
.bar{display:flex;align-items:center;justify-content:space-between;padding:20px 0}
.brand{display:inline-flex;align-items:center;gap:10px;text-decoration:none;font-weight:600;letter-spacing:-.01em}
.brand img{width:28px;height:28px;border-radius:7px}
.bar nav a{color:var(--muted);text-decoration:none;font-size:14px;margin-left:18px}
.bar nav a:hover{color:var(--text)}
.hero{padding:56px 0 40px;border-bottom:1px solid var(--hairline)}
.hero h1{font-size:clamp(2rem,5vw,2.75rem);line-height:1.1;letter-spacing:-.03em;margin:0 0 10px}
.hero p{margin:0;color:var(--muted);font-size:1.05rem;max-width:34rem}
.release{display:grid;grid-template-columns:11rem 1fr;gap:32px;padding:40px 0;border-bottom:1px solid var(--hairline)}
.meta{position:sticky;top:24px;align-self:start;display:flex;flex-direction:column;gap:4px}
.ver{font:600 1.5rem/1.2 var(--font-sans);letter-spacing:-.02em;text-decoration:none;font-variant-numeric:tabular-nums}
.ver::before{content:"v";color:var(--faint);font-weight:500;margin-right:1px}
.ver:hover{color:var(--accent)}
time{color:var(--muted);font-size:13.5px}
.badges{display:flex;flex-wrap:wrap;gap:6px;margin-top:6px}
.badge{font-size:11.5px;font-weight:600;letter-spacing:.02em;padding:2px 8px;border-radius:999px;line-height:1.5}
.badge.latest{color:var(--accent);background:var(--accent-soft)}
.badge.pre{color:var(--pre);background:var(--pre-soft)}
.changes{background:var(--card);border:1px solid var(--hairline);border-radius:14px;box-shadow:var(--shadow);padding:22px 26px;min-width:0}
.changes ul{margin:0;padding:0;list-style:none}
.changes li{position:relative;padding:5px 0 5px 18px;overflow-wrap:anywhere}
.changes li::before{content:"";position:absolute;left:3px;top:.95em;width:5px;height:5px;border-radius:50%;background:var(--faint)}
.lead li{font-size:15.5px}
.lead + .group,.group + .group{margin-top:20px;padding-top:18px;border-top:1px solid var(--hairline)}
.group h3{margin:0 0 4px;font-size:12px;font-weight:600;text-transform:uppercase;letter-spacing:.07em;color:var(--muted)}
.group li{color:color-mix(in oklab,var(--text) 88%,var(--muted))}
.tag{display:inline-block;margin-right:8px;padding:0 7px;border-radius:6px;font-size:12px;font-weight:600;line-height:1.75;color:var(--muted);background:color-mix(in oklab,var(--muted) 12%,transparent);vertical-align:1px}
.lede{margin:0;color:var(--muted)}
.empty{padding:48px 0;color:var(--muted)}
footer{padding:32px 0 56px;color:var(--muted);font-size:13.5px;display:flex;gap:18px}
footer a{color:var(--muted);text-decoration:none}footer a:hover{color:var(--text)}
@media (max-width:720px){
  .hero{padding:32px 0 24px}
  .release{grid-template-columns:1fr;gap:14px;padding:28px 0}
  .meta{position:static;flex-direction:row;flex-wrap:wrap;align-items:baseline;gap:4px 12px}
  .badges{margin-top:0}
  .changes{padding:16px 18px;border-radius:12px}
}
</style></head><body><div class="wrap">
<div class="bar"><a class="brand" href="/"><img src="/icon.png" alt="">Celestine</a><nav><a href="/">Download</a><a href="/privacy">Privacy</a></nav></div>
<div class="hero"><h1>Changelog</h1><p>What is new in each version of Celestine. The app updates itself; this is what came with each update.</p></div>
<main>
${list}
</main>
<footer><a href="/">Celestine</a><a href="/privacy">Privacy</a></footer>
</div></body></html>`;
}
