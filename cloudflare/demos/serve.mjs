// a small static server for the demos: node serve.mjs [port]
import { createServer } from "node:http";
import { readFileSync, existsSync } from "node:fs";
import { extname, join } from "node:path";
const types = { ".html": "text/html; charset=utf-8", ".js": "text/javascript; charset=utf-8" };
createServer((req, res) => {
  let p = new URL(req.url, "http://x").pathname; if (p === "/") p = "/index.html";
  const f = join("dist", p);
  if (!f.startsWith("dist") || !existsSync(f)) { res.writeHead(404); return res.end("not found"); }
  res.writeHead(200, { "content-type": types[extname(f)] || "text/plain", "cache-control": "no-store" }); res.end(readFileSync(f));
}).listen(Number(process.argv[2]) || 8800, () => console.log("demos on http://localhost:" + (process.argv[2] || 8800)));
