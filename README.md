<div align="center">

# tcode

**Editor de texto en la terminal. Rápido, liviano, extensible.**

_Escrito en Go puro. Sin dependencias nativas. Abre al instante._

[![CI](https://github.com/leav-dev/tcode/actions/workflows/ci.yml/badge.svg)](https://github.com/leav-dev/tcode/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/leav-dev/tcode)](https://github.com/leav-dev/tcode/releases)
[![Go](https://img.shields.io/github/go-mod/go-version/leav-dev/tcode)](https://go.dev)
[![Platform](https://img.shields.io/badge/platform-macos%20%7C%20linux%20%7C%20windows-blue)](docs/install.md)

```bash
curl -fsSL https://raw.githubusercontent.com/leav-dev/tcode/main/scripts/install.sh | bash
tcode .
```

[Instalación](#-instalación-en-30-segundos) · [Preview](#-preview) · [Atajos](#-uso-diario) · [Extensiones](#-extensiones) · [Docs](docs/constitution.md)

</div>

---

## Por qué tcode

| | |
|---|---|
| ⚡ **Arranca ya** | Binario estático de ~4 MB. Sin Electron, sin Node, sin espera. |
| 📄 **Piezas, no RAM** | `Piece Table` sobre `mmap`: abrís archivos grandes sin cargarlos en memoria. |
| 🧩 **Extensible en serio** | Comandos, keybindings y hooks declarativos + scripting en Lua. Catálogo instalable por id. |
| 🎨 **Lindo de verdad** | 7 paletas, resaltado por rol, temas por JSON, ventana de config en vivo (`Ctrl+P`). |
| 🔒 **Predecible** | Go puro (`tcell`), chequeo de updates en segundo plano, nunca bloquea el arranque. |

## ✨ Lo esencial

- **Edición** — undo/redo, auto-indent, salto de palabra, guardado con detección de cambios externos
- **Explorador** — árbol lateral con lazy loading, reveal del archivo activo, crear/borrar desde teclado
- **Pestañas** — fila + menú (`Ctrl+T`), cambio rápido, deduplicación de rutas
- **Selección** — teclado, mouse, `Shift+clic` para extender, clipboard del sistema
- **Config viva** — `Ctrl+P` cambia todo en vivo y persiste en `~/.tcode/config.json`
- **Self-update** — `tcode update` trae el último release verificado

> Detalle completo de cada área en [`docs/constitution.md`](docs/constitution.md) y [`docs/memory.md`](docs/memory.md).

## 🚀 Instalación en 30 segundos

**Recomendada — release precompilado (sin Go, sin repo):**

```bash
# macOS / Linux
curl -fsSL https://raw.githubusercontent.com/leav-dev/tcode/main/scripts/install.sh | bash

# Windows (PowerShell)
powershell -ExecutionPolicy Bypass -Command "irm https://raw.githubusercontent.com/leav-dev/tcode/main/scripts/install.ps1 | iex"
```

Después abrí una terminal nueva y corré `tcode`. El binario queda en `~/.tcode/bin` con checksum verificado.

**Alternativas:**

```bash
brew tap leav-dev/tcode && brew install tcode   # Homebrew
go install github.com/leav-dev/tcode@latest     # Go 1.25+
```

> ¿Recién salió un tag y `@latest` trae la anterior? Es el caché del proxy de Go: pedí la versión explícita (`@vX.Y.Z`) o usá `GOPROXY=direct`.
>
> Compilar desde fuente, cross-build y notas de macOS/Gatekeeper → [`docs/install.md`](docs/install.md)

## 🧪 Preview

Probá lo último de la rama `preview` sin ensuciar tu instalación estable. Los tags `preview-v*` salen como **Pre-release** en GitHub: `tcode update` y el instalador por defecto los ignoran a propósito.

**Opción A — último preview (sin Go):**

```bash
# macOS / Linux
bash scripts/install.sh --preview
# o directo desde GitHub:
curl -fsSL https://raw.githubusercontent.com/leav-dev/tcode/main/scripts/install.sh | bash -s -- --preview
```

```powershell
# Windows (PowerShell)
powershell -ExecutionPolicy Bypass -File scripts/install.ps1 -Preview
```

> Avanzado: pineá una versión con `TCODE_RELEASE_BASE=https://github.com/leav-dev/tcode/releases/download/preview-vX.Y.Z bash scripts/install.sh` (`$env:TCODE_RELEASE_BASE` en PowerShell).

**Opción B — desde la rama (siempre al día, requiere Go 1.25+):**

```bash
git fetch origin preview && git checkout preview
bash scripts/install.sh --build
```

> Volver a estable es el instalador normal (sin `TCODE_RELEASE_BASE`) o `tcode update`. Nunca taggees `v*-preview`: matchea `v*` y saldría como estable.

## ⌨️ Uso diario

```bash
tcode main.go        # abrir archivo
tcode ./proyecto     # abrir carpeta (árbol visible)
tcode                # carpeta actual
```

| Querés | Hacé |
|---|---|
| Guardar / Guardar como | `Ctrl+S` / `Ctrl+Shift+S` |
| Explorador on/off | `Ctrl+B` |
| Crear archivo / carpeta | `Ctrl+N` / `Ctrl+Shift+N` |
| Borrar nodo del cursor | `Delete` (con confirmación) |
| Cambiar pestaña | `Ctrl+PgDn` / `Ctrl+PgUp` · `Ctrl+K` / `Ctrl+L` |
| Menú de pestañas | `Ctrl+T` |
| Configuración | `Ctrl+P` |
| Salir | Doble `Esc` rápido |

<details>
<summary><b>Más atajos (selección, edición, extensiones)</b></summary>

| Querés | Hacé |
|---|---|
| Seleccionar | `Shift`+flechas, `Ctrl+Shift+Left/Right` por palabra, `Ctrl+A` todo, arrastrar con mouse, `Shift+clic` extiende |
| Copiar / cortar / pegar | `Ctrl+C` / `Ctrl+X` / `Ctrl+V` |
| Deshacer / rehacer | `Ctrl+Z` / `Ctrl+Y` |
| Salto por palabra | `Ctrl+Left` / `Ctrl+Right` (con `Shift` extiende selección) |
| Salto de línea visual (wrap) | `Ctrl+Shift+W` |
| Ventana de extensiones | `Ctrl+P` → **Extensiones** (`←`/`→` cambia pestaña, `Enter` instala/actualiza/borra, `r` revalida) |
| Instalar por id | `tcode --install-extension tcode.vimlite` |
| Listar / borrar | `tcode --list-extensions` · `tcode --remove-extension <proveedor:id\|id>` |
| Ayuda / versión | `tcode --help` · `tcode --version` (un flag que no existe falla con la guía de permitidos) |
| Actualizar tcode | `tcode update` (el preview solo sigue previews, nunca baja a estable) |
| Desinstalar tcode | `tcode uninstall` (saca binario + PATH, conserva config y extensiones) |

</details>

## 🧩 Extensiones

Sistema de proveedores: instalás por id desde un catálogo (el oficial o el tuyo).

```bash
tcode --install-extension tcode.vimlite
tcode --add-provider <url-git|carpeta>   # registra fuente (sin aprobar)
tcode --approve-provider mios            # confía explícito
```

El editor revisa updates **en segundo plano** al arrancar y avisa en la barra sin bloquearte:

```
2 actualizaciones, 1 novedad — Ctrl+P → Extensiones
```

> Cómo publicar, versionar y aprobar proveedores → [`docs/extension-system.md`](docs/extension-system.md)

## 🎨 Config y temas

Todo vive en `~/.tcode/` y se edita desde `Ctrl+P` o a mano:

| Archivo | Qué es | Doc |
|---|---|---|
| `config.json` | tab, word wrap, ancho del panel, tema | [`docs/config.md`](docs/config.md) |
| `theme.json` | tus colores por rol | [`docs/editor-theme.md`](docs/editor-theme.md) |
| `providers.json` | tus fuentes de extensiones | [`docs/extension-system.md`](docs/extension-system.md) |

## 🏗️ Arquitectura en 10 segundos

```
internal/model       Piece Table + mmap + workspace
internal/view        EditorView, árbol, pestañas, config flotante, temas
internal/controller  App: eventos, panes, persistencia
internal/ext         Extensiones (comandos + keybindings + Lua)
```

> Principios y decisiones → [`docs/constitution.md`](docs/constitution.md)

## 🛠️ Desarrollo

```bash
go test ./...
go vet ./...
```

Convenciones: rama por feature en `feat/*`, commits en inglés, se commitea al terminar y no se hace push salvo pedido (`agents.md` §4).

---

<div align="center">

**¿Te sirve tcode? Dejale una ⭐ y contanos qué extensión te falta.**

[Releases](https://github.com/leav-dev/tcode/releases) · [Issues](https://github.com/leav-dev/tcode/issues) · [Docs](docs/constitution.md)

</div>
