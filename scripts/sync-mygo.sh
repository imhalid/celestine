#!/bin/sh
# Brings celestine to the newest MyGo: the fork's celestine branch (MyGo and the few patches the app needs, each also a
# pull request to MyGo) is put on top of MyGo's newest release or main, pushed, and celestine's go.mod points at it.
# Then the gate runs. Nothing is committed: look, then commit go.mod and go.sum.
#
#   scripts/sync-mygo.sh            onto MyGo's newest release tag
#   scripts/sync-mygo.sh main       onto MyGo's main
#   MYGO=../mygo scripts/sync-mygo.sh   where the fork is checked out (default ../mygo)
#
# A patch that MyGo has taken in comes out of the rebase empty and is dropped; when none is left, the app needs
# no fork: the replace goes, and go.mod requires MyGo itself. A conflict stops here: the rebase is left open in
# the fork to be resolved by hand (git rebase --continue), and the script run again.
set -eu
cd "$(dirname "$0")/.."
MYGO=${MYGO:-../mygo}
[ -d "$MYGO/.git" ] || { echo "no fork at $MYGO: git clone git@github.com:imhalid/mygo.git $MYGO"; exit 1; }
git -C "$MYGO" remote get-url upstream >/dev/null 2>&1 || git -C "$MYGO" remote add upstream https://github.com/egoist/mygo.git
git -C "$MYGO" fetch -q upstream --tags
git -C "$MYGO" fetch -q origin celestine
if [ "${1:-}" = main ]; then
	onto=upstream/main
else
	onto=$(git -C "$MYGO" describe --tags --abbrev=0 --match 'v*' upstream/main)
fi
echo "putting the patches on $onto"
git -C "$MYGO" checkout -q celestine
# a patch committed here and not pushed would be lost to the reset: stop and say so instead (#261)
if [ -n "$(git -C "$MYGO" log --oneline origin/celestine..celestine)" ]; then
	echo "celestine in $MYGO has commits origin/celestine does not; push or drop them first:"
	git -C "$MYGO" log --oneline origin/celestine..celestine
	exit 1
fi
git -C "$MYGO" reset -q --hard origin/celestine
git -C "$MYGO" rebase --empty=drop "$onto"
git -C "$MYGO" log --oneline "$onto"..celestine
commit=$(git -C "$MYGO" rev-parse HEAD)
export GOWORK=off
if [ "$commit" = "$(git -C "$MYGO" rev-parse "$onto")" ]; then
	echo "no patch is left: MyGo itself, no fork"
	go mod edit -dropreplace=github.com/egoist/mygo
	go get github.com/egoist/mygo@"$commit"
else
	git -C "$MYGO" push -q --force-with-lease origin celestine
	version=$(git -C "$MYGO" describe --tags --abbrev=0 "$onto" 2>/dev/null || echo "")
	case "$version" in
	v*) go mod edit -require=github.com/egoist/mygo@"$version" ;;
	esac
	go mod edit -replace=github.com/egoist/mygo=github.com/imhalid/mygo@"$commit"
fi
go mod tidy
echo "gate"
test -z "$(gofmt -l .)" || { gofmt -l .; exit 1; }
go vet ./...
go test ./...
for os in darwin linux windows; do CGO_ENABLED=0 GOOS=$os go build -o /dev/null .; done
GOOS=js GOARCH=wasm go build -o /dev/null .
echo "celestine builds on MyGo $onto with its patches: commit go.mod and go.sum"
