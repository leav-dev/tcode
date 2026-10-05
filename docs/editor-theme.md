# Tema del editor

tcode dibuja con una **paleta por rol** (un `Theme`). La paleta por defecto es
estilo oscuro tipo VSCode Dark+; un archivo JSON del usuario la reemplaza sin
tocar código.

## Paletas integradas (selector de la ventana de configuración)

Desde la ventana flotante de configuración (**`Ctrl+P`**, fila *Theme*) se
cicla entre seis paletas; los cambios aplican **en vivo** (todas las vistas,
los editores abiertos incluidos) y persisten en `~/.tcode/config.json`:

| id | Nombre en el selector | Estilo |
| --- | --- | --- |
| `dark` | Dark | El default actual (VSCode Dark+) |
| `light` | Light | Claro tipo VSCode Light |
| `light-hc` | Light HC | Claro de alto contraste (estilo High Contrast) |
| `dark-hc` | Dark HC | Oscuro de alto contraste |
| `tokyo-night` | Tokyo Night | Paleta Tokyo Night |
| `dracula` | Dracula | Paleta Dracula |

La opción **Custom** aplica el `theme.json` del usuario (ver abajo); con el
selector sin tocar, es el comportamiento original.

## El fondo del documento es del tema, no de la terminal

Cada paleta integrada pinta su **propio fondo de documento** (`Text` lleva su
par fg/fondo, y `StyleForRole` lo propaga a la sintaxis; `TabIdle` comparte el
par y el editor rellena su viewport): el tema `Light` se ve claro aunque tu
terminal sea negra, y Tokyo Night/Dracula no dependen del fondo del emulador.

Excepción documentada: el tema **Custom** —o cualquier tema sin fondo—
heredan el par del tema base o, si tu `theme.json` define `"text":
"default"`, vuelven a depender de los colores default de la terminal (el
comportamiento original). El esquema `theme.json` no tiene clave de fondo.

## El archivo

`~/.tcode/theme.json` mapea nombres de rol a colores:

```json
{
  "text": "default",
  "cursorLine": "236",
  "tabActive": "45",
  "treeCursor": "237",
  "keyword": "#569cd6",
  "comment": "244",
  "string": "173",
  "number": "114",
  "status": "white",
  "message": "220"
}
```

- **Colores**: nombre tcell (`red`, `navy`, `gray`…), hex (`#rrggbb`) o índice
  ANSI (`0`–`255`).
- Un rol **ausente o con valor inválido conserva su default**; un JSON roto
  devuelve la paleta completa por defecto. El tema del usuario jamás rompe el
  editor.

## Roles

| Rol | Elemento |
| --- | --- |
| `text` | Texto del documento sin resaltar |
| `cursorLine` | Fondo de la línea del cursor (un color) |
| `tabActive` / `tabIdle` | Pestaña activa / inactivas |
| `treeCursor` | Fondo de la fila activa del árbol (un color, como `cursorLine`) |
| `status` / `message` | Barra de estado / mensaje transitorio |
| `modified` | Marca de documento sucio *(reservado)* |
| `comment`, `keyword`, `string`, `number`, `type`, `function`, `variable`, `punct` | Roles de sintaxis |

`tabActive` conserva el atributo `Reverse` además del color de acento: la
noción de "seleccionado" nunca depende solo del color de la terminal.

`treeCursor` es un color de **fondo** —como `cursorLine`—: la fila activa del
árbol lleva esa barra de selección a todo el ancho y el texto en el color por
defecto, que se adapta a terminales claras y oscuras. Así la selección nunca
cae en un color invisible aunque el tema cargue un índice oscuro (el default es
blanco sobre azul oscuro, el acento de la barra de estado).

## Resaltado de sintaxis

El documento se resalta **por línea visible y sin estado entre líneas**, según
la extensión del archivo: palabras clave, comentarios (`//` y `#`), bloques
`/* */` dentro de la línea, strings (`"`, `'`), números (decimal y hex), y los
roles **`type`** (tipos y primitivas: `int`, `string`, `float64`, `bool`…),
**`function`** (un identificador seguido de `(` pegado) y **`variable`** (todo
identificador que no sea keyword, tipo ni función), para `.go`, `.py`,
`.js`/`.ts`, c-like y `.json`.

Los tipos viven en sets por lenguaje (`types` en `highlight.go`); los
identificadores que no caen en ninguno de los otros roles son variables —también
en archivos sin lenguaje reconocido—. La función se detecta por el `(` **pegado**
(`foo(`); la variante con espacio (`foo (x)`) queda como variable: límite
documentado de la regla mecánica.

**Límites documentados:** un comentario `/*` o un string sin cerrar colorean
solo su línea (no se arrastra el estado a la siguiente); `#` de Python se toma
como comentario aunque esté en un string (regla mecánica); un rol inválido en
`theme.json` conserva el default. Reciente y a propósito: las reglas de Python
con `:` del auto-indent y el resto son de la unidad de indentación, no del
tema.

## Roadmap

- **Temas por proyecto** (`.tcode/theme.json` de la sesión con precedencia) y
  un commando para recargar sin reiniciar.
- **Highlighter con estado multilínea** (strings largos, comentarios anidados)
  — requiere un lexer por buffer, la siguiente capa.
- **Rol `modified`** aplicado a la marca `[+]` en la barra de estado.

En el código: `Theme`/`LoadTheme` en `internal/view/theme.go`, el highlighter en
`internal/view/highlight.go`, y cada componente consume `themeOr(...)` — el
único punto donde un rol se vuelve color es `Theme.StyleForRole`.