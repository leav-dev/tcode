#!/usr/bin/env bash
# tcode installer for macOS and Linux.
#
# Default: downloads the prebuilt binary for this platform from the latest
# GitHub release (no repo, no Go needed) into ~/.tcode/bin and adds that
# directory to the PATH of the current user (marked block in ~/.zshrc and
# ~/.bashrc, idempotent).
#
#   --preview     install the latest `preview-v*` pre-release (no Go needed)
#   --build      compile from the current checkout instead (devs)
#   --uninstall  remove the binary and the PATH entries
#
# Advanced: TCODE_RELEASE_BASE overrides the release base URL (tests);
# TCODE_INSTALL_DIR overrides the install directory.

set -euo pipefail

TCODE_HOME="${HOME}/.tcode"
INSTALL_DIR="${TCODE_INSTALL_DIR:-${TCODE_HOME}/bin}"
RELEASE_BASE="${TCODE_RELEASE_BASE:-https://github.com/leav-dev/tcode/releases/latest/download}"
BUILD_LOCAL="${TCODE_BUILD_LOCAL:-0}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

uninstall() {
  rm -rf "${INSTALL_DIR}"
  for rc in "${HOME}/.bashrc" "${HOME}/.zshrc"; do
    if [ -f "${rc}" ]; then
      sed -i.bak '/^# >>> tcode >>>$/,/^# <<< tcode <<<$/d' "${rc}"
      rm -f "${rc}.bak"
    fi
  done
  echo "tcode removed from ${INSTALL_DIR} and from your shell rc files."
  echo "Open a new terminal (or unset PATH entries manually) to drop it from the current session."
  exit 0
}

PREVIEW=0
for arg in "$@"; do
  case "${arg}" in
    --uninstall) uninstall ;;
    --build) BUILD_LOCAL=1 ;;
    --preview) PREVIEW=1 ;;
    *)
      echo "error: unknown argument ${arg} (expected --preview, --build or --uninstall)" >&2
      exit 1
      ;;
  esac
done

if [ "${PREVIEW}" = "1" ] && [ "${BUILD_LOCAL}" = "1" ]; then
  echo "error: --preview y --build son incompatibles (preview descarga binario, build compila local)." >&2
  exit 1
fi

# resolve_preview_tag: latest `preview-v*` tag via the GitHub releases API.
# No jq: python3 first, grep/sed fallback. Fails with a clear message
# when offline or when there are no preview releases.
resolve_preview_tag() {
  local api_url="https://api.github.com/repos/leav-dev/tcode/releases"
  local json
  if ! json="$(curl -fsSL --max-time 20 "${api_url}" 2>/dev/null)"; then
    echo "error: no se pudo consultar ${api_url} (sin red o GitHub no responde)." >&2
    exit 1
  fi
  local tag=""
  if command -v python3 >/dev/null 2>&1; then
    tag="$(printf '%s' "${json}" | python3 -c 'import json,sys
try:
    data = json.load(sys.stdin)
except Exception:
    sys.exit(1)
for r in data:
    t = r.get("tag_name", "")
    if isinstance(t, str) and t.startswith("preview-v"):
        print(t)
        break
' 2>/dev/null)" || tag=""
  fi
  if [ -z "${tag}" ]; then
    tag="$(printf '%s' "${json}" | grep -o '"tag_name"[[:space:]]*:[[:space:]]*"preview-v[^"]*"' | head -n 1 | sed -E 's/^"tag_name"[[:space:]]*:[[:space:]]*"//;s/"$//')"
  fi
  if [ -z "${tag}" ]; then
    echo "error: no hay tags preview-v* en ${api_url}." >&2
    exit 1
  fi
  printf '%s' "${tag}"
}

if [ "${PREVIEW}" = "1" ]; then
  if [ -n "${TCODE_RELEASE_BASE:-}" ]; then
    echo "Preview pedido pero TCODE_RELEASE_BASE explícito gana: ${RELEASE_BASE}"
  else
    PREVIEW_TAG="$(resolve_preview_tag)"
    RELEASE_BASE="https://github.com/leav-dev/tcode/releases/download/${PREVIEW_TAG}"
    echo "Preview: ${PREVIEW_TAG} (${RELEASE_BASE})"
  fi
fi

# build_local: compile from the checkout (requires Go 1.25+).
build_local() {
  command -v go >/dev/null 2>&1 || {
    echo "error: Go is required to build tcode (go.mod requires Go 1.25+)." >&2
    exit 1
  }
  mkdir -p "${INSTALL_DIR}"
  echo "Building tcode into ${INSTALL_DIR} ..."
  ( cd "${REPO_ROOT}" && go build -o "${INSTALL_DIR}/tcode" . ) >/dev/null
}

# platform: maps the host to our asset names (tcode-<os>-<arch>[.exe]).
detect_platform() {
  case "$(uname -s)" in
    Linux) OS=linux ;;
    Darwin) OS=darwin ;;
    MINGW* | MSYS* | CYGWIN*) OS=windows ;;
    *)
      echo "error: unsupported system $(uname -s)" >&2
      exit 1
      ;;
  esac
  case "$(uname -m)" in
    x86_64 | amd64) ARCH=amd64 ;;
    arm64 | aarch64) ARCH=arm64 ;;
    *)
      echo "error: unsupported architecture $(uname -m)" >&2
      exit 1
      ;;
  esac
}

