// The server of Celestine: a Cloudflare Worker over an R2 bucket (the files) and a D1 database (the accounts).
//
//   /files…  synchronization: it keeps files, not meaning; with end-to-end encryption on, all it holds is opaque
//            names and sealed bytes. The protocol is that of sync.go's httpRemote:
//   GET    /files          { "<name>": { "hash": "<sha256 of the bytes>", "deleted": false }, … }
//   GET    /files/<name>   the bytes
//   PUT    /files/<name>   the bytes; If-Match: "<hash seen>" ("" when there was none); 412 when it is stale
//   DELETE /files/<name>   If-Match as above; the file becomes a tombstone
//   GET    /changes        { "changes": <n> }: the account's change counter, bumped by every PUT and DELETE; a device asks
//                          for it often (it is one small read) and lists the files only when it moved
//   /api/…   accounts: waitlist, registration with an invite code, login, devices (auth.js)
//   /        the waitlist page;  /privacy  what is kept;  /admin  the administrator's page (pages.js)
//   /changelog  every release, from the releases bucket's changelog.json (changelog.js)
//
// Every device has a token, "Authorization: Bearer <token>"; it leads to an account, and an account's files are
// under its own prefix ("u<id>/"). Every write is checked by R2 itself (onlyIf on the object's etag), so that two
// devices writing at once cannot both win.
//
// A deletion is not a second object: the file's own object is replaced by an empty one marked
// deleted ("tomb" in its metadata), by the same conditional write as any other change. A tombstone older
// than TOMB_DAYS is dropped when the files are listed: a device that was away longer than that has, for
// the files it deletes or keeps, the same outcome as with the tombstone.
import { handleApi, userOfToken } from "./auth.js";
import { waitlistPage, adminPage, privacyPage } from "./pages.js";
import { changelogPage, bannerImage } from "./changelog.js";
import { landingImage } from "./landing.js";
import { ADMIN_JS, ADMIN_JS_PATH, ADMIN_WASM_PATH } from "./admin.gen.js";
import ADMIN_WASM from "./admin.bin";
import ICON from "./icon.bin"; // the app's icon (assets/icon-256.png), the pages' favicon and mark

const MAX_BYTES = 50 * 1024 * 1024; // a file larger than this is refused (413)
const TOMB_DAYS = 180;
// The earlier versions of a file: when a file is written over (or deleted), what it was is kept under "v<account>/<name>@<when>",
// at most one in VERSION_GAP ms, VERSIONS of them for a file, VERSION_DAYS days, and only of a file up to VERSION_MAX bytes (a page,
// not a picture). They count in the account's quota like the files. They are sealed bytes as the files are: the server cannot read them.
const VERSIONS = 10;
const VERSION_GAP = 10 * 60 * 1000;
const VERSION_DAYS = 30;
const VERSION_MAX = 1024 * 1024;
const vkey = (id, name, at) => `v${id}/${name}@${String(at).padStart(13, "0")}`;
const CORS = {
  "access-control-allow-origin": "*",
  "access-control-allow-methods": "GET, PUT, POST, DELETE, OPTIONS",
  "access-control-allow-headers": "authorization, if-match, content-type",
  "access-control-max-age": "86400",
};
async function sha256(buf) {
  const d = await crypto.subtle.digest("SHA-256", buf);
  return [...new Uint8Array(d)].map((b) => b.toString(16).padStart(2, "0")).join("");
}
const dead = (o) => o?.customMetadata?.tomb === "1";

