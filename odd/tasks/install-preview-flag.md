# Install preview flag

## Goal
`scripts/install.sh --preview` y `scripts/install.ps1 -Preview` instalan el último `preview-v*` sin hardcodear versión en el README. El README usa el flag, sin versión específica.

## Context
- Stable vive en `releases/latest` (lo leen `tcode update` y el instalador default). Preview son tags `preview-v*` marcados como Pre-release, nunca en `latest`.
- `scripts/install.sh` ya soporta `TCODE_RELEASE_BASE` + `TCODE_INSTALL_DIR` + `--build`. `scripts/install.ps1` soporta `$env:TCODE_RELEASE_BASE`, `-Build`, `-InstallDir`, `-NoPath`.
- `README.md` sección `🧪 Preview` hoy hardcodea `preview-v0.1.3` como ejemplo.

## Tasks
- [x] `install.sh --preview`: resuelve último tag `preview-v*` vía GitHub API sin depender de `jq` (curl + grep/sed/python3), setea `RELEASE_BASE=.../download/<tag>`, error claro si no hay red o no hay previews. Incompatible con `--build` (error). `TCODE_RELEASE_BASE` explícito sigue ganando (tests). `bash -n` ok.
- [x] `install.ps1 -Preview`: mismo vía `Invoke-RestMethod` a `repos/leav-dev/tcode/releases`, filtra `preview-v*`, setea `ReleaseBase`. Incompatible con `-Build` (error). Parse check ok.
- [x] `README.md`: reemplaza bloque con `preview-v0.1.3` por flag dinámico (bash + powershell), mantiene ejemplo de pineo con `TCODE_RELEASE_BASE` como avanzado. Español, corto.
- [x] Evidencia: `bash -n scripts/install.sh`, resolución del tag en dry-run (echo, sin instalar), y muestra del diff.

## Evidencia (2026-10-07)
- `bash -n scripts/install.sh` → OK.
- Dry-run `bash scripts/install.sh --preview` → `error: no hay tags preview-v* ...`, exit 1, sin efectos (la API hoy solo lista `v0.1.x`: el camino de error es el esperado hasta publicar el primer preview).
- `pwsh` no disponible en esta máquina: el `.ps1` se revisó por diff (espejo del `.sh`).

## Constraints
- Superficies: `scripts/install.sh`, `scripts/install.ps1`, `README.md`. Nada más.
- No commitear, no pushear. Devuelve diff + comandos corridos.
- Sin `jq` obligatorio en bash; en ps1 solo cmdlets estándar.
