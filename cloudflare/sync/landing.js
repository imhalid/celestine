// The entry screen ("/"), in the manner of ui.halid.dev: the name being typed and one line on what it is, the waitlist
// in a well, and below, the app's carets each in a card writing little notes. Light or dark as the system has it, or
// as the toggle in its bar says. With reduced motion asked for, nothing moves.

import { LANDING_JS, LANDING_CSS } from "./landing.gen.js";

// The page itself is a small Svelte app (landing/: App, SignupForm, EmailField, CaretCard, writers.js), built by landing/build.mjs into
// landing.gen.js and put inline here: the CSP allows only this page's nonce'd script.
export function landingHTML(n, site, contact, esc) {
  return `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover">
<title>Ryudit</title><meta name="description" content="Ryudit is in early access: join the waitlist."><meta name="color-scheme" content="light dark">
<style>${LANDING_CSS}</style></head><body>
<div id="app" data-contact="${contact}"></div>
<noscript>JavaScript is needed to join the waitlist.</noscript>
${site ? `<div class="cf-turnstile" data-sitekey="${esc(site)}" data-appearance="interaction-only"></div><script src="https://challenges.cloudflare.com/turnstile/v0/api.js" async defer></script>` : ""}
<script nonce="${n}">${LANDING_JS}</script></body></html>`;
}
