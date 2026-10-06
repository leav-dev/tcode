# tcode

Editor de texto en la terminal, escrito en Go. Rápido, con piezas por
documento (`Piece Table` sobre `mmap`), explorador de archivos, pestañas,
resaltado de sintaxis, temas y configuración desde una ventana flotante.

## Características

| Área | Qué incluye |
| --- | --- |
| Edición | Undo/redo, auto-indent, salto de palabra visual, guardado con detección de cambios externos y recarga segura, `Ctrl+S` con confirmación de pisado |
| Explorador de archivos | Árbol lateral con lazy loading (los directorios se leen al expandir), reveal del archivo activo, creación de archivos y carpetas (`Ctrl+N` / `Ctrl+Shift+N`, o los botones del pie del panel) y borrado del nodo del cursor (`Delete`, con confirmación), archivos ocultos fuera del árbol |
| Selección | Por teclado (`Shift`+flechas/Home/End, `Ctrl+A`) y por mouse (arrastrar, `Shift`+clic extiende); `Ctrl+C`/`Ctrl+X`/`Ctrl+V` con el portapapeles del sistema |
| Pestañas | Fila de pestañas, menú (`Ctrl+T`), cambio rápido desde teclado, reabrir y deduplicar rutas |
| Apariencia | 7 paletas (Light, Dark, Light/Dark HC, Tokyo Night, Dracula, Mocha) con fondo propio, resaltado por rol para Go, Python, JS/TS, C-like y JSON, tema del usuario por JSON |
| Configuración | Ventana flotante (`Ctrl+P`): tamaño de tab, salto de palabra, ancho del panel, tema — cambios en vivo y persistidos en `~/.tcode/config.json` |
| Extensiones | Manifiestos JSON declarativos (comandos, keybindings, hooks) con **backend de scripting Lua** (comandos con implementación propia), **sistema de proveedores**: se instalan por id desde un catálogo (el oficial o el que agregues) con `tcode --install-extension <id>` (`docs/extension-system.md`) |
| Portabilidad | Go puro (`tcell`), sin dependencias nativas; instalador multi-SO en camino |

## Instalación

### Go install (un solo comando)

La vía más corta (requiere Go 1.25+):

```bash
go install github.com/leav-dev/tcode@latest
```

El binario queda en `$(go env GOPATH)/bin` (en tu caso ya está en el PATH).
Para actualizar, se repite el mismo comando.

### Homebrew (macOS / Linux)

La vía recomendada para macOS y Linux con Homebrew:

```bash
brew tap leav-dev/tcode
brew install tcode
```

Para actualizar:

```bash
brew update
brew upgrade tcode
```

### Desde fuente

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

### Para desarrollo: que `tcode` compile solo si hace falta

Ojo con una trampa: ese enlace apunta a un **artefacto compilado**, así que
cambiar un `.go` **no** cambia el binario hasta que corras `go build`. Es la
causa más común de "mi cambio no aparece". Para no acordarte, el repo trae
`scripts/tcode-dev.sh`: un wrapper que reconstruye si alguna fuente es más nueva
que el binario y después lo ejecuta.

```bash
ln -sfn "$PWD/scripts/tcode-dev.sh" ~/.local/bin/tcode
```

