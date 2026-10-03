# Tema del editor

tcode dibuja con una **paleta por rol** (un `Theme`). La paleta por defecto es
estilo oscuro tipo VSCode Dark+; un archivo JSON del usuario la reemplaza sin
tocar código.

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
| `treeCursor` | Nodo activo del árbol |
| `status` / `message` | Barra de estado / mensaje transitorio |
| `modified` | Marca de documento sucio *(reservado)* |
| `comment`, `keyword`, `string`, `number`, `punct` | Roles de sintaxis |

Los roles "activos" (`tabActive`, `treeCursor`) conservan el atributo `Reverse`
además del color de acento: la noción de "seleccionado" nunca depende solo del
color de la terminal.

## Resaltado de sintaxis

El documento se resalta **por línea visible y sin estado entre líneas**, según
la extensión del archivo: palabras clave, comentarios (`//` y `#`), bloques
`/* */` dentro de la línea, strings (`"`, `'`) y números (decimal y hex) para
`.go`, `.py`, `.js`/`.ts`, c-like y `.json`. Lo que el léxico de esa línea no
cubre queda como texto.

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