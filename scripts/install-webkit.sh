#!/usr/bin/env bash
set -euo pipefail

repo=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
sdk_destination="$repo/.webkit-sdk"
cache="$repo/install_files/webkit"

# shellcheck source=/dev/null
. /etc/os-release

signature="${ID:-}|${VERSION_ID:-}|$(uname -m)|$repo"
if [[ "${1:-}" == --check && -f "$sdk_destination/.host" &&
      "$(cat "$sdk_destination/.host")" == "$signature" ]]; then
  exit 0
fi

# Reconstruit dans un dossier distinct : un échec conserve le SDK utilisable,
# et les bibliothèques d'une ancienne distribution ne sont jamais mélangées.
sdk=$(mktemp -d "$repo/.webkit-sdk.stage.XXXXXX")
trap '[[ ! -d "$sdk" ]] || rm -rf -- "$sdk"' EXIT
export PATH="$repo:$sdk/usr/bin:$PATH"
export LD_LIBRARY_PATH="$sdk/usr/lib64:$sdk/usr/lib/x86_64-linux-gnu:$sdk/usr/lib:${LD_LIBRARY_PATH:-}"
mkdir -p "$cache"

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
      echo "Placez les archives RPM GTK3/WebKitGTK 4.1 et leurs dépendances dans $cache, puis relancez make install." >&2
      exit 1
    fi
    for package in "$cache"/*."$(uname -m)".rpm "$cache"/*.noarch.rpm; do
      [[ -s "$package" ]] || continue
      (cd "$sdk" && rpm2cpio "$package" | cpio -idmu --quiet)
    done
    ;;
  *" debian "*|*" ubuntu "*)
    command -v dpkg-deb >/dev/null || { echo 'dpkg-deb est requis sur Debian/Ubuntu.' >&2; exit 1; }
    [[ -x "$repo/node" ]] || { echo 'Node local manquant. Lancez make install.' >&2; exit 1; }
    # Recalcule la sélection pour cet hôte, même si quelques archives existent.
    # Les index et les archives complets sont réutilisés et vérifiés localement.
    "$repo/node" "$repo/scripts/fetch-webkit.mjs" "$ID" "${VERSION_CODENAME:-}"
    while IFS= read -r filename; do
      [[ "$filename" != */* && "$filename" == *.deb ]] || { echo 'Liste des archives invalide.' >&2; exit 1; }
      dpkg-deb -x "$cache/$filename" "$sdk"
    done < "$cache/.selected-archives"
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
[[ -x "$sdk/usr/bin/pkg-config" ]] && "$sdk/usr/bin/pkg-config" --exists gtk+-3.0 webkit2gtk-4.1 || {
  echo 'Headers GTK3/WebKit2GTK 4.1 introuvables via pkg-config.' >&2
  exit 1
}

# Le SDK validé doit garder des chemins stables après son déplacement.
while IFS= read -r -d '' pc; do
  sed -i "s|$sdk|$sdk_destination|g" "$pc"
done < <(find "$sdk/usr" -type f -name '*.pc' -print0)
printf '%s\n' "$signature" > "$sdk/.host"
touch "$sdk/.installed"
old_sdk=''
if [[ -d "$sdk_destination" ]]; then
  old_sdk=$(mktemp -d "$repo/.webkit-sdk.old.XXXXXX")
  rmdir "$old_sdk"
  mv "$sdk_destination" "$old_sdk"
fi
if ! mv "$sdk" "$sdk_destination"; then
  [[ -z "$old_sdk" ]] || mv "$old_sdk" "$sdk_destination"
  exit 1
fi
[[ -z "$old_sdk" ]] || rm -rf -- "$old_sdk"
