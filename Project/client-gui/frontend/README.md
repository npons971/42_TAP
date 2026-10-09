# Frontend du client 42 TAP

Svelte 5 et Vite, montés par `mount()` dans `src/main.js`.

Depuis la racine : `make dev-gui`, `make build-frontend`, `make test`.
Le frontend a besoin du bridge Wails pour jouer ; un simple serveur Vite ne
fournit pas la connexion TCP. Voir [le guide du GUI](../README.md).
