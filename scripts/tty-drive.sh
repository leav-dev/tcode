#!/usr/bin/env bash
# tty-drive: maneja tcode en un PTY real (tmux) y devuelve la pantalla como texto
# limpio. Es la capa que `script(1)` no da: mandar teclas y LEER lo dibujado sin
# parsear secuencias de escape.
#
# Uso:
#   scripts/tty-drive.sh [opciones] [--] [args de tcode...]
#
#   --keys 'C-p Enter'   teclas para tmux send-keys (C-p = Ctrl+P, Enter,
#                        Escape, Down, Delete, F1; el resto es texto literal)
#   --size 100x30        tamaño del PTY (default 100x30)
#   --wait 2             segundos de espera tras arrancar (default 2)
#   --after 0.5          segundos de espera tras mandar las teclas (default 0.5)
#   --raw                imprime la salida cruda del arranque, no la pantalla
#
# El socket de tmux es propio (tmux -L tcode-tty-$$), así que NUNCA toca las
# sesiones del usuario. El servidor se mata siempre, incluso si algo falla.

set -u

repo="$(cd "$(dirname "$(readlink -f "$0")")/.." && pwd)"
bin="$repo/dist/tcode-linux-amd64"

keys=""
size="100x30"
wait_s="2"
after_s="0.5"

while [ $# -gt 0 ]; do
	case "$1" in
	--keys)
		keys="$2"
		shift 2
		;;
	--size)
		size="$2"
		shift 2
		;;
	--wait)
		wait_s="$2"
		shift 2
		;;
	--after)
		after_s="$2"
		shift 2
		;;
	--)
		shift
		break
		;;
	*)
		break
		;;
	esac
done

if [ ! -x "$bin" ]; then
	printf 'tty-drive: falta %s; compilá primero (o usá el wrapper tcode-dev.sh)\n' "$bin" >&2
	exit 1
fi

sock="tcode-tty-$$"
cleanup() { tmux -L "$sock" kill-server 2>/dev/null || true; }
trap cleanup EXIT

cols="${size%x*}"
rows="${size#*x}"

# La sesión arranca con el binario y los argumentos tal cual: tcode abre su UI
# dentro del PTY que le da tmux.
tmux -L "$sock" new-session -d -s drive -x "$cols" -y "$rows" \
	"TERM=xterm-256color '$bin' $*" 2>/dev/null || {
	printf 'tty-drive: no se pudo crear la sesión de tmux\n' >&2
	exit 1
}

sleep "$wait_s"

if [ -n "$keys" ]; then
	# send-keys interpreta los nombres de tecla y manda el resto como texto.
	# shellcheck disable=SC2086
	tmux -L "$sock" send-keys $keys 2>/dev/null || true
	sleep "$after_s"
fi

tmux -L "$sock" capture-pane -p 2>/dev/null
