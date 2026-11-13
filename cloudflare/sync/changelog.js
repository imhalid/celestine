// The changelog ("/changelog"): every release of the app, newest first, as the release workflow wrote it to the
// releases bucket (changelog.json, made by `go run ./scripts/release changelog` from the git tags). A new tag shows
// here without a deploy of this Worker: the page is drawn from the file and kept in Cloudflare's cache for a few
// minutes, so the bucket is read once in that time, not at every visit. The app's "what's new" note links here.
//
// The file has no issue numbers, no addresses and no internal work (scripts/release/changelog.go); this page adds
// none. It is plain HTML with two small scripts (the scheme's and the rail's), in the entry screen's tokens
// (landing/app.css): a watercolor banner (banner.bin, made for the page), on the left a rail of the releases, each
// with its version and its day (the year once above them), the one being read bright, and on the right each release:
// its title, its badges, a summary and its changes in two folding boxes (<details>), the new and improved and the fixes.

// the banner, two WebPs made with an image model from the app's icon, in watercolor: at dawn for the light theme, the
// same scene at night for the dark one
import BANNER from "./banner.bin";
import BANNER_DARK from "./banner-dark.bin";
import { ogTags } from "./og.js";

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
      "content-security-policy": "default-src 'none'; script-src 'self'; style-src 'unsafe-inline'; img-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'",
      "x-frame-options": "DENY",
      "referrer-policy": "no-referrer",
      "x-content-type-options": "nosniff",
      "cache-control": `public, max-age=${CACHE_SECONDS}`,
    },
  });
  if (cache && data) ctx?.waitUntil?.(cache.put(key, res.clone()));
  return res;
}

// bannerV is a picture's address suffix: it changes when the picture does, so no browser keeps the old one for its day.
const bannerV = (b) => (b.byteLength || b.length || 0).toString(36);
const BANNER_V = bannerV(BANNER);
const BANNER_DARK_V = bannerV(BANNER_DARK);

// bannerImage answers GET /changelog/banner.webp and /changelog/banner-dark.webp. A new banner comes with a deploy.
export function bannerImage(dark) {
  return new Response(dark ? BANNER_DARK : BANNER, {
    headers: { "content-type": "image/webp", "cache-control": "public, max-age=86400", "x-content-type-options": "nosniff" },
  });
}

// SCHEME_JS is the page's one script, served at /changelog/scheme.js: the light or dark scheme the visitor chose here
// or on the entry screen (the same "celestine-scheme" in localStorage), set before anything is drawn, and the toggle
// in the top bar that changes it, as the entry screen's does.
const SCHEME_JS = `(() => {
  const root = document.documentElement;
  // dark unless light was chosen (the system's scheme is not asked)
  try { root.dataset.scheme = localStorage.getItem("celestine-scheme") === "light" ? "light" : "dark"; } catch { root.dataset.scheme = "dark"; }
  const now = () => root.dataset.scheme;
  // the banner of the scheme in use is preloaded at once, the other one when the page is idle, so it is there at a switch
  const banners = { light: "/changelog/banner.webp?v=${BANNER_V}", dark: "/changelog/banner-dark.webp?v=${BANNER_DARK_V}" };
  const pre = document.createElement("link");
  pre.rel = "preload"; pre.as = "image"; pre.setAttribute("fetchpriority", "high"); pre.href = banners[now()];
  document.head.append(pre);
  addEventListener("load", () => (window.requestIdleCallback || ((f) => setTimeout(f, 600)))(() => { new Image().src = banners[now() === "dark" ? "light" : "dark"]; }));
  addEventListener("DOMContentLoaded", () => {
    const b = document.querySelector(".icon");
    if (!b) return;
    const label = () => b.setAttribute("aria-label", now() === "dark" ? "Switch to light" : "Switch to dark");
    label();
    b.hidden = false;
    b.addEventListener("click", () => {
      const s = now() === "dark" ? "light" : "dark";
      const apply = () => { root.dataset.scheme = s; label(); };
      // a view transition: the page cross-fades to the other scheme; with none in the browser, or reduced motion asked
      // for, it changes at once
      if (document.startViewTransition && !matchMedia("(prefers-reduced-motion: reduce)").matches) document.startViewTransition(apply);
      else apply();
      try { localStorage.setItem("celestine-scheme", s); } catch {}
    });
  });
})();
`;

