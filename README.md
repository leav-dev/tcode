# tcode

Editor de texto en la terminal, escrito en Go. Rápido, con piezas por
documento (`Piece Table` sobre `mmap`), explorador de archivos, pestañas,
resaltado de sintaxis, temas y configuración desde una ventana flotante.

## Características

| Área | Qué incluye |
| --- | --- |
| Edición | Undo/redo, auto-indent, salto de palabra visual, guardado con detección de cambios externos y recarga segura, `Ctrl+S` con confirmación de pisado |
| Explorador de archivos | Árbol lateral con lazy loading (los directorios se leen al expandir), reveal del archivo activo, creación de archivos y carpetas (`Ctrl+N` / `Ctrl+Shift+N`), archivos ocultos fuera del árbol |
| Pestañas | Fila de pestañas, menú (`Ctrl+T`), cambio rápido desde teclado, reabrir y deduplicar rutas |
| Apariencia | 6 paletas (Light, Dark, Light/Dark HC, Tokyo Night, Dracula) con fondo propio, resaltado por rol para Go, Python, JS/TS, C-like y JSON, tema del usuario por JSON |
| Configuración | Ventana flotante (`Ctrl+P`): tamaño de tab, salto de palabra, ancho del panel, tema — cambios en vivo y persistidos en `~/.tcode/config.json` |
| Extensiones | Manifiestos JSON declarativos (comandos, keybindings, hooks) con **backend de scripting Lua** (comandos con implementación propia), **sistema de proveedores**: se instalan por id desde un catálogo (el oficial o el que agregues) con `tcode --install-extension <id>` (`docs/extension-system.md`) |
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

