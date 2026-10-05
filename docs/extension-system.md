# Extensiones en tcode

El sistema de extensiones de tcode sigue el **modelo de VSCode pero nativo**:
una extensión es una carpeta con un `extension.json` que declara lo que
contribuye —comandos, re-mapeos de teclado y hooks sobre eventos de buffer— sin
ejecutar código. Es el milestone declarativo: las costuras (registro de
comandos, bus de hooks, activación) son exactamente donde un futuro backend de
scripting (WASM o Lua) se enchufará para que las extensiones corran código.

## Dónde viven las extensiones

El editor las busca al arrancar en dos raíces, en este orden (la primera gana
ante ids duplicados):

1. Las del **proyecto**: `<raíz de la sesión>/.tcode/extensions/`.
2. Las del **usuario**: `~/.tcode/extensions/`.

El árbol esperado por extensión:

```
~/.tcode/extensions/
  mi-ext/                # instalación heredada (sin proveedor)
    extension.json
  <proveedor>/           # instalación por proveedor (layout actual)
    mi-ext/
      extension.json
```

Una carpeta sin `extension.json` se ignora en silencio (salvo que contenga
extensiones namespadas dentro, que sí se reportan con su proveedor). Una
extensión rota (JSON inválido o manifest que no valida) **nunca impide el
arranque**: su error se avisa una vez en la barra de estado y el resto se carga
igual.

> **Resuelto:** el arranque descubre ambos layouts. `Discover` escanea dos
> profundidades — `<root>/<id>/extension.json` (plano, raíz de proyecto) y
> `<root>/<proveedor>/<id>/extension.json` (namespaced, raíz de usuario) — así
> que una extensión instalada por proveedor se carga al abrir el editor.

## Backend de scripting (Lua)

Desde el hito 1, un comando declarado puede delegar su implementación a una
**función Lua** del script de la extensión (gopher-lua embebido, sin cgo). El
manifest agrega `script` (ruta relativa al dir de la extensión) y `fn` (función
global) — los dos juntos:

```json
"contributes": {
  "commands": [
    { "id": "autor.mi-ext.hola", "title": "Hola", "script": "main.lua", "fn": "hola" }
  ]
}
```

```lua
function hola()
  tcode.insert("HOLA")
  tcode.message("hola desde Lua")
end
```

El comando se sigue declarando igual (keybindings y hooks lo referencian de
siempre); solo cambia la implementación. El host expone la tabla global
`tcode`:

| Función | Qué hace |
| --- | --- |
| `tcode.command(id)` | Ejecuta un comando registrado (`tcode.*` u otros). Un script que se llama a sí mismo corta con un guard de recursión (máx 8). |
| `tcode.buffer()` | Devuelve `{path, content, ok}` del buffer activo (`ok=false` sin buffer). `content` es el documento completo (límite del hito 1). |
| `tcode.insert(text)` | Inserta `text` en la posición del cursor del buffer activo. |
| `tcode.lineCount()` / `tcode.line(n)` | Líneas del buffer activo y su contenido por línea (`n` 1-indexado; errores "sin buffer activo"/"línea fuera de rango"). |
| `tcode.diagnostics.set(lista)` / `tcode.diagnostics.clear()` | Reemplaza las **anotaciones** del buffer activo: lista de `{line, message, severity}` (severidad `error`\|`warning`\|`info`, default `error`; un elemento inválido aborta todo). |
| `tcode.message(msg)` | Muestra un mensaje en la barra de estado. |

### Ejemplo: un mini-linter con diagnostics

`extension.json` con el comando `linter.marcar` (script+fn), keybinding y hook
`onDidSaveBuffer` → el script marca las líneas con `TODO`:

```lua
function marcar()
  local n = tcode.lineCount()
  local diags = {}
  for i = 1, n do
    if string.find(tcode.line(i), "TODO") then
      table.insert(diags, { line = i, message = "todo pendiente", severity = "warning" })
    end
  end
  tcode.diagnostics.set(diags)
end
```

El editor pinta el diagnóstico en el **gutter** (marcador `!`/`?`/`i` según la
severidad), subraya la línea anotada y muestra el mensaje en la barra de
estado cuando el cursor está sobre ella. Límite del hito: las anotaciones van
al **buffer activo** del momento; el hook `onDidSaveBuffer` anota el activo,
no necesariamente el guardado.

