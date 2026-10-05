# Feature: Instalador por releases precompilados (sin repo, sin Go)

## Description
Para que una persona SIN el repo (y sin Go) instale tcode: GitHub Actions
compila los binarios en cada tag `v*` y los sube como release assets; el
instalador detecta la plataforma, **descarga** el binario correspondiente a
`~/.tcode/bin` y configura el PATH (como ya hace). La compilación local queda
como modo explícito `--build` / `-Build` para desarrolladores.

## Tasks
- [ ] `.github/workflows/release.yml`: on `push: tags ['v*']`; matrix de targets
  (linux-amd64, darwin-amd64, darwin-arm64, windows-amd64) cross-compilando con
  CGO_ENABLED=0 desde ubuntu; nombres `tcode-<os>-<arch>[.exe]`; `checksums.txt`
  (sha256); upload con softprops/action-gh-release <!-- id: 0 -->
- [ ] `scripts/install.sh`: por defecto descarga de
  `https://github.com/leav-dev/tcode/releases/latest/download/` (detecta
  Linux/Darwin/MINGW + arquitectura), verifica sha256 contra checksums.txt,
  instala como `tcode` (o `.exe`) y configura el PATH; `--build` ejecuta la
  compilación local actual; `TCODE_RELEASE_BASE` para pruebas <!-- id: 1 -->
- [ ] `scripts/install.ps1`: por defecto descarga vía Invoke-WebRequest
  (detecta AMD64/ARM64), Get-FileHash + comparación, instala `tcode.exe`;
  `-Build` compila local; `TCODE_RELEASE_BASE` para pruebas <!-- id: 2 -->
- [ ] Docs: `docs/install.md` con la vía recomendada (release, sin requisitos
  de Go/git) y la vía dev (`--build`); nota del primer tag pendiente
  <!-- id: 3 -->
- [ ] Verificación funcional: release falso servido local (python http.server
  con assets + checksums) y ambos instaladores apuntados a esa base con
  `TCODE_RELEASE_BASE`, en sandbox (HOME/`-InstallDir` + `-NoPath`); mapping
  de plataforma validado en el host (Git Bash → windows/amd64) <!-- id: 4 -->
- [ ] Commit de unidad <!-- id: 5 -->

## Design decisions
- **Cross-compile desde un solo runner:** tcode es Go puro (tcell sin cgo),
  `CGO_ENABLED=0` da binarios estáticos correctos para las 4 plataformas; se
  evita una matrix pesada por runner nativo.
- **`latest/download`** con assets planos + un `checksums.txt`: el instalador
  verifica la integridad del binario (sha256) antes de instalarlo; el nombre
  del asset por convención `tcode-<os>-<arch>[.exe]`.
- **Descarga por defecto, compilación opcional:** quien desarrolla sigue con
  `--build`; quien solo usa tcode no necesita ni Go ni git — solo el script.
- **`TCODE_RELEASE_BASE` inyectable:** permite probar el flujo completo contra
  un servidor local (release falso) sin depender de GitHub.