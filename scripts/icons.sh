#!/bin/sh
# Makes the app's icon at every size from its drawings in assets/ (ImageMagick with librsvg, png2icns):
#
#   assets/icon.svg       the icon: two crystal carets as stickers on a paper ground, for 48 px and up
#   assets/icon-16.svg    the same carets alone on whole pixels, for 16 to 24 px, where an outline cannot be seen
#   assets/icon-32.svg    the same, for 32 to 40 px
#
# and writes what the builds read: in the Dock's shape for macOS and the window (icon-*.png, icon.icns), filling
# its square for Windows (icon-win.png, icon.ico, and the .syso through scripts/winicon), for the web, the
# Android launcher (its ground in values/ic_launcher_background.xml is the paper tone) and the site's favicon
# (icon.bin of the celestine-site repository, when its checkout is at $CELESTINE_SITE, ../celestine-site by default).
#
#   scripts/icons.sh
set -eu
cd "$(dirname "$0")/.."
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

# square draws a drawing filling an n-pixel square: the small ones for the sizes they were drawn for
square() { # n out
	src=assets/icon.svg w=1024
	[ "$1" -le 24 ] && src=assets/icon-16.svg w=16
	[ "$1" -gt 24 ] && [ "$1" -le 40 ] && src=assets/icon-32.svg w=32
	# rendered at its size, not drawn small and stretched: the density that makes w units n pixels
	magick -background none -density "$((96 * $1 / w))" "$src" -resize "$1x$1" "$2"
}

# dock draws the icon in the Dock's shape at n pixels: a rounded square 824/1024 of it with a soft shadow below,
# as macOS's own icons are
dock() { # n out
	s=$(($1 * 824 / 1024))
	m=$((($1 - s) / 2))
	r=$(($1 * 185 / 1024))
	square "$s" "$work/art.png"
	magick -size "${s}x$s" xc:black -fill white -draw "roundrectangle 0,0 $((s - 1)),$((s - 1)) $r,$r" "$work/mask.png"
	magick "$work/art.png" "$work/mask.png" -alpha off -compose copy_opacity -composite "$work/shaped.png"
	magick "$work/shaped.png" \( +clone -background black -shadow "30x$(($1 / 80 + 1))+0+$(($1 / 100))" \) +swap \
		-background none -layers merge +repage "$work/shadowed.png"
	magick -size "${1}x$1" xc:none "$work/shadowed.png" -geometry "+$((m - $1 / 80 - 1))+$((m - $1 / 80 - 1))" \
		-composite "$2"
}

for n in 32 64 128 256 512 1024; do dock "$n" "assets/icon-$n.png"; done
for n in 16 48; do dock "$n" "$work/icns-$n.png"; done
png2icns assets/icon.icns "$work/icns-16.png" assets/icon-32.png "$work/icns-48.png" assets/icon-128.png \
	assets/icon-256.png assets/icon-512.png assets/icon-1024.png >/dev/null

square 256 assets/icon-win.png
for n in 16 20 24 32 40 48 64 96 128 256; do square "$n" "$work/ico-$n.png"; done
magick $(for n in 16 20 24 32 40 48 64 96 128 256; do echo "$work/ico-$n.png"; done) assets/icon.ico
go run ./scripts/winicon

cp assets/icon-256.png web/icon-256.png
cp assets/icon-512.png web/icon-512.png
cp assets/icon-256.png android/app/src/main/assets/web/icon-256.png
cp assets/icon-512.png android/app/src/main/assets/web/icon-512.png
site=${CELESTINE_SITE:-../celestine-site}
if [ -d "$site" ]; then cp assets/icon-256.png "$site/icon.bin"; else echo "no site checkout at $site: its icon.bin is left as it is" >&2; fi

# Android: the legacy icon fills its square with rounded corners; the adaptive one's foreground is the picture
# within the middle 60%, which every launcher's mask leaves whole, on the paper ground
res=android/app/src/main/res
for d in mdpi:48 hdpi:72 xhdpi:96 xxhdpi:144 xxxhdpi:192; do
	dir=${d%:*} n=${d#*:}
	square "$n" "$work/a.png"
	r=$((n * 18 / 100))
	magick -size "${n}x$n" xc:black -fill white -draw "roundrectangle 0,0 $((n - 1)),$((n - 1)) $r,$r" "$work/am.png"
	magick "$work/a.png" "$work/am.png" -alpha off -compose copy_opacity -composite "$res/mipmap-$dir/ic_launcher.png"
	f=$((n * 9 / 4))
	inner=$((f * 60 / 100))
	square "$inner" "$work/fg.png"
	magick -size "${f}x$f" xc:'#F7F6F2' "$work/fg.png" -gravity center -composite "$res/mipmap-$dir/ic_launcher_foreground.png"
done
