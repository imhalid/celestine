// The service worker of the web version: the program (html, js, wasm, icons) is kept in the browser's cache
// on the first visit, so that the app opens with no network and starts at once the second time; the page is
// asked of the network first, so that a new build is taken as soon as there is one. Nothing
// else is fetched or kept: the library is in localStorage, not here. A new build has a new wasm name, and
// with it a new cache: the old one is dropped.
const CACHE = "ryudit-" + (self.registration ? new URL(self.location).searchParams.get("v") || "1" : "1");
// what the app is made of, kept at once when the worker comes: the page opens with no network even if
// it was never asked for again (the wasm's name is the build's, given to the worker as ?v=)
const PROGRAM = ["./", "wasm_exec.js", "manifest.webmanifest", "icon-256.png", "icon-512.png"];
self.addEventListener("install", (e) => {
  const wasm = new URL(self.location).searchParams.get("v");
  const files = wasm ? PROGRAM.concat([wasm]) : PROGRAM;
  e.waitUntil(caches.open(CACHE).then((c) => c.addAll(files)).catch(() => {}).then(() => self.skipWaiting()));
});
self.addEventListener("activate", (e) => {
  e.waitUntil(caches.keys().then((ks) => Promise.all(ks.filter((k) => k !== CACHE && k.startsWith("ryudit-")).map((k) => caches.delete(k)))).then(() => self.clients.claim()));
});
self.addEventListener("fetch", (e) => {
  const r = e.request;
  const url = new URL(r.url);
  if (r.method !== "GET" || url.origin !== self.location.origin) return;
  // the page itself from the network first, the cache only when there is none: it names the wasm of the
  // build, and a page kept in the cache would keep the old build forever. The rest (the wasm, named for its
  // build, the script, the icons) from the cache first.
  const page = r.mode === "navigate" || url.pathname.endsWith("/") || url.pathname.endsWith(".html");
  e.respondWith(
    caches.open(CACHE).then(async (c) => {
      if (page) {
        try {
          const res = await fetch(r);
          if (res.ok) c.put(r, res.clone());
          return res;
        } catch (err) {
          const hit = await c.match(r);
          if (hit) return hit;
          throw err;
        }
      }
      const hit = await c.match(r);
      if (hit) return hit;
      const res = await fetch(r);
      if (res.ok) c.put(r, res.clone());
      return res;
    }).catch(() => caches.match(r))
  );
});
