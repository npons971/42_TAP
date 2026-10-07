#!/usr/bin/env bash
# ==============================================================================
# 42 TAP - Environment Checker for GUI Toolkits (Fyne vs Wails)
# ==============================================================================

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

echo -e "${BLUE}${BOLD}======================================================${NC}"
echo -e "${BLUE}${BOLD}     42 TAP - GUI Toolkit Environment Checker        ${NC}"
echo -e "${BLUE}${BOLD}======================================================${NC}\n"

OS="$(uname -s)"
ARCH="$(uname -m)"
echo -e "${CYAN}OS détecté :${NC} $OS ($ARCH)"

PROJECT_ROOT=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
LOCAL_OK=1

check_local_file() {
    local label=$1 path=$2
    if [ -e "$PROJECT_ROOT/$path" ]; then
        echo -e "  [✔] $label : ${GREEN}présent${NC} ($path)"
    else
        echo -e "  [✘] $label : ${RED}absent${NC} ($path)"
        LOCAL_OK=0
    fi
}

echo -e "\n${BOLD}[Local] Fichiers du projet${NC}"
check_local_file 'Go' '.go-sdk/go/bin/go'
check_local_file 'Node.js' '.node-sdk/bin/node'
check_local_file 'npm' '.node-sdk/lib/node_modules/npm/bin/npm-cli.js'
check_local_file 'Wails CLI' '.go-work/bin/wails'
if [ "$OS" = "Linux" ]; then
    check_local_file 'Header GTK3' '.webkit-sdk/usr/include/gtk-3.0/gtk/gtk.h'
    check_local_file 'Header WebKitGTK' '.webkit-sdk/usr/include/webkitgtk-4.1/webkit2/webkit2.h'
    if [ -f "$PROJECT_ROOT/.webkit-sdk/usr/lib64/pkgconfig/gtk+-3.0.pc" ] ||
       [ -f "$PROJECT_ROOT/.webkit-sdk/usr/lib/x86_64-linux-gnu/pkgconfig/gtk+-3.0.pc" ]; then
        echo -e "  [✔] Fichier pkg-config GTK3 : ${GREEN}présent${NC}"
    else
        echo -e "  [✘] Fichier pkg-config GTK3 : ${RED}absent${NC}"
        LOCAL_OK=0
    fi
    if [ -f "$PROJECT_ROOT/.webkit-sdk/usr/lib64/pkgconfig/webkit2gtk-4.1.pc" ] ||
       [ -f "$PROJECT_ROOT/.webkit-sdk/usr/lib/x86_64-linux-gnu/pkgconfig/webkit2gtk-4.1.pc" ]; then
        echo -e "  [✔] Fichier pkg-config WebKitGTK : ${GREEN}présent${NC}"
    else
        echo -e "  [✘] Fichier pkg-config WebKitGTK : ${RED}absent${NC}"
        LOCAL_OK=0
    fi
fi

FYNE_OK=1
WAILS_OK=1

# ------------------------------------------------------------------------------
# 1. OUTILS DE BASE
# ------------------------------------------------------------------------------
echo -e "\n${BOLD}[1/4] Vérification des outils de base${NC}"

# Go
if command -v go >/dev/null 2>&1; then
    GO_VER=$(go version | awk '{print $3}')
    echo -e "  [✔] Go installé : ${GREEN}$GO_VER${NC} ($(command -v go))"
else
    echo -e "  [✘] Go : ${RED}NON INSTALLÉ${NC}"
    FYNE_OK=0
    WAILS_OK=0
fi

# Make
if command -v make >/dev/null 2>&1; then
    echo -e "  [✔] Make : ${GREEN}OK${NC} ($(command -v make))"
else
    echo -e "  [✘] Make : ${RED}NON INSTALLÉ${NC}"
fi

# C Compiler (CGo requis par Fyne et les bindings C)
CC_CMD=""
if command -v gcc >/dev/null 2>&1; then
    CC_CMD="gcc"
elif command -v clang >/dev/null 2>&1; then
    CC_CMD="clang"
elif command -v cc >/dev/null 2>&1; then
    CC_CMD="cc"
fi

if [ -n "$CC_CMD" ]; then
    CC_VER=$($CC_CMD --version | head -n 1)
    echo -e "  [✔] Compilateur C ($CC_CMD) : ${GREEN}OK${NC} ($CC_VER)"
else
    echo -e "  [✘] Compilateur C (gcc/clang) : ${RED}NON INSTALLÉ${NC} (Requis pour CGo !)"
    FYNE_OK=0
    WAILS_OK=0
fi

# pkg-config
if command -v pkg-config >/dev/null 2>&1; then
    echo -e "  [✔] pkg-config : ${GREEN}OK${NC}"
else
    echo -e "  [!] pkg-config : ${YELLOW}NON INSTALLÉ${NC} (utile pour détecter les libs C)"
