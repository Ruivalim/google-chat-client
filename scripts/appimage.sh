#!/usr/bin/env bash
# Empacota o build Linux do MyGo (build/linux-amd64) num AppImage.
# O MyGo gera .deb e .tar.gz, mas não AppImage. O mesmo script roda local
# (make appimage) e no workflow de release.
#
# Uso: scripts/appimage.sh [versão]   (padrão: version de mygo.config.ts)
set -euo pipefail

cd "$(dirname "$0")/.."
version=${1:-$(sed -n 's/^ *version: "\(.*\)",$/\1/p' mygo.config.ts)}
src=build/linux-amd64
tools=${APPIMAGE_TOOLS:-.cache/appimage}

# Versões fixas e conferidas por hash: "continuous" muda sem aviso, e o
# appimagetool baixaria o runtime sozinho se não recebesse um.
appimagetool_url=https://github.com/AppImage/appimagetool/releases/download/1.9.1/appimagetool-x86_64.AppImage
appimagetool_sha=ed4ce84f0d9caff66f50bcca6ff6f35aae54ce8135408b3fa33abfc3cb384eb0
runtime_url=https://github.com/AppImage/type2-runtime/releases/download/20251108/runtime-x86_64
runtime_sha=2fca8b443c92510f1483a883f60061ad09b46b978b2631c807cd873a47ec260d

fetch() {
	local url=$1 sha=$2 dest=$3
	if [ -f "$dest" ] && echo "$sha  $dest" | sha256sum -c --status; then
		return
	fi
	mkdir -p "$(dirname "$dest")"
	curl -fsSL -o "$dest.part" "$url"
	if ! echo "$sha  $dest.part" | sha256sum -c --status; then
		rm -f "$dest.part"
		echo "hash inesperado em $url" >&2
		exit 1
	fi
	mv "$dest.part" "$dest"
	chmod +x "$dest"
}

if [ ! -x "$src/google-chat-client" ]; then
	echo "falta $src/google-chat-client; rode make build antes" >&2
	exit 1
fi
fetch "$appimagetool_url" "$appimagetool_sha" "$tools/appimagetool"
fetch "$runtime_url" "$runtime_sha" "$tools/runtime"

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
appdir=$work/AppDir
# Mesmo layout do .tar.gz: o MyGo procura os recursos ao lado do executável.
mkdir -p "$appdir/usr/bin" "$appdir/usr/share/icons/hicolor/1024x1024/apps"
install -m 755 "$src/google-chat-client" "$appdir/usr/bin/google-chat-client"
install -m 644 resources/icon.png "$appdir/usr/bin/icon.png"
install -m 644 resources/icon.png "$appdir/usr/share/icons/hicolor/1024x1024/apps/google-chat-client.png"
install -m 644 resources/icon.png "$appdir/google-chat-client.png"
ln -s google-chat-client.png "$appdir/.DirIcon"
{
	cat "$src/google-chat-client.desktop"
	echo "X-AppImage-Version=$version"
} >"$appdir/google-chat-client.desktop"
cat >"$appdir/AppRun" <<'APPRUN'
#!/bin/sh
here=$(dirname "$(readlink -f "$0")")
exec "$here/usr/bin/google-chat-client" "$@"
APPRUN
chmod 755 "$appdir/AppRun"

out=$src/Google_Chat_Client-$version-x86_64.AppImage
# APPIMAGE_EXTRACT_AND_RUN: o appimagetool também é AppImage, e o runner do
# CI não tem FUSE 2.
ARCH=x86_64 APPIMAGE_EXTRACT_AND_RUN=1 "$tools/appimagetool" --no-appstream \
	--runtime-file "$tools/runtime" "$appdir" "$out"
echo "$out"
