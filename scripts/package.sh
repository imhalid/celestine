#!/bin/sh
# Packages the builds in dist/ (release.yml, scripts/build.sh) the way a person installs them:
#
#   dist/celestine-darwin-<arch>      → dist/celestine-darwin-<arch>.zip: celestine.app at its top, what the app's
#                                    update unpacks (update.go: installApp, kind app-zip); the bare binary goes
#   dist/celestine-linux-<arch>       → also dist/celestine-linux-<arch>.tar.gz: the binary, a .desktop file and the
#                                    icon, for the GitHub release; the bare binary stays, the update reads it
#
#   VERSION=v1.2.3 scripts/package.sh
#
# The manifest (scripts/release) is made from what is left: the zips for macOS, the bare binaries for Linux and
# Windows; it leaves the .tar.gz out.
set -eu
cd "$(dirname "$0")/.."
VERSION=${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}
# CFBundleShortVersionString wants numbers: v1.2.3-beta.1 is 1.2.3
short=$(echo "$VERSION" | sed -e 's/^v//' -e 's/[-+].*//')
echo "$short" | grep -Eq '^[0-9]+(\.[0-9]+){0,2}$' || short=0.0.0
root=$(pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

# notices puts the licences of the third-party software in the package, with the app's own (#256):
# dist/THIRD_PARTY_NOTICES.md, which the release makes (go run ./scripts/notices), when it is there
notices() {
	install -m 0644 LICENSE "$1/LICENSE"
	if [ -f dist/THIRD_PARTY_NOTICES.md ]; then install -m 0644 dist/THIRD_PARTY_NOTICES.md "$1/THIRD_PARTY_NOTICES.md"; fi
}

for bin in dist/celestine-darwin-*; do
	case $bin in *.zip) continue ;; esac
	[ -f "$bin" ] || continue
	arch=${bin##*-}
	app=$work/darwin-$arch/celestine.app
	mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
	install -m 0755 "$bin" "$app/Contents/MacOS/celestine"
	install -m 0644 assets/icon.icns "$app/Contents/Resources/icon.icns"
	notices "$app/Contents/Resources"
	cat > "$app/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleExecutable</key><string>celestine</string>
	<key>CFBundleIdentifier</key><string>dev.halid.celestine</string>
	<key>CFBundleName</key><string>Celestine</string>
	<key>CFBundleDisplayName</key><string>Celestine</string>
	<key>CFBundleIconFile</key><string>icon</string>
	<key>CFBundlePackageType</key><string>APPL</string>
	<key>CFBundleShortVersionString</key><string>$short</string>
	<key>CFBundleVersion</key><string>$short</string>
	<key>CFBundleInfoDictionaryVersion</key><string>6.0</string>
	<key>LSMinimumSystemVersion</key><string>12.0</string>
	<key>NSHighResolutionCapable</key><true/>
</dict>
</plist>
EOF
	chmod 0644 "$app/Contents/Info.plist"
	rm -f "$bin.zip"
	# -X leaves out the extra fields of times and owners; the Unix modes are kept (the program's 0755)
	(cd "$work/darwin-$arch" && zip -qrX "$root/$bin.zip" celestine.app)
	rm "$bin"
	echo "$bin.zip"
done

for bin in dist/celestine-linux-*; do
	case $bin in *.tar.gz) continue ;; esac
	[ -f "$bin" ] || continue
	name=$(basename "$bin")
	dir=$work/$name
	mkdir -p "$dir"
	install -m 0755 "$bin" "$dir/celestine"
	install -m 0644 assets/icon-256.png "$dir/celestine.png"
	notices "$dir"
	cat > "$dir/celestine.desktop" <<EOF
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
	chmod 0644 "$dir/celestine.desktop"
	cat > "$dir/README" <<EOF
Celestine $VERSION

Run ./celestine, or install it for yourself:
  install -Dm755 celestine ~/.local/bin/celestine
  install -Dm644 celestine.png ~/.local/share/icons/hicolor/256x256/apps/celestine.png
  install -Dm644 celestine.desktop ~/.local/share/applications/celestine.desktop
EOF
	chmod 0644 "$dir/README"
	tar -C "$work" --owner=0 --group=0 --numeric-owner -czf "$bin.tar.gz" "$name"
	echo "$bin.tar.gz"
done