Errores del script (Lua o del puente) → mensaje en la barra de estado, nunca
rompen el editor. El runtime solo abre las librerías base/tabla/string/math
(nada de `os` ni `io` del host): el repositorio del autor es confiado, pero el
host no se expone más de lo necesario. Sin `onDidChangeText` por diseño
(deuda de rendimiento ya documentada); los hooks (`onDidSaveBuffer`, …) ya
fluyen al comando con `fn`.

## Instalar extensiones

Las extensiones se instalan por **id**, resolviendo contra los **proveedores**
(sección siguiente). Desde la línea de comandos, sin abrir el editor:

| Comando | Efecto |
| --- | --- |
| `tcode --add-provider <url-git\|carpeta>` | Valida la fuente, le deriva un nombre y la registra en `~/.tcode/providers.json` **sin aprobar**. |
| `tcode --install-extension <id>` | Resuelve el id en los proveedores (gana el primero) e instala en `~/.tcode/extensions/<proveedor>/<id>/`. |
| `tcode --approve-provider <nombre>` | Aprueba un proveedor: a partir de ahí instalar desde él no pregunta. |
| `tcode --list-extensions` | Lista las instaladas como `proveedor/id (nombre) v<versión>`. |
| `tcode --remove-extension <proveedor:id\|id>` | Borra una extensión; con id suelto la busca en todos los proveedores. |

Reinstalar la misma extensión reemplaza el árbol anterior (iterar sobre la
propia extensión es el flujo normal). Un manifest inválido se rechaza sin tocar
nada. La extensión queda disponible en la **próxima sesión** (no hay recarga
en caliente).

### El prompt del arranque: actualizaciones y novedades

Al abrir el editor, `tcode` revisa **antes** de cargar las extensiones y, si
encuentra algo, **pregunta**:

```
Aplicar 1 actualización y 2 novedades? [s/N]
```

- **Qué cubre**: las dos cosas a la vez. `ext.CheckUpdates` detecta las
  extensiones instaladas cuyo proveedor declara otra versión (sin tocar el
  disco) y `ext.AvailableExtensions` detecta las que el proveedor ofrece y no
  están instaladas. El usuario revisa todo y decide una vez.
- **Cómo se responde**: `s` (o `y`) aplica; cualquier otra tecla —`Enter`,
  `Escape`, `n`— omite. El default es **NO**, así que nada se instala sin un sí
  explícito. El prompt vive en la barra de estado y se atiende con un **loop
  anidado de eventos** (como el menú de configuración, pero antes del loop
  principal).
- **Denegar es por esta vez**: la decisión no se persiste, así que el próximo
  arranque vuelve a preguntar.
- **Qué pasa al aceptar**: se aplican las actualizaciones (`ext.UpdateAll`) y
  las novedades de los proveedores **aprobados** (`ext.InstallAvailable`), y
  el mismo arranque las carga: el prompt va **antes** de `loadExtensions` a
  propósito, porque `Manager.AddExtensions` no es idempotente (appendea a los
  estados del manager y el registro rechaza el segundo registro).
- **Novedades sin aprobar**: no se instalan (instalar desde una fuente sin
  confianza es lo que exige confirmación) y se nombran en la barra agrupadas
  por proveedor, con el comando `--approve-provider <nombre>` para destrabarlas.
- **Nunca impide arrancar**: sin cambios no hay prompt; un proveedor caído se
  avisa en la barra y el editor abre igual con lo que ya está instalado.

> El chequeo vivía en `main.go` y se ejecutaba antes de abrir la TUI, así que
> solo se veía por stdout y las extensiones se aplicaban en silencio. Hoy corre
> dentro del controller, que es donde se puede preguntar y mostrar.

> El comando clásico sin `script`/`fn` sigue siendo un stub "sin implementación":
> dale lógica declarándole la función Lua (sección de arriba).

## Proveedores de extensiones

Un **proveedor** es una fuente de extensiones: un repositorio git o una carpeta
local donde **cada subcarpeta con `extension.json`** es una extensión (un
monorepo). Los ids se resuelven contra todos los proveedores, así que el autor
puede publicar un catálogo y el usuario instalar lo que le sirva.

### El proveedor por defecto

El editor trae uno built-in, `https://github.com/leav-dev/tcode-extention`, el
monorepo oficial. Es una **constante en código**: no está en
`~/.tcode/providers.json`, siempre está **aprobado** y siempre va **primero**
en la resolución. Solo se **registra** — no instala nada por su cuenta.

| Comando | Efecto |
| --- | --- |
| `tcode --install-extension tcode.vimlite` | Instala la extensión `tcode.vimlite` del proveedor por defecto. |

### Agregar proveedores