// RAIL_JS makes the rail follow the page: the release being read (the last whose top has passed a line a third of the
// way down the window, and the last one at the page's end) is "on", and each row has its distance from it in --d, which
// the CSS turns into brightness and room. Without it the rail is as it was, every row alike.
const RAIL_JS = `(() => {
  const rail = document.querySelector(".rail");
  const rows = [...document.querySelectorAll(".rail li:not(.year)")];
  const rels = [...document.querySelectorAll("main .release")];
  if (!rail || !rows.length || rows.length !== rels.length) return;
  let on = -1, queued = false;
  const update = () => {
    queued = false;
    const line = innerHeight * 0.35;
    let i = 0;
    for (let k = 0; k < rels.length; k++) if (rels[k].getBoundingClientRect().top <= line) i = k;
    if (innerHeight + scrollY >= document.documentElement.scrollHeight - 2) i = rels.length - 1;
    if (i === on) return;
    on = i;
    rows.forEach((li, k) => { li.style.setProperty("--d", Math.abs(k - i)); li.classList.toggle("on", k === i); });
  };
  const ask = () => { if (!queued) { queued = true; requestAnimationFrame(update); } };
  addEventListener("scroll", ask, { passive: true });
  addEventListener("resize", ask);
  update();
  rail.classList.add("live");
})();
`;

// railScript answers GET /changelog/rail.js.
export function railScript() {
  return new Response(RAIL_JS, { headers: { "content-type": "text/javascript; charset=utf-8", "cache-control": "public, max-age=3600", "x-content-type-options": "nosniff" } });
}

// schemeScript answers GET /changelog/scheme.js.
export function schemeScript() {
  return new Response(SCHEME_JS, { headers: { "content-type": "text/javascript; charset=utf-8", "cache-control": "public, max-age=3600", "x-content-type-options": "nosniff" } });
}

// shortDate is "Nov 11, 2026" for "2026-11-11" and "Nov 09, 2026" for "2026-11-09" (the day always has two digits, so the
// dates in the rail stay as long as one another), read as a day, not an instant.
function shortDate(d) {
  const t = new Date(d + "T12:00:00Z");
  if (isNaN(t)) return esc(d);
  return t.toLocaleDateString("en-US", { year: "numeric", month: "short", day: "2-digit", timeZone: "UTC" });
}

