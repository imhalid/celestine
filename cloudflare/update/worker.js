// The server of updates: a Cloudflare Worker over an R2 bucket.
//
//   GET /update.json          the manifest of the latest release (channel=beta for trials)
//   GET /<version>/<file>     the release's files
//
// The release's workflow (.github/workflows/release.yml) puts the files and update.json in the bucket.
// The Worker adds nothing and logs nothing of the user: the request is a version in the User-Agent, and
// Cloudflare's own counters say how many downloads a file had.
export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    if (request.method !== "GET" && request.method !== "HEAD") return new Response("no", { status: 405 });
    let key = url.pathname.replace(/^\/+/, "");
    if (key === "update.json") key = url.searchParams.get("channel") === "beta" ? "beta.json" : "update.json";
    if (!/^[A-Za-z0-9._\/-]+$/.test(key) || key.includes("..")) return new Response("not found", { status: 404 });
    const obj = await env.RELEASES.get(key);
    if (!obj) return new Response("not found", { status: 404 });
    const headers = new Headers();
    obj.writeHttpMetadata(headers);
    headers.set("etag", obj.httpEtag);
    headers.set("cache-control", key.endsWith(".json") ? "public, max-age=300" : "public, max-age=31536000, immutable");
    return new Response(request.method === "HEAD" ? null : obj.body, { headers });
  },
};