export default {
  // The daily pass: the tombstones older than TOMB_DAYS are deleted, in the thousand at a time that R2 deletes. They were
  // deleted while listing, which a long list of them made slow, and took the subrequests of the request.
  async scheduled(event, env) {
    const old = Date.now() - TOMB_DAYS * 864e5;
    let cursor;
    let gone = 0;
    do {
      const page = await env.SYNC.list({ cursor, include: ["customMetadata"] });
      const keys = page.objects.filter((o) => dead(o) && o.uploaded.getTime() < old).map((o) => o.key);
      if (keys.length) await env.SYNC.delete(keys);
      gone += keys.length;
      cursor = page.truncated ? page.cursor : undefined;
    } while (cursor);
    console.log(`tombstones deleted: ${gone}`);
    // the versions older than VERSION_DAYS; what they weighed goes back to their owners' quota
    const oldV = Date.now() - VERSION_DAYS * 864e5;
    const refund = new Map();
    cursor = undefined;
    do {
      const page = await env.SYNC.list({ prefix: "v", cursor });
      const old = page.objects.filter((o) => o.uploaded.getTime() < oldV);
      if (old.length) await env.SYNC.delete(old.map((o) => o.key));
      for (const o of old) {
        const id = o.key.match(/^v(\d+)\//)?.[1];
        if (id) refund.set(id, (refund.get(id) || 0) + o.size);
      }
      cursor = page.truncated ? page.cursor : undefined;
    } while (cursor);
    for (const [id, n] of refund) await env.DB.prepare("UPDATE users SET bytes=MAX(0,bytes-?) WHERE id=?").bind(n, Number(id)).run();
    console.log(`versions deleted for ${refund.size} accounts`);
  },
  async fetch(request, env, ctx) {
    if (request.method === "OPTIONS") return new Response(null, { status: 204, headers: CORS });
    let res;
    try {
      res = await handle(request, env, ctx);
    } catch (e) {
      console.error(e);
      res = new Response("error", { status: 500 });
    }
    const out = new Response(res.body, res);
    for (const [k, v] of Object.entries(CORS)) if (!out.headers.has(k)) out.headers.set(k, v);
    return out;
  },
};

async function handle(request, env, ctx) {
  const url = new URL(request.url);
  if (request.method === "GET" && url.pathname === "/") return waitlistPage(env);
  if (request.method === "GET" && url.pathname === "/admin") return adminPage();
  // the admin panel's module and the WebAssembly it draws with (admin/build.mjs): nothing secret, the key stays the page's
  if (request.method === "GET" && url.pathname === "/admin/" + ADMIN_JS_PATH) return asset(ADMIN_JS, "text/javascript; charset=utf-8");
  if (request.method === "GET" && url.pathname === "/admin/" + ADMIN_WASM_PATH) return asset(ADMIN_WASM, "application/wasm");
  if (request.method === "GET" && url.pathname === "/icon.png") return new Response(ICON, { headers: { "content-type": "image/png", "cache-control": "public, max-age=86400", "x-content-type-options": "nosniff" } });
  if (request.method === "GET" && url.pathname === "/privacy") return privacyPage(env);
  if (request.method === "GET" && /^\/(hero|foot)(-dark)?\.webp$/.test(url.pathname)) return landingImage(url.pathname);
  if (request.method === "GET" && url.pathname === "/changelog") return changelogPage(request, env, ctx);
  if (request.method === "GET" && url.pathname === "/changelog/banner.webp") return bannerImage(false);
  if (request.method === "GET" && url.pathname === "/changelog/banner-dark.webp") return bannerImage(true);
  if (url.pathname.startsWith("/api/")) return (await handleApi(request, env, url)) || new Response("not found", { status: 404 });
  if (/^\/versions\//.test(url.pathname)) return versions(request, env, url);
  if (url.pathname === "/changes") return changes(request, env);
  if (!/^\/files(\/|$)/.test(url.pathname)) return new Response("not found", { status: 404 });
  return files(request, env, url);
}

// Puts what the file is, before it is written over, among its versions (see VERSIONS). It never fails the write: a version that cannot be kept
// (the quota is full, the file is big, the last one is recent) is only not kept. It returns when the file's last version was kept, as
// the file's own metadata keeps it ("kept"): a write within VERSION_GAP of it, as most are while one types, is told so by the file
// it writes over, without listing the versions.
async function keepVersion(env, user, name, head, key) {
  const was = head.customMetadata?.kept || "";
  if (name.startsWith("_") || head.size === 0 || head.size > VERSION_MAX) return was; // _index and _salt are not pages
  const at = head.uploaded.getTime(); // when what is being kept was written
  const gap = Number(env.VERSION_GAP_MS ?? VERSION_GAP); // (the variable is for trying it: a test cannot wait ten minutes)
  if (was && at - Number(was) < gap) return was;
  const prefix = `v${user.id}/${name}@`;
  const mine = (await env.SYNC.list({ prefix })).objects.filter((o) => o.key.length === prefix.length + 13 && /^\d{13}$/.test(o.key.slice(prefix.length)));
  mine.sort((a, b) => (a.key < b.key ? -1 : 1));
  const last = mine.length ? Number(mine[mine.length - 1].key.slice(prefix.length)) : 0;
  if (last && at - last < gap) return String(last);
  if (mine.some((o) => o.key === vkey(user.id, name, at))) return String(at);
  const r = await env.DB.prepare("UPDATE users SET bytes=bytes+?1 WHERE id=?2 AND bytes+?1<=quota").bind(head.size, user.id).run();
  if (r.meta.changes !== 1) return was;
  const old = await env.SYNC.get(key);
  if (!old) {
    await env.DB.prepare("UPDATE users SET bytes=MAX(0,bytes-?) WHERE id=?").bind(head.size, user.id).run();
    return was;
  }
  await env.SYNC.put(vkey(user.id, name, at), old.body);
  const extra = mine.slice(0, Math.max(0, mine.length + 1 - VERSIONS));
  if (extra.length) {
    await env.SYNC.delete(extra.map((o) => o.key));
    await env.DB.prepare("UPDATE users SET bytes=MAX(0,bytes-?) WHERE id=?").bind(extra.reduce((n, o) => n + o.size, 0), user.id).run();
  }
  return String(at);
}

// GET /versions/<name>: the versions kept of a file, newest first ([{at, size}]); with ?at=<when>, the bytes of one.
async function versions(request, env, url) {
  if (request.method !== "GET") return new Response("no", { status: 405 });
  const token = (request.headers.get("authorization") || "").replace(/^Bearer\s+/i, "");
  const user = await userOfToken(env, token);
  if (!user) return new Response("no", { status: 401 });
  const name = decodeURIComponent(url.pathname.slice("/versions/".length));
  if (!name || name.split("/").includes("..") || name.startsWith("/") || name.includes("@")) return new Response("bad name", { status: 400 });
  const at = url.searchParams.get("at");
  if (at !== null) {
    if (!/^\d{13}$/.test(at)) return new Response("bad request", { status: 400 });
    const o = await env.SYNC.get(vkey(user.id, name, at));
    return o ? new Response(o.body) : new Response("not found", { status: 404 });
  }
  const prefix = `v${user.id}/${name}@`;
  const list = (await env.SYNC.list({ prefix })).objects
    .filter((o) => o.key.length === prefix.length + 13 && /^\d{13}$/.test(o.key.slice(prefix.length)))
    .map((o) => ({ at: Number(o.key.slice(prefix.length)), size: o.size }));
  list.sort((a, b) => b.at - a.at);
  return Response.json(list);
}

// GET /changes: the account's change counter (see files()).
async function changes(request, env) {
  if (request.method !== "GET") return new Response("no", { status: 405 });
  const token = (request.headers.get("authorization") || "").replace(/^Bearer\s+/i, "");
  const user = await userOfToken(env, token);
  if (!user) return new Response("no", { status: 401 });
  const row = await env.DB.prepare("SELECT changes FROM users WHERE id=?").bind(user.id).first();
  return Response.json({ changes: row?.changes || 0 }, { headers: { "cache-control": "no-store" } });
}

async function files(request, env, url) {
  const token = (request.headers.get("authorization") || "").replace(/^Bearer\s+/i, "");
  const user = await userOfToken(env, token);
  if (!user) return new Response("no", { status: 401 });
  const prefix = `u${user.id}/`;
  const m = url.pathname.match(/^\/files(?:\/(.+))?$/);
  const name = m[1] ? decodeURIComponent(m[1]) : "";
  if (name.split("/").includes("..") || name.startsWith("/")) return new Response("bad name", { status: 400 });
  const bucket = env.SYNC;

  if (!name) {
    const out = {};
    const old = Date.now() - TOMB_DAYS * 864e5;
    const expired = [];
    let cursor;
    do {
      const page = await bucket.list({ prefix, cursor, include: ["customMetadata"] });
      for (const o of page.objects) {
        const n = o.key.slice(prefix.length);
        if (dead(o)) {
          if (o.uploaded.getTime() >= old) out[n] = { deleted: true };
          else expired.push(o.key); // left out of the list, and deleted below
        } else out[n] = { hash: o.customMetadata?.hash || "" };
      }
      cursor = page.truncated ? page.cursor : undefined;
    } while (cursor);
    // the expired tombstones go in one call, at most a thousand (R2's limit for it): not one request each, which a long list of
    // them made slow and over the limit of subrequests. The rest go with the next listing, and the daily pass (scheduled).
    if (expired.length) await bucket.delete(expired.slice(0, 1000));
    return Response.json(out);
  }
  const key = prefix + name;
  if (request.method === "GET") {
    const o = await bucket.get(key);
    return o && !dead(o) ? new Response(o.body) : new Response("not found", { status: 404 });
  }
  if (request.method !== "PUT" && request.method !== "DELETE") return new Response("no", { status: 405 });
  if (request.method === "PUT" && Number(request.headers.get("content-length") || 0) > MAX_BYTES)
    return new Response("too large", { status: 413 });

  const head = await bucket.head(key);
  const cur = head && !dead(head) ? head.customMetadata?.hash || "" : ""; // a tombstone is "not there"
  const ifMatch = (request.headers.get("if-match") || "").replace(/"/g, "");
  if (ifMatch !== cur) return new Response("stale", { status: 412 });
  const only = head ? { etagMatches: head.etag } : { etagDoesNotMatch: "*" };
  const oldSize = head && !dead(head) ? head.size : 0;
  const bytes = (n) => (user && n ? env.DB.prepare("UPDATE users SET bytes=MAX(0,bytes+?) WHERE id=?").bind(n, user.id).run() : null);
  // a write done: the account's change counter moves, and the other devices, asking /changes, sync
  const bump = () => (user ? env.DB.prepare("UPDATE users SET changes=changes+1 WHERE id=?").bind(user.id).run() : null);
  if (request.method === "PUT") {
    const body = await request.arrayBuffer();
    if (body.byteLength > MAX_BYTES) return new Response("too large", { status: 413 });
    const delta = body.byteLength - oldSize;
    // what the write adds is taken from the quota first, in one statement that checks it, so two writes at once cannot both
    // pass; it is given back if the write does not happen
    if (delta > 0) {
      const r = await env.DB.prepare("UPDATE users SET bytes=bytes+?1 WHERE id=?2 AND bytes+?1<=quota").bind(delta, user.id).run();
      if (r.meta.changes !== 1) return new Response("quota", { status: 413 });
    }
    const kept = head && !dead(head) ? await keepVersion(env, user, name, head, key) : "";
    const meta = { hash: await sha256(body) };
    if (kept) meta.kept = kept;
    let o;
    try {
      o = await bucket.put(key, body, { onlyIf: only, customMetadata: meta });
    } catch (e) {
      if (delta > 0) await bytes(-delta);
      throw e;
    }
    if (!o) {
      if (delta > 0) await bytes(-delta);
      return new Response("stale", { status: 412 });
    }
    if (delta < 0) await bytes(delta);
    await bump();
    return new Response("ok");
  }
  if (!head || dead(head)) return new Response("ok"); // already gone
  await keepVersion(env, user, name, head, key);
  const o = await bucket.put(key, new Uint8Array(0), { onlyIf: only, customMetadata: { tomb: "1" } });
  if (!o) return new Response("stale", { status: 412 });
  await bytes(-oldSize);
  await bump();
  return new Response("ok");
}

// asset is a file of the admin panel: its name says what is in it (admin/build.mjs), so it is cached for good.
function asset(body, type) {
  return new Response(body, { headers: { "content-type": type, "cache-control": "public, max-age=31536000, immutable", "x-content-type-options": "nosniff" } });
}
