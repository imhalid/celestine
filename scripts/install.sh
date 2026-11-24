#!/bin/sh
# Installs the latest Celestine on macOS or Linux:
#
#   curl -fsSL https://celestine.halid.dev/install.sh | sh
#
# It reads the latest version from the update server (dl.celestine.halid.dev/update.json), downloads the build
# for this computer, checks it against the release's SHA256SUMS, and puts it in place:
#
#   macOS   Celestine.app in /Applications, or ~/Applications when that is not writable
#   Linux   the program in ~/.local/bin, with its icon and .desktop file, so it shows among the applications
#
# SHA256SUMS itself is checked first against its signature, made with the key the app checks its updates with
# (scripts/release sums): whoever can write to the server cannot install through this script what the app's own
# updater would refuse (#540). The check is `ssh-keygen -Y verify` (OpenSSH 8.1 or later), which every Mac has and
# most Linux systems too: the stock openssl of macOS cannot check an ed25519 signature. Where no ssh-keygen can, the
# script stops, unless CELESTINE_UNVERIFIED=1 says to install on the checksums alone.
#
# A file curl downloads carries no quarantine mark, so macOS opens the app without Gatekeeper's warning about an
# unverified developer, which a DMG from a browser meets until the app is signed with a Developer ID (#514). Both
# installs update themselves afterwards (update.go). The release workflow puts this file on R2, and the site serves
# it at /install.sh.
#
#   CELESTINE_DL=https://…   another update server (a test one)
#   CELESTINE_KEY="ssh-ed25519 AAAA…"   the key of that server's releases
#   CELESTINE_DIR=…          another folder for the app (macOS) or the program (Linux)
#   CELESTINE_UNVERIFIED=1   install where no ssh-keygen can check the signature, on the checksums alone
#   NO_COLOR=1               no colors
#
# Everything is in functions and runs from the last line, so a download cut short runs nothing.
set -eu

DL=${CELESTINE_DL:-https://dl.celestine.halid.dev}
# KEY is the public key of the releases as ssh-keygen reads it: the release workflow writes it into the copy it
# puts on the server (go run ./scripts/release sshkey); the one in the repository has none.
KEY=${CELESTINE_KEY:-@RELEASE_SSH_KEY@}
# FLOOR is the version of the release that wrote this script: one older is not installed, so that update.json, which
# is not signed, cannot point the script at an older signed release with its fixed holes (#564). The copy in the
# repository has none.
FLOOR=@RELEASE_VERSION@

# ---- the look ----

# paint sets the colors when the output is a terminal that takes them: truecolor when it says so, else the
# 256-color palette's nearest blues.
paint() {
	B="" D="" R="" TRUE=""
	if [ -t 1 ] && [ -z "${NO_COLOR:-}" ] && [ "${TERM:-dumb}" != dumb ]; then
		B=$(printf '\033[1m') D=$(printf '\033[2m') R=$(printf '\033[0m')
		case ${COLORTERM:-} in truecolor | 24bit) TRUE=1 ;; esac
	fi
}

# blue R G B N256 prints the escape for a blue, by its RGB or its 256-color number.
blue() {
	[ -n "$R" ] || return 0
	if [ -n "$TRUE" ]; then printf '\033[38;2;%s;%s;%sm' "$1" "$2" "$3"; else printf '\033[38;5;%sm' "$4"; fi
}

# logo draws the app's icon, two blue crystals, the right one taller, lighter at the top as in assets/icon.svg.
logo() {
	echo
	printf '            %s▄████▄%s\n' "$(blue 178 212 250 153)" "$R"
	printf '   %s▄████▄%s   %s██████%s\n' "$(blue 166 203 247 153)" "$R" "$(blue 160 199 246 153)" "$R"
	printf '   %s██████%s   %s██████%s\n' "$(blue 140 186 243 111)" "$R" "$(blue 140 186 243 111)" "$R"
	printf '   %s██████%s   %s██████%s     %sCelestine%s\n' "$(blue 118 171 238 111)" "$R" "$(blue 118 171 238 111)" "$R" "$B" "$R"
	printf '   %s██████%s   %s██████%s     %sa one-of-a-kind notes app%s\n' "$(blue 98 155 233 75)" "$R" "$(blue 98 155 233 75)" "$R" "$D" "$R"
	printf '   %s██████%s   %s██████%s\n' "$(blue 80 140 228 68)" "$R" "$(blue 80 140 228 68)" "$R"
	printf '   %s▀████▀%s   %s▀████▀%s\n' "$(blue 64 126 222 68)" "$R" "$(blue 64 126 222 68)" "$R"
	echo
}

