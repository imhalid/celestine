#!/bin/sh
# Builds celestine for the five targets, as small as it goes without leaving a feature out, and writes
# the sizes of each next to the last build's, with a word when a size goes over the budget.
#
#   scripts/build.sh              all targets, into dist/
#   BUDGET_MB=40 scripts/build.sh the budget of a binary, in MB (default 50, as the release)
#   scripts/build.sh linux/amd64  one target
#   LDFLAGS='-X main.updateURL=…' scripts/build.sh   more -ldflags, as the release puts its update address and key in
#
# The flags, each measured (docs/17-boyut-ve-performans.md): -s -w takes the symbols and the DWARF off
# (a third of the file), -trimpath and -buildvcs=false take the paths and the version control's stamp.
# SMALL=1 also turns inlining off (-gcflags=all=-l): 7% smaller (23,5 → 21,9 MB), and slower in the hot loops of layout and
# painting, so it is not the default. UPX is not used: it breaks macOS signing and notarization and Windows virus checkers dislike it.
set -eu
cd "$(dirname "$0")/.."
BUDGET_MB=${BUDGET_MB:-50}
VERSION=${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}
TARGETS=${*:-"darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64"}
mkdir -p dist
prev=dist/sizes.txt
[ -f "$prev" ] && cp "$prev" dist/sizes.prev.txt || : > dist/sizes.prev.txt
: > "$prev"
status=0
for t in $TARGETS; do
	os=${t%/*}; arch=${t#*/}
	ext=""; [ "$os" = windows ] && ext=".exe"
	out="dist/celestine-$os-$arch$ext"
	CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath -buildvcs=false ${SMALL:+-gcflags=all=-l} \
		-ldflags="-s -w -X main.version=$VERSION ${LDFLAGS:-}" -o "$out" .
	bytes=$(wc -c < "$out" | tr -d ' ')
	echo "$os/$arch $bytes" >> "$prev"
	old=$(grep "^$os/$arch " dist/sizes.prev.txt | cut -d' ' -f2 || true)
	diff=""
	if [ -n "${old:-}" ]; then diff=" ($(( (bytes - old) / 1024 )) KB since the last build)"; fi
	mb=$(( bytes / 1048576 ))
	line="$os/$arch: $(( bytes / 1024 )) KB$diff"
	if [ "$mb" -ge "$BUDGET_MB" ]; then line="$line  ** over the budget of ${BUDGET_MB} MB **"; status=1; fi
	echo "$line"
done
exit $status
