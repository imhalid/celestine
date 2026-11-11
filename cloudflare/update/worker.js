// The server of updates: a Cloudflare Worker over an R2 bucket.
//
//   GET /update.json          the manifest of the latest release (channel=beta for trials)
//   GET /<version>/<file>     the release's files
//   GET /download/<os-arch>   the latest release's file for darwin-arm64, windows-amd64, …: a redirect, for the site's buttons;
//                             macOS's is the zipped celestine.app, Linux's the .tar.gz beside the binary when the release has one
//
// The release's workflow (.github/workflows/release.yml) puts the files and update.json in the bucket.
// The Worker adds nothing and logs nothing of the user: the request is a version in the User-Agent, and
// Cloudflare's own counters say how many downloads a file had.
export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    if (request.method !== "GET" && request.method !== "HEAD") return new Response("no", { status: 405 });
    const dl = url.pathname.match(/^\/download\/([a-z0-9]+-[a-z0-9]+)$/);
    if (dl) return download(env, dl[1]);
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

// download sends the browser to the latest release's file for a platform, as update.json names it. The
// manifest names the bare Linux binary (what the update takes); a person gets the .tar.gz put beside it
// (scripts/package.sh), with the .desktop file and the icon, when that release has one.
async function download(env, platform) {
  const obj = await env.RELEASES.get("update.json");
  if (!obj) return new Response("no release yet", { status: 404 });
  const manifest = await obj.json();
  const asset = manifest.assets?.[platform];
  if (!asset?.url) return new Response("no build for " + platform, { status: 404 });
  let location = asset.url;
  if (platform.startsWith("linux-") && (await env.RELEASES.head(`${manifest.version}/celestine-${platform}.tar.gz`)))
    location += ".tar.gz";
  return new Response(null, { status: 302, headers: { location, "cache-control": "public, max-age=300" } });
}