warn() { printf '  %s!%s %s\n' "$B" "$R" "$1" >&2; }
step() { printf '  %s→%s %s\n' "$(blue 118 171 238 111)" "$R" "$1"; }
done_() { printf '  %s✓%s %s\n' "$(blue 98 155 233 75)" "$R" "$1"; }
fail() {
	printf '  %s✗ %s%s\n' "$B" "$1" "$R" >&2
	exit 1
}

# ---- the work ----

# fetch URL [FILE] downloads with curl, or wget when there is no curl: to FILE, or to the output.
fetch() {
	if command -v curl >/dev/null 2>&1; then
		if [ $# -gt 1 ]; then curl -fsSL --retry 3 -o "$2" "$1"; else curl -fsSL --retry 3 "$1"; fi
	elif command -v wget >/dev/null 2>&1; then
		if [ $# -gt 1 ]; then wget -q -O "$2" "$1"; else wget -q -O - "$1"; fi
	else
		fail "curl or wget is needed"
	fi
}

# sha256 FILE prints the file's SHA-256.
sha256() {
	if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | cut -d' ' -f1; else shasum -a 256 "$1" | cut -d' ' -f1; fi
}

# platform sets OS and ARCH as the releases name them (darwin/linux, arm64/amd64). A shell under Rosetta says
# x86_64 on an Apple silicon Mac, which gets the arm64 build.
platform() {
	case $(uname -s) in
	Darwin) OS=darwin ;;
	Linux) OS=linux ;;
	MINGW* | MSYS* | CYGWIN*) fail "on Windows, download Celestine from https://celestine.halid.dev" ;;
	*) fail "Celestine is made for macOS, Linux and Windows, not $(uname -s)" ;;
	esac
	case $(uname -m) in
	arm64 | aarch64) ARCH=arm64 ;;
	x86_64 | amd64) ARCH=amd64 ;;
	*) fail "no build for $(uname -m)" ;;
	esac
	if [ "$OS" = darwin ] && [ "$(sysctl -n sysctl.proc_translated 2>/dev/null || echo 0)" = 1 ]; then ARCH=arm64; fi
}

# verify_sums checks the release's SHA256SUMS against its signature, SHA256SUMS.sig, with KEY. The signed text is
# a line naming the format, the version and the channel, then the file (scripts/release/sums.go), so the sums of
# another version, or of a trial, do not pass for this one's.
verify_sums() {
	case $KEY in "ssh-ed25519 "*) ;; *) fail "this install script carries no release key: use the one at https://celestine.halid.dev/install.sh" ;; esac
	printf 'celestine %s\n' "$KEY" > "$WORK/signers"
	{
		printf 'celestine-sums-1\n%s\nstable\n' "$VERSION"
		cat "$WORK/SHA256SUMS"
	} > "$WORK/SHA256SUMS.signed"
	if command -v ssh-keygen >/dev/null 2>&1; then
		if ssh-keygen -Y verify -f "$WORK/signers" -I celestine -n celestine-release -s "$WORK/SHA256SUMS.sig" \
			< "$WORK/SHA256SUMS.signed" >/dev/null 2>&1; then
			return 0
		fi
		# an OpenSSH before 8.1 has no -Y, and one built for FIPS refuses ed25519: neither can tell, which is not a bad
		# signature (#564). One that can make an ed25519 key and knows -Y could tell, and said no.
		if ! ssh-keygen -Y verify 2>&1 | grep -qi -e 'unknown option' -e 'illegal option' -e 'invalid option' &&
			ssh-keygen -q -t ed25519 -N '' -f "$WORK/probe" </dev/null >/dev/null 2>&1; then
			fail "the release's checksums are not signed with Celestine's key: not installed"
		fi
	fi
	[ "${CELESTINE_UNVERIFIED:-}" = 1 ] ||
		fail "checking the release's signature needs ssh-keygen (OpenSSH 8.1 or later): install it, or run with CELESTINE_UNVERIFIED=1 to install on the checksums alone"
	warn "the signature was not checked (no ssh-keygen that can): installing on the checksums alone, as CELESTINE_UNVERIFIED=1 says"
}

