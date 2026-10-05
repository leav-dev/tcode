# Feature: Fondos de documento por tema (la terminal no manda)

## Description
Feedback del usuario: los temas integrados dependen del **fondo de la
terminal**. Hoy `Theme.Text` es `StyleDefault` (sin fondo propio): el
documento, el árbol y los paneles heredan el fondo default del emulador. Con
una terminal negra y el tema Light activado, la sintaxis oscura desaparece.

Solución: cada paleta integrada fija su **propio fondo de documento** (y su
fg de texto base) en `Theme.Text`; `StyleForRole` propaga ese fondo a los
roles de sintaxis (una sola costura: todos los tokens del highlighter pasan
por ahí); el editor **rellena su viewport** con `Text` para que el emulador no
asome en las celdas sin texto; `TabIdle` lleva el mismo fondo (la fila de
pestañas no desentona). Custom (`theme.json`) sin redefinir `text` hereda el
fondo del tema base (dark); con `"text": "default"` vuelve a depender de la
terminal (comportamiento original, documentado).

## Tasks
- [x] `view/theme.go`: `docBg()` (bg de `Text`); `StyleForRole` compone
  `.Background(docBg())` en roles (default devuelve `t.Text`); `DarkTheme` y
  `DefaultTheme` pasan a compartir una sola construcción con fg/bg explícitos;
  las seis fábricas ganan `Text = on(fg, bg)` y `TabIdle = on(fg, bg)` con los
  colores del documento de cada paleta <!-- id: 0 -->
- [x] `view/editor_view.go`: en `Draw`, relleno del viewport completo con
  `th.Text` antes de pintar las líneas (guard de width/height) <!-- id: 1 -->
- [x] Ajustes de expectativas: `file_browser_test.go` (el fondo del texto de
  las filas inactivas ya no es `ColorDefault`) y cualquier otro assert de
  fondo que falle en la superficie al correr la suite <!-- id: 2 -->
- [x] Tests nuevos: las 6 paletas con `Text` fg+bg no-default; `StyleForRole`
  del keyword hereda el bg del tema; un tema sin bg (StyleDefault) deja los
  roles con bg default (sigue dependiendo de la terminal) <!-- id: 3 -->
- [x] Verificación y commit: `37538bd` <!-- id: 4 -->

## Design decisions
- **`StyleForRole` es la única costura del highlighter** (grep: ningún
  consumidor usa `Theme.Comment/etc.` directo): componer ahí el bg del tema
  evita tocar `highlight.go`.
- **El relleno del editor cubre el viewport completo** (fila a fila de
  `viewport.Height × Width`, como ya hace el árbol): las filas sin texto
  también llevan el fondo del tema.
- **`bg == ColorDefault` se propaga igual** (`Background(ColorDefault)` lo
  deja en default): los temas sin fondo explícito —Custom con
  `"text": "default"`— se comportan exactamente como hoy.
- **Paletas:** dark #1E1E1E/#D4D4D4 (VSCode Dark+), light #FFFFFF/#000000,
  light-hc #FFFFFF/#000000, dark-hc #000000/#FFFFFF, tokyo-night
  #1A1B26/#C0CAF5, dracula #282A36/#F8F8F2.
- **Limitación documentada:** el esquema `theme.json` no tiene clave de fondo;
  el custom hereda el del tema base o la terminal (`"text": "default"`).