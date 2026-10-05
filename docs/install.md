# Instalación

tcode se **compila en tu máquina** (no se distribuyen binarios) y se instala en
la carpeta `.tcode` de tu usuario (`~/.tcode/bin`, o sea junto a tu
`config.json`/`theme.json`), con esa carpeta agregada al **PATH de tu usuario**
para ejecutar `tcode` desde cualquier terminal.

## Requisito

- **Go 1.25+** en el PATH (lo declara `go.mod`). Verificá con `go version`.

## Comandos

| SO | Instalar | Desinstalar |
| --- | --- | --- |
| **macOS** | `bash scripts/install.sh` | `bash scripts/install.sh --uninstall` |
| **Linux** | `bash scripts/install.sh` | `bash scripts/install.sh --uninstall` |
| **Windows** | `powershell -ExecutionPolicy Bypass -File scripts/install.ps1` | `... scripts/install.ps1 -Uninstall` |

Después de instalar, **abrí una terminal nueva** (el PATH se aplica a
terminales nuevas) y ejecutá `tcode`.

## Qué hace

1. Compila el binario con `go build` desde el checkout:
   - macOS/Linux → `~/.tcode/bin/tcode`
   - Windows → `%USERPROFILE%\.tcode\bin\tcode.exe`
2. Agrega la carpeta al PATH **del usuario**:
   - macOS/Linux: un bloque marcado (`# >>> tcode >>>`) en `~/.zshrc` y
     `~/.bashrc` (crea los archivos si no existen; repetir el instalador no
     duplica).
   - Windows: el registro `Path` de **usuario** (no el del sistema), con
     dedupe; no se usa `setx` (trunca el PATH en 1024 caracteres).
3. Idempotente: correrlo de nuevo no duplica entradas.

## Notas y límites

- El PATH del **usuario** no toca el PATH del **sistema** (no se necesita
  admin).
- El instalador no toca tu `config.json`/`theme.json` — solo agrega `bin/`.
- El binario se compila de la rama de trabajo actual: `git pull` + re-instalar
  o `go build` a mano actualiza la versión.
- Uso avanzado: `TCODE_INSTALL_DIR=<dir> bash scripts/install.sh` y
  `-InstallDir <dir>` / `-NoPath` en PowerShell (para rutas y pruebas
  personalizadas).

En el código: `scripts/install.sh` (bash) y `scripts/install.ps1` (Windows).