```
tcode --add-provider https://github.com/leav-dev/mis-extensiones
tcode --add-provider /home/tcode/mis-extensiones   # carpeta local, sin git
```

Agregar **valida** la fuente (debe ofrecer al menos una extensión válida) y la
guarda en `~/.tcode/providers.json`:

```json
{
  "providers": [
    { "name": "tcode-extention", "source": "https://github.com/…", "approved": true },
    { "name": "mis-extensiones", "source": "/home/tcode/mis-extensiones", "approved": false }
  ]
}
```

- El **nombre** es la identidad con la que se namespacan las instalaciones
  (`~/.tcode/extensions/<nombre>/…`): sale del último componente de la URL git
  (`tcode-extention`) o del nombre de la carpeta local. **Riesgo conocido:**
  dos proveedores con el mismo nombre colisionan y en la resolución gana el
  primero; por eso agregar un nombre ya usado se rechaza.
- El **orden** del archivo es el **orden de resolución** (después del
  proveedor por defecto). Ante una colisión de ids gana el primero.
- Una carpeta local se persiste como **ruta absoluta**, así el config sigue
  valiendo desde cualquier directorio de trabajo.
- Un proveedor guardado con el nombre del proveedor por defecto se descarta:
  el built-in gana.

### Confianza: aprobación explícita por proveedor

Agregar un proveedor **no** es confiar en él: queda con `approved: false`, y la
primera instalación desde ahí pregunta:

```
El proveedor "mis-extensiones" no está aprobado.
fuente: https://github.com/leav-dev/mis-extensiones
¿Confías en este proveedor? [s/N]:
```

Responder que no cancela sin instalar nada; `tcode --approve-provider <nombre>`
deja de preguntar. Sin terminal (uso no interactivo) la instalación se rechaza
con un error que dice qué aprobar.

**Modelo de confianza:** no hay marketplace, checksum ni firma. La barrera es
estructural (el manifest valida) más la aprobación explícita de la fuente: el
código se copia, no se ejecuta, y solo baja cuando la extensión elegida se
instala.

### Listado liviano (no baja código)

Buscar o listar las extensiones de un proveedor **no descarga ningún `.lua`**.
Para un proveedor git se hace un **clone parcial** que trae el árbol de commits
sin ningún blob y materializa solo los manifests:

```
git clone --depth 1 --filter=blob:none --no-checkout <url> <tmp>
git -C <tmp> sparse-checkout init --no-cone
git -C <tmp> sparse-checkout set '*/extension.json'
git -C <tmp> checkout
```

Un proveedor local se escanea directo, sin git. Los `.lua` solo bajan al
**instalar** la extensión elegida, y solo los de esa extensión (un segundo
clone acotado a su subcarpeta): es el único momento en que el código va a
ejecutarse.

Optimización futura (no implementada): la API de GitHub (árbol + contents) que
listaría sin clonar nada. Es específica de GitHub y tiene rate limit, así que
hoy el camino único es git.

> Nota: `git --filter` no aplica a transportes locales (`file://`, rutas
> locales), que avisan `filtering not recognized by server`; el sparse
> checkout sigue acotando el materializado, así que el comportamiento observable
> (no bajar los `.lua`) se mantiene igual.

## El manifest (`extension.json`)

```json
{
  "id": "demo.saludo",
  "name": "Demo Saludo",
  "version": "1.0.0",
  "activation": ["onStartup"],
  "contributes": {
    "commands": [
      { "id": "demo.saludo.hola", "title": "Saludar" }
    ],
    "keybindings": [
      { "key": "ctrl+k ctrl+g", "command": "tcode.toggleExplorer" }
    ],
    "hooks": [
      { "event": "onDidSaveBuffer", "command": "demo.saludo.hola" }
    ]
  }
}
```

| Campo | Regla |
| --- | --- |
| `id` | Obligatorio. Alfanumérico inicial, luego alfanumérico, `.`, `_`, `-`. Convenio: `editor.extensión`. |
| `name` | Opcional; si falta, se adopta el `id` para los mensajes. |
| `version` | Obligatorio. Semver `X.Y.Z`. |
| `activation` | Eventos que despiertan la extensión (ver más abajo). Sin eventos, nunca se activa. |
| `contributes.commands` | Comandos declarados. Se registran como stubs: ejecutarlos activa la extensión y avisa que falta el backend de scripting. |
| `contributes.keybindings` | Tecla (o chord de dos tiempos) → comando registrado. Puede apuntar a `tcode.*`. |
| `contributes.hooks` | `evento → comando`; el comando corre cuando el editor emite el evento y la extensión está activa. |

