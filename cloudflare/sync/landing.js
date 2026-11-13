// The entry screen ("/"), in the manner of ui.halid.dev: the name being typed and one line on what it is, the download,
// below, the app's carets each in a card writing little notes, and the waitlist of sync in the footer. Light or dark as the system has it, or
// as the toggle in its bar says. With reduced motion asked for, nothing moves.

import { LANDING_JS, LANDING_CSS } from "./landing.gen.js";
import { ogTags } from "./og.js";
// the hero's picture, two WebPs made with an image model in the changelog banner's watercolor: at dawn for the light
// scheme, at night for the dark one (App.svelte draws it from --hero and --hero-dark)
import HERO from "./hero.bin";
import HERO_DARK from "./hero-dark.bin";
// the footer's picture, a still lake with the crystals on its far shore, the same way (--foot and --foot-dark)
import FOOT from "./foot.bin";
import FOOT_DARK from "./foot-dark.bin";

// heroV is a picture's address suffix: it changes when the picture does, so no browser keeps the old one for its day.
const heroV = (b) => (b.byteLength || b.length || 0).toString(36);

// PICTURES are the entry screen's pictures by their address. A new picture comes with a deploy.
const PICTURES = { "/hero.webp": HERO, "/hero-dark.webp": HERO_DARK, "/foot.webp": FOOT, "/foot-dark.webp": FOOT_DARK };

// landingImage answers GET of one of PICTURES, or gives null for another address.
export function landingImage(path) {
  const b = PICTURES[path];
  if (!b) return null;
  return new Response(b, {
    headers: { "content-type": "image/webp", "cache-control": "public, max-age=86400", "x-content-type-options": "nosniff" },
  });
}

// schemeHead is the script in the head that runs before anything is drawn: the page is dark unless the visitor chose light
// with the toggle (the system's scheme is not asked), and the hero picture of that scheme is preloaded at once, so it is
// being fetched while the rest of the page is still being read. (The other pictures are fetched when the page is idle: main.js.)
const schemeHead = (v, vDark) => `(function(){var d=document.documentElement,s="dark";try{if(localStorage.getItem("celestine-scheme")==="light")s="light"}catch(e){}d.dataset.scheme=s;var l=document.createElement("link");l.rel="preload";l.as="image";l.setAttribute("fetchpriority","high");l.href=s==="light"?"/hero.webp?v=${v}":"/hero-dark.webp?v=${vDark}";document.head.appendChild(l)})()`;

// The page itself is a small Svelte app (landing/: App, SignupForm, EmailField, CaretCard, writers.js), built by landing/build.mjs into
// landing.gen.js and put inline here: the CSP allows only this page's nonce'd script.
export function landingHTML(n, site, contact, download, esc) {
  return `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover">
<title>Celestine</title><link rel="icon" href="/icon.png"><link rel="apple-touch-icon" href="/icon.png"><meta name="description" content="Celestine, a one-of-a-kind notes app: free to download, no account needed."><meta name="color-scheme" content="light dark">${ogTags("Celestine", "A one-of-a-kind notes app, built with MyGo. Free to download, no account needed.", "/")}
<script nonce="${n}">${schemeHead(heroV(HERO), heroV(HERO_DARK))}</script>
<style>:root{--hero:url(/hero.webp?v=${heroV(HERO)});--hero-dark:url(/hero-dark.webp?v=${heroV(HERO_DARK)});--foot:url(/foot.webp?v=${heroV(FOOT)});--foot-dark:url(/foot-dark.webp?v=${heroV(FOOT_DARK)})}${LANDING_CSS}</style></head><body>
<div id="app" data-contact="${contact}" data-download="${esc(download)}"></div>
<noscript>JavaScript is needed for this page.</noscript>
${site ? `<div class="cf-turnstile" data-sitekey="${esc(site)}" data-appearance="interaction-only" data-execution="execute"></div><script src="https://challenges.cloudflare.com/turnstile/v0/api.js" async defer></script>` : ""}
<script nonce="${n}">${LANDING_JS}</script></body></html>`;
}
