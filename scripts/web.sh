#!/bin/sh
# Builds the browser version into dist/web (index.html, the wasm, wasm_exec.js, and what makes it an app that
# can be installed and opens offline: manifest, service worker, icons) and serves it on :8765.
#   scripts/web.sh          build and serve
#   scripts/web.sh build    build only
# The library lives in the browser's localStorage (store_js.go); see docs/18-web.md.
set -eu
cd "$(dirname "$0")/.."
GOFLAGS=-mod=mod go run github.com/egoist/mygo/cmd/mygo build -platform web
cp web/manifest.webmanifest web/sw.js web/icon-256.png web/icon-512.png dist/web/
WASM=$(ls dist/web/*.wasm | head -1 | xargs basename)
# the page says it is an app, and registers the worker; the cache is named for the wasm, so a new build is a new cache
python3 - "$WASM" <<'PY'
import sys,re
w=sys.argv[1]
p='dist/web/index.html'
s=open(p).read()
if 'manifest.webmanifest' not in s:
    s=s.replace('</head>','<link rel="manifest" href="manifest.webmanifest"><link rel="icon" href="icon-256.png"><meta name="theme-color" content="#17171a">\n</head>',1)
    s=s.replace('</body>','<script>if("serviceWorker"in navigator&&location.protocol!=="file:"){navigator.serviceWorker.register("sw.js?v='+w+'").catch(function(){})}</script>\n</body>',1)
    open(p,'w').write(s)
PY
[ "${1:-}" = build ] && exit 0
echo "http://localhost:8765/"
cd dist/web && exec python3 -m http.server 8765
