// The changelog ("/changelog"): every release of the app, newest first, as the release workflow wrote it to the
// releases bucket (changelog.json, made by `go run ./scripts/release changelog` from the git tags). A new tag shows
// here without a deploy of this Worker: the page is drawn from the file and kept in Cloudflare's cache for a few
// minutes, so the bucket is read once in that time, not at every visit. The app's "what's new" note links here.
//
// The file has no issue numbers, no addresses and no internal work (scripts/release/changelog.go); this page adds
// none. It is plain HTML with no script, in the entry screen's tokens (landing/app.css): a
// timeline, each release with its date and version on the left, and on the right its title, its badges, a summary and
// its changes in two folding boxes (<details>), the new and improved and the fixes.

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

function releaseHTML(r, i) {
  const id = esc(r.version);
  const badges = [
    i === 0 ? `<span class="badge latest"><i></i>Latest</span>` : "",
    r.prerelease ? `<span class="badge pre">Preview</span>` : `<span class="badge">Stable</span>`,
    `<span class="badge">macOS · Windows · Linux</span>`,
  ].join("");
  const open = i < OPEN;
  return `<article class="release" id="${id}">
<div class="when"><time datetime="${esc(r.date)}">${shortDate(r.date)}</time><a class="ver" href="#${id}">${esc(r.version.replace(/^v/, ""))}</a></div>
<div class="body"><h2>${esc(r.title || r.version)}</h2><div class="badges">${badges}</div>${lede(r)}
${boxHTML("New and improved", r.features, open)}${boxHTML("Fixes", r.fixes, open)}</div></article>`;
}

// SKY is the banner: a sky at dusk over hills, with stars and a crescent, drawn here so the page needs no picture.
const SKY = `<svg class="sky" viewBox="0 0 1200 260" preserveAspectRatio="xMidYMid slice" aria-hidden="true">
<defs>
<linearGradient id="dusk" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="var(--sky-top)"/><stop offset=".62" stop-color="var(--sky-mid)"/><stop offset="1" stop-color="var(--sky-low)"/></linearGradient>
<radialGradient id="glow" cx=".78" cy=".3" r=".35"><stop offset="0" stop-color="var(--glow)" stop-opacity=".55"/><stop offset="1" stop-color="var(--glow)" stop-opacity="0"/></radialGradient>
<mask id="moon"><rect width="1200" height="260" fill="#fff"/><circle cx="948" cy="66" r="25" fill="#000"/></mask>
</defs>
<rect width="1200" height="260" fill="url(#dusk)"/><rect width="1200" height="260" fill="url(#glow)"/>
<g fill="var(--star)">${[
  [60, 40, 1.4], [140, 92, 1], [210, 30, 1.8], [300, 70, 1.1], [372, 22, 1.3], [430, 110, .9], [512, 48, 1.6], [590, 88, 1], [655, 26, 1.2],
  [720, 64, 1.7], [800, 30, 1], [860, 112, 1.2], [1030, 40, 1.5], [1090, 96, 1], [1150, 26, 1.3], [258, 132, .9], [690, 130, .8], [1000, 140, .9],
  [96, 150, .8], [560, 150, .8], [1170, 150, .9], [35, 100, 1], [470, 20, 1], [1110, 60, 1.1],
].map(([x, y, r]) => `<circle cx="${x}" cy="${y}" r="${r}"/>`).join("")}</g>
<path d="M930 30 l2 7 7 2 -7 2 -2 7 -2 -7 -7 -2 7 -2z M330 46 l1.4 5 5 1.4 -5 1.4 -1.4 5 -1.4 -5 -5 -1.4 5 -1.4z" fill="var(--star)"/>
<circle cx="932" cy="76" r="27" fill="var(--moon)" mask="url(#moon)"/>
<path d="M0 205 C120 170 230 180 330 196 S560 160 700 176 S960 150 1200 184 V260 H0Z" fill="var(--hill-far)"/>
<path d="M0 232 C160 200 300 214 420 226 S700 196 860 214 S1080 204 1200 222 V260 H0Z" fill="var(--hill-near)"/>
</svg>`;

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
  --line:light-dark(oklch(0% 0 0 / 9%),oklch(100% 0 0 / 11%));
  --accent:light-dark(oklch(54% .19 256),oklch(70% .16 256));
  --ok:light-dark(oklch(52% .14 150),oklch(76% .14 150));
  --pre:light-dark(oklch(56% .14 60),oklch(80% .12 70));
  --ink:light-dark(oklch(25% .02 270),oklch(94% .01 270));
  --on-ink:light-dark(oklch(98% 0 0),oklch(18% .01 270));
  --shadow:0 1px 2px light-dark(oklch(0% 0 0 / 5%),oklch(0% 0 0 / 50%)),0 10px 30px light-dark(oklch(0% 0 0 / 7%),oklch(0% 0 0 / 45%));
  --sky-top:light-dark(oklch(47% .12 275),oklch(18% .06 275));
  --sky-mid:light-dark(oklch(72% .09 300),oklch(32% .08 295));
  --sky-low:light-dark(oklch(88% .06 55),oklch(48% .09 20));
  --glow:light-dark(oklch(97% .04 90),oklch(85% .08 80));
  --star:light-dark(oklch(99% .01 90),oklch(96% .03 90));
  --moon:light-dark(oklch(98% .03 90),oklch(93% .06 90));
  --hill-far:light-dark(oklch(62% .06 300),oklch(22% .04 290));
  --hill-near:light-dark(oklch(45% .06 290),oklch(14% .03 285));
  --font:-apple-system,BlinkMacSystemFont,"SF Pro Text","Helvetica Neue",Inter,system-ui,sans-serif}
