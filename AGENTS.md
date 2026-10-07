# Instructions pour les Agents IA - Projet 42 TAP

Ce document définit les règles, contraintes d'environnement et bonnes pratiques que **tous les agents IA** (et contributeurs automatisés) doivent impérativement respecter sur ce projet.

---

## 1. Règle Fondamentale : Dépendances et Outils STRICTEMENT en Local

> **IMPORTANT**  
> Ce projet est développé dans le cadre du cursus **42**. Les postes de travail ne disposent **pas d'accès super-utilisateur (`sudo`)** et l'environnement système global est verrouillé ou restreint.  
> **Toutes les dépendances, SDKs, runtimes et outils de build sont installés localement à la racine du dépôt via le `Makefile`.**

### 1.1. Interdictions absolues

- ❌ **JAMAIS** de commandes d'administration système : `sudo`, `apt`, `apt-get`, `dnf`, `pacman`, `brew`, etc.
- ❌ **JAMAIS** d'appel direct aux binaires globaux du système sans passer par les wrappers locaux (`/usr/bin/go`, `/usr/bin/node`, `/usr/bin/npm`).
- ❌ **JAMAIS** d'installation globale d'outils (ex: `npm install -g ...` ou `go install` sans configurer `GOPATH` local).
- ❌ **JAMAIS** d'ajout des dossiers d'outils ou d'archives locales dans Git (`.go-sdk`, `.node-sdk`, `.go-work`, `.go-cache`, `.npm-cache`, `.webkit-sdk`, `install_files`, ou les binaires `go`, `node`, `npm`, `wails`). Ils sont déjà listés dans `.gitignore`.

---

## 2. Guide d'Exécution des Outils Locaux

Les outils sont provisionnés par la commande `make install` à la racine du dépôt :

```bash
make install
```

Cette commande met en place :
- **Go** : Archive extraite dans `.go-sdk/`, lien symbolique `./go` à la racine.
- **Node.js** : Archive extraite dans `.node-sdk/`, lien symbolique `./node` à la racine.
- **npm** : Wrapper exécutable `./npm` configurant le cache local (`.npm-cache`) et le `PATH`.
- **Wails CLI** : Installé dans `.go-work/bin/wails` et wrappé par `./wails` avec les variables d'environnement appropriées.
- **WebKitGTK / GTK3** : SDK local sous `.webkit-sdk/` via `scripts/install-webkit.sh`.

### 2.1. Tableau des binaires à utiliser

| Outil | Commande à utiliser | Emplacement réel / Wrapper |
|---|---|---|
| **Go** | `./go` | Racine du projet (`.go-sdk/go/bin/go`) |
| **Node.js** | `./node` | Racine du projet (`.node-sdk/bin/node`) |
| **npm** | `./npm` | Wrapper racine (`.node-sdk/.../npm-cli.js`) |
| **Wails** | `./wails` | Wrapper racine avec GTK/WebKit exportés |

---

## 3. Configuration des Variables d'Environnement

Lorsque vous exécutez des commandes ou écrivez des scripts bash / Makefile / tâches automatisées, vous devez toujours vous assurer que l'environnement pointe vers le projet local :

```bash
# Récupérer la racine du projet
PROJECT_ROOT="$(git rev-parse --show-toplevel)"

# Priorité aux binaires locaux
export PATH="$PROJECT_ROOT:$PATH"

# Configuration Go localisée
export GOPATH="$PROJECT_ROOT/.go-work"
export GOCACHE="$PROJECT_ROOT/.go-cache"

# Configuration npm localisée
export npm_config_cache="$PROJECT_ROOT/.npm-cache"

# Configuration bibliothèques WebKit / GTK (si compilation CGO directe)
export PKG_CONFIG_PATH="$PROJECT_ROOT/.webkit-sdk/usr/lib64/pkgconfig:$PROJECT_ROOT/.webkit-sdk/usr/lib/x86_64-linux-gnu/pkgconfig:$PROJECT_ROOT/.webkit-sdk/usr/lib/pkgconfig:$PROJECT_ROOT/.webkit-sdk/usr/share/pkgconfig:${PKG_CONFIG_PATH:-}"
export LD_LIBRARY_PATH="$PROJECT_ROOT/.webkit-sdk/usr/lib64:$PROJECT_ROOT/.webkit-sdk/usr/lib/x86_64-linux-gnu:$PROJECT_ROOT/.webkit-sdk/usr/lib:${LD_LIBRARY_PATH:-}"
```

> **Astuce** : Le wrapper `./wails` exporte déjà automatiquement ces variables. Privilégiez toujours l'utilisation directe de `./wails`.

---

## 4. Exemples Pratiques de Commandes

### 4.1. Lancer ou tester du code Go

```bash
# À la racine :
./go run ./Project/cmd/server
./go test ./Project/...
./go build -o bin/server ./Project/cmd/server

# Depuis un sous-dossier :
../../go test ./...
```

### 4.2. Gérer les dépendances Go (`go.mod` / `go.sum`)

```bash
GOPATH="$(git rev-parse --show-toplevel)/.go-work" \
GOCACHE="$(git rev-parse --show-toplevel)/.go-cache" \
./go get <module>
```

### 4.3. Installer et utiliser des paquets Node / Frontend

```bash
# À la racine ou dans le sous-dossier frontend :
"$PROJECT_ROOT/npm" install
"$PROJECT_ROOT/npm" run build
"$PROJECT_ROOT/node" path/to/script.js
```

### 4.4. Compiler l'application Wails

```bash
./wails build
./wails dev
```

### 4.5. Vérification de l'état de l'environnement

Pour diagnostiquer la présence de toutes les dépendances locales requises :
```bash
bash scripts/check_42_env.sh
```

---

## 5. Règles pour l'Écriture de Nouveaux Scripts ou Makefiles

Si vous devez créer ou éditer des scripts, des Makefiles ou des tâches CI/CD :
1. **Ne pas hardcoder de chemins absolus** (`/home/...`). Utiliser des résolutions relatives ou `$(CURDIR)` / `$(git rev-parse --show-toplevel)`.
2. **Toujours tester la présence des binaires locaux** avant d'exécuter une tâche (`test -x ./go || make install`).
3. **Conserver l'isolation complète** dans le dépôt pour garantir la portabilité entre machines de l'école 42.
