# Feature: Tema del editor + sintaxis básica

## Description
El editor es monocromo: todo se dibuja con `StyleDefault` salvo el `Reverse`
de la barra de estado, la pestaña activa y el nodo del árbol. Esta unidad agrega
una **paleta por rol** (tema) configurable por archivo y un **resaltado de
sintaxis básico** en el documento.

Decisiones de producto (usuario):

- **Alcance:** tema de interfaz + resaltado básico (palabras clave, comentarios,
  strings, números) para lenguajes comunes (`.go`, `.py`, `.js`/`.ts`,
  c-like, `.json`).
- **Configurable ya**: `~/.tcode/theme.json` mapea roles → colores; si falta o
  está malformado se usa la paleta por defecto (estilo oscuro, tipo VSCode
  Dark+) y se avisa una vez en la barra de estado. Por ahora solo la raíz del
  usuario; la del proyecto y el sistema de config amplio quedan para después.

Diseño:

- **Roles** (`view/theme.go`): `Text`, `CursorLineBg`, `TabActive`, `TabIdle`,
  `TreeCursor`, `StatusBg`/`StatusFg`, `StatusMessage`, `Modified`,
  `Comment`, `Keyword`, `String`, `Number`, `Punct`. JSON:
  `{"text":"default","cursorLine":"234","keyword":"#569cd6",...}` — colores por
  nombre tcell (`red`, `navy`…), hex (`#569cd6`) o índice ANSI (`0`–`255`).
- **Highlight** (`view/highlight.go`): reglas por extensión, mecánicas, por
  LÍNEA visible (sin estado entre líneas: un comentario `/*` abierto o un
  string sin cerrar colorean el resto de su línea y se documenta como límite).
  El detectro de lenguaje usa la extensión del `Path()` del buffer.
- **Aplicación**: el editor pinta la fila del cursor con `CursorLineBg` antes
  del texto y dibuja cada cluster con el estilo de su token; la tab activa, el
  nodo activo del árbol, la barra de estado y los mensajes usan el tema.

Qué NO cambia: la semántica de edición, el layout, los tests que comparan
runas (la pantalla sigue siendo la misma; solo cambian los estilos por celda).

## Tasks
- [ ] `view/theme.go`: `Theme` + roles + paleta default + parse de JSON y de
  colores (nombre/hex/índice); fallback default ante error <!-- id: 0 -->
- [ ] Tests de tema: parse válido, inválido → default, cada formato de color
  <!-- id: 1 -->
- [ ] `view/highlight.go`: reglas por extensión (keywords, comentario `//` `#`
  `/*`…, strings `"` `'` `` ` `` , números) y `lineStyleAt(pos)` por byte;
  tokens de una línea, sin estado <!-- id: 2 -->
- [ ] Tests de highlight: keyword/string/comment/number por lenguaje, límites
  de la línea, extensión desconocida = solo texto <!-- id: 3 -->
- [ ] Editor: `Draw` pinta la línea del cursor con su fondo y estila los
  clusters por token <!-- id: 4 -->
- [ ] Tab/status/tree/menu: estilos del tema <!-- id: 5 -->
- [ ] Controller: carga `~/.tcode/theme.json` al arranque (fallback sin romper,
  aviso único); tests <!-- id: 6 -->
- [ ] Integración: render con estilos verificados vía `GetContents`; suite
  completa sin fallos nuevos <!-- id: 7 -->
- [ ] `docs/editor-theme.md`: formato del JSON, roles, límites del resaltado
  y roadmap (temas por proyecto, highlighter con estado multilínea)
  <!-- id: 8 -->

## Design decisions

### Un tema, roles, y fallback silencioso
El tema es una `Theme` (struct) con estilos por rol; la paleta default vive en
código y un JSON opcional la reemplaza. El editor nunca depende del archivo:
si falta o está roto, el default funciona y el aviso llega una vez a la barra.
Es la misma filosofía que las extensiones: un recurso del usuario no puede
romper el editor.

### Highlight por línea, sin estado
El resaltado tokeniza CADA línea visible de forma independiente. Los tokens
multilínea (string con salto, comentario `/*…*/`, heredoc) colorean solo su
línea: límite documentado. El motivo es la constitución: la vista proyecta la
región visible y no quiere un estado de lexer global por buffer; además cada
`Draw` puede repetir el scan de las pocas líneas visibles sin costo notable.

### Colores como "roles ", no como valores esparcidos
Cada componente consume `theme.RoleX()`; ningún módulo hardcodea un `tcell.Style`
con color (salvo el default). Eso hace que un JSON futuro pueda re-mapear todo
sin tocar código, y es la costura donde un día entrará un tema de sintaxis real
(TextMate). Las reglas de highlight devuelven ROLES (comment/keyword/string…),
no colores.

## Falsificación (tests que escriben primero contra el código roto)
1. Sin `Theme` → `TestThemeParse*` no compilan.
2. Sin paleta default → `TestThemeDefaults*` fallan (roles vacíos).
3. Sin `highlight` → `TestHighlightGoKeyword` no compila.
4. Editor sin estilo de línea de cursor → `TestCursorLineHasItsBackground` falla.
5. Sin carga del JSON → `TestControllerLoadsThemeFallback` falla.

## Evidence
(Rellenar por unidad.)