Para armarte tus propios binarios de Linux y macOS sin depender de un release,
ver [Compilar para Linux y macOS](#compilar-para-linux-y-macos).

### Dejarlo disponible como `tcode`

El shell resuelve un nombre suelto (`tcode`) buscándolo en los directorios de
`$PATH`, de izquierda a derecha. Para invocarlo desde cualquier carpeta, el
binario tiene que estar en uno de esos directorios (o llegar por un enlace
desde uno).

En Ubuntu/Debian `~/.local/bin` ya viene en el `PATH` del usuario, así que
alcanza con un enlace simbólico: sin `sudo` y sin editar ningún archivo de
configuración.

```bash
mkdir -p ~/.local/bin
ln -sfn "$PWD/dist/tcode-linux-amd64" ~/.local/bin/tcode

command -v tcode            # /home/<usuario>/.local/bin/tcode
tcode --list-extensions     # verifica que resuelve y ejecuta
```

Es un **enlace y no una copia** a propósito: cada `go build ... -o
dist/tcode-linux-amd64` queda activo al instante, sin reinstalar nada. El precio
es que el comando depende de que el repo siga en su lugar; si lo movés, volvé a
correr el `ln -sfn`. Si preferís una versión congelada e independiente del repo,
copiá el binario en vez de enlazarlo (`cp dist/tcode-linux-amd64
~/.local/bin/tcode`), a costa de repetir la copia en cada recompilación.

En otros SO vale la misma idea: `~/.local/bin` es la convención XDG, y en macOS
lo habitual es `/usr/local/bin`.

## Compilar para Linux y macOS

tcode es Go puro (`tcell`, sin `import "C"` ni build tags por plataforma), así
que la compilación cruzada no necesita toolchain extra: la cadena de Go ya
genera los cuatro binarios desde cualquier SO anfitrión.

```bash
go version                      # hace falta Go 1.25+
mkdir -p dist

# Linux
CGO_ENABLED=0 GOOS=linux  GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/tcode-linux-amd64 .
CGO_ENABLED=0 GOOS=linux  GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o dist/tcode-linux-arm64 .

# macOS
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o dist/tcode-darwin-arm64 .
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/tcode-darwin-amd64 .
```

`dist/` ya está en `.gitignore`: los binarios se generan, no se versionan.

| Objetivo | `GOOS` / `GOARCH` | Para qué máquina |
| --- | --- | --- |
| `tcode-linux-amd64` | `linux` / `amd64` | PC/server Linux x86-64 |
| `tcode-linux-arm64` | `linux` / `arm64` | Linux ARM64 (Raspberry Pi, ARM en la nube) |
| `tcode-darwin-arm64` | `darwin` / `arm64` | Mac con Apple Silicon (M1/M2/M3/M4) |
| `tcode-darwin-amd64` | `darwin` / `amd64` | Mac con Intel |

### Qué hace cada flag

- **`CGO_ENABLED=0`** — deja los binarios estáticos y sin dependencias del
  sistema. Hoy el proyecto no usa cgo, pero el flag fija esa garantía: si
  mañana alguien agrega una dependencia con cgo, el build falla en vez de
  producir un binario atado a las librerías del anfitrión.
- **`-trimpath`** — saca las rutas absolutas de tu máquina del binario
  (reproducibilidad; no filtra tu `$HOME` al distribuir).
- **`-ldflags="-s -w"`** — omite la tabla de símbolos y la info de debug. En
  este repo baja de ~6,4 MB a ~4,4 MB por binario. Si necesitás depurar con
  `dlv` o leer panics con líneas, sacá este flag.

### Verificar los binarios

```bash
file dist/*        # cada uno debe reportar su arquitectura correcta
```

Esperás `ELF 64-bit LSB executable, x86-64` (Linux amd64),
`ELF 64-bit LSB executable, ARM aarch64` (Linux arm64), y
`Mach-O 64-bit arm64 executable` / `Mach-O 64-bit x86_64 executable` (macOS).

El binario del SO anfitrión se puede probar en el lugar:

```bash
./dist/tcode-linux-amd64 --list-extensions   # sale sin abrir la UI
```

Los binarios de otro SO **no** se pueden ejecutar ni testear en el anfitrión:
`go test ./...` solo corre compilando para la plataforma local. Los `.dmg`,
`.deb` o tarballs de release quedan fuera de este flujo.

### Notas de macOS

- **En Apple Silicon usá el `arm64`.** El `darwin-amd64` corre bajo Rosetta 2
  y solo si Rosetta está instalada; no hay motivo para bajar a `x86_64` en un
  Mac ARM.
- **Firma y cuarentena.** El linker de Go le pone firma ad-hoc al binario
  `darwin/arm64`, así que arranca sin certificado de Apple. Pero si el archivo
  llega descargado, por AirDrop o por carpeta compartida, macOS le agrega el
  atributo de cuarentena y Gatekeeper lo bloquea con *"no se puede abrir porque
  proviene de un desarrollador no identificado"*. Se destraba con:

  ```bash
  xattr -d com.apple.quarantine ./tcode-darwin-arm64
  chmod +x ./tcode-darwin-arm64
  ```

  (o clic derecho → Abrir la primera vez). Esto es solo para tu uso local y
  para pruebas; distribuir binarios a terceros sin notarización es otra
  historia y necesita cuenta de desarrollador de Apple.
- **Terminal:** `tcell` ya trae la info de terminales comunes; con
  `TERM=xterm-256color` no hace falta configurar nada.

## Uso rápido

| Querés… | Hacé |
| --- | --- |
| Abrir archivo | `tcode main.go` · abrir dir: `tcode ./proyecto` |
| Guardar / Guardar como | `Ctrl+S` / `Ctrl+Shift+S` |
| Crear archivo / carpeta | `Ctrl+N` / `Ctrl+Shift+N` (en la carpeta del cursor, o en la raíz) |
| Mostrar el explorador | `Ctrl+B` |
| Cerrar el editor | Doble `Esc` rápido (la única salida; `Ctrl+C` no cierra) |
| Cambiar pestaña | `Ctrl+PageDown` / `Ctrl+PageUp` · `Ctrl+K` (siguiente) / `Ctrl+L` (anterior) |
| Menú de pestañas | `Ctrl+T` |
| Configuración | `Ctrl+P` |
| Salto de palabra | `Ctrl+Shift+W` |
| Deshacer / rehacer | `Ctrl+Z` / `Ctrl+Y` o `Ctrl+Shift+Z` |
| Instalar una extensión por id | `tcode --install-extension tcode.vimlite` |
| Agregar una fuente de extensiones | `tcode --add-provider <url-git\|carpeta>` (luego `tcode --approve-provider <nombre>`) |
| Ver / borrar extensiones | `tcode --list-extensions` · `tcode --remove-extension <proveedor:id\|id>` |

Al arrancar, `tcode` revisa las extensiones y **te pregunta antes de tocar nada**:
compara la versión de cada extensión instalada con la que declara su proveedor,
y compara el catálogo de cada proveedor con lo que tenés instalado. Si hay
actualizaciones o novedades, aparece un prompt en la barra de estado con el
detalle y solo se aplican si respondés `s` (o `y`):

```
Aplicar 1 actualización y 2 novedades? [s/N] s
1 actualización aplicada, 2 novedades instaladas
```

El default es **no**: cualquier otra tecla —incluido `Enter`— omite todo por
esta vez, sin tocar el disco. Denegar no es "nunca más": el próximo arranque
vuelve a preguntar. Sin cambios no hay nada que preguntar y el editor abre
derecho.

**Actualizaciones.** Cuando respondés `s`, `tcode` vuelve a leer cada proveedor
del que salió una extensión instalada y reemplaza la instalación por la versión
nueva antes de cargar las extensiones (no hay que reinstalar ni volver a
aprobar el proveedor). Solo se revisan las que cambiaron de versión, así que no
baja código que no haga falta.

Dos cosas para saber: la comparación es por **versión**, así que si el autor de
la extensión publica un cambio sin subir el campo `version` del manifest, el
editor no lo detecta (reinstalar a mano con `--install-extension` lo trae); y
un proveedor caído o inalcanzable no impide arrancar —se avisa en la barra y el
editor abre igual— ni impide revisar los demás proveedores.

**Novedades.** Las extensiones que un proveedor ofrece y no tenés instaladas se
revisan en la misma pregunta y, si aceptás, se instalan —en el mismo arranque—
cuando vienen de un proveedor **aprobado**. Instalar una extensión es continuar
la confianza que ya se le dio a esa fuente, así que no vuelve a preguntar.

Las novedades de un proveedor **sin aprobar** no se instalan ni se ocultan:
aceptando, la barra las nombra agrupadas por proveedor y quedan a la espera de
`tcode --approve-provider <nombre>`:

```
1 actualización aplicada, 0 novedades instaladas — sin aprobar: mios (tcode.mioformato, tcode.miolinter)
```

Directorio como argumento arranca con el árbol visible y enfocado; sin
argumento, sobre el directorio actual.

## Configuración

- **`~/.tcode/config.json`** — por la ventana `Ctrl+P` o a mano: `IndentUnit`,
  `WordWrap`, `ExplorerWidth`, `Theme` (`light`, `dark`, `light-hc`,
  `dark-hc`, `tokyo-night`, `dracula`; vacío = Custom). Detalles en
  [`docs/config.md`](docs/config.md).
- **`~/.tcode/theme.json`** — re-mapea colores por rol. Detalles en
  [`docs/editor-theme.md`](docs/editor-theme.md).
- **`~/.tcode/providers.json`** — las fuentes de extensiones agregadas con
  `tcode --add-provider` (el proveedor por defecto es built-in y no aparece
  acá). Listar un proveedor no baja código, solo sus manifests. Detalles en
  [`docs/extension-system.md`](docs/extension-system.md).

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