*{box-sizing:border-box}
html,body{margin:0;background:var(--bg)}
body{color:var(--text);font:15px/1.6 var(--font);-webkit-font-smoothing:antialiased}
::selection{background:color-mix(in oklab,var(--accent) 28%,transparent)}
a{color:inherit}
.wrap{max-width:62rem;margin:0 auto;padding:0 16px}
.nav{position:sticky;top:12px;z-index:5;display:flex;align-items:center;gap:8px;margin:12px auto 0;max-width:62rem;padding:7px 7px 7px 16px;border-radius:999px;
  background:color-mix(in oklab,var(--card) 78%,transparent);border:1px solid var(--line);box-shadow:var(--shadow);backdrop-filter:blur(14px) saturate(1.4);-webkit-backdrop-filter:blur(14px) saturate(1.4)}
.brand{display:inline-flex;align-items:center;gap:9px;text-decoration:none;font-weight:600;letter-spacing:-.01em;margin-right:auto}
.brand img{width:24px;height:24px;border-radius:6px}
.nav .link{color:var(--muted);text-decoration:none;font-size:14px;padding:6px 10px;border-radius:999px}
.nav .link:hover,.nav .link[aria-current]{color:var(--text)}
.nav .dl{background:var(--ink);color:var(--on-ink);text-decoration:none;font-size:14px;font-weight:600;padding:8px 16px;border-radius:999px;box-shadow:inset 0 1px 0 oklch(100% 0 0 / 18%),0 1px 2px oklch(0% 0 0 / 20%)}
.nav .dl:hover{opacity:.9}
.banner{position:relative;margin:16px 0 8px;border-radius:22px;overflow:hidden;height:250px;-webkit-mask-image:linear-gradient(#000 78%,transparent);mask-image:linear-gradient(#000 78%,transparent)}
.sky{position:absolute;inset:0;width:100%;height:100%;display:block}
.banner h1{position:absolute;left:28px;top:26px;margin:0;color:#fff;font-size:clamp(1.9rem,4.6vw,2.6rem);line-height:1.1;letter-spacing:-.03em;text-shadow:0 1px 18px oklch(20% .08 280 / 45%)}
.banner p{position:absolute;left:28px;top:calc(26px + clamp(1.9rem,4.6vw,2.6rem) * 1.1 + 6px);margin:0;max-width:30rem;color:oklch(100% 0 0 / 88%);font-size:15px;text-shadow:0 1px 12px oklch(20% .08 280 / 50%)}
.release{display:grid;grid-template-columns:10rem 1fr;column-gap:28px}
.when{position:sticky;top:84px;align-self:start;padding-top:44px;display:flex;flex-direction:column;align-items:flex-start;gap:10px}
time{color:var(--muted);font-size:13.5px}
.ver{font-size:13px;font-weight:700;text-decoration:none;padding:5px 9px;border:1px solid var(--line);border-radius:8px;background:var(--card);font-variant-numeric:tabular-nums;box-shadow:0 1px 2px light-dark(oklch(0% 0 0 / 4%),oklch(0% 0 0 / 40%))}
.ver:hover{border-color:var(--accent);color:var(--accent)}
.body{position:relative;border-left:1px solid var(--line);padding:40px 0 36px 32px;min-width:0}
.body::before{content:"";position:absolute;left:-5px;top:56px;width:9px;height:9px;border-radius:50%;background:var(--text);box-shadow:0 0 0 4px var(--bg)}
.release:first-child .body::before{background:var(--accent);box-shadow:0 0 0 4px var(--bg),0 0 0 7px color-mix(in oklab,var(--accent) 25%,transparent)}
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
footer{display:flex;gap:18px;padding:28px 0 56px 0;margin-left:calc(10rem + 28px);border-top:1px solid var(--line);color:var(--muted);font-size:13.5px}
footer a{color:var(--muted);text-decoration:none}footer a:hover{color:var(--text)}
:focus-visible{outline:2px solid var(--accent);outline-offset:2px;border-radius:6px}
@media (max-width:720px){
  .nav{margin:10px 10px 0;top:10px}.nav .link.opt{display:none}
  .banner{height:200px;border-radius:18px}.banner h1,.banner p{left:20px}.banner p{font-size:14px;max-width:16rem}
  .release{grid-template-columns:1fr}
  .when{position:static;flex-direction:row;align-items:center;padding-top:28px;gap:10px}
  .body{border-left:0;padding:12px 0 24px}.body::before{display:none}
  .box ul{padding-left:28px}
  footer{margin-left:0}
}
</style></head><body>
<nav class="nav"><a class="brand" href="/"><img src="/icon.png" alt="">Celestine</a><a class="link" href="/changelog" aria-current="page">Changelog</a><a class="link opt" href="/privacy">Privacy</a><a class="dl" href="/">Download</a></nav>
<div class="wrap">
<div class="banner">${SKY}<h1>Changelog</h1><p>What came with each version of Celestine. The app updates itself.</p></div>
<main>
${list}
</main>
<footer><a href="/">Celestine</a><a href="/privacy">Privacy</a></footer>
</div></body></html>`;
}
