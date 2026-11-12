// The changelog ("/changelog"): every release of the app, newest first, as the release workflow wrote it to the
// releases bucket (changelog.json, made by `go run ./scripts/release changelog` from the git tags). A new tag shows
// here without a deploy of this Worker: the page is drawn from the file and kept in Cloudflare's cache for a few
// minutes, so the bucket is read once in that time, not at every visit. The app's "what's new" note links here.
//
// The file has no issue numbers, no addresses and no internal work (scripts/release/changelog.go); this page adds
// none. It is plain HTML with no script, in the entry screen's tokens (landing/app.css): a watercolor banner
// (banner.bin, made for the page), on the left a rail of bars, one for each release and as long as it is big, the
// way the app's page history draws its copies, and on the right each release: its title, its badges, a summary and
// its changes in two folding boxes (<details>), the new and improved and the fixes.

import BANNER from "./banner.bin"; // the banner, a WebP (made with an image model from the app's icon, in watercolor)

const CACHE_SECONDS = 300;
const SHOWN = 10; // the changes a box shows before "Show n more"
const OPEN = 3; // the newest releases whose boxes are open
const esc = (s) => String(s).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]);

// changelogPage answers GET /changelog, from the cache when it can.
export async function changelogPage(request, env, ctx) {
  // (not under `wrangler dev`, where a change to the page should show at once)
  const local = /^(localhost|127\.0\.0\.1)$/.test(new URL(request.url).hostname);
  const cache = typeof caches !== "undefined" && !local ? caches.default : null;
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

// BANNER_V changes the banner's address when the picture changes, so no browser keeps the old one for its day.
const BANNER_V = (BANNER.byteLength || BANNER.length || 0).toString(36);

// bannerImage answers GET /changelog/banner.webp. A new banner comes with a deploy, so a day's cache is enough.
export function bannerImage() {
  return new Response(BANNER, { headers: { "content-type": "image/webp", "cache-control": "public, max-age=86400", "x-content-type-options": "nosniff" } });
}

// shortDate is "Nov 11, 2026" for "2026-11-11", read as a day, not an instant.
function shortDate(d) {
  const t = new Date(d + "T12:00:00Z");
  if (isNaN(t)) return esc(d);
  return t.toLocaleDateString("en-US", { year: "numeric", month: "short", day: "numeric", timeZone: "UTC" });
}

const plural = (n, one, many) => `${n} ${n === 1 ? one : many}`;

// changeHTML is one change, its area in bold before it as the commit title had it ("Sync: …").
const changeHTML = (c) => `<li>${c.area ? `<strong>${esc(c.area)}:</strong> ` : ""}${esc(c.text)}</li>`;

// boxHTML is a folding box of changes: the first SHOWN of them, and the rest behind "Show n more".
function boxHTML(label, list, open) {
  if (!list?.length) return "";
  const cut = list.length - SHOWN < 3 ? list.length : SHOWN; // two more are shown rather than folded
  const head = list.slice(0, cut).map(changeHTML).join("");
  const rest = list.slice(cut);
  const more = rest.length
    ? `<details class="more"><summary>Show ${plural(rest.length, "more change", "more changes")}</summary><ul>${rest.map(changeHTML).join("")}</ul></details>`
    : "";
  return `<details class="box"${open ? " open" : ""}><summary>${label} <span class="n">(${list.length})</span></summary><ul>${head}</ul>${more}</details>`;
}

// lede is the release's summary: its tag's words, or one made from what it has.
function lede(r) {
  if (r.summary) return r.summary.split(/\n\s*\n/).map((p) => `<p class="lede">${esc(p.trim())}</p>`).join("");
  const v = esc(r.version.replace(/^v/, ""));
  if (r.first) return `<p class="lede">Celestine ${v} is where it starts: a notes app of its own kind, free to download, with no account needed.</p>`;
  const f = r.features?.length || 0;
  const x = r.fixes?.length || 0;
  if (!f && !x) return `<p class="lede">Celestine ${v} brings smaller fixes and improvements.</p>`;
  const parts = [f ? plural(f, "new thing or improvement", "new things and improvements") : "", x ? plural(x, "fix", "fixes") : ""].filter(Boolean);
  return `<p class="lede">Celestine ${v} brings ${parts.join(" and ")}.</p>`;
}

// size is how many changes a release has, for the length of its bar.
const size = (r) => (r.features?.length || 0) + (r.fixes?.length || 0);

// railHTML is the rail of releases: a bar for each, its version and date before it, its length by how much it changed
// (on a square root, so a big release does not dwarf the rest), as the app's page history draws its copies. A bar
// grows a little under the pointer, its neighbours less.
function railHTML(releases) {
  const most = Math.max(1, ...releases.map(size));
  const rows = releases.map((r, i) => {
    const len = r.first ? 0.55 : 0.28 + 0.72 * Math.sqrt(size(r) / most);
    const v = esc(r.version.replace(/^v/, ""));
    return `<li><a class="bar${i === 0 ? " now" : ""}" href="#${esc(r.version)}" style="--len:${len.toFixed(3)}" title="Version ${v} on ${shortDate(r.date)}"><span class="lbl"><b>${v}</b><time datetime="${esc(r.date)}">${shortDate(r.date)}</time></span><span class="track"><span class="line"></span></span></a></li>`;
  });
  return `<nav class="rail" aria-label="Releases"><ol>${rows.join("")}</ol></nav>`;
}

function releaseHTML(r, i) {
  const id = esc(r.version);
  const v = esc(r.version.replace(/^v/, ""));
  const badges = [
    i === 0 ? `<span class="badge latest"><i></i>Latest</span>` : "",
    r.prerelease ? `<span class="badge pre">Preview</span>` : `<span class="badge">Stable</span>`,
    `<span class="badge">macOS · Windows · Linux</span>`,
  ].join("");
  const open = i < OPEN;
  return `<article class="release" id="${id}">
<p class="kicker"><a href="#${id}">${v}</a><span>·</span><time datetime="${esc(r.date)}">${shortDate(r.date)}</time></p>
<h2>${esc(r.title || r.version)}</h2><div class="badges">${badges}</div>${lede(r)}
${boxHTML("New and improved", r.features, open)}${boxHTML("Fixes", r.fixes, open)}</article>`;
}

// changelogHTML is the page for a list of releases, newest first.
export function changelogHTML(releases) {
  const list = releases.length
    ? releases.map(releaseHTML).join("\n")
    : `<p class="empty">No releases yet. The first one will be listed here.</p>`;
  return `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover">
<title>Changelog — Celestine</title><link rel="icon" href="/icon.png"><link rel="apple-touch-icon" href="/icon.png">
<meta name="description" content="What is new in each version of Celestine."><meta name="color-scheme" content="light dark"><style>
:root{color-scheme:light dark;
  --bg:light-dark(oklch(98.5% .002 286),oklch(9% .004 286));
  --card:light-dark(oklch(100% 0 0),oklch(15% .004 286));
  --well:light-dark(oklch(97% .003 286),oklch(18% .004 286));
  --text:light-dark(oklch(23% .004 286),oklch(96% .003 286));
  --body:light-dark(oklch(34% .006 286),oklch(84% .005 286));
  --muted:light-dark(oklch(53% .008 286),oklch(66% .007 286));
  --faint:light-dark(oklch(80% .006 286),oklch(36% .006 286));
  --line:light-dark(oklch(0% 0 0 / 9%),oklch(100% 0 0 / 11%));
  --accent:light-dark(oklch(58% .17 256),oklch(72% .14 256));
  --ok:light-dark(oklch(52% .14 150),oklch(76% .14 150));
  --pre:light-dark(oklch(56% .14 60),oklch(80% .12 70));
  --ink:light-dark(oklch(25% .02 270),oklch(94% .01 270));
  --on-ink:light-dark(oklch(98% 0 0),oklch(18% .01 270));
  --shadow:0 1px 2px light-dark(oklch(0% 0 0 / 5%),oklch(0% 0 0 / 50%)),0 10px 30px light-dark(oklch(0% 0 0 / 7%),oklch(0% 0 0 / 45%));
  --ease:cubic-bezier(.2,.8,.2,1);
  --font:-apple-system,BlinkMacSystemFont,"SF Pro Text","Helvetica Neue",Inter,system-ui,sans-serif}
*{box-sizing:border-box}
html{scroll-behavior:smooth;scroll-padding-top:84px}
html,body{margin:0;background:var(--bg)}
body{color:var(--text);font:15px/1.6 var(--font);-webkit-font-smoothing:antialiased}
::selection{background:color-mix(in oklab,var(--accent) 28%,transparent)}
a{color:inherit}
.wrap{max-width:64rem;margin:0 auto;padding:0 16px}
.nav{position:sticky;top:12px;z-index:5;display:flex;align-items:center;gap:8px;margin:12px auto 0;max-width:64rem;padding:7px 7px 7px 16px;border-radius:999px;
  background:color-mix(in oklab,var(--card) 78%,transparent);border:1px solid var(--line);box-shadow:var(--shadow);backdrop-filter:blur(14px) saturate(1.4);-webkit-backdrop-filter:blur(14px) saturate(1.4)}
.brand{display:inline-flex;align-items:center;gap:9px;text-decoration:none;font-weight:600;letter-spacing:-.01em;margin-right:auto}
.brand img{width:24px;height:24px;border-radius:6px}
.nav .link{color:var(--muted);text-decoration:none;font-size:14px;padding:6px 10px;border-radius:999px}
.nav .link:hover,.nav .link[aria-current]{color:var(--text)}
.nav .dl{background:var(--ink);color:var(--on-ink);text-decoration:none;font-size:14px;font-weight:600;padding:8px 16px;border-radius:999px;box-shadow:inset 0 1px 0 oklch(100% 0 0 / 18%),0 1px 2px oklch(0% 0 0 / 20%)}
.nav .dl:hover{opacity:.9}
.banner{position:relative;margin:16px 0 0;height:300px;border-radius:22px;overflow:hidden;background:#f7f4f1 url(/changelog/banner.webp?v=${BANNER_V}) center 54%/cover no-repeat;
  -webkit-mask-image:linear-gradient(#000 80%,transparent);mask-image:linear-gradient(#000 80%,transparent)}
@media (prefers-color-scheme:dark){.banner{filter:brightness(.82) saturate(.95)}}
.banner h1{position:absolute;left:30px;top:28px;margin:0;color:oklch(24% .03 265);font-size:clamp(1.9rem,4.6vw,2.7rem);line-height:1.1;letter-spacing:-.03em}
.banner p{position:absolute;left:30px;top:calc(28px + clamp(1.9rem,4.6vw,2.7rem) * 1.1 + 6px);margin:0;max-width:22rem;color:oklch(36% .03 265);font-size:15px}
.page{display:grid;grid-template-columns:15.5rem 1fr;column-gap:40px;margin-top:8px}
.rail{position:sticky;top:84px;align-self:start;padding:34px 0 24px}
.rail ol{list-style:none;margin:0;padding:0;display:flex;flex-direction:column;gap:2px}
.bar{display:grid;grid-template-columns:8.6rem 1fr;align-items:center;column-gap:14px;padding:6px 8px;border-radius:8px;text-decoration:none;color:var(--muted);transition:background-color .2s var(--ease)}
.bar:hover{background:var(--well)}
.lbl{display:flex;align-items:baseline;gap:8px;white-space:nowrap;font-variant-numeric:tabular-nums}
.lbl b{font-size:13px;font-weight:600;color:var(--text);min-width:2.6rem;text-align:right}
.lbl time{font-size:12.5px;color:var(--muted)}
.track{display:block;height:10px;position:relative}
.line{position:absolute;left:0;top:50%;height:6px;margin-top:-3px;width:calc(var(--len) * 100%);border-radius:6px;background:var(--faint);transform-origin:left center;
  transition:transform .25s var(--ease),background-color .2s var(--ease)}
.bar.now .line{background:var(--accent)}
.bar:hover .line,.bar:focus-visible .line{transform:scale(1.12,1.35);background:var(--muted)}
.bar.now:hover .line{background:var(--accent)}
.rail li:has(+ li .bar:hover) .line,.rail li:has(.bar:hover) + li .line{transform:scale(1.05,1.15)}
.bar:hover .lbl b{color:var(--accent)}
main{min-width:0}
.release{padding:36px 0 34px;border-bottom:1px solid var(--line)}
.release:last-child{border-bottom:0}
.kicker{display:flex;gap:8px;align-items:center;margin:0 0 6px;color:var(--muted);font-size:13px}
.kicker a{font-weight:600;color:var(--text);text-decoration:none;font-variant-numeric:tabular-nums}
.kicker a::before{content:"v";color:var(--muted);font-weight:500}
h2{margin:0 0 10px;font-size:clamp(1.45rem,3vw,1.85rem);line-height:1.2;letter-spacing:-.025em;text-wrap:balance}
.badges{display:flex;flex-wrap:wrap;gap:6px;margin-bottom:14px}
.badge{display:inline-flex;align-items:center;gap:6px;font-size:12px;font-weight:500;line-height:1;padding:5px 9px;border-radius:999px;border:1px solid var(--line);color:var(--muted);background:var(--card)}
.badge.latest{color:var(--ok);border-color:color-mix(in oklab,var(--ok) 35%,transparent);background:color-mix(in oklab,var(--ok) 8%,var(--card))}
.badge.latest i{width:6px;height:6px;border-radius:50%;background:currentColor;box-shadow:0 0 0 3px color-mix(in oklab,currentColor 22%,transparent)}
.badge.pre{color:var(--pre);border-color:color-mix(in oklab,var(--pre) 38%,transparent);background:color-mix(in oklab,var(--pre) 9%,var(--card))}
.lede{margin:0 0 16px;color:var(--body);font-size:15.5px;max-width:44rem}
.box{border:1px solid var(--line);border-radius:12px;background:var(--card);margin-top:12px;overflow:hidden}
.box>summary{display:flex;align-items:center;gap:6px;padding:10px 14px;background:var(--well);font-weight:600;font-size:14px;cursor:pointer;list-style:none;user-select:none}
.box>summary::-webkit-details-marker,.more>summary::-webkit-details-marker{display:none}
.box>summary .n{color:var(--muted);font-weight:500}
.box>summary::after{content:"+";margin-left:auto;color:var(--muted);font-weight:400;font-size:17px;line-height:1}
.box[open]>summary{border-bottom:1px solid var(--line)}
.box[open]>summary::after{content:"−"}
.box ul{margin:0;padding:10px 16px 12px 34px;color:var(--body)}
.box li{margin:6px 0;padding-left:2px;overflow-wrap:anywhere}
.box li::marker{color:var(--muted)}
.box strong{color:var(--text);font-weight:600}
.more{border-top:1px dashed var(--line)}
.more>summary{padding:9px 16px;color:var(--accent);font-size:13.5px;font-weight:500;cursor:pointer;list-style:none}
.more[open]>summary{display:none}
.more ul{padding-top:2px}
.empty{padding:48px 0;color:var(--muted)}
footer{display:flex;gap:18px;padding:28px 0 56px;border-top:1px solid var(--line);color:var(--muted);font-size:13.5px}
footer a{color:var(--muted);text-decoration:none}footer a:hover{color:var(--text)}
:focus-visible{outline:2px solid var(--accent);outline-offset:2px;border-radius:6px}
@media (prefers-reduced-motion:reduce){html{scroll-behavior:auto}.line{transition:none}}
@media (max-width:760px){
  .nav{margin:10px 10px 0;top:10px}.nav .link.opt{display:none}
  .banner{height:200px;border-radius:18px;background-position:88% 55%}.banner h1,.banner p{left:20px}.banner p{font-size:14px;max-width:13rem}
  .page{grid-template-columns:1fr}
  .rail{position:static;padding:18px 0 4px;overflow-x:auto}
  .rail ol{flex-direction:row;align-items:flex-end;gap:2px;min-height:110px}
  .bar{display:flex;flex-direction:column-reverse;align-items:center;gap:8px;padding:6px 4px}
  .lbl{flex-direction:column;align-items:center;gap:0}.lbl b{min-width:0;text-align:center}.lbl time{display:none}
  .track{width:44px;height:72px}
  .line{left:50%;top:auto;bottom:0;margin:0 0 0 -3px;width:6px;height:calc(var(--len) * 100%);transform-origin:center bottom}
  .release{padding:24px 0}
}
</style></head><body>
<nav class="nav"><a class="brand" href="/"><img src="/icon.png" alt="">Celestine</a><a class="link" href="/changelog" aria-current="page">Changelog</a><a class="link opt" href="/privacy">Privacy</a><a class="dl" href="/">Download</a></nav>
<div class="wrap">
<div class="banner" role="img" aria-label="A watercolor landscape at dawn, two blue crystals standing in a meadow"><h1>Changelog</h1><p>What came with each version of Celestine. The app updates itself.</p></div>
<div class="page">${releases.length ? railHTML(releases) : "<div></div>"}<main>
${list}
</main></div>
<footer><a href="/">Celestine</a><a href="/privacy">Privacy</a></footer>
</div></body></html>`;
}