El entry del PATH pasa a ser un enlace al **script**, no al binario. El script
resuelve el repo desde su propia ubicación (siguiendo symlinks) y conserva tu
directorio de trabajo, así `tcode` sin argumentos abre la carpeta donde estás
parado. Sin cambios en las fuentes solo corre un `find` (milisegundos); con
fuentes nuevas compila antes de abrir y avisa por stderr.

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
| Crear archivo / carpeta | `Ctrl+N` / `Ctrl+Shift+N`, o los botones `+ Archivo` / `+ Carpeta` del pie del explorador (en la carpeta del cursor, o en la raíz) |
| Borrar archivo / carpeta | `Delete` o `Backspace` sobre el nodo del cursor, con confirmación (`s` = sí, `N` por omisión); una carpeta se borra con todo su contenido, sin papelera |
| Mostrar el explorador | `Ctrl+B` |
| Cerrar el editor | Doble `Esc` rápido (la única salida; `Ctrl+C` no cierra) |
| Cambiar pestaña | `Ctrl+PageDown` / `Ctrl+PageUp` · `Ctrl+K` (siguiente) / `Ctrl+L` (anterior) |
| Menú de pestañas | `Ctrl+T` |
| Configuración | `Ctrl+P` |
| Gestionar extensiones | `Ctrl+P` → fila **Extensiones**: ventana flotante con pestañas (instaladas, actualizables, disponibles, proveedores). `Left`/`Right` cambia de pestaña, `Enter` instala / actualiza / borra la fila del cursor —con confirmación— y desde la pestaña de proveedores se agrega una fuente |
| Salto de palabra | `Ctrl+Shift+W` |
| Deshacer / rehacer | `Ctrl+Z` / `Ctrl+Y` o `Ctrl+Shift+Z` |
| Instalar una extensión por id | `tcode --install-extension tcode.vimlite` |
| Agregar una fuente de extensiones | `tcode --add-provider <url-git\|carpeta>` (luego `tcode --approve-provider <nombre>`) |
| Ver / borrar extensiones | `tcode --list-extensions` · `tcode --remove-extension <proveedor:id\|id>` |

Al arrancar, `tcode` revisa las extensiones **en segundo plano**: en cuanto el
editor abre, una goroutine lee el catálogo de cada proveedor (comparte la versión
de cada extensión instalada con la que declara su proveedor, y compara el
catálogo con lo que tenés instalado). El arranque **no espera** esa lectura, así
que se puede escribir mientras corre.

Cuando llega, si hay actualizaciones o novedades, la barra de estado lo avisa
—sin prompt, sin bloquear, sin aplicar nada por su cuenta—:

```
2 actualizaciones, 1 novedad — Ctrl+P → Extensiones
```

| Seleccionar | `Shift`+flechas / `Ctrl+A` todo · arrastrar con el mouse · `Shift`+clic extiende |
| Copiar / cortar / pegar | `Ctrl+C` / `Ctrl+X` / `Ctrl+V` (portapapeles del sistema) |

De ahí se gestiona todo: la ventana de extensiones abre **instantánea** con los
datos ya cacheados (si la lectura todavía no terminó, muestra `cargando…` y se
rellena sola), y `Enter` sobre la fila instala, actualiza o borra con su
confirmación. La tecla `r` relanza la validación en segundo plano sin cerrar
la ventana: al terminar, un toast avisa lo que hay (o que está todo al día). La ventana no vuelve a leer los proveedores: tras una acción solo
relee la lista local de instaladas y vuelve a derivar las dos listas del mismo
catálogo, así que no se paga de nuevo la lectura de red. Instalar o actualizar
corre en **segundo plano**: el pedido se cierra al confirmar y podés seguir
escribiendo mientras git clona; al terminar, un toast avisa éxito o error y la
ventana se repinta sola. Solo corre una instalación a la vez: si hay otra en
curso, la ventana lo dice en la barra. La extensión instalada o
actualizada entra en la sesión en el acto (sus comandos y keybindings quedan
registrados al instante, con el código nuevo).

**Actualizaciones.** Aplicarlas es la acción de la fila en la pestaña
*actualizables*: `tcode` vuelve a leer el proveedor del que salió la extensión y
reemplaza la instalación por la versión nueva. Solo se revisan las que cambiaron
de versión, así que no baja código que no haga falta, y no hay que reinstalar ni
volver a aprobar el proveedor.

Dos cosas para saber: la comparación es por **versión**, así que si el autor de
la extensión publica un cambio sin subir el campo `version` del manifest, el
editor no lo detecta (reinstalar a mano con `--install-extension` lo trae); y
un proveedor caído o inalcanzable no impide arrancar —se avisa en la barra y el
editor abre igual— ni impide revisar los demás proveedores.

**Novedades.** Las extensiones que un proveedor ofrece y no tenés instaladas
aparecen en la pestaña *disponibles*. Instalar una es continuar la confianza que
ya se le dio a esa fuente, así que no vuelve a preguntar; las de un proveedor
**sin aprobar** no se ofrecen ni se ocultan: la fila lo dice al lado y hay que
aprobarla primero por terminal.

```
tcode --approve-provider mios
```

Directorio como argumento arranca con el árbol visible y enfocado; sinDirectorio como argumento arranca con el árbol visible y enfocado; sin
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