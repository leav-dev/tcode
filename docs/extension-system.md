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
  mi-ext/
    extension.json
```

Una carpeta sin `extension.json` se ignora en silencio. Una extensión rota
(JSON inválido o manifest que no valida) **nunca impide el arranque**: su error
se avisa una vez en la barra de estado y el resto se carga igual.

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
| `tcode.message(msg)` | Muestra un mensaje en la barra de estado. |

Errores del script (Lua o del puente) → mensaje en la barra de estado, nunca
rompen el editor. El runtime solo abre las librerías base/tabla/string/math
(nada de `os` ni `io` del host): el repositorio del autor es confiado, pero el
host no se expone más de lo necesario. Sin `onDidChangeText` por diseño
(deuda de rendimiento ya documentada); los hooks (`onDidSaveBuffer`, …) ya
fluyen al comando con `fn`.

## Instalar extensiones

Las extensiones del **autor** (repositorios propios, considerados confiados) se
instalan desde la línea de comandos, sin abrir el editor:

| Comando | Efecto |
| --- | --- |
| `tcode --install-extension <url-git>` | Clona el repo (`git clone --depth 1`), valida su `extension.json` y lo despliega en `~/.tcode/extensions/<id>/`. Reinstalar reemplaza. |

> El comando clásico sin `script`/`fn` sigue siendo un stub "sin implementación":
> dale lógica declarándole la función Lua (sección de arriba).
| `tcode --list-extensions` | Lista las instaladas del usuario (id, nombre, versión). |
| `tcode --remove-extension <id>` | Borra `~/.tcode/extensions/<id>`. |

El repositorio de la extensión debe tener **`extension.json` en su raíz** (la
misma estructura de carpeta de arriba). Un manifest inválido se rechaza sin
tocar nada; el `id` del manifest da nombre a la carpeta instalada. La
extensión queda disponible en la **próxima sesión** (no hay recarga en
caliente).

**Modelo de confianza:** los repositorios del propio autor se consideran
seguros por definición — no hay marketplace, checksum ni firma; la barrera es
estructural (el manifest valida).

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