### Eventos de activación

- `onStartup`: se activa al arrancar el editor.
- `onCommand:<id>`: se activa al ejecutar el comando `<id>`.
- `onDidOpenBuffer`: se activa la primera vez que se abre un buffer.
- `*`: se activa con el primer evento de cualquier tipo.

Los keybindings declarados **resuelven desde el arranque aunque la extensión
esté inactiva** (como VSCode); ejecutar su comando es lo que la despierta si
declara `onCommand:<id>`.

### Eventos de hook

| Evento | Cuándo se emite |
| --- | --- |
| `onDidOpenBuffer` | Al abrir un buffer (arranque con archivo o desde el explorador). |
| `onDidSaveBuffer` | Al guardar con éxito (`Ctrl+S` o Save As). Un guardado bloqueado por cambios externos no emite. |
| `onDidCloseBuffer` | Al cerrar una pestaña con `Ctrl+W`. El cierre por salida del editor no emite hooks. |

Los hooks de una extensión corren **solo si está activa**, en orden de
declaración. Un hook que falla se avisa en la barra de estado y no corta a los
demás. Un hook cuyo comando vuelve a emitir el mismo evento (p. ej.
`tcode.closeTab` dentro de un `onDidCloseBuffer`) **no recurre**: el Emit
anidado se corta con un guard, por lo que cerrar pestañas desde un hook jamás
puede desbordar la pila. Nota: `onDidChangeText` queda fuera a propósito —cada tecla es un
evento, y sin un diseño de debounce es un riesgo de rendimiento que contradice
la constitución de eficiencia—.

## Comandos built-in (`tcode.*`)

Los keybindings y hooks pueden invocar las acciones existentes del editor:

| Comando | Acción |
| --- | --- |
| `tcode.save` | Guarda el buffer activo. |
| `tcode.saveAs` | Abre el pedido de Save As. |
| `tcode.closeTab` | Cierra la pestaña activa. |
| `tcode.toggleExplorer` | Muestra/oculta el explorador. |
| `tcode.undo` / `tcode.redo` | Deshace / rehace. |
| `tcode.switchTabNext` / `tcode.switchTabPrev` | Cambia de pestaña. |

Los comandos que necesitan un buffer abierto fallan con un error legible si el
workspace está vacío.

## Referencia de keybindings

Gramática: uno o dos tiempos separados por espacio; cada tiempo son
modificadores más una tecla.

- **Modificadores:** `ctrl+`, `shift+`, `alt+`, combinables (`ctrl+shift+k`),
  sin repetir.
- **Teclas:** una letra (`k`), un dígito (`5`), una F-key (`f5` … `f24`) o una
  nombrada: `enter`, `tab`, `escape`, `space`, `backspace`, `delete`, `insert`,
  `home`, `end`, `pageup`, `pagedown`, `up`, `down`, `left`, `right`.
- **Chords de dos tiempos:** `ctrl+k ctrl+g`. Un chord exige al menos un mod en
  algún tiempo (sin eso, `k k` chocaría con el tecleo normal).

Prioridad y semántica:

1. Los atajos del núcleo (`Ctrl+B`, `Ctrl+S`, `Ctrl+T`, `Ctrl+W`, undo/redo,
   cambio de pestaña, salida) ganan siempre: una extensión no puede pisarlos.
2. Con el workspace vacío (sin buffers) los keybindings de extensión no
   disparan: solo navegación y salida.
3. A igual tecla, un single gana sobre ser prefijo de chord.
4. Un chord pendiente se cancela con cualquier otra tecla; la tecla que lo
   cancela se evalúa ella misma como evento nuevo.
5. El único estado mutable del resolver es el chord pendiente; todo lo demás es
   tablas congeladas.

## Límites y roadmap

Este milestone es **declarativo por diseño**: una extensión no ejecuta código.
Ejecutar un comando declarado sin backend produce el mensaje honesto en la
barra de estado. La evolución prevista, por la misma costura:

1. **Backend de scripting** (WASM vía wazero o Lua vía gopher-lua): el registro
   de comandos y el bus de hooks ya aceptan handlers por id; un backend solo
   necesita registrar los suyos y enganchar la activación a la carga del módulo.
2. **Command palette**: invocar cualquier comando registrado por id, sin
   depender de un keybinding.
3. **Contribuciones de lenguaje**: gramáticas, snippets y temas declarativos
   (la siguiente capa estática, sin scripts).

El costo de diseño de las costuras es cero hoy, y el modelo queda fiel al de
VSCode sin prometer lo que no ejecuta.