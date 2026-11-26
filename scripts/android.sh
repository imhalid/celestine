#!/bin/sh
# Builds the Android app (mobile/, MyGo's Android host, #633) and installs it on the device or emulator adb sees.
#   scripts/android.sh               an .apk and an .aab for every ABI, in mobile/dist/android/
#   scripts/android.sh install       for the ABI of the device or emulator adb sees, installed and started there
#   scripts/android.sh dev           mygo dev: rebuilt and started again as the Go code changes
# It needs Android Studio's SDK, its NDK and a JDK (go run github.com/egoist/mygo/cmd/mygo doctor says what is missing).
set -eu
cd "$(dirname "$0")/../mobile"
mygo() { go run github.com/egoist/mygo/cmd/mygo "$@"; }
ADB="${ANDROID_HOME:-$HOME/Android/Sdk}/platform-tools/adb"
case "${1:-}" in
dev)
	mygo dev -platform android
	;;
install)
	abi=$("$ADB" shell getprop ro.product.cpu.abi | tr -d '\r')
	case "$abi" in
	arm64*) arch=arm64 ;;
	x86_64) arch=amd64 ;;
	armeabi*) arch=arm ;;
	*) echo "no build for the ABI $abi"; exit 1 ;;
	esac
	mygo build -platform "android/$arch"
	"$ADB" install -r dist/android/*.apk
	"$ADB" shell am start -n dev.halid.celestine/dev.mygo.MyGoActivity
	;;
*)
	mygo build -platform android
	;;
esac