fi

# ------------------------------------------------------------------------------
# 2. DÉPENDANCES WAILS
# ------------------------------------------------------------------------------
echo -e "\n${BOLD}[2/4] Vérification des dépendances WAILS${NC}"

# Node.js & npm
if command -v node >/dev/null 2>&1; then
    NODE_VER=$(node -v)
    echo -e "  [✔] Node.js : ${GREEN}$NODE_VER${NC}"
else
    echo -e "  [✘] Node.js : ${RED}NON INSTALLÉ${NC} (Indispensable pour builder le frontend Wails)"
    WAILS_OK=0
fi

if command -v npm >/dev/null 2>&1; then
    NPM_VER=$(npm -v)
    echo -e "  [✔] npm : ${GREEN}v$NPM_VER${NC}"
else
    echo -e "  [✘] npm : ${RED}NON INSTALLÉ${NC}"
    WAILS_OK=0
fi

# Wails CLI
if command -v wails >/dev/null 2>&1; then
    WAILS_VER=$(wails version 2>/dev/null | head -n 1)
    echo -e "  [✔] Wails CLI : ${GREEN}OK${NC} ($WAILS_VER)"
else
    echo -e "  [!] Wails CLI : ${YELLOW}NON TROUVÉ DANS PATH${NC} (s'installe via 'go install github.com/wailsapp/wails/v2/cmd/wails@latest')"
fi

# Dépendances système C / WebView
if [ "$OS" = "Linux" ]; then
    echo "  -> Test des bibliothèques Linux (WebKitGTK & GTK3)..."
    
    HAS_GTK3=0
    HAS_WEBKIT=0

    if pkg-config --exists gtk+-3.0 2>/dev/null; then
        HAS_GTK3=1
        echo -e "     [✔] Header GTK3 : ${GREEN}Présent via pkg-config${NC}"
    fi

    if pkg-config --exists webkit2gtk-4.0 2>/dev/null || pkg-config --exists webkit2gtk-4.1 2>/dev/null; then
        HAS_WEBKIT=1
        echo -e "     [✔] Header WebKit2GTK : ${GREEN}Présent via pkg-config${NC}"
    fi

    # Si pkg-config échoue, tester via le compilateur C ou l'arborescence /usr/include
    if [ $HAS_GTK3 -eq 0 ]; then
        if [ -n "$CC_CMD" ] && echo '#include <gtk/gtk.h>' | $CC_CMD -x c -E - $(pkg-config --cflags gtk+-3.0 2>/dev/null) - >/dev/null 2>&1; then
            HAS_GTK3=1
            echo -e "     [✔] Header GTK3 : ${GREEN}Trouvé par le compilateur${NC}"
        elif [ -d "/usr/include/gtk-3.0" ]; then
            HAS_GTK3=1
            echo -e "     [✔] Header GTK3 : ${GREEN}Dossier /usr/include/gtk-3.0 présent${NC}"
        else
            echo -e "     [✘] Header GTK3 (libgtk-3-dev) : ${RED}MANQUANT${NC}"
        fi
    fi

    if [ $HAS_WEBKIT -eq 0 ]; then
        if [ -n "$CC_CMD" ] && echo '#include <webkit2/webkit2.h>' | $CC_CMD -x c -E - $(pkg-config --cflags webkit2gtk-4.0 webkit2gtk-4.1 2>/dev/null) - >/dev/null 2>&1; then
            HAS_WEBKIT=1
            echo -e "     [✔] Header WebKit2GTK : ${GREEN}Trouvé par le compilateur${NC}"
        elif ls -d /usr/include/webkit2gtk* >/dev/null 2>&1; then
            HAS_WEBKIT=1
            echo -e "     [✔] Header WebKit2GTK : ${GREEN}Dossier /usr/include/webkit2gtk* présent${NC}"
        else
            echo -e "     [✘] Header WebKit2GTK (libwebkit2gtk-4.0-dev ou 4.1-dev) : ${RED}MANQUANT${NC}"
        fi
    fi

    if [ $HAS_GTK3 -eq 0 ] || [ $HAS_WEBKIT -eq 0 ]; then
        WAILS_OK=0
    fi

elif [ "$OS" = "Darwin" ]; then
    echo "  -> Test macOS (WebKit Cocoa framework natif)..."
    if [ -n "$CC_CMD" ]; then
        if echo '#import <WebKit/WebKit.h>' | $CC_CMD -x objective-c -E - >/dev/null 2>&1; then
            echo -e "     [✔] Framework WebKit macOS : ${GREEN}DISPONIBLE${NC}"
        else
            echo -e "     [✘] Framework WebKit : ${RED}MANQUANT${NC} (Vérifier Xcode Command Line Tools)"
            WAILS_OK=0
        fi
    fi
