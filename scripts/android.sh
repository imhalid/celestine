#!/bin/sh
# Builds the web version and wraps it in an Android app (android/): a WebView that serves the web build from
# its assets. Output: android/app/build/outputs/apk/debug/app-debug.apk
#   scripts/android.sh           build the APK
#   scripts/android.sh install   build, and install and start it on the device or emulator that adb sees
set -eu
cd "$(dirname "$0")/.."
./scripts/web.sh build
rm -rf android/app/src/main/assets/web
mkdir -p android/app/src/main/assets
cp -r dist/web android/app/src/main/assets/web
cd android
# Gradle 8 does not run on the newest JDKs: a 17 or 21 of the machine is used when there is one
for j in "$HOME"/.local/share/jdks/jdk-17* "$HOME"/.local/share/jdks/jdk-21* /usr/lib/jvm/java-17* /usr/lib/jvm/java-21*; do
	[ -x "$j/bin/java" ] && export JAVA_HOME="$j" && break
done
./gradlew --offline assembleDebug || ./gradlew assembleDebug
APK=app/build/outputs/apk/debug/app-debug.apk
ls -l "$APK"
if [ "${1:-}" = install ]; then
	ADB="${ANDROID_HOME:-$HOME/Android/Sdk}/platform-tools/adb"
	"$ADB" install -r "$APK"
	"$ADB" shell am start -n dev.halid.ryudit/.MainActivity
fi
