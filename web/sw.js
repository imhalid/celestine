// The service worker of the web version: the program (html, js, wasm, icons) is kept in the browser's cache
// on the first visit, so that the app opens with no network and starts at once the second time. Nothing
// else is fetched or kept: the library is in localStorage, not here. A new build has a new wasm name, and
// with it a new cache: the old one is dropped.
const CACHE = "ryudit-" + (self.registration ? new URL(self.location).searchParams.get("v") || "1" : "1");
self.addEventListener("install", (e) => { self.skipWaiting(); });
self.addEventListener("activate", (e) => {
  e.waitUntil(caches.keys().then((ks) => Promise.all(ks.filter((k) => k !== CACHE && k.startsWith("ryudit-")).map((k) => caches.delete(k)))).then(() => self.clients.claim()));
});
self.addEventListener("fetch", (e) => {
  const r = e.request;
  if (r.method !== "GET" || new URL(r.url).origin !== self.location.origin) return;
  e.respondWith(
    caches.open(CACHE).then(async (c) => {
      const hit = await c.match(r);
      if (hit) return hit;
      const res = await fetch(r);
      if (res.ok) c.put(r, res.clone());
      return res;
    }).catch(() => caches.match(r))
  );
});
