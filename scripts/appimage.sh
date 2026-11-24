#!/bin/sh
# Makes an AppImage of each Linux build in dist/ (scripts/build.sh): dist/celestine-linux-<arch>.AppImage, one
# file a person downloads, marks executable and runs, on any distribution. The bare binary and the .tar.gz
# (scripts/package.sh) stay beside it. The manifest lists it apart, under linux-<arch>-appimage (scripts/release),
# so that a program run from an AppImage updates by replacing the AppImage, not the program in its read-only
# mount (update.go: updatePlatform, #513).
#
#   scripts/appimage.sh           (Linux on x86-64: appimagetool is an x86-64 program; it packs arm64 too)
#
# appimagetool and the runtime each AppImage starts with are pinned by version and hash and downloaded into a
# folder of their own, never run from the working tree; appimagetool runs extracted, without FUSE.
set -eu
cd "$(dirname "$0")/.."
[ "$(uname -s)-$(uname -m)" = Linux-x86_64 ] || { echo "scripts/appimage.sh runs on Linux x86-64"; exit 1; }

TOOL_URL=https://github.com/AppImage/appimagetool/releases/download/1.9.1/appimagetool-x86_64.AppImage
TOOL_SHA=ed4ce84f0d9caff66f50bcca6ff6f35aae54ce8135408b3fa33abfc3cb384eb0
RUNTIME_URL=https://github.com/AppImage/type2-runtime/releases/download/20251108/runtime
RUNTIME_SHA_x86_64=2fca8b443c92510f1483a883f60061ad09b46b978b2631c807cd873a47ec260d
RUNTIME_SHA_aarch64=00cbdfcf917cc6c0ff6d3347d59e0ca1f7f45a6df1a428a0d6d8a78664d87444

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/tools"

# fetch URL FILE SHA puts a pinned file in the tools folder, or stops when its hash is not the one written here.
fetch() {
	curl -fsSL --retry 3 -o "$work/tools/$2" "$1"
	echo "$3  $work/tools/$2" | sha256sum -c --quiet - || { echo "$2: not the pinned file"; exit 1; }
	chmod 0755 "$work/tools/$2"
}
fetch "$TOOL_URL" appimagetool "$TOOL_SHA"

made=0
for bin in dist/celestine-linux-*; do
	case $bin in *.tar.gz | *.AppImage) continue ;; esac
	[ -f "$bin" ] || continue
	arch=${bin##*-}
	case $arch in
	amd64) aarch=x86_64; sha=$RUNTIME_SHA_x86_64 ;;
	arm64) aarch=aarch64; sha=$RUNTIME_SHA_aarch64 ;;
	*) echo "$bin: no AppImage runtime for $arch"; exit 1 ;;
	esac
	fetch "$RUNTIME_URL-$aarch" "runtime-$aarch" "$sha"

	dir=$work/$arch/Celestine.AppDir
	mkdir -p "$dir/usr/bin" "$dir/usr/share/applications" "$dir/usr/share/icons/hicolor/256x256/apps"
	install -m 0755 "$bin" "$dir/usr/bin/celestine"
	install -m 0644 assets/icon-256.png "$dir/usr/share/icons/hicolor/256x256/apps/celestine.png"
	mkdir -p "$dir/usr/share/doc/celestine" # the licences, the app's and the third-party software's (#256)
	install -m 0644 LICENSE "$dir/usr/share/doc/celestine/LICENSE"
	if [ -f dist/THIRD_PARTY_NOTICES.md ]; then install -m 0644 dist/THIRD_PARTY_NOTICES.md "$dir/usr/share/doc/celestine/"; fi
	cat > "$dir/usr/share/applications/celestine.desktop" <<EOF
[Desktop Entry]
Type=Application
Name=Celestine
Comment=Notes
Exec=celestine %F
Icon=celestine
Terminal=false
Categories=Office;TextEditor;
StartupWMClass=celestine
EOF
	chmod 0644 "$dir/usr/share/applications/celestine.desktop"
	# what the AppImage's runtime and appimagetool look for at the top: the launcher, the .desktop file, the icon
	ln -s usr/bin/celestine "$dir/AppRun"
	ln -s usr/share/applications/celestine.desktop "$dir/celestine.desktop"
	ln -s usr/share/icons/hicolor/256x256/apps/celestine.png "$dir/celestine.png"
	ln -s celestine.png "$dir/.DirIcon"

	out=$bin.AppImage
	rm -f "$out"
	ARCH=$aarch APPIMAGE_EXTRACT_AND_RUN=1 "$work/tools/appimagetool" --no-appstream \
		--runtime-file "$work/tools/runtime-$aarch" "$dir" "$out" > "$work/$arch.log" 2>&1 ||
		{ cat "$work/$arch.log"; exit 1; }
	chmod 0755 "$out"
	echo "$out ($(( $(wc -c < "$out") / 1024 )) KB)"
	made=$((made + 1))
done
[ "$made" -gt 0 ] || { echo "no dist/celestine-linux-*: run scripts/build.sh first"; exit 1; }
