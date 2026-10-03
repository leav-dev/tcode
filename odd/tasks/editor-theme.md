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
- [x] `view/theme.go`: `Theme` + roles + paleta default + parse de JSON y de
  colores (nombre/hex/índice); fallback default ante error <!-- id: 0 --> · `20c44f4`
- [x] Tests de tema: parse válido, inválido → default, cada formato de color
  <!-- id: 1 --> · `20c44f4`
- [x] `view/highlight.go`: reglas por extensión (keywords, comentario `//` `#`
  `/*`…, strings `"` `'`, números) y `styleAt(pos)` por byte; tokens de una
  línea, sin estado <!-- id: 2 --> · `a8e2245`
- [x] Tests de highlight: keyword/string/comment/number por lenguaje, límites
  de la línea, extensión desconocida = solo texto <!-- id: 3 --> · `a8e2245`
- [x] Editor: `Draw` pinta la línea del cursor con su fondo y estila los
  clusters por token <!-- id: 4 --> · `1b805c6`
- [x] Tab/status/tree/menu: estilos del tema <!-- id: 5 --> · `f7964f9`
- [x] Controller: carga `~/.tcode/theme.json` al arranque (fallback sin romper,
  aviso único); tests <!-- id: 6 --> · `f7964f9`
- [x] Integración: render con estilos verificados vía `GetContents`; suite
  completa sin fallos nuevos <!-- id: 7 -->
- [x] `docs/editor-theme.md`: formato del JSON, roles, límites del resaltado
  y roadmap (temas por proyecto, highlighter con estado multilínea)
  <!-- id: 8 -->

### Ampliación posterior: variables, tipos y funciones · `*commit*`
- [x] Roles `type`/`function`/`variable` en el scanner (sets de tipos por
  lenguaje; función = identificador con `(` pegado; el resto variable), en el
  `Theme` (defaults + JSON) y en `StyleForRole` <!-- id: 9 -->
- [x] Tests: tipos Go/Python, función (también `def nombre(` y el caso de
  `if (` que sigue keyword), variable, y strings que no se confunden;
  render de los tres roles en la celda <!-- id: 10 -->
- [x] Verificación: suite completa sin fallos nuevos vs base <!-- id: 11 -->

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

### U-tema · `20c44f4`
- **RED:** `DefaultTheme`/`LoadTheme`/`parseThemeColor` no existían.
- **Hallazgo tcell:** los índices de paleta NO son `tcell.Color(n)` (inválido
  sin el flag); la API correcta es `tcell.PaletteColor(n)`; el getter de estilo
  es `Decompose()` (Foreground es setter).
- **Ajuste de expectativa:** `"244"` != `GetColor("Gray")` (el nombre es RGB
  truecolor); el índice se verifica contra `PaletteColor(244)`.

### U-highlight · `a8e2245` + `1b805c6`
- **RED:** los unitarios del scanner y el render del editor.
- **Bug real del scanner (mío):** `styleAt` consulta una posición puntual pero
  retornaba el rol del PRIMER span de comentario/string sin verificar si `pos`
  caía dentro — los clusters iniciales tomaban roles del final (el 'x' de
  "x := 1" se pintaba Comment). Fix: cada span consulta `pos`.
- **Bug de test (mío):** `NewEditorView(m, height, width)` — los tests pasaban
  `(30, 3)` invertido: el viewport quedaba de 3 columnas y las celdas
  posteriores no se dibujaban; un assert "pasó por coincidencia" (celda vacía
  con fg default igual al esperado). Es el mismo tipo de confusión que ya
  documentó tab-bar (pestaña sobre el editor) en su unidad.
- **Decisión:** los roles "activos" conservan `Reverse` + color de acento, así
  los tests de render existentes (`cellReverse`) quedaron verdes.

### Ampliación sintaxis (variables/tipos/funciones) · `*hash*`
- **RED:** los tests del scanner y del tema no compilaban (roles inexistentes).
- **Semántica nueva ajustada en tests viejos (a propósito):** lo que era Text
  ahora es `variable` (identificadores: `funcXYZ`, `abc123`, y en archivos sin
  lenguaje); `main` de `func main()` pasó de texto a **Function** (va seguido de
  `(` — regla nueva); dos posiciones de byte de mis tests estaban mal (el `f`
  de `x := f(2)` está en el byte 5, no 6).
- **Docs:** `docs/editor-theme.md` con los roles nuevos y el límite de `(` pegado.

### U-controlador · `f7964f9`
- **GREEN:** carga con archivo, fallback con JSON roto, default sin archivo
  (path inyectable para tests).
- **Verificación final:** suite completa, set idéntico al base (48 ambientales,
  0 nuevos); `go build`/`go vet` limpios; `docs/editor-theme.md` publicado.