# release_install: download + verify checksum + install (no Go needed).
release_install() {
  detect_platform
  # Git Bash and friends run this script on Windows: keep the .exe name.
  if [ "${OS}" = "windows" ]; then
    BIN_FILE="tcode.exe"
  else
    BIN_FILE="tcode"
  fi
  ASSET="tcode-${OS}-${ARCH}$( [ "${OS}" = "windows" ] && printf '.exe' )"

  command -v curl >/dev/null 2>&1 || {
    echo "error: curl is required to download the release binary." >&2
    exit 1
  }

  mkdir -p "${INSTALL_DIR}"
  TMP_DIR="$(mktemp -d)"
  trap 'rm -rf "${TMP_DIR}"' EXIT

  echo "Downloading ${ASSET} (${RELEASE_BASE}) ..."
  curl -fSL -o "${TMP_DIR}/${ASSET}" "${RELEASE_BASE}/${ASSET}"
  curl -fSL -o "${TMP_DIR}/checksums.txt" "${RELEASE_BASE}/checksums.txt"

  # Verify sha256 from checksums.txt (sha256sum on Linux/Git Bash, shasum on macOS).
  ( cd "${TMP_DIR}" && grep "${ASSET}" checksums.txt > checksums.line )
  if command -v sha256sum >/dev/null 2>&1; then
    ( cd "${TMP_DIR}" && sha256sum -c checksums.line )
  else
    ( cd "${TMP_DIR}" && shasum -a 256 -c checksums.line )
  fi

  cp "${TMP_DIR}/${ASSET}" "${INSTALL_DIR}/${BIN_FILE}"
  chmod 755 "${INSTALL_DIR}/${BIN_FILE}"
  trap - EXIT
  rm -rf "${TMP_DIR}"
  echo "Installed: ${INSTALL_DIR}/${BIN_FILE}"
}

# --- PATH setup (shared by both install modes) ---
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

if [ "${BUILD_LOCAL}" = "1" ]; then
  build_local
else
  release_install
fi

echo
echo "tcode installed:"
echo "  binary: ${INSTALL_DIR}/$( [ "${OS:-}" = "windows" ] && printf 'tcode.exe' || printf 'tcode')"
echo "  PATH:   ${INSTALL_DIR} (user scope)"
echo
echo "Open a NEW terminal and run: tcode"
echo "To remove: bash scripts/install.sh --uninstall"