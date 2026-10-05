# Instalación

tcode se instala en la carpeta `.tcode` de tu usuario (`~/.tcode/bin`, o sea
junto a tu `config.json`/`theme.json`), con esa carpeta agregada al **PATH de
tu usuario** para ejecutar `tcode` desde cualquier terminal.

## Vía recomendada: release precompilado (sin repo, sin Go)

Los binarios se publican como assets de las releases de GitHub (workflow
`.github/workflows/release.yml`, tag `v*`): el instalador detecta tu
plataforma, **descarga** el binario correspondiente, **verifica su checksum
(sha256)** y lo instala.

| SO | Instalar | Desinstalar |
| --- | --- | --- |
| **macOS** | `curl -fsSL https://raw.githubusercontent.com/leav-dev/tcode/main/scripts/install.sh \| bash` | `bash scripts/install.sh --uninstall` |
| **Linux** | `curl -fsSL https://raw.githubusercontent.com/leav-dev/tcode/main/scripts/install.sh \| bash` | `bash scripts/install.sh --uninstall` |
| **Windows** | `powershell -ExecutionPolicy Bypass -Command "irm https://raw.githubusercontent.com/leav-dev/tcode/main/scripts/install.ps1 \| iex"` | `powershell -ExecutionPolicy Bypass -File scripts/install.ps1 -Uninstall` |

> La persona que instala no necesita el repo ni Go: solo el comando de
> arriba. El primer release (`v…`) todavía no fue publicado: hasta entonces,
> usá la vía de desarrollo o creá un tag.

## Vía de desarrollo: compilar (requiere Go 1.25+, lo declara `go.mod`)

Desde un checkout del repo:

| SO | Instalar |
| --- | --- |
| **macOS / Linux** | `bash scripts/install.sh --build` |
| **Windows** | `powershell -ExecutionPolicy Bypass -File scripts/install.ps1 -Build` |

## Qué hace (ambas vías)

El binario cae en `~/.tcode/bin` (o `%USERPROFILE%\.tcode\bin\tcode.exe`) y,
después, la carpeta se agrega al PATH **del usuario**:

- macOS/Linux: bloque marcado (`# >>> tcode >>>`) en `~/.zshrc` y `~/.bashrc`
  (idempotente).
- Windows: registro `Path` de **usuario** (no el del sistema; no se usa `setx`,
  que trunca en 1024 caracteres).

Después de instalar, **abrí una terminal nueva** y ejecutá `tcode`.

## Notas y límites

- El PATH del **usuario** no toca el PATH del **sistema** (sin admin).
- El instalador no toca tu `config.json`/`theme.json` — solo agrega `bin/`.
- La vía release verifica el sha256 contra `checksums.txt` antes de instalar;
  un binario corrupto se rechaza.
- Uso avanzado/pruebas: ``TCODE_RELEASE_BASE`` / `TCODE_INSTALL_DIR` (bash) y
  `-InstallDir` / `-NoPath` + `$env:TCODE_RELEASE_BASE` (PowerShell).

En el código: `.github/workflows/release.yml`, `scripts/install.sh` (bash),
`scripts/install.ps1` (Windows).