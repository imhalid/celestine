#!/bin/sh
# Makes dist/celestine-macos.dmg, the first install on macOS, from the zips scripts/package.sh wrote
# (dist/celestine-darwin-arm64.zip and -amd64.zip): one Celestine.app for Apple Silicon and Intel, beside a
# shortcut to /Applications to drag it onto. The zips stay as they are; the app's update reads them.
#
#   scripts/dmg.sh                (macOS only: lipo, codesign and hdiutil)
#
# The two programs become one universal binary, and the bundle is signed ad hoc as a whole, as the Go linker
# signs the arm64 program alone. The app keeps the update address and key the zips' programs were built with,
# so an app installed from the DMG updates itself. Without a Developer ID and notarization, Gatekeeper still
# says the app cannot be verified when the DMG comes from a browser (#479).
set -eu
cd "$(dirname "$0")/.."
[ "$(uname)" = Darwin ] || { echo "scripts/dmg.sh runs on macOS"; exit 1; }
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

bins=""
for z in dist/celestine-darwin-*.zip; do
	[ -f "$z" ] || continue
	arch=$(basename "$z" .zip)
	arch=${arch##*-}
	mkdir -p "$work/$arch"
	unzip -q "$z" -d "$work/$arch"
	bins="$bins $work/$arch/celestine.app/Contents/MacOS/celestine"
	src=$work/$arch/celestine.app
done
[ -n "$bins" ] || { echo "no dist/celestine-darwin-*.zip: run scripts/build.sh and scripts/package.sh first"; exit 1; }

# the bundle of either zip (they differ only in the program), named as Finder shows it
mkdir -p "$work/disk"
app=$work/disk/Celestine.app
cp -R "$src" "$app"
# shellcheck disable=SC2086 # one path per architecture
lipo -create $bins -output "$app/Contents/MacOS/celestine"
chmod 0755 "$app/Contents/MacOS/celestine"
codesign --force --deep -s - "$app"
codesign --verify --deep --strict "$app"
ln -s /Applications "$work/disk/Applications"

out=dist/celestine-macos.dmg
rm -f "$out"
hdiutil create -quiet -volname Celestine -srcfolder "$work/disk" -fs HFS+ -format UDZO -imagekey zlib-level=9 "$out"
hdiutil verify -quiet "$out"
echo "$out ($(lipo -archs "$app/Contents/MacOS/celestine"), $(( $(wc -c < "$out") / 1024 )) KB)"
