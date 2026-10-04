# TAP Pixel

Police originale dessinée pour le projet sur une grille de 5 × 7 pixels.
Les lettres minuscules utilisent les formes capitales, comme dans un menu de
console portable. Les données et pseudos conservent leur casse réelle.

`tap-pixel.ttf` est embarquée dans le frontend et ne demande aucun téléchargement.
Le générateur n'utilise aucune dépendance : depuis la racine du dépôt,
`./node scripts/build-pixel-font.mjs` reconstruit le fichier à l'identique.

La police comprend l'ASCII, les lettres latines décomposables avec accents et
la ponctuation courante. Les autres caractères Unicode utilisent le repli
monospace du système.
