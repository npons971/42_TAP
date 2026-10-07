#!/usr/bin/env bash
set -euo pipefail

repo=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
sdk="$repo/.webkit-sdk"
cache="$repo/install_files/webkit"

# shellcheck source=/dev/null
. /etc/os-release

mkdir -p "$sdk" "$cache"

# Utilise les archives déjà présentes dans le projet avant de solliciter le réseau.
has_archive() {
  local pattern=$1 package
  for package in "$cache"/$pattern; do
    [[ -s "$package" ]] && return 0
  done
  return 1
}

case " ${ID:-} ${ID_LIKE:-} " in
  *" fedora "*|*" rhel "*)
    command -v rpm2cpio >/dev/null || { echo 'rpm2cpio est requis sur Fedora.' >&2; exit 1; }
    command -v cpio >/dev/null || { echo 'cpio est requis sur Fedora.' >&2; exit 1; }
    if has_archive "gtk3-devel-*.$(uname -m).rpm" && \
       has_archive "webkit2gtk4.1-devel-*.$(uname -m).rpm"; then
      echo "Archives GTK3 et WebKitGTK trouvées dans $cache ; téléchargement ignoré."
    else
      command -v dnf >/dev/null || { echo 'dnf est requis sur Fedora si les archives sont absentes.' >&2; exit 1; }
      dnf --setopt=multilib_policy=best download --resolve \
        --arch="$(uname -m)" --arch=noarch --destdir "$cache" \
        gtk3-devel webkit2gtk4.1-devel
    fi
    for package in "$cache"/*."$(uname -m)".rpm "$cache"/*.noarch.rpm; do
      [[ -s "$package" ]] || continue
      (cd "$sdk" && rpm2cpio "$package" | cpio -idmu --quiet)
    done
    ;;
  *" debian "*|*" ubuntu "*)
    command -v dpkg-deb >/dev/null || { echo 'dpkg-deb est requis sur Debian/Ubuntu.' >&2; exit 1; }
    mkdir -p "$cache/partial"
    if has_archive 'libgtk-3-dev_*.deb' && \
       has_archive 'libwebkit2gtk-4.1-dev_*.deb'; then
      echo "Archives GTK3 et WebKitGTK trouvées dans $cache ; téléchargement ignoré."
    else
      command -v apt-get >/dev/null || { echo 'apt-get est requis sur Debian/Ubuntu si les archives sont absentes.' >&2; exit 1; }
      apt-get -y -o Debug::NoLocking=1 -o Dir::Cache::archives="$cache" \
        --download-only --reinstall install libgtk-3-dev libwebkit2gtk-4.1-dev
    fi
    for package in "$cache"/*.deb; do
      [[ -s "$package" ]] || continue
      dpkg-deb -x "$package" "$sdk"
    done
    ;;
  *)
    echo "Distribution non prise en charge: ${ID:-inconnue}" >&2
    exit 1
    ;;
esac

# Les fichiers .pc des paquets pointent vers /usr ; adapte-les au SDK local.
while IFS= read -r -d '' pc; do
  sed -i -e "s|=/usr/|=$sdk/usr/|g" -e "s|=/usr$|=$sdk/usr|g" \
    -e "s|-I/usr/|-I$sdk/usr/|g" -e "s|-L/usr/|-L$sdk/usr/|g" "$pc"
done < <(find "$sdk/usr" -type f -name '*.pc' -print0)

# Les paquets -devel contiennent des liens .so vers les bibliothèques déjà
# présentes sur l'hôte. Copie ces cibles dans le SDK pour que l'édition de liens
# et l'exécution utilisent les fichiers du projet.
while IFS= read -r -d '' link; do
  [[ -e "$link" ]] && continue
  target=$(readlink "$link")
  if [[ "$target" = /* ]]; then
    source="$target"
    destination="$link"
  else
    source="$(dirname "${link#"$sdk"}")/$target"
    destination="$(dirname "$link")/$target"
  fi
  [[ -f "$source" ]] || continue
  [[ -L "$destination" ]] && continue
  mkdir -p "$(dirname "$destination")"
  cp -L "$source" "$destination"
done < <(find "$sdk/usr/lib" "$sdk/usr/lib64" -type l -name '*.so*' -print0 2>/dev/null)

export PKG_CONFIG_PATH="$sdk/usr/lib64/pkgconfig:$sdk/usr/lib/x86_64-linux-gnu/pkgconfig:$sdk/usr/lib/pkgconfig:$sdk/usr/share/pkgconfig:${PKG_CONFIG_PATH:-}"
pkg-config --exists gtk+-3.0 webkit2gtk-4.1 || {
  echo 'Headers GTK3/WebKit2GTK 4.1 introuvables via pkg-config.' >&2
  exit 1
}
