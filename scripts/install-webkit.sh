#!/usr/bin/env bash
set -euo pipefail

repo=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
sdk_destination="$repo/.webkit-sdk"
cache="$repo/install_files/webkit"

# shellcheck source=/dev/null
. /etc/os-release

signature="local-dependencies-v3|${ID:-}|${VERSION_ID:-}|$(uname -m)|$repo"
has_webkit_processes() {
  "$repo/node" --input-type=module -e \
    'const { webkitRuntimePaths } = await import(process.argv[2]); webkitRuntimePaths(process.argv[1]);' \
    "$1" "$repo/scripts/webkit-runtime.mjs"
}
if [[ "${1:-}" == --check && -f "$sdk_destination/.host" &&
      "$(cat "$sdk_destination/.host")" == "$signature" ]] && has_webkit_processes "$sdk_destination" && \
      "$repo/node" "$repo/scripts/check-webkit-libraries.mjs" "$sdk_destination"; then
  exit 0
fi

# Reconstruit dans un dossier distinct : un échec conserve le SDK utilisable,
# et les bibliothèques d'une ancienne distribution ne sont jamais mélangées.
sdk=$(mktemp -d "$repo/.webkit-sdk.stage.XXXXXX")
cleanup_sdk() {
  [[ -d "$sdk" ]] || return 0
  find "$sdk" -type d -exec chmod u+rwx {} +
  rm -rf -- "$sdk"
}
trap cleanup_sdk EXIT
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

extract_rpm() {
  local package=$1 patterns="$sdk/.rpm-files"
  # Fedora's filesystem RPM gives /usr/lib* mode 0555. Do not restore directory
  # permissions: cpio creates writable parent directories for this user's SDK.
  rpm -qp --qf '[%{FILEMODES:perms} %{FILENAMES}\n]' "$package" \
    | awk 'substr($0, 1, 1) != "d" { sub(/^[^ ]+ /, "."); print }' > "$patterns"
  if [[ -s "$patterns" ]]; then
    (cd "$sdk" && rpm2cpio "$package" | cpio -idmu --no-preserve-owner --no-absolute-filenames --quiet --pattern-file="$patterns")
  fi
  rm -f "$patterns"
}

case " ${ID:-} ${ID_LIKE:-} " in
  *" fedora "*|*" rhel "*)
    command -v rpm2cpio >/dev/null || { echo 'rpm2cpio est requis sur Fedora.' >&2; exit 1; }
    command -v cpio >/dev/null || { echo 'cpio est requis sur Fedora.' >&2; exit 1; }
    if [[ "$ID" == fedora ]]; then
      [[ -x "$repo/node" ]] || { echo 'Node local manquant. Lancez make install.' >&2; exit 1; }
      "$repo/node" "$repo/scripts/fetch-webkit.mjs" "$ID" "$VERSION_ID"
      while IFS= read -r filename; do
        [[ "$filename" != */* && "$filename" == *.rpm ]] || { echo 'Liste des archives invalide.' >&2; exit 1; }
        extract_rpm "$cache/$filename"
      done < "$cache/.selected-archives"
    elif has_archive "gtk3-devel-*.$(uname -m).rpm" && \
       has_archive "webkit2gtk4.1-devel-*.$(uname -m).rpm"; then
      echo "Archives GTK3 et WebKitGTK trouvées dans $cache ; téléchargement ignoré."
      for package in "$cache"/*."$(uname -m)".rpm "$cache"/*.noarch.rpm; do
        [[ -s "$package" ]] || continue
        extract_rpm "$package"
      done
    else
      echo "Placez les archives RPM GTK3/WebKitGTK 4.1 et leurs dépendances dans $cache, puis relancez make install." >&2
      exit 1
    fi
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

has_webkit_processes "$sdk" || {
  echo 'Processus WebKit ou bundle injecté manquants dans le SDK local.' >&2
  exit 1
}

# Les fichiers .pc des paquets pointent vers /usr ; adapte-les au SDK local.
while IFS= read -r -d '' pc; do
  sed -i -e "s|=/usr/|=$sdk/usr/|g" -e "s|=/usr$|=$sdk/usr|g" \
    -e "s|-I/usr/|-I$sdk/usr/|g" -e "s|-L/usr/|-L$sdk/usr/|g" "$pc"
done < <(find "$sdk/usr" -type f -name '*.pc' -print0)

# Relocalise les liens absolus sans copier de bibliothèques de l'hôte.
while IFS= read -r -d '' link; do
  target=$(readlink "$link")
  if [[ "$target" == /usr/* || "$target" == /lib/* || "$target" == /lib64/* ]]; then
    # A relative target remains valid after the staging SDK is renamed.
    local_target=$(realpath -m --relative-to="$(dirname "$link")" "$sdk$target")
    ln -sfn "$local_target" "$link"
  fi
done < <(find "$sdk" -type l -print0)

export PKG_CONFIG_LIBDIR="$sdk/usr/lib64/pkgconfig:$sdk/usr/lib/x86_64-linux-gnu/pkgconfig:$sdk/usr/lib/pkgconfig:$sdk/usr/share/pkgconfig"
export PKG_CONFIG_PATH="$PKG_CONFIG_LIBDIR"
[[ -x "$sdk/usr/bin/pkg-config" ]] && "$sdk/usr/bin/pkg-config" --exists gtk+-3.0 webkit2gtk-4.1 || {
  echo 'Headers GTK3/WebKit2GTK 4.1 introuvables via pkg-config.' >&2
  exit 1
}

"$repo/node" "$repo/scripts/check-webkit-libraries.mjs" "$sdk"

# L'extraction des archives n'exécute pas les hooks des gestionnaires de paquets.
if [[ -x "$sdk/usr/bin/glib-compile-schemas" && -d "$sdk/usr/share/glib-2.0/schemas" ]]; then
  "$sdk/usr/bin/glib-compile-schemas" "$sdk/usr/share/glib-2.0/schemas"
fi
"$repo/node" "$repo/scripts/prepare-webkit-images.mjs" "$sdk" "$sdk_destination"

# Le SDK validé doit garder des chemins stables après son déplacement.
while IFS= read -r -d '' pc; do
  sed -i "s|$sdk|$sdk_destination|g" "$pc"
done < <(find "$sdk/usr" -type f -name '*.pc' -print0)
printf '%s\n' "$signature" > "$sdk/.host"
"$repo/node" "$repo/scripts/configure-webkit-sdk.mjs" "$sdk" "$sdk_destination"
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