# older A B says whether version A is older than version B, both vX.Y.Z (what follows a "-" is not compared: the
# script installs stable releases).
older() {
	a=${1#v} b=${2#v}
	a=${a%%-*} b=${b%%-*}
	for _ in 1 2 3; do
		x=${a%%.*} y=${b%%.*}
		x=${x:-0} y=${y:-0}
		case $x$y in *[!0-9]*) return 1 ;; esac
		[ "$x" -lt "$y" ] && return 0
		[ "$x" -gt "$y" ] && return 1
		case $a in *.*) a=${a#*.} ;; *) a=0 ;; esac
		case $b in *.*) b=${b#*.} ;; *) b=0 ;; esac
	done
	return 1
}

# get FILE downloads a file of the latest release into the work folder and checks it against SHA256SUMS.
get() {
	fetch "$DL/$VERSION/$1" "$WORK/$1" || fail "could not download $1"
	want=$(awk -v f="$1" '$2 == f || $2 == "*" f { print $1 }' "$WORK/SHA256SUMS")
	[ -n "$want" ] || fail "$1 is not in the release's SHA256SUMS"
	[ "$(sha256 "$WORK/$1")" = "$want" ] || fail "$1 does not match its checksum: not installed"
}

install_mac() {
	get "celestine-darwin-$ARCH.zip"
	unzip -q "$WORK/celestine-darwin-$ARCH.zip" -d "$WORK/app" || fail "could not unpack the app"
	[ -d "$WORK/app/celestine.app" ] || fail "the download holds no celestine.app"
	dir=${CELESTINE_DIR:-/Applications}
	if [ -z "${CELESTINE_DIR:-}" ] && [ ! -w "$dir" ]; then
		dir=$HOME/Applications
	fi
	mkdir -p "$dir"
	app=$dir/Celestine.app new=$dir/.Celestine.app.new old=$dir/.Celestine.app.old
	if pgrep -xq celestine 2>/dev/null; then step "Celestine is open: it takes the new version when it is opened again"; fi
	# the new app is first moved beside the old one (the work folder can be on another volume, so this is a copy),
	# then the old one aside and the new one in, by renames: a step that fails leaves the old app in its place
	rm -rf "$new" "$old"
	if ! mv "$WORK/app/celestine.app" "$new"; then
		rm -rf "$new"
		fail "could not copy the app into $dir: the old one is kept"
	fi
	# on a volume that does not tell cases apart, an older install's celestine.app is this same Celestine.app
	if [ -e "$app" ] && ! mv "$app" "$old"; then
		rm -rf "$new"
		fail "could not move the old app aside: it is kept"
	fi
	if ! mv "$new" "$app"; then
		[ ! -e "$old" ] || mv "$old" "$app" || true
		rm -rf "$new"
		fail "could not put the app in place: the old one is kept"
	fi
	rm -rf "$old"
	# an older install's celestine.app, on a volume that tells cases apart
	if [ -e "$dir/celestine.app" ] && ! [ "$dir/celestine.app" -ef "$app" ]; then rm -rf "$dir/celestine.app"; fi
	xattr -dr com.apple.quarantine "$app" 2>/dev/null || true
	# the bundle as a whole signed ad hoc, as scripts/dmg.sh does: the linker signs the arm64 program alone
	codesign --force --deep -s - "$app" >/dev/null 2>&1 || true
	done_ "Installed in $app"
	echo
	printf '  Open it from Launchpad or Spotlight, or: %sopen -a Celestine%s\n' "$B" "$R"
}