// dayDate is "Nov 09" for "2026-11-09": a row of the rail has no year, the year is written once above its rows.
function dayDate(d) {
  const t = new Date(d + "T12:00:00Z");
  if (isNaN(t)) return esc(d);
  return t.toLocaleDateString("en-US", { month: "short", day: "2-digit", timeZone: "UTC" });
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

// railHTML is the rail of releases: a row for each, its version and its day, under the year of the rows below it.
function railHTML(releases) {
  let year = "";
  const rows = releases.map((r) => {
    const v = esc(r.version.replace(/^v/, ""));
    const y = esc(String(r.date).slice(0, 4));
    const head = y !== year ? `<li class="year" aria-hidden="true">${y}</li>` : ""; // the year, once above the rows that have it
    year = y;
    return `${head}<li><a class="bar" href="#${esc(r.version)}" title="Version ${v} on ${shortDate(r.date)}"><span class="lbl"><b>${v}</b><time datetime="${esc(r.date)}">${dayDate(r.date)}</time></span></a></li>`;
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
<meta name="description" content="What is new in each version of Celestine.">${ogTags("Changelog — Celestine", "What is new in each version of Celestine.", "/changelog")}<meta name="color-scheme" content="light dark"><script src="/changelog/scheme.js?v=${SCHEME_JS.length.toString(36)}"></script><script src="/changelog/rail.js?v=${RAIL_JS.length.toString(36)}" defer></script><style>
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
  --control:linear-gradient(180deg,light-dark(oklch(100% 0 0),oklch(35% .003 286)),light-dark(oklch(94.5% .005 286),oklch(28% .004 286)));
  --control-border:light-dark(oklch(0% 0 0 / 10%),oklch(100% 0 0 / 8%));
  --control-shadow:inset 0 1px light-dark(oklch(100% 0 0 / 50%),oklch(100% 0 0 / 8%)),0 1px 2px light-dark(oklch(0% 0 0 / 6%),oklch(0% 0 0 / 40%));
  --font:-apple-system,BlinkMacSystemFont,"SF Pro Text","Helvetica Neue",Inter,system-ui,sans-serif;
  --mono:ui-monospace,"SF Mono",SFMono-Regular,Menlo,Consolas,"Liberation Mono",monospace}
html[data-scheme=light]{color-scheme:light}
html[data-scheme=dark]{color-scheme:dark}
*{box-sizing:border-box}
html{scroll-behavior:smooth;scroll-padding-top:72px}
html,body{margin:0;background:var(--bg)}
body{color:var(--text);font:15px/1.6 var(--font);-webkit-font-smoothing:antialiased}
::selection{background:color-mix(in oklab,var(--accent) 28%,transparent)}
a{color:inherit}
.wrap{max-width:64rem;margin:0 auto;padding:0 16px}
/* the top bar, the entry screen's own (landing/App.svelte): the name, the changelog, "Early access" and the toggle */
.topbar{position:sticky;top:0;z-index:5;border-bottom:1px solid var(--line);background:color-mix(in oklab,var(--bg) 82%,transparent);backdrop-filter:saturate(1.4) blur(12px);-webkit-backdrop-filter:saturate(1.4) blur(12px)}
.topbar .inner{width:min(1180px,100%);margin:0 auto;display:flex;align-items:center;justify-content:space-between;height:56px;padding:0 24px}
.brand{display:inline-flex;align-items:center;gap:10px;font-weight:600;font-size:15px;letter-spacing:-.01em;color:var(--text);text-decoration:none}
.brand img{width:24px;height:24px;display:block;transform:scale(1.2)}
.right{display:flex;align-items:center;gap:14px}
.status{font-size:13px;color:var(--muted)}
.link{font-size:13.5px;color:var(--text);text-decoration:none;padding:6px 10px;border-radius:999px;transition:background-color .2s}
.link:hover{background:var(--well)}
.icon{width:32px;height:32px;display:grid;place-items:center;padding:0;border-radius:999px;color:var(--text);background:var(--control);border:1px solid var(--control-border);box-shadow:var(--control-shadow);cursor:pointer;transition:filter .2s,transform .15s var(--ease)}
.icon[hidden]{display:none}
.icon:hover{filter:brightness(1.06)}.icon:active{transform:translateY(1px)}
.icon .moon{display:none}
html[data-scheme=dark] .icon .moon{display:block}html[data-scheme=dark] .icon .sun{display:none}
@media (prefers-color-scheme:dark){html:not([data-scheme=light]) .icon .moon{display:block}html:not([data-scheme=light]) .icon .sun{display:none}}
.banner{position:relative;margin:16px 0 0;height:300px;border-radius:22px;overflow:hidden;background:#f7f4f1 url(/changelog/banner.webp?v=${BANNER_V}) center 54%/cover no-repeat;
  -webkit-mask-image:linear-gradient(#000 80%,transparent);mask-image:linear-gradient(#000 80%,transparent)}
.intro{position:absolute;left:32px;top:30px;max-width:24rem;display:flex;flex-direction:column;gap:8px}
.banner h1{margin:0;color:oklch(24% .03 265);font-size:clamp(1.9rem,4.6vw,2.75rem);line-height:1.05;letter-spacing:-.035em}
.banner p{margin:0;color:oklch(34% .03 265);font-size:15.5px;line-height:1.45;max-width:21rem;text-wrap:balance}
/* the night banner: in the dark scheme, chosen with the toggle or the system's */
html[data-scheme=dark] .banner{background-color:#0b1026;background-image:url(/changelog/banner-dark.webp?v=${BANNER_DARK_V})}
html[data-scheme=dark] .banner::before{content:"";position:absolute;inset:0;background:linear-gradient(100deg,oklch(12% .05 270 / 70%) 0%,oklch(12% .05 270 / 35%) 32%,transparent 55%)}
html[data-scheme=dark] .intro{z-index:1}
html[data-scheme=dark] .banner h1{color:oklch(98% .01 255);text-shadow:0 0 24px oklch(70% .15 255 / 55%),0 1px 2px oklch(10% .05 270 / 60%)}
html[data-scheme=dark] .banner p{color:oklch(90% .03 260);text-shadow:0 1px 10px oklch(10% .05 270 / 80%)}
@media (prefers-color-scheme:dark){
  html:not([data-scheme=light]) .banner{background-color:#0b1026;background-image:url(/changelog/banner-dark.webp?v=${BANNER_DARK_V})}
  html:not([data-scheme=light]) .banner::before{content:"";position:absolute;inset:0;background:linear-gradient(100deg,oklch(12% .05 270 / 70%) 0%,oklch(12% .05 270 / 35%) 32%,transparent 55%)}
  html:not([data-scheme=light]) .intro{z-index:1}
  html:not([data-scheme=light]) .banner h1{color:oklch(98% .01 255);text-shadow:0 0 24px oklch(70% .15 255 / 55%),0 1px 2px oklch(10% .05 270 / 60%)}
  html:not([data-scheme=light]) .banner p{color:oklch(90% .03 260);text-shadow:0 1px 10px oklch(10% .05 270 / 80%)}
}
.page{display:grid;grid-template-columns:max-content 1fr;column-gap:56px;margin-top:8px}
.rail{position:sticky;top:72px;align-self:start;padding:34px 0 24px}
.rail ol{list-style:none;margin:0;padding:0;display:flex;flex-direction:column;gap:14px}
/* the rail that follows the page (rail.js): the release being read is bright and pushes the rows around it away, the rest
   dim the further they are */
.rail li{transition:opacity .35s var(--ease),margin .4s var(--ease)}
.rail.live li:not(.year){opacity:max(.28,calc(.62 - (var(--d) - 1) * .12))}
.rail.live li.on{opacity:1;margin-block:12px}
.rail.live li.on time{color:var(--text)}
.rail .year{font:600 13px/1 var(--mono);font-variant-numeric:tabular-nums;color:var(--muted);text-box-trim:trim-both;text-box-edge:ex alphabetic}
.bar{display:block;text-decoration:none;color:var(--muted)}
.lbl{display:flex;align-items:center;gap:12px;white-space:nowrap;font-family:var(--mono);font-variant-numeric:tabular-nums}
/* the labels' boxes are trimmed to the x-height and the baseline, so they sit centered on the bar, not off by the line's leading */
.lbl b,.lbl time{text-box-trim:trim-both;text-box-edge:ex alphabetic}
.lbl b{font-size:13px;font-weight:600;color:var(--text);min-width:5ch;text-align:right}
.lbl time{font-size:13px;color:var(--muted)}
.bar:hover .lbl b{color:var(--accent)}
main{min-width:0}
/* the light and dark switch (scheme.js) is a view transition: the old page fades into the new one */
::view-transition-group(root){animation-duration:.45s;animation-timing-function:cubic-bezier(.4,0,.2,1)}
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
@media (prefers-reduced-motion:reduce){html{scroll-behavior:auto}.rail li{transition:none}}
@media (max-width:480px){.status{display:none}}
@media (max-width:760px){
  .topbar .inner{padding:0 16px}
  .banner{height:200px;border-radius:18px;background-position:88% 55%}.intro{left:20px;top:22px;max-width:13.5rem}.banner p{font-size:14px}
  .page{grid-template-columns:1fr}
  .rail{position:static;padding:18px 0 4px;overflow-x:auto}
  .rail ol{flex-direction:row;align-items:center;gap:18px}
  .rail .year{display:none}
  .rail.live li.on{margin-block:0}
  .lbl b{min-width:0;text-align:center}.lbl time{display:none}
  .release{padding:24px 0}
}
</style></head><body>
<nav class="topbar"><div class="inner"><a class="brand" href="/" aria-label="Celestine"><img src="/icon.png" alt="" width="24" height="24">Celestine</a>
<div class="right"><a class="link" href="/changelog" aria-current="page">Changelog</a><span class="status">Early access</span>
<button class="icon" type="button" aria-label="Switch scheme" hidden><svg class="moon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true"><path d="M20 14.5A8 8 0 0 1 9.5 4a8 8 0 1 0 10.5 10.5Z"/></svg><svg class="sun" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true"><circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"/></svg></button></div></div></nav>
<div class="wrap">
<div class="banner" role="img" aria-label="A watercolor landscape at dawn, two blue crystals standing in a meadow"><div class="intro"><h1>Changelog</h1><p>What came with each version of Celestine. The app updates itself.</p></div></div>
<div class="page">${releases.length ? railHTML(releases) : "<div></div>"}<main>
${list}
</main></div>
<footer><a href="/">Celestine</a><a href="/privacy">Privacy</a></footer>
</div></body></html>`;
}
