# tcode

Editor de texto en la terminal, escrito en Go. Rápido, con piezas por
documento (`Piece Table` sobre `mmap`), explorador de archivos, pestañas,
resaltado de sintaxis, temas y configuración desde una ventana flotante.

## Características

| Área | Qué incluye |
| --- | --- |
| Edición | Undo/redo, auto-indent, salto de palabra visual, guardado con detección de cambios externos y recarga segura, `Ctrl+S` con confirmación de pisado |
| Selección | Por teclado (`Shift`+flechas/Home/End, `Ctrl+A`) y por mouse (arrastrar, `Shift`+clic extiende); `Ctrl+C`/`Ctrl+X`/`Ctrl+V` con el portapapeles del sistema |
| Explorador de archivos | Árbol lateral con lazy loading (los directorios se leen al expandir), reveal del archivo activo, archivos ocultos fuera del árbol |
| Pestañas | Fila de pestañas, menú (`Ctrl+T`), cambio rápido desde teclado, reabrir y deduplicar rutas |
| Apariencia | 6 paletas (Light, Dark, Light/Dark HC, Tokyo Night, Dracula) con fondo propio, resaltado por rol para Go, Python, JS/TS, C-like y JSON, tema del usuario por JSON |
| Configuración | Ventana flotante (`Ctrl+P`): tamaño de tab, salto de palabra, ancho del panel, tema — cambios en vivo y persistidos en `~/.tcode/config.json` |
| Extensiones | Manifiestos JSON declarativos (comandos, keybindings, hooks) con **backend de scripting Lua** (comandos con implementación propia), instalación por CLI de tus repos: `tcode --install-extension <url-git>` (`docs/extension-system.md`) |
| Portabilidad | Go puro (`tcell`), sin dependencias nativas; instalador multi-SO en camino |

## Instalación

La vía recomendada (binarios precompilados en releases) está **próximamente
disponible**. Hoy:

```bash
# Requisito: Go 1.25+ (lo declara go.mod)
go build -o tcode .
./tcode [archivo-o-directorio]
```

También crudo para probar: `go run . [archivo-o-directorio]`.

## Uso rápido

| Querés… | Hacé |
| --- | --- |
| Abrir archivo | `tcode main.go` · abrir dir: `tcode ./proyecto` |
| Guardar / Guardar como | `Ctrl+S` / `Ctrl+Shift+S` |
| Mostrar el explorador | `Ctrl+B` |
| Cerrar el editor | Doble `Esc` rápido (la única salida; `Ctrl+C` no cierra) |
| Cambiar pestaña | `Ctrl+PageDown` / `Ctrl+PageUp` · `Ctrl+K` (siguiente) / `Ctrl+L` (anterior) |
| Menú de pestañas | `Ctrl+T` |
| Configuración | `Ctrl+P` |
| Salto de palabra | `Ctrl+Shift+W` |
| Deshacer / rehacer | `Ctrl+Z` / `Ctrl+Y` o `Ctrl+Shift+Z` |
| Seleccionar | `Shift`+flechas / `Ctrl+A` todo · arrastrar con el mouse · `Shift`+clic extiende |
| Copiar / cortar / pegar | `Ctrl+C` / `Ctrl+X` / `Ctrl+V` (portapapeles del sistema) |

Directorio como argumento arranca con el árbol visible y enfocado; sin
argumento, sobre el directorio actual.

## Configuración

- **`~/.tcode/config.json`** — por la ventana `Ctrl+P` o a mano: `IndentUnit`,
  `WordWrap`, `ExplorerWidth`, `Theme` (`light`, `dark`, `light-hc`,
  `dark-hc`, `tokyo-night`, `dracula`; vacío = Custom). Detalles en
  [`docs/config.md`](docs/config.md).
- **`~/.tcode/theme.json`** — re-mapea colores por rol. Detalles en
  [`docs/editor-theme.md`](docs/editor-theme.md).

## Arquitectura

MVC con vista compuesta por el controlador:

```
internal/model       Piece Table + mmap + workspace (los datos)
internal/view        EditorView, árbol, pestañas, ventana de config, temas
internal/controller  App: eventos, composición de panes, persistencia
internal/ext         Extensiones (comandos + keybindings)
```

Los archivos abiertos se leen por `mmap` (sin cargar en RAM) y toda edición
pasa por la `Piece Table`: ver [`docs/constitution.md`](docs/constitution.md).

## Documentación

- [`docs/constitution.md`](docs/constitution.md) — arquitectura y convenciones
- [`docs/config.md`](docs/config.md) — configuración del editor
- [`docs/editor-theme.md`](docs/editor-theme.md) — paletas y tema del usuario
- [`docs/extension-system.md`](docs/extension-system.md) — extensiones
- [`docs/memory.md`](docs/memory.md) — historia y decisiones del proyecto

## Desarrollo

```bash
go test ./...        # suite (la rama corre en cualquier SO, Go 1.25+)
go vet ./...
```

Convenciones del repo: rama por feature en `feat/*`, merges a `main` por
pedido del autor, commits en inglés con la convención del autor (`agents.md`
sección 4: el agente commitea al terminar y no hace push salvo pedido).