fi

# ------------------------------------------------------------------------------
# 3. DÉPENDANCES FYNE
# ------------------------------------------------------------------------------
echo -e "\n${BOLD}[3/4] Vérification des dépendances FYNE${NC}"

if [ "$OS" = "Linux" ]; then
    echo "  -> Test des bibliothèques Linux (OpenGL & X11)..."
    
    HAS_GL=0
    HAS_X11=0

    if pkg-config --exists gl x11 xcursor xrandr xinerama xi 2>/dev/null; then
        HAS_GL=1
        HAS_X11=1
        echo -e "     [✔] OpenGL & X11 : ${GREEN}Présents via pkg-config${NC}"
    else
        # Test direct avec compilateur ou dossiers
        if [ $HAS_GL -eq 0 ]; then
            if [ -n "$CC_CMD" ] && echo '#include <GL/gl.h>' | $CC_CMD -x c -E - >/dev/null 2>&1; then
                HAS_GL=1
                echo -e "     [✔] Header OpenGL (GL/gl.h) : ${GREEN}Présent${NC}"
            elif [ -d "/usr/include/GL" ]; then
                HAS_GL=1
                echo -e "     [✔] Header OpenGL : ${GREEN}Dossier /usr/include/GL présent${NC}"
            else
                echo -e "     [✘] Header OpenGL (libgl1-mesa-dev) : ${RED}MANQUANT${NC}"
            fi
        fi

        if [ $HAS_X11 -eq 0 ]; then
            if [ -n "$CC_CMD" ] && echo '#include <X11/Xlib.h>' | $CC_CMD -x c -E - >/dev/null 2>&1; then
                HAS_X11=1
                echo -e "     [✔] Header X11 (X11/Xlib.h) : ${GREEN}Présent${NC}"
            elif [ -d "/usr/include/X11" ]; then
                HAS_X11=1
                echo -e "     [✔] Header X11 : ${GREEN}Dossier /usr/include/X11 présent${NC}"
            else
                echo -e "     [✘] Header X11 (xorg-dev / libx11-dev) : ${RED}MANQUANT${NC}"
            fi
        fi
    fi

    if [ $HAS_GL -eq 0 ] || [ $HAS_X11 -eq 0 ]; then
        FYNE_OK=0
    fi

elif [ "$OS" = "Darwin" ]; then
    echo "  -> macOS supporte nativement Metal/OpenGL via les SDKs Apple"
    echo -e "     [✔] Support macOS pour Fyne : ${GREEN}OK${NC}"
fi

# ------------------------------------------------------------------------------
# 4. CONTEXTE ÉCOLE 42 (Droits & Homebrew)
# ------------------------------------------------------------------------------
echo -e "\n${BOLD}[4/4] Environnement Session 42${NC}"

if command -v brew >/dev/null 2>&1; then
    echo -e "  [✔] Homebrew utilisateur détecté : ${GREEN}$(command -v brew)${NC}"
    echo "      (Permet d'installer des paquets sans mot de passe root si besoin)"
else
    echo -e "  [i] Homebrew : ${YELLOW}Non présent dans PATH${NC}"
fi

if sudo -n true 2>/dev/null; then
    echo -e "  [✔] Accès sudo : ${GREEN}OUI${NC}"
else
    echo -e "  [i] Accès sudo : ${YELLOW}NON (Standard à 42 - tout doit compiler en espace utilisateur)${NC}"
fi

# ------------------------------------------------------------------------------
# VERDICT FINAL
# ------------------------------------------------------------------------------
echo -e "\n${BLUE}${BOLD}======================================================${NC}"
echo -e "${BLUE}${BOLD}                   VERDICT TECHNIQUE                  ${NC}"
echo -e "${BLUE}${BOLD}======================================================${NC}"

if [ "$LOCAL_OK" -eq 1 ]; then
    echo -e "  ${GREEN}${BOLD}✔ FICHIERS LOCAUX : PRÉSENTS${NC}"
else
    echo -e "  ${RED}${BOLD}✘ FICHIERS LOCAUX : MANQUANTS${NC}"
fi

if [ $FYNE_OK -eq 1 ]; then
    echo -e "  ${GREEN}${BOLD}✔ FYNE : COMPATIBLE SUR CETTE MACHINE${NC}"
else
    echo -e "  ${RED}${BOLD}✘ FYNE : DÉPENDANCES MANQUANTES${NC}"
fi

if [ $WAILS_OK -eq 1 ]; then
    echo -e "  ${GREEN}${BOLD}✔ WAILS : COMPATIBLE SUR CETTE MACHINE${NC}"
else
    echo -e "  ${RED}${BOLD}✘ WAILS : DÉPENDANCES MANQUANTES${NC}"
fi
echo -e "${BLUE}======================================================${NC}\n"
