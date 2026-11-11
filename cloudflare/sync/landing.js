// The entry screen ("/"), in the manner of ui.halid.dev: the name being typed and one line on what it is, the download,
// below, the app's carets each in a card writing little notes, and the waitlist of sync in the footer. Light or dark as the system has it, or
// as the toggle in its bar says. With reduced motion asked for, nothing moves.

import { LANDING_JS, LANDING_CSS } from "./landing.gen.js";

// The page itself is a small Svelte app (landing/: App, SignupForm, EmailField, CaretCard, writers.js), built by landing/build.mjs into
// landing.gen.js and put inline here: the CSP allows only this page's nonce'd script.
export function landingHTML(n, site, contact, download, esc) {
  return `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover">
<title>Celestine</title><link rel="icon" href="/icon.png"><link rel="apple-touch-icon" href="/icon.png"><meta name="description" content="Celestine, a one-of-a-kind notes app: free to download, no account needed."><meta name="color-scheme" content="light dark">
<style>${LANDING_CSS}</style></head><body>
<div id="app" data-contact="${contact}" data-download="${esc(download)}"></div>
<noscript>JavaScript is needed for this page.</noscript>
${site ? `<div class="cf-turnstile" data-sitekey="${esc(site)}" data-appearance="interaction-only"></div><script src="https://challenges.cloudflare.com/turnstile/v0/api.js" async defer></script>` : ""}
<script nonce="${n}">${LANDING_JS}</script></body></html>`;
}
