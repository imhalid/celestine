// The server of synchronization: a Cloudflare Worker over an R2 bucket. It is the protocol of sync.go's
// httpRemote, and it keeps files, not meaning: with end-to-end encryption on, all it holds is opaque
// names and sealed bytes.
//
//   GET    /files          { "<name>": { "hash": "<sha256 of the bytes>", "deleted": false }, … }
//   GET    /files/<name>   the bytes
//   PUT    /files/<name>   the bytes; If-Match: "<hash seen>" ("" when there was none); 412 when it is stale
//   DELETE /files/<name>   If-Match as above; a tombstone is left
//
// Every device has a token, "Authorization: Bearer <token>"; env.TOKENS is a JSON object { "<token>": "<account>" }
// (a Worker secret), and an account's files are under its own prefix. The check of If-Match is made
// by R2 itself (onlyIf), so that two devices writing at once cannot both win.
const enc = new TextEncoder();
async function sha256(buf) {
  const d = await crypto.subtle.digest("SHA-256", buf);
  return [...new Uint8Array(d)].map((b) => b.toString(16).padStart(2, "0")).join("");
}
// The web build (a browser) speaks to this server too, so every answer says who may read it. The token
// is what protects the files, not the origin: "*" is safe because no cookie is ever used.
const CORS = {
  "access-control-allow-origin": "*",
  "access-control-allow-methods": "GET, PUT, DELETE, OPTIONS",
  "access-control-allow-headers": "authorization, if-match, content-type",
  "access-control-max-age": "86400",
};
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
  {
    const auth = (request.headers.get("authorization") || "").replace(/^Bearer\s+/i, "");
    const account = JSON.parse(env.TOKENS || "{}")[auth];
    if (!auth || !account) return new Response("no", { status: 401 });
    const url = new URL(request.url);
    const m = url.pathname.match(/^\/files(?:\/(.+))?$/);
    if (!m) return new Response("not found", { status: 404 });
    const prefix = `${account}/`;
    const name = m[1] ? decodeURIComponent(m[1]) : "";
    if (name.includes("..") || name.startsWith("/")) return new Response("bad name", { status: 400 });
    const bucket = env.SYNC;

    if (!name) {
      const out = {};
      let cursor;
      do {
        const page = await bucket.list({ prefix, cursor, include: ["customMetadata"] });
        for (const o of page.objects) {
          const n = o.key.slice(prefix.length);
          if (n.endsWith(".deleted~")) out[n.slice(0, -9)] ??= { deleted: true };
          else out[n] = { hash: o.customMetadata?.hash || "" };
        }
        cursor = page.truncated ? page.cursor : undefined;
      } while (cursor);
      return Response.json(out);
    }
    const key = prefix + name;
    if (request.method === "GET") {
      const o = await bucket.get(key);
      return o ? new Response(o.body) : new Response("not found", { status: 404 });
    }
    const head = await bucket.head(key);
    const cur = head?.customMetadata?.hash || "";
    const ifMatch = (request.headers.get("if-match") || "").replace(/"/g, "");
    if (ifMatch !== cur) return new Response("stale", { status: 412 });
    const only = head ? { etagMatches: head.etag } : { etagDoesNotMatch: "*" };
    if (request.method === "PUT") {
      const body = await request.arrayBuffer();
      const o = await bucket.put(key, body, { onlyIf: only, customMetadata: { hash: await sha256(body) } });
      if (!o) return new Response("stale", { status: 412 });
      await bucket.delete(key + ".deleted~");
      return new Response("ok");
    }
    if (request.method === "DELETE") {
      await bucket.delete(key);
      await bucket.put(key + ".deleted~", enc.encode(""), { customMetadata: { hash: "" } });
      return new Response("ok");
    }
    return new Response("no", { status: 405 });
  }
}
