#!/bin/sh
# Packages the builds in dist/ (release.yml, scripts/build.sh) the way a person installs them:
#
#   dist/ryudit-darwin-<arch>      → dist/ryudit-darwin-<arch>.zip: ryudit.app at its top, what the app's
#                                    update unpacks (update.go: installApp, kind app-zip); the bare binary goes
#   dist/ryudit-linux-<arch>       → also dist/ryudit-linux-<arch>.tar.gz: the binary, a .desktop file and the
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

for bin in dist/ryudit-darwin-*; do
	case $bin in *.zip) continue ;; esac
	[ -f "$bin" ] || continue
	arch=${bin##*-}
	app=$work/darwin-$arch/ryudit.app
	mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
	install -m 0755 "$bin" "$app/Contents/MacOS/ryudit"
	install -m 0644 assets/icon.icns "$app/Contents/Resources/icon.icns"
	cat > "$app/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleExecutable</key><string>ryudit</string>
	<key>CFBundleIdentifier</key><string>dev.halid.ryudit</string>
	<key>CFBundleName</key><string>Ryudit</string>
	<key>CFBundleDisplayName</key><string>Ryudit</string>
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
	(cd "$work/darwin-$arch" && zip -qrX "$root/$bin.zip" ryudit.app)
	rm "$bin"
	echo "$bin.zip"
done

for bin in dist/ryudit-linux-*; do
	case $bin in *.tar.gz) continue ;; esac
	[ -f "$bin" ] || continue
	name=$(basename "$bin")
	dir=$work/$name
	mkdir -p "$dir"
	install -m 0755 "$bin" "$dir/ryudit"
	install -m 0644 assets/icon-256.png "$dir/ryudit.png"
	cat > "$dir/ryudit.desktop" <<EOF
[Desktop Entry]
Type=Application
Name=Ryudit
Comment=Notes
Exec=ryudit %F
Icon=ryudit
Terminal=false
Categories=Office;TextEditor;
StartupWMClass=ryudit
EOF
	chmod 0644 "$dir/ryudit.desktop"
	cat > "$dir/README" <<EOF
Ryudit $VERSION

Run ./ryudit, or install it for yourself:
  install -Dm755 ryudit ~/.local/bin/ryudit
  install -Dm644 ryudit.png ~/.local/share/icons/hicolor/256x256/apps/ryudit.png
  install -Dm644 ryudit.desktop ~/.local/share/applications/ryudit.desktop
EOF
	chmod 0644 "$dir/README"
	tar -C "$work" --owner=0 --group=0 --numeric-owner -czf "$bin.tar.gz" "$name"
	echo "$bin.tar.gz"
done
