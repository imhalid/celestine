// The server of synchronization: a Cloudflare Worker over an R2 bucket. It is the protocol of sync.go's
// httpRemote, and it keeps files, not meaning: with end-to-end encryption on, all it holds is opaque
// names and sealed bytes.
//
//   GET    /files          { "<name>": { "hash": "<sha256 of the bytes>", "deleted": false }, … }
//   GET    /files/<name>   the bytes
//   PUT    /files/<name>   the bytes; If-Match: "<hash seen>" ("" when there was none); 412 when it is stale
//   DELETE /files/<name>   If-Match as above; the file becomes a tombstone
//
// Every device has a token, "Authorization: Bearer <token>"; env.TOKENS is a JSON object { "<token>": "<account>" }
// (a Worker secret), and an account's files are under its own prefix. Every write is checked by R2
// itself (onlyIf on the object's etag), so that two devices writing at once cannot both win.
//
// A deletion is not a second object: the file's own object is replaced by an empty one marked
// deleted ("tomb" in its metadata), by the same conditional write as any other change. A tombstone older
// than TOMB_DAYS is dropped when the files are listed: a device that was away longer than that has, for
// the files it deletes or keeps, the same outcome as with the tombstone.
const MAX_BYTES = 50 * 1024 * 1024; // a file larger than this is refused (413)
const TOMB_DAYS = 180;
const CORS = {
  "access-control-allow-origin": "*",
  "access-control-allow-methods": "GET, PUT, DELETE, OPTIONS",
  "access-control-allow-headers": "authorization, if-match, content-type",
  "access-control-max-age": "86400",
};
async function sha256(buf) {
  const d = await crypto.subtle.digest("SHA-256", buf);
  return [...new Uint8Array(d)].map((b) => b.toString(16).padStart(2, "0")).join("");
}
// a token is looked for without stopping at the first byte that differs
function same(a, b) {
  if (a.length !== b.length) return false;
  let d = 0;
  for (let i = 0; i < a.length; i++) d |= a.charCodeAt(i) ^ b.charCodeAt(i);
  return d === 0;
}
function accountOf(env, token) {
  if (!token) return "";
  let found = "";
  for (const [t, acc] of Object.entries(JSON.parse(env.TOKENS || "{}"))) if (same(t, token)) found = acc;
  return found;
}
const dead = (o) => o?.customMetadata?.tomb === "1";

export default {
  async fetch(request, env) {
    if (request.method === "OPTIONS") return new Response(null, { status: 204, headers: CORS });
    const res = await handle(request, env);
    const out = new Response(res.body, res);
    for (const [k, v] of Object.entries(CORS)) out.headers.set(k, v);
    return out;
  },
};

async function handle(request, env) {
  const auth = (request.headers.get("authorization") || "").replace(/^Bearer\s+/i, "");
  const account = accountOf(env, auth);
  if (!account) return new Response("no", { status: 401 });
  const url = new URL(request.url);
  const m = url.pathname.match(/^\/files(?:\/(.+))?$/);
  if (!m) return new Response("not found", { status: 404 });
  const prefix = `${account}/`;
  const name = m[1] ? decodeURIComponent(m[1]) : "";
  if (name.split("/").includes("..") || name.startsWith("/")) return new Response("bad name", { status: 400 });
  const bucket = env.SYNC;

  if (!name) {
    const out = {};
    const old = Date.now() - TOMB_DAYS * 864e5;
    let cursor;
    do {
      const page = await bucket.list({ prefix, cursor, include: ["customMetadata"] });
      for (const o of page.objects) {
        const n = o.key.slice(prefix.length);
        if (dead(o)) {
          if (o.uploaded.getTime() < old) await bucket.delete(o.key);
          else out[n] = { deleted: true };
        } else out[n] = { hash: o.customMetadata?.hash || "" };
      }
      cursor = page.truncated ? page.cursor : undefined;
    } while (cursor);
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
  if (request.method === "PUT") {
    const body = await request.arrayBuffer();
    if (body.byteLength > MAX_BYTES) return new Response("too large", { status: 413 });
    const o = await bucket.put(key, body, { onlyIf: only, customMetadata: { hash: await sha256(body) } });
    return o ? new Response("ok") : new Response("stale", { status: 412 });
  }
  if (!head || dead(head)) return new Response("ok"); // already gone
  const o = await bucket.put(key, new Uint8Array(0), { onlyIf: only, customMetadata: { tomb: "1" } });
  return o ? new Response("ok") : new Response("stale", { status: 412 });
}
