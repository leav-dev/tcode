#!/usr/bin/env bash
# tcode-dev: punto de entrada de desarrollo.
#
# Reconstruye el binario si hay fuentes más nuevas y después lo ejecuta, así
# `tcode` en el PATH siempre corre el código actual sin un paso manual de
# compilación. No es un enlace al binario a propósito: un symlink apunta a bytes
# congelados y no puede recompilar nada.
#
# El repo se deriva de la ubicación REAL de este script (resolviendo symlinks),
# así el mismo archivo sirve desde <repo>/scripts/ y desde ~/.local/bin/tcode
# cuando ese entry es un symlink a él. Si lo copiás en vez de enlazarlo, el
# repo se resuelve mal: enlazalo.
#
# El directorio de trabajo del llamador se conserva: la compilación corre en una
# subshell, y el binario se ejecuta con el cwd original (importa: `tcode` sin
# argumentos abre el directorio actual como workspace).

set -u

self="$(readlink -f "$0")"
repo="$(dirname "$(dirname "$self")")"

# El binario vive en dist/ y se nombra según el SO anfitrión: `tcode` en
# Linux/macOS, `tcode.exe` en Windows (Git Bash/MSYS/CYGWIN). Si el canónico
# todavía no existe pero hay un binario alojado con otro nombre (p. ej.
# `tcode` sin extensión en Windows o un asset `tcode-<os>-<arch>`), se usa
# ese para no recompilar sin necesidad; si no hay ninguno, se compila el
# canónico abajo.
dev_bin_for_this_os() {
	case "$(uname -s)" in
		MINGW* | MSYS* | CYGWIN*) printf '%s' "$repo/dist/tcode.exe" ;; # Windows
		*) printf '%s' "$repo/dist/tcode" ;; # Linux, macOS y demás Unix
	esac
}

bin="$(dev_bin_for_this_os)"
if [ ! -e "$bin" ]; then
	for legacy in "$repo/dist/tcode" "$repo/dist/tcode.exe" "$repo/dist/tcode-linux-amd64" "$repo/dist/tcode-darwin-arm64" "$repo/dist/tcode-windows-amd64.exe"; do
		if [ -e "$legacy" ]; then
			bin="$legacy"
			break
		fi
	done
fi

# Reconstruye si el binario falta o si algún .go es más nuevo. `.git` se poda
# porque recorrerlo no aporta fuentes y es lo más pesado del árbol.
if [ ! -x "$bin" ] || [ -n "$(find "$repo" -path "$repo/.git" -prune -o -name '*.go' -newer "$bin" -print -quit 2>/dev/null)" ]; then
	printf 'tcode: Iniciando...\n' >&2
	if ! (cd "$repo" && go build -trimpath -ldflags="-s -w" -o "$bin" .); then
		printf 'tcode: la compilación falló; el binario NO se actualizó\n' >&2
		exit 1
	fi
fi

exec "$bin" "$@"
