// The entry screen ("/"), in the manner of ui.halid.dev: the name being typed and one line on what it is, the download,
// below, the app's carets each in a card writing little notes, and the waitlist of sync in the footer. Light or dark as the system has it, or
// as the toggle in its bar says. With reduced motion asked for, nothing moves.

import { LANDING_JS, LANDING_CSS } from "./landing.gen.js";
// the hero's picture, two WebPs made with an image model in the changelog banner's watercolor: at dawn for the light
// scheme, at night for the dark one (App.svelte draws it from --hero and --hero-dark)
import HERO from "./hero.bin";
import HERO_DARK from "./hero-dark.bin";

// heroV is a picture's address suffix: it changes when the picture does, so no browser keeps the old one for its day.
const heroV = (b) => (b.byteLength || b.length || 0).toString(36);

// heroImage answers GET /hero.webp and /hero-dark.webp. A new picture comes with a deploy.
export function heroImage(dark) {
  return new Response(dark ? HERO_DARK : HERO, {
    headers: { "content-type": "image/webp", "cache-control": "public, max-age=86400", "x-content-type-options": "nosniff" },
  });
}

// The page itself is a small Svelte app (landing/: App, SignupForm, EmailField, CaretCard, writers.js), built by landing/build.mjs into
// landing.gen.js and put inline here: the CSP allows only this page's nonce'd script.
export function landingHTML(n, site, contact, download, esc) {
  return `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover">
<title>Celestine</title><link rel="icon" href="/icon.png"><link rel="apple-touch-icon" href="/icon.png"><meta name="description" content="Celestine, a one-of-a-kind notes app: free to download, no account needed."><meta name="color-scheme" content="light dark">
<style>:root{--hero:url(/hero.webp?v=${heroV(HERO)});--hero-dark:url(/hero-dark.webp?v=${heroV(HERO_DARK)})}${LANDING_CSS}</style></head><body>
<div id="app" data-contact="${contact}" data-download="${esc(download)}"></div>
<noscript>JavaScript is needed for this page.</noscript>
${site ? `<div class="cf-turnstile" data-sitekey="${esc(site)}" data-appearance="interaction-only"></div><script src="https://challenges.cloudflare.com/turnstile/v0/api.js" async defer></script>` : ""}
<script nonce="${n}">${LANDING_JS}</script></body></html>`;
}