# desktop_exec PROGRAM copies a .desktop file from the input to the output with its Exec=celestine run as PROGRAM, a
# full path, quoted as the Desktop Entry spec asks (a space, a "|" or a "&" in a home folder's name breaks
# nothing): within the quotes a backslash before each of \ " ` $, every backslash doubled again for the file's
# string values, and a % doubled.
desktop_exec() {
	case $1 in *"
"*) fail "a line break in the folder's name: $1" ;; esac
	EXEC="\"$(printf '%s' "$1" | sed -e 's/\\/\\\\\\\\/g' -e 's/["`$]/\\\\&/g' -e 's/%/%%/g')\"" \
		awk '/^Exec=celestine( |$)/ { print "Exec=" ENVIRON["EXEC"] substr($0, 15); next } { print }'
}

install_linux() {
	name=celestine-linux-$ARCH
	get "$name.tar.gz"
	tar -xzf "$WORK/$name.tar.gz" -C "$WORK" || fail "could not unpack the program"
	bin=${CELESTINE_DIR:-${XDG_BIN_HOME:-$HOME/.local/bin}}
	data=${XDG_DATA_HOME:-$HOME/.local/share}
	[ -f "$WORK/$name/celestine" ] || fail "the download holds no program"
	mkdir -p "$bin" "$data/applications" "$data/icons/hicolor/256x256/apps"
	# beside the old program and renamed over it, so a running Celestine keeps its file
	cp "$WORK/$name/celestine" "$bin/.celestine.new"
	chmod 0755 "$bin/.celestine.new"
	mv -f "$bin/.celestine.new" "$bin/celestine"
	cp "$WORK/$name/celestine.png" "$data/icons/hicolor/256x256/apps/celestine.png"
	# the menu starts it by its full path: ~/.local/bin is not on every desktop's PATH
	desktop_exec "$bin/celestine" < "$WORK/$name/celestine.desktop" > "$data/applications/celestine.desktop"
	command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database -q "$data/applications" 2>/dev/null || true
	command -v gtk-update-icon-cache >/dev/null 2>&1 && gtk-update-icon-cache -q "$data/icons/hicolor" 2>/dev/null || true
	done_ "Installed in $bin/celestine"
	echo
	case ":$PATH:" in
	*":$bin:"*) printf '  Open it from your applications, or run: %scelestine%s\n' "$B" "$R" ;;
	*) printf '  Open it from your applications, or run: %s%s/celestine%s\n  (%s is not on your PATH)\n' "$B" "$bin" "$R" "$bin" ;;
	esac
}

main() {
	paint
	logo
	platform
	step "Looking for the latest version"
	VERSION=$(fetch "$DL/update.json" | sed -n 's/^ *"version": *"\([^"]*\)".*/\1/p' | head -n 1) || true
	case $VERSION in v[0-9]*) ;; *) fail "could not read the latest version from $DL" ;; esac
	case $VERSION in *[!A-Za-z0-9.+-]*) fail "not a version: $VERSION" ;; esac
	case $FLOOR in v[0-9]*) ! older "$VERSION" "$FLOOR" || fail "the server offers $VERSION, older than $FLOOR, which this script came with: not installed" ;; esac
	WORK=$(mktemp -d)
	trap 'rm -rf "$WORK"' EXIT INT TERM
	fetch "$DL/$VERSION/SHA256SUMS" "$WORK/SHA256SUMS" || fail "could not download the release's checksums"
	fetch "$DL/$VERSION/SHA256SUMS.sig" "$WORK/SHA256SUMS.sig" || fail "could not download the signature of the release's checksums"
	verify_sums
	step "Downloading Celestine $VERSION for $OS/$ARCH"
	if [ "$OS" = darwin ]; then install_mac; else install_linux; fi
	echo
}

main "$@"
