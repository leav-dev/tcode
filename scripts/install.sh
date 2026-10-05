#!/usr/bin/env bash
# tcode installer for macOS and Linux.
#
# Builds tcode to ~/.tcode/bin and adds that directory to the PATH of the
# current user (via ~/.zshrc and ~/.bashrc, with a marked block so repeated
# runs do not duplicate). Needs Go 1.25+ on the machine (see go.mod).
#
# Usage:
#   bash scripts/install.sh            # install
#   bash scripts/install.sh --uninstall
#
# Advanced: TCODE_INSTALL_DIR overrides the install directory (used by tests
# and power users). The PATH entries always point at ~/.tcode/bin.

set -euo pipefail

TCODE_HOME="${HOME}/.tcode"
INSTALL_DIR="${TCODE_INSTALL_DIR:-${TCODE_HOME}/bin}"
BIN_NAME="tcode"

# repo_root is the root of the tcode checkout (this script lives in scripts/).
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

uninstall() {
  rm -rf "${INSTALL_DIR}"
  for rc in "${HOME}/.bashrc" "${HOME}/.zshrc"; do
    if [ -f "${rc}" ]; then
      # Remove the whole marked block, sed-in-place (GNU and BSD compatible).
      sed -i.bak '/^# >>> tcode >>>$/,/^# <<< tcode <<<$/d' "${rc}"
      rm -f "${rc}.bak"
    fi
  done
  echo "tcode removed from ${INSTALL_DIR} and from your shell rc files."
  echo "Open a new terminal (or unset PATH entries manually) to drop it from the current session."
  exit 0
}

if [ "${1:-}" = "--uninstall" ]; then
  uninstall
fi

command -v go >/dev/null 2>&1 || {
  echo "error: Go is required to build tcode (go.mod requires Go 1.25+)." >&2
  exit 1
}

mkdir -p "${INSTALL_DIR}"
echo "Building tcode into ${INSTALL_DIR} ..."
( cd "${REPO_ROOT}" && go build -o "${INSTALL_DIR}/${BIN_NAME}" . )

# Shell rc files: a marked block per file, idempotent.
PATH_LINE="export PATH=\"\${HOME}/.tcode/bin:\${PATH}\""
for rc in "${HOME}/.zshrc" "${HOME}/.bashrc"; do
  touch "${rc}"
  if grep -q "^# >>> tcode >>>\$" "${rc}"; then
    echo "PATH already configured in ${rc}"
    continue
  fi
  {
    printf '%s\n' "# >>> tcode >>>"
    printf '%s\n' "${PATH_LINE}"
    printf '%s\n' "# <<< tcode <<<"
  } >> "${rc}"
  echo "PATH configured in ${rc}"
done

echo
echo "tcode installed:"
echo "  binary: ${INSTALL_DIR}/${BIN_NAME}"
echo "  PATH:   ${INSTALL_DIR} (user scope)"
echo
echo "Open a NEW terminal and run: tcode"
echo "To remove: bash scripts/install.sh